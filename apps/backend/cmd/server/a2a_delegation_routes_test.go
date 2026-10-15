package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// These tests drive /api/v1/a2a/delegations through the server's route table
// (mountAPIRoutes), its authentication and role middleware, the handler and
// the service. Only the two stores are in memory: the delegation store keeps
// the a2a_delegation table's organization scoping and its one-active-
// delegation-per-pair index, and the agent store answers by organization as
// AgentRepository.GetByIDs does. The SQL itself is exercised against
// Postgres in the repository's integration test.

type delegationRouteStore struct {
	mu   sync.Mutex
	rows map[uuid.UUID]*domain.A2ADelegation
}

func (s *delegationRouteStore) Create(_ context.Context, d *domain.A2ADelegation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.rows {
		if r.IsActive && r.OrganizationID == d.OrganizationID &&
			r.DelegatorAgentID == d.DelegatorAgentID && r.DelegateAgentID == d.DelegateAgentID {
			return domain.ErrA2ADelegationActive
		}
	}
	stored := *d
	stored.RequiresConsent = true // the column default
	stored.CreatedAt = time.Now().UTC().Add(time.Duration(len(s.rows)) * time.Millisecond)
	stored.UpdatedAt = stored.CreatedAt
	s.rows[d.ID] = &stored
	*d = stored
	return nil
}

