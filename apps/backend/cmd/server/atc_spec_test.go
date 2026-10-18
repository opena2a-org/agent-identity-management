package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
	infraatc "github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/atc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// The ATC verification specification is read against the server: the codes,
// statuses and next steps of its error table, and the 503 answer it shows.

const atcSpecFile = "docs/specs/atc-verification-v1.md"

// atcSpecSection returns the lines of the specification from the heading that
// starts with heading to the next heading of the same or a higher level. A
// line inside a fenced code block is never read as a heading.
func atcSpecSection(t *testing.T, heading string) string {
	t.Helper()
	requirePublicRepositoryFiles(t)
	level := len(heading) - len(strings.TrimLeft(heading, "#"))
	var section []string
	inside, fenced := false, false
	for _, line := range strings.Split(aim03ReadRepoFile(t, atcSpecFile), "\n") {
		if strings.HasPrefix(line, "```") {
			fenced = !fenced
		}
		if !fenced && strings.HasPrefix(line, "#") {
			if inside && len(line)-len(strings.TrimLeft(line, "#")) <= level {
				break
			}
			if strings.HasPrefix(line, heading) {
				inside = true
			}
		}
		if inside {
			section = append(section, line)
		}
	}
	require.NotEmpty(t, section, "%s has no heading starting %q", atcSpecFile, heading)
	return strings.Join(section, "\n")
}

// atcSpecTableRows returns the rows of the table in section whose header row
// is header, each row as its cells.
func atcSpecTableRows(t *testing.T, section string, header ...string) [][]string {
	t.Helper()
	var rows [][]string
	found := false
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			if found {
				break
			}
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		switch {
		case !found:
			found = slices.Equal(cells, header)
		case strings.Trim(cells[0], "-") != "":
			rows = append(rows, cells)
		}
	}
	require.True(t, found, "%s has no table with the header | %s |", atcSpecFile, strings.Join(header, " | "))
	return rows
}

// atcSpecVerifier is an ATC verifier that returns err for every token.
type atcSpecVerifier struct{ err error }

func (v atcSpecVerifier) Verify(string) (*atcdomain.ATCClaims, error) { return nil, v.err }

// atcSpecStatus returns the status the server's ATC middleware answers when
// the verifier returns an error with code.
func atcSpecStatus(t *testing.T, code string) int {
	t.Helper()
	app := fiber.New()
	app.Use(middleware.ATCAuthMiddleware(atcSpecVerifier{err: &atcdomain.ATCError{Code: code, Message: "the error table"}}))
	app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "ATC error-table")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

// Each row of the error table of section 4.2 gives a next step, the table's
// codes are the codes the ATC verifier returns, and the status of each is the
// one the server's ATC middleware answers when the verifier returns it.
func TestATCSpecErrorTableGivesANextStepAndTheServersStatusForEachCode(t *testing.T) {
	rows := atcSpecTableRows(t, atcSpecSection(t, "### 4.2 "), "Error Code", "HTTP Status", "Meaning", "Next step")

	var codes []string
	for _, row := range rows {
		require.Len(t, row, 4, "the error table's row %q", strings.Join(row, " | "))
		code := strings.Trim(row[0], "`")
		codes = append(codes, code)
		assert.NotEmpty(t, row[3], "the error table gives %s no next step", code)
		status, err := strconv.Atoi(row[1])
		require.NoError(t, err, "the error table's status of %s", code)
		assert.Equal(t, status, atcSpecStatus(t, code), "the error table gives %s status %d, and the server answers it with another", code, status)
	}
	assert.ElementsMatch(t, []string{
		atcdomain.ErrCodeMalformed,
		atcdomain.ErrCodeOversized,
		atcdomain.ErrCodeExpired,
		atcdomain.ErrCodeUntrustedIssuer,
		atcdomain.ErrCodeSignatureInvalid,
		atcdomain.ErrCodePQCSigInvalid,
		atcdomain.ErrCodeRevoked,
		atcdomain.ErrCodeRevocationUnknown,
		atcdomain.ErrCodeChainInvalid,
		atcdomain.ErrCodeInsufficientScope,
	}, codes, "the error table's codes are not the codes the ATC verifier returns")
}

