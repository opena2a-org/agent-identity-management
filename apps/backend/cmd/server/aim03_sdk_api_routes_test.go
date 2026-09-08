package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AIM-03 — the backend serves the path the SDKs actually call.
//
// All three shipped clients POST /api/v1/sdk-api/agents/{id}/isolation-attestation
// (AIMClient.ts:729, client.py:2427, AIMClient.java:1698). The backend
// registered only /agents/:id/isolation, so every attestation any agent ever
// sent returned 404 and isolation_attestations stayed near-empty — with a green
// test suite, because the integration tests hand-registered the handler at the
// server's own path and so agreed with the server about a path no client used.
//
// Per the recorded architecture decision of 2026-08-29 the SDK/spec path is canonical: the
// backend gains it on the existing handler and keeps /agents/:id/isolation as a
// deprecated alias (BD6 forbids removing a published path).
//
// These tests take their paths from two places only: the route table the server
// mounts (sdkAPIRouteTable, via registerSDKAPIRoutes) and the SDK sources
// themselves. Nothing here spells out a path the way the old tests did.

// aim03TSSDKPayload is the body AIMClient.attestIsolation builds — four posture
// strings, no score, no verification (AIMClient.ts:726-731).
const aim03TSSDKPayload = `{"sandbox":"firecracker","network":"airgap","filesystem":"readonly","process":"full"}`

// aim03CountingRepo wraps the production repository so a test can assert "no row
// was written" directly instead of inferring it from an absent SQL call. Create
// still runs the real INSERT, so the sqlmock expectation is what proves the row
// landed in isolation_attestations.
type aim03CountingRepo struct {
	*repository.IsolationAttestationRepository
	creates int
}

func (r *aim03CountingRepo) Create(attestation *domain.IsolationAttestation) error {
	r.creates++
	return r.IsolationAttestationRepository.Create(attestation)
}

// aim03AgentStub resolves the one agent these tests use. The embedded interface
// supplies the rest of AgentServicer: a handler that reached any other method
// would be doing something this test has no business passing.
type aim03AgentStub struct {
	handlers.AgentServicer
	agent *domain.Agent
}

func (s *aim03AgentStub) GetAgent(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
	if s.agent == nil || s.agent.ID != id {
		return nil, fmt.Errorf("agent %s not found", id)
	}
	return s.agent, nil
}

// aim03AuditStub records which audit path the handler took.
type aim03AuditStub struct {
	handlers.AuditServicer
	agentActions int
	userActions  int
}

func (s *aim03AuditStub) LogAgentAction(_ context.Context, _, _ uuid.UUID, _ domain.AuditAction, _ string, _ uuid.UUID, _, _ string, _ map[string]interface{}) error {
	s.agentActions++
	return nil
}

func (s *aim03AuditStub) LogAction(_ context.Context, _, _ uuid.UUID, _ domain.AuditAction, _ string, _ uuid.UUID, _, _ string, _ map[string]interface{}) error {
	s.userActions++
	return nil
}

// sdkAPITestHandlers fills every field of sdkAPIHandlers with h.
//
// It is how a test mounts the REAL route table without real services: what a
// test may substitute is the handler, never the path. TestAIM03_SDKAPIHandlers
// SetIsComplete keeps it honest as the table grows.
func sdkAPITestHandlers(h fiber.Handler) sdkAPIHandlers {
	return sdkAPIHandlers{
		CreateVerification:          h,
		GetVerificationSDK:          h,
		SubmitVerificationResult:    h,
		UpdateExecutionStatus:       h,
		GetAgentByIdentifier:        h,
		GrantCapability:             h,
		RegisterCapability:          h,
		ListAgentCapabilityRequests: h,
		CreateCapabilityRequest:     h,
		CreateMCPServer:             h,
		ListMCPServers:              h,
		GetMCPServerByName:          h,
		RecordMCPConnection:         h,
		RecordMCPUsageReport:        h,
		ReportDetection:             h,
		Heartbeat:                   h,
		SubmitIsolationAttestation:  h,
	}
}

