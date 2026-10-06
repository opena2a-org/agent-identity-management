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
		TriggerRegistrationBaseline: store.ClassExpansion,
		TriggerCapabilityRequested:  store.ClassExpansion,
		TriggerDirectGrant:          store.ClassExpansion,
		TriggerRequestAutoApproved:  store.ClassExpansion,
		TriggerAgentReactivated:     store.ClassExpansion,
		TriggerKeyRotated:           store.ClassExpansion,
		TriggerRevocation:           store.ClassReduction,
		TriggerAgentSuspended:       store.ClassReduction,
		TriggerAgentRevoked:         store.ClassReduction,
		TriggerAgentDeleted:         store.ClassDestruction,
	} {
		got, ok := trigger.Class()
		require.True(t, ok, trigger)
		assert.Equal(t, want, got, trigger)
	}
	_, ok := Trigger("drift_approved").Class()
	assert.False(t, ok, "a trigger outside the agent-space set has no class")
	assert.False(t, Trigger("drift_approved").ClassedByComparison(), "drift_approved has no writer")

	for _, trigger := range []Trigger{TriggerTalksToReplaced, TriggerTalksToAdded, TriggerTalksToRemoved, TriggerDetectionReported} {
		assert.True(t, trigger.ClassedByComparison(), trigger)
		_, ok := trigger.Class()
		assert.False(t, ok, "%s takes its class from its lists, never from its name", trigger)
	}
	assert.False(t, TriggerDirectGrant.ClassedByComparison())
}

// A talks_to change is a reduction only when the new list is non-empty and
// every entry of it was in the old one; entries compare as exact strings,
// and order and repeats do not count.
func TestTalksToClass(t *testing.T) {
	for name, tc := range map[string]struct {
		before, after []string
		class         store.Class
		changed       bool
	}{
		"an entry added":          {[]string{"a"}, []string{"a", "b"}, store.ClassExpansion, true},
		"the first entry":         {nil, []string{"a"}, store.ClassExpansion, true},
		"an entry removed":        {[]string{"a", "b"}, []string{"b"}, store.ClassReduction, true},
		"the last entry removed":  {[]string{"a"}, []string{}, store.ClassExpansion, true},
		"one removed, one added":  {[]string{"a", "b"}, []string{"a", "c"}, store.ClassExpansion, true},
		"a replacement":           {[]string{"a"}, []string{"b"}, store.ClassExpansion, true},
		"a case change":           {[]string{"github"}, []string{"GitHub"}, store.ClassExpansion, true},
		"an entry split in two":   {[]string{"a,b"}, []string{"a", "b"}, store.ClassExpansion, true},
		"the same set reordered":  {[]string{"a", "b"}, []string{"b", "a"}, "", false},
		"a repeat dropped":        {[]string{"a", "a", "b"}, []string{"a", "b"}, "", false},
		"both empty":              {nil, []string{}, "", false},
		"a repeat kept, one gone": {[]string{"a", "a", "b"}, []string{"a", "a"}, store.ClassReduction, true},
	} {
		class, changed := TalksToClass(tc.before, tc.after)
		assert.Equal(t, tc.changed, changed, name)
		assert.Equal(t, tc.class, class, name)
	}
}

// A talks_to trigger needs the class its lists give, and no other trigger
// takes a class from its caller.
func TestRecordRefusesAClassItsTriggerDoesNotTake(t *testing.T) {
	r := newUnusedRecorder(t)
	ran := false
	base := Change{
		OrganizationID: uuid.New(),
		AgentID:        uuid.New(),
		Actor:          User(uuid.New()),
		TraceID:        strings.Repeat("ab", 16),
		Apply:          func(context.Context, *sql.Tx) error { ran = true; return nil },
	}
	for name, c := range map[string]Change{
		"a talks_to trigger with no class":       {Trigger: TriggerTalksToAdded},
		"a talks_to trigger as a destruction":    {Trigger: TriggerTalksToRemoved, Class: store.ClassDestruction},
		"a talks_to trigger as an observation":   {Trigger: TriggerDetectionReported, Class: store.ClassObservation},
		"a suspension given the reduction class": {Trigger: TriggerAgentSuspended, Class: store.ClassReduction},
		"a grant given the reduction class":      {Trigger: TriggerDirectGrant, Class: store.ClassReduction},
	} {
		c.OrganizationID, c.AgentID, c.Actor, c.TraceID, c.Apply = base.OrganizationID, base.AgentID, base.Actor, base.TraceID, base.Apply
		_, err := r.Record(context.Background(), c)
		require.ErrorIs(t, err, ErrInvalidChange, name)
	}
	assert.False(t, ran, "a refused change ran its statement")
}

