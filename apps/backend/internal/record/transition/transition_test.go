package transition

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every lifecycle path wired to the recorder has the class its trigger
// names: a widening fails closed, a narrowing is never blocked.
func TestTriggerClasses(t *testing.T) {
	for trigger, want := range map[Trigger]store.Class{
		TriggerDirectGrant:         store.ClassExpansion,
		TriggerRequestAutoApproved: store.ClassExpansion,
		TriggerAgentReactivated:    store.ClassExpansion,
		TriggerKeyRotated:          store.ClassExpansion,
		TriggerRevocation:          store.ClassReduction,
		TriggerAgentSuspended:      store.ClassReduction,
		TriggerAgentRevoked:        store.ClassReduction,
		TriggerAgentDeleted:        store.ClassDestruction,
	} {
		got, ok := trigger.Class()
		require.True(t, ok, trigger)
		assert.Equal(t, want, got, trigger)
	}
	_, ok := Trigger("drift_approved").Class()
	assert.False(t, ok, "a trigger outside the agent-space set has no class")
}

func TestNewTraceID(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 64; i++ {
		id, err := NewTraceID()
		require.NoError(t, err)
		require.True(t, validTraceID(id), id)
		require.False(t, seen[id])
		seen[id] = true
	}
	for _, bad := range []string{"", strings.Repeat("0", 32), strings.Repeat("A", 32), strings.Repeat("a", 31), strings.Repeat("g", 32)} {
		assert.False(t, validTraceID(bad), bad)
	}
}

// newUnusedRecorder returns a recorder whose database is never reached: every
// change these tests make is refused before the write starts.
func newUnusedRecorder(t *testing.T) *Recorder {
	t.Helper()
	db, err := sql.Open("postgres", "postgres://unused.invalid/none")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	public, private, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	w, err := store.NewWriter(store.Config{
		DB: db, Keys: keyFunc(private), Key: record.PublicKey{Alg: record.AlgEd25519, Key: public},
	})
	require.NoError(t, err)
	r, err := NewRecorder(Config{Writer: w, DB: db, Issuer: "urn:uuid:" + uuid.NewString()})
	require.NoError(t, err)
	return r
}

type keyFunc ed25519.PrivateKey

func (k keyFunc) SignPayload(_ context.Context, class record.PayloadClass, payload record.Payload) (string, []byte, error) {
	message, err := record.SigningInput(class, payload)
	if err != nil {
		return "", nil, err
	}
	public := ed25519.PrivateKey(k).Public().(ed25519.PublicKey)
	return record.PublicKey{Alg: record.AlgEd25519, Key: public}.KeyID(), ed25519.Sign(ed25519.PrivateKey(k), message), nil
}

func (k keyFunc) PublicKey(context.Context) (record.PublicKey, error) {
	return record.PublicKey{Alg: record.AlgEd25519, Key: ed25519.PrivateKey(k).Public().(ed25519.PublicKey)}, nil
}

// The writer refuses a change without a trace id, and any other malformed
// change, before its statement runs.
func TestRecordRefusesAChangeWithoutATraceID(t *testing.T) {
	r := newUnusedRecorder(t)
	ran := false
	valid := Change{
		OrganizationID: uuid.New(),
		AgentID:        uuid.New(),
		Trigger:        TriggerAgentSuspended,
		Actor:          User(uuid.New()),
		TraceID:        strings.Repeat("ab", 16),
		Apply:          func(context.Context, *sql.Tx) error { ran = true; return nil },
	}
	for name, edit := range map[string]func(c *Change){
		"no trace id":         func(c *Change) { c.TraceID = "" },
		"an all-zero trace":   func(c *Change) { c.TraceID = strings.Repeat("0", 32) },
		"an unknown trigger":  func(c *Change) { c.Trigger = "drift_approved" },
		"no actor":            func(c *Change) { c.Actor = Actor{} },
		"a system with an id": func(c *Change) { c.Actor = Actor{Type: ActorSystem, ID: uuid.New()} },
		"no agent":            func(c *Change) { c.AgentID = uuid.Nil },
		"a malformed parent":  func(c *Change) { c.ParentID = "not-a-uuid" },
		"no statement":        func(c *Change) { c.Apply = nil },
	} {
		c := valid
		edit(&c)
		_, err := r.Record(context.Background(), c)
		require.ErrorIs(t, err, ErrInvalidChange, name)
	}
	assert.False(t, ran, "a refused change ran its statement")
}

func TestNewRecorderNeedsAnIssuer(t *testing.T) {
	db, err := sql.Open("postgres", "postgres://unused.invalid/none")
	require.NoError(t, err)
	defer db.Close()
	public, private, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	w, err := store.NewWriter(store.Config{DB: db, Keys: keyFunc(private), Key: record.PublicKey{Alg: record.AlgEd25519, Key: public}})
	require.NoError(t, err)
	id := uuid.NewString()
	for _, bad := range []string{"", id, "urn:uuid:" + strings.ToUpper(id), "urn:uuid:{" + id + "}"} {
		_, err := NewRecorder(Config{Writer: w, DB: db, Issuer: bad})
		assert.Error(t, err, bad)
	}
}