// aim03Fixture is the production ingest path end to end: the registered route
// table -> handlers.TrustScoreHandler -> application.TrustCalculator ->
// repository.IsolationAttestationRepository -> INSERT INTO
// isolation_attestations. Only the agent lookup, the audit sink and the database
// driver are stand-ins; the paths, the handler, the scoring and the SQL are real.
type aim03Fixture struct {
	app     *fiber.App
	table   []sdkAPIRoute
	mock    sqlmock.Sqlmock
	repo    *aim03CountingRepo
	audit   *aim03AuditStub
	agentID uuid.UUID
}

func newAIM03Fixture(t *testing.T) *aim03Fixture {
	t.Helper()

	orgID, agentID := uuid.New(), uuid.New()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &aim03CountingRepo{IsolationAttestationRepository: repository.NewIsolationAttestationRepository(db)}
	calculator := &application.TrustCalculator{}
	calculator.SetIsolationRepo(repo)

	audit := &aim03AuditStub{}
	handler := handlers.NewTrustScoreHandlerWithInterfaces(
		calculator,
		&aim03AgentStub{agent: &domain.Agent{ID: agentID, OrganizationID: orgID, Name: "aim03-agent"}},
		audit,
	)

	// Every other route gets a handler that says so out loud: if a test lands on
	// one, the assertion failure names the route rather than a status code.
	set := sdkAPITestHandlers(func(c fiber.Ctx) error {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "not wired in this test"})
	})
	set.SubmitIsolationAttestation = handler.SubmitIsolationAttestation

	app := fiber.New()
	table := registerSDKAPIRoutes(app, sdkAPIDeps{
		// Stands in for PQCAgentMiddleware: on the SDK path the caller IS the
		// agent, so organization_id and agent_id are set and user_id is not.
		GroupMiddleware: []fiber.Handler{func(c fiber.Ctx) error {
			c.Locals("organization_id", orgID)
			c.Locals("agent_id", agentID)
			return c.Next()
		}},
		Handlers: set,
	})

	return &aim03Fixture{app: app, table: table, mock: mock, repo: repo, audit: audit, agentID: agentID}
}