// Under the agent's row lock, a talks_to trigger must change the list, only
// the list, and in the class its caller read; any other trigger must leave
// the list as it was.
func TestCheckStatesOfATalksToChange(t *testing.T) {
	prev := State{Scope: []string{"api:call"}, GrantedScope: []string{"api:call"}, Status: "verified",
		Keys: []Key{}, TalksTo: []string{"a", "b"}}
	with := func(edit func(s *State)) State {
		s := prev
		edit(&s)
		return s
	}
	removed := with(func(s *State) { s.TalksTo = []string{"a"} })
	added := with(func(s *State) { s.TalksTo = []string{"a", "b", "c"} })

	reduction := Change{Trigger: TriggerTalksToRemoved, Class: store.ClassReduction}
	require.NoError(t, checkStates(reduction, prev, removed))
	require.ErrorIs(t, checkStates(reduction, prev, prev), ErrNoChange)
	require.ErrorIs(t, checkStates(reduction, prev, added), ErrInvalidChange, "a removal that became an addition")
	emptied := with(func(s *State) { s.TalksTo = []string{} })
	require.ErrorIs(t, checkStates(reduction, prev, emptied), ErrInvalidChange, "an emptied list is not a reduction")
	suspended := with(func(s *State) { s.TalksTo = []string{"a"}; s.Status = "suspended"; s.Scope = []string{} })
	require.ErrorIs(t, checkStates(reduction, prev, suspended), ErrInvalidChange, "a talks_to change that also suspends")

	expansion := Change{Trigger: TriggerTalksToAdded, Class: store.ClassExpansion}
	require.NoError(t, checkStates(expansion, prev, added))
	require.NoError(t, checkStates(expansion, prev, emptied))
	require.ErrorIs(t, checkStates(expansion, prev, removed), ErrInvalidChange, "an addition that became a removal")

	grant := Change{Trigger: TriggerDirectGrant}
	granted := with(func(s *State) { s.GrantedScope = []string{"api:call", "files:read"}; s.Scope = s.GrantedScope })
	require.NoError(t, checkStates(grant, prev, granted))
	require.ErrorIs(t, checkStates(grant, prev, with(func(s *State) {
		s.GrantedScope = []string{"api:call", "files:read"}
		s.Scope = s.GrantedScope
		s.TalksTo = []string{"a", "b", "c"}
	})), ErrInvalidChange, "a grant that also widens talks_to")
	require.NoError(t, checkStates(Change{Trigger: TriggerRegistrationBaseline}, State{}, added))
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
		"an uppercase event id": func(c *Change) {
			c.EventID = strings.ToUpper(uuid.NewString())
		},
		"no statement": func(c *Change) { c.Apply = nil },
	} {
		c := valid
		edit(&c)
		_, err := r.Record(context.Background(), c)
		require.ErrorIs(t, err, ErrInvalidChange, name)
	}
	assert.False(t, ran, "a refused change ran its statement")
}

// A rejected request's record says it was rejected; no other trigger's record
// carries an outcome.
func TestOnlyARejectionCarriesAnOutcome(t *testing.T) {
	r := &Recorder{issuer: "urn:uuid:" + uuid.NewString()}
	for trigger := range classes {
		d := r.draft(Change{Trigger: trigger, Actor: System(), TraceID: strings.Repeat("ab", 16)})
		outcome, ok := d.Retained["opena2a"].(map[string]any)["outcome"]
		if trigger == TriggerRequestRejected {
			assert.Equal(t, OutcomeRejected, outcome)
		} else {
			assert.False(t, ok, "%s carries an outcome", trigger)
		}
	}
}

