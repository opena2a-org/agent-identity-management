package handlers

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// A suspended agent that sends a signed action-request statement, signed with its
// registered key, is refused whatever its organisation's enforcement mode, and the
// refusal leaves one audit entry, as the plain request body's refusal does. The entry
// is written only after the signature verifies and the nonce is admitted, so a caller
// without the agent's key cannot create one and a replayed statement creates no second
// one, and it records the refusal without the statement's signature, key, signed bytes
// or context.
//
// Before, the statement form's refusal wrote nothing.

// statementSecretContext is signed into the statement's context so a test can see
// whether the context reached the audit entry.
const statementSecretContext = "context-value-that-must-not-be-recorded"

// statementRefusalApp mounts CreateVerification on the SDK and the JWT routes, as
// cmd/server does, with every audit entry captured. reached is set when a request
// gets past authentication to the capability check.
type statementRefusalApp struct {
	app      *fiber.App
	agent    *domain.Agent
	signer   *statementSigner
	admitter *memoryAdmitter
	logged   []*domain.AuditLog
	reached  bool
}

// newStatementRefusalApp answers the organisation lookup with mode, or with an error
// when orgErr is set, and the audit write with logErr.
func newStatementRefusalApp(t *testing.T, status domain.AgentStatus, mode domain.EnforcementMode, orgErr, logErr error) *statementRefusalApp {
	t.Helper()
	sa := &statementRefusalApp{signer: newStatementSigner(t), admitter: newMemoryAdmitter()}
	sa.agent = &domain.Agent{
		ID:             sa.signer.agentID,
		OrganizationID: uuid.New(),
		CreatedBy:      uuid.New(),
		Name:           "statement-agent",
		Status:         status,
		PublicKey:      sa.signer.registeredKey(),
	}
	svc := &MockAgentServiceForVerificationImpl{}
	svc.GetAgentFunc = func(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
		if id != sa.agent.ID {
			return nil, errors.New("agent not found")
		}
		copied := *sa.agent
		return &copied, nil
	}
	svc.VerifyCapabilityFunc = func(context.Context, uuid.UUID, string, string, map[string]interface{}, string) (bool, string, uuid.UUID, error) {
		sa.reached = true
		return false, "stopped by the test", uuid.Nil, errors.New("stopped by the test")
	}
	audit := &MockAuditServiceForVerificationImpl{
		LogFunc: func(_ context.Context, entry *domain.AuditLog) error {
			sa.logged = append(sa.logged, entry)
			return logErr
		},
	}
	orgs := &MockOrganizationRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Organization, error) {
			if orgErr != nil {
				return nil, orgErr
			}
			return &domain.Organization{ID: id, EnforcementMode: mode}, nil
		},
	}
	h := NewVerificationHandlerWithInterfaces(svc, audit, &MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{}, orgs).WithActionRequestNonces(sa.admitter)

	sa.app = fiber.New()
	sa.app.Post(sdkVerificationsPath, h.CreateVerification)
	jwt := sa.app.Group(jwtVerificationsPath)
	jwt.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", sa.agent.OrganizationID)
		return c.Next()
	})
	jwt.Post("/", h.CreateVerification)
	return sa
}

// signedPayload is a statement payload whose context carries statementSecretContext.
func (sa *statementRefusalApp) signedPayload(t *testing.T) string {
	return sa.signer.payload(t, `,"context":{"note":"`+statementSecretContext+`"}`)
}

