package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Containment stage of the verification write-path fix.
//
// WHAT THESE TESTS ARE FOR, and what they deliberately are NOT.
//
// The defect these guard was not "an unsigned request was accepted". It was that
// POST /api/v1/sdk-api/verifications/:id/result wrote the AUTHORIZATION DECISION
// from a body the agent controls, and the agent whose action is pending approval
// is handed the verification ID by CreateVerification itself. Authentication
// cannot fix that, because in the primary scenario the attacker IS the owning
// agent and holds its own signing key.
//
// So the test that matters here is
// TestSubmitVerificationResult_OwningAgentCannotApprovePendingEvent: a caller
// that has passed every authentication control and genuinely owns the event
// still cannot move it from pending to approved. A suite that only proved
// "unauthenticated is rejected" would be green over a live bypass — the exact
// shape both chiefs named as the most likely way this fix fails.
//
// This file is cloud-only by construction. The sync from the public repo copies
// files that exist there over cloud's copies; a file with no public counterpart
// is never overwritten, and only apps/backend/{schema,scripts,seed,tests}/ are
// delete-mirrored. So if a future sync restores the vulnerable handler, these go
// red in pr-check.yml rather than shipping.

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

// writeSpy records whether the handlers reached a store. Absence of a write is
// the real assertion; a status code alone would pass even if the row changed.
type writeSpy struct {
	updateResultCalls    int
	updateExecCalls      int
	lastExecStrictMode   bool
	lastExecExecuted     bool
	lastExecutionErrorIn *string
}

func (s *writeSpy) service(event *domain.VerificationEvent) *MockVerificationEventServiceForVerificationImpl {
	return &MockVerificationEventServiceForVerificationImpl{
		GetVerificationEventFunc: func(ctx context.Context, id uuid.UUID) (*domain.VerificationEvent, error) {
			if event == nil || event.ID != id {
				return nil, fmt.Errorf("verification event not found")
			}
			return event, nil
		},
		UpdateVerificationResultFunc: func(ctx context.Context, id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
			s.updateResultCalls++
			return nil
		},
		UpdateExecutionStatusFunc: func(ctx context.Context, id uuid.UUID, executed bool, strictMode bool, executedAt time.Time, executionError *string) error {
			s.updateExecCalls++
			s.lastExecExecuted = executed
			s.lastExecStrictMode = strictMode
			s.lastExecutionErrorIn = executionError
			return nil
		},
	}
}

// principal is what a middleware chain leaves in Locals. The sdkAPI group can
// produce any of these: PQCAgentMiddleware sets agent_id + organization_id
// (pqc_agent_auth.go:162-163), and its JWT fall-through (:39-41, :50-52)
// reaches AuthMiddleware, which sets organization_id/user_id and never agent_id
// (auth.go:132-135).
type principal struct {
	name    string
	agentID *uuid.UUID // nil => no agent principal, i.e. a JWT/cookie caller
	orgID   *uuid.UUID
	userID  *uuid.UUID
}

func agentPrincipal(name string, agentID, orgID uuid.UUID) principal {
	return principal{name: name, agentID: &agentID, orgID: &orgID}
}

func jwtPrincipal(name string, orgID uuid.UUID) principal {
	userID := uuid.New()
	return principal{name: name, orgID: &orgID, userID: &userID}
}

func anonymousPrincipal() principal { return principal{name: "unauthenticated"} }

// sdkWriteApp mounts the two write handlers behind a middleware that injects the
// given principal, mirroring what the real sdkAPI group leaves in Locals. The
// paths match cmd/server/main.go's group registrations.
func sdkWriteApp(handler *VerificationHandler, p principal) *fiber.App {
	app := fiber.New()
	group := app.Group("/api/v1/sdk-api")
	group.Use(func(c fiber.Ctx) error {
		if p.agentID != nil {
			c.Locals("agent_id", *p.agentID)
			c.Locals("authenticated_via", "ed25519")
			c.Locals("auth_method", "ed25519")
		}
		if p.orgID != nil {
			c.Locals("organization_id", *p.orgID)
		}
		if p.userID != nil {
			c.Locals("user_id", *p.userID)
		}
		return c.Next()
	})
	group.Post("/verifications/:id/result", handler.SubmitVerificationResult)
	group.Post("/verifications/:id/execution-status", handler.UpdateExecutionStatus)
	return app
}

