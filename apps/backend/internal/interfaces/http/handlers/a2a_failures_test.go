package handlers

// Every failure an A2A route can meet, injected on each route that can meet
// it, with the status and body the route answers.
//
// The handler runs on the real A2AService, AgentService and AuditService over
// one sqlmock database. A store plan decides what each statement finds: the
// caller's agent, no agent, another organization's agent, a suspended agent, a
// failed agent read, or a failure on every statement after the agent read. A
// store that serves answers every other statement with nothing found and every
// write with one row changed. The consent and task loads that guard revoke and
// task-state changes go through the A2AReader seam.
//
// A string argument that is not UTF-8 text, or holds a NUL character, fails as
// PostgreSQL fails it as a text parameter. Every injected error carries
// a2aInjectedText, and no answer may repeat it.
//
// A failure is answered 4xx or 5xx, never 2xx. A 4xx or 5xx body states a fixed
// reason in "error". A refusal from POST /api/v1/a2a/verify also carries
// valid: false; its 500 carries the fixed line only, since it says nothing
// about the request. A route that reads the organization through
// RequireOrganizationID answers a request without one with that helper's 401,
// which the server's error handler writes; the test app's error handler writes
// it in the same shape without the timestamp.
//
// Paging and threshold values (limit, offset, minTrustScore) fall back to a
// default when they do not parse, so no failure is injected on them. A path
// value is injected with a byte that is not UTF-8 only: a request whose path
// holds a NUL byte is refused by the HTTP server before it is routed.

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

var (
	a2aCallerOrg = uuid.MustParse("0a000000-0000-4000-8000-000000000001")
	a2aOtherOrg  = uuid.MustParse("0a000000-0000-4000-8000-000000000002")
	a2aAgent     = uuid.MustParse("0a000000-0000-4000-8000-0000000000a1")
	a2aPeer      = uuid.MustParse("0a000000-0000-4000-8000-0000000000a2")
	a2aConsent   = uuid.MustParse("0a000000-0000-4000-8000-0000000000c1")
	a2aTask      = uuid.MustParse("0a000000-0000-4000-8000-0000000000d1")
	a2aUser      = uuid.MustParse("0a000000-0000-4000-8000-0000000000e1")
)

// a2aInjectedText is in the text of every error the test injects.
const a2aInjectedText = "injected failure 7c2e"

// a2aAgentKey signs the requests POST /api/v1/a2a/verify accepts; every agent
// row carries its public key.
var a2aAgentKey = ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))

// a2aAgentColumns are the columns AgentRepository.GetByID selects, in order.
var a2aAgentColumns = []string{
	"id", "organization_id", "name", "display_name", "description", "agent_type", "status", "version",
	"public_key", "encrypted_private_key", "key_algorithm", "certificate_url", "repository_url", "documentation_url",
	"trust_score", "verified_at", "talks_to", "capabilities", "metadata", "created_at", "updated_at", "created_by", "last_active",
	"key_created_at", "key_expires_at", "key_rotation_grace_until", "previous_public_key", "rotation_count",
	"pqc_public_key", "pqc_key_algorithm", "hybrid_mode_enabled", "pqc_key_created_at", "pqc_key_expires_at", "previous_pqc_public_key",
	"created_by_name", "created_by_email", "created_by_sdk_token_id", "created_by_api_key_id",
	"updated_by", "updated_by_name", "updated_by_email",
	"capability_violation_count", "is_compromised", "declared_purpose",
}

func a2aAgentRow(orgID uuid.UUID, status domain.AgentStatus) *sqlmock.Rows {
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	publicKey := base64.StdEncoding.EncodeToString(a2aAgentKey.Public().(ed25519.PublicKey))
	return sqlmock.NewRows(a2aAgentColumns).AddRow(
		a2aAgent.String(), orgID.String(), "billing-agent", "Billing Agent", nil, "ai_agent", string(status), nil,
		publicKey, nil, nil, nil, nil, nil,
		0.8, nil, nil, []byte(`[]`), nil, at, at, a2aUser.String(), nil,
		nil, nil, nil, nil, nil,
		nil, nil, false, nil, nil, nil,
		"", "", nil, nil,
		nil, "", "",
		int64(0), false, nil,
	)
}

// a2aStore is what the database holds and how it fails.
type a2aStore int

const (
	a2aStoreServes          a2aStore = iota // each agent read finds the caller's verified agent
	a2aStoreNoAgent                         // an agent read finds no row
	a2aStoreForeignAgent                    // an agent read finds another organization's agent
	a2aStoreSuspendedAgent                  // an agent read finds the caller's suspended agent
	a2aStoreAgentReadFails                  // an agent read fails
	a2aStoreFails                           // agent reads serve; every other statement fails
	a2aStoreNonceUsed                       // the nonce read finds the nonce recorded
	a2aStoreNonceWriteFails                 // recording a nonce fails
)

// a2aTextOnly refuses an argument PostgreSQL refuses: a string that is not
// UTF-8 text or holds a NUL character, and a JSON document that holds the
// escape \u0000, which jsonb refuses ("unsupported Unicode escape sequence").
type a2aTextOnly struct{}

func (a2aTextOnly) ConvertValue(v any) (driver.Value, error) {
	dv, err := driver.DefaultParameterConverter.ConvertValue(v)
	if err != nil {
		return nil, err
	}
	switch arg := dv.(type) {
	case string:
		if !utf8.ValidString(arg) || strings.ContainsRune(arg, 0) {
			return nil, errors.New(a2aInjectedText + `: invalid byte sequence for encoding "UTF8"`)
		}
	case []byte:
		if json.Valid(arg) && bytes.Contains(arg, []byte(`\u0000`)) {
			return nil, errors.New(a2aInjectedText + ": unsupported Unicode escape sequence")
		}
	}
	return dv, nil
}

func newA2AStore(t *testing.T, store a2aStore) *sql.DB {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.ValueConverterOption(a2aTextOnly{}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.MatchExpectationsInOrder(false)
	injected := errors.New(a2aInjectedText + ": read tcp 10.0.0.5:5432: connection reset by peer")

	// Each expectation answers one statement; more than any route issues.
	const each = 24
	for i := 0; i < each; i++ {
		read := mock.ExpectQuery(`FROM agents\s+WHERE id = \$1`)
		switch store {
		case a2aStoreNoAgent:
			read.WillReturnRows(sqlmock.NewRows(a2aAgentColumns))
		case a2aStoreForeignAgent:
			read.WillReturnRows(a2aAgentRow(a2aOtherOrg, domain.AgentStatusVerified))
		case a2aStoreSuspendedAgent:
			read.WillReturnRows(a2aAgentRow(a2aCallerOrg, domain.AgentStatusSuspended))
		case a2aStoreAgentReadFails:
			read.WillReturnError(injected)
		default:
			read.WillReturnRows(a2aAgentRow(a2aCallerOrg, domain.AgentStatusVerified))
		}
	}
	for i := 0; i < each; i++ {
		switch store {
		case a2aStoreFails:
			mock.ExpectBegin().WillReturnError(injected)
			mock.ExpectQuery(`.`).WillReturnError(injected)
			mock.ExpectExec(`.`).WillReturnError(injected)
		case a2aStoreNonceUsed:
			mock.ExpectQuery(`FROM a2a_request_nonces`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		case a2aStoreNonceWriteFails:
			mock.ExpectExec(`INSERT INTO a2a_request_nonces`).WillReturnError(injected)
		}
	}
	for i := 0; i < each; i++ {
		mock.ExpectQuery(`(?is)^\s*SELECT\s+COUNT\(`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(`(?is)^\s*SELECT\s+EXISTS`).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectQuery(`.`).WillReturnRows(sqlmock.NewRows(nil))
		mock.ExpectExec(`.`).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectBegin()
		mock.ExpectCommit()
		mock.ExpectRollback()
	}
	return db
}

// a2aRecordRead is what the consent and task loads find.
type a2aRecordRead int

const (
	a2aRecordOwned   a2aRecordRead = iota // the caller's consent; a task whose client is the caller's agent
	a2aRecordMissing                      // no record
	a2aRecordForeign                      // another organization's consent
	a2aRecordFails                        // the load fails
)

func a2aRecordReader(read a2aRecordRead) A2AReader {
	injected := errors.New(a2aInjectedText + ": read tcp 10.0.0.5:5432: connection reset by peer")
	return &MockA2AReaderImpl{
		GetConsentFunc: func(_ context.Context, id uuid.UUID) (*domain.A2AConsentRecord, error) {
			switch read {
			case a2aRecordMissing:
				return nil, nil
			case a2aRecordFails:
				return nil, injected
			case a2aRecordForeign:
				return &domain.A2AConsentRecord{ID: id, OrganizationID: &a2aOtherOrg}, nil
			}
			return &domain.A2AConsentRecord{ID: id, OrganizationID: &a2aCallerOrg}, nil
		},
		GetA2ATaskFunc: func(_ context.Context, id uuid.UUID) (*domain.A2ATask, error) {
			switch read {
			case a2aRecordMissing:
				return nil, nil
			case a2aRecordFails:
				return nil, injected
			}
			return &domain.A2ATask{ID: id, ClientAgentID: a2aAgent, RemoteAgentID: a2aPeer}, nil
		},
	}
}

func newA2AFailureHandler(t *testing.T, db *sql.DB, read a2aRecordRead) *A2AHandler {
	t.Helper()
	kv, err := crypto.NewKeyVault(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32)))
	require.NoError(t, err)
	ring, err := crypto.LoadSigningKeyRing(kv, func(string) string { return "" })
	require.NoError(t, err)

	agentRepo := repository.NewAgentRepository(db)
	svc := application.NewA2AService(
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
		repository.NewCapabilityRepository(sqlx.NewDb(db, "postgres")),
		kv,
		ring.Key(crypto.PurposeCardAttestation),
	)
	agentSvc := application.NewAgentService(agentRepo, nil, nil, kv, nil, nil, nil, nil, nil, nil, nil, nil)
	h := NewA2AHandler(svc, agentSvc, application.NewAuditService(repository.NewAuditLogRepository(db)))
	h.a2aReader = a2aRecordReader(read)
	return h
}

// a2aFailureCell is one failure injected on one route.
type a2aFailureCell struct {
	failure string
	target  string  // "" sends the route's target
	body    *string // nil sends the route's body
	noOrg   bool    // no organization in context
	noAgent bool    // no agent principal in context
	store   a2aStore
	record  a2aRecordRead
}

type a2aFailureRoute struct {
	method, path string
	handle       func(*A2AHandler) fiber.Handler
	target, body string
	cells        []a2aFailureCell
}

func a2aBody(s string) *string { return &s }

func a2aNoOrg() a2aFailureCell {
	return a2aFailureCell{failure: "no organization in context", noOrg: true}
}

func a2aMalformed() a2aFailureCell {
	return a2aFailureCell{failure: "malformed JSON", body: a2aBody(`{"agentId": `)}
}

func a2aAt(failure, target string) a2aFailureCell {
	return a2aFailureCell{failure: failure, target: target}
}

func a2aSends(failure, body string) a2aFailureCell {
	return a2aFailureCell{failure: failure, body: a2aBody(body)}
}

func a2aStored(store a2aStore) a2aFailureCell {
	names := map[a2aStore]string{
		a2aStoreNoAgent:         "unknown agent",
		a2aStoreForeignAgent:    "agent of another organization",
		a2aStoreSuspendedAgent:  "suspended agent",
		a2aStoreAgentReadFails:  "agent read fails",
		a2aStoreFails:           "repository fails",
		a2aStoreNonceUsed:       "nonce already used",
		a2aStoreNonceWriteFails: "nonce write fails",
	}
	return a2aFailureCell{failure: names[store], store: store}
}

func a2aRecord(read a2aRecordRead, failure string) a2aFailureCell {
	return a2aFailureCell{failure: failure, record: read}
}

func a2aNoAgentPrincipal(failure, body string) a2aFailureCell {
	c := a2aFailureCell{failure: failure, noAgent: true}
	if body != "" {
		c.body = a2aBody(body)
	}
	return c
}

// a2aVerifyBody is a verification request for the caller's agent, signed with
// key. A nil key signs nothing valid.
func a2aVerifyBody(nonce string, at time.Time, key ed25519.PrivateKey) string {
	hash := sha256.Sum256([]byte("POST /api/v1/a2a/tasks"))
	signature := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, ed25519.SignatureSize))
	if key != nil {
		signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, hash[:]))
	}
	body, err := json.Marshal(map[string]any{
		"agentId":     a2aAgent,
		"timestamp":   at.Unix(),
		"nonce":       nonce,
		"signature":   signature,
		"requestHash": hex.EncodeToString(hash[:]),
	})
	if err != nil {
		panic(err)
	}
	return string(body)
}