// An Ed25519 key is identified by its did:key, which decodes back to the key.
func TestKeyIdentifier(t *testing.T) {
	assert.Equal(t, "2NEpo7TZRRrLZSi2U", base58btc([]byte("Hello World!")))
	assert.Equal(t, "17paNL19xttacUY", base58btc([]byte("\x00yes mani !")))

	public, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	id := keyIdentifier("Ed25519", base64.StdEncoding.EncodeToString(public))
	require.True(t, strings.HasPrefix(id, "did:key:z6Mk"), id)
	decoded := base58decode(t, strings.TrimPrefix(id, "did:key:z"))
	assert.Equal(t, append([]byte{0xed, 0x01}, public...), decoded)

	pqc := bytes.Repeat([]byte{7}, 1952)
	assert.Equal(t, "u"+base64.RawURLEncoding.EncodeToString(pqc),
		keyIdentifier("ML-DSA-65", base64.StdEncoding.EncodeToString(pqc)))
	assert.Equal(t, "u"+base64.RawURLEncoding.EncodeToString([]byte("not base64!")),
		keyIdentifier("Ed25519", "not base64!"))
}

func base58decode(t *testing.T, s string) []byte {
	t.Helper()
	n := new(big.Int)
	for _, c := range s {
		i := strings.IndexRune(base58Alphabet, c)
		require.GreaterOrEqual(t, i, 0)
		n.Mul(n, big.NewInt(58))
		n.Add(n, big.NewInt(int64(i)))
	}
	return n.Bytes()
}

// A state written as a record member reads back as the same state.
func TestStateMemberRoundTrip(t *testing.T) {
	grace := "2026-10-07T12:00:00.000001Z"
	for _, s := range []State{
		{Scope: []string{}, GrantedScope: []string{}, Status: "pending", Keys: []Key{}},
		{
			Scope:        []string{},
			GrantedScope: []string{"api:call", "files:read"},
			Status:       "suspended",
			Keys: []Key{
				{Alg: "Ed25519", ID: "did:key:z6MkA", Role: KeyRoleCurrent, Custody: KeyCustodyServer},
				{Alg: "Ed25519", ID: "did:key:z6MkB", Role: KeyRolePrevious, GraceUntil: &grace},
				{Alg: "ML-DSA-65", ID: "uAAAA", Role: KeyRoleCurrent, Custody: KeyCustodyExternal},
				{Alg: "ML-DSA-65", ID: "uBBBB", Role: KeyRolePrevious},
			},
		},
	} {
		raw, err := json.Marshal(s.member())
		require.NoError(t, err)
		var m stateMember
		require.NoError(t, json.Unmarshal(raw, &m))
		assert.True(t, Equal(s, m.state()), "%s", raw)
	}
	a := State{Scope: []string{}, GrantedScope: []string{}, Status: "verified", Keys: []Key{{Alg: "Ed25519", ID: "x", Role: KeyRolePrevious}}}
	b := a
	b.Keys = []Key{{Alg: "Ed25519", ID: "x", Role: KeyRolePrevious, GraceUntil: &grace}}
	assert.False(t, Equal(a, b), "a grace deadline is part of the state")
}

// Fixed inputs produce fixed canonical bytes. The retained part commits to
// the tenant and personal parts, so a change of any member name or value this
// package writes changes these bytes. Change the expected value only together
// with WriterVersion.
func TestTransitionRecordGoldenBytes(t *testing.T) {
	r := &Recorder{issuer: "urn:uuid:7c9e6679-7425-40de-944b-e07fc1f90ae7"}
	agent := uuid.MustParse("9b2f1c44-6a0d-4e53-8f7a-1d2c3b4a5e6f")
	d := r.draft(Change{
		OrganizationID: uuid.MustParse("3f2504e0-4f89-41d3-9a0c-0305e82c3301"),
		AgentID:        agent,
		Trigger:        TriggerKeyRotated,
		Actor:          User(uuid.MustParse("5d4c3b2a-1f0e-4d9c-8b7a-6f5e4d3c2b1a")),
		TraceID:        "4bf92f3577b34da6a3ce929d0e0e4736",
	})
	d.EventID = "a1a1a1a1-0000-4000-8000-000000000001"
	d.Timestamp = time.Date(2026, time.January, 2, 3, 4, 5, 6000, time.UTC)
	d.TenantSalt = bytes.Repeat([]byte{0x21}, record.SaltSize)
	d.PersonalSalt = bytes.Repeat([]byte{0x22}, record.SaltSize)
	prev := State{Scope: []string{"files:read"}, GrantedScope: []string{"files:read"}, Status: "verified",
		Keys: []Key{{Alg: "Ed25519", ID: "did:key:z6MkOld", Role: KeyRoleCurrent, Custody: KeyCustodyExternal}}}
	next := prev
	next.Keys = []Key{
		{Alg: "Ed25519", ID: "did:key:z6MkNew", Role: KeyRoleCurrent, Custody: KeyCustodyServer},
		{Alg: "Ed25519", ID: "did:key:z6MkOld", Role: KeyRolePrevious},
	}
	d.Tenant["previous_state"] = prev.member()
	d.Tenant["new_state"] = next.member()

	body, err := record.NewRecord(record.Head{
		ChainID: "0f0e0d0c-0b0a-4908-8706-050403020100",
		Seq:     6,
		Hash:    strings.Repeat("ab", 32),
	}, d)
	require.NoError(t, err)
	sum := sha256Hex(body.Canonical())
	assert.Equal(t, goldenTransitionHash, sum, "canonical bytes:\n%s", body.Canonical())
}

const goldenTransitionHash = "e2ce1845137680d018661662880f38ee91665b2bf3a3a7888087a989ca28fcae"

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