func newHandlerFor(svc VerificationEventServicerForVerification, orgRepo OrganizationRepositoryer) *VerificationHandler {
	return NewVerificationHandlerWithInterfaces(
		&MockAgentServiceForVerificationImpl{},
		&MockAuditServiceForVerificationImpl{},
		&MockAlertServiceForVerificationImpl{},
		svc,
		orgRepo,
	)
}

// pendingEvent is the state the whole defect turns on: a strict-mode JIT request
// that CreateVerification wrote with Result nil (verification_handler.go:457-469)
// and that the SDK is polling, reading "pending".
func pendingEvent(agentID, orgID uuid.UUID) *domain.VerificationEvent {
	return &domain.VerificationEvent{
		ID:             uuid.New(),
		OrganizationID: orgID,
		AgentID:        &agentID,
		Status:         domain.VerificationEventStatusPending,
		Result:         nil,
		CreatedAt:      time.Now().Add(-time.Minute),
	}
}

func postJSON(t *testing.T, app *fiber.App, path string, body map[string]interface{}) (*fiber.Ctx, int, map[string]interface{}) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest("POST", path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	var decoded map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&decoded)
	return nil, resp.StatusCode, decoded
}

// ---------------------------------------------------------------------------
// R1: the test that matters
// ---------------------------------------------------------------------------

// TestSubmitVerificationResult_OwningAgentCannotApprovePendingEvent is CISO R1's
// acceptance test. The caller here is not an attacker who stole anything: it has
// passed the group's Ed25519 signature check, the ±30s window and the revocation
// gate, and it is the agent the event actually belongs to. It must still be
// unable to turn its own pending authorization request into an approval.
//
// RED-PROOF: on pre-fix code this fails on both assertions — the handler returned
// 200 {"status":"result_recorded"} and called UpdateVerificationResult, which
// sets result='verified', status='success', which the SDK reads back as
// status:"approved", approvedBy:"system" and executes on.
func TestSubmitVerificationResult_OwningAgentCannotApprovePendingEvent(t *testing.T) {
	agentID, orgID := uuid.New(), uuid.New()
	event := pendingEvent(agentID, orgID)

	spy := &writeSpy{}
	handler := newHandlerFor(spy.service(event), &MockOrganizationRepositoryerImpl{})
	app := sdkWriteApp(handler, agentPrincipal("owning agent", agentID, orgID))

	_, status, body := postJSON(t, app,
		fmt.Sprintf("/api/v1/sdk-api/verifications/%s/result", event.ID),
		map[string]interface{}{"result": "success", "reason": "done"})

	assert.Equal(t, fiber.StatusForbidden, status,
		"a correctly-authenticated OWNING agent must still be refused; authentication is not the control here")
	assert.Equal(t, "executionOutcomeNotAccepted", body["code"],
		"the refusal must be machine-readable and distinct from the group's auth failure, so a degraded SDK is visibly degraded")

	// The load-bearing assertion. A 403 with a completed write would pass a
	// status-code-only test and still be the live bypass.
	assert.Zero(t, spy.updateResultCalls,
		"the authorization-decision write must not be reached at all; R1 is satisfied by construction, not by a guard")
	assert.Nil(t, event.Result, "the pending event must remain pending")
}

