package record

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
	"time"
)

// memoryKeyProvider is the key provider of this package's tests: an Ed25519
// key generated when the test runs and held only in memory.
type memoryKeyProvider struct {
	public  ed25519.PublicKey
	private ed25519.PrivateKey
}

func newMemoryKeyProvider(t *testing.T) *memoryKeyProvider {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return &memoryKeyProvider{public: public, private: private}
}

func (m *memoryKeyProvider) PublicKey(context.Context) (PublicKey, error) {
	return PublicKey{Alg: AlgEd25519, Key: append([]byte(nil), m.public...)}, nil
}

func (m *memoryKeyProvider) SignPayload(_ context.Context, class PayloadClass, payload Payload) (string, []byte, error) {
	message, err := SigningInput(class, payload)
	if err != nil {
		return "", nil, err
	}
	key := PublicKey{Alg: AlgEd25519, Key: m.public}
	return key.KeyID(), ed25519.Sign(m.private, message), nil
}

func (m *memoryKeyProvider) key(t *testing.T) PublicKey {
	t.Helper()
	key, err := m.PublicKey(context.Background())
	if err != nil {
		t.Fatalf("public key: %v", err)
	}
	return key
}

// Fixed inputs of the chain the tests and the golden vectors share.
const (
	fixtureChainID = "0b0e7a52-5d0c-4c1e-9d55-2f6a3c1b7e10"
	fixtureIssuer  = "urn:uuid:7c9e6679-7425-40de-944b-e07fc1f90ae7"
	fixtureOrg     = "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	fixtureAgent   = "9b2f1c44-6a0d-4e53-8f7a-1d2c3b4a5e6f"
	fixtureTrace   = "4bf92f3577b34da6a3ce929d0e0e4736"
	fixtureWriter  = "vectors-1"

	fixtureGenesisEvent    = "a1a1a1a1-0000-4000-8000-000000000000"
	fixtureTransitionEvent = "a1a1a1a1-0000-4000-8000-000000000001"
	fixtureActionEvent     = "a1a1a1a1-0000-4000-8000-000000000002"
	fixtureAdminEvent      = "a1a1a1a1-0000-4000-8000-000000000003"
)

var fixtureStart = time.Date(2026, time.January, 2, 3, 4, 5, 123456789, time.UTC)

// fixtureSalt is a fixed salt for vectors. A real salt comes from a
// cryptographic random source.
func fixtureSalt(fill byte) []byte { return bytes.Repeat([]byte{fill}, SaltSize) }

func fixtureRetained(source string, more map[string]any) map[string]any {
	ext := map[string]any{
		"issuer":         fixtureIssuer,
		"writer_version": fixtureWriter,
		"source":         source,
	}
	for k, v := range more {
		ext[k] = v
	}
	return map[string]any{"opena2a": ext}
}

func fixtureGenesis(first PublicKey, pre []PreGenesisRow) Genesis {
	return Genesis{
		ChainID:    fixtureChainID,
		FirstKey:   first,
		PreGenesis: pre,
		Draft: Draft{
			EventID:   fixtureGenesisEvent,
			Timestamp: fixtureStart,
			Retained:  fixtureRetained("system", nil),
			Tenant: map[string]any{
				"trace_id": fixtureTrace,
				"opena2a":  map[string]any{"organization_id": fixtureOrg},
			},
			TenantSalt: fixtureSalt(0x10),
		},
	}
}

func fixturePreGenesis() []PreGenesisRow {
	return []PreGenesisRow{
		{Table: TableVerificationEvents, ID: "c3c3c3c3-0000-4000-8000-000000000001", Timestamp: time.Date(2025, time.December, 30, 8, 0, 0, 0, time.UTC)},
		{Table: TableAuditLogs, ID: "b2b2b2b2-0000-4000-8000-000000000002", Timestamp: time.Date(2025, time.December, 31, 23, 59, 59, 999999000, time.UTC)},
		{Table: TableAuditLogs, ID: "b2b2b2b2-0000-4000-8000-000000000001", Timestamp: time.Date(2025, time.November, 15, 12, 30, 0, 0, time.FixedZone("", 2*60*60))},
	}
}