// unreadableAgentStatus is a revocation source whose read always fails.
type unreadableAgentStatus struct{}

func (unreadableAgentStatus) AgentRevoked(context.Context, uuid.UUID) (bool, error) {
	return false, errors.New("the agent store did not answer")
}

// mintATCSpecToken mints a CBOR ATC for agentID, issued by issuer and signed
// with key, the format the server's verifier accepts.
func mintATCSpecToken(t *testing.T, key ed25519.PrivateKey, issuer string, agentID uuid.UUID) string {
	t.Helper()
	now := time.Now()
	payload := atcdomain.ATCPayload{
		ATCID:           uuid.New().String(),
		AgentID:         agentID.String(),
		Issuer:          issuer,
		IssuedAt:        uint64(now.Unix()),
		ExpiresAt:       uint64(now.Add(5 * time.Minute).Unix()),
		Capabilities:    []string{"secrets:resolve"},
		IssuerPublicKey: []byte(key.Public().(ed25519.PublicKey)),
	}
	encMode, err := cbor.CanonicalEncOptions().EncMode()
	require.NoError(t, err)
	signed, err := encMode.Marshal(atcdomain.SignedPayload{
		ATCID:           payload.ATCID,
		AgentID:         payload.AgentID,
		Issuer:          payload.Issuer,
		IssuedAt:        payload.IssuedAt,
		ExpiresAt:       payload.ExpiresAt,
		Capabilities:    payload.Capabilities,
		IssuerPublicKey: payload.IssuerPublicKey,
	})
	require.NoError(t, err)
	payload.Ed25519Sig = ed25519.Sign(key, signed)
	raw, err := encMode.Marshal(payload)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(raw)
}

// The 503 answer section 4.2 shows is the server's answer to an ATC whose
// agent's status cannot be read: its status and reason phrase, each header it
// shows, and its body.
func TestATCSpecRetryAfterExampleIsTheServersAnswer(t *testing.T) {
	var answers []string
	var block []string
	open := false
	for _, line := range strings.Split(atcSpecSection(t, "### 4.2 "), "\n") {
		switch {
		case !open && line == "```http":
			open, block = true, nil
		case open && line == "```":
			open = false
			answers = append(answers, strings.Join(block, "\n"))
		case open:
			block = append(block, line)
		}
	}
	require.Len(t, answers, 1, "section 4.2 shows %d HTTP answers; want 1, the answer with Retry-After", len(answers))
	head, body, ok := strings.Cut(answers[0], "\n\n")
	require.True(t, ok, "the answer section 4.2 shows has no blank line between its headers and its body")
	lines := strings.Split(head, "\n")
	statusLine := strings.SplitN(lines[0], " ", 3)
	require.Len(t, statusLine, 3, "the status line %q", lines[0])
	status, err := strconv.Atoi(statusLine[1])
	require.NoError(t, err, "the status line %q", lines[0])
	assert.Equal(t, "HTTP/1.1", statusLine[0])
	assert.Equal(t, http.StatusText(status), statusLine[2], "the reason phrase of status %d", status)
	headers := map[string]string{}
	for _, line := range lines[1:] {
		name, value, ok := strings.Cut(line, ": ")
		require.True(t, ok, "the header line %q", line)
		headers[name] = value
	}
	require.Contains(t, headers, fiber.HeaderRetryAfter, "the answer section 4.2 shows carries no Retry-After")

	const issuer = "https://aim.example.test"
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	verifier, err := infraatc.NewServerATCVerifier(issuer, key, unreadableAgentStatus{})
	require.NoError(t, err)
	reached := false
	app := fiber.New()
	app.Use(middleware.ATCAuthMiddleware(verifier))
	app.Get("/", func(c fiber.Ctx) error {
		reached = true
		return c.SendStatus(fiber.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "ATC "+mintATCSpecToken(t, key, issuer, uuid.New()))
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	answer, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.False(t, reached, "the route ran for an ATC whose revocation state could not be read")
	assert.Equal(t, status, resp.StatusCode)
	for name, value := range headers {
		assert.Equal(t, value, resp.Header.Get(name), "the %s header of the server's answer", name)
	}
	assert.JSONEq(t, body, string(answer), "the body of the server's answer")
}