func a2aFailureRoutes() []a2aFailureRoute {
	A, P, C, T := a2aAgent.String(), a2aPeer.String(), a2aConsent.String(), a2aTask.String()
	now := time.Now().UTC()

	agentPath := func(prefix, suffix string) []a2aFailureCell {
		return []a2aFailureCell{a2aAt("id not UTF-8", prefix+"\xff"+suffix)}
	}
	agentLoads := []a2aFailureCell{
		a2aStored(a2aStoreNoAgent),
		a2aStored(a2aStoreForeignAgent),
		a2aStored(a2aStoreAgentReadFails),
		a2aStored(a2aStoreFails),
	}
	cells := func(groups ...[]a2aFailureCell) []a2aFailureCell {
		var all []a2aFailureCell
		for _, g := range groups {
			all = append(all, g...)
		}
		return all
	}
	one := func(c ...a2aFailureCell) []a2aFailureCell { return c }

	return []a2aFailureRoute{
		{
			method: "GET", path: "/.well-known/agent.json", target: "/.well-known/agent.json?agentId=" + A,
			handle: func(h *A2AHandler) fiber.Handler { return h.GetPublicAgentCard },
			cells: one(
				a2aAt("agentId missing", "/.well-known/agent.json"),
				a2aAt("agentId not UTF-8", "/.well-known/agent.json?agentId=%FF"),
				a2aAt("agentId holds NUL", "/.well-known/agent.json?agentId=%00"),
				a2aStored(a2aStoreNoAgent),
				a2aStored(a2aStoreAgentReadFails),
				a2aStored(a2aStoreFails),
			),
		},
		{
			method: "POST", path: "/api/v1/a2a/verify", target: "/api/v1/a2a/verify",
			body:   a2aVerifyBody("nonce-1", now, a2aAgentKey),
			handle: func(h *A2AHandler) fiber.Handler { return h.VerifyRequest },
			cells: one(
				a2aMalformed(),
				a2aSends("nonce empty", a2aVerifyBody("", now, a2aAgentKey)),
				a2aSends("timestamp outside the window", a2aVerifyBody("nonce-1", now.Add(-time.Hour), a2aAgentKey)),
				a2aSends("invalid signature", a2aVerifyBody("nonce-1", now, nil)),
				a2aStored(a2aStoreNoAgent),
				a2aStored(a2aStoreSuspendedAgent),
				a2aStored(a2aStoreAgentReadFails),
				a2aStored(a2aStoreNonceUsed),
				a2aStored(a2aStoreFails),
				a2aStored(a2aStoreNonceWriteFails),
			),
		},
		{
			method: "POST", path: "/api/v1/a2a/agents/:id/card", target: "/api/v1/a2a/agents/" + A + "/card",
			body:   `{"cardData":{"name":"billing-agent","url":"https://agents.example.com/billing"}}`,
			handle: func(h *A2AHandler) fiber.Handler { return h.RegisterAgentCard },
			cells: cells(
				one(a2aNoOrg()),
				agentPath("/api/v1/a2a/agents/", "/card"),
				agentLoads,
				one(
					a2aMalformed(),
					a2aSends("neither cardUrl nor cardData", `{}`),
					a2aSends("card URL refused", `{"cardUrl":"http://127.0.0.1/card.json"}`),
				),
			),
		},
		{
			method: "GET", path: "/api/v1/a2a/agents/:id/card", target: "/api/v1/a2a/agents/" + A + "/card",
			handle: func(h *A2AHandler) fiber.Handler { return h.GetAgentCard },
			cells:  cells(one(a2aNoOrg()), agentPath("/api/v1/a2a/agents/", "/card"), agentLoads),
		},
		{
			method: "POST", path: "/api/v1/a2a/agents/:id/card/refresh", target: "/api/v1/a2a/agents/" + A + "/card/refresh",
			handle: func(h *A2AHandler) fiber.Handler { return h.RefreshCardAttestation },
			cells:  cells(one(a2aNoOrg()), agentPath("/api/v1/a2a/agents/", "/card/refresh"), agentLoads),
		},
		{
			method: "POST", path: "/api/v1/a2a/agents/:id/sign", target: "/api/v1/a2a/agents/" + A + "/sign",
			body:   `{"method":"POST","path":"/api/v1/a2a/tasks","body":"{}"}`,
			handle: func(h *A2AHandler) fiber.Handler { return h.SignRequest },
			cells:  cells(one(a2aNoOrg(), a2aMalformed()), agentPath("/api/v1/a2a/agents/", "/sign"), agentLoads),
		},
		{
			method: "GET", path: "/api/v1/a2a/agents/:id/trust-score", target: "/api/v1/a2a/agents/" + A + "/trust-score",
			handle: func(h *A2AHandler) fiber.Handler { return h.GetA2ATrustScore },
			cells:  cells(one(a2aNoOrg()), agentPath("/api/v1/a2a/agents/", "/trust-score"), agentLoads),
		},
		{
			method: "POST", path: "/api/v1/a2a/agents/:id/trust-score/compute", target: "/api/v1/a2a/agents/" + A + "/trust-score/compute",
			handle: func(h *A2AHandler) fiber.Handler { return h.ComputeA2ATrustScore },
			cells:  cells(one(a2aNoOrg()), agentPath("/api/v1/a2a/agents/", "/trust-score/compute"), agentLoads),
		},
		{
			method: "GET", path: "/api/v1/a2a/agents/:id/peers/:peer_id/trust", target: "/api/v1/a2a/agents/" + A + "/peers/" + P + "/trust",
			handle: func(h *A2AHandler) fiber.Handler { return h.GetPeerTrustScore },
			cells: cells(
				one(a2aNoOrg()),
				agentPath("/api/v1/a2a/agents/", "/peers/"+P+"/trust"),
				one(
					a2aAt("peer_id not UTF-8", "/api/v1/a2a/agents/"+A+"/peers/\xff/trust"),
				),
				agentLoads,
			),
		},
		{
			method: "GET", path: "/api/v1/a2a/agents/:id/skills", target: "/api/v1/a2a/agents/" + A + "/skills",
			handle: func(h *A2AHandler) fiber.Handler { return h.GetAgentSkills },
			cells:  cells(one(a2aNoOrg()), agentPath("/api/v1/a2a/agents/", "/skills"), agentLoads),
		},
		{
			method: "GET", path: "/api/v1/a2a/skills/search", target: "/api/v1/a2a/skills/search?q=summarize",
			handle: func(h *A2AHandler) fiber.Handler { return h.SearchSkills },
			cells: one(
				a2aAt("q missing", "/api/v1/a2a/skills/search"),
				a2aAt("q not UTF-8", "/api/v1/a2a/skills/search?q=%FF"),
				a2aAt("q holds NUL", "/api/v1/a2a/skills/search?q=a%00b"),
				a2aStored(a2aStoreFails),
			),
		},
		{
			method: "GET", path: "/api/v1/a2a/route", target: "/api/v1/a2a/route?intent=summarize",
			handle: func(h *A2AHandler) fiber.Handler { return h.RouteByIntent },
			cells: one(
				a2aAt("intent missing", "/api/v1/a2a/route"),
				a2aAt("intent not UTF-8", "/api/v1/a2a/route?intent=%FF"),
				a2aAt("intent holds NUL", "/api/v1/a2a/route?intent=a%00b"),
				a2aAt("no agent matches the intent", ""),
				a2aStored(a2aStoreFails),
			),
		},
		{
			method: "GET", path: "/api/v1/a2a/capable-of", target: "/api/v1/a2a/capable-of?intent=summarize",
			handle: func(h *A2AHandler) fiber.Handler { return h.CapableOf },
			cells: one(
				a2aAt("intent missing", "/api/v1/a2a/capable-of"),
				a2aAt("intent not UTF-8", "/api/v1/a2a/capable-of?intent=%FF"),
				a2aAt("intent holds NUL", "/api/v1/a2a/capable-of?intent=a%00b"),
				a2aStored(a2aStoreFails),
			),
		},
		{
			method: "GET", path: "/api/v1/a2a/tasks", target: "/api/v1/a2a/tasks",
			handle: func(h *A2AHandler) fiber.Handler { return h.ListTasks },
			cells: one(
				a2aNoOrg(),
				a2aAt("agentId not UTF-8", "/api/v1/a2a/tasks?agentId=%FF"),
				a2aAt("agentId holds NUL", "/api/v1/a2a/tasks?agentId=%00"),
				a2aAt("agentId not a UUID", "/api/v1/a2a/tasks?agentId=billing"),
				a2aAt("state not UTF-8", "/api/v1/a2a/tasks?state=%FF"),
				a2aAt("state holds NUL", "/api/v1/a2a/tasks?state=a%00b"),
				a2aStored(a2aStoreFails),
			),
		},
		{
			method: "POST", path: "/api/v1/a2a/tasks", target: "/api/v1/a2a/tasks",
			body:   `{"targetAgentId":"` + P + `","taskId":"task-1","taskType":"summarize"}`,
			handle: func(h *A2AHandler) fiber.Handler { return h.LogTask },
			cells: cells(
				one(
					a2aNoOrg(),
					a2aMalformed(),
					a2aSends("targetAgentId not a UUID", `{"targetAgentId":"billing","taskId":"task-1"}`),
					a2aSends("clientAgentId not a UUID", `{"targetAgentId":"`+P+`","clientAgentId":"billing"}`),
					a2aSends("internal clientAgentId not a UUID", `{"remoteAgentId":"`+P+`","clientAgentId":"billing"}`),
					a2aNoAgentPrincipal("no client agent", ""),
					a2aNoAgentPrincipal("no client agent, internal form", `{"remoteAgentId":"`+P+`"}`),
				),
				agentLoads,
			),
		},
		{
			method: "PUT", path: "/api/v1/a2a/tasks/:id/state", target: "/api/v1/a2a/tasks/" + T + "/state",
			body:   `{"state":"COMPLETED"}`,
			handle: func(h *A2AHandler) fiber.Handler { return h.UpdateTaskState },
			cells: one(
				a2aNoOrg(),
				a2aAt("id not UTF-8", "/api/v1/a2a/tasks/\xff/state"),
				a2aMalformed(),
				a2aSends("state unknown", `{"state":"done"}`),
				a2aRecord(a2aRecordMissing, "unknown task"),
				a2aRecord(a2aRecordFails, "task read fails"),
				a2aStored(a2aStoreForeignAgent),
				a2aStored(a2aStoreAgentReadFails),
				a2aStored(a2aStoreFails),
			),
		},
		{
			method: "GET", path: "/api/v1/a2a/consents", target: "/api/v1/a2a/consents",
			handle: func(h *A2AHandler) fiber.Handler { return h.ListAllConsents },
			cells:  one(a2aNoOrg(), a2aStored(a2aStoreFails)),
		},
		{
			method: "POST", path: "/api/v1/a2a/consent", target: "/api/v1/a2a/consent",
			body:   `{"userId":"user-1","grantorAgentId":"` + A + `","recipientAgentId":"` + P + `","scope":["read"],"purpose":"billing"}`,
			handle: func(h *A2AHandler) fiber.Handler { return h.RecordConsent },
			cells:  cells(one(a2aNoOrg(), a2aMalformed()), agentLoads),
		},
		{
			method: "GET", path: "/api/v1/a2a/consent/check",
			target: "/api/v1/a2a/consent/check?userId=user-1&grantorAgentId=" + A + "&recipientAgentId=" + P + "&scope=read",
			handle: func(h *A2AHandler) fiber.Handler { return h.CheckConsent },
			cells: cells(
				one(
					a2aNoOrg(),
					a2aAt("parameters missing", "/api/v1/a2a/consent/check"),
					a2aAt("grantorAgentId not UTF-8", "/api/v1/a2a/consent/check?userId=user-1&grantorAgentId=%FF&recipientAgentId="+P+"&scope=read"),
					a2aAt("recipientAgentId holds NUL", "/api/v1/a2a/consent/check?userId=user-1&grantorAgentId="+A+"&recipientAgentId=%00&scope=read"),
					a2aAt("userId not UTF-8", "/api/v1/a2a/consent/check?userId=%FF&grantorAgentId="+A+"&recipientAgentId="+P+"&scope=read"),
					a2aAt("userId holds NUL", "/api/v1/a2a/consent/check?userId=a%00b&grantorAgentId="+A+"&recipientAgentId="+P+"&scope=read"),
					a2aAt("scope not UTF-8", "/api/v1/a2a/consent/check?userId=user-1&grantorAgentId="+A+"&recipientAgentId="+P+"&scope=%FF"),
					a2aAt("scope holds NUL", "/api/v1/a2a/consent/check?userId=user-1&grantorAgentId="+A+"&recipientAgentId="+P+"&scope=a%00b"),
				),
				agentLoads,
			),
		},
		{
			method: "POST", path: "/api/v1/a2a/consent/:id/revoke", target: "/api/v1/a2a/consent/" + C + "/revoke",
			body:   `{"reason":"user asked"}`,
			handle: func(h *A2AHandler) fiber.Handler { return h.RevokeConsent },
			cells: one(
				a2aNoOrg(),
				a2aAt("id not UTF-8", "/api/v1/a2a/consent/\xff/revoke"),
				a2aRecord(a2aRecordMissing, "unknown consent"),
				a2aRecord(a2aRecordForeign, "consent of another organization"),
				a2aRecord(a2aRecordFails, "consent read fails"),
				a2aMalformed(),
				a2aStored(a2aStoreFails),
			),
		},
		{
			method: "GET", path: "/api/v1/a2a/consent/user/:userId", target: "/api/v1/a2a/consent/user/user-1",
			handle: func(h *A2AHandler) fiber.Handler { return h.ListUserConsents },
			cells: one(
				a2aNoOrg(),
				a2aAt("userId not UTF-8", "/api/v1/a2a/consent/user/\xff"),
				a2aStored(a2aStoreFails),
			),
		},
		{
			method: "GET", path: "/api/v1/a2a/trust-scores", target: "/api/v1/a2a/trust-scores",
			handle: func(h *A2AHandler) fiber.Handler { return h.ListAllTrustScores },
			cells:  one(a2aNoOrg(), a2aStored(a2aStoreFails)),
		},
		{
			method: "POST", path: "/api/v1/a2a/policies/evaluate", target: "/api/v1/a2a/policies/evaluate",
			body:   `{"event":"task.create","clientAgentId":"` + A + `","remoteAgentId":"` + P + `","skillId":"summarize","organizationId":"` + a2aCallerOrg.String() + `"}`,
			handle: func(h *A2AHandler) fiber.Handler { return h.EvaluatePolicy },
			cells: one(
				a2aNoOrg(),
				a2aMalformed(),
				a2aSends("organizationId of another organization", `{"event":"task.create","clientAgentId":"`+A+`","remoteAgentId":"`+P+`","skillId":"summarize","organizationId":"`+a2aOtherOrg.String()+`"}`),
				a2aStored(a2aStoreFails),
			),
		},
		{
			method: "POST", path: "/api/v1/a2a/skills", target: "/api/v1/a2a/skills",
			body:   `{"agentId":"` + A + `","id":"summarize","name":"Summarize"}`,
			handle: func(h *A2AHandler) fiber.Handler { return h.RegisterSkill },
			cells: one(
				a2aMalformed(),
				a2aNoAgentPrincipal("no agent", `{"id":"summarize","name":"Summarize"}`),
				a2aStored(a2aStoreNoAgent),
				a2aStored(a2aStoreAgentReadFails),
				a2aStored(a2aStoreFails),
			),
		},
		{
			method: "POST", path: "/api/v1/a2a/attestations", target: "/api/v1/a2a/attestations",
			body:   `{"attestingAgentId":"` + A + `","attestedAgentId":"` + P + `","skillId":"summarize","attestationType":"capability","confidence":0.9}`,
			handle: func(h *A2AHandler) fiber.Handler { return h.AttestSkill },
			cells: one(
				a2aMalformed(),
				a2aSends("attestedAgentId not a UUID", `{"attestingAgentId":"`+A+`","attestedAgentId":"billing"}`),
				a2aSends("attestingAgentId not a UUID", `{"attestingAgentId":"billing","attestedAgentId":"`+P+`"}`),
				a2aNoAgentPrincipal("no attesting agent", `{"attestedAgentId":"`+P+`","skillId":"summarize"}`),
				a2aStored(a2aStoreNoAgent),
				a2aStored(a2aStoreAgentReadFails),
				a2aStored(a2aStoreFails),
			),
		},
		{
			method: "GET", path: "/api/v1/a2a/attestations/:agentId/:skillId", target: "/api/v1/a2a/attestations/" + A + "/summarize",
			handle: func(h *A2AHandler) fiber.Handler { return h.GetSkillAttestations },
			cells: cells(
				one(
					a2aNoOrg(),
					a2aAt("agentId not UTF-8", "/api/v1/a2a/attestations/\xff/summarize"),
					a2aAt("skillId not UTF-8", "/api/v1/a2a/attestations/"+A+"/\xff"),
				),
				agentLoads,
			),
		},
		{
			method: "GET", path: "/api/v1/a2a/consensus/:agentId/:skillId", target: "/api/v1/a2a/consensus/" + A + "/summarize",
			handle: func(h *A2AHandler) fiber.Handler { return h.GetConsensusStatus },
			cells: cells(
				one(
					a2aNoOrg(),
					a2aAt("agentId not UTF-8", "/api/v1/a2a/consensus/\xff/summarize"),
					a2aAt("skillId not UTF-8", "/api/v1/a2a/consensus/"+A+"/\xff"),
				),
				agentLoads,
			),
		},
		{
			method: "POST", path: "/api/v1/a2a/security/check", target: "/api/v1/a2a/security/check",
			body:   `{"requestingAgentId":"` + A + `","targetAgentId":"` + P + `","skillId":"summarize"}`,
			handle: func(h *A2AHandler) fiber.Handler { return h.CheckSecurity },
			cells: one(
				a2aMalformed(),
				a2aNoAgentPrincipal("no requesting agent", `{"targetAgentId":"`+P+`"}`),
				a2aSends("targetAgentId not a UUID", `{"requestingAgentId":"`+A+`","targetAgentId":"billing"}`),
				a2aStored(a2aStoreFails),
			),
		},
		{
			method: "GET", path: "/api/v1/a2a/security/settings", target: "/api/v1/a2a/security/settings",
			handle: func(h *A2AHandler) fiber.Handler { return h.GetSecuritySettings },
			cells:  one(a2aNoOrg(), a2aStored(a2aStoreFails)),
		},
		{
			method: "PUT", path: "/api/v1/a2a/security/settings", target: "/api/v1/a2a/security/settings",
			body:   `{}`,
			handle: func(h *A2AHandler) fiber.Handler { return h.UpdateSecuritySettings },
			cells:  one(a2aNoOrg(), a2aMalformed(), a2aStored(a2aStoreFails)),
		},
		{
			method: "POST", path: "/api/v1/a2a/sign", target: "/api/v1/a2a/sign",
			body:   `{"agentId":"` + A + `","method":"POST","path":"/api/v1/a2a/tasks","body":"{}"}`,
			handle: func(h *A2AHandler) fiber.Handler { return h.SignRequestAlt },
			cells: one(
				a2aMalformed(),
				a2aNoAgentPrincipal("no agent", `{"method":"POST","path":"/api/v1/a2a/tasks"}`),
				a2aStored(a2aStoreNoAgent),
				a2aStored(a2aStoreAgentReadFails),
				a2aStored(a2aStoreFails),
			),
		},
		{
			method: "GET", path: "/api/v1/a2a/trust/:id", target: "/api/v1/a2a/trust/" + A,
			handle: func(h *A2AHandler) fiber.Handler { return h.GetTrustScoreAlt },
			cells:  cells(one(a2aNoOrg()), agentPath("/api/v1/a2a/trust/", ""), agentLoads),
		},
		{
			method: "PUT", path: "/api/v1/a2a/trust/:id", target: "/api/v1/a2a/trust/" + A,
			body:   `{"score":0.9,"confidence":0.8,"reason":"observed"}`,
			handle: func(h *A2AHandler) fiber.Handler { return h.RefuseTrustScoreWrite },
			cells:  one(a2aAt("a score written", "")),
		},
		{
			method: "POST", path: "/api/v1/a2a/trust/:id/interaction", target: "/api/v1/a2a/trust/" + A + "/interaction",
			body:   `{"taskType":"summarize","success":true,"durationMs":120}`,
			handle: func(h *A2AHandler) fiber.Handler { return h.RecordInteraction },
			cells: cells(
				one(a2aNoOrg(), a2aMalformed(), a2aNoAgentPrincipal("no agent", "")),
				agentPath("/api/v1/a2a/trust/", "/interaction"),
				agentLoads,
			),
		},
		{
			method: "POST", path: "/api/v1/a2a/discovery/route", target: "/api/v1/a2a/discovery/route",
			body:   `{"intent":"summarize","minTrustScore":0.5}`,
			handle: func(h *A2AHandler) fiber.Handler { return h.RouteByIntentPost },
			cells:  one(a2aMalformed(), a2aSends("intent missing", `{}`), a2aStored(a2aStoreFails)),
		},
		{
			method: "POST", path: "/api/v1/a2a/discovery/capable", target: "/api/v1/a2a/discovery/capable",
			body:   `{"skillIds":["summarize"],"minTrustScore":0.5}`,
			handle: func(h *A2AHandler) fiber.Handler { return h.CapableOfPost },
			cells:  one(a2aMalformed(), a2aStored(a2aStoreFails)),
		},
		{
			method: "GET", path: "/api/v1/a2a/cards", target: "/api/v1/a2a/cards",
			handle: func(h *A2AHandler) fiber.Handler { return h.ListAgentCards },
			cells:  one(a2aNoOrg(), a2aStored(a2aStoreFails)),
		},
		{
			// The card fetch is not reached: every cell fails before it, and
			// the refused card URL is refused before a connection is opened.
			method: "POST", path: "/api/v1/a2a/cards", target: "/api/v1/a2a/cards",
			body:   `{"agentId":"` + A + `","cardUrl":"http://127.0.0.1/card.json"}`,
			handle: func(h *A2AHandler) fiber.Handler { return h.RegisterAgentCardAlt },
			cells: one(
				a2aNoOrg(),
				a2aMalformed(),
				a2aSends("agentId not a UUID", `{"agentId":"billing","cardUrl":"http://127.0.0.1/card.json"}`),
				a2aSends("another agent's ID", `{"agentId":"`+P+`","cardUrl":"http://127.0.0.1/card.json"}`),
				a2aNoAgentPrincipal("no agent", `{"cardUrl":"http://127.0.0.1/card.json"}`),
				a2aStored(a2aStoreNoAgent),
				a2aStored(a2aStoreForeignAgent),
				a2aStored(a2aStoreAgentReadFails),
				a2aAt("card URL refused", ""),
			),
		},
		{
			method: "GET", path: "/api/v1/a2a/cards/:id", target: "/api/v1/a2a/cards/" + A,
			handle: func(h *A2AHandler) fiber.Handler { return h.GetAgentCard },
			cells:  cells(one(a2aNoOrg()), agentPath("/api/v1/a2a/cards/", ""), agentLoads),
		},
		{
			method: "PUT", path: "/api/v1/a2a/cards/:id", target: "/api/v1/a2a/cards/" + A,
			body:   `{"cardData":{"name":"billing-agent","url":"https://agents.example.com/billing"}}`,
			handle: func(h *A2AHandler) fiber.Handler { return h.UpdateAgentCard },
			cells: cells(
				one(a2aNoOrg(), a2aMalformed()),
				agentPath("/api/v1/a2a/cards/", ""),
				agentLoads,
				one(
					a2aSends("neither cardUrl nor cardData", `{}`),
					a2aSends("field members without a card document", `{"name":"billing-agent"}`),
					a2aSends("card URL refused", `{"cardUrl":"http://127.0.0.1/card.json"}`),
				),
			),
		},
		{
			method: "POST", path: "/api/v1/a2a/cards/:id/attestation", target: "/api/v1/a2a/cards/" + A + "/attestation",
			handle: func(h *A2AHandler) fiber.Handler { return h.RefreshCardAttestation },
			cells:  cells(one(a2aNoOrg()), agentPath("/api/v1/a2a/cards/", "/attestation"), agentLoads),
		},
		{
			method: "GET", path: "/api/v1/a2a/peers", target: "/api/v1/a2a/peers",
			handle: func(h *A2AHandler) fiber.Handler { return h.ListPeerTrusts },
			cells:  one(a2aNoAgentPrincipal("no agent", ""), a2aStored(a2aStoreFails)),
		},
		{
			method: "DELETE", path: "/api/v1/a2a/skills/:id", target: "/api/v1/a2a/skills/summarize",
			handle: func(h *A2AHandler) fiber.Handler { return h.DeleteSkill },
			cells: one(
				a2aAt("id not UTF-8", "/api/v1/a2a/skills/\xff"),
			),
		},
		{
			method: "GET", path: "/api/v1/a2a/security/violations", target: "/api/v1/a2a/security/violations",
			handle: func(h *A2AHandler) fiber.Handler { return h.GetSecurityViolations },
			cells:  one(a2aNoOrg(), a2aStored(a2aStoreFails)),
		},
		{
			method: "GET", path: "/api/v1/a2a/agents/:id/attestations", target: "/api/v1/a2a/agents/" + A + "/attestations?skillId=summarize",
			handle: func(h *A2AHandler) fiber.Handler { return h.GetAgentAttestations },
			cells: cells(
				one(
					a2aNoOrg(),
					a2aAt("skillId not UTF-8", "/api/v1/a2a/agents/"+A+"/attestations?skillId=%FF"),
					a2aAt("skillId holds NUL", "/api/v1/a2a/agents/"+A+"/attestations?skillId=a%00b"),
				),
				agentPath("/api/v1/a2a/agents/", "/attestations"),
				agentLoads,
			),
		},
		{
			method: "GET", path: "/api/v1/a2a/agents/:id/skills/:skillId/consensus", target: "/api/v1/a2a/agents/" + A + "/skills/summarize/consensus",
			handle: func(h *A2AHandler) fiber.Handler { return h.GetConsensusStatus },
			cells: cells(
				one(
					a2aNoOrg(),
					a2aAt("skillId not UTF-8", "/api/v1/a2a/agents/"+A+"/skills/\xff/consensus"),
				),
				agentPath("/api/v1/a2a/agents/", "/skills/summarize/consensus"),
				agentLoads,
			),
		},
		{
			method: "POST", path: "/api/v1/a2a/maintenance/cleanup-nonces", target: "/api/v1/a2a/maintenance/cleanup-nonces",
			handle: func(h *A2AHandler) fiber.Handler { return h.CleanupExpiredNonces },
			cells:  one(a2aStored(a2aStoreFails)),
		},
		{
			method: "POST", path: "/api/v1/a2a/maintenance/refresh-cards", target: "/api/v1/a2a/maintenance/refresh-cards",
			handle: func(h *A2AHandler) fiber.Handler { return h.RefreshExpiredCards },
			cells:  one(a2aStored(a2aStoreFails)),
		},
	}
}

