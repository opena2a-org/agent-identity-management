package record

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// fixtureCheckpointAt is when the vectors' checkpoint of the chain's head is
// taken: thirty seconds after the chain's last record.
var fixtureCheckpointAt = fixtureStart.Add(3*time.Minute + 30*time.Second)

// checkpointOf signs the checkpoint of head taken at the given time with kp.
func checkpointOf(t *testing.T, head Head, at time.Time, kp *memoryKeyProvider) Envelope {
	t.Helper()
	body, err := NewCheckpoint(head, at)
	if err != nil {
		t.Fatalf("NewCheckpoint: %v", err)
	}
	env, err := SignCheckpoint(context.Background(), body, kp)
	if err != nil {
		t.Fatalf("SignCheckpoint: %v", err)
	}
	return env
}

// wantCheckpointFailure asserts that a checkpoint check failed at position
// with kind and that its detail carries the given text.
func wantCheckpointFailure(t *testing.T, f *Failure, position int, kind FailureKind, detail string) {
	t.Helper()
	if f == nil {
		t.Fatalf("the checkpoint was accepted; want %s at position %d", kind, position)
	}
	if f.Position != position || f.Kind != kind {
		t.Fatalf("got %s at position %d (%s); want %s at position %d", f.Kind, f.Position, f.Detail, kind, position)
	}
	if !strings.Contains(f.Detail, detail) {
		t.Fatalf("detail %q does not say %q", f.Detail, detail)
	}
}

// checkpointBytes is the canonical text of a checkpoint of head.
func checkpointBytes(head Head, seq string, timestamp string) string {
	return `{"opena2a":{"chain":{"hash":"` + head.Hash + `","id":"` + head.ChainID + `","seq":` + seq + `},` +
		`"schema":"opena2a-audit-checkpoint/1"},"timestamp":"` + timestamp + `"}`
}

func TestNewCheckpoint_NamesTheHeadAndWhenItWasTaken(t *testing.T) {
	head := buildChain(t, keylessKey())[3].Head()
	cp, err := NewCheckpoint(head, fixtureCheckpointAt)
	if err != nil {
		t.Fatalf("NewCheckpoint: %v", err)
	}
	want := checkpointBytes(head, "3", "2026-01-02T03:07:35.123456Z")
	if got := string(cp.Canonical()); got != want {
		t.Fatalf("canonical bytes\n got %s\nwant %s", got, want)
	}
	if cp.Head() != head {
		t.Fatalf("the body names head %+v; want %+v", cp.Head(), head)
	}

	// The same microsecond in another zone, with more nanoseconds, gives the
	// same bytes: the timestamp is rendered in UTC to the microsecond.
	elsewhere := fixtureCheckpointAt.Add(100 * time.Nanosecond).In(time.FixedZone("", -5*60*60))
	again, err := NewCheckpoint(head, elsewhere)
	if err != nil {
		t.Fatalf("NewCheckpoint: %v", err)
	}
	if !bytes.Equal(again.Canonical(), cp.Canonical()) {
		t.Fatalf("the same instant in another zone gives other bytes")
	}
}

func TestNewCheckpoint_RefusesInvalidInput(t *testing.T) {
	head := buildChain(t, keylessKey())[3].Head()
	cases := map[string]struct {
		head Head
		at   time.Time
	}{
		"a chain id in upper case":           {Head{ChainID: strings.ToUpper(head.ChainID), Seq: head.Seq, Hash: head.Hash}, fixtureCheckpointAt},
		"a negative sequence number":         {Head{ChainID: head.ChainID, Seq: -1, Hash: head.Hash}, fixtureCheckpointAt},
		"a hash that is not lowercase hex":   {Head{ChainID: head.ChainID, Seq: head.Seq, Hash: strings.ToUpper(head.Hash)}, fixtureCheckpointAt},
		"a hash of the wrong length":         {Head{ChainID: head.ChainID, Seq: head.Seq, Hash: head.Hash[:63]}, fixtureCheckpointAt},
		"no time":                            {head, time.Time{}},
		"a time outside RFC 3339":            {head, time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC)},
		"a chain id that is not a UUID form": {Head{ChainID: "not-a-chain", Seq: head.Seq, Hash: head.Hash}, fixtureCheckpointAt},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewCheckpoint(c.head, c.at); !errors.Is(err, ErrInvalidRecord) {
				t.Fatalf("got %v; want ErrInvalidRecord", err)
			}
		})
	}
}