// TestSubmitVerificationResult_RefusesEveryPrincipal pins that the refusal is not
// scoped to agents. Keeping the route open for JWT callers would re-create the
// cross-tenant write the ledger already recorded (the second mount at
// main.go:2101), now with an ownership test passing beside it.
func TestSubmitVerificationResult_RefusesEveryPrincipal(t *testing.T) {
	agentID, orgID := uuid.New(), uuid.New()
	event := pendingEvent(agentID, orgID)

	for _, p := range []principal{
		agentPrincipal("owning agent", agentID, orgID),
		agentPrincipal("other agent, same org", uuid.New(), orgID),
		agentPrincipal("other agent, other org", uuid.New(), uuid.New()),
		jwtPrincipal("jwt user, same org", orgID),
		jwtPrincipal("jwt user, other org", uuid.New()),
		anonymousPrincipal(),
	} {
		t.Run(p.name, func(t *testing.T) {
			spy := &writeSpy{}
			handler := newHandlerFor(spy.service(event), &MockOrganizationRepositoryerImpl{})
			app := sdkWriteApp(handler, p)

			_, status, _ := postJSON(t, app,
				fmt.Sprintf("/api/v1/sdk-api/verifications/%s/result", event.ID),
				map[string]interface{}{"result": "success"})

			assert.Equal(t, fiber.StatusForbidden, status)
			assert.Zero(t, spy.updateResultCalls, "no principal may reach the decision write")
		})
	}
}

// TestSubmitVerificationResult_RefusesBeforeAnyLookup pins that the handler is
// terminal rather than merely guarded: it must not read the store even to decide
// whom to refuse, so the route cannot be used as an existence oracle for a
// guessed UUID.
func TestSubmitVerificationResult_RefusesBeforeAnyLookup(t *testing.T) {
	reads := 0
	svc := &MockVerificationEventServiceForVerificationImpl{
		GetVerificationEventFunc: func(ctx context.Context, id uuid.UUID) (*domain.VerificationEvent, error) {
			reads++
			return nil, fmt.Errorf("must not be called")
		},
	}
	handler := newHandlerFor(svc, &MockOrganizationRepositoryerImpl{})
	app := sdkWriteApp(handler, agentPrincipal("owning agent", uuid.New(), uuid.New()))

	// A malformed ID would previously have produced a distinguishable 400.
	for _, id := range []string{uuid.New().String(), "not-a-uuid", ""} {
		_, status, _ := postJSON(t, app,
			fmt.Sprintf("/api/v1/sdk-api/verifications/%s/result", id),
			map[string]interface{}{"result": "success"})
		if id == "" {
			continue // routes to a different path; nothing to assert
		}
		assert.Equal(t, fiber.StatusForbidden, status,
			"every id shape gets the same refusal, so the response carries no information about the row")
	}
	assert.Zero(t, reads, "the withdrawn endpoint must perform no lookup")
}

// ---------------------------------------------------------------------------
// /execution-status: ownership, and the JWT fall-through
// ---------------------------------------------------------------------------

// TestUpdateExecutionStatus_OwningAgentSucceeds is the POSITIVE control. Without
// it the negative tests below could all pass because the route is broken for
// everyone, which would be a different defect wearing this fix's clothes.
func TestUpdateExecutionStatus_OwningAgentSucceeds(t *testing.T) {
	agentID, orgID := uuid.New(), uuid.New()
	event := pendingEvent(agentID, orgID)

	spy := &writeSpy{}
	handler := newHandlerFor(spy.service(event), &MockOrganizationRepositoryerImpl{})
	app := sdkWriteApp(handler, agentPrincipal("owning agent", agentID, orgID))

	_, status, body := postJSON(t, app,
		fmt.Sprintf("/api/v1/sdk-api/verifications/%s/execution-status", event.ID),
		map[string]interface{}{"executed": true, "executedAt": time.Now().Format(time.RFC3339)})

	require.Equal(t, fiber.StatusOK, status, "the owning agent must still be able to report execution")
	assert.Equal(t, "execution_status_recorded", body["status"])
	assert.Equal(t, 1, spy.updateExecCalls)
	assert.True(t, spy.lastExecExecuted)
}

