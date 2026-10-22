package record

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// Fixed inputs of the key rotation records the tests and the vectors share.
const (
	fixtureRotationEvent         = "a1a1a1a1-0000-4000-8000-000000000004"
	fixtureExposureRotationEvent = "a1a1a1a1-0000-4000-8000-000000000005"
	fixtureAfterRotationEvent    = "a1a1a1a1-0000-4000-8000-000000000006"
)

// fixtureExposureBegan is when the exposure of the second key is believed to
// have begun. It is given in another zone and with nanoseconds, so the vector
// pins its rendering.
var fixtureExposureBegan = fixtureStart.Add(4*time.Minute + 30*time.Second + 999).In(time.FixedZone("", -5*60*60))

// keylessNextKey is a fixed public key with no private half for the vectors'
// rotations: a different key for each n, none of them keylessKey.
func keylessNextKey(n int) PublicKey {
	sum := sha256.Sum256([]byte(fmt.Sprintf("opena2a audit-record vectors: next key %d, a public key with no private half", n)))
	return PublicKey{Alg: AlgEd25519, Key: sum[:]}
}

// fixtureRotationDraft is the draft of a key rotation the system writes at
// the given time after fixtureStart.
func fixtureRotationDraft(eventID string, after time.Duration, salt byte) Draft {
	return Draft{
		EventID:   eventID,
		Timestamp: fixtureStart.Add(after),
		Retained:  fixtureRetained("system", map[string]any{"trace_origin": "server"}),
		Tenant: map[string]any{
			"trace_id":  fixtureTrace,
			"parent_id": nil,
			"opena2a":   map[string]any{"organization_id": fixtureOrg},
		},
		TenantSalt: fixtureSalt(salt),
	}
}

// built returns a function that fails the test when a record cannot be built.
func built(t *testing.T) func(Body, error) Body {
	return func(b Body, err error) Body {
		t.Helper()
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		return b
	}
}

func mustSign(t *testing.T, b Body, kp *memoryKeyProvider) Record {
	t.Helper()
	rec, err := Sign(context.Background(), b, kp)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return rec
}

// twiceRotatedBodies builds a genesis that names keys[0], a scheduled rotation
// to keys[1], a rotation on exposure to keys[2], and the fixture's
// administrative record after them.
func twiceRotatedBodies(t *testing.T, keys [3]PublicKey) []Body {
	t.Helper()
	b := built(t)
	genesis := b(NewGenesis(fixtureGenesis(keys[0], nil)))
	scheduled := b(NewKeyRotation(genesis.Head(), KeyRotation{
		NextKey: keys[1],
		Reason:  RotationScheduled,
		Draft:   fixtureRotationDraft(fixtureRotationEvent, 4*time.Minute, 0x51),
	}))
	exposure := b(NewKeyRotation(scheduled.Head(), KeyRotation{
		NextKey:       keys[2],
		Reason:        RotationExposure,
		ExposureBegan: fixtureExposureBegan,
		Draft:         fixtureRotationDraft(fixtureExposureRotationEvent, 5*time.Minute, 0x61),
	}))
	after := fixtureDrafts()[2]
	after.EventID = fixtureAfterRotationEvent
	after.Timestamp = fixtureStart.Add(6 * time.Minute)
	return []Body{genesis, scheduled, exposure, b(NewRecord(exposure.Head(), after))}
}

// signedTwiceRotatedChain signs twiceRotatedBodies with three keys generated
// for this test: each rotation by the key it retires, and the record after
// them by the third key.
func signedTwiceRotatedChain(t *testing.T) ([]Record, [3]*memoryKeyProvider) {
	t.Helper()
	keys := [3]*memoryKeyProvider{newMemoryKeyProvider(t), newMemoryKeyProvider(t), newMemoryKeyProvider(t)}
	bodies := twiceRotatedBodies(t, [3]PublicKey{keys[0].key(t), keys[1].key(t), keys[2].key(t)})
	signers := []*memoryKeyProvider{keys[0], keys[0], keys[1], keys[2]}
	records := make([]Record, len(bodies))
	for i, body := range bodies {
		records[i] = mustSign(t, body, signers[i])
	}
	return records, keys
}

// resigned returns rec with the one occurrence of old in its payload replaced
// by new and the stored hash recomputed, signed again by kp, with its erasable
// parts kept.
func resigned(t *testing.T, rec Record, kp *memoryKeyProvider, old, new string) Record {
	t.Helper()
	out := signRaw(t, kp, ClassRecordV1, replaceInPayload(t, payloadBytes(t, rec), old, new))
	out.TenantPart, out.TenantSalt = rec.TenantPart, rec.TenantSalt
	out.PersonalPart, out.PersonalSalt = rec.PersonalPart, rec.PersonalSalt
	return out
}