func TestSignCheckpoint_SignsUnderTheCheckpointPayloadType(t *testing.T) {
	records, kp := signedChain(t)
	key := kp.key(t)
	res := mustVerify(t, records, key)
	body, err := NewCheckpoint(*res.Head, fixtureCheckpointAt)
	if err != nil {
		t.Fatalf("NewCheckpoint: %v", err)
	}
	env, err := SignCheckpoint(context.Background(), body, kp)
	if err != nil {
		t.Fatalf("SignCheckpoint: %v", err)
	}
	if env.PayloadType != PayloadTypeCheckpointV1 {
		t.Fatalf("payload type %q; want %q", env.PayloadType, PayloadTypeCheckpointV1)
	}
	if len(env.Signatures) != 1 || env.Signatures[0].KeyID != key.KeyID() {
		t.Fatalf("the envelope does not carry one signature by the provider's key")
	}
	payload, err := Open(env, ClassCheckpointV1, key)
	if err != nil {
		t.Fatalf("the envelope does not open as a checkpoint under the provider's key: %v", err)
	}
	if !bytes.Equal(payload, body.Canonical()) {
		t.Fatalf("the envelope carries other bytes than the checkpoint's canonical bytes")
	}
	input, err := SigningInput(ClassCheckpointV1, Payload{canonical: body.Canonical()})
	if err != nil {
		t.Fatalf("SigningInput: %v", err)
	}
	if !strings.HasPrefix(string(input), "DSSEv1 48 application/vnd.opena2a.audit-checkpoint.v1+json ") {
		t.Fatalf("the signed bytes do not begin with the checkpoint payload type: %q", input[:60])
	}
	if _, err := Open(env, ClassRecordV1, key); err == nil {
		t.Fatalf("a checkpoint envelope opens as a record")
	} else if reason, _ := ReasonOf(err); reason != ReasonPayloadTypeMismatch {
		t.Fatalf("a checkpoint envelope offered as a record: got %v; want %s", err, ReasonPayloadTypeMismatch)
	}
}

func TestSignCheckpoint_Refuses(t *testing.T) {
	kp := newMemoryKeyProvider(t)
	body, err := NewCheckpoint(buildChain(t, kp.key(t))[3].Head(), fixtureCheckpointAt)
	if err != nil {
		t.Fatalf("NewCheckpoint: %v", err)
	}
	if _, err := SignCheckpoint(context.Background(), CheckpointBody{}, kp); !errors.Is(err, ErrSigning) {
		t.Errorf("a body NewCheckpoint did not build: got %v; want ErrSigning", err)
	}
	if _, err := SignCheckpoint(context.Background(), body, nil); !errors.Is(err, ErrSigning) {
		t.Errorf("no key provider: got %v; want ErrSigning", err)
	}
	if _, err := SignCheckpoint(context.Background(), body, brokenProvider{kp}); !errors.Is(err, ErrSigning) {
		t.Errorf("a provider whose signature does not verify: got %v; want ErrSigning", err)
	}
	if _, err := SignCheckpoint(context.Background(), body, misnamingProvider{kp}); !errors.Is(err, ErrSigning) {
		t.Errorf("a provider that names another key identifier: got %v; want ErrSigning", err)
	}
}

