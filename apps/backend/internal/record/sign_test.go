package record

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
)

func TestSign_SignsTheCanonicalBytesUnderTheRecordPayloadType(t *testing.T) {
	kp := newMemoryKeyProvider(t)
	key := kp.key(t)
	bodies := buildChain(t, key)
	for i, b := range bodies {
		rec, err := Sign(context.Background(), b, kp)
		if err != nil {
			t.Fatalf("record %d: Sign: %v", i, err)
		}
		if rec.Envelope.PayloadType != "application/vnd.opena2a.audit-record.v1+json" {
			t.Fatalf("record %d: payload type %q", i, rec.Envelope.PayloadType)
		}
		if !bytes.Equal(payloadBytes(t, rec), b.Canonical()) {
			t.Fatalf("record %d: the payload is not the canonical bytes", i)
		}
		sum := sha256.Sum256(b.Canonical())
		if rec.RecordHash != hex.EncodeToString(sum[:]) {
			t.Fatalf("record %d: the record hash is not the SHA-256 of the canonical bytes", i)
		}
		if len(rec.Envelope.Signatures) != 1 {
			t.Fatalf("record %d: %d signatures", i, len(rec.Envelope.Signatures))
		}
		s := rec.Envelope.Signatures[0]
		keyDigest := sha256.Sum256(kp.public)
		if s.KeyID != hex.EncodeToString(keyDigest[:]) {
			t.Fatalf("record %d: signature names key %q", i, s.KeyID)
		}
		sig, err := base64.StdEncoding.DecodeString(s.Sig)
		if err != nil {
			t.Fatalf("record %d: signature: %v", i, err)
		}
		// The signed bytes, written out independently of the package.
		signed := "DSSEv1 44 application/vnd.opena2a.audit-record.v1+json " +
			strconv.Itoa(len(b.Canonical())) + " " + string(b.Canonical())
		if !ed25519.Verify(kp.public, []byte(signed), sig) {
			t.Fatalf("record %d: the signature does not verify over the pre-authentication encoding", i)
		}
		if ed25519.Verify(kp.public, b.Canonical(), sig) {
			t.Fatalf("record %d: the signature verifies over the bare payload", i)
		}
	}
}

func TestRecord_SerializesAsADSSEEnvelopeWithItsParts(t *testing.T) {
	records, _ := signedChain(t)
	out, err := json.Marshal(records[1])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var doc struct {
		Envelope struct {
			PayloadType string `json:"payloadType"`
			Payload     string `json:"payload"`
			Signatures  []struct {
				KeyID string `json:"keyid"`
				Sig   string `json:"sig"`
			} `json:"signatures"`
		} `json:"envelope"`
		RecordHash   string `json:"recordHash"`
		TenantPart   string `json:"tenantPart"`
		TenantSalt   string `json:"tenantSalt"`
		PersonalPart string `json:"personalPart"`
		PersonalSalt string `json:"personalSalt"`
	}
	decoder := json.NewDecoder(bytes.NewReader(out))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		t.Fatalf("the record has a member this test does not name: %v\n%s", err, out)
	}
	payload, err := base64.StdEncoding.DecodeString(doc.Envelope.Payload)
	if err != nil || !bytes.Equal(payload, payloadBytes(t, records[1])) {
		t.Fatalf("payload is not standard base64 of the canonical bytes: %v", err)
	}
	if len(doc.Envelope.Signatures) != 1 || doc.Envelope.Signatures[0].Sig == "" || doc.Envelope.Signatures[0].KeyID == "" {
		t.Fatalf("signatures are not an array of keyid and sig:\n%s", out)
	}
	for name, value := range map[string]string{
		"tenantPart": doc.TenantPart, "tenantSalt": doc.TenantSalt,
		"personalPart": doc.PersonalPart, "personalSalt": doc.PersonalSalt,
	} {
		if value == "" {
			t.Fatalf("%s is missing:\n%s", name, out)
		}
	}

	var back Record
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Envelope.Payload != records[1].Envelope.Payload || !bytes.Equal(back.TenantPart, records[1].TenantPart) {
		t.Fatalf("a record does not survive a JSON round trip byte for byte")
	}
}

// brokenProvider returns a signature that is not its key's.
type brokenProvider struct{ *memoryKeyProvider }

func (b brokenProvider) SignPayload(ctx context.Context, class PayloadClass, payload Payload) (string, []byte, error) {
	keyID, sig, err := b.memoryKeyProvider.SignPayload(ctx, class, payload)
	if err != nil {
		return "", nil, err
	}
	sig[0] ^= 0x01
	return keyID, sig, nil
}

// misnamingProvider signs with its key and names another key identifier.
type misnamingProvider struct{ *memoryKeyProvider }

func (m misnamingProvider) SignPayload(ctx context.Context, class PayloadClass, payload Payload) (string, []byte, error) {
	_, sig, err := m.memoryKeyProvider.SignPayload(ctx, class, payload)
	return keylessKey().KeyID(), sig, err
}

func TestSign_Refuses(t *testing.T) {
	kp := newMemoryKeyProvider(t)
	bodies := buildChain(t, kp.key(t))

	if _, err := Sign(context.Background(), Body{}, kp); !errors.Is(err, ErrSigning) {
		t.Errorf("a body no constructor built: got %v; want ErrSigning", err)
	}
	if _, err := Sign(context.Background(), bodies[1], nil); !errors.Is(err, ErrSigning) {
		t.Errorf("no key provider: got %v; want ErrSigning", err)
	}
	other := newMemoryKeyProvider(t)
	if _, err := Sign(context.Background(), bodies[0], other); !errors.Is(err, ErrSigning) {
		t.Errorf("a genesis that names another key: got %v; want ErrSigning", err)
	}
	if _, err := Sign(context.Background(), bodies[1], brokenProvider{kp}); !errors.Is(err, ErrSigning) {
		t.Errorf("a provider whose signature does not verify: got %v; want ErrSigning", err)
	}
	if _, err := Sign(context.Background(), bodies[1], misnamingProvider{kp}); !errors.Is(err, ErrSigning) {
		t.Errorf("a provider that names another key identifier: got %v; want ErrSigning", err)
	}
}

func TestPublicKey_IDIsTheHashOfTheKey(t *testing.T) {
	key := keylessKey()
	sum := sha256.Sum256(key.Key)
	if got, want := key.KeyID(), hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("got %s; want %s", got, want)
	}
}
