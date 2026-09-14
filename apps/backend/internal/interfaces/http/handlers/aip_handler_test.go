package handlers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aip_routes_test.go pins the MOUNTING of the two AIP endpoints and says in its own
// header comment that it deliberately does not exercise handler bodies. That left the
// resolver's response body — the part served unauthenticated to the whole internet —
// with no test at all, which is how it came to publish `agents.updated_at`, reference
// a key the document does not contain, and resolve agents that were never verified.
//
// This file is that missing test. It resolves through a fake domain.AgentRepository so
// the body can be asserted directly, and it is written as a LEAK test: the interesting
// assertions are about what is absent.

// aipTestAgentRepo is a domain.AgentRepository stub implementing only GetByID, the
// single method ResolveDID calls. The remaining methods are inherited from the
// nil-embedded interface and panic if reached — so a resolver that starts reading
// anything else fails loudly rather than silently widening what it discloses.
type aipTestAgentRepo struct {
	domain.AgentRepository
	getByID func(id uuid.UUID) (*domain.Agent, error)
}

func (r *aipTestAgentRepo) GetByID(id uuid.UUID) (*domain.Agent, error) {
	return r.getByID(id)
}

// resolve mounts ResolveDID the way cmd/server/main.go does — on the /api/v1 group,
// under the global security-headers middleware — and resolves did:aip:aim_<id>.
// lookup stands in for the repository, so "unknown id" and "id found, agent in state
// X" are both expressible against the same DID string.
func resolve(t *testing.T, id uuid.UUID, lookup func(uuid.UUID) (*domain.Agent, error)) (*http.Response, string) {
	t.Helper()

	app := fiber.New()
	app.Use(middleware.SecurityHeadersMiddleware())
	h := NewAIPHandler(&aipTestAgentRepo{getByID: lookup})
	v1 := app.Group("/api/v1")
	v1.Get("/did/*", h.ResolveDID)

	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/did/"+domain.BuildAgentDID(id), nil))
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, string(body)
}

// resolveAgent resolves an agent that exists, and returns the decoded body alongside
// the raw JSON text (the leak assertions are made against the text).
func resolveAgent(t *testing.T, agent *domain.Agent) (*http.Response, map[string]any, string) {
	t.Helper()

	resp, raw := resolve(t, agent.ID, func(id uuid.UUID) (*domain.Agent, error) {
		if id != agent.ID {
			return nil, errors.New("sql: no rows in result set")
		}
		return agent, nil
	})

	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &doc), "resolution body must be JSON:\n%s", raw)
	return resp, doc, raw
}

func didDocumentOf(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	d, ok := doc["didDocument"].(map[string]any)
	require.True(t, ok, "resolution response must carry a didDocument object")
	return d
}

func didMetadataOf(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	m, ok := doc["didDocumentMetadata"].(map[string]any)
	require.True(t, ok, "resolution response must carry a didDocumentMetadata object")
	return m
}

// strings extracts a JSON array of strings, requiring it to be present and an array
// (a missing key and an empty list are different answers, and the difference is the
// point for a keyless agent).
func stringList(t *testing.T, m map[string]any, key string) []string {
	t.Helper()
	raw, present := m[key]
	require.True(t, present, "%s must be present in the DID document", key)
	arr, ok := raw.([]any)
	require.True(t, ok, "%s must be a JSON array, got %T", key, raw)
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		s, ok := v.(string)
		require.True(t, ok, "%s entries must be strings, got %T", key, v)
		out = append(out, s)
	}
	return out
}

const (
	aipTestAgentID = "3f2504e0-4f89-11d3-9a0c-0305e82c3301"
	aipTestOrgID   = "6b1a9d34-2e7c-4f10-b8ad-9c5e0f3a7d21"
)