// keyMemberText is the canonical text of the member that names k.
func keyMemberText(k PublicKey) string {
	return `"keyid":"` + k.KeyID() + `","public_key":"` + hex.EncodeToString(k.Key) + `"`
}

func TestNewKeyRotation_NamesTheNextKeyAndItsReason(t *testing.T) {
	b := built(t)
	genesis := b(NewGenesis(fixtureGenesis(keylessKey(), nil)))
	next := keylessNextKey(1)
	rotation := b(NewKeyRotation(genesis.Head(), KeyRotation{
		NextKey: next,
		Reason:  RotationScheduled,
		Draft:   fixtureRotationDraft(fixtureRotationEvent, 4*time.Minute, 0x51),
	}))
	canonical := string(rotation.Canonical())
	for _, want := range []string{
		`"type":"opena2a.key_rotation"`,
		`"next_key":{"alg":"Ed25519",` + keyMemberText(next) + `}`,
		`"rotation_reason":"scheduled"`,
		`"chain":{"id":"` + fixtureChainID + `","prev_hash":"` + genesis.Head().Hash + `","seq":1}`,
	} {
		if !strings.Contains(canonical, want) {
			t.Errorf("the rotation's canonical bytes lack %s:\n%s", want, canonical)
		}
	}
	if strings.Contains(canonical, "exposure_began_at") {
		t.Errorf("a scheduled rotation states an exposure time:\n%s", canonical)
	}
	if got, want := rotation.Head(), (Head{ChainID: fixtureChainID, Seq: 1, Hash: hashHex(rotation.Canonical())}); got != want {
		t.Errorf("head %+v; want %+v", got, want)
	}
}

func TestNewKeyRotation_OnExposureStatesWhenTheExposureBegan(t *testing.T) {
	b := built(t)
	genesis := b(NewGenesis(fixtureGenesis(keylessKey(), nil)))
	rotation := b(NewKeyRotation(genesis.Head(), KeyRotation{
		NextKey:       keylessNextKey(1),
		Reason:        RotationExposure,
		ExposureBegan: fixtureExposureBegan,
		Draft:         fixtureRotationDraft(fixtureExposureRotationEvent, 5*time.Minute, 0x61),
	}))
	// In UTC and truncated to microseconds, as every timestamp of a record.
	for _, want := range []string{`"exposure_began_at":"2026-01-02T03:08:35.123457Z"`, `"rotation_reason":"exposure"`} {
		if !strings.Contains(string(rotation.Canonical()), want) {
			t.Errorf("the rotation's canonical bytes lack %s:\n%s", want, rotation.Canonical())
		}
	}
}

func TestNewKeyRotation_RefusesInvalidInput(t *testing.T) {
	head := Head{ChainID: fixtureChainID, Seq: 0, Hash: strings.Repeat("a", 64)}
	next := keylessNextKey(1)
	valid := func() KeyRotation {
		return KeyRotation{
			NextKey: next,
			Reason:  RotationScheduled,
			Draft:   fixtureRotationDraft(fixtureRotationEvent, 4*time.Minute, 0x51),
		}
	}
	if _, err := NewKeyRotation(head, valid()); err != nil {
		t.Fatalf("the valid rotation is refused: %v", err)
	}

	setExt := func(name string, v any) func(r *KeyRotation) {
		return func(r *KeyRotation) { r.Draft.Retained["opena2a"].(map[string]any)[name] = v }
	}
	cases := map[string]func(r *KeyRotation){
		"another type":                    func(r *KeyRotation) { r.Draft.Type = "action" },
		"the genesis type":                func(r *KeyRotation) { r.Draft.Type = TypeChainGenesis },
		"event_id not a UUID":             func(r *KeyRotation) { r.Draft.EventID = "not-a-uuid" },
		"no next key":                     func(r *KeyRotation) { r.NextKey = PublicKey{} },
		"a next key of another algorithm": func(r *KeyRotation) { r.NextKey = PublicKey{Alg: "ES256", Key: next.Key} },
		"a next key of 16 bytes":          func(r *KeyRotation) { r.NextKey = PublicKey{Alg: AlgEd25519, Key: next.Key[:16]} },
		"no reason":                       func(r *KeyRotation) { r.Reason = "" },
		"a reason outside the set":        func(r *KeyRotation) { r.Reason = "rotated" },
		"a reason in another case":        func(r *KeyRotation) { r.Reason = "Scheduled" },
		"exposure with no time":           func(r *KeyRotation) { r.Reason = RotationExposure },
		"a time on a scheduled rotation":  func(r *KeyRotation) { r.ExposureBegan = fixtureExposureBegan },
		"a time on a custody change": func(r *KeyRotation) {
			r.Reason = RotationCustodyChange
			r.ExposureBegan = fixtureExposureBegan
		},
		"an exposure time outside RFC 3339": func(r *KeyRotation) {
			r.Reason = RotationExposure
			r.ExposureBegan = time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC)
		},
		"caller sets next_key":          setExt("next_key", map[string]any{}),
		"caller sets rotation_reason":   setExt("rotation_reason", RotationScheduled),
		"caller sets exposure_began_at": setExt("exposure_began_at", "2026-01-01T00:00:00.000000Z"),
		"caller sets first_key":         setExt("first_key", map[string]any{}),
	}
	for name, mutate := range cases {
		r := valid()
		mutate(&r)
		if _, err := NewKeyRotation(head, r); !errors.Is(err, ErrInvalidRecord) {
			t.Errorf("%s: got %v; want ErrInvalidRecord", name, err)
		}
	}

	for name, h := range map[string]Head{
		"chain id not a UUID": {ChainID: "x", Seq: 0, Hash: head.Hash},
		"negative sequence":   {ChainID: fixtureChainID, Seq: -1, Hash: head.Hash},
		"hash not a digest":   {ChainID: fixtureChainID, Seq: 0, Hash: "abc"},
	} {
		if _, err := NewKeyRotation(h, valid()); !errors.Is(err, ErrInvalidRecord) {
			t.Errorf("head with %s: got %v; want ErrInvalidRecord", name, err)
		}
	}
}

