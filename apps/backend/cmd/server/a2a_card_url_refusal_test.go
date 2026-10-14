package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	jwtauth "github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// A card URL the server refuses to fetch is the caller's input at fault, so
// both card registration routes, POST /a2a/agents/:id/card and the SDK alias
// POST /a2a/cards, answer it with 400 and a fixed reason that names no host or
// address. The card service's reason stays in the server log. A refusal is
// made when the URL is checked and again when the client connects, so a host
// whose DNS answer changes between the two lookups is refused too. A host
// lookup that fails for a reason other than the name not existing says nothing
// about the URL and is a server fault.
//
// These tests serve the production route table (setupRoutes) with the real
// middleware chain, A2A handler and card service, and the database behind
// sqlmock, so they pin what a caller of each route sees.

const (
	cardURLRefusedBody = `{"error":"cardUrl must be a public http or https URL"}`
	// cardURLLocalhost names a host the card service refuses by name, before
	// any lookup or network call.
	cardURLLocalhost = "http://localhost/.well-known/agent.json"
)

// cardURLAuditSink accepts audit entries.
type cardURLAuditSink struct{ domain.AuditLogRepository }

func (cardURLAuditSink) Create(*domain.AuditLog) error { return nil }

type cardURLFixture struct {
	app     *fiber.App
	mock    sqlmock.Sqlmock
	jwt     *jwtauth.JWTService
	orgID   uuid.UUID
	userID  uuid.UUID
	agentID uuid.UUID
}