var (
	// Distinct, non-overlapping instants so an assertion can name exactly which one
	// the resolver served. updatedAt is deliberately the LATEST of the four: an
	// implementation that publishes the most recent timestamp it can find still fails.
	aipCreatedAt    = time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	aipKeyCreatedAt = time.Date(2026, 3, 14, 10, 30, 0, 0, time.UTC)
	aipPQCCreatedAt = time.Date(2026, 4, 1, 8, 15, 0, 0, time.UTC)
	aipUpdatedAt    = time.Date(2026, 8, 29, 23, 45, 0, 0, time.UTC)
	aipHeartbeatAt  = time.Date(2026, 9, 1, 6, 0, 0, 0, time.UTC)
)

func ptr[T any](v T) *T { return &v }

// populatedAgent is an agent row with every field the DPO's ruling names filled in
// with a value that is recognisable in a response body. Anything here that reaches
// the wire is a disclosure about the agent's operator made to an unauthenticated
// caller who knew only a UUID.
func populatedAgent() *domain.Agent {
	return &domain.Agent{
		ID:              uuid.MustParse(aipTestAgentID),
		OrganizationID:  uuid.MustParse(aipTestOrgID),
		Name:            "payroll-reconciler",
		DisplayName:     "Payroll Reconciler",
		Description:     "Reconciles payroll ledgers nightly",
		AgentType:       domain.AgentTypeClaude,
		Status:          domain.AgentStatusVerified,
		Version:         "4.2.1",
		PublicKey:       ptr(base64.StdEncoding.EncodeToString([]byte("ed25519-key-material-for-the-aip-test"))),
		PQCPublicKey:    ptr(base64.StdEncoding.EncodeToString([]byte("ml-dsa-65-key-material-for-the-aip-test"))),
		PQCKeyAlgorithm: ptr("ML-DSA-65"),
		KeyCreatedAt:    &aipKeyCreatedAt,
		PQCKeyCreatedAt: &aipPQCCreatedAt,
		CreatedAt:       aipCreatedAt,
		UpdatedAt:       aipUpdatedAt,
		CreatedByName:   "Treasury Operations",
		CreatedByEmail:  "treasury-ops@tenant-example.invalid",
		LastHeartbeat:   &aipHeartbeatAt,
		LastActive:      &aipHeartbeatAt,
		TrustScore:      66.25,
		Capabilities:    []string{"db:read", "ledger:reconcile"},
		TalksTo:         []string{"mcp-ledger-server"},
		DeclaredPurpose: &domain.DeclaredPurpose{
			Category:   "finance.reconciliation",
			Statement:  "Nightly reconciliation of the payroll ledger",
			TaskScopes: []string{"ledger:reconcile"},
		},
	}
}

// keylessAgent is a verified agent that has not yet published key material — the
// state every agent passes through between verification and first key generation.
func keylessAgent() *domain.Agent {
	a := populatedAgent()
	a.PublicKey = nil
	a.PQCPublicKey = nil
	a.PQCKeyAlgorithm = nil
	a.KeyCreatedAt = nil
	a.PQCKeyCreatedAt = nil
	return a
}

// ---------------------------------------------------------------------------
// AIMC-06.AC3 — the resolver discloses key material and nothing else.
// ---------------------------------------------------------------------------

