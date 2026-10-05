package record

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// keylessKey is a fixed public key for tests that sign nothing: 32 bytes of a
// hash output, for which no private half is held anywhere.
func keylessKey() PublicKey {
	sum := sha256.Sum256([]byte("opena2a audit-record vectors: a public key with no private half"))
	return PublicKey{Alg: AlgEd25519, Key: sum[:]}
}

// chainOf reads opena2a.chain from canonical bytes with a plain JSON decoder.
func chainOf(t *testing.T, canonical []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(canonical, &doc); err != nil {
		t.Fatalf("canonical bytes are not JSON: %v", err)
	}
	return doc["opena2a"].(map[string]any)["chain"].(map[string]any)
}

func TestNewGenesis_HasSequenceZeroAndNoPredecessor(t *testing.T) {
	b, err := NewGenesis(fixtureGenesis(keylessKey(), nil))
	if err != nil {
		t.Fatalf("NewGenesis: %v", err)
	}
	if !strings.Contains(string(b.Canonical()), `"chain":{"id":"`+fixtureChainID+`","prev_hash":null,"seq":0}`) {
		t.Fatalf("the genesis does not carry sequence 0 and a null predecessor hash:\n%s", b.Canonical())
	}
	if !strings.Contains(string(b.Canonical()), `"type":"opena2a.chain_genesis"`) {
		t.Fatalf("the genesis does not carry its type:\n%s", b.Canonical())
	}
	if got, want := b.Head(), (Head{ChainID: fixtureChainID, Seq: 0, Hash: hashHex(b.Canonical())}); got != want {
		t.Fatalf("head %+v; want %+v", got, want)
	}
}

func TestNewGenesis_NamesItsFirstKey(t *testing.T) {
	key := keylessKey()
	b, err := NewGenesis(fixtureGenesis(key, nil))
	if err != nil {
		t.Fatalf("NewGenesis: %v", err)
	}
	want := `"first_key":{"alg":"Ed25519","keyid":"` + key.KeyID() + `","public_key":"` + hex.EncodeToString(key.Key) + `"}`
	if !strings.Contains(string(b.Canonical()), want) {
		t.Fatalf("the genesis does not name its first key:\n%s", b.Canonical())
	}
}

func TestNewGenesis_PinsThePreGenesisRows(t *testing.T) {
	rows := fixturePreGenesis()
	b, err := NewGenesis(fixtureGenesis(keylessKey(), rows))
	if err != nil {
		t.Fatalf("NewGenesis: %v", err)
	}
	digest, err := SetDigest(rows)
	if err != nil {
		t.Fatalf("SetDigest: %v", err)
	}
	want := `"pre_genesis":{"newest_timestamp":"2025-12-31T23:59:59.999999Z","row_counts":{"audit_logs":2,"verification_events":1},"set_digest":"` + digest + `"}`
	if !strings.Contains(string(b.Canonical()), want) {
		t.Fatalf("the genesis does not pin the pre-genesis rows as %s:\n%s", want, b.Canonical())
	}

	empty, err := NewGenesis(fixtureGenesis(keylessKey(), nil))
	if err != nil {
		t.Fatalf("NewGenesis: %v", err)
	}
	wantEmpty := `"pre_genesis":{"newest_timestamp":null,"row_counts":{"audit_logs":0,"verification_events":0},"set_digest":"4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945"}`
	if !strings.Contains(string(empty.Canonical()), wantEmpty) {
		t.Fatalf("a genesis with no earlier rows does not carry %s:\n%s", wantEmpty, empty.Canonical())
	}
}