// TestUpdateExecutionStatus_RejectsNonOwners covers the cases the group move
// alone does NOT close. The JWT rows are the important ones: the group's
// Ed25519 middleware hands off to the JWT middleware whenever an Authorization
// header is present, so after the move a human user of any organization reaches
// this handler with no agent_id at all. An ownership check written as
// `if id, ok := ...; ok && id != owner { reject }` — the idiom at
// trust_score_handler.go:415 — never fires for them and converts an
// unauthenticated forgery into an authenticated cross-tenant one.
//
// RED-PROOF: every row fails on pre-fix code, which returned 200 and wrote.
func TestUpdateExecutionStatus_RejectsNonOwners(t *testing.T) {
	agentID, orgID := uuid.New(), uuid.New()

	for _, tc := range []struct {
		p     principal
		event *domain.VerificationEvent
	}{
		{agentPrincipal("other agent, same org", uuid.New(), orgID), pendingEvent(agentID, orgID)},
		{agentPrincipal("other agent, other org", uuid.New(), uuid.New()), pendingEvent(agentID, orgID)},
		{jwtPrincipal("jwt user, same org as event", orgID), pendingEvent(agentID, orgID)},
		{jwtPrincipal("jwt user, unrelated org", uuid.New()), pendingEvent(agentID, orgID)},
		{anonymousPrincipal(), pendingEvent(agentID, orgID)},
	} {
		t.Run(tc.p.name, func(t *testing.T) {
			spy := &writeSpy{}
			handler := newHandlerFor(spy.service(tc.event), &MockOrganizationRepositoryerImpl{})
			app := sdkWriteApp(handler, tc.p)

			_, status, body := postJSON(t, app,
				fmt.Sprintf("/api/v1/sdk-api/verifications/%s/execution-status", tc.event.ID),
				map[string]interface{}{"executed": false})

			assert.Equal(t, fiber.StatusNotFound, status,
				"404 not 403: the response must not distinguish 'exists but not yours' from 'does not exist'")
			assert.Equal(t, "not found", body["error"],
				"the body must be identical to a genuine miss, or it is an existence oracle")
			assert.Zero(t, spy.updateExecCalls, "a non-owner must not reach the write")
		})
	}
}

// TestUpdateExecutionStatus_EventWithNoAgentIsNotOwnable: a verification event
// may target an MCP server instead of an agent, leaving AgentID nil. A nil owner
// must be refused, never treated as a wildcard that any caller matches.
func TestUpdateExecutionStatus_EventWithNoAgentIsNotOwnable(t *testing.T) {
	orgID := uuid.New()
	event := pendingEvent(uuid.New(), orgID)
	event.AgentID = nil

	spy := &writeSpy{}
	handler := newHandlerFor(spy.service(event), &MockOrganizationRepositoryerImpl{})
	app := sdkWriteApp(handler, agentPrincipal("some agent", uuid.New(), orgID))

	_, status, _ := postJSON(t, app,
		fmt.Sprintf("/api/v1/sdk-api/verifications/%s/execution-status", event.ID),
		map[string]interface{}{"executed": true})

	assert.Equal(t, fiber.StatusNotFound, status)
	assert.Zero(t, spy.updateExecCalls)
}

// TestUpdateExecutionStatus_UnknownEventDoesNotDistinguish pins that a genuine
// miss and a cross-agent hit produce byte-identical responses.
func TestUpdateExecutionStatus_UnknownEventDoesNotDistinguish(t *testing.T) {
	agentID, orgID := uuid.New(), uuid.New()
	realEvent := pendingEvent(agentID, orgID)

	spy := &writeSpy{}
	handler := newHandlerFor(spy.service(realEvent), &MockOrganizationRepositoryerImpl{})

	// Same caller, two ids: one that exists but belongs to another agent, one
	// that does not exist at all.
	app := sdkWriteApp(handler, agentPrincipal("stranger", uuid.New(), orgID))

	_, statusExisting, bodyExisting := postJSON(t, app,
		fmt.Sprintf("/api/v1/sdk-api/verifications/%s/execution-status", realEvent.ID),
		map[string]interface{}{"executed": true})
	_, statusMissing, bodyMissing := postJSON(t, app,
		fmt.Sprintf("/api/v1/sdk-api/verifications/%s/execution-status", uuid.New()),
		map[string]interface{}{"executed": true})

	assert.Equal(t, statusMissing, statusExisting)
	assert.Equal(t, bodyMissing, bodyExisting,
		"an attacker must not be able to tell a real other-agent event from a guess")
	assert.Zero(t, spy.updateExecCalls)
}