func TestResolveDIDDoesNotDiscloseAgentRowFields(t *testing.T) {
	agent := populatedAgent()
	resp, doc, raw := resolveAgent(t, agent)
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "a verified agent must resolve")

	t.Run("AIMC-06.AC3 no value from the agent row other than key material reaches the body", func(t *testing.T) {
		// value -> the field it would disclose. Every one of these is administrative
		// information about the agent's operator, available to a caller who knows
		// only the agent's UUID.
		forbidden := map[string]string{
			agent.Name:                               "agents.name",
			agent.DisplayName:                        "agents.display_name",
			agent.Description:                        "agents.description",
			string(agent.AgentType):                  "agents.agent_type",
			agent.Version:                            "agents.version",
			agent.CreatedByEmail:                     "agents.created_by_email",
			agent.CreatedByName:                      "agents.created_by_name",
			agent.OrganizationID.String():            "agents.organization_id",
			string(agent.Status):                     "agents.status",
			agent.UpdatedAt.Format(time.RFC3339):     "agents.updated_at",
			agent.LastHeartbeat.Format(time.RFC3339): "agents.last_heartbeat",
			fmt.Sprintf("%g", agent.TrustScore):      "agents.trust_score",
			agent.Capabilities[0]:                    "agents.capabilities",
			agent.Capabilities[1]:                    "agents.capabilities",
			agent.TalksTo[0]:                         "agents.talks_to",
			agent.DeclaredPurpose.Category:           "agents.declared_purpose.category",
			agent.DeclaredPurpose.Statement:          "agents.declared_purpose.statement",
		}
		for value, field := range forbidden {
			assert.NotContains(t, raw, value,
				"%s reached an unauthenticated DID resolution as %q; the DID document may carry key material only",
				field, value)
		}
	})

	t.Run("AIMC-06.AC3 no key naming an agent-row attribute appears in the body", func(t *testing.T) {
		// `type` is legitimate inside a verificationMethod (the key suite) and inside
		// a service entry (the service class); anywhere else it would be the agent's
		// type. So this is asserted over key PATHS, not over the raw text.
		banned := map[string]bool{
			"name": true, "org": true, "owner": true, "type": true,
			"capabilities": true, "trust": true, "status": true,
			"last_active": true, "lastHeartbeat": true,
		}
		typeIsAllowedUnder := func(path string) bool {
			return strings.HasPrefix(path, "didDocument.verificationMethod[") ||
				strings.HasPrefix(path, "didDocument.service[")
		}
		for _, kp := range keyPaths(doc, "") {
			if !banned[kp.key] {
				continue
			}
			if kp.key == "type" && typeIsAllowedUnder(kp.path) {
				continue
			}
			t.Errorf("DID resolution body carries key %q at %s; that names an agent-row "+
				"attribute, not key material", kp.key, kp.path)
		}
	})

	t.Run("AIMC-06.AC3 the served updated is key-derived, not agents.updated_at", func(t *testing.T) {
		// The later of the two key timestamps: the PQC key was generated after the
		// Ed25519 one, so it is the last change to the DOCUMENT.
		want := aipPQCCreatedAt.Format(time.RFC3339)
		assert.Equal(t, want, didDocumentOf(t, doc)["updated"],
			"didDocument.updated must be the latest key-material timestamp")
		assert.Equal(t, want, didMetadataOf(t, doc)["updated"],
			"didDocumentMetadata.updated must be the latest key-material timestamp")
		assert.NotEqual(t, agent.UpdatedAt.Format(time.RFC3339), didDocumentOf(t, doc)["updated"],
			"agents.updated_at moves on every row write (a rename, a tag, a trust recompute); "+
				"publishing it turns the resolver into an activity feed for the agent's operator")
	})
}