func newCardURLFixture(t *testing.T) *cardURLFixture {
	t.Helper()
	t.Setenv("JWT_SECRET", "card-url-refusal-test-secret-of-32-chars-or-more")

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	agentRepo := repository.NewAgentRepository(db)
	agentService := application.NewAgentService(agentRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	a2aService := application.NewA2AService(
		repository.NewA2AAgentCardRepository(db),
		repository.NewA2ASkillRepository(db),
		repository.NewA2ATaskRepository(db),
		repository.NewA2APeerTrustRepository(db),
		repository.NewA2AConsentRepository(db),
		repository.NewA2ATrustScoreRepository(db),
		repository.NewA2ARequestNonceRepository(db),
		repository.NewA2APolicyRepository(db),
		repository.NewA2AAgentAttestationRepository(db),
		repository.NewA2ARevokedAgentRepository(db),
		repository.NewA2ASecuritySettingsRepository(db),
		repository.NewA2ASecurityViolationRepository(db),
		agentRepo,
		nil,
		nil,
		nil,
	)
	auditService := application.NewAuditService(cardURLAuditSink{})

	// Handlers the route table only references are left nil: a method value
	// on a nil pointer is not called until a request reaches it, and no
	// request in these tests leaves the /a2a card routes.
	h := &Handlers{
		Auth: &handlers.AuthHandler{},
		A2A:  handlers.NewA2AHandler(a2aService, agentService, auditService),
	}
	services := &Services{Agent: agentService, Audit: auditService}
	// setupRoutes reads the ATC verifier from services.Secrets as it mounts
	// the agent routes; a zero SecretsService has none. The field is set by
	// name, so the fixture compiles against a Services without it.
	if secrets := reflect.ValueOf(services).Elem().FieldByName("Secrets"); secrets.IsValid() {
		secrets.Set(reflect.New(secrets.Type().Elem()))
	}
	repos := &Repositories{Agent: agentRepo}
	jwtService := jwtauth.NewJWTService()

	app := fiber.New(fiber.Config{ErrorHandler: customErrorHandler})
	app.Use(middleware.RecoveryMiddleware())
	mountCardURLRoutes(app.Group("/api/v1"), h, services, repos, jwtService, db)

	return &cardURLFixture{
		app:     app,
		mock:    mock,
		jwt:     jwtService,
		orgID:   uuid.New(),
		userID:  uuid.New(),
		agentID: uuid.New(),
	}
}

// mountCardURLRoutes calls setupRoutes by reflection: each parameter gets the
// value of its type from values, and any other parameter its zero value, so a
// parameter added to setupRoutes does not stop this fixture from compiling.
func mountCardURLRoutes(v1 fiber.Router, values ...any) {
	setup := reflect.ValueOf(setupRoutes)
	args := make([]reflect.Value, setup.Type().NumIn())
	for i := range args {
		param := setup.Type().In(i)
		args[i] = reflect.Zero(param)
		if reflect.TypeOf(v1).AssignableTo(param) {
			args[i] = reflect.ValueOf(v1).Convert(param)
			continue
		}
		for _, v := range values {
			if reflect.TypeOf(v) == param {
				args[i] = reflect.ValueOf(v)
				break
			}
		}
	}
	setup.Call(args)
}

// expectAgentLookup answers AgentRepository.GetByID with a verified agent in
// the caller's organization.
func (f *cardURLFixture) expectAgentLookup() {
	now := time.Now().UTC()
	f.mock.ExpectQuery(`FROM agents\s+WHERE id = \$1`).WillReturnRows(sqlmock.NewRows([]string{
		"id", "organization_id", "name", "display_name", "description", "agent_type", "status", "version",
		"public_key", "encrypted_private_key", "key_algorithm", "certificate_url", "repository_url", "documentation_url",
		"trust_score", "verified_at", "talks_to", "capabilities", "metadata", "created_at", "updated_at", "created_by", "last_active",
		"key_created_at", "key_expires_at", "key_rotation_grace_until", "previous_public_key", "rotation_count",
		"pqc_public_key", "pqc_key_algorithm", "hybrid_mode_enabled", "pqc_key_created_at", "pqc_key_expires_at", "previous_pqc_public_key",
		"created_by_name", "created_by_email", "created_by_sdk_token_id", "created_by_api_key_id",
		"updated_by", "updated_by_name", "updated_by_email",
		"capability_violation_count", "is_compromised", "declared_purpose",
	}).AddRow(
		f.agentID.String(), f.orgID.String(), "card-agent", "Card Agent", nil, string(domain.AgentTypeAI), string(domain.AgentStatusVerified), nil,
		nil, nil, "Ed25519", nil, nil, nil,
		0.0, nil, nil, nil, nil, now, now, f.userID.String(), nil,
		nil, nil, nil, nil, 0,
		nil, nil, false, nil, nil, nil,
		"", "", nil, nil,
		nil, "", "",
		0, false, nil,
	))
}

// expectNoAPIKey answers the API-key lookup the /a2a group runs for a bearer
// token with no row, so the JWT is authenticated by AuthMiddleware.
func (f *cardURLFixture) expectNoAPIKey() {
	f.mock.ExpectQuery(`FROM api_keys`).WillReturnError(sql.ErrNoRows)
}

type cardURLResponse struct {
	status int
	body   string
}

// postAsMember posts body to path with a member's JWT of the agent's
// organization.
func (f *cardURLFixture) postAsMember(t *testing.T, path, body string) cardURLResponse {
	t.Helper()
	token, _, err := f.jwt.GenerateTokenPair(f.userID.String(), f.orgID.String(), "caller@example.com", string(domain.RoleMember))
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	req := httptest.NewRequest(fiber.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	answer, _ := io.ReadAll(resp.Body)
	return cardURLResponse{status: resp.StatusCode, body: string(answer)}
}

func (f *cardURLFixture) registerCanonical(t *testing.T, cardURL string) cardURLResponse {
	t.Helper()
	return f.postAsMember(t, "/api/v1/a2a/agents/"+f.agentID.String()+"/card", `{"cardUrl":"`+cardURL+`"}`)
}

func (f *cardURLFixture) registerAlias(t *testing.T, cardURL string) cardURLResponse {
	t.Helper()
	return f.postAsMember(t, "/api/v1/a2a/cards", `{"agentId":"`+f.agentID.String()+`","cardUrl":"`+cardURL+`"}`)
}

// cardURLRoutes registers a card from a URL as a member, on each card route.
var cardURLRoutes = []struct {
	name     string
	register func(f *cardURLFixture, t *testing.T, cardURL string) cardURLResponse
}{
	{"canonical", (*cardURLFixture).registerCanonical},
	{"alias", (*cardURLFixture).registerAlias},
}

func captureCardURLLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &logged
}

// A member who registers a card for their own agent with a card URL the server
// refuses gets 400 and a reason that does not say what the URL names.
func TestCardURLRefusedOnTheCanonicalRouteIsA400WithAFixedReason(t *testing.T) {
	logged := captureCardURLLog(t)
	f := newCardURLFixture(t)
	f.expectNoAPIKey()
	f.expectAgentLookup() // the handler's tenant check
	f.expectAgentLookup() // the card service
	got := f.registerCanonical(t, cardURLLocalhost)

	if got.status != fiber.StatusBadRequest {
		t.Errorf("status = %d, body = %s; want %d for a card URL the server refuses", got.status, got.body, fiber.StatusBadRequest)
	}
	if got.body != cardURLRefusedBody {
		t.Errorf("body = %s, want %s", got.body, cardURLRefusedBody)
	}
	if err := f.mock.ExpectationsWereMet(); err != nil {
		t.Errorf("the request did not reach the card service: %v", err)
	}
	if strings.TrimSpace(logged.String()) == "" {
		t.Error("the server log has no line for the refused card URL")
	}
}

// A member naming an agent of their own organization on the alias passes the
// tenant check and reaches the card service, which loads the agent and then
// refuses the card URL's host: 400, after both agent lookups.
func TestCardURLAliasAdmitsAMemberForTheirOwnAgent(t *testing.T) {
	f := newCardURLFixture(t)
	f.expectNoAPIKey()
	f.expectAgentLookup() // the handler's tenant check
	f.expectAgentLookup() // the card service
	got := f.registerAlias(t, cardURLLocalhost)

	if got.status != fiber.StatusBadRequest {
		t.Errorf("status = %d, body = %s; want %d, the card service's refusal of the card URL", got.status, got.body, fiber.StatusBadRequest)
	}
	if err := f.mock.ExpectationsWereMet(); err != nil {
		t.Errorf("the request did not reach the card service: %v", err)
	}
}

// The card service's reason for refusing a card names the outcome of the
// server-side check (here, that the card URL's host is internal). The alias
// answers with the fixed reason and keeps the service's reason in the server
// log. The test pins that a log line was written, not how the reason reads.
func TestCardURLAliasWithholdsTheCardServiceReason(t *testing.T) {
	logged := captureCardURLLog(t)
	f := newCardURLFixture(t)
	f.expectNoAPIKey()
	f.expectAgentLookup() // the handler's tenant check
	f.expectAgentLookup() // the card service
	got := f.registerAlias(t, cardURLLocalhost)

	if got.status != fiber.StatusBadRequest {
		t.Fatalf("status = %d, body = %s; want %d, the card service's refusal of the card URL", got.status, got.body, fiber.StatusBadRequest)
	}
	if got.body != cardURLRefusedBody {
		t.Errorf("body = %s, want %s", got.body, cardURLRefusedBody)
	}
	for _, reason := range []string{"localhost", "not allowed", "fetch", "invalid agent card URL"} {
		if strings.Contains(got.body, reason) {
			t.Errorf("body = %s; it carries the card service's reason (%q)", got.body, reason)
		}
	}
	if strings.TrimSpace(logged.String()) == "" {
		t.Error("the server log has no line for the refused card URL")
	}
}

const (
	cardURLDNSTypeA    = 1
	cardURLDNSServFail = 2
	cardURLDNSNXDomain = 3
)

// cardURLDNS answers DNS queries for any name. A query for an A record gets
// the address addressFor returns for its count, from 1; a query of another
// type gets no record. When rcode is set, every query is answered with that
// response code and no record.
type cardURLDNS struct {
	rcode      byte
	addressFor func(aQuery int32) net.IP
	aQueries   atomic.Int32
}

// useCardURLDNS makes dns answer every lookup the server makes until the test
// ends: it replaces net.DefaultResolver, which both the URL check and the
// egress client's dialer use.
func useCardURLDNS(t *testing.T, dns *cardURLDNS) {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("dns listener: %v", err)
	}
	go dns.serve(conn)

	previous := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "udp", conn.LocalAddr().String())
		},
	}
	t.Cleanup(func() {
		net.DefaultResolver = previous
		_ = conn.Close()
	})
}