// ---------------------------------------------------------------------------
// strict_mode is ours, not the agent's
// ---------------------------------------------------------------------------

// TestUpdateExecutionStatus_StrictModeIsServerDerived is the D-B half. The alert
// detail panel renders "Action BLOCKED (strict mode enforced)" from executed +
// strictMode. While strictMode came off the request body, the agent being
// audited could assert what OUR enforcement configuration was.
//
// RED-PROOF: the monitoring case fails on pre-fix code, which stored the
// body-supplied true.
func TestUpdateExecutionStatus_StrictModeIsServerDerived(t *testing.T) {
	for _, tc := range []struct {
		name     string
		orgMode  domain.EnforcementMode
		bodySays bool
		want     bool
	}{
		{"monitoring org cannot be made to look strict", domain.EnforcementModeMonitoring, true, false},
		{"strict org is recorded strict even if the agent denies it", domain.EnforcementModeStrict, false, true},
		{"agreement is still server-sourced", domain.EnforcementModeStrict, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agentID, orgID := uuid.New(), uuid.New()
			event := pendingEvent(agentID, orgID)

			spy := &writeSpy{}
			orgRepo := &MockOrganizationRepositoryerImpl{
				GetByIDFunc: func(id uuid.UUID) (*domain.Organization, error) {
					return &domain.Organization{ID: id, EnforcementMode: tc.orgMode}, nil
				},
			}
			handler := newHandlerFor(spy.service(event), orgRepo)
			app := sdkWriteApp(handler, agentPrincipal("owning agent", agentID, orgID))

			_, status, body := postJSON(t, app,
				fmt.Sprintf("/api/v1/sdk-api/verifications/%s/execution-status", event.ID),
				map[string]interface{}{"executed": false, "strictMode": tc.bodySays})

			require.Equal(t, fiber.StatusOK, status)
			assert.Equal(t, tc.want, spy.lastExecStrictMode,
				"strict_mode stored must come from the organization, not the request body")
			assert.Equal(t, tc.want, body["strictMode"],
				"the echoed value must be the server-derived one, or a caller learns its input was ignored only from the database")
		})
	}
}

// TestUpdateExecutionStatus_UnknownEnforcementModeRecordsNothing pins the
// unknown case. strict_mode is persisted and later rendered to an operator as a
// statement about OUR configuration, so when the organization cannot be read the
// handler must decline to write rather than pick a label. Defaulting to false
// would print "enforcement mode: monitoring" for what may have been a strict
// organization — a smaller false claim, but the same kind this change removes.
func TestUpdateExecutionStatus_UnknownEnforcementModeRecordsNothing(t *testing.T) {
	for _, tc := range []struct {
		name    string
		orgRepo OrganizationRepositoryer
	}{
		{"lookup errors", &MockOrganizationRepositoryerImpl{
			GetByIDFunc: func(id uuid.UUID) (*domain.Organization, error) {
				return nil, fmt.Errorf("connection reset")
			},
		}},
		{"organization missing", &MockOrganizationRepositoryerImpl{
			GetByIDFunc: func(id uuid.UUID) (*domain.Organization, error) {
				return nil, nil
			},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agentID, orgID := uuid.New(), uuid.New()
			event := pendingEvent(agentID, orgID)

			spy := &writeSpy{}
			handler := newHandlerFor(spy.service(event), tc.orgRepo)
			app := sdkWriteApp(handler, agentPrincipal("owning agent", agentID, orgID))

			_, status, body := postJSON(t, app,
				fmt.Sprintf("/api/v1/sdk-api/verifications/%s/execution-status", event.ID),
				map[string]interface{}{"executed": false})

			assert.Equal(t, fiber.StatusServiceUnavailable, status,
				"an unknown enforcement mode must not be coerced into a persisted label")
			assert.Equal(t, "enforcementModeUnavailable", body["code"])
			assert.Zero(t, spy.updateExecCalls,
				"nothing may be written when the mode cannot be established")
		})
	}
}