func (s *delegationRouteStore) GetByIDs(_ context.Context, orgID uuid.UUID, ids []uuid.UUID) ([]*domain.A2ADelegation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*domain.A2ADelegation
	for _, id := range ids {
		if r, ok := s.rows[id]; ok && r.OrganizationID == orgID {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (s *delegationRouteStore) ListByOrganization(_ context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.A2ADelegation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []*domain.A2ADelegation{}
	for _, r := range s.rows {
		if r.OrganizationID == orgID {
			cp := *r
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if offset >= len(out) {
		return []*domain.A2ADelegation{}, nil
	}
	out = out[offset:]
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *delegationRouteStore) Revoke(_ context.Context, orgID, id uuid.UUID) (*domain.A2ADelegation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.rows[id]
	if !ok || r.OrganizationID != orgID {
		return nil, domain.ErrA2ADelegationNotFound
	}
	r.IsActive = false
	r.UpdatedAt = time.Now().UTC()
	cp := *r
	return &cp, nil
}

type delegationRouteAgents struct {
	mu     sync.Mutex
	agents map[uuid.UUID]*domain.Agent
}

func (a *delegationRouteAgents) GetByIDs(_ context.Context, orgID uuid.UUID, ids []uuid.UUID) ([]*domain.Agent, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []*domain.Agent
	for _, id := range ids {
		if agent, ok := a.agents[id]; ok && agent.OrganizationID == orgID {
			cp := *agent
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (a *delegationRouteAgents) setTrust(id uuid.UUID, trust float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.agents[id].TrustScore = trust
}

type delegationRouteAudit struct {
	domain.AuditLogRepository
	mu      sync.Mutex
	entries []*domain.AuditLog
}

func (r *delegationRouteAudit) Create(log *domain.AuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, log)
	return nil
}

type delegationRouteFixture struct {
	app    *fiber.App
	jwt    *auth.JWTService
	store  *delegationRouteStore
	agents *delegationRouteAgents
	audit  *delegationRouteAudit
	mock   sqlmock.Sqlmock
	client int

	org, otherOrg     uuid.UUID
	a, b, c, d, other uuid.UUID
}

// newDelegationRouteFixture stands up organization org with agents a (trust
// 0.9), b, c and d, and otherOrg with agent other.
func newDelegationRouteFixture(t *testing.T) *delegationRouteFixture {
	t.Helper()
	withEnvironment(t, "", false)
	t.Setenv("JWT_SECRET", "delegation-route-test-secret-of-at-least-32-chars")
	t.Setenv("TRUSTED_PROXIES", "0.0.0.0")

	f := &delegationRouteFixture{
		jwt:      auth.NewJWTService(),
		store:    &delegationRouteStore{rows: map[uuid.UUID]*domain.A2ADelegation{}},
		agents:   &delegationRouteAgents{agents: map[uuid.UUID]*domain.Agent{}},
		audit:    &delegationRouteAudit{},
		org:      uuid.New(),
		otherOrg: uuid.New(),
		a:        uuid.New(), b: uuid.New(), c: uuid.New(), d: uuid.New(), other: uuid.New(),
	}
	for id, trust := range map[uuid.UUID]float64{f.a: 0.9, f.b: 0.6, f.c: 0.5, f.d: 0.4} {
		f.agents.agents[id] = &domain.Agent{ID: id, OrganizationID: f.org, Status: domain.AgentStatusVerified, TrustScore: trust}
	}
	f.agents.agents[f.other] = &domain.Agent{ID: f.other, OrganizationID: f.otherOrg, Status: domain.AgentStatusVerified, TrustScore: 1}

	// The /a2a group looks a bearer token up as an API key before
	// AuthMiddleware reads it as a JWT; do answers that lookup with no row.
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	f.mock = mock

	svc := application.NewA2ADelegationService(f.store, f.agents)
	h := &Handlers{A2ADelegation: handlers.NewA2ADelegationHandler(svc, application.NewAuditService(f.audit))}
	f.app = fiber.New(fiber.Config{ErrorHandler: customErrorHandler})
	f.app.Use(middleware.RecoveryMiddleware())
	mountAPIRoutes(f.app, h, &Services{Secrets: &application.SecretsService{}}, f.jwt, nil, db)
	return f
}

func (f *delegationRouteFixture) token(t *testing.T, org uuid.UUID, role domain.UserRole) string {
	t.Helper()
	token, err := f.jwt.GenerateAccessToken(uuid.NewString(), org.String(), string(role)+"@example.com", string(role))
	require.NoError(t, err)
	return token
}

// do sends one request, from its own client address, and decodes the JSON body.
func (f *delegationRouteFixture) do(t *testing.T, method, path, token, body string) (int, map[string]interface{}) {
	t.Helper()
	f.client++
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("X-Real-IP", fmt.Sprintf("192.0.2.%d", f.client))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		f.mock.ExpectQuery(`FROM api_keys`).WillReturnError(sql.ErrNoRows)
	}
	resp, err := f.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var decoded map[string]interface{}
	if len(raw) > 0 {
		require.NoError(t, json.Unmarshal(raw, &decoded), "body: %s", raw)
	}
	return resp.StatusCode, decoded
}

// grant creates a delegation as a manager of org and returns its ID.
func (f *delegationRouteFixture) grant(t *testing.T, body string) uuid.UUID {
	t.Helper()
	status, resp := f.do(t, fiber.MethodPost, "/api/v1/a2a/delegations", f.token(t, f.org, domain.RoleManager), body)
	require.Equal(t, fiber.StatusCreated, status, "%v", resp)
	id, err := uuid.Parse(resp["id"].(string))
	require.NoError(t, err)
	return id
}

func (f *delegationRouteFixture) verify(t *testing.T, org uuid.UUID, body string) (int, map[string]interface{}) {
	t.Helper()
	return f.do(t, fiber.MethodPost, "/api/v1/a2a/delegations/verify", f.token(t, org, domain.RoleMember), body)
}

func chainBody(ids ...uuid.UUID) string {
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = `"` + id.String() + `"`
	}
	return `{"delegationIds":[` + strings.Join(quoted, ",") + `]}`
}

func verdictHops(t *testing.T, verdict map[string]interface{}) []map[string]interface{} {
	t.Helper()
	raw, ok := verdict["hops"].([]interface{})
	require.True(t, ok, "verdict has hops: %v", verdict)
	hops := make([]map[string]interface{}, len(raw))
	for i, h := range raw {
		hops[i] = h.(map[string]interface{})
	}
	return hops
}

func hopFigures(t *testing.T, verdict map[string]interface{}, field string) []interface{} {
	t.Helper()
	var out []interface{}
	for _, h := range verdictHops(t, verdict) {
		out = append(out, h[field])
	}
	return out
}

func TestA2ADelegationChainVerificationAppliesEachDelegationsTrustAttenuation(t *testing.T) {
	f := newDelegationRouteFixture(t)
	ab := f.grant(t, fmt.Sprintf(`{"delegatorAgentId":%q,"delegateAgentId":%q,"canDelegateFurther":true}`, f.a, f.b))
	bc := f.grant(t, fmt.Sprintf(`{"delegatorAgentId":%q,"delegateAgentId":%q,"canDelegateFurther":true}`, f.b, f.c))
	cd := f.grant(t, fmt.Sprintf(`{"delegatorAgentId":%q,"delegateAgentId":%q}`, f.c, f.d))

	status, verdict := f.verify(t, f.org, chainBody(ab, bc, cd))
	require.Equal(t, fiber.StatusOK, status, "%v", verdict)
	assert.Equal(t, true, verdict["valid"])
	assert.Equal(t, f.a.String(), verdict["rootAgentId"])
	assert.Equal(t, 0.9, verdict["rootTrustScore"])
	assert.Equal(t, []interface{}{0.72, 0.576, 0.4608}, hopFigures(t, verdict, "effectiveTrust"))
	assert.Equal(t, []interface{}{0.8, 0.8, 0.8}, hopFigures(t, verdict, "trustAttenuationFactor"))
	assert.Equal(t, []interface{}{0.3, 0.3, 0.3}, hopFigures(t, verdict, "trustFloor"))
	assert.Equal(t, []interface{}{nil, nil, nil}, hopFigures(t, verdict, "error"))
	assert.Equal(t, 0.4608, verdict["effectiveTrust"])

	// With the root delegator's trust at 0.5, the third delegation's trust,
	// 0.5 x 0.8 x 0.8 x 0.8, is below the 0.3 floor.
	f.agents.setTrust(f.a, 0.5)
	status, verdict = f.verify(t, f.org, chainBody(ab, bc, cd))
	require.Equal(t, fiber.StatusOK, status, "%v", verdict)
	assert.Equal(t, false, verdict["valid"])
	assert.Equal(t, []interface{}{0.4, 0.32, 0.256}, hopFigures(t, verdict, "effectiveTrust"))
	assert.Equal(t, []interface{}{nil, nil, "effective trust 0.256 is below the trust floor 0.3"}, hopFigures(t, verdict, "error"))
}

func TestA2ADelegationChainVerificationReadsTheFactorAndFloorStoredOnEachDelegation(t *testing.T) {
	f := newDelegationRouteFixture(t)
	ab := f.grant(t, fmt.Sprintf(`{"delegatorAgentId":%q,"delegateAgentId":%q,"canDelegateFurther":true,"trustAttenuationFactor":1,"minDelegatedTrustScore":0.5}`, f.a, f.b))
	bc := f.grant(t, fmt.Sprintf(`{"delegatorAgentId":%q,"delegateAgentId":%q,"trustAttenuationFactor":0.5,"minDelegatedTrustScore":0}`, f.b, f.c))

	status, stored := f.do(t, fiber.MethodGet, "/api/v1/a2a/delegations/"+bc.String(), f.token(t, f.org, domain.RoleViewer), "")
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, 0.5, stored["trustAttenuationFactor"])
	assert.Equal(t, 0.0, stored["minDelegatedTrustScore"])

	status, verdict := f.verify(t, f.org, chainBody(ab))
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, true, verdict["valid"])
	assert.Equal(t, []interface{}{0.9}, hopFigures(t, verdict, "effectiveTrust"))

	// The second delegation's own floor is 0, and the first one's 0.5 still
	// holds it: 0.9 x 0.5 is below 0.5.
	status, verdict = f.verify(t, f.org, chainBody(ab, bc))
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, false, verdict["valid"])
	assert.Equal(t, []interface{}{0.9, 0.45}, hopFigures(t, verdict, "effectiveTrust"))
	assert.Equal(t, []interface{}{0.5, 0.5}, hopFigures(t, verdict, "trustFloor"))
	assert.Equal(t, []interface{}{nil, "effective trust 0.45 is below the trust floor 0.5"}, hopFigures(t, verdict, "error"))
}

func TestA2ADelegationCreateTakesTheColumnDefaultsAndIsAudited(t *testing.T) {
	f := newDelegationRouteFixture(t)
	status, d := f.do(t, fiber.MethodPost, "/api/v1/a2a/delegations", f.token(t, f.org, domain.RoleAdmin),
		fmt.Sprintf(`{"delegatorAgentId":%q,"delegateAgentId":%q,"skillIds":["search"]}`, f.a, f.b))
	require.Equal(t, fiber.StatusCreated, status, "%v", d)
	assert.Equal(t, f.org.String(), d["organizationId"])
	assert.Equal(t, 0.8, d["trustAttenuationFactor"])
	assert.Equal(t, 0.3, d["minDelegatedTrustScore"])
	assert.Equal(t, 3.0, d["maxChainDepth"])
	assert.Equal(t, false, d["canDelegateFurther"])
	assert.Equal(t, true, d["isActive"])
	assert.Equal(t, []interface{}{"search"}, d["skillIds"])

	require.Len(t, f.audit.entries, 1)
	entry := f.audit.entries[0]
	assert.Equal(t, domain.AuditActionCreate, entry.Action)
	assert.Equal(t, "a2a_delegation", entry.ResourceType)
	assert.Equal(t, d["id"], entry.ResourceID.String())
	assert.Equal(t, f.org, entry.OrganizationID)

	status, body := f.do(t, fiber.MethodPost, "/api/v1/a2a/delegations", f.token(t, f.org, domain.RoleAdmin),
		fmt.Sprintf(`{"delegatorAgentId":%q,"delegateAgentId":%q}`, f.a, f.b))
	assert.Equal(t, fiber.StatusConflict, status)
	assert.Equal(t, domain.ErrA2ADelegationActive.Error(), body["error"])
}

func TestA2ADelegationRevokedDelegationFailsTheChain(t *testing.T) {
	f := newDelegationRouteFixture(t)
	ab := f.grant(t, fmt.Sprintf(`{"delegatorAgentId":%q,"delegateAgentId":%q,"canDelegateFurther":true}`, f.a, f.b))
	bc := f.grant(t, fmt.Sprintf(`{"delegatorAgentId":%q,"delegateAgentId":%q}`, f.b, f.c))

	status, revoked := f.do(t, fiber.MethodPost, "/api/v1/a2a/delegations/"+bc.String()+"/revoke", f.token(t, f.org, domain.RoleManager), "")
	require.Equal(t, fiber.StatusOK, status, "%v", revoked)
	assert.Equal(t, false, revoked["isActive"])
	require.Len(t, f.audit.entries, 3)
	assert.Equal(t, domain.AuditActionRevoke, f.audit.entries[2].Action)
	assert.Equal(t, bc, f.audit.entries[2].ResourceID)

	status, verdict := f.verify(t, f.org, chainBody(ab, bc))
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, false, verdict["valid"])
	assert.Equal(t, []interface{}{nil, "the delegation is revoked"}, hopFigures(t, verdict, "error"))

	// A revoked pair can be granted again.
	f.grant(t, fmt.Sprintf(`{"delegatorAgentId":%q,"delegateAgentId":%q}`, f.b, f.c))
}

func TestA2ADelegationWritesNeedAManager(t *testing.T) {
	f := newDelegationRouteFixture(t)
	ab := f.grant(t, fmt.Sprintf(`{"delegatorAgentId":%q,"delegateAgentId":%q}`, f.a, f.b))
	for _, role := range []domain.UserRole{domain.RoleMember, domain.RoleViewer} {
		status, body := f.do(t, fiber.MethodPost, "/api/v1/a2a/delegations", f.token(t, f.org, role),
			fmt.Sprintf(`{"delegatorAgentId":%q,"delegateAgentId":%q}`, f.b, f.c))
		assert.Equal(t, fiber.StatusForbidden, status, "create as %s", role)
		assert.Equal(t, "Manager or admin access required", body["error"])

		status, _ = f.do(t, fiber.MethodPost, "/api/v1/a2a/delegations/"+ab.String()+"/revoke", f.token(t, f.org, role), "")
		assert.Equal(t, fiber.StatusForbidden, status, "revoke as %s", role)
	}
	status, _ := f.do(t, fiber.MethodPost, "/api/v1/a2a/delegations/verify", "", chainBody(ab))
	assert.Equal(t, fiber.StatusUnauthorized, status, "verify without credentials")

	assert.Len(t, f.store.rows, 1)
	assert.True(t, f.store.rows[ab].IsActive)
}

func TestA2ADelegationsStayInsideTheOrganization(t *testing.T) {
	f := newDelegationRouteFixture(t)
	ab := f.grant(t, fmt.Sprintf(`{"delegatorAgentId":%q,"delegateAgentId":%q}`, f.a, f.b))
	otherManager := f.token(t, f.otherOrg, domain.RoleManager)

	status, body := f.do(t, fiber.MethodPost, "/api/v1/a2a/delegations", otherManager,
		fmt.Sprintf(`{"delegatorAgentId":%q,"delegateAgentId":%q}`, f.other, f.a))
	assert.Equal(t, fiber.StatusNotFound, status)
	assert.Equal(t, "Agent not found", body["error"])

	status, body = f.verify(t, f.otherOrg, chainBody(ab))
	assert.Equal(t, fiber.StatusNotFound, status)
	assert.Equal(t, map[string]interface{}{"error": "Delegation not found", "delegationId": ab.String()}, body)

	status, body = f.do(t, fiber.MethodGet, "/api/v1/a2a/delegations/"+ab.String(), otherManager, "")
	assert.Equal(t, fiber.StatusNotFound, status)
	assert.Equal(t, map[string]interface{}{"error": "not found"}, body)

	audited := len(f.audit.entries)
	status, body = f.do(t, fiber.MethodPost, "/api/v1/a2a/delegations/"+ab.String()+"/revoke", otherManager, "")
	assert.Equal(t, fiber.StatusNotFound, status)
	assert.Equal(t, map[string]interface{}{"error": "not found"}, body)
	assert.True(t, f.store.rows[ab].IsActive, "another organization's revoke changed nothing")
	assert.Len(t, f.audit.entries, audited, "another organization's revoke recorded nothing")

	status, body = f.do(t, fiber.MethodGet, "/api/v1/a2a/delegations/"+uuid.NewString(), f.token(t, f.org, domain.RoleViewer), "")
	assert.Equal(t, fiber.StatusNotFound, status)
	assert.Equal(t, map[string]interface{}{"error": "not found"}, body)

	status, body = f.do(t, fiber.MethodGet, "/api/v1/a2a/delegations", otherManager, "")
	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, []interface{}{}, body["delegations"])

	status, body = f.do(t, fiber.MethodGet, "/api/v1/a2a/delegations?limit=10", f.token(t, f.org, domain.RoleViewer), "")
	assert.Equal(t, fiber.StatusOK, status)
	list := body["delegations"].([]interface{})
	require.Len(t, list, 1)
	assert.Equal(t, ab.String(), list[0].(map[string]interface{})["id"])
	assert.Equal(t, 10.0, body["limit"])
}