func TestResolveDIDStatusHandling(t *testing.T) {
	t.Run("AIMC-06.AC3 a pending agent is indistinguishable from an unknown one", func(t *testing.T) {
		agent := populatedAgent()
		agent.Status = domain.AgentStatusPending

		pendingResp, pendingBody := resolve(t, agent.ID, func(uuid.UUID) (*domain.Agent, error) {
			return agent, nil
		})
		unknownResp, unknownBody := resolve(t, agent.ID, func(uuid.UUID) (*domain.Agent, error) {
			return nil, errors.New("sql: no rows in result set")
		})

		assert.Equal(t, fiber.StatusNotFound, pendingResp.StatusCode,
			"a pending agent has registered but has not been verified; resolving it would let "+
				"anyone who can register publish a DID document from this provider")
		assert.Equal(t, unknownResp.StatusCode, pendingResp.StatusCode)
		assert.JSONEq(t, unknownBody, pendingBody,
			"the pending body must be byte-for-byte the unknown-DID body, or the difference "+
				"is an oracle for 'this UUID is registered but unverified'")

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(pendingBody), &body))
		assert.Equal(t, "did_not_found", body["error"])
	})

	t.Run("AIMC-06.AC2 a suspended agent resolves deactivated", func(t *testing.T) {
		agent := populatedAgent()
		agent.Status = domain.AgentStatusSuspended

		resp, doc, _ := resolveAgent(t, agent)
		require.Equal(t, fiber.StatusOK, resp.StatusCode,
			"suspension does not unpublish the document; it marks the key material untrusted")
		assert.Equal(t, true, didMetadataOf(t, doc)["deactivated"],
			"a suspended agent may not authenticate (domain.AgentStatusPermitsAuth), so a "+
				"relying party reading this document must not accept its keys")
	})

	t.Run("AIMC-06.AC2 a revoked agent resolves deactivated", func(t *testing.T) {
		agent := populatedAgent()
		agent.Status = domain.AgentStatusRevoked

		_, doc, _ := resolveAgent(t, agent)
		assert.Equal(t, true, didMetadataOf(t, doc)["deactivated"])
	})

	t.Run("AIMC-06.AC2 a verified agent resolves active", func(t *testing.T) {
		_, doc, _ := resolveAgent(t, populatedAgent())
		assert.Equal(t, false, didMetadataOf(t, doc)["deactivated"])
	})

	t.Run("AIMC-06.AC2 an unrecognised status fails closed as deactivated", func(t *testing.T) {
		// agents.status is a plain VARCHAR(50) with no CHECK constraint, so a value
		// outside the four domain constants is storable. A deny-list on {revoked,
		// suspended} would publish such a row as live key material.
		agent := populatedAgent()
		agent.Status = domain.AgentStatus("quarantined")

		_, doc, _ := resolveAgent(t, agent)
		assert.Equal(t, true, didMetadataOf(t, doc)["deactivated"])
	})
}

// ---------------------------------------------------------------------------
// AIMC-06.AC2 — key references are derived from the keys actually emitted.
// ---------------------------------------------------------------------------

func TestResolveDIDKeyReferences(t *testing.T) {
	did := domain.BuildAgentDID(uuid.MustParse(aipTestAgentID))

	t.Run("AIMC-06.AC3 a keyless agent references no keys", func(t *testing.T) {
		_, doc, raw := resolveAgent(t, keylessAgent())
		document := didDocumentOf(t, doc)

		assert.Empty(t, stringList(t, document, "verificationMethod"))
		assert.Empty(t, stringList(t, document, "authentication"),
			"authentication listed %s#key-1 for an agent that publishes no key: the document "+
				"pointed at a verification method it did not contain", did)
		assert.Empty(t, stringList(t, document, "assertionMethod"))
		assert.NotContains(t, raw, "#key-1",
			"no key fragment may appear anywhere in the body of a keyless agent's document")
	})

	t.Run("AIMC-06.AC2 an Ed25519-only agent references only #key-1", func(t *testing.T) {
		agent := keylessAgent()
		agent.PublicKey = ptr(base64.StdEncoding.EncodeToString([]byte("ed25519-only")))
		agent.KeyCreatedAt = &aipKeyCreatedAt

		_, doc, _ := resolveAgent(t, agent)
		document := didDocumentOf(t, doc)

		assert.Equal(t, []string{did + "#key-1"}, stringList(t, document, "authentication"))
		assert.Equal(t, []string{did + "#key-1"}, stringList(t, document, "assertionMethod"))
	})

	t.Run("AIMC-06.AC2 a PQC-only agent references only #pqc-key-1", func(t *testing.T) {
		agent := keylessAgent()
		agent.PQCPublicKey = ptr(base64.StdEncoding.EncodeToString([]byte("pqc-only")))
		agent.PQCKeyAlgorithm = ptr("ML-DSA-87")
		agent.PQCKeyCreatedAt = &aipPQCCreatedAt

		_, doc, raw := resolveAgent(t, agent)
		document := didDocumentOf(t, doc)

		assert.Equal(t, []string{did + "#pqc-key-1"}, stringList(t, document, "authentication"))
		assert.Equal(t, []string{did + "#pqc-key-1"}, stringList(t, document, "assertionMethod"))
		assert.NotContains(t, raw, "#key-1\"",
			"the Ed25519 fragment must not be referenced by an agent that has no Ed25519 key")
	})

	t.Run("AIMC-06.AC2 an agent with both keys references both, in emission order", func(t *testing.T) {
		_, doc, _ := resolveAgent(t, populatedAgent())
		document := didDocumentOf(t, doc)

		want := []string{did + "#key-1", did + "#pqc-key-1"}
		assert.Equal(t, want, stringList(t, document, "authentication"))
		assert.Equal(t, want, stringList(t, document, "assertionMethod"))

		// Every reference must resolve inside this same document.
		methods, ok := document["verificationMethod"].([]any)
		require.True(t, ok)
		emitted := make([]string, 0, len(methods))
		for _, m := range methods {
			emitted = append(emitted, m.(map[string]any)["id"].(string))
		}
		assert.Equal(t, want, emitted)
	})
}