func TestNewRecord_CommitsToItsPredecessorsHash(t *testing.T) {
	bodies := buildChain(t, keylessKey())
	for i := 1; i < len(bodies); i++ {
		chain := chainOf(t, bodies[i].Canonical())
		predecessor := sha256.Sum256(bodies[i-1].Canonical())
		if got, want := chain["prev_hash"], hex.EncodeToString(predecessor[:]); got != want {
			t.Fatalf("record %d names predecessor hash %v; want %s", i, got, want)
		}
		if got := chain["seq"]; got != float64(i) {
			t.Fatalf("record %d carries sequence number %v", i, got)
		}
		if got := chain["id"]; got != fixtureChainID {
			t.Fatalf("record %d carries chain id %v", i, got)
		}
	}
}

func TestNewRecord_ChangingAnEarlierRecordChangesEveryLaterHash(t *testing.T) {
	before := buildChain(t, keylessKey())

	genesis := fixtureGenesis(keylessKey(), fixturePreGenesis())
	genesis.Draft.Timestamp = genesis.Draft.Timestamp.Add(time.Microsecond)
	changed, err := NewGenesis(genesis)
	if err != nil {
		t.Fatalf("NewGenesis: %v", err)
	}
	after := []Body{changed}
	for _, d := range fixtureDrafts() {
		next, err := NewRecord(after[len(after)-1].Head(), d)
		if err != nil {
			t.Fatalf("NewRecord: %v", err)
		}
		after = append(after, next)
	}
	for i := range before {
		if before[i].Head().Hash == after[i].Head().Hash {
			t.Fatalf("record %d kept its hash after the genesis changed", i)
		}
	}
}

func TestSeal_EqualRecordsGiveEqualBytes(t *testing.T) {
	first := buildChain(t, keylessKey())
	for run := 0; run < 20; run++ {
		again := buildChain(t, keylessKey())
		for i := range first {
			if string(first[i].Canonical()) != string(again[i].Canonical()) {
				t.Fatalf("run %d: record %d gave different canonical bytes", run, i)
			}
			if string(first[i].tenantPart) != string(again[i].tenantPart) || string(first[i].personalPart) != string(again[i].personalPart) {
				t.Fatalf("run %d: record %d gave different part bytes", run, i)
			}
		}
	}
}

func TestSeal_CommitsToEachErasablePartThatExists(t *testing.T) {
	bodies := buildChain(t, keylessKey())

	both := bodies[1]
	for name, part := range map[string]struct{ bytes, salt []byte }{
		"tenant":   {both.tenantPart, both.tenantSalt},
		"personal": {both.personalPart, both.personalSalt},
	} {
		sum := sha256.Sum256(append(append([]byte(nil), part.salt...), part.bytes...))
		want := `"` + name + `":"` + hex.EncodeToString(sum[:]) + `"`
		if !strings.Contains(string(both.Canonical()), want) {
			t.Fatalf("the retained part does not carry the %s commitment %s:\n%s", name, want, both.Canonical())
		}
	}
	for _, erasable := range []string{"user:5d4c3b2a", "Prüfung", fixtureOrg, fixtureTrace, "files:read"} {
		if strings.Contains(string(both.Canonical()), erasable) {
			t.Fatalf("an erasable value %q entered the canonical bytes:\n%s", erasable, both.Canonical())
		}
	}

	tenantOnly := bodies[2]
	if tenantOnly.personalPart != nil || strings.Contains(string(tenantOnly.Canonical()), `"personal"`) {
		t.Fatalf("a record with no personal part commits to one:\n%s", tenantOnly.Canonical())
	}

	none, err := NewRecord(bodies[0].Head(), Draft{
		EventID:   fixtureActionEvent,
		Type:      "action",
		Timestamp: fixtureStart,
	})
	if err != nil {
		t.Fatalf("NewRecord: %v", err)
	}
	if !strings.Contains(string(none.Canonical()), `"commitments":{}`) {
		t.Fatalf("a record with no erasable part does not carry empty commitments:\n%s", none.Canonical())
	}
}