// TestUpdateExecutionStatusRequest_HasNoStrictModeField stops the field being
// quietly reintroduced. Deleting the assignment is reversible in one line; a
// struct with no such field makes the reintroduction a compile-visible change.
func TestUpdateExecutionStatusRequest_HasNoStrictModeField(t *testing.T) {
	typ := reflect.TypeOf(UpdateExecutionStatusRequest{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		// Normalise away the spellings a reintroduction could hide behind:
		// StrictMode, strictMode, strict_mode, strict-mode all collapse to the
		// same token. Checking only "strictmode" would miss `json:"strict_mode"`.
		norm := func(s string) string {
			s = strings.ToLower(s)
			for _, sep := range []string{"_", "-", " "} {
				s = strings.ReplaceAll(s, sep, "")
			}
			return s
		}
		tagName, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		assert.NotContains(t, norm(f.Name), "strictmode",
			"strict mode must be server-derived; it must not be bindable from the request body")
		assert.NotContains(t, norm(tagName), "strictmode",
			"no json tag may bind strict mode from the request body (any spelling)")
	}
}

// ---------------------------------------------------------------------------
// Class guards — these outlive the two handlers
// ---------------------------------------------------------------------------

// TestUpdateVerificationResultHasOnlyPermittedCallSites is the class guard. The
// point of Stage 1 is not that these two handlers were fixed; it is that
// result/status have a closed set of writers. This fails if any handler in the
// file re-opens the authorization-decision write, including a new one nobody
// thought to review.
//
// Permitted writers:
// the lazy-expiry path, ApproveVerification, DenyVerification. Deliberately
// enumerated by ENCLOSING FUNCTION rather than by line number, so the guard
// survives edits above it and names what it is protecting.
//
// KNOWN LIMITS of this guard, stated so nobody mistakes it for the whole class:
// it parses verification_handler.go only, and matches the selector name
// UpdateVerificationResult. It does NOT see a direct call to the repository's
// UpdateResult, a writer added in a different handler file, the raw-SQL expiry
// job in cmd/server/main.go, or any INSERT path. In particular
// VerificationEventHandler.CreateVerificationEvent inserts a row with a
// caller-chosen status and result for a caller-supplied agentId — a fifth
// writer of these columns that this guard cannot reach. It is pre-existing and
// recorded in the roadmap rather than fixed here; do not read a green run of
// this test as "result/status have exactly three writers".
func TestUpdateVerificationResultHasOnlyPermittedCallSites(t *testing.T) {
	permitted := map[string]bool{
		"writeVerificationResponse": true, // lazy expiry of an elapsed event
		"ApproveVerification":       true, // the human approval path
		"DenyVerification":          true, // the human denial path
	}

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	target := filepath.Join(filepath.Dir(thisFile), "verification_handler.go")

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, target, nil, 0)
	require.NoError(t, err, "parse verification_handler.go")

	var offenders []string
	for _, decl := range file.Decls {
		fn, isFn := decl.(*ast.FuncDecl)
		if !isFn {
			continue
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			call, isCall := n.(*ast.CallExpr)
			if !isCall {
				return true
			}
			sel, isSel := call.Fun.(*ast.SelectorExpr)
			if !isSel || sel.Sel.Name != "UpdateVerificationResult" {
				return true
			}
			if !permitted[fn.Name.Name] {
				offenders = append(offenders, fmt.Sprintf("%s (%s)",
					fn.Name.Name, fset.Position(call.Pos())))
			}
			return true
		})
	}

	assert.Empty(t, offenders,
		"result/status are the authorization-decision channel. Only the approval, denial and "+
			"expiry paths may write them. A new writer here is the Stage 1 defect reopening: an "+
			"agent-reported outcome reaching UpdateVerificationResult sets result='verified', which "+
			"the SDK reads back as approved. If a new writer is genuinely correct, change this "+
			"guard deliberately and say why in the commit that does it.")
}