// ---------------------------------------------------------------------------
// AIMC-06.AC2 — `updated` is derived from key material in every combination.
// ---------------------------------------------------------------------------

func TestResolveDIDUpdatedIsDerivedFromKeyMaterial(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*domain.Agent)
		want    time.Time
		because string
	}{
		{
			name:    "AIMC-06.AC2 both key timestamps set: the later one wins",
			mutate:  func(a *domain.Agent) {},
			want:    aipPQCCreatedAt,
			because: "the PQC key was generated last, so it is the last change to the document",
		},
		{
			name: "AIMC-06.AC2 both key timestamps set in the other order: still the later one",
			mutate: func(a *domain.Agent) {
				later := aipUpdatedAt.Add(-time.Hour)
				a.KeyCreatedAt = &later
			},
			want:    aipUpdatedAt.Add(-time.Hour),
			because: "the rule is 'the later of the two', not 'the PQC one'",
		},
		{
			name: "AIMC-06.AC2 only the Ed25519 key timestamp is set",
			mutate: func(a *domain.Agent) {
				a.PQCPublicKey, a.PQCKeyAlgorithm, a.PQCKeyCreatedAt = nil, nil, nil
			},
			want:    aipKeyCreatedAt,
			because: "one key set means one candidate",
		},
		{
			name: "AIMC-06.AC2 only the PQC key timestamp is set",
			mutate: func(a *domain.Agent) {
				a.PublicKey, a.KeyCreatedAt = nil, nil
			},
			want:    aipPQCCreatedAt,
			because: "a PQC-only agent's document changed when the PQC key was generated",
		},
		{
			name: "AIMC-06.AC2 neither key timestamp is set: creation, never agents.updated_at",
			mutate: func(a *domain.Agent) {
				a.KeyCreatedAt, a.PQCKeyCreatedAt = nil, nil
			},
			want:    aipCreatedAt,
			because: "with no key material there is nothing that has changed since creation",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agent := populatedAgent()
			tc.mutate(agent)

			_, doc, _ := resolveAgent(t, agent)
			want := tc.want.Format(time.RFC3339)

			assert.Equal(t, want, didDocumentOf(t, doc)["updated"], tc.because)
			assert.Equal(t, want, didMetadataOf(t, doc)["updated"], tc.because)
			assert.Equal(t, aipCreatedAt.Format(time.RFC3339), didDocumentOf(t, doc)["created"],
				"created stays the agent's creation instant")
			assert.NotEqual(t, agent.UpdatedAt.Format(time.RFC3339), didDocumentOf(t, doc)["updated"],
				"agents.updated_at is not an allowed source for the published `updated`")
		})
	}
}

// ---------------------------------------------------------------------------
// AIMC-06.AC2 — the emitted key set is an allow-list, and resolution is uncached.
// ---------------------------------------------------------------------------