func TestOpenCheckpoint_ReadsASignedCheckpoint(t *testing.T) {
	kp := newMemoryKeyProvider(t)
	key := kp.key(t)
	head := buildChain(t, key)[3].Head()
	taken := fixtureCheckpointAt.Add(100 * time.Nanosecond).In(time.FixedZone("", 2*60*60))
	cp, err := OpenCheckpoint(checkpointOf(t, head, taken, kp), key)
	if err != nil {
		t.Fatalf("OpenCheckpoint: %v", err)
	}
	if cp.Head != head {
		t.Fatalf("head %+v; want %+v", cp.Head, head)
	}
	if want := fixtureCheckpointAt.Truncate(time.Microsecond); !cp.Timestamp.Equal(want) {
		t.Fatalf("timestamp %s; want %s", cp.Timestamp, want)
	}
	if cp.Timestamp.Location() != time.UTC {
		t.Fatalf("the timestamp is read in zone %s, not UTC", cp.Timestamp.Location())
	}
	if cp.KeyID != key.KeyID() {
		t.Fatalf("key id %s; want the signing key's %s", cp.KeyID, key.KeyID())
	}
}

func TestOpenCheckpoint_RefusesWhatIsNotACheckpoint(t *testing.T) {
	records, kp := signedChain(t)
	key := kp.key(t)
	head := buildChain(t, key)[3].Head()
	good := checkpointBytes(head, "3", "2026-01-02T03:07:35.123456Z")
	edited := func(old, new string) Envelope {
		t.Helper()
		if strings.Count(good, old) != 1 {
			t.Fatalf("%q occurs %d times in the checkpoint", old, strings.Count(good, old))
		}
		return signRaw(t, kp, ClassCheckpointV1, []byte(strings.Replace(good, old, new, 1))).Envelope
	}
	raw := func(payload string) Envelope {
		return signRaw(t, kp, ClassCheckpointV1, []byte(payload)).Envelope
	}
	other := newMemoryKeyProvider(t)
	unverifiable := raw(good)
	flipSignatureByte(t, &unverifiable.Signatures[0], 0)

	cases := []struct {
		name string
		env  Envelope
		keys SignatureVerifier
		want Reason
	}{
		{"a record envelope", records[1].Envelope, key, ReasonPayloadTypeMismatch},
		{"a record's bytes under the checkpoint class", signRaw(t, kp, ClassCheckpointV1, payloadBytes(t, records[1])).Envelope, key, ReasonMalformedPayload},
		{"bytes that are not JSON", raw("not a checkpoint"), key, ReasonMalformedPayload},
		{"a JSON array", raw("[]"), key, ReasonMalformedPayload},
		{"another format version", edited(`"schema":"opena2a-audit-checkpoint/1"`, `"schema":"opena2a-audit-checkpoint/2"`), key, ReasonSchemaVersion},
		{"the record schema", edited(`"schema":"opena2a-audit-checkpoint/1"`, `"schema":"opena2a-audit-record/1"`), key, ReasonSchemaVersion},
		{"no schema", edited(`,"schema":"opena2a-audit-checkpoint/1"`, ``), key, ReasonMalformedPayload},
		{"no opena2a member", raw(`{"timestamp":"2026-01-02T03:07:35.123456Z"}`), key, ReasonMalformedPayload},
		{"no chain", raw(`{"opena2a":{"schema":"opena2a-audit-checkpoint/1"},"timestamp":"2026-01-02T03:07:35.123456Z"}`), key, ReasonMalformedPayload},
		{"no hash", edited(`"hash":"`+head.Hash+`",`, ``), key, ReasonMalformedPayload},
		{"no timestamp", edited(`,"timestamp":"2026-01-02T03:07:35.123456Z"`, ``), key, ReasonMalformedPayload},
		{"a null timestamp", edited(`"timestamp":"2026-01-02T03:07:35.123456Z"`, `"timestamp":null`), key, ReasonMalformedPayload},
		{"a chain id in upper case", edited(`"id":"`+head.ChainID+`"`, `"id":"`+strings.ToUpper(head.ChainID)+`"`), key, ReasonMalformedPayload},
		{"a negative sequence number", edited(`"seq":3`, `"seq":-1`), key, ReasonMalformedPayload},
		{"a fractional sequence number", edited(`"seq":3`, `"seq":3.0`), key, ReasonMalformedPayload},
		{"a sequence number as text", edited(`"seq":3`, `"seq":"3"`), key, ReasonMalformedPayload},
		{"a hash in upper case", edited(`"hash":"`+head.Hash+`"`, `"hash":"`+strings.ToUpper(head.Hash)+`"`), key, ReasonMalformedPayload},
		{"a timestamp without microseconds", edited(`"2026-01-02T03:07:35.123456Z"`, `"2026-01-02T03:07:35Z"`), key, ReasonMalformedPayload},
		{"a timestamp with a zone offset", edited(`"2026-01-02T03:07:35.123456Z"`, `"2026-01-02T05:07:35.123456+02:00"`), key, ReasonMalformedPayload},
		{"a member this format does not have", edited(`,"timestamp"`, `,"note":"x","timestamp"`), key, ReasonMalformedPayload},
		{"members out of canonical order", raw(`{"timestamp":"2026-01-02T03:07:35.123456Z","opena2a":{"chain":{"hash":"` + head.Hash + `","id":"` + head.ChainID + `","seq":3},"schema":"opena2a-audit-checkpoint/1"}}`), key, ReasonMalformedPayload},
		{"whitespace between members", edited(`},"schema"`, `}, "schema"`), key, ReasonMalformedPayload},
		{"bytes after the checkpoint", raw(good + "{}"), key, ReasonMalformedPayload},
		{"a key the verifier does not hold", raw(good), other.key(t), ReasonUnknownKey},
		{"a signature that does not verify", unverifiable, key, ReasonSignatureInvalid},
		{"no signature", Envelope{Payload: raw(good).Payload, PayloadType: PayloadTypeCheckpointV1}, key, ReasonSignatureCount},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := OpenCheckpoint(c.env, c.keys)
			if err == nil {
				t.Fatalf("accepted; want %s", c.want)
			}
			if reason, _ := ReasonOf(err); reason != c.want {
				t.Fatalf("got %v; want %s", err, c.want)
			}
			if strings.Contains(err.Error(), head.Hash) || strings.Contains(err.Error(), "note") {
				t.Fatalf("the refusal carries a value taken from the envelope: %v", err)
			}
		})
	}

	t.Run("the canonical bytes themselves open", func(t *testing.T) {
		cp, err := OpenCheckpoint(raw(good), key)
		if err != nil {
			t.Fatalf("OpenCheckpoint: %v", err)
		}
		if cp.Head != head {
			t.Fatalf("head %+v; want %+v", cp.Head, head)
		}
	})
}