func TestSeal_RefusesInvalidDrafts(t *testing.T) {
	head := Head{ChainID: fixtureChainID, Seq: 0, Hash: strings.Repeat("a", 64)}
	valid := func() Draft {
		return Draft{
			EventID:    fixtureActionEvent,
			Type:       "action",
			Timestamp:  fixtureStart,
			Retained:   map[string]any{"opena2a": map[string]any{"source": "service"}},
			Tenant:     map[string]any{"trace_id": fixtureTrace},
			TenantSalt: fixtureSalt(1),
		}
	}
	if _, err := NewRecord(head, valid()); err != nil {
		t.Fatalf("the valid draft is refused: %v", err)
	}

	cases := map[string]func(d *Draft){
		"event_id not a UUID":       func(d *Draft) { d.EventID = "not-a-uuid" },
		"event_id in upper case":    func(d *Draft) { d.EventID = strings.ToUpper(fixtureActionEvent) },
		"no type":                   func(d *Draft) { d.Type = "" },
		"genesis type after start":  func(d *Draft) { d.Type = TypeChainGenesis },
		"no timestamp":              func(d *Draft) { d.Timestamp = time.Time{} },
		"caller sets type":          func(d *Draft) { d.Retained["type"] = "other" },
		"caller sets timestamp":     func(d *Draft) { d.Retained["timestamp"] = "2026-01-01T00:00:00.000000Z" },
		"caller sets chain":         func(d *Draft) { d.Retained["opena2a"].(map[string]any)["chain"] = map[string]any{"seq": 9} },
		"caller sets schema":        func(d *Draft) { d.Retained["opena2a"].(map[string]any)["schema"] = "x" },
		"caller sets commitments":   func(d *Draft) { d.Retained["opena2a"].(map[string]any)["commitments"] = map[string]any{} },
		"caller sets sig_algs":      func(d *Draft) { d.Retained["opena2a"].(map[string]any)["sig_algs"] = []any{} },
		"caller sets first_key":     func(d *Draft) { d.Retained["opena2a"].(map[string]any)["first_key"] = map[string]any{} },
		"caller sets pre_genesis":   func(d *Draft) { d.Retained["opena2a"].(map[string]any)["pre_genesis"] = map[string]any{} },
		"opena2a not an object":     func(d *Draft) { d.Retained["opena2a"] = "x" },
		"float in retained":         func(d *Draft) { d.Retained["score"] = 0.5 },
		"float in tenant":           func(d *Draft) { d.Tenant["score"] = 0.5 },
		"salt too short":            func(d *Draft) { d.TenantSalt = d.TenantSalt[:16] },
		"part without salt":         func(d *Draft) { d.TenantSalt = nil },
		"salt without part":         func(d *Draft) { d.PersonalSalt = fixtureSalt(2) },
		"tenant opena2a not object": func(d *Draft) { d.Tenant["opena2a"] = 1 },
		"member in two parts": func(d *Draft) {
			d.Personal = map[string]any{"trace_id": fixtureTrace}
			d.PersonalSalt = fixtureSalt(2)
		},
		"member in retained and tenant": func(d *Draft) { d.Retained["trace_id"] = fixtureTrace },
		"type in an erasable part":      func(d *Draft) { d.Tenant["type"] = "action" },
		"opena2a member in two parts": func(d *Draft) {
			d.Tenant["opena2a"] = map[string]any{"source": "service"}
		},
		"chain in an erasable part": func(d *Draft) {
			d.Tenant["opena2a"] = map[string]any{"chain": map[string]any{"seq": 0}}
		},
	}
	for name, mutate := range cases {
		d := valid()
		mutate(&d)
		if _, err := NewRecord(head, d); !errors.Is(err, ErrInvalidRecord) {
			t.Errorf("%s: got %v; want ErrInvalidRecord", name, err)
		}
	}

	for name, h := range map[string]Head{
		"chain id not a UUID": {ChainID: "x", Seq: 0, Hash: head.Hash},
		"negative sequence":   {ChainID: fixtureChainID, Seq: -1, Hash: head.Hash},
		"hash not a digest":   {ChainID: fixtureChainID, Seq: 0, Hash: "abc"},
		"hash in upper case":  {ChainID: fixtureChainID, Seq: 0, Hash: strings.Repeat("A", 64)},
	} {
		if _, err := NewRecord(h, valid()); !errors.Is(err, ErrInvalidRecord) {
			t.Errorf("head with %s: got %v; want ErrInvalidRecord", name, err)
		}
	}
}

