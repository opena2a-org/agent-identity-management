package record

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "rewrite the golden vectors under testdata")

const (
	vectorPath = "envelope_vectors.json"

	// The payload of every vector. The vectors pin the envelope, not the
	// contents of a record or a checkpoint, so the payload is the empty object.
	vectorPayload = "{}"

	// A record payload type with no format version. It is not a member of the
	// closed set and no verifier accepts it.
	unversionedRecordType = "application/vnd.opena2a.audit-record+json"
)

// testKeys is a KeyProvider and a SignatureVerifier over one Ed25519 key. The
// key is derived from a public label so the golden vectors can be regenerated
// byte for byte; it protects nothing.
type testKeys struct {
	id      string
	private ed25519.PrivateKey
	signed  int
	checked int
}

func newTestKeys() *testKeys {
	seed := sha256.Sum256([]byte("aim record envelope test vectors"))
	return &testKeys{id: "test-key-1", private: ed25519.NewKeyFromSeed(seed[:])}
}

func (k *testKeys) public() ed25519.PublicKey {
	return k.private.Public().(ed25519.PublicKey)
}

func (k *testKeys) SignPayload(_ context.Context, class PayloadClass, payload Payload) (string, []byte, error) {
	k.signed++
	message, err := SigningInput(class, payload)
	if err != nil {
		return "", nil, err
	}
	return k.id, ed25519.Sign(k.private, message), nil
}

func (k *testKeys) VerifySignature(keyID string, message, signature []byte) error {
	k.checked++
	return publicKey{id: k.id, key: k.public()}.VerifySignature(keyID, message, signature)
}

// publicKey verifies with a public key alone, as a holder of an export does.
type publicKey struct {
	id  string
	key ed25519.PublicKey
}

func (p publicKey) VerifySignature(keyID string, message, signature []byte) error {
	if keyID != p.id {
		return ErrUnknownKey
	}
	if !ed25519.Verify(p.key, message, signature) {
		return errors.New("ed25519 signature does not verify")
	}
	return nil
}

func payloadOf(s string) Payload {
	return Payload{canonical: []byte(s)}
}

// signedAs returns an envelope of any payload type, signed by k over that
// type's own pre-authentication encoding. A refusal of such an envelope can
// only come from the payload type, never from its signature.
func signedAs(k *testKeys, payloadType, body string) Envelope {
	sig := ed25519.Sign(k.private, pae(payloadType, []byte(body)))
	return Envelope{
		Payload:     base64.StdEncoding.EncodeToString([]byte(body)),
		PayloadType: payloadType,
		Signatures:  []Signature{{KeyID: k.id, Sig: base64.StdEncoding.EncodeToString(sig)}},
	}
}

func requireReason(t *testing.T, err error, want Reason) {
	t.Helper()
	require.Error(t, err)
	got, ok := ReasonOf(err)
	require.True(t, ok, "error is not a refusal: %v", err)
	assert.Equal(t, want, got, "refusal: %v", err)
}

func TestSigningInputBeginsWithThePayloadType(t *testing.T) {
	record, err := SigningInput(ClassRecordV1, payloadOf("{}"))
	require.NoError(t, err)
	assert.Equal(t, "DSSEv1 44 application/vnd.opena2a.audit-record.v1+json 2 {}", string(record))

	checkpoint, err := SigningInput(ClassCheckpointV1, payloadOf("{}"))
	require.NoError(t, err)
	assert.Equal(t, "DSSEv1 48 application/vnd.opena2a.audit-checkpoint.v1+json 2 {}", string(checkpoint))

	// Lengths are byte lengths: the two-byte letter makes this payload 10 bytes.
	wide, err := SigningInput(ClassRecordV1, payloadOf(`{"a":"é"}`))
	require.NoError(t, err)
	assert.Equal(t, "DSSEv1 44 application/vnd.opena2a.audit-record.v1+json 10 {\"a\":\"é\"}", string(wide))
}

func TestPayloadTypeOfEachClass(t *testing.T) {
	record, ok := ClassRecordV1.PayloadType()
	require.True(t, ok)
	assert.Equal(t, "application/vnd.opena2a.audit-record.v1+json", record)
	assert.Len(t, record, 44)

	checkpoint, ok := ClassCheckpointV1.PayloadType()
	require.True(t, ok)
	assert.Equal(t, "application/vnd.opena2a.audit-checkpoint.v1+json", checkpoint)
	assert.Len(t, checkpoint, 48)

	for _, class := range []PayloadClass{0, 3, 255} {
		got, ok := class.PayloadType()
		assert.False(t, ok, "class %d", class)
		assert.Empty(t, got)
	}
}