func (d *cardURLDNS) serve(conn net.PacketConn) {
	buf := make([]byte, 1500)
	for {
		n, from, err := conn.ReadFrom(buf)
		if err != nil {
			return
		}
		if reply := d.reply(buf[:n]); reply != nil {
			_, _ = conn.WriteTo(reply, from)
		}
	}
}

// reply answers query, a DNS message with one question: a 12-byte header, then
// the name as length-prefixed labels ending in a zero byte, a type and a class.
func (d *cardURLDNS) reply(query []byte) []byte {
	if len(query) < 12 {
		return nil
	}
	end := 12
	for end < len(query) && query[end] != 0 {
		end += int(query[end]) + 1
	}
	end += 5 // the zero byte, the type and the class
	if end > len(query) {
		return nil
	}
	qtype := binary.BigEndian.Uint16(query[end-4 : end-2])

	var record net.IP
	if d.rcode == 0 && qtype == cardURLDNSTypeA {
		record = d.addressFor(d.aQueries.Add(1)).To4()
	}
	var answers byte
	if record != nil {
		answers = 1
	}
	reply := []byte{
		query[0], query[1], // the query's ID
		0x80 | query[2]&0x01,         // a response, recursion desired as the query asked
		0x80 | d.rcode,               // recursion available, the response code
		0, 1, 0, answers, 0, 0, 0, 0, // one question, the answers, nothing else
	}
	reply = append(reply, query[12:end]...)
	if record != nil {
		reply = append(reply,
			0xc0, 12, // the name, as in the question
			0, cardURLDNSTypeA, 0, 1, // type A, class IN
			0, 0, 0, 0, // TTL
			0, 4, // data length
		)
		reply = append(reply, record...)
	}
	return reply
}