func TestResolveDIDEmitsOnlyAllowlistedKeys(t *testing.T) {
	_, doc, _ := resolveAgent(t, populatedAgent())
	document := didDocumentOf(t, doc)

	// An allow-list, not a deny-list: the failure this closes is a FUTURE field
	// added to the document by someone who did not read the ruling. A deny-list
	// would admit it.
	assertKeysWithin := func(t *testing.T, where string, m map[string]any, allowed ...string) {
		t.Helper()
		ok := map[string]bool{}
		for _, k := range allowed {
			ok[k] = true
		}
		for k := range m {
			assert.True(t, ok[k],
				"%s carries key %q, which is outside the ruled allow-list %v; every key served "+
					"from this unauthenticated route must be key material or a DID-core wrapper",
				where, k, allowed)
		}
	}

	t.Run("AIMC-06.AC2 top-level keys are the resolution wrappers only", func(t *testing.T) {
		assertKeysWithin(t, "the resolution response", doc,
			"@context", "didDocument", "didDocumentMetadata", "didResolutionMetadata")
	})

	t.Run("AIMC-06.AC2 document keys are within the allow-list", func(t *testing.T) {
		assertKeysWithin(t, "didDocument", document,
			"@context", "id", "verificationMethod", "authentication", "assertionMethod",
			"service", "created", "updated", "deactivated")
	})

	t.Run("AIMC-06.AC2 document metadata keys are within the allow-list", func(t *testing.T) {
		assertKeysWithin(t, "didDocumentMetadata", didMetadataOf(t, doc),
			"created", "updated", "deactivated")
	})

	t.Run("AIMC-06.AC2 each verification method carries only the four DID-core key fields", func(t *testing.T) {
		methods, ok := document["verificationMethod"].([]any)
		require.True(t, ok)
		require.NotEmpty(t, methods)
		for i, m := range methods {
			entry, ok := m.(map[string]any)
			require.True(t, ok)
			assertKeysWithin(t, fmt.Sprintf("verificationMethod[%d]", i), entry,
				"id", "type", "controller", "publicKeyMultibase")
		}
	})

	t.Run("AIMC-06.AC2 each service entry carries only id, type and a relative endpoint", func(t *testing.T) {
		services, ok := document["service"].([]any)
		require.True(t, ok)
		require.NotEmpty(t, services)
		for i, s := range services {
			entry, ok := s.(map[string]any)
			require.True(t, ok)
			assertKeysWithin(t, fmt.Sprintf("service[%d]", i), entry,
				"id", "type", "serviceEndpoint")

			endpoint, ok := entry["serviceEndpoint"].(string)
			require.True(t, ok, "serviceEndpoint must be a string")
			assert.True(t, strings.HasPrefix(endpoint, "/"),
				"serviceEndpoint %q must stay a relative path: an absolute URL would publish "+
					"the deployment's hostname from an unauthenticated route", endpoint)
			assert.NotContains(t, endpoint, "://")
		}
	})

	t.Run("AIMC-06.AC2 resolution is served no-store", func(t *testing.T) {
		resp, _, _ := resolveAgent(t, populatedAgent())
		assert.Contains(t, resp.Header.Get("Cache-Control"), "no-store",
			"a resolution answer must not be cached by an intermediary: the answer changes "+
				"the moment a key is rotated or an agent is suspended")
	})
}

// keyPath is one JSON object key together with the path it was found at.
type keyPath struct {
	key  string
	path string
}

// keyPaths walks a decoded JSON value and returns every object key with its path, so
// a key-name assertion can except a name in the one place it is legitimate.
func keyPaths(v any, prefix string) []keyPath {
	var out []keyPath
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			out = append(out, keyPath{key: k, path: path})
			out = append(out, keyPaths(child, path)...)
		}
	case []any:
		for i, child := range t {
			out = append(out, keyPaths(child, fmt.Sprintf("%s[%d]", prefix, i))...)
		}
	}
	return out
}