// The example of the DSSE specification's protocol document.
func TestPAEMatchesTheDSSEExample(t *testing.T) {
	got := pae("http://example.com/HelloWorld", []byte("hello world"))
	assert.Equal(t, "DSSEv1 29 http://example.com/HelloWorld 11 hello world", string(got))
}

func TestNothingIsSignedForAClassOutsideTheClosedSet(t *testing.T) {
	for _, class := range []PayloadClass{0, 3, 255} {
		keys := newTestKeys()

		_, err := Seal(context.Background(), keys, class, payloadOf("{}"))
		requireReason(t, err, ReasonUnknownClass)
		assert.Zero(t, keys.signed, "class %d reached the key provider", class)

		message, err := SigningInput(class, payloadOf("{}"))
		requireReason(t, err, ReasonUnknownClass)
		assert.Nil(t, message)
	}
}

func TestNothingIsSignedForAnEmptyPayload(t *testing.T) {
	keys := newTestKeys()

	_, err := Seal(context.Background(), keys, ClassRecordV1, Payload{})
	requireReason(t, err, ReasonEmptyPayload)
	assert.Zero(t, keys.signed)

	_, err = SigningInput(ClassRecordV1, Payload{})
	requireReason(t, err, ReasonEmptyPayload)
}

func TestSealThenOpen(t *testing.T) {
	for _, class := range []PayloadClass{ClassRecordV1, ClassCheckpointV1} {
		keys := newTestKeys()
		wantType, _ := class.PayloadType()

		env, err := Seal(context.Background(), keys, class, payloadOf(`{"n":1}`))
		require.NoError(t, err)
		assert.Equal(t, wantType, env.PayloadType)
		assert.Equal(t, base64.StdEncoding.EncodeToString([]byte(`{"n":1}`)), env.Payload)
		require.Len(t, env.Signatures, 1)
		assert.Equal(t, "test-key-1", env.Signatures[0].KeyID)

		body, err := Open(env, class, keys)
		require.NoError(t, err)
		assert.Equal(t, `{"n":1}`, string(body))
		assert.Equal(t, 1, keys.checked)
	}
}

func TestSealReportsAKeyProviderFailure(t *testing.T) {
	unavailable := errors.New("key unavailable")

	_, err := Seal(context.Background(), stubProvider{err: unavailable}, ClassRecordV1, payloadOf("{}"))
	require.ErrorIs(t, err, unavailable)

	_, err = Seal(context.Background(), stubProvider{sig: []byte{1}}, ClassRecordV1, payloadOf("{}"))
	require.Error(t, err, "a signature with no key id")

	_, err = Seal(context.Background(), stubProvider{keyID: "k"}, ClassRecordV1, payloadOf("{}"))
	require.Error(t, err, "a key id with no signature")
}

type stubProvider struct {
	keyID string
	sig   []byte
	err   error
}

func (s stubProvider) SignPayload(context.Context, PayloadClass, Payload) (string, []byte, error) {
	return s.keyID, s.sig, s.err
}

func TestOpenComparesThePayloadTypeByteForByte(t *testing.T) {
	offered := []string{
		PayloadTypeCheckpointV1,
		unversionedRecordType,
		"application/vnd.opena2a.audit-record.v2+json",
		"APPLICATION/VND.OPENA2A.AUDIT-RECORD.V1+JSON",
		"application/vnd.opena2a.audit-record.v1+json ",
		" application/vnd.opena2a.audit-record.v1+json",
		"application/vnd.opena2a.audit-record.v1+json; charset=utf-8",
		"application/vnd.opena2a.audit-record.v1+json\x00",
		"",
	}
	for _, payloadType := range offered {
		keys := newTestKeys()

		body, err := Open(signedAs(keys, payloadType, "{}"), ClassRecordV1, keys)
		requireReason(t, err, ReasonPayloadTypeMismatch)
		assert.Nil(t, body, "payload type %q", payloadType)
		assert.Zero(t, keys.checked, "payload type %q reached signature verification", payloadType)
	}
}