func TestA2ADelegationRequestsTheServiceRefuses(t *testing.T) {
	f := newDelegationRouteFixture(t)
	ab := f.grant(t, fmt.Sprintf(`{"delegatorAgentId":%q,"delegateAgentId":%q}`, f.a, f.b))
	manager := f.token(t, f.org, domain.RoleManager)
	pair := fmt.Sprintf(`"delegatorAgentId":%q,"delegateAgentId":%q`, f.b, f.c)
	skills := make([]string, application.MaxA2ADelegationSkills+1)
	for i := range skills {
		skills[i] = fmt.Sprintf("%q", fmt.Sprintf("skill-%d", i))
	}
	tooManySkills := `{` + pair + `,"skillIds":[` + strings.Join(skills, ",") + `]}`

	for body, want := range map[string]string{
		tooManySkills: "skillIds may name at most 100 skills",
		`{` + pair + `,"trustAttenuationFactor":1.5}`:                         "trustAttenuationFactor must be a number from 0 to 1",
		`{` + pair + `,"minDelegatedTrustScore":-0.1}`:                        "minDelegatedTrustScore must be a number from 0 to 1",
		`{` + pair + `,"maxChainDepth":0}`:                                    "maxChainDepth must be from 1 to 10",
		`{` + pair + `,"maxChainDepth":11}`:                                   "maxChainDepth must be from 1 to 10",
		`{` + pair + `,"skillIds":[" "]}`:                                     "skillIds must not contain an empty skill ID",
		`{` + pair + `,"validUntil":"2020-01-01T00:00:00Z"}`:                  "validUntil must be in the future",
		fmt.Sprintf(`{"delegatorAgentId":%q,"delegateAgentId":%q}`, f.a, f.a): "an agent cannot delegate to itself",
		fmt.Sprintf(`{"delegatorAgentId":%q}`, f.a):                           "delegatorAgentId and delegateAgentId are required",
		`{"delegatorAgentId":"not-a-uuid"}`:                                   "Invalid request body",
	} {
		status, resp := f.do(t, fiber.MethodPost, "/api/v1/a2a/delegations", manager, body)
		assert.Equal(t, fiber.StatusBadRequest, status, body)
		assert.Equal(t, want, resp["error"], body)
	}
	assert.Len(t, f.store.rows, 1, "no refused request stored a delegation")

	eleven := make([]uuid.UUID, 11)
	for i := range eleven {
		eleven[i] = uuid.New()
	}
	for body, want := range map[string]string{
		`{"delegationIds":[]}`:   "delegationIds must name from 1 to 10 delegations",
		chainBody(eleven...):     "delegationIds must name from 1 to 10 delegations",
		chainBody(ab, ab):        "delegationIds names delegation " + ab.String() + " more than once",
		`{"delegationIds":"ab"}`: "Invalid request body",
	} {
		status, resp := f.verify(t, f.org, body)
		assert.Equal(t, fiber.StatusBadRequest, status, body)
		assert.Equal(t, want, resp["error"], body)
	}

	status, resp := f.do(t, fiber.MethodGet, "/api/v1/a2a/delegations/not-a-uuid", manager, "")
	assert.Equal(t, fiber.StatusBadRequest, status)
	assert.Equal(t, "Invalid delegation ID", resp["error"])
}