func TestResult_VerifyCheckpoint_AgreesWithACheckpointOfTheChain(t *testing.T) {
	records, kp := signedChain(t)
	key := kp.key(t)
	res := mustVerify(t, records, key)
	// A checkpoint at every position agrees with the walk, including one taken
	// before the chain grew past it.
	for i, b := range buildChain(t, key) {
		env := checkpointOf(t, b.Head(), fixtureStart.Add(time.Duration(i+1)*time.Hour), kp)
		cp, failure := res.VerifyCheckpoint(env)
		if failure != nil {
			t.Fatalf("checkpoint at sequence %d: %s: %s", i, failure.Kind, failure.Detail)
		}
		if cp.Head != b.Head() || cp.KeyID != key.KeyID() {
			t.Fatalf("checkpoint at sequence %d read back as %+v", i, cp)
		}
	}
}

func TestResult_VerifyCheckpoint_ReportsRecordsRemovedFromTheEnd(t *testing.T) {
	records, kp := signedChain(t)
	key := kp.key(t)
	bodies := buildChain(t, key)
	atHead := checkpointOf(t, bodies[3].Head(), fixtureCheckpointAt, kp)

	t.Run("the last two records removed", func(t *testing.T) {
		res := mustVerify(t, cloneRecords(records[:2]), key)
		if !res.OK() {
			t.Fatalf("the shortened chain does not verify on its own: %v", res.Failure)
		}
		_, failure := res.VerifyCheckpoint(atHead)
		wantCheckpointFailure(t, failure, 3, FailureTruncated, "end at sequence 1")
	})

	t.Run("every record removed", func(t *testing.T) {
		res := mustVerify(t, nil, key)
		_, failure := res.VerifyCheckpoint(atHead)
		wantCheckpointFailure(t, failure, 3, FailureTruncated, "no record verified")
	})

	t.Run("the walk ended at a failure before the checkpoint", func(t *testing.T) {
		tampered := cloneRecords(records)
		flipSignatureByte(t, &tampered[2].Envelope.Signatures[0], 0)
		res := mustVerify(t, tampered, key)
		wantFailure(t, res, 2, FailureBadSignature)
		_, failure := res.VerifyCheckpoint(atHead)
		wantCheckpointFailure(t, failure, 3, FailureTruncated, "failed at position 2")
	})

	t.Run("a checkpoint taken before the removed records agrees", func(t *testing.T) {
		res := mustVerify(t, cloneRecords(records[:2]), key)
		if _, failure := res.VerifyCheckpoint(checkpointOf(t, bodies[1].Head(), fixtureCheckpointAt, kp)); failure != nil {
			t.Fatalf("%s: %s", failure.Kind, failure.Detail)
		}
	})
}