func TestSign_RefusesARotationSignedByTheKeyItNames(t *testing.T) {
	current, next := newMemoryKeyProvider(t), newMemoryKeyProvider(t)
	b := built(t)
	genesis := b(NewGenesis(fixtureGenesis(current.key(t), nil)))
	rotation := b(NewKeyRotation(genesis.Head(), KeyRotation{
		NextKey: next.key(t),
		Reason:  RotationScheduled,
		Draft:   fixtureRotationDraft(fixtureRotationEvent, 4*time.Minute, 0x51),
	}))
	if _, err := Sign(context.Background(), rotation, next); !errors.Is(err, ErrSigning) {
		t.Fatalf("a rotation signed by the next key it names: got %v; want ErrSigning", err)
	}
	if _, err := Sign(context.Background(), rotation, current); err != nil {
		t.Fatalf("a rotation signed by the key it retires is refused: %v", err)
	}
}

func TestVerify_FollowsKeyRotationsByPosition(t *testing.T) {
	records, keys := signedTwiceRotatedChain(t)
	first := keys[0].key(t)
	res := mustVerify(t, records, first)
	if !res.OK() || res.Verified != len(records) {
		t.Fatalf("a chain with two key rotations fails: %+v", res.Failure)
	}
	if res.Head == nil || res.Head.Hash != records[len(records)-1].RecordHash {
		t.Fatalf("head %+v; want the last record's", res.Head)
	}
	// The key in force after each prefix: the first key after the genesis,
	// then the key each rotation names.
	wantKeys := []*memoryKeyProvider{keys[0], keys[1], keys[2], keys[2]}
	for n := 1; n <= len(records); n++ {
		prefix := mustVerify(t, records[:n], first)
		if !prefix.OK() || prefix.Verified != n {
			t.Fatalf("the first %d records fail: %+v", n, prefix.Failure)
		}
		if want := wantKeys[n-1].key(t).KeyID(); prefix.Key == nil || prefix.Key.KeyID() != want {
			t.Fatalf("after the first %d records the key in force is %+v; want key %s", n, prefix.Key, want)
		}
	}
	if empty := mustVerify(t, nil, first); empty.Key != nil {
		t.Fatalf("an empty sequence reports a key in force")
	}

	t.Run("a verifier that holds a later key and not the first", func(t *testing.T) {
		// The genesis names the first key, so a later key is reached only
		// from it.
		wantFailure(t, mustVerify(t, records, keys[2].key(t)), 0, FailureBadSignature)
	})
}