// a2aFailureAnswer runs one cell and returns its status, its body and the
// panic the handler raised, if any.
func a2aFailureAnswer(t *testing.T, route a2aFailureRoute, cell a2aFailureCell) (int, string, string) {
	t.Helper()
	h := newA2AFailureHandler(t, newA2AStore(t, cell.store), cell.record)

	app := fiber.New(fiber.Config{
		// The server's error handler writes this shape and a timestamp.
		ErrorHandler: func(c fiber.Ctx, err error) error {
			code, message := fiber.StatusInternalServerError, "Internal Server Error"
			var fe *fiber.Error
			if errors.As(err, &fe) {
				code, message = fe.Code, fe.Message
			}
			return c.Status(code).JSON(fiber.Map{"error": true, "message": message})
		},
	})
	var panicked string
	app.Add([]string{route.method}, route.path, func(c fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				panicked = fmt.Sprint(r)
				err = errors.New("handler panicked")
			}
		}()
		if !cell.noOrg {
			c.Locals("organization_id", a2aCallerOrg)
		}
		c.Locals("user_id", a2aUser)
		if !cell.noAgent {
			c.Locals("agent_id", a2aAgent)
		}
		return route.handle(h)(c)
	})

	target, body := route.target, route.body
	if cell.target != "" {
		target = cell.target
	}
	if cell.body != nil {
		body = *cell.body
	}
	req := httptest.NewRequest(route.method, "/", strings.NewReader(body))
	// The request line carries target as written: a raw byte in the path
	// reaches the router as it does from a client that sends one.
	req.RequestURI = target
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(raw), panicked
}

