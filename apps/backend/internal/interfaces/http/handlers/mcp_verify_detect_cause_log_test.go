package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// Verifying an MCP server and detecting its capabilities answer a service
// failure without the service's error, and write no error text to the audit
// entry the organization's members read: the error can name the address a
// refused connection was aimed at, or storage details. The error goes to the
// server log with the server's ID.

// mcpCauseLogDetail is the service error the routes below fail with. It must
// reach the server log and nothing else.
const mcpCauseLogDetail = "FAKE-storage-detail dial tcp 10.0.0.5:5432: connection reset"

var (
	mcpCauseLogOrgID    = uuid.MustParse("44444444-4444-4444-4444-444444444444")
	mcpCauseLogUserID   = uuid.MustParse("55555555-5555-5555-5555-555555555555")
	mcpCauseLogServerID = uuid.MustParse("66666666-6666-6666-6666-666666666666")
)

// mcpCauseLogServicer serves the stored server, in the caller's organization,
// and fails the verification challenge with challengeErr and the verification
// with verifyErr.
type mcpCauseLogServicer struct {
	MCPServicerExtended
	challengeErr error
	verifyErr    error
}

func (mcpCauseLogServicer) GetMCPServer(context.Context, uuid.UUID) (*domain.MCPServer, error) {
	return &domain.MCPServer{ID: mcpCauseLogServerID, OrganizationID: mcpCauseLogOrgID, Name: "cause-log-mcp"}, nil
}

func (s mcpCauseLogServicer) GenerateVerificationChallenge(context.Context, uuid.UUID) (string, error) {
	if s.challengeErr != nil {
		return "", s.challengeErr
	}
	return "FAKE-challenge", nil
}

func (s mcpCauseLogServicer) VerifyMCPServer(context.Context, uuid.UUID, uuid.UUID, string) error {
	return s.verifyErr
}

// mcpCauseLogCapabilities fails capability detection with detectErr and the
// read of the detected capabilities with getErr.
type mcpCauseLogCapabilities struct {
	MCPCapabilityServicer
	detectErr error
	getErr    error
}

func (s mcpCauseLogCapabilities) DetectCapabilities(context.Context, uuid.UUID) error {
	return s.detectErr
}

func (s mcpCauseLogCapabilities) GetCapabilities(context.Context, uuid.UUID) ([]*domain.MCPServerCapability, error) {
	return nil, s.getErr
}

// mcpCauseLogAudit records the metadata of every audit entry written.
type mcpCauseLogAudit struct {
	AuditServicer
	entries *[]map[string]interface{}
}

func (a mcpCauseLogAudit) LogAction(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ domain.AuditAction, _ string, _ uuid.UUID, _ string, _ string, metadata map[string]interface{}) error {
	*a.entries = append(*a.entries, metadata)
	return nil
}

type mcpCauseLogAnswer struct {
	status int
	body   string
	log    string
}

func callMCPCauseLogRoute(t *testing.T, handler *MCPHandler, path string) mcpCauseLogAnswer {
	t.Helper()
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	app := fiber.New()
	// A handler that panics is answered 500, so each case reports its own
	// outcome instead of ending the test binary.
	app.Use(recover.New())
	app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", mcpCauseLogOrgID)
		c.Locals("user_id", mcpCauseLogUserID)
		return c.Next()
	})
	app.Post("/mcp-servers/:id/verify", handler.VerifyMCPServer)
	app.Post("/mcp-servers/:id/detect-capabilities", handler.DetectCapabilities)

	resp, err := app.Test(httptest.NewRequest(http.MethodPost, path, nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return mcpCauseLogAnswer{status: resp.StatusCode, body: string(body), log: logged.String()}
}

// A failure at any step of verifying an MCP server or detecting its
// capabilities is answered without the service's error, no audit entry carries
// it, and it is written to the server log with the server's ID. A failed
// verification is still audited, with its outcome.
func TestMCPVerifyAndDetectKeepTheCauseInTheServerLog(t *testing.T) {
	serviceErr := errors.New(mcpCauseLogDetail)
	serverPath := "/mcp-servers/" + mcpCauseLogServerID.String()
	for _, tc := range []struct {
		name         string
		path         string
		mcp          mcpCauseLogServicer
		capabilities mcpCauseLogCapabilities
		status       int
		// want is the body, or for the verification answer, which carries
		// the time it was checked, the part of it that names the outcome.
		want    string
		audited int
	}{
		{
			name: "verification challenge", path: serverPath + "/verify",
			mcp:    mcpCauseLogServicer{challengeErr: serviceErr},
			status: fiber.StatusInternalServerError,
			want:   `{"error":"Failed to generate verification challenge"}`,
		},
		{
			name: "verification", path: serverPath + "/verify",
			mcp:     mcpCauseLogServicer{verifyErr: serviceErr},
			status:  fiber.StatusOK,
			want:    `"message":"Verification failed: server is unreachable"`,
			audited: 1,
		},
		{
			name: "capability detection", path: serverPath + "/detect-capabilities",
			capabilities: mcpCauseLogCapabilities{detectErr: serviceErr},
			status:       fiber.StatusInternalServerError,
			want:         `{"error":"` + ServerErrorMessage + `"}`,
		},
		{
			name: "capability read after detection", path: serverPath + "/detect-capabilities",
			capabilities: mcpCauseLogCapabilities{getErr: serviceErr},
			status:       fiber.StatusInternalServerError,
			want:         `{"error":"Failed to fetch capabilities after detection"}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var entries []map[string]interface{}
			handler := NewMCPHandlerWithInterfaces(tc.mcp, tc.capabilities, mcpCauseLogAudit{entries: &entries}, nil, nil, nil, nil)
			got := callMCPCauseLogRoute(t, handler, tc.path)

			if got.status != tc.status {
				t.Errorf("status = %d, body = %s; want %d", got.status, got.body, tc.status)
			}
			if !strings.Contains(got.body, tc.want) {
				t.Errorf("body = %s, want %s", got.body, tc.want)
			}
			if strings.Contains(got.body, mcpCauseLogDetail) {
				t.Errorf("body = %s; it carries the service error", got.body)
			}
			if len(entries) != tc.audited {
				t.Errorf("audit entries = %d, want %d", len(entries), tc.audited)
			}
			for _, metadata := range entries {
				for key, value := range metadata {
					if strings.Contains(fmt.Sprint(value), mcpCauseLogDetail) {
						t.Errorf("the audit entry's %q carries the service error: %v", key, value)
					}
				}
			}
			var line string
			for _, l := range strings.Split(got.log, "\n") {
				if strings.Contains(l, mcpCauseLogDetail) {
					line = l
					break
				}
			}
			if line == "" {
				t.Fatalf("the server log does not carry the service error:\n%s", got.log)
			}
			if !strings.Contains(line, mcpCauseLogServerID.String()) {
				t.Errorf("the server log line does not name the server: %s", line)
			}
		})
	}
}