func TestVerify_RefusesARecordSignedByARetiredKey(t *testing.T) {
	records, keys := signedTwiceRotatedChain(t)
	first := keys[0].key(t)
	last := len(records) - 1

	// The record after both rotations, written again with a timestamp before
	// the genesis: whatever its timestamp says, the key that signs it decides.
	early := fixtureDrafts()[2]
	early.EventID = fixtureAfterRotationEvent
	early.Timestamp = fixtureStart.Add(-24 * time.Hour)
	body := built(t)(NewRecord(Head{ChainID: fixtureChainID, Seq: int64(last - 1), Hash: records[last-1].RecordHash}, early))

	for name, c := range map[string]struct {
		signer  *memoryKeyProvider
		retired int64
	}{
		"signed by the key the first rotation retired":  {keys[0], 1},
		"signed by the key the second rotation retired": {keys[1], 2},
	} {
		t.Run(name, func(t *testing.T) {
			tampered := append(cloneRecords(records[:last]), mustSign(t, body, c.signer))
			res := mustVerify(t, tampered, first)
			wantFailure(t, res, last, FailureRetiredKey)
			want := fmt.Sprintf("signed by key %s, which the key rotation at sequence %d retired", c.signer.key(t).KeyID(), c.retired)
			if !strings.Contains(res.Failure.Detail, want) {
				t.Fatalf("detail %q; want it to name the key and the rotation that retired it", res.Failure.Detail)
			}
		})
	}

	t.Run("the same record signed by the key in force", func(t *testing.T) {
		res := mustVerify(t, append(cloneRecords(records[:last]), mustSign(t, body, keys[2])), first)
		if !res.OK() {
			t.Fatalf("the record fails under the key in force: %+v", res.Failure)
		}
	})

	t.Run("a rotation signed by a key an earlier rotation retired", func(t *testing.T) {
		tampered := cloneRecords(records)
		tampered[2] = signRaw(t, keys[0], ClassRecordV1, payloadBytes(t, records[2]))
		tampered[2].TenantPart, tampered[2].TenantSalt = records[2].TenantPart, records[2].TenantSalt
		wantFailure(t, mustVerify(t, tampered, first), 2, FailureRetiredKey)
	})
}

func TestVerify_AcceptsALaterKeyOnlyThroughARotationTheKeyInForceSigns(t *testing.T) {
	records, keys := signedTwiceRotatedChain(t)
	first := keys[0].key(t)

	t.Run("a rotation signed by the key it names", func(t *testing.T) {
		tampered := cloneRecords(records)
		tampered[1] = signRaw(t, keys[1], ClassRecordV1, payloadBytes(t, records[1]))
		tampered[1].TenantPart, tampered[1].TenantSalt = records[1].TenantPart, records[1].TenantSalt
		wantFailure(t, mustVerify(t, tampered, first), 1, FailureBadSignature)
	})

	t.Run("a rotation signed by a key the chain never held", func(t *testing.T) {
		tampered := cloneRecords(records)
		tampered[1] = signRaw(t, newMemoryKeyProvider(t), ClassRecordV1, payloadBytes(t, records[1]))
		tampered[1].TenantPart, tampered[1].TenantSalt = records[1].TenantPart, records[1].TenantSalt
		wantFailure(t, mustVerify(t, tampered, first), 1, FailureBadSignature)
	})

	t.Run("a record under the next key with no rotation before it", func(t *testing.T) {
		// The fixture chain with no rotation, its last record signed by a
		// key the genesis does not name.
		plain, kp := signedChain(t)
		other := newMemoryKeyProvider(t)
		tampered := cloneRecords(plain)
		last := len(tampered) - 1
		tampered[last] = signRaw(t, other, ClassRecordV1, payloadBytes(t, plain[last]))
		tampered[last].TenantPart, tampered[last].TenantSalt = plain[last].TenantPart, plain[last].TenantSalt
		wantFailure(t, mustVerify(t, tampered, kp.key(t)), last, FailureBadSignature)
	})
}