func TestA2AFailuresOnEveryRoute(t *testing.T) {
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })

	routes := a2aFailureRoutes()
	record := os.Getenv("A2A_FAILURES_RECORD") != ""
	seen := map[string]bool{}
	for _, route := range routes {
		for _, cell := range route.cells {
			name := route.method + " " + route.path + " | " + cell.failure
			require.Falsef(t, seen[name], "cell %q is listed twice", name)
			seen[name] = true
			t.Run(name, func(t *testing.T) {
				logs.Reset()
				status, body, panicked := a2aFailureAnswer(t, route, cell)
				got := strconv.Itoa(status) + " " + body
				if record {
					t.Logf("RECORD %q: %q panic=%q", name, got, panicked)
					return
				}
				require.Emptyf(t, panicked, "handler panicked; log:\n%s", logs.String())
				want, ok := a2aFailureAnswers[name]
				require.Truef(t, ok, "no recorded answer for %q; it answers %q", name, got)
				assert.Equalf(t, want, got, "log:\n%s", logs.String())
				assertA2AFailureAnswer(t, route, cell, name, status, body)
			})
		}
	}
	for name := range a2aFailureAnswers {
		assert.Truef(t, seen[name], "recorded answer %q has no cell", name)
	}
	t.Logf("%d routes, %d cells", len(routes), len(seen))
}

