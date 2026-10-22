package record

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// Fixed inputs of the correction the tests and the vector share.
const (
	fixtureCorrectionEvent   = "a1a1a1a1-0000-4000-8000-000000000007"
	fixtureCorrectionTrace   = "5b8aa5a2d2c872e8321cf37308d69df2"
	fixtureCorrectedResource = "d4d4d4d4-0000-4000-8000-000000000002"
	fixtureLaterEvent        = "a1a1a1a1-0000-4000-8000-000000000008"
)

// fixtureCorrectionDraft is the draft of a correction a user writes to the
// fixture's administrative record: the resource that record names was another
// one.
func fixtureCorrectionDraft() Draft {
	return Draft{
		EventID:   fixtureCorrectionEvent,
		Timestamp: fixtureStart.Add(7 * time.Minute),
		Retained:  fixtureRetained("service", map[string]any{"trace_origin": "server"}),
		Tenant: map[string]any{
			"trace_id":    fixtureCorrectionTrace,
			"parent_id":   nil,
			"resource_id": fixtureCorrectedResource,
			"opena2a":     map[string]any{"organization_id": fixtureOrg},
		},
		TenantSalt:   fixtureSalt(0x71),
		Personal:     map[string]any{"actor": "user:5d4c3b2a-1f0e-4d9c-8b7a-6f5e4d3c2b1a"},
		PersonalSalt: fixtureSalt(0x72),
	}
}

// correctionDraft is the fixture correction with another event id.
func correctionDraft(eventID string) Draft {
	d := fixtureCorrectionDraft()
	d.EventID = eventID
	return d
}

// correctedBodies builds a genesis that names first, the fixture's
// administrative record, and a correction of that record.
func correctedBodies(t *testing.T, first PublicKey) []Body {
	t.Helper()
	b := built(t)
	genesis := b(NewGenesis(fixtureGenesis(first, nil)))
	admin := b(NewRecord(genesis.Head(), fixtureDrafts()[2]))
	correction := b(NewCorrection(admin.Head(), Correction{
		CorrectedEventID: fixtureAdminEvent,
		Draft:            fixtureCorrectionDraft(),
	}))
	return []Body{genesis, admin, correction}
}

func signAll(t *testing.T, bodies []Body, kp *memoryKeyProvider) []Record {
	t.Helper()
	records := make([]Record, len(bodies))
	for i, body := range bodies {
		records[i] = mustSign(t, body, kp)
	}
	return records
}