func TestResult_VerifyCheckpoint_ReportsAChainReplacedAfterTheCheckpoint(t *testing.T) {
	kp := newMemoryKeyProvider(t)
	key := kp.key(t)
	bodies := buildChain(t, key)
	original := signAll(t, bodies, kp)
	taken := checkpointOf(t, bodies[3].Head(), fixtureCheckpointAt, kp)

	// The chain's own key signs another record in place of the last one. On
	// its own the replaced chain verifies: every link and signature holds.
	other := fixtureDrafts()[2]
	other.EventID = "a1a1a1a1-0000-4000-8000-00000000000f"
	replacement := mustSign(t, built(t)(NewRecord(bodies[2].Head(), other)), kp)
	replaced := append(cloneRecords(original[:3]), replacement)
	res := mustVerify(t, replaced, key)
	if !res.OK() {
		t.Fatalf("the replaced chain does not verify on its own: %v", res.Failure)
	}
	_, failure := res.VerifyCheckpoint(taken)
	wantCheckpointFailure(t, failure, 3, FailureCheckpointMismatch, "different hash")

	// A checkpoint taken before the replaced record agrees with both chains.
	before := checkpointOf(t, bodies[2].Head(), fixtureStart.Add(3*time.Minute), kp)
	if _, failure := res.VerifyCheckpoint(before); failure != nil {
		t.Fatalf("a checkpoint before the replacement: %s: %s", failure.Kind, failure.Detail)
	}
	if _, failure := mustVerify(t, original, key).VerifyCheckpoint(taken); failure != nil {
		t.Fatalf("the original chain against its own checkpoint: %s: %s", failure.Kind, failure.Detail)
	}
}

func TestResult_VerifyCheckpoint_AcceptsOnlyTheKeyInForceAfterItsSequence(t *testing.T) {
	records, keys := signedTwiceRotatedChain(t)
	bodies := twiceRotatedBodies(t, [3]PublicKey{keys[0].key(t), keys[1].key(t), keys[2].key(t)})
	res := mustVerify(t, records, keys[0].key(t))
	if !res.OK() {
		t.Fatalf("the rotated chain does not verify: %v", res.Failure)
	}
	// After the genesis the first key is in force; the scheduled rotation at
	// sequence 1 moves the chain to the second key; the rotation on exposure
	// at sequence 2 moves it to the third.
	cases := []struct {
		seq    int
		signer int
		ok     bool
	}{
		{0, 0, true}, {0, 1, false}, {0, 2, false},
		{1, 1, true}, {1, 0, false}, {1, 2, false},
		{2, 2, true}, {2, 1, false}, {2, 0, false},
		{3, 2, true}, {3, 0, false}, {3, 1, false},
	}
	for _, c := range cases {
		env := checkpointOf(t, bodies[c.seq].Head(), fixtureStart.Add(time.Hour), keys[c.signer])
		cp, failure := res.VerifyCheckpoint(env)
		switch {
		case c.ok && failure != nil:
			t.Errorf("sequence %d signed by key %d: %s: %s", c.seq, c.signer, failure.Kind, failure.Detail)
		case c.ok && cp.KeyID != keys[c.signer].key(t).KeyID():
			t.Errorf("sequence %d: the checkpoint names key %s", c.seq, cp.KeyID)
		case !c.ok:
			if failure == nil {
				t.Errorf("sequence %d signed by key %d was accepted", c.seq, c.signer)
			} else if failure.Kind != FailureCheckpoint || failure.Position != c.seq || !strings.Contains(failure.Detail, "not in force") {
				t.Errorf("sequence %d signed by key %d: got %s at %d (%s)", c.seq, c.signer, failure.Kind, failure.Position, failure.Detail)
			}
		}
	}
}