// A revoke without a body, or with only white space, is revoked with the
// default reason. Only a body that is present and is not JSON is refused.
func TestA2ARevokeConsentWithoutABodyUsesTheDefaultReason(t *testing.T) {
	for name, body := range map[string]string{"no body": "", "white space": " \n"} {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			mock.MatchExpectationsInOrder(false)
			mock.ExpectExec(`UPDATE a2a_consent_records SET`).
				WithArgs(sqlmock.AnyArg(), "User revoked consent", sqlmock.AnyArg(), a2aConsent).
				WillReturnResult(sqlmock.NewResult(0, 1))

			h := newA2AFailureHandler(t, db, a2aRecordOwned)
			app := fiber.New()
			app.Post("/api/v1/a2a/consent/:id/revoke", func(c fiber.Ctx) error {
				c.Locals("organization_id", a2aCallerOrg)
				c.Locals("user_id", a2aUser)
				return h.RevokeConsent(c)
			})
			req := httptest.NewRequest("POST", "/api/v1/a2a/consent/"+a2aConsent.String()+"/revoke", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			raw, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, fiber.StatusOK, resp.StatusCode)
			assert.JSONEq(t, `{"consentId":"`+a2aConsent.String()+`","message":"Consent revoked successfully","revoked":true}`, string(raw))
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// a2aFailuresTheHandlerCannotSee are cells the service answers with a result
// of its own: it reads the store, ignores the failure and returns no error,
// so the handler has no failure to answer. Each records the 200 it answers.
var a2aFailuresTheHandlerCannotSee = map[string]bool{
	"GET /api/v1/a2a/consensus/:agentId/:skillId | repository fails":          true,
	"GET /api/v1/a2a/agents/:id/skills/:skillId/consensus | repository fails": true,
	"POST /api/v1/a2a/security/check | repository fails":                      true,
	"GET /api/v1/a2a/security/settings | repository fails":                    true,
}

// assertA2AFailureAnswer holds an answer to what every failure answer is.
func assertA2AFailureAnswer(t *testing.T, route a2aFailureRoute, cell a2aFailureCell, name string, status int, body string) {
	t.Helper()
	for _, text := range []string{a2aInjectedText, "invalid byte sequence", "connection reset", "sqlmock", "SELECT", "INSERT"} {
		assert.NotContains(t, body, text, "an answer states a reason, never an error's text")
	}
	if a2aFailuresTheHandlerCannotSee[name] {
		return
	}
	assert.GreaterOrEqual(t, status, 400, "a failure is answered 4xx or 5xx")
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(body), &got), "the body is a JSON object")
	if cell.noOrg && got["error"] == true {
		assert.Equal(t, ErrOrganizationIDNotFound.Message, got["message"], "the server's error handler writes the context helper's 401")
		return
	}
	reason, ok := got["error"].(string)
	assert.Truef(t, ok && reason != "", "error states a reason: %s", body)
	if route.path == "/api/v1/a2a/verify" {
		if status < 500 {
			assert.Equal(t, false, got["valid"], "a refused verification carries valid: false")
		} else {
			assert.NotContains(t, got, "valid", "a 500 says nothing about the request")
		}
	}
}