// fixtureDrafts are the three records that follow the genesis: one with both
// erasable parts and two with a tenant part alone.
func fixtureDrafts() []Draft {
	return []Draft{
		{
			EventID:   fixtureTransitionEvent,
			Type:      "authorization_transition",
			Timestamp: fixtureStart.Add(90 * time.Second),
			Retained: map[string]any{
				"trigger": map[string]any{"type": "direct_grant"},
				"opena2a": map[string]any{
					"issuer":         fixtureIssuer,
					"writer_version": fixtureWriter,
					"source":         "service",
					"state_space":    "agent",
					"trace_origin":   "server",
				},
			},
			Tenant: map[string]any{
				"trace_id":       fixtureTrace,
				"parent_id":      nil,
				"previous_state": map[string]any{"scope": []any{}},
				"new_state":      map[string]any{"scope": []any{"files:read"}},
				"opena2a": map[string]any{
					"organization_id":  fixtureOrg,
					"subject_agent_id": fixtureAgent,
				},
			},
			TenantSalt: fixtureSalt(0x21),
			Personal: map[string]any{
				"actor":    "user:5d4c3b2a-1f0e-4d9c-8b7a-6f5e4d3c2b1a",
				"metadata": map[string]any{"reason": "Prüfung für Kunde €42 ✓", "case_number": int64(4711)},
			},
			PersonalSalt: fixtureSalt(0x22),
		},
		{
			EventID:   fixtureActionEvent,
			Type:      "action",
			Timestamp: fixtureStart.Add(2 * time.Minute),
			Retained: fixtureRetained("agent_reported", map[string]any{
				"outcome":      "allowed",
				"trace_origin": "parent",
			}),
			Tenant: map[string]any{
				"trace_id":  fixtureTrace,
				"parent_id": fixtureTransitionEvent,
				"opena2a": map[string]any{
					"organization_id":   fixtureOrg,
					"subject_agent_id":  fixtureAgent,
					"authorization_ref": fixtureTransitionEvent,
				},
			},
			TenantSalt: fixtureSalt(0x31),
		},
		{
			EventID:   fixtureAdminEvent,
			Type:      "opena2a.administrative",
			Timestamp: fixtureStart.Add(3 * time.Minute),
			Retained: fixtureRetained("service", map[string]any{
				"admin_action":  "tag_created",
				"resource_type": "tag",
				"trace_origin":  "server",
			}),
			Tenant: map[string]any{
				"trace_id":    "00f067aa0ba902b7a3ce929d0e0e4736",
				"resource_id": "d4d4d4d4-0000-4000-8000-000000000001",
				"opena2a":     map[string]any{"organization_id": fixtureOrg},
			},
			TenantSalt: fixtureSalt(0x41),
		},
	}
}

// buildChain builds the fixture chain: a genesis that names first, then the
// three fixture records.
func buildChain(t *testing.T, first PublicKey) []Body {
	t.Helper()
	genesis, err := NewGenesis(fixtureGenesis(first, fixturePreGenesis()))
	if err != nil {
		t.Fatalf("genesis: %v", err)
	}
	bodies := []Body{genesis}
	for _, d := range fixtureDrafts() {
		next, err := NewRecord(bodies[len(bodies)-1].Head(), d)
		if err != nil {
			t.Fatalf("record %s: %v", d.Type, err)
		}
		bodies = append(bodies, next)
	}
	return bodies
}

// signedChain builds the fixture chain signed by a key generated for this
// test.
func signedChain(t *testing.T) ([]Record, *memoryKeyProvider) {
	t.Helper()
	kp := newMemoryKeyProvider(t)
	bodies := buildChain(t, kp.key(t))
	records := make([]Record, 0, len(bodies))
	for _, b := range bodies {
		rec, err := Sign(context.Background(), b, kp)
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		records = append(records, rec)
	}
	return records, kp
}

// signRaw signs caller-built bytes as the given class. Only a test inside the
// package can do this: no exported operation builds a Payload from bytes.
func signRaw(t *testing.T, kp *memoryKeyProvider, class PayloadClass, payload []byte) Record {
	t.Helper()
	env, err := Seal(context.Background(), kp, class, Payload{canonical: payload})
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	return Record{Envelope: env, RecordHash: hashHex(payload)}
}

// payloadBytes returns the decoded payload of a record's envelope.
func payloadBytes(t *testing.T, rec Record) []byte {
	t.Helper()
	payload, err := base64.StdEncoding.DecodeString(rec.Envelope.Payload)
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	return payload
}

// setPayload stores payload in a record's envelope.
func setPayload(rec *Record, payload []byte) {
	rec.Envelope.Payload = base64.StdEncoding.EncodeToString(payload)
}

// flipSignatureByte changes one byte of a signature.
func flipSignatureByte(t *testing.T, s *Signature, at int) {
	t.Helper()
	sig, err := base64.StdEncoding.DecodeString(s.Sig)
	if err != nil {
		t.Fatalf("signature: %v", err)
	}
	sig[at] ^= 0x01
	s.Sig = base64.StdEncoding.EncodeToString(sig)
}

func mustVerify(t *testing.T, records []Record, key PublicKey) Result {
	t.Helper()
	res, err := Verify(records, key)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	return res
}

// wantFailure asserts that verification stopped at position with kind.
func wantFailure(t *testing.T, res Result, position int, kind FailureKind) {
	t.Helper()
	if res.Failure == nil {
		t.Fatalf("verification passed; want %s at position %d", kind, position)
	}
	if res.Failure.Position != position || res.Failure.Kind != kind {
		t.Fatalf("got %s at position %d (%s); want %s at position %d",
			res.Failure.Kind, res.Failure.Position, res.Failure.Detail, kind, position)
	}
	if res.OK() {
		t.Fatalf("a failed result reports OK")
	}
	if res.Verified != position {
		t.Fatalf("verified %d records before the failure; want %d", res.Verified, position)
	}
}

func cloneRecords(in []Record) []Record {
	out := make([]Record, len(in))
	for i, r := range in {
		out[i] = r
		out[i].Envelope.Signatures = append([]Signature(nil), r.Envelope.Signatures...)
		out[i].TenantPart = append([]byte(nil), r.TenantPart...)
		out[i].TenantSalt = append([]byte(nil), r.TenantSalt...)
		out[i].PersonalPart = append([]byte(nil), r.PersonalPart...)
		out[i].PersonalSalt = append([]byte(nil), r.PersonalSalt...)
	}
	return out
}