// tenantStrings returns the string values of a tenant part, at its top level
// and under "opena2a".
func tenantStrings(t *testing.T, part []byte) []string {
	t.Helper()
	if len(part) == 0 {
		return nil
	}
	var doc map[string]any
	if err := json.Unmarshal(part, &doc); err != nil {
		t.Fatalf("the tenant part is not JSON: %v", err)
	}
	var out []string
	ext, _ := doc["opena2a"].(map[string]any)
	for _, members := range []map[string]any{doc, ext} {
		for _, v := range members {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

// signedCorrectedChain signs correctedBodies with a key generated for this
// test.
func signedCorrectedChain(t *testing.T) ([]Record, *memoryKeyProvider) {
	t.Helper()
	kp := newMemoryKeyProvider(t)
	return signAll(t, correctedBodies(t, kp.key(t)), kp), kp
}

func TestNewCorrection_NamesTheCorrectedRecordInItsTenantPart(t *testing.T) {
	bodies := correctedBodies(t, keylessKey())
	admin, correction := bodies[1], bodies[2]
	canonical := string(correction.Canonical())
	for _, want := range []string{
		`"type":"opena2a.correction"`,
		`"event_id":"` + fixtureCorrectionEvent + `"`,
		`"chain":{"id":"` + fixtureChainID + `","prev_hash":"` + admin.Head().Hash + `","seq":2}`,
		`"tenant":"` + commitment(fixtureSalt(0x71), correction.tenantPart) + `"`,
	} {
		if !strings.Contains(canonical, want) {
			t.Errorf("the correction's canonical bytes lack %s:\n%s", want, canonical)
		}
	}
	// A reference from one record to another is a tenant value, so the bytes
	// the chain covers do not name the corrected record.
	if strings.Contains(canonical, "corrected_event_id") || strings.Contains(canonical, fixtureAdminEvent) {
		t.Errorf("the retained part names the corrected record:\n%s", canonical)
	}
	wantTenant := `{"opena2a":{"corrected_event_id":"` + fixtureAdminEvent + `","organization_id":"` + fixtureOrg + `"},` +
		`"parent_id":null,"resource_id":"` + fixtureCorrectedResource + `","trace_id":"` + fixtureCorrectionTrace + `"}`
	if got := string(correction.tenantPart); got != wantTenant {
		t.Errorf("tenant part\n%s\nwant\n%s", got, wantTenant)
	}
	if got := string(correction.personalPart); got != `{"actor":"user:5d4c3b2a-1f0e-4d9c-8b7a-6f5e4d3c2b1a"}` {
		t.Errorf("personal part %s", got)
	}
	if got, want := correction.Head(), (Head{ChainID: fixtureChainID, Seq: 2, Hash: hashHex(correction.Canonical())}); got != want {
		t.Errorf("head %+v; want %+v", got, want)
	}
}

func TestNewCorrection_LeavesTheCallersDraftUnchanged(t *testing.T) {
	head := Head{ChainID: fixtureChainID, Seq: 1, Hash: strings.Repeat("a", 64)}
	d := fixtureCorrectionDraft()
	for _, corrected := range []string{fixtureAdminEvent, fixtureGenesisEvent} {
		if _, err := NewCorrection(head, Correction{CorrectedEventID: corrected, Draft: d}); err != nil {
			t.Fatalf("a correction of %s from the same draft is refused: %v", corrected, err)
		}
	}
	if _, set := d.Tenant["opena2a"].(map[string]any)["corrected_event_id"]; set {
		t.Fatalf("NewCorrection wrote the reference into the caller's tenant part")
	}
}

func TestNewCorrection_RefusesInvalidInput(t *testing.T) {
	head := Head{ChainID: fixtureChainID, Seq: 1, Hash: strings.Repeat("a", 64)}
	valid := func() Correction {
		return Correction{CorrectedEventID: fixtureAdminEvent, Draft: fixtureCorrectionDraft()}
	}
	if _, err := NewCorrection(head, valid()); err != nil {
		t.Fatalf("the valid correction is refused: %v", err)
	}

	cases := map[string]func(c *Correction){
		"another type":                            func(c *Correction) { c.Draft.Type = "opena2a.administrative" },
		"the genesis type":                        func(c *Correction) { c.Draft.Type = TypeChainGenesis },
		"the key rotation type":                   func(c *Correction) { c.Draft.Type = TypeKeyRotation },
		"event_id not a UUID":                     func(c *Correction) { c.Draft.EventID = "not-a-uuid" },
		"no corrected event id":                   func(c *Correction) { c.CorrectedEventID = "" },
		"a corrected event id that is not a UUID": func(c *Correction) { c.CorrectedEventID = "not-a-uuid" },
		"a corrected event id in upper case":      func(c *Correction) { c.CorrectedEventID = strings.ToUpper(fixtureAdminEvent) },
		"a corrected event id in braces":          func(c *Correction) { c.CorrectedEventID = "{" + fixtureAdminEvent + "}" },
		"a correction of itself":                  func(c *Correction) { c.CorrectedEventID = c.Draft.EventID },
		"no tenant salt":                          func(c *Correction) { c.Draft.TenantSalt = nil },
		"no tenant part and no tenant salt":       func(c *Correction) { c.Draft.Tenant, c.Draft.TenantSalt = nil, nil },
		"tenant opena2a not an object":            func(c *Correction) { c.Draft.Tenant["opena2a"] = "x" },
		"caller sets next_key": func(c *Correction) {
			c.Draft.Retained["opena2a"].(map[string]any)["next_key"] = map[string]any{}
		},
	}
	// The reference is the package's to set, in whichever part a caller puts it.
	for part, members := range map[string]func(d *Draft) map[string]any{
		"tenant":   func(d *Draft) map[string]any { return d.Tenant["opena2a"].(map[string]any) },
		"retained": func(d *Draft) map[string]any { return d.Retained["opena2a"].(map[string]any) },
		"personal": func(d *Draft) map[string]any {
			d.Personal["opena2a"] = map[string]any{}
			return d.Personal["opena2a"].(map[string]any)
		},
	} {
		cases["caller sets corrected_event_id in the "+part+" part"] = func(c *Correction) {
			members(&c.Draft)["corrected_event_id"] = fixtureAdminEvent
		}
	}
	for name, mutate := range cases {
		c := valid()
		mutate(&c)
		if _, err := NewCorrection(head, c); !errors.Is(err, ErrInvalidRecord) {
			t.Errorf("%s: got %v; want ErrInvalidRecord", name, err)
		}
	}

	for name, h := range map[string]Head{
		"chain id not a UUID": {ChainID: "x", Seq: 1, Hash: head.Hash},
		"negative sequence":   {ChainID: fixtureChainID, Seq: -1, Hash: head.Hash},
		"hash not a digest":   {ChainID: fixtureChainID, Seq: 1, Hash: "abc"},
	} {
		if _, err := NewCorrection(h, valid()); !errors.Is(err, ErrInvalidRecord) {
			t.Errorf("head with %s: got %v; want ErrInvalidRecord", name, err)
		}
	}
}

func TestBuilders_RefuseACorrectedEventIDTheCallerSets(t *testing.T) {
	head := Head{ChainID: fixtureChainID, Seq: 1, Hash: strings.Repeat("a", 64)}
	withReference := func(d Draft) Draft {
		d.Tenant["opena2a"].(map[string]any)["corrected_event_id"] = fixtureAdminEvent
		return d
	}
	genesis := fixtureGenesis(keylessKey(), nil)
	genesis.Draft = withReference(genesis.Draft)
	if _, err := NewGenesis(genesis); !errors.Is(err, ErrInvalidRecord) {
		t.Errorf("NewGenesis: got %v; want ErrInvalidRecord", err)
	}
	if _, err := NewRecord(head, withReference(fixtureDrafts()[2])); !errors.Is(err, ErrInvalidRecord) {
		t.Errorf("NewRecord: got %v; want ErrInvalidRecord", err)
	}
	if _, err := NewKeyRotation(head, KeyRotation{
		NextKey: keylessNextKey(1),
		Reason:  RotationScheduled,
		Draft:   withReference(fixtureRotationDraft(fixtureRotationEvent, 4*time.Minute, 0x51)),
	}); !errors.Is(err, ErrInvalidRecord) {
		t.Errorf("NewKeyRotation: got %v; want ErrInvalidRecord", err)
	}
}

func TestVerify_AcceptsACorrectionOfAnEarlierRecord(t *testing.T) {
	records, kp := signedCorrectedChain(t)
	if res := mustVerify(t, records, kp.key(t)); !res.OK() || res.Verified != len(records) {
		t.Fatalf("a chain with a correction fails: %+v", res.Failure)
	}

	t.Run("a correction of the genesis and a correction of a correction", func(t *testing.T) {
		b := built(t)
		bodies := correctedBodies(t, kp.key(t))
		ofCorrection := b(NewCorrection(bodies[2].Head(), Correction{
			CorrectedEventID: fixtureCorrectionEvent,
			Draft:            correctionDraft("a1a1a1a1-0000-4000-8000-000000000009"),
		}))
		ofGenesis := b(NewCorrection(ofCorrection.Head(), Correction{
			CorrectedEventID: fixtureGenesisEvent,
			Draft:            correctionDraft("a1a1a1a1-0000-4000-8000-00000000000a"),
		}))
		chain := signAll(t, append(bodies, ofCorrection, ofGenesis), kp)
		if res := mustVerify(t, chain, kp.key(t)); !res.OK() || res.Verified != len(chain) {
			t.Fatalf("the corrections fail: %+v", res.Failure)
		}
	})

	t.Run("a correction of a key rotation", func(t *testing.T) {
		rotated, keys := signedTwiceRotatedChain(t)
		last := rotated[len(rotated)-1]
		head := Head{ChainID: fixtureChainID, Seq: int64(len(rotated) - 1), Hash: last.RecordHash}
		correction := built(t)(NewCorrection(head, Correction{CorrectedEventID: fixtureRotationEvent, Draft: fixtureCorrectionDraft()}))
		chain := append(cloneRecords(rotated), mustSign(t, correction, keys[2]))
		if res := mustVerify(t, chain, keys[0].key(t)); !res.OK() || res.Verified != len(chain) {
			t.Fatalf("a correction of a key rotation fails: %+v", res.Failure)
		}
	})
}

func TestVerify_FailsACorrectionThatNamesNoEarlierRecord(t *testing.T) {
	kp := newMemoryKeyProvider(t)
	bodies := correctedBodies(t, kp.key(t))
	admin := bodies[1].Head()
	before := signAll(t, bodies[:2], kp)

	// sealed builds a correction at sequence 2 whose tenant part carries the
	// given members under "opena2a", past the checks of NewCorrection, and
	// signs it with the chain's key: only the verifier stands between it and
	// the chain.
	sealed := func(d Draft, reference map[string]any) Record {
		t.Helper()
		d.Type = TypeCorrection
		body, err := seal(admin.ChainID, admin.Seq+1, admin.Hash, d, owned{tenant: reference})
		if err != nil {
			t.Fatalf("seal: %v", err)
		}
		return mustSign(t, body, kp)
	}
	naming := func(v any) map[string]any { return map[string]any{"corrected_event_id": v} }
	noTenant := fixtureCorrectionDraft()
	noTenant.Tenant, noTenant.TenantSalt = nil, nil

	for name, c := range map[string]struct {
		correction Record
		detail     string
	}{
		"an event id that no record of the chain carries": {
			sealed(fixtureCorrectionDraft(), naming("e5e5e5e5-0000-4000-8000-000000000001")),
			"is the event id of no record before the correction",
		},
		"its own event id": {
			sealed(fixtureCorrectionDraft(), naming(fixtureCorrectionEvent)),
			"is the event id of no record before the correction",
		},
		"an event id in upper case": {
			sealed(fixtureCorrectionDraft(), naming(strings.ToUpper(fixtureAdminEvent))),
			"is not a lowercase hyphenated UUID",
		},
		"an event id that is a number": {
			sealed(fixtureCorrectionDraft(), naming(int64(7))),
			"is not a lowercase hyphenated UUID",
		},
		"an event id that is null": {
			sealed(fixtureCorrectionDraft(), naming(nil)),
			"is not a lowercase hyphenated UUID",
		},
		"a tenant part with no corrected_event_id": {
			sealed(fixtureCorrectionDraft(), nil),
			"does not name the corrected record",
		},
		"no tenant part": {
			sealed(noTenant, nil),
			"commits to no tenant part",
		},
	} {
		t.Run(name, func(t *testing.T) {
			res := mustVerify(t, append(cloneRecords(before), c.correction), kp.key(t))
			wantFailure(t, res, 2, FailureCorrection)
			if !strings.Contains(res.Failure.Detail, c.detail) {
				t.Fatalf("detail %q; want it to contain %q", res.Failure.Detail, c.detail)
			}
			// The tenant part's values are erasable, so a failure that can
			// reach a log quotes none of them.
			for _, value := range tenantStrings(t, c.correction.TenantPart) {
				if strings.Contains(res.Failure.Detail, value) {
					t.Fatalf("detail %q quotes %q from the tenant part", res.Failure.Detail, value)
				}
			}
		})
	}

	t.Run("the event id of a record after it", func(t *testing.T) {
		correction := sealed(fixtureCorrectionDraft(), naming(fixtureLaterEvent))
		later := fixtureDrafts()[2]
		later.EventID = fixtureLaterEvent
		later.Timestamp = fixtureStart.Add(8 * time.Minute)
		after := built(t)(NewRecord(Head{ChainID: fixtureChainID, Seq: 2, Hash: correction.RecordHash}, later))
		chain := append(cloneRecords(before), correction, mustSign(t, after, kp))
		res := mustVerify(t, chain, kp.key(t))
		wantFailure(t, res, 2, FailureCorrection)
		for _, value := range tenantStrings(t, correction.TenantPart) {
			if strings.Contains(res.Failure.Detail, value) {
				t.Fatalf("detail %q quotes %q from the tenant part", res.Failure.Detail, value)
			}
		}
	})
}

func TestVerify_ErasedPartsLeaveACorrectionVerifying(t *testing.T) {
	records, kp := signedCorrectedChain(t)

	t.Run("the correction's tenant part, which holds its reference", func(t *testing.T) {
		erased := cloneRecords(records)
		erased[2].TenantPart, erased[2].TenantSalt = nil, nil
		if res := mustVerify(t, erased, kp.key(t)); !res.OK() || res.Verified != len(records) {
			t.Fatalf("a correction whose tenant part is erased fails: %+v", res.Failure)
		}
	})

	t.Run("the corrected record's erasable parts", func(t *testing.T) {
		// The corrected record's event_id is in its retained part, so the
		// reference still resolves.
		erased := cloneRecords(records)
		erased[1].TenantPart, erased[1].TenantSalt = nil, nil
		erased[1].PersonalPart, erased[1].PersonalSalt = nil, nil
		if res := mustVerify(t, erased, kp.key(t)); !res.OK() || res.Verified != len(records) {
			t.Fatalf("a correction of a record whose erasable parts are erased fails: %+v", res.Failure)
		}
	})

	t.Run("every erasable part of every record", func(t *testing.T) {
		erased := cloneRecords(records)
		for i := range erased {
			erased[i].TenantPart, erased[i].TenantSalt = nil, nil
			erased[i].PersonalPart, erased[i].PersonalSalt = nil, nil
		}
		if res := mustVerify(t, erased, kp.key(t)); !res.OK() || res.Verified != len(records) {
			t.Fatalf("a corrected chain whose erasable parts are gone fails: %+v", res.Failure)
		}
	})
}

func TestVerify_ReportsAReferenceChangedAfterTheCorrectionWasWritten(t *testing.T) {
	records, kp := signedCorrectedChain(t)
	// The stored tenant part now names the genesis, a record before the
	// correction, but the record committed to the part that named the
	// administrative record.
	tampered := cloneRecords(records)
	tampered[2].TenantPart = []byte(strings.Replace(string(records[2].TenantPart), fixtureAdminEvent, fixtureGenesisEvent, 1))
	if string(tampered[2].TenantPart) == string(records[2].TenantPart) {
		t.Fatalf("the tenant part does not name the administrative record")
	}
	wantFailure(t, mustVerify(t, tampered, kp.key(t)), 2, FailureModified)
}