func TestResult_VerifyCheckpoint_ReportsACheckpointSignedByAKeyTheChainNeverNamed(t *testing.T) {
	records, keys := signedTwiceRotatedChain(t)
	bodies := twiceRotatedBodies(t, [3]PublicKey{keys[0].key(t), keys[1].key(t), keys[2].key(t)})
	atHead := checkpointOf(t, bodies[3].Head(), fixtureCheckpointAt, keys[2])

	// Truncated before the rotation that named the third key, the chain as
	// held never named the key that signed the checkpoint.
	res := mustVerify(t, cloneRecords(records[:2]), keys[0].key(t))
	_, failure := res.VerifyCheckpoint(atHead)
	wantCheckpointFailure(t, failure, 2, FailureCheckpoint, "no verified record of the chain named")

	// Truncated after that rotation, the key is known and the truncation is
	// what is reported.
	res = mustVerify(t, cloneRecords(records[:3]), keys[0].key(t))
	_, failure = res.VerifyCheckpoint(atHead)
	wantCheckpointFailure(t, failure, 3, FailureTruncated, "end at sequence 2")
}

func TestResult_VerifyCheckpoint_RefusesACheckpointOfAnotherChain(t *testing.T) {
	records, kp := signedChain(t)
	key := kp.key(t)
	res := mustVerify(t, records, key)
	other := Head{ChainID: "1c1c1c1c-0000-4000-8000-000000000001", Seq: res.Head.Seq, Hash: res.Head.Hash}
	_, failure := res.VerifyCheckpoint(checkpointOf(t, other, fixtureCheckpointAt, kp))
	wantCheckpointFailure(t, failure, 3, FailureCheckpoint, "names chain 1c1c1c1c-0000-4000-8000-000000000001, not "+fixtureChainID)

	// What does not open as a checkpoint is reported at the end of the
	// verified records, with the refusal's reason.
	_, failure = res.VerifyCheckpoint(records[1].Envelope)
	wantCheckpointFailure(t, failure, 4, FailureCheckpoint, string(ReasonPayloadTypeMismatch))
}

func TestResult_ChainKeysHoldsEveryKeyTheWalkSaw(t *testing.T) {
	records, keys := signedTwiceRotatedChain(t)
	head := twiceRotatedBodies(t, [3]PublicKey{keys[0].key(t), keys[1].key(t), keys[2].key(t)})[3].Head()
	res := mustVerify(t, records, keys[0].key(t))
	for i, kp := range keys {
		if _, err := OpenCheckpoint(checkpointOf(t, head, fixtureCheckpointAt, kp), res.ChainKeys()); err != nil {
			t.Errorf("key %d, which the chain named: %v", i, err)
		}
	}
	stranger := newMemoryKeyProvider(t)
	_, err := OpenCheckpoint(checkpointOf(t, head, fixtureCheckpointAt, stranger), res.ChainKeys())
	if reason, _ := ReasonOf(err); reason != ReasonUnknownKey {
		t.Errorf("a key the chain never named: got %v; want %s", err, ReasonUnknownKey)
	}

	// With no record verified, the walk holds the verifier's key alone.
	empty := mustVerify(t, nil, keys[0].key(t))
	if _, err := OpenCheckpoint(checkpointOf(t, head, fixtureCheckpointAt, keys[0]), empty.ChainKeys()); err != nil {
		t.Errorf("the verifier's key on an empty walk: %v", err)
	}
	_, err = OpenCheckpoint(checkpointOf(t, head, fixtureCheckpointAt, keys[1]), empty.ChainKeys())
	if reason, _ := ReasonOf(err); reason != ReasonUnknownKey {
		t.Errorf("a later key on an empty walk: got %v; want %s", err, ReasonUnknownKey)
	}
}
