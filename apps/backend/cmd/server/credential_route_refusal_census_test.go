package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
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
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// credentialCensusAudit is an audit log repository that keeps the entries it
// is given.
type credentialCensusAudit struct {
	domain.AuditLogRepository
	rows []*domain.AuditLog
}

func (a *credentialCensusAudit) Create(entry *domain.AuditLog) error {
	a.rows = append(a.rows, entry)
	return nil
}

// credentialCensusStore is a revocation store that keeps the keys written to
// it.
type credentialCensusStore struct{ keys map[string]bool }

func (s *credentialCensusStore) Exists(_ context.Context, key string) (bool, error) {
	return s.keys[key], nil
}

func (s *credentialCensusStore) Set(_ context.Context, key string, _ interface{}, _ time.Duration) error {
	s.keys[key] = true
	return nil
}

// routeKey names a route as "METHOD /path", with the trailing slash a route
// registered as a group's "/" carries trimmed, as the router trims it.
func routeKey(method, path string) string {
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	return method + " " + path
}

// TestSessionMiddlewareRecordsARevokedSessionAtExactlyTheCredentialRoutes
// probes every GET, POST, PUT, PATCH and DELETE route the server mounts under
// /api/v1, through mountAPIRoutes with the real middleware and empty
// handlers, with the access token of one sign-in whose session was revoked,
// each probe as a fresh client. On a route behind the session middleware the
// middleware refuses the token before the handler runs. A probe it records
// adds one credential_mint_refused row to the audit log mountAPIRoutes
// attaches and writes one SECURITY line, under the route's label with the
// cause sessionRevoked. The routes recorded must be middleware.CredentialRoutes
// under the labels the table gives them, no more and no fewer.
func TestSessionMiddlewareRecordsARevokedSessionAtExactlyTheCredentialRoutes(t *testing.T) {
	// The production limits: ENVIRONMENT unset. app.Test connects from
	// 0.0.0.0; trusting it as a proxy gives each probe its own client address
	// (X-Real-IP), so no limiter refuses a probe before the session check.
	withEnvironment(t, "", false)
	t.Setenv("JWT_SECRET", "FAKE-credential-route-census-jwt-secret")
	t.Setenv("TRUSTED_PROXIES", "0.0.0.0")
	jwtService := auth.NewJWTService()
	jwtService.SetRevoker(auth.NewTokenRevoker(&credentialCensusStore{keys: map[string]bool{}}, false))
	audit := &credentialCensusAudit{}

	var logged bytes.Buffer
	prevLog := log.Writer()
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(prevLog) })

	app := fiber.New()
	app.Use(func(c fiber.Ctx) (err error) {
		defer func() {
			if recover() != nil {
				err = c.SendStatus(fiber.StatusInternalServerError)
			}
		}()
		return c.Next()
	})
	// The /agents group looks a bearer up as an API key before the session
	// check. A database that answers no query (as an API key lookup that
	// finds no row is answered) lets the bearer through to the session
	// middleware, as the server's database does for a session token.
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	services := &Services{Secrets: &application.SecretsService{}, Audit: application.NewAuditService(audit)}
	mountAPIRoutes(app, &Handlers{}, services, jwtService, nil, db)

	userID, orgID := uuid.New(), uuid.New()
	access, refresh, err := jwtService.GenerateTokenPair(userID.String(), orgID.String(), "census@example.com", "admin")
	require.NoError(t, err)
	accessClaims, err := jwtService.ValidateToken(access)
	require.NoError(t, err)
	refreshClaims, err := jwtService.ValidateToken(refresh)
	require.NoError(t, err)
	written, err := jwtService.RevokeFamily(context.Background(), refreshClaims)
	require.NoError(t, err)
	require.True(t, written, "the session's family key is written")

	recorded := map[string]string{}
	probed := map[string]bool{}
	client := 0
	for _, route := range app.GetRoutes(true) {
		switch route.Method {
		case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		default:
			continue
		}
		key := routeKey(route.Method, route.Path)
		if probed[key] || !strings.HasPrefix(route.Path, "/api/v1/") {
			continue
		}
		probed[key] = true

		client++
		before := len(audit.rows)
		logged.Reset()
		req := httptest.NewRequest(route.Method, concreteRoutePath(route.Path), nil)
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("X-Real-IP", fmt.Sprintf("10.%d.%d.%d", client>>16&255, client>>8&255, client&255))
		req.Header.Set("User-Agent", "credential-route-census/1")
		resp, err := app.Test(req)
		require.NoError(t, err, key)
		_ = resp.Body.Close()

		rows := audit.rows[before:]
		lines := strings.Count(logged.String(), "SECURITY credential_mint_refused ")
		if len(rows) == 0 {
			assert.Zero(t, lines, "%s: a SECURITY line with no row", key)
			continue
		}
		require.Len(t, rows, 1, "%s: one row for the refused request", key)
		row := rows[0]
		label, _ := row.Metadata["route"].(string)
		recorded[key] = label
		assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode, key)
		assert.Equal(t, domain.AuditActionCredentialMintRefused, row.Action, key)
		assert.Equal(t, orgID, row.OrganizationID, key)
		if assert.NotNil(t, row.UserID, key) {
			assert.Equal(t, userID, *row.UserID, key)
		}
		assert.Equal(t, "credential-route-census/1", row.UserAgent, key)
		assert.Equal(t, map[string]interface{}{
			"familyId": accessClaims.AccessFamilyID(),
			"jti":      accessClaims.ID,
			"route":    label,
			"cause":    "sessionRevoked",
		}, row.Metadata, key)
		assert.Equal(t, 1, lines, "%s: one SECURITY line", key)
		assert.Contains(t, logged.String(), "route="+label+" cause=sessionRevoked ", key)
	}

	want := map[string]string{}
	for _, route := range middleware.CredentialRoutes() {
		want[routeKey(route.Method, route.Path)] = route.Label
	}
	require.Greater(t, len(probed), len(want), "the census mounted too few routes to be the server's table")
	t.Logf("probed %d routes; %d recorded the refusal", len(probed), len(recorded))
	assert.Equal(t, want, recorded,
		"the routes where the session middleware records a revoked session's request changed. "+
			"Every route whose handler records credential_mint_refused belongs in middleware.CredentialRoutes, "+
			"under the label the handler records.")
}