// a2aFailureAnswers is the status and body each cell is answered with.
var a2aFailureAnswers = map[string]string{
	"GET /.well-known/agent.json | agentId missing":                                        `400 {"error":"Agent ID is required (query param 'agentId' or header 'X-Agent-ID')"}`,
	"GET /.well-known/agent.json | agentId not UTF-8":                                      `400 {"error":"Invalid agent ID"}`,
	"GET /.well-known/agent.json | agentId holds NUL":                                      `400 {"error":"Invalid agent ID"}`,
	"GET /.well-known/agent.json | unknown agent":                                          `404 {"error":"Agent card not found"}`,
	"GET /.well-known/agent.json | agent read fails":                                       `404 {"error":"Agent card not found"}`,
	"GET /.well-known/agent.json | repository fails":                                       `404 {"error":"Agent card not found"}`,
	"POST /api/v1/a2a/verify | malformed JSON":                                             `400 {"error":"Invalid request body","valid":false}`,
	"POST /api/v1/a2a/verify | nonce empty":                                                `400 {"error":"nonce must be 1 to 64 characters of UTF-8 text without a NUL character","valid":false}`,
	"POST /api/v1/a2a/verify | timestamp outside the window":                               `401 {"error":"timestamp outside tolerance window","valid":false}`,
	"POST /api/v1/a2a/verify | invalid signature":                                          `401 {"error":"signature verification failed","valid":false}`,
	"POST /api/v1/a2a/verify | unknown agent":                                              `401 {"error":"agent not found","valid":false}`,
	"POST /api/v1/a2a/verify | suspended agent":                                            `401 {"error":"Agent is not permitted to authenticate (status: suspended)","valid":false}`,
	"POST /api/v1/a2a/verify | agent read fails":                                           `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/verify | nonce already used":                                         `401 {"error":"nonce already used (replay attack)","valid":false}`,
	"POST /api/v1/a2a/verify | repository fails":                                           `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/verify | nonce write fails":                                          `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/agents/:id/card | no organization in context":                        `401 {"error":true,"message":"Organization ID not found in context"}`,
	"POST /api/v1/a2a/agents/:id/card | id not UTF-8":                                      `400 {"error":"Invalid agent ID"}`,
	"POST /api/v1/a2a/agents/:id/card | unknown agent":                                     `404 {"error":"Agent not found"}`,
	"POST /api/v1/a2a/agents/:id/card | agent of another organization":                     `403 {"error":"Access denied"}`,
	"POST /api/v1/a2a/agents/:id/card | agent read fails":                                  `404 {"error":"Agent not found"}`,
	"POST /api/v1/a2a/agents/:id/card | repository fails":                                  `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/agents/:id/card | malformed JSON":                                    `400 {"error":"Invalid request body"}`,
	"POST /api/v1/a2a/agents/:id/card | neither cardUrl nor cardData":                      `400 {"error":"cardUrl or cardData is required"}`,
	"POST /api/v1/a2a/agents/:id/card | card URL refused":                                  `400 {"error":"cardUrl must be a public http or https URL"}`,
	"GET /api/v1/a2a/agents/:id/card | no organization in context":                         `401 {"error":true,"message":"Organization ID not found in context"}`,
	"GET /api/v1/a2a/agents/:id/card | id not UTF-8":                                       `400 {"error":"Invalid agent ID"}`,
	"GET /api/v1/a2a/agents/:id/card | unknown agent":                                      `404 {"error":"not found"}`,
	"GET /api/v1/a2a/agents/:id/card | agent of another organization":                      `404 {"error":"not found"}`,
	"GET /api/v1/a2a/agents/:id/card | agent read fails":                                   `404 {"error":"not found"}`,
	"GET /api/v1/a2a/agents/:id/card | repository fails":                                   `404 {"error":"Agent card not found"}`,
	"POST /api/v1/a2a/agents/:id/card/refresh | no organization in context":                `401 {"error":true,"message":"Organization ID not found in context"}`,
	"POST /api/v1/a2a/agents/:id/card/refresh | id not UTF-8":                              `400 {"error":"Invalid agent ID"}`,
	"POST /api/v1/a2a/agents/:id/card/refresh | unknown agent":                             `404 {"error":"Agent not found"}`,
	"POST /api/v1/a2a/agents/:id/card/refresh | agent of another organization":             `403 {"error":"Access denied"}`,
	"POST /api/v1/a2a/agents/:id/card/refresh | agent read fails":                          `404 {"error":"Agent not found"}`,
	"POST /api/v1/a2a/agents/:id/card/refresh | repository fails":                          `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/agents/:id/sign | no organization in context":                        `401 {"error":true,"message":"Organization ID not found in context"}`,
	"POST /api/v1/a2a/agents/:id/sign | malformed JSON":                                    `400 {"error":"Invalid request body"}`,
	"POST /api/v1/a2a/agents/:id/sign | id not UTF-8":                                      `400 {"error":"Invalid agent ID"}`,
	"POST /api/v1/a2a/agents/:id/sign | unknown agent":                                     `404 {"error":"not found"}`,
	"POST /api/v1/a2a/agents/:id/sign | agent of another organization":                     `404 {"error":"not found"}`,
	"POST /api/v1/a2a/agents/:id/sign | agent read fails":                                  `404 {"error":"not found"}`,
	"POST /api/v1/a2a/agents/:id/sign | repository fails":                                  `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/agents/:id/trust-score | no organization in context":                  `401 {"error":true,"message":"Organization ID not found in context"}`,
	"GET /api/v1/a2a/agents/:id/trust-score | id not UTF-8":                                `400 {"error":"Invalid agent ID"}`,
	"GET /api/v1/a2a/agents/:id/trust-score | unknown agent":                               `404 {"error":"not found"}`,
	"GET /api/v1/a2a/agents/:id/trust-score | agent of another organization":               `404 {"error":"not found"}`,
	"GET /api/v1/a2a/agents/:id/trust-score | agent read fails":                            `404 {"error":"not found"}`,
	"GET /api/v1/a2a/agents/:id/trust-score | repository fails":                            `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/agents/:id/trust-score/compute | no organization in context":         `401 {"error":true,"message":"Organization ID not found in context"}`,
	"POST /api/v1/a2a/agents/:id/trust-score/compute | id not UTF-8":                       `400 {"error":"Invalid agent ID"}`,
	"POST /api/v1/a2a/agents/:id/trust-score/compute | unknown agent":                      `404 {"error":"Agent not found"}`,
	"POST /api/v1/a2a/agents/:id/trust-score/compute | agent of another organization":      `403 {"error":"Access denied"}`,
	"POST /api/v1/a2a/agents/:id/trust-score/compute | agent read fails":                   `404 {"error":"Agent not found"}`,
	"POST /api/v1/a2a/agents/:id/trust-score/compute | repository fails":                   `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/agents/:id/peers/:peer_id/trust | no organization in context":         `401 {"error":true,"message":"Organization ID not found in context"}`,
	"GET /api/v1/a2a/agents/:id/peers/:peer_id/trust | id not UTF-8":                       `400 {"error":"Invalid agent ID"}`,
	"GET /api/v1/a2a/agents/:id/peers/:peer_id/trust | peer_id not UTF-8":                  `400 {"error":"Invalid peer ID"}`,
	"GET /api/v1/a2a/agents/:id/peers/:peer_id/trust | unknown agent":                      `404 {"error":"not found"}`,
	"GET /api/v1/a2a/agents/:id/peers/:peer_id/trust | agent of another organization":      `404 {"error":"not found"}`,
	"GET /api/v1/a2a/agents/:id/peers/:peer_id/trust | agent read fails":                   `404 {"error":"not found"}`,
	"GET /api/v1/a2a/agents/:id/peers/:peer_id/trust | repository fails":                   `404 {"error":"not found"}`,
	"GET /api/v1/a2a/agents/:id/skills | no organization in context":                       `401 {"error":true,"message":"Organization ID not found in context"}`,
	"GET /api/v1/a2a/agents/:id/skills | id not UTF-8":                                     `400 {"error":"Invalid agent ID"}`,
	"GET /api/v1/a2a/agents/:id/skills | unknown agent":                                    `404 {"error":"not found"}`,
	"GET /api/v1/a2a/agents/:id/skills | agent of another organization":                    `404 {"error":"not found"}`,
	"GET /api/v1/a2a/agents/:id/skills | agent read fails":                                 `404 {"error":"not found"}`,
	"GET /api/v1/a2a/agents/:id/skills | repository fails":                                 `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/skills/search | q missing":                                            `400 {"error":"Query parameter 'q' is required"}`,
	"GET /api/v1/a2a/skills/search | q not UTF-8":                                          `400 {"error":"q must be UTF-8 text without a NUL character"}`,
	"GET /api/v1/a2a/skills/search | q holds NUL":                                          `400 {"error":"q must be UTF-8 text without a NUL character"}`,
	"GET /api/v1/a2a/skills/search | repository fails":                                     `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/route | intent missing":                                               `400 {"error":"Query parameter 'intent' is required"}`,
	"GET /api/v1/a2a/route | intent not UTF-8":                                             `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/route | intent holds NUL":                                             `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/route | no agent matches the intent":                                  `404 {"alternatives":0,"error":"No agent found matching intent","intent":"summarize"}`,
	"GET /api/v1/a2a/route | repository fails":                                             `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/capable-of | intent missing":                                          `400 {"error":"Query parameter 'intent' is required"}`,
	"GET /api/v1/a2a/capable-of | intent not UTF-8":                                        `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/capable-of | intent holds NUL":                                        `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/capable-of | repository fails":                                        `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/tasks | no organization in context":                                   `401 {"error":true,"message":"Organization ID not found in context"}`,
	"GET /api/v1/a2a/tasks | agentId not UTF-8":                                            `400 {"error":"Invalid agent ID"}`,
	"GET /api/v1/a2a/tasks | agentId holds NUL":                                            `400 {"error":"Invalid agent ID"}`,
	"GET /api/v1/a2a/tasks | agentId not a UUID":                                           `400 {"error":"Invalid agent ID"}`,
	"GET /api/v1/a2a/tasks | state not UTF-8":                                              `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/tasks | state holds NUL":                                              `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/tasks | repository fails":                                             `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/tasks | no organization in context":                                  `401 {"error":true,"message":"Organization ID not found in context"}`,
	"POST /api/v1/a2a/tasks | malformed JSON":                                              `400 {"error":"Invalid request body"}`,
	"POST /api/v1/a2a/tasks | targetAgentId not a UUID":                                    `400 {"error":"Invalid target agent ID"}`,
	"POST /api/v1/a2a/tasks | clientAgentId not a UUID":                                    `400 {"error":"Invalid client agent ID"}`,
	"POST /api/v1/a2a/tasks | internal clientAgentId not a UUID":                           `400 {"error":"Invalid client agent ID"}`,
	"POST /api/v1/a2a/tasks | no client agent":                                             `400 {"error":"Client agent ID required - provide in request body or context"}`,
	"POST /api/v1/a2a/tasks | no client agent, internal form":                              `400 {"error":"Client agent ID required"}`,
	"POST /api/v1/a2a/tasks | unknown agent":                                               `404 {"error":"not found"}`,
	"POST /api/v1/a2a/tasks | agent of another organization":                               `404 {"error":"not found"}`,
	"POST /api/v1/a2a/tasks | agent read fails":                                            `404 {"error":"not found"}`,
	"POST /api/v1/a2a/tasks | repository fails":                                            `500 {"error":"Failed to log task"}`,
	"PUT /api/v1/a2a/tasks/:id/state | no organization in context":                         `401 {"error":true,"message":"Organization ID not found in context"}`,
	"PUT /api/v1/a2a/tasks/:id/state | id not UTF-8":                                       `400 {"error":"Invalid task ID"}`,
	"PUT /api/v1/a2a/tasks/:id/state | malformed JSON":                                     `400 {"error":"Invalid request body"}`,
	"PUT /api/v1/a2a/tasks/:id/state | state unknown":                                      `400 {"error":"Invalid task state"}`,
	"PUT /api/v1/a2a/tasks/:id/state | unknown task":                                       `404 {"error":"not found"}`,
	"PUT /api/v1/a2a/tasks/:id/state | task read fails":                                    `404 {"error":"not found"}`,
	"PUT /api/v1/a2a/tasks/:id/state | agent of another organization":                      `404 {"error":"not found"}`,
	"PUT /api/v1/a2a/tasks/:id/state | agent read fails":                                   `404 {"error":"not found"}`,
	"PUT /api/v1/a2a/tasks/:id/state | repository fails":                                   `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/consents | no organization in context":                                `401 {"error":true,"message":"Organization ID not found in context"}`,
	"GET /api/v1/a2a/consents | repository fails":                                          `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/consent | no organization in context":                                `401 {"error":true,"message":"Organization ID not found in context"}`,
	"POST /api/v1/a2a/consent | malformed JSON":                                            `400 {"error":"Invalid request body"}`,
	"POST /api/v1/a2a/consent | unknown agent":                                             `404 {"error":"Grantor agent not found"}`,
	"POST /api/v1/a2a/consent | agent of another organization":                             `404 {"error":"Grantor agent not found"}`,
	"POST /api/v1/a2a/consent | agent read fails":                                          `404 {"error":"Grantor agent not found"}`,
	"POST /api/v1/a2a/consent | repository fails":                                          `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/consent/check | no organization in context":                           `401 {"error":true,"message":"Organization ID not found in context"}`,
	"GET /api/v1/a2a/consent/check | parameters missing":                                   `400 {"error":"userId, grantorAgentId, recipientAgentId, and scope are required"}`,
	"GET /api/v1/a2a/consent/check | grantorAgentId not UTF-8":                             `400 {"error":"Invalid grantor agent ID"}`,
	"GET /api/v1/a2a/consent/check | recipientAgentId holds NUL":                           `400 {"error":"Invalid recipient agent ID"}`,
	"GET /api/v1/a2a/consent/check | userId not UTF-8":                                     `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/consent/check | userId holds NUL":                                     `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/consent/check | scope not UTF-8":                                      `400 {"error":"scope must be UTF-8 text without a NUL character"}`,
	"GET /api/v1/a2a/consent/check | scope holds NUL":                                      `400 {"error":"scope must be UTF-8 text without a NUL character"}`,
	"GET /api/v1/a2a/consent/check | unknown agent":                                        `404 {"error":"not found"}`,
	"GET /api/v1/a2a/consent/check | agent of another organization":                        `404 {"error":"not found"}`,
	"GET /api/v1/a2a/consent/check | agent read fails":                                     `404 {"error":"not found"}`,
	"GET /api/v1/a2a/consent/check | repository fails":                                     `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/consent/:id/revoke | no organization in context":                     `401 {"error":true,"message":"Organization ID not found in context"}`,
	"POST /api/v1/a2a/consent/:id/revoke | id not UTF-8":                                   `400 {"error":"Invalid consent ID"}`,
	"POST /api/v1/a2a/consent/:id/revoke | unknown consent":                                `404 {"error":"not found"}`,
	"POST /api/v1/a2a/consent/:id/revoke | consent of another organization":                `404 {"error":"not found"}`,
	"POST /api/v1/a2a/consent/:id/revoke | consent read fails":                             `404 {"error":"not found"}`,
	"POST /api/v1/a2a/consent/:id/revoke | malformed JSON":                                 `400 {"error":"Invalid request body"}`,
	"POST /api/v1/a2a/consent/:id/revoke | repository fails":                               `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/consent/user/:userId | no organization in context":                    `401 {"error":true,"message":"Organization ID not found in context"}`,
	"GET /api/v1/a2a/consent/user/:userId | userId not UTF-8":                              `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/consent/user/:userId | repository fails":                              `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/trust-scores | no organization in context":                            `401 {"error":true,"message":"Organization ID not found in context"}`,
	"GET /api/v1/a2a/trust-scores | repository fails":                                      `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/policies/evaluate | no organization in context":                      `401 {"error":true,"message":"Organization ID not found in context"}`,
	"POST /api/v1/a2a/policies/evaluate | malformed JSON":                                  `400 {"error":"Invalid request body"}`,
	"POST /api/v1/a2a/policies/evaluate | organizationId of another organization":          `403 {"error":"organizationId must be your own organization; leave it out to evaluate your organization's policies"}`,
	"POST /api/v1/a2a/policies/evaluate | repository fails":                                `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/skills | malformed JSON":                                             `400 {"error":"Invalid request body"}`,
	"POST /api/v1/a2a/skills | no agent":                                                   `400 {"error":"Invalid or missing agent ID"}`,
	"POST /api/v1/a2a/skills | unknown agent":                                              `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/skills | agent read fails":                                           `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/skills | repository fails":                                           `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/attestations | malformed JSON":                                       `400 {"error":"Invalid request body"}`,
	"POST /api/v1/a2a/attestations | attestedAgentId not a UUID":                           `400 {"error":"Invalid attested agent ID"}`,
	"POST /api/v1/a2a/attestations | attestingAgentId not a UUID":                          `400 {"error":"Invalid attesting agent ID"}`,
	"POST /api/v1/a2a/attestations | no attesting agent":                                   `401 {"error":"Could not determine attesting agent - provide attestingAgentId in request"}`,
	"POST /api/v1/a2a/attestations | unknown agent":                                        `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/attestations | agent read fails":                                     `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/attestations | repository fails":                                     `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/attestations/:agentId/:skillId | no organization in context":          `401 {"error":true,"message":"Organization ID not found in context"}`,
	"GET /api/v1/a2a/attestations/:agentId/:skillId | agentId not UTF-8":                   `400 {"error":"Invalid agent ID"}`,
	"GET /api/v1/a2a/attestations/:agentId/:skillId | skillId not UTF-8":                   `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/attestations/:agentId/:skillId | unknown agent":                       `404 {"error":"not found"}`,
	"GET /api/v1/a2a/attestations/:agentId/:skillId | agent of another organization":       `404 {"error":"not found"}`,
	"GET /api/v1/a2a/attestations/:agentId/:skillId | agent read fails":                    `404 {"error":"not found"}`,
	"GET /api/v1/a2a/attestations/:agentId/:skillId | repository fails":                    `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/consensus/:agentId/:skillId | no organization in context":             `401 {"error":true,"message":"Organization ID not found in context"}`,
	"GET /api/v1/a2a/consensus/:agentId/:skillId | agentId not UTF-8":                      `400 {"error":"Invalid agent ID"}`,
	"GET /api/v1/a2a/consensus/:agentId/:skillId | skillId not UTF-8":                      `400 {"error":"skillId must be UTF-8 text without a NUL character"}`,
	"GET /api/v1/a2a/consensus/:agentId/:skillId | unknown agent":                          `404 {"error":"not found"}`,
	"GET /api/v1/a2a/consensus/:agentId/:skillId | agent of another organization":          `404 {"error":"not found"}`,
	"GET /api/v1/a2a/consensus/:agentId/:skillId | agent read fails":                       `404 {"error":"not found"}`,
	"GET /api/v1/a2a/consensus/:agentId/:skillId | repository fails":                       `200 {"skillId":"summarize","agentId":"0a000000-0000-4000-8000-0000000000a1","attestationCount":0,"uniqueAttesters":0,"uniqueOwners":0,"confidenceScore":0,"isVerified":false,"verifiedAt":"0001-01-01T00:00:00Z"}`,
	"POST /api/v1/a2a/security/check | malformed JSON":                                     `400 {"error":"Invalid request body"}`,
	"POST /api/v1/a2a/security/check | no requesting agent":                                `400 {"error":"Invalid requesting agent ID"}`,
	"POST /api/v1/a2a/security/check | targetAgentId not a UUID":                           `400 {"error":"Invalid target agent ID"}`,
	"POST /api/v1/a2a/security/check | repository fails":                                   `200 {"allowed":true,"violations":["UNSIGNED_REQUEST"],"mode":"monitor"}`,
	"GET /api/v1/a2a/security/settings | no organization in context":                       `401 {"error":"Could not determine organization"}`,
	"GET /api/v1/a2a/security/settings | repository fails":                                 `200 {"id":"00000000-0000-0000-0000-000000000000","organizationId":"0a000000-0000-4000-8000-000000000001","enforcementMode":"monitor","requireVerifiedSkills":false,"minTrustScore":0,"minAttestationCount":0,"requireRequestSigning":true,"requireNonceValidation":true,"nonceValiditySeconds":300,"maxRequestsPerMinute":60,"maxRequestsPerHour":1000,"createdAt":"0001-01-01T00:00:00Z","updatedAt":"0001-01-01T00:00:00Z"}`,
	"PUT /api/v1/a2a/security/settings | no organization in context":                       `401 {"error":"Could not determine organization"}`,
	"PUT /api/v1/a2a/security/settings | malformed JSON":                                   `400 {"error":"Invalid request body"}`,
	"PUT /api/v1/a2a/security/settings | repository fails":                                 `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/sign | malformed JSON":                                               `400 {"error":"Invalid request body"}`,
	"POST /api/v1/a2a/sign | no agent":                                                     `400 {"error":"Invalid or missing agent ID"}`,
	"POST /api/v1/a2a/sign | unknown agent":                                                `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/sign | agent read fails":                                             `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/sign | repository fails":                                             `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/trust/:id | no organization in context":                               `401 {"error":true,"message":"Organization ID not found in context"}`,
	"GET /api/v1/a2a/trust/:id | id not UTF-8":                                             `400 {"error":"Invalid agent ID"}`,
	"GET /api/v1/a2a/trust/:id | unknown agent":                                            `404 {"error":"not found"}`,
	"GET /api/v1/a2a/trust/:id | agent of another organization":                            `404 {"error":"not found"}`,
	"GET /api/v1/a2a/trust/:id | agent read fails":                                         `404 {"error":"not found"}`,
	"GET /api/v1/a2a/trust/:id | repository fails":                                         `500 {"error":"An internal error occurred. Please try again later."}`,
	"PUT /api/v1/a2a/trust/:id | a score written":                                          `405 {"error":"the A2A trust score is measured from recorded interactions, not asserted; there is no write for it","read":"GET /api/v1/a2a/trust/:id","record":"POST /api/v1/a2a/trust/:id/interaction"}`,
	"POST /api/v1/a2a/trust/:id/interaction | no organization in context":                  `401 {"error":true,"message":"Organization ID not found in context"}`,
	"POST /api/v1/a2a/trust/:id/interaction | malformed JSON":                              `400 {"error":"Invalid request body"}`,
	"POST /api/v1/a2a/trust/:id/interaction | no agent":                                    `401 {"error":"Agent ID required"}`,
	"POST /api/v1/a2a/trust/:id/interaction | id not UTF-8":                                `400 {"error":"Invalid agent ID"}`,
	"POST /api/v1/a2a/trust/:id/interaction | unknown agent":                               `404 {"error":"not found"}`,
	"POST /api/v1/a2a/trust/:id/interaction | agent of another organization":               `404 {"error":"not found"}`,
	"POST /api/v1/a2a/trust/:id/interaction | agent read fails":                            `404 {"error":"not found"}`,
	"POST /api/v1/a2a/trust/:id/interaction | repository fails":                            `500 {"error":"Failed to compute trust score"}`,
	"POST /api/v1/a2a/discovery/route | malformed JSON":                                    `400 {"error":"Invalid request body"}`,
	"POST /api/v1/a2a/discovery/route | intent missing":                                    `400 {"error":"Intent is required"}`,
	"POST /api/v1/a2a/discovery/route | repository fails":                                  `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/discovery/capable | malformed JSON":                                  `400 {"error":"Invalid request body"}`,
	"POST /api/v1/a2a/discovery/capable | repository fails":                                `500 {"error":"Failed to find capable agents"}`,
	"GET /api/v1/a2a/cards | no organization in context":                                   `401 {"error":true,"message":"Organization ID not found in context"}`,
	"GET /api/v1/a2a/cards | repository fails":                                             `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/cards | no organization in context":                                  `401 {"error":true,"message":"Organization ID not found in context"}`,
	"POST /api/v1/a2a/cards | malformed JSON":                                              `400 {"error":"Invalid request body"}`,
	"POST /api/v1/a2a/cards | agentId not a UUID":                                          `400 {"error":"Invalid agent ID"}`,
	"POST /api/v1/a2a/cards | another agent's ID":                                          `403 {"error":"An agent may only act on its own agent ID on this route","reasonCode":"agent_path_mismatch"}`,
	"POST /api/v1/a2a/cards | no agent":                                                    `401 {"error":"Agent ID required"}`,
	"POST /api/v1/a2a/cards | unknown agent":                                               `404 {"error":"not found"}`,
	"POST /api/v1/a2a/cards | agent of another organization":                               `404 {"error":"not found"}`,
	"POST /api/v1/a2a/cards | agent read fails":                                            `404 {"error":"not found"}`,
	"POST /api/v1/a2a/cards | card URL refused":                                            `400 {"error":"cardUrl must be a public http or https URL"}`,
	"GET /api/v1/a2a/cards/:id | no organization in context":                               `401 {"error":true,"message":"Organization ID not found in context"}`,
	"GET /api/v1/a2a/cards/:id | id not UTF-8":                                             `400 {"error":"Invalid agent ID"}`,
	"GET /api/v1/a2a/cards/:id | unknown agent":                                            `404 {"error":"not found"}`,
	"GET /api/v1/a2a/cards/:id | agent of another organization":                            `404 {"error":"not found"}`,
	"GET /api/v1/a2a/cards/:id | agent read fails":                                         `404 {"error":"not found"}`,
	"GET /api/v1/a2a/cards/:id | repository fails":                                         `404 {"error":"Agent card not found"}`,
	"PUT /api/v1/a2a/cards/:id | no organization in context":                               `401 {"error":true,"message":"Organization ID not found in context"}`,
	"PUT /api/v1/a2a/cards/:id | malformed JSON":                                           `400 {"error":"Invalid request body"}`,
	"PUT /api/v1/a2a/cards/:id | id not UTF-8":                                             `400 {"error":"Invalid agent ID"}`,
	"PUT /api/v1/a2a/cards/:id | unknown agent":                                            `404 {"error":"not found"}`,
	"PUT /api/v1/a2a/cards/:id | agent of another organization":                            `404 {"error":"not found"}`,
	"PUT /api/v1/a2a/cards/:id | agent read fails":                                         `404 {"error":"not found"}`,
	"PUT /api/v1/a2a/cards/:id | repository fails":                                         `500 {"error":"An internal error occurred. Please try again later."}`,
	"PUT /api/v1/a2a/cards/:id | neither cardUrl nor cardData":                             `400 {"error":"cardUrl or cardData is required: this route replaces the agent card document and does not edit fields"}`,
	"PUT /api/v1/a2a/cards/:id | field members without a card document":                    `400 {"error":"cardUrl or cardData is required: this route replaces the agent card document and does not edit fields; it does not read name","unreadMembers":["name"]}`,
	"PUT /api/v1/a2a/cards/:id | card URL refused":                                         `400 {"error":"cardUrl must be a public http or https URL"}`,
	"POST /api/v1/a2a/cards/:id/attestation | no organization in context":                  `401 {"error":true,"message":"Organization ID not found in context"}`,
	"POST /api/v1/a2a/cards/:id/attestation | id not UTF-8":                                `400 {"error":"Invalid agent ID"}`,
	"POST /api/v1/a2a/cards/:id/attestation | unknown agent":                               `404 {"error":"Agent not found"}`,
	"POST /api/v1/a2a/cards/:id/attestation | agent of another organization":               `403 {"error":"Access denied"}`,
	"POST /api/v1/a2a/cards/:id/attestation | agent read fails":                            `404 {"error":"Agent not found"}`,
	"POST /api/v1/a2a/cards/:id/attestation | repository fails":                            `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/peers | no agent":                                                     `401 {"error":"Agent ID required"}`,
	"GET /api/v1/a2a/peers | repository fails":                                             `500 {"error":"An internal error occurred. Please try again later."}`,
	"DELETE /api/v1/a2a/skills/:id | id not UTF-8":                                         `400 {"error":"id must be UTF-8 text without a NUL character"}`,
	"GET /api/v1/a2a/security/violations | no organization in context":                     `401 {"error":"Organization ID required"}`,
	"GET /api/v1/a2a/security/violations | repository fails":                               `500 {"error":"Failed to get security violations"}`,
	"GET /api/v1/a2a/agents/:id/attestations | no organization in context":                 `401 {"error":true,"message":"Organization ID not found in context"}`,
	"GET /api/v1/a2a/agents/:id/attestations | skillId not UTF-8":                          `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/agents/:id/attestations | skillId holds NUL":                          `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/agents/:id/attestations | id not UTF-8":                               `400 {"error":"Invalid agent ID"}`,
	"GET /api/v1/a2a/agents/:id/attestations | unknown agent":                              `404 {"error":"not found"}`,
	"GET /api/v1/a2a/agents/:id/attestations | agent of another organization":              `404 {"error":"not found"}`,
	"GET /api/v1/a2a/agents/:id/attestations | agent read fails":                           `404 {"error":"not found"}`,
	"GET /api/v1/a2a/agents/:id/attestations | repository fails":                           `500 {"error":"An internal error occurred. Please try again later."}`,
	"GET /api/v1/a2a/agents/:id/skills/:skillId/consensus | no organization in context":    `401 {"error":true,"message":"Organization ID not found in context"}`,
	"GET /api/v1/a2a/agents/:id/skills/:skillId/consensus | skillId not UTF-8":             `400 {"error":"skillId must be UTF-8 text without a NUL character"}`,
	"GET /api/v1/a2a/agents/:id/skills/:skillId/consensus | id not UTF-8":                  `400 {"error":"Invalid agent ID"}`,
	"GET /api/v1/a2a/agents/:id/skills/:skillId/consensus | unknown agent":                 `404 {"error":"not found"}`,
	"GET /api/v1/a2a/agents/:id/skills/:skillId/consensus | agent of another organization": `404 {"error":"not found"}`,
	"GET /api/v1/a2a/agents/:id/skills/:skillId/consensus | agent read fails":              `404 {"error":"not found"}`,
	"GET /api/v1/a2a/agents/:id/skills/:skillId/consensus | repository fails":              `200 {"skillId":"summarize","agentId":"0a000000-0000-4000-8000-0000000000a1","attestationCount":0,"uniqueAttesters":0,"uniqueOwners":0,"confidenceScore":0,"isVerified":false,"verifiedAt":"0001-01-01T00:00:00Z"}`,
	"POST /api/v1/a2a/maintenance/cleanup-nonces | repository fails":                       `500 {"error":"An internal error occurred. Please try again later."}`,
	"POST /api/v1/a2a/maintenance/refresh-cards | repository fails":                        `500 {"error":"An internal error occurred. Please try again later."}`,
}