func TestOpenChecksThePayloadTypeBeforeItReadsAnythingElse(t *testing.T) {
	keys := newTestKeys()
	env := Envelope{PayloadType: PayloadTypeCheckpointV1, Payload: "not base64"}

	_, err := Open(env, ClassRecordV1, keys)
	requireReason(t, err, ReasonPayloadTypeMismatch)
	assert.Zero(t, keys.checked)
}

func TestOpenRefusesAnExpectedClassOutsideTheClosedSet(t *testing.T) {
	keys := newTestKeys()
	for _, class := range []PayloadClass{0, 3, 255} {
		_, err := Open(signedAs(keys, PayloadTypeRecordV1, "{}"), class, keys)
		requireReason(t, err, ReasonUnknownClass)
	}
	assert.Zero(t, keys.checked)
}

func TestOpenBindsTheSignatureToThePayloadType(t *testing.T) {
	keys := newTestKeys()
	env, err := Seal(context.Background(), keys, ClassRecordV1, payloadOf("{}"))
	require.NoError(t, err)

	// The same payload and signature under the other class's payload type.
	env.PayloadType = PayloadTypeCheckpointV1
	_, err = Open(env, ClassCheckpointV1, keys)
	requireReason(t, err, ReasonSignatureInvalid)
}

func TestOpenRequiresExactlyOneSignature(t *testing.T) {
	keys := newTestKeys()
	env := signedAs(keys, PayloadTypeRecordV1, "{}")

	none := env
	none.Signatures = nil
	_, err := Open(none, ClassRecordV1, keys)
	requireReason(t, err, ReasonSignatureCount)

	two := env
	two.Signatures = []Signature{env.Signatures[0], env.Signatures[0]}
	_, err = Open(two, ClassRecordV1, keys)
	requireReason(t, err, ReasonSignatureCount)

	assert.Zero(t, keys.checked)
}

func TestOpenAcceptsOnlyCanonicalStandardBase64(t *testing.T) {
	keys := newTestKeys()
	// Bytes whose standard encoding is "+/8=" and whose URL-safe one is "-_8=".
	env := signedAs(keys, PayloadTypeRecordV1, "\xfb\xff")
	require.Equal(t, "+/8=", env.Payload)
	_, err := Open(env, ClassRecordV1, keys)
	require.NoError(t, err)

	for _, payload := range []string{"-_8=", "+/8", "+/8=\n", "+/9=", "not base64", ""} {
		bad := env
		bad.Payload = payload
		_, err := Open(bad, ClassRecordV1, keys)
		if payload == "" {
			requireReason(t, err, ReasonEmptyPayload)
			continue
		}
		requireReason(t, err, ReasonPayloadEncoding)
	}

	sig := env.Signatures[0].Sig
	for _, encoded := range []string{sig + "\n", sig[:len(sig)-2], "not base64", ""} {
		bad := env
		bad.Signatures = []Signature{{KeyID: keys.id, Sig: encoded}}
		_, err := Open(bad, ClassRecordV1, keys)
		requireReason(t, err, ReasonSignatureEncoding)
	}

	assert.Equal(t, 1, keys.checked, "only the well-formed envelope reaches signature verification")
}

func TestOpenRefusesAChangedPayload(t *testing.T) {
	keys := newTestKeys()
	env := signedAs(keys, PayloadTypeRecordV1, `{"n":1}`)
	env.Payload = base64.StdEncoding.EncodeToString([]byte(`{"n":2}`))

	_, err := Open(env, ClassRecordV1, keys)
	requireReason(t, err, ReasonSignatureInvalid)
}

func TestOpenRefusesAKeyTheVerifierDoesNotHold(t *testing.T) {
	keys := newTestKeys()
	env := signedAs(keys, PayloadTypeRecordV1, "{}")
	env.Signatures[0].KeyID = "another-key"

	_, err := Open(env, ClassRecordV1, keys)
	requireReason(t, err, ReasonUnknownKey)
	require.ErrorIs(t, err, ErrUnknownKey)
}

func TestPayloadBytesIsACopy(t *testing.T) {
	payload := payloadOf("{}")
	got := payload.Bytes()
	got[0] = 'x'
	assert.Equal(t, "{}", string(payload.Bytes()))
}

// ---- golden vectors ----

type vectorFile struct {
	Comment   string   `json:"comment"`
	KeyID     string   `json:"keyId"`
	PublicKey string   `json:"publicKey"`
	Vectors   []vector `json:"vectors"`
}

type vector struct {
	Name       string   `json:"name"`
	OpenAs     string   `json:"openAs"`
	WantReason Reason   `json:"wantReason"`
	Envelope   Envelope `json:"envelope"`
}