// post sends the SDK's request: POST, application/json, posture body.
func (f *aim03Fixture) post(t *testing.T, path, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.app.Test(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// expectAttestationInsert expects one INSERT INTO isolation_attestations for the
// posture in aim03TSSDKPayload. The id, score and timestamps are generated
// server-side; the agent id and the four posture columns are what the request
// determines, so those are matched exactly.
func (f *aim03Fixture) expectAttestationInsert() {
	f.mock.ExpectExec(regexp.QuoteMeta("INSERT INTO isolation_attestations")).
		WithArgs(
			sqlmock.AnyArg(), f.agentID,
			"firecracker", "airgap", "readonly", "full",
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
}

// aim03PathFor returns the registered path for a table entry, with the :id
// parameter filled in — so a request is addressed by the table, never by a
// string the test chose.
func (f *aim03Fixture) aim03PathFor(t *testing.T, relPath string) string {
	t.Helper()
	for _, route := range f.table {
		if route.Path == relPath && route.Method == http.MethodPost {
			return strings.ReplaceAll(route.FullPath(), ":id", f.agentID.String())
		}
	}
	t.Fatalf("no POST %s in the registered route table", relPath)
	return ""
}

// TestAIM03_CanonicalIsolationAttestationPathLandsARow drives the exact request
// the shipped TypeScript SDK sends.
func TestAIM03_CanonicalIsolationAttestationPathLandsARow(t *testing.T) {
	t.Run("AIM-03.AC1 the path the shipped SDK posts writes a row in isolation_attestations", func(t *testing.T) {
		f := newAIM03Fixture(t)
		f.expectAttestationInsert()

		// The path comes from the SDK source, not from this test: whatever
		// AIMClient.attestIsolation emits is what gets called.
		paths := aim03SDKIsolationPaths(t)
		require.Contains(t, paths, "typescript", "no isolation-attestation call site found in the TypeScript SDK")
		path := strings.ReplaceAll(paths["typescript"], ":id", f.agentID.String())

		resp := f.post(t, path, aim03TSSDKPayload)

		require.Equal(t, fiber.StatusCreated, resp.StatusCode,
			"POST %s must be served; on origin/main ac01cfd this was a 404 and every SDK attestation was silently dropped", path)
		assert.Equal(t, 1, f.repo.creates, "exactly one attestation must be persisted")
		assert.NoError(t, f.mock.ExpectationsWereMet(),
			"the request must reach INSERT INTO isolation_attestations — a 201 with no row is the same outage in a different disguise")
		assert.Equal(t, 1, f.audit.agentActions, "an agent self-report is audited on the agent path")
		assert.Zero(t, f.audit.userActions, "there is no user on the SDK path, so nothing may be attributed to one")

		var result map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
		assert.Equal(t, f.agentID.String(), result["agentId"],
			"the row is scoped to the agent in the path")
		assert.Equal(t, 1.0, result["score"],
			"the score is computed server-side from the posture the SDK reported")
	})
}

// TestAIM03_RouteTableCoversEverySDKPath is the parity guard: the registered
// table has to contain every path the shipped clients emit for this feature.
func TestAIM03_RouteTableCoversEverySDKPath(t *testing.T) {
	t.Run("AIM-03.AC2 every SDK isolation-attestation path exists in the registered route table", func(t *testing.T) {
		paths := aim03SDKIsolationPaths(t)
		require.Len(t, paths, len(aim03SDKCallSites),
			"every shipped client must contribute a call site; a missing one means this guard stopped watching an SDK")

		f := newAIM03Fixture(t)

		registered := map[string]bool{}
		for _, route := range f.table {
			registered[route.Method+" "+route.FullPath()] = true
		}

		for language, sdkPath := range paths {
			assert.True(t, registered["POST "+sdkPath],
				"%s SDK posts %s, which the backend does not register. The registered table is:\n%s",
				language, sdkPath, aim03FormatTable(f.table))

			// Registration is not service: ask the mounted app.
			f.expectAttestationInsert()
			resp := f.post(t, strings.ReplaceAll(sdkPath, ":id", f.agentID.String()), aim03TSSDKPayload)
			assert.Equal(t, fiber.StatusCreated, resp.StatusCode, "%s SDK path %s is registered but not served", language, sdkPath)
		}
	})

	t.Run("AIM-03.AC2 the route table is the only place a path is written down", func(t *testing.T) {
		// registerSDKAPIRoutes returns what it registered, which is what makes
		// the parity assertion above possible at all: a test that built its own
		// list of paths would be testing its own list.
		f := newAIM03Fixture(t)
		require.NotEmpty(t, f.table)
		for _, route := range f.table {
			assert.True(t, strings.HasPrefix(route.FullPath(), sdkAPIBasePath),
				"%s does not hang off the SDK-API base path", route.FullPath())
			assert.NotNil(t, route.Handler, "%s %s has no handler", route.Method, route.FullPath())
			assert.NotEmpty(t, route.Note,
				"%s %s has no Note. The table is where a reader learns why a route is shaped the way it is — "+
					"an unexplained entry is how a bare mount or a duplicate path gets waved through review.",
				route.Method, route.FullPath())
		}
	})
}

// TestAIM03_SDKAPIHandlersSetIsComplete keeps the test mount honest: if a field
// is added to sdkAPIHandlers and not to sdkAPITestHandlers, the new route mounts
// a nil handler and every guard that mounts the table starts lying.
func TestAIM03_SDKAPIHandlersSetIsComplete(t *testing.T) {
	t.Run("AIM-03.AC2 sdkAPITestHandlers fills every handler slot", func(t *testing.T) {
		set := reflect.ValueOf(sdkAPITestHandlers(func(c fiber.Ctx) error { return nil }))
		for i := 0; i < set.NumField(); i++ {
			assert.False(t, set.Field(i).IsNil(),
				"sdkAPIHandlers.%s is not set by sdkAPITestHandlers — add it, or the table mounts a nil handler",
				set.Type().Field(i).Name)
		}
	})
}

// TestAIM03_UnregisteredPathIs404 is the non-vacuity proof for the two tests
// above.
func TestAIM03_UnregisteredPathIs404(t *testing.T) {
	t.Run("AIM-03.AC3 a path that is not in the table gets 404 and writes nothing", func(t *testing.T) {
		f := newAIM03Fixture(t)
		// No ExpectExec: an INSERT reaching the driver here would fail the
		// unexpected-call check as well as the counter below.

		// Plural — one character away from the canonical path, and exactly the
		// kind of near-miss that produced this defect in the first place.
		canonical := aim03SDKIsolationPaths(t)["typescript"]
		plural := strings.ReplaceAll(canonical, ":id", f.agentID.String()) + "s"

		resp := f.post(t, plural, aim03TSSDKPayload)

		require.Equal(t, fiber.StatusNotFound, resp.StatusCode,
			"POST %s is not registered, so the mounted table must 404 — if this passes as 201 the tests above prove nothing", plural)
		assert.Equal(t, 0, f.repo.creates, "an unrouted POST must not reach the repository")
		assert.NoError(t, f.mock.ExpectationsWereMet(), "an unrouted POST must not reach the database")

		// Same mount, canonical path, same body: the 404 above is about the
		// path and nothing else.
		f.expectAttestationInsert()
		served := f.post(t, strings.ReplaceAll(canonical, ":id", f.agentID.String()), aim03TSSDKPayload)
		require.Equal(t, fiber.StatusCreated, served.StatusCode)
		assert.Equal(t, 1, f.repo.creates)
	})
}

// TestAIM03_DeprecatedAliasStillServed holds the ruling's other half: the alias
// is deprecated, not removed (Binding Decision 6).
func TestAIM03_DeprecatedAliasStillServed(t *testing.T) {
	t.Run("AIM-03.AC4 the deprecated /isolation alias still succeeds on the same handler", func(t *testing.T) {
		f := newAIM03Fixture(t)
		f.expectAttestationInsert()

		alias := f.aim03PathFor(t, sdkAPIIsolationAliasPath)
		resp := f.post(t, alias, aim03TSSDKPayload)

		require.Equal(t, fiber.StatusCreated, resp.StatusCode,
			"the alias must keep returning the handler's success status: BD6 forbids removing a published path")
		assert.Equal(t, 1, f.repo.creates, "the alias reaches the same ingest, so it lands the same row")
		assert.NoError(t, f.mock.ExpectationsWereMet())
	})

	t.Run("AIM-03.AC4 the alias is marked deprecated and the canonical path is not", func(t *testing.T) {
		f := newAIM03Fixture(t)

		var canonical, alias *sdkAPIRoute
		for i := range f.table {
			switch f.table[i].Path {
			case sdkAPIIsolationAttestationPath:
				canonical = &f.table[i]
			case sdkAPIIsolationAliasPath:
				alias = &f.table[i]
			}
		}
		require.NotNil(t, canonical, "the canonical isolation-attestation route is missing from the table")
		require.NotNil(t, alias, "the deprecated alias was removed; BD6 forbids that")

		assert.True(t, alias.Deprecated, "the alias must be marked deprecated so nothing advertises it")
		assert.False(t, canonical.Deprecated, "the canonical path is the contract, not a legacy shim")
		assert.NotNil(t, canonical.Handler)
		assert.NotNil(t, alias.Handler)
	})

	t.Run("AIM-03.AC4 the API documentation names the canonical path and marks the alias deprecated", func(t *testing.T) {
		doc := aim03ReadRepoFile(t, filepath.Join("apps", "web", "lib", "api-documentation.ts"))

		canonicalEntry := fmt.Sprintf("path: %q,", sdkAPIBasePath+sdkAPIIsolationAttestationPath)
		aliasEntry := fmt.Sprintf("path: %q,", sdkAPIBasePath+sdkAPIIsolationAliasPath)

		require.Contains(t, doc, canonicalEntry,
			"the developer docs must document the canonical path an SDK actually calls")
		require.Contains(t, doc, aliasEntry,
			"the alias is still served, so it stays documented — as deprecated")

		lines := strings.Split(doc, "\n")
		aliasLine := -1
		for i, line := range lines {
			if strings.Contains(line, aliasEntry) {
				aliasLine = i
				break
			}
		}
		require.GreaterOrEqual(t, aliasLine, 0)

		var marked bool
		for i := aliasLine; i < len(lines) && i < aliasLine+12; i++ {
			if strings.Contains(lines[i], "deprecated: true") {
				marked = true
				break
			}
		}
		assert.True(t, marked,
			"the %s entry must carry `deprecated: true`; an alias documented as an equal alternative is how a "+
				"deprecated path acquires new callers", sdkAPIBasePath+sdkAPIIsolationAliasPath)
	})
}

// ---------------------------------------------------------------------------
// Reading the paths out of the shipped SDKs
// ---------------------------------------------------------------------------

// aim03SDKCallSites are the three shipped clients that carry this feature's
// path. Java is unpublished but its source is in this tree and emits the path,
// so it is held to the same parity.
var aim03SDKCallSites = []struct{ language, file string }{
	{"typescript", "sdk/typescript/src/client/AIMClient.ts"},
	{"python", "sdk/python/aim_sdk/client.py"},
	{"java", "sdk/java/src/main/java/org/opena2a/aim/client/AIMClient.java"},
}

// aim03SDKAgentPathRe matches an /api/v1/sdk-api/agents/<id>/... path in SDK
// source, in each language's interpolation form:
//
//	TypeScript  `/api/v1/sdk-api/agents/${this.credentials.agentId}/isolation-attestation`
//	Python      f"/api/v1/sdk-api/agents/{self.agent_id}/isolation-attestation"
//	Java        "/api/v1/sdk-api/agents/" + agentId + "/isolation-attestation"
//
// The capture is the tail after the agent id, which is what the backend's
// route pattern has to match.
var aim03SDKAgentPathRe = regexp.MustCompile(
	`/api/v1/sdk-api/agents/(?:\$\{[^}]*\}|\{[^}]*\}|"\s*\+\s*[A-Za-z_][A-Za-z0-9_.()]*\s*\+\s*")(/[A-Za-z0-9_/.\-]*)`)

// aim03SDKIsolationPaths returns, per SDK, the isolation-attestation path that
// client emits, normalised to the backend's parameter syntax.
//
// Scope is this feature deliberately: the repo-wide path parity guard is the
// registry-side unit of the same ruling, not this one.
func aim03SDKIsolationPaths(t *testing.T) map[string]string {
	t.Helper()

	paths := map[string]string{}
	for _, site := range aim03SDKCallSites {
		src := aim03ReadRepoFile(t, filepath.FromSlash(site.file))

		var found []string
		for _, match := range aim03SDKAgentPathRe.FindAllStringSubmatch(src, -1) {
			tail := match[1]
			if !strings.Contains(tail, "isolation") {
				continue
			}
			found = append(found, sdkAPIBasePath+"/agents/:id"+tail)
		}
		require.NotEmpty(t, found,
			"no isolation-attestation call site found in %s. Either the SDK stopped emitting the path (then this guard "+
				"must be repointed, not deleted) or the file moved — an unreadable call site is how the mismatch hid.", site.file)

		for _, path := range found[1:] {
			require.Equal(t, found[0], path, "%s emits more than one isolation path: %v", site.file, found)
		}
		paths[site.language] = found[0]
	}
	return paths
}

// aim03FormatTable renders the registered table for a failure message: when
// parity breaks, the useful thing to see is what the server does serve.
func aim03FormatTable(table []sdkAPIRoute) string {
	var b strings.Builder
	for _, route := range table {
		fmt.Fprintf(&b, "  %-4s %s\n", route.Method, route.FullPath())
	}
	return b.String()
}

// aim03ReadRepoFile reads a repo-relative file, resolving the repository root
// from this test file's own location so the read does not depend on the
// directory `go test` was invoked from.
func aim03ReadRepoFile(t *testing.T, rel string) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must resolve this test file's path")

	dir := filepath.Dir(thisFile)
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "apps")); err == nil {
				break
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate repository root above %s", thisFile)
		}
		dir = parent
	}

	content, err := os.ReadFile(filepath.Join(dir, rel)) //nolint:gosec // G304: path derived from runtime.Caller, not user input
	require.NoError(t, err, "read %s", rel)
	return string(content)
}