// TestSDKWriteRoutesAreNotRegisteredOnBareApp guards the wiring from inside the
// handlers package as well as from cmd/server, because the two failure modes are
// different: cmd/server's guard dies if the registration is restructured, this
// one dies if the file moves. Fiber matches in registration order, so a bare
// app.Post registered above the group keeps winning and an "add to the group"
// change that forgets the delete is a no-op that reviews as done.
//
// AIM-03 moved SDK-API registration out of main() into the route table in
// cmd/server/sdk_api_routes.go, where a bare mount is the `Bare: true` field on
// a table entry rather than an `app.Post` call. cmd/server's guard now mounts
// that table and asserts on the running app; this one stays a source scan of
// both files, on purpose — it is the copy that survives cmd/server being
// restructured again.
func TestSDKWriteRoutesAreNotRegisteredOnBareApp(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	serverDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "cmd", "server")

	// Only executable lines count. A comment naming a removed registration — and
	// main.go now carries one, explaining why the JWT mount was deleted — is
	// documentation, not a route. Matching it would make the guard fire on its
	// own explanation and push the next person to delete the explanation.
	executableLines := func(name string) []string {
		src, err := os.ReadFile(filepath.Clean(filepath.Join(serverDir, name)))
		require.NoError(t, err, "locate cmd/server/%s", name)
		var code []string
		for _, line := range strings.Split(string(src), "\n") {
			if trimmed := strings.TrimSpace(line); !strings.HasPrefix(trimmed, "//") {
				code = append(code, line)
			}
		}
		return code
	}

	mainGo := strings.Join(executableLines("main.go"), "\n")
	routeTable := strings.Join(executableLines("sdk_api_routes.go"), "\n")

	for _, forbidden := range []string{
		`app.Post("/api/v1/sdk-api/verifications/:id/result"`,
		`app.Post("/api/v1/sdk-api/verifications/:id/execution-status"`,
		`verifications.Post("/:id/result"`,
	} {
		assert.NotContains(t, mainGo, forbidden,
			"route registration %s must not exist: the bare app mount bypasses the sdkAPI group's "+
				"authentication entirely, and the JWT mount had no ownership check", forbidden)
		assert.NotContains(t, routeTable, forbidden,
			"route registration %s must not exist: the bare app mount bypasses the sdkAPI group's "+
				"authentication entirely, and the JWT mount had no ownership check", forbidden)
	}

	// In the table, "registered on the group" is the ABSENCE of Bare on the
	// entry. Each entry is one line, so the assertion is per line: the route has
	// to be there, and it has to not be bare.
	tableLines := executableLines("sdk_api_routes.go")
	for _, required := range []string{
		`Path: "/verifications/:id/result"`,
		`Path: "/verifications/:id/execution-status"`,
	} {
		var entry string
		for _, line := range tableLines {
			if strings.Contains(line, required) {
				entry = line
				break
			}
		}
		if !assert.NotEmpty(t, entry,
			"no route table entry with %s: the write routes must still be registered somewhere the group authenticates", required) {
			continue
		}
		assert.NotContains(t, entry, "Bare: true",
			"the entry with %s is marked Bare, which registers it above the sdkAPI group where the Ed25519/JWT "+
				"middleware never runs — that is the unauthenticated write reopening:\n  %s",
			required, strings.TrimSpace(entry))
	}
}