func TestVerify_ReportsARotationThatCannotBeFollowed(t *testing.T) {
	records, keys := signedTwiceRotatedChain(t)
	first := keys[0].key(t)
	k0, k1, k2 := keys[0].key(t), keys[1].key(t), keys[2].key(t)
	if res := mustVerify(t, records[:3], first); !res.OK() {
		t.Fatalf("the two rotations fail unchanged: %+v", res.Failure)
	}

	// Each case changes one rotation and signs it again with the key in force
	// at its position, so its signature holds and only its content is wrong.
	for name, c := range map[string]struct {
		position int
		old, new string
	}{
		"no next key":                      {1, `"next_key":{"alg":"Ed25519",` + keyMemberText(k1) + `},`, ``},
		"a next key of another algorithm":  {1, `"alg":"Ed25519"`, `"alg":"ES256"`},
		"a next key in upper case hex":     {1, hex.EncodeToString(k1.Key), strings.ToUpper(hex.EncodeToString(k1.Key))},
		"a next key of 16 bytes":           {1, hex.EncodeToString(k1.Key), hex.EncodeToString(k1.Key[:16])},
		"a key id that is not its hash":    {1, k1.KeyID(), keylessKey().KeyID()},
		"the key in force as the next key": {1, keyMemberText(k1), keyMemberText(k0)},
		"a key a rotation retired":         {2, keyMemberText(k2), keyMemberText(k0)},
		"no reason":                        {1, `"rotation_reason":"scheduled",`, ``},
		"a reason outside the set":         {1, `"rotation_reason":"scheduled"`, `"rotation_reason":"rotated"`},
		"exposure with no time":            {1, `"rotation_reason":"scheduled"`, `"rotation_reason":"exposure"`},
		"a time on a scheduled rotation":   {1, `"issuer":`, `"exposure_began_at":"2026-01-02T03:08:35.123457Z","issuer":`},
		"a time on a custody change":       {2, `"rotation_reason":"exposure"`, `"rotation_reason":"custody_change"`},
		"an exposure time with no fraction": {2,
			`"exposure_began_at":"2026-01-02T03:08:35.123457Z"`, `"exposure_began_at":"2026-01-02T03:08:35Z"`},
		"an exposure time that is not a time": {2,
			`"exposure_began_at":"2026-01-02T03:08:35.123457Z"`, `"exposure_began_at":"yesterday"`},
	} {
		t.Run(name, func(t *testing.T) {
			signer := keys[c.position-1]
			tampered := append(cloneRecords(records[:c.position]), resigned(t, records[c.position], signer, c.old, c.new))
			wantFailure(t, mustVerify(t, tampered, first), c.position, FailureKeyRotation)
		})
	}
}

func TestVerify_ReportsTamperingInARotatedChain(t *testing.T) {
	records, keys := signedTwiceRotatedChain(t)
	first := keys[0].key(t)

	t.Run("one byte of the canonical bytes changed", func(t *testing.T) {
		for position := range records {
			tampered := cloneRecords(records)
			payload := payloadBytes(t, tampered[position])
			payload[len(payload)/2] ^= 0x01
			setPayload(&tampered[position], payload)
			wantFailure(t, mustVerify(t, tampered, first), position, FailureModified)
		}
	})

	t.Run("a rotation's reason changed and its stored hash recomputed", func(t *testing.T) {
		tampered := cloneRecords(records)
		payload := replaceInPayload(t, payloadBytes(t, tampered[1]), `"rotation_reason":"scheduled"`, `"rotation_reason":"custody_change"`)
		setPayload(&tampered[1], payload)
		tampered[1].RecordHash = hashHex(payload)
		wantFailure(t, mustVerify(t, tampered, first), 1, FailureModified)
	})

	t.Run("a rotation's next key changed and its stored hash recomputed", func(t *testing.T) {
		// The record after it is signed by the key the original bytes name,
		// which the changed bytes no longer name, so the walk sees bytes that
		// their signature does not cover.
		tampered := cloneRecords(records)
		payload := replaceInPayload(t, payloadBytes(t, tampered[1]), keyMemberText(keys[1].key(t)), keyMemberText(keylessKey()))
		setPayload(&tampered[1], payload)
		tampered[1].RecordHash = hashHex(payload)
		wantFailure(t, mustVerify(t, tampered, first), 1, FailureBadSignature)
	})

	t.Run("a record removed from the middle", func(t *testing.T) {
		// The record that now sits where the removed one was is signed by a
		// key that is not in force at that position.
		for removed := 1; removed < len(records)-1; removed++ {
			tampered := append(cloneRecords(records[:removed]), cloneRecords(records[removed+1:])...)
			wantFailure(t, mustVerify(t, tampered, first), removed, FailureBadSignature)
		}
	})
}

func TestVerify_ErasedPartsLeaveARotatedChainVerifying(t *testing.T) {
	records, keys := signedTwiceRotatedChain(t)
	erased := cloneRecords(records)
	for i := range erased {
		erased[i].TenantPart, erased[i].TenantSalt = nil, nil
		erased[i].PersonalPart, erased[i].PersonalSalt = nil, nil
	}
	res := mustVerify(t, erased, keys[0].key(t))
	if !res.OK() || res.Verified != len(records) {
		t.Fatalf("a rotated chain whose erasable parts are gone fails: %+v", res.Failure)
	}
	if res.Key == nil || res.Key.KeyID() != keys[2].key(t).KeyID() {
		t.Fatalf("after erasure the key in force is %+v; want the third key", res.Key)
	}
}