var classByName = map[string]PayloadClass{
	"record":     ClassRecordV1,
	"checkpoint": ClassCheckpointV1,
}

func buildVectors(t *testing.T) vectorFile {
	t.Helper()
	keys := newTestKeys()

	record, err := Seal(context.Background(), keys, ClassRecordV1, payloadOf(vectorPayload))
	require.NoError(t, err)
	checkpoint, err := Seal(context.Background(), keys, ClassCheckpointV1, payloadOf(vectorPayload))
	require.NoError(t, err)

	recordRelabelled := record
	recordRelabelled.PayloadType = PayloadTypeCheckpointV1
	checkpointRelabelled := checkpoint
	checkpointRelabelled.PayloadType = PayloadTypeRecordV1

	unversioned := signedAs(keys, unversionedRecordType, vectorPayload)

	return vectorFile{
		Comment: "Envelope vectors. Every payload is the empty object: these pin the " +
			"envelope and its signed bytes, not the contents of a record or a checkpoint. " +
			"The key is a test key derived from a public label. An empty wantReason means the " +
			"envelope is accepted.",
		KeyID:     keys.id,
		PublicKey: base64.StdEncoding.EncodeToString(keys.public()),
		Vectors: []vector{
			{Name: "record envelope opened as a record", OpenAs: "record", Envelope: record},
			{Name: "checkpoint envelope opened as a checkpoint", OpenAs: "checkpoint", Envelope: checkpoint},
			{Name: "valid checkpoint envelope offered as a record", OpenAs: "record", WantReason: ReasonPayloadTypeMismatch, Envelope: checkpoint},
			{Name: "valid record envelope offered as a checkpoint", OpenAs: "checkpoint", WantReason: ReasonPayloadTypeMismatch, Envelope: record},
			{Name: "record payload type with no format version offered as a record", OpenAs: "record", WantReason: ReasonPayloadTypeMismatch, Envelope: unversioned},
			{Name: "record payload type with no format version offered as a checkpoint", OpenAs: "checkpoint", WantReason: ReasonPayloadTypeMismatch, Envelope: unversioned},
			{Name: "record signature under the checkpoint payload type", OpenAs: "checkpoint", WantReason: ReasonSignatureInvalid, Envelope: recordRelabelled},
			{Name: "checkpoint signature under the record payload type", OpenAs: "record", WantReason: ReasonSignatureInvalid, Envelope: checkpointRelabelled},
		},
	}
}

func encodeVectors(t *testing.T, file vectorFile) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	require.NoError(t, enc.Encode(file))
	return buf.Bytes()
}

// The committed vectors are exactly what this package produces today. A change
// to the signed bytes or to the envelope's form turns this red.
func TestGoldenVectorsAreCurrent(t *testing.T) {
	want := encodeVectors(t, buildVectors(t))
	path := filepath.Join("testdata", vectorPath)

	if *update {
		require.NoError(t, os.MkdirAll("testdata", 0o750))
		require.NoError(t, os.WriteFile(path, want, 0o600))
	}

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got), "run the test with -update only for a deliberate change of format")
}

// The committed vectors verify with the public key they carry and nothing else.
func TestGoldenVectors(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", vectorPath))
	require.NoError(t, err)

	var file vectorFile
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	require.NoError(t, dec.Decode(&file))

	key, err := base64.StdEncoding.DecodeString(file.PublicKey)
	require.NoError(t, err)
	require.Len(t, key, ed25519.PublicKeySize)
	verifier := publicKey{id: file.KeyID, key: ed25519.PublicKey(key)}

	seen := map[string]bool{}
	for _, v := range file.Vectors {
		t.Run(v.Name, func(t *testing.T) {
			class, ok := classByName[v.OpenAs]
			require.True(t, ok, "openAs %q", v.OpenAs)

			body, err := Open(v.Envelope, class, verifier)
			if v.WantReason == "" {
				require.NoError(t, err)
				assert.Equal(t, vectorPayload, string(body))
				return
			}
			requireReason(t, err, v.WantReason)
			assert.Nil(t, body)
		})
		seen[v.Name] = true
	}

	for _, name := range []string{
		"valid checkpoint envelope offered as a record",
		"valid record envelope offered as a checkpoint",
		"record payload type with no format version offered as a record",
	} {
		assert.True(t, seen[name], "missing vector %q", name)
	}
}