func TestNewGenesis_RefusesInvalidInput(t *testing.T) {
	cases := map[string]func(g *Genesis){
		"another type":      func(g *Genesis) { g.Draft.Type = "action" },
		"chain id not UUID": func(g *Genesis) { g.ChainID = "chain-1" },
		"unsupported key":   func(g *Genesis) { g.FirstKey.Alg = "ES256" },
		"short key":         func(g *Genesis) { g.FirstKey.Key = g.FirstKey.Key[:16] },
		"unknown table": func(g *Genesis) {
			g.PreGenesis = []PreGenesisRow{{Table: "agents", ID: fixtureAgent, Timestamp: fixtureStart}}
		},
		"row id not UUID": func(g *Genesis) {
			g.PreGenesis = []PreGenesisRow{{Table: TableAuditLogs, ID: "7", Timestamp: fixtureStart}}
		},
		"row with no time":   func(g *Genesis) { g.PreGenesis = []PreGenesisRow{{Table: TableAuditLogs, ID: fixtureAgent}} },
		"row named twice":    func(g *Genesis) { g.PreGenesis = append(fixturePreGenesis(), fixturePreGenesis()[0]) },
		"caller pre_genesis": func(g *Genesis) { g.Draft.Retained["opena2a"].(map[string]any)["pre_genesis"] = map[string]any{} },
	}
	for name, mutate := range cases {
		g := fixtureGenesis(keylessKey(), fixturePreGenesis())
		mutate(&g)
		if _, err := NewGenesis(g); !errors.Is(err, ErrInvalidRecord) {
			t.Errorf("%s: got %v; want ErrInvalidRecord", name, err)
		}
	}
}

func TestSetDigest_IsTheHashOfTheSortedRows(t *testing.T) {
	// Written out by hand: sorted by table, then id; ids in lowercase
	// hyphenated form; timestamps in UTC with six fractional digits.
	const serialized = `[["audit_logs","b2b2b2b2-0000-4000-8000-000000000001","2025-11-15T10:30:00.000000Z"],` +
		`["audit_logs","b2b2b2b2-0000-4000-8000-000000000002","2025-12-31T23:59:59.999999Z"],` +
		`["verification_events","c3c3c3c3-0000-4000-8000-000000000001","2025-12-30T08:00:00.000000Z"]]`
	sum := sha256.Sum256([]byte(serialized))
	want := hex.EncodeToString(sum[:])

	rows := fixturePreGenesis()
	got, err := SetDigest(rows)
	if err != nil {
		t.Fatalf("SetDigest: %v", err)
	}
	if got != want {
		t.Fatalf("got %s; want %s", got, want)
	}
	reordered := []PreGenesisRow{rows[2], rows[0], rows[1]}
	if again, err := SetDigest(reordered); err != nil || again != want {
		t.Fatalf("the digest depends on the order the rows are given in: %s, %v", again, err)
	}
	if rows[0].Table != TableVerificationEvents {
		t.Fatalf("SetDigest reordered its input")
	}
}

func TestSetDigest_OfNoRowsIsTheHashOfTheEmptyArray(t *testing.T) {
	// SHA-256 of the two bytes "[]".
	const want = "4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945"
	for name, rows := range map[string][]PreGenesisRow{"nil": nil, "empty": {}} {
		got, err := SetDigest(rows)
		if err != nil {
			t.Fatalf("%s: SetDigest: %v", name, err)
		}
		if got != want {
			t.Fatalf("%s: got %s; want %s", name, got, want)
		}
	}
}