// The card host's first answer, to the URL check, is a public address; every
// later one, to the client that connects, is loopback, where a server listens.
// Both card routes answer 400 with the fixed reason, as for any card URL the
// server refuses, and no request reaches the loopback server.
func TestCardURLHostThatRebindsToLoopbackIsRefused(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"card-agent","version":"0.3"}`))
	}))
	t.Cleanup(srv.Close)
	_, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("server address: %v", err)
	}
	cardURL := "http://card-rebind.aim.test:" + port + "/.well-known/agent.json"

	for _, route := range cardURLRoutes {
		t.Run(route.name, func(t *testing.T) {
			dns := &cardURLDNS{addressFor: func(aQuery int32) net.IP {
				if aQuery == 1 {
					return net.ParseIP("93.184.215.14")
				}
				return net.ParseIP("127.0.0.1")
			}}
			useCardURLDNS(t, dns)
			logged := captureCardURLLog(t)
			f := newCardURLFixture(t)
			f.expectNoAPIKey()
			f.expectAgentLookup() // the handler's tenant check
			f.expectAgentLookup() // the card service
			got := route.register(f, t, cardURL)

			if got.status != fiber.StatusBadRequest {
				t.Errorf("status = %d, body = %s; want %d for a card host that resolves to loopback when the client connects", got.status, got.body, fiber.StatusBadRequest)
			}
			if got.body != cardURLRefusedBody {
				t.Errorf("body = %s, want %s", got.body, cardURLRefusedBody)
			}
			// Control: the URL check passed and the client resolved the host
			// again, so the refusal came from the connection.
			if n := dns.aQueries.Load(); n < 2 {
				t.Errorf("the card host was resolved %d time(s); the request did not get past the URL check to the connection", n)
			}
			if err := f.mock.ExpectationsWereMet(); err != nil {
				t.Errorf("the request did not reach the card service: %v", err)
			}
			if strings.TrimSpace(logged.String()) == "" {
				t.Error("the server log has no line for the refused card URL")
			}
		})
	}

	if n := hits.Load(); n != 0 {
		t.Errorf("%d card fetch(es) reached the loopback server", n)
	}
}

// A card host whose lookup fails for a reason other than the name not existing
// (here the resolver answers SERVFAIL, as a failing or overloaded one does)
// says nothing about the caller's URL: both card routes answer with the 5xx
// line every handler answers an unexpected failure with. A host the resolver
// reports does not exist is a problem with the URL and is answered with 400.
func TestCardURLFailedHostLookupIsAServerFault(t *testing.T) {
	for _, tc := range []struct {
		name   string
		rcode  byte
		status int
		body   string
	}{
		{"resolver failure", cardURLDNSServFail, fiber.StatusInternalServerError, `{"error":"` + handlers.ServerErrorMessage + `"}`},
		{"no such host", cardURLDNSNXDomain, fiber.StatusBadRequest, cardURLRefusedBody},
	} {
		for _, route := range cardURLRoutes {
			t.Run(tc.name+"/"+route.name, func(t *testing.T) {
				useCardURLDNS(t, &cardURLDNS{rcode: tc.rcode})
				logged := captureCardURLLog(t)
				f := newCardURLFixture(t)
				f.expectNoAPIKey()
				f.expectAgentLookup() // the handler's tenant check
				f.expectAgentLookup() // the card service
				got := route.register(f, t, "http://card-lookup.aim.test/.well-known/agent.json")

				if got.status != tc.status {
					t.Errorf("status = %d, body = %s; want %d", got.status, got.body, tc.status)
				}
				if got.body != tc.body {
					t.Errorf("body = %s, want %s", got.body, tc.body)
				}
				if err := f.mock.ExpectationsWereMet(); err != nil {
					t.Errorf("the request did not reach the card service: %v", err)
				}
				if strings.TrimSpace(logged.String()) == "" {
					t.Error("the server log has no line for the failed card fetch")
				}
			})
		}
	}
}