// Only a registration opens an agent's history, and only a pending request
// and a rejection are null transitions.
func TestOpeningAndNullTriggers(t *testing.T) {
	for trigger := range classes {
		assert.Equal(t, trigger == TriggerRegistrationBaseline, trigger.opens(), trigger)
		assert.Equal(t, trigger == TriggerCapabilityRequested || trigger == TriggerRequestRejected, trigger.null(), trigger)
	}
}

// Only a deletion closes an agent's history. A deletion is a destruction, and
// its states are not held to the talks_to rule: the list goes with the agent.
func TestClosingTrigger(t *testing.T) {
	for trigger := range classes {
		assert.Equal(t, trigger == TriggerAgentDeleted, trigger.closes(), trigger)
	}
	for trigger := range talksToTriggers {
		assert.False(t, trigger.closes(), trigger)
	}
	class, ok := TriggerAgentDeleted.Class()
	require.True(t, ok)
	assert.Equal(t, store.ClassDestruction, class)
	prev := State{Scope: []string{}, GrantedScope: []string{}, Status: "suspended", Keys: []Key{}, TalksTo: []string{"a"}}
	require.NoError(t, checkStates(Change{Trigger: TriggerAgentDeleted}, prev, State{}))
}

// A request's record has an event id derived from the request's id: the same
// for the same request, another for another request, and never the request's
// own id.
func TestRequestEventID(t *testing.T) {
	request := uuid.New()
	id := RequestEventID(request)
	parsed, err := uuid.Parse(id)
	require.NoError(t, err)
	assert.Equal(t, parsed.String(), id, "not lowercase hyphenated")
	assert.Equal(t, uuid.Version(5), parsed.Version())
	assert.Equal(t, id, RequestEventID(request))
	assert.NotEqual(t, request.String(), id)
	assert.NotEqual(t, id, RequestEventID(uuid.New()))
	assert.Equal(t, "d791c794-14bc-5b5d-a1a5-33bdf99b351a",
		RequestEventID(uuid.MustParse("3f2504e0-4f89-41d3-9a0c-0305e82c3301")),
		"the derivation changed, so a decision no longer finds the records of requests filed before it")
}

// A change's record takes the event id the change names, and a random one
// otherwise.
func TestDraftEventID(t *testing.T) {
	r := &Recorder{issuer: "urn:uuid:" + uuid.NewString()}
	c := Change{Trigger: TriggerCapabilityRequested, Actor: System(), TraceID: strings.Repeat("ab", 16)}
	first, second := r.draft(c), r.draft(c)
	assert.True(t, isCanonicalUUID(first.EventID), first.EventID)
	assert.NotEqual(t, first.EventID, second.EventID)
	c.EventID = RequestEventID(uuid.New())
	assert.Equal(t, c.EventID, r.draft(c).EventID)
}

// The record that opens an agent's history has an event id derived from the
// agent's id: the same for the same agent, another for another agent, never
// the agent's own id, and never the event id a request of the same id has.
func TestOpeningEventID(t *testing.T) {
	agent := uuid.New()
	id := OpeningEventID(agent)
	parsed, err := uuid.Parse(id)
	require.NoError(t, err)
	assert.Equal(t, parsed.String(), id, "not lowercase hyphenated")
	assert.Equal(t, uuid.Version(5), parsed.Version())
	assert.Equal(t, id, OpeningEventID(agent))
	assert.NotEqual(t, agent.String(), id)
	assert.NotEqual(t, id, OpeningEventID(uuid.New()))
	assert.NotEqual(t, RequestEventID(agent), id)
	assert.Equal(t, "c9e2e5af-cc42-50eb-9680-b0a9573f10e7",
		OpeningEventID(uuid.MustParse("3f2504e0-4f89-41d3-9a0c-0305e82c3301")),
		"the derivation changed, so the recorder no longer finds the records that opened agents' histories before it")
}