func (sa *statementRefusalApp) post(t *testing.T, path string, body []byte) (int, string) {
	t.Helper()
	req := httptest.NewRequest("POST", path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := sa.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(out)
}

func TestActionRequest_ASuspendedAgentIsRefusedInBothModesAndTheRefusalIsAudited(t *testing.T) {
	cells := []struct {
		name     string
		status   domain.AgentStatus
		mode     domain.EnforcementMode
		orgErr   error
		wantMode string
	}{
		{name: "strict", status: domain.AgentStatusSuspended, mode: domain.EnforcementModeStrict, wantMode: "strict"},
		{name: "monitoring", status: domain.AgentStatusSuspended, mode: domain.EnforcementModeMonitoring, wantMode: "monitoring"},
		{name: "revoked", status: domain.AgentStatusRevoked, mode: domain.EnforcementModeMonitoring, wantMode: "monitoring"},
		// The refusal does not depend on the mode: an organisation that does not load
		// still gets the refusal, recorded with the mode unknown.
		{name: "organisation lookup fails", status: domain.AgentStatusSuspended, orgErr: errors.New("organisation lookup failed"), wantMode: "unknown"},
	}
	for _, path := range []string{sdkVerificationsPath, jwtVerificationsPath} {
		for _, cell := range cells {
			t.Run(path+"/"+cell.name, func(t *testing.T) {
				sa := newStatementRefusalApp(t, cell.status, cell.mode, cell.orgErr, nil)

				code, body := sa.post(t, path, sa.signer.statement(sa.signedPayload(t)))
				require.Equal(t, fiber.StatusUnauthorized, code, body)
				require.Equal(t, refusalBody("agentStatusDenied", domain.AgentStatusDeniedMessage(cell.status)), body)
				require.False(t, sa.reached, "a refused agent must not reach the capability check")

				require.Len(t, sa.logged, 1, "the refusal must write exactly one audit entry")
				entry := sa.logged[0]
				require.Equal(t, verificationRefusedAction, entry.Action)
				require.Equal(t, sa.agent.OrganizationID, entry.OrganizationID)
				require.NotNil(t, entry.UserID)
				require.Equal(t, sa.agent.CreatedBy, *entry.UserID)
				require.NotNil(t, entry.AgentID)
				require.Equal(t, sa.agent.ID, *entry.AgentID)
				require.Equal(t, "agent_action", entry.ResourceType)
				require.Equal(t, sa.agent.ID, entry.ResourceID)
				require.NotEmpty(t, entry.IPAddress)

				keys := make([]string, 0, len(entry.Metadata))
				for k := range entry.Metadata {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				require.Equal(t, []string{"agentStatus", "capability", "enforcementMode", "httpStatus", "refusal", "resource"}, keys)
				require.Equal(t, "agent_status", entry.Metadata["refusal"])
				require.Equal(t, string(cell.status), entry.Metadata["agentStatus"])
				require.Equal(t, cell.wantMode, entry.Metadata["enforcementMode"])
				require.Equal(t, "db:read", entry.Metadata["capability"])
				require.Equal(t, "orders", entry.Metadata["resource"])
				require.Equal(t, fiber.StatusUnauthorized, entry.Metadata["httpStatus"])
			})
		}
	}
}

func TestActionRequest_TheRefusalEntryCarriesNoSignatureKeySignedBytesOrContext(t *testing.T) {
	sa := newStatementRefusalApp(t, domain.AgentStatusSuspended, domain.EnforcementModeStrict, nil, nil)
	frame := paeFrame(domain.ActionRequestPayloadType, []byte(sa.signedPayload(t)))
	sig := ed25519.Sign(sa.signer.priv, frame)

	code, body := sa.post(t, sdkVerificationsPath, sa.signer.bodyWith(frame, sig, sa.signer.pub))
	require.Equal(t, fiber.StatusUnauthorized, code, body)
	require.Len(t, sa.logged, 1, "the refusal must write exactly one audit entry")
	metadata := sa.logged[0].Metadata

	for _, forbidden := range []string{"signature", "publicKey", "message", "context", "signedBytes", "signedStatement", "nonce"} {
		require.NotContains(t, metadata, forbidden)
	}
	for k, v := range metadata {
		if k == "capability" || k == "resource" {
			continue
		}
		if s, ok := v.(string); ok {
			require.LessOrEqual(t, len(s), 64, "metadata %q carries a value-shaped string", k)
		}
	}
	raw, err := json.Marshal(sa.logged[0])
	require.NoError(t, err)
	for name, value := range map[string]string{
		"public key (standard)":  base64.StdEncoding.EncodeToString(sa.signer.pub),
		"public key (url)":       b64url(sa.signer.pub),
		"signature (url)":        b64url(sig),
		"signed context":         statementSecretContext,
		"signed bytes (url, 32)": b64url(frame)[:32],
	} {
		require.NotContains(t, string(raw), value, "the refusal entry carries the %s", name)
	}
}

func TestActionRequest_AFailedRefusalAuditWriteStillRefuses(t *testing.T) {
	sa := newStatementRefusalApp(t, domain.AgentStatusSuspended, domain.EnforcementModeMonitoring, nil, errors.New("audit store unavailable"))

	code, body := sa.post(t, sdkVerificationsPath, sa.signer.statement(sa.signedPayload(t)))
	require.Equal(t, fiber.StatusUnauthorized, code, body)
	require.Equal(t, refusalBody("agentStatusDenied", domain.AgentStatusDeniedMessage(domain.AgentStatusSuspended)), body)
	require.False(t, sa.reached)
}

// Guard: only a fresh statement signed with the agent's registered key creates an
// entry. A different key, a bad signature, a statement outside the time window and a
// replay of a refused statement write none, so the audit log cannot be filled by
// anyone who knows only an agent id, nor by re-sending one captured statement.
func TestActionRequest_OnlyAFreshSignedStatementWritesARefusalEntry(t *testing.T) {
	other := newStatementSigner(t)
	for _, mode := range []domain.EnforcementMode{domain.EnforcementModeStrict, domain.EnforcementModeMonitoring} {
		t.Run(string(mode)+"/different key", func(t *testing.T) {
			sa := newStatementRefusalApp(t, domain.AgentStatusSuspended, mode, nil, nil)
			impostor := &statementSigner{agentID: sa.signer.agentID, pub: other.pub, priv: other.priv}
			code, body := sa.post(t, sdkVerificationsPath, impostor.statement(impostor.payload(t, "")))
			require.Equal(t, fiber.StatusUnauthorized, code, body)
			require.Empty(t, sa.logged, "a statement that failed authentication wrote an audit entry")
		})
		t.Run(string(mode)+"/flipped signature", func(t *testing.T) {
			sa := newStatementRefusalApp(t, domain.AgentStatusSuspended, mode, nil, nil)
			frame := paeFrame(domain.ActionRequestPayloadType, []byte(sa.signer.payload(t, "")))
			sig := ed25519.Sign(sa.signer.priv, frame)
			sig[10] ^= 0x01
			code, body := sa.post(t, sdkVerificationsPath, sa.signer.bodyWith(frame, sig, sa.signer.pub))
			require.Equal(t, fiber.StatusUnauthorized, code, body)
			require.Empty(t, sa.logged, "a statement that failed authentication wrote an audit entry")
		})
		t.Run(string(mode)+"/outside the window", func(t *testing.T) {
			sa := newStatementRefusalApp(t, domain.AgentStatusSuspended, mode, nil, nil)
			payload := fmt.Sprintf(`{"action_type":"db:read","agent_id":"%s","resource":"orders","timestamp":"%s","nonce":"%s"}`,
				sa.signer.agentID, time.Now().Add(-31*time.Second).UTC().Format(time.RFC3339Nano), newTestNonce(t))
			code, body := sa.post(t, sdkVerificationsPath, sa.signer.statement(payload))
			require.Equal(t, fiber.StatusUnauthorized, code, body)
			require.Empty(t, sa.logged, "a statement refused by the time window wrote an audit entry")
		})
		t.Run(string(mode)+"/replay of a refused statement", func(t *testing.T) {
			sa := newStatementRefusalApp(t, domain.AgentStatusSuspended, mode, nil, nil)
			statement := sa.signer.statement(sa.signer.payload(t, ""))
			code, body := sa.post(t, sdkVerificationsPath, statement)
			require.Equal(t, fiber.StatusUnauthorized, code, body)
			require.Len(t, sa.logged, 1, "the first refusal must write exactly one audit entry")

			for _, path := range []string{sdkVerificationsPath, jwtVerificationsPath} {
				code, body = sa.post(t, path, statement)
				require.Equal(t, fiber.StatusUnauthorized, code, body)
				require.Equal(t, refusalBody("nonceReused", "this nonce was already used by this agent"), body)
			}
			require.Len(t, sa.logged, 1, "a replayed statement wrote another audit entry")
		})
	}
}

// Guard: an agent whose status permits it is not recorded as refused.
func TestActionRequest_AnActiveAgentIsNotRecordedAsRefused(t *testing.T) {
	sa := newStatementRefusalApp(t, domain.AgentStatusVerified, domain.EnforcementModeStrict, nil, nil)

	sa.post(t, sdkVerificationsPath, sa.signer.statement(sa.signer.payload(t, "")))
	require.True(t, sa.reached, "the control must get past authentication")
	for _, entry := range sa.logged {
		require.NotEqual(t, verificationRefusedAction, entry.Action)
	}
}