// A registration's record and an opening state's take the agent's opening
// event id. An opening state is the system's, joins the trace it is written
// in, has no parent and no outcome, and holds one state as both its states.
func TestOpeningDrafts(t *testing.T) {
	r := &Recorder{issuer: "urn:uuid:" + uuid.NewString()}
	org, agent := uuid.New(), uuid.New()
	trace := strings.Repeat("ab", 16)
	reg := r.draft(Change{OrganizationID: org, AgentID: agent, Trigger: TriggerRegistrationBaseline,
		Actor: User(uuid.New()), TraceID: trace})
	assert.Equal(t, OpeningEventID(agent), reg.EventID)

	d := r.openingDraft(org, agent, trace)
	assert.Equal(t, OpeningEventID(agent), d.EventID)
	assert.Equal(t, RecordType, d.Type)
	assert.Equal(t, map[string]any{"type": "opening_state"}, d.Retained["trigger"])
	retained := d.Retained["opena2a"].(map[string]any)
	assert.Equal(t, "server", retained["trace_origin"])
	assert.Equal(t, StateSpaceAgent, retained["state_space"])
	_, hasOutcome := retained["outcome"]
	assert.False(t, hasOutcome, "an opening state carries an outcome")
	assert.Equal(t, map[string]any{"actor": "system"}, d.Personal)
	assert.Equal(t, trace, d.Tenant["trace_id"])
	assert.Nil(t, d.Tenant["parent_id"])
	s := State{Scope: []string{}, GrantedScope: []string{"files:read"}, Status: "suspended", Keys: []Key{}, TalksTo: []string{"a"}}
	withState(&d, s)
	assert.Equal(t, s.member(), d.Tenant["previous_state"])
	assert.Equal(t, s.member(), d.Tenant["new_state"])
}

// Only the recorder writes an opening state, and only a registration and an
// opening state start an agent's history. A change that names an opening
// state, a registration that names an event id, and a sweep of no
// organization are refused before anything runs.
func TestOpeningStateIsTheRecordersOwn(t *testing.T) {
	_, ok := TriggerOpeningState.Class()
	assert.False(t, ok, "an opening state takes the class of the change it leads, or is the sweep's observation")
	assert.False(t, TriggerOpeningState.ClassedByComparison())
	assert.True(t, TriggerOpeningState.first())
	assert.False(t, TriggerOpeningState.opens(), "an opening state's previous_state is not null")
	for trigger := range classes {
		assert.Equal(t, trigger == TriggerRegistrationBaseline, trigger.first(), trigger)
	}
	for trigger := range talksToTriggers {
		assert.False(t, trigger.first(), trigger)
	}

	r := newUnusedRecorder(t)
	ran := false
	base := Change{
		OrganizationID: uuid.New(),
		AgentID:        uuid.New(),
		Actor:          System(),
		TraceID:        strings.Repeat("ab", 16),
		Apply:          func(context.Context, *sql.Tx) error { ran = true; return nil },
	}
	opening := base
	opening.Trigger = TriggerOpeningState
	_, err := r.Record(context.Background(), opening)
	require.ErrorIs(t, err, ErrInvalidChange)
	registration := base
	registration.Trigger = TriggerRegistrationBaseline
	registration.EventID = uuid.NewString()
	_, err = r.Record(context.Background(), registration)
	require.ErrorIs(t, err, ErrInvalidChange)
	assert.False(t, ran, "a refused change ran its statement")

	_, err = r.OpenStates(context.Background(), uuid.Nil)
	require.ErrorIs(t, err, ErrInvalidChange)
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
		{Scope: []string{}, GrantedScope: []string{}, Status: "verified", Keys: []Key{}, TalksTo: []string{"filesystem", "github"}},
	} {
		raw, err := json.Marshal(s.member())
		require.NoError(t, err)
		var m stateMember
		require.NoError(t, json.Unmarshal(raw, &m))
		assert.True(t, Equal(s, m.state()), "%s", raw)
	}
	talks := State{Scope: []string{}, GrantedScope: []string{}, Status: "verified", Keys: []Key{}, TalksTo: []string{"github"}}
	raw, err := json.Marshal(talks.member())
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"talks_to":["github"]`)
	none := talks
	none.TalksTo = nil
	raw, err = json.Marshal(none.member())
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"talks_to":[]`, "an empty list is written, not left out")
	assert.False(t, Equal(talks, none), "talks_to is part of the state")
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
		Keys:    []Key{{Alg: "Ed25519", ID: "did:key:z6MkOld", Role: KeyRoleCurrent, Custody: KeyCustodyExternal}},
		TalksTo: []string{"filesystem", "github"}}
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

const goldenTransitionHash = "397ed37f91441f10cee02d132b651a5a6f7b326a8776a3794919cdedbb878ba7"

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
