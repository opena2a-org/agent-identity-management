package record

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
)

// PayloadClass names one kind of payload the record key signs. The set is
// closed: every operation of this package refuses a value that is not one of
// the constants below.
type PayloadClass uint8

const (
	// ClassRecordV1 is an audit record in format version 1.
	ClassRecordV1 PayloadClass = iota + 1
	// ClassCheckpointV1 is a chain checkpoint in format version 1.
	ClassCheckpointV1
)

// The payload type of each class. A verifier compares an envelope's
// payloadType with exactly one of these, byte for byte.
const (
	PayloadTypeRecordV1     = "application/vnd.opena2a.audit-record.v1+json"
	PayloadTypeCheckpointV1 = "application/vnd.opena2a.audit-checkpoint.v1+json"
)

// PayloadType returns the payload type of c. The second result is false when
// c is not a member of the closed set.
func (c PayloadClass) PayloadType() (string, bool) {
	switch c {
	case ClassRecordV1:
		return PayloadTypeRecordV1, true
	case ClassCheckpointV1:
		return PayloadTypeCheckpointV1, true
	default:
		return "", false
	}
}

// Payload is the canonical bytes of one record or checkpoint. Its only
// constructor is this package's serializer, so a caller cannot hand the record
// key bytes of its own choosing. The zero Payload is empty and is never signed.
type Payload struct {
	canonical []byte
}

// Bytes returns a copy of the canonical bytes.
func (p Payload) Bytes() []byte {
	return append([]byte(nil), p.canonical...)
}

// KeyProvider holds the record key. Its one operation signs a payload of a
// class from the closed set. There is deliberately no operation that takes a
// payload type or bytes of the caller's choosing.
//
// An implementation takes the bytes to sign from SigningInput and signs
// nothing else with the record key. It returns the identifier under which the
// verifying key is published, and the raw signature.
type KeyProvider interface {
	SignPayload(ctx context.Context, class PayloadClass, payload Payload) (keyID string, signature []byte, err error)
}

// SignatureVerifier checks a signature of the record key that keyID names over
// message. It returns ErrUnknownKey when it holds no key by that identifier.
type SignatureVerifier interface {
	VerifySignature(keyID string, message, signature []byte) error
}

// ErrUnknownKey is returned by a SignatureVerifier that holds no key by the
// identifier it was asked for.
var ErrUnknownKey = errors.New("record: no key with that identifier")

// Envelope is a DSSE v1 envelope in its JSON form. Payload and Sig are standard
// base64 with padding. The member names are the DSSE specification's.
type Envelope struct {
	Payload     string      `json:"payload"`
	PayloadType string      `json:"payloadType"`
	Signatures  []Signature `json:"signatures"`
}

// Signature is one signature of an Envelope.
type Signature struct {
	KeyID string `json:"keyid"`
	Sig   string `json:"sig"`
}

// Reason names why a signing request or an envelope was refused. The set is
// closed.
type Reason string

const (
	// ReasonUnknownClass: the payload class is not a member of the closed set.
	ReasonUnknownClass Reason = "unknown_class"
	// ReasonEmptyPayload: there are no payload bytes.
	ReasonEmptyPayload Reason = "empty_payload"
	// ReasonPayloadTypeMismatch: the envelope's payloadType is not, byte for
	// byte, the payload type of the class the caller expects.
	ReasonPayloadTypeMismatch Reason = "payload_type_mismatch"
	// ReasonSignatureCount: the envelope does not carry exactly one signature.
	ReasonSignatureCount Reason = "signature_count"
	// ReasonPayloadEncoding: the payload is not canonical standard base64.
	ReasonPayloadEncoding Reason = "payload_encoding"
	// ReasonSignatureEncoding: the signature is empty or is not canonical
	// standard base64.
	ReasonSignatureEncoding Reason = "signature_encoding"
	// ReasonUnknownKey: the verifier holds no key by the signature's keyid.
	ReasonUnknownKey Reason = "unknown_key"
	// ReasonSignatureInvalid: the signature does not verify over the signing
	// input of the expected class.
	ReasonSignatureInvalid Reason = "signature_invalid"
)

// Refusal is the error of every refusal in this package. Its text never
// carries a value taken from the refused envelope.
type Refusal struct {
	Reason Reason
	detail string
	cause  error
}

func (r *Refusal) Error() string {
	return "record: " + string(r.Reason) + ": " + r.detail
}

func (r *Refusal) Unwrap() error {
	return r.cause
}

// ReasonOf returns the reason of the refusal that err is or wraps.
func ReasonOf(err error) (Reason, bool) {
	var refusal *Refusal
	if errors.As(err, &refusal) {
		return refusal.Reason, true
	}
	return "", false
}

func refuse(reason Reason, detail string) *Refusal {
	return &Refusal{Reason: reason, detail: detail}
}

// pae is the DSSE v1 pre-authentication encoding:
//
//	"DSSEv1" SP LEN(type) SP type SP LEN(body) SP body
//
// LEN is a byte length in ASCII decimal with no leading zeros and SP is one
// space. The encoding begins with the fixed bytes "DSSEv1", a space, the
// payload type's byte length, a space and the payload type. So the bytes
// signed for a record begin
// "DSSEv1 44 application/vnd.opena2a.audit-record.v1+json " and those signed
// for a checkpoint begin
// "DSSEv1 48 application/vnd.opena2a.audit-checkpoint.v1+json ".
func pae(payloadType string, body []byte) []byte {
	out := make([]byte, 0, len("DSSEv1")+len(payloadType)+len(body)+44)
	out = append(out, "DSSEv1 "...)
	out = strconv.AppendInt(out, int64(len(payloadType)), 10)
	out = append(out, ' ')
	out = append(out, payloadType...)
	out = append(out, ' ')
	out = strconv.AppendInt(out, int64(len(body)), 10)
	out = append(out, ' ')
	out = append(out, body...)
	return out
}

// signable returns the payload type under which payload is signed as class.
func signable(class PayloadClass, payload Payload) (string, error) {
	payloadType, ok := class.PayloadType()
	if !ok {
		return "", refuse(ReasonUnknownClass, fmt.Sprintf("payload class %d is not one the record key signs", uint8(class)))
	}
	if len(payload.canonical) == 0 {
		return "", refuse(ReasonEmptyPayload, "there is no payload to sign")
	}
	return payloadType, nil
}

// SigningInput returns the bytes the record key signs for payload as class.
// It is the only source of those bytes.
func SigningInput(class PayloadClass, payload Payload) ([]byte, error) {
	payloadType, err := signable(class, payload)
	if err != nil {
		return nil, err
	}
	return pae(payloadType, payload.canonical), nil
}

// Seal signs payload as class through keys and returns its envelope. A class
// outside the closed set and an empty payload are refused before keys is
// called.
func Seal(ctx context.Context, keys KeyProvider, class PayloadClass, payload Payload) (Envelope, error) {
	payloadType, err := signable(class, payload)
	if err != nil {
		return Envelope{}, err
	}
	keyID, signature, err := keys.SignPayload(ctx, class, payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("record: key provider: %w", err)
	}
	if keyID == "" || len(signature) == 0 {
		return Envelope{}, errors.New("record: key provider returned no key identifier or no signature")
	}
	return Envelope{
		Payload:     base64.StdEncoding.EncodeToString(payload.canonical),
		PayloadType: payloadType,
		Signatures:  []Signature{{KeyID: keyID, Sig: base64.StdEncoding.EncodeToString(signature)}},
	}, nil
}

// Open checks env as a payload of class want and returns its payload bytes.
//
// The checks run in this order and the first failure ends them:
//
//  1. env.PayloadType equals, byte for byte, the one payload type of want. A
//     verifier that accepts both classes still accepts only one of them at any
//     position, so a checkpoint offered where a record is expected is refused
//     here, as is a payload type with another or no format version.
//  2. The envelope carries exactly one signature.
//  3. The payload and the signature are canonical standard base64.
//  4. The signature verifies over the signing input rebuilt from want's
//     payload type and the payload.
//
// The bytes returned are authenticated and not parsed: nothing in the payload
// is read by this function.
func Open(env Envelope, want PayloadClass, keys SignatureVerifier) ([]byte, error) {
	wantType, ok := want.PayloadType()
	if !ok {
		return nil, refuse(ReasonUnknownClass, fmt.Sprintf("payload class %d is not one a verifier expects", uint8(want)))
	}
	if env.PayloadType != wantType {
		return nil, refuse(ReasonPayloadTypeMismatch, "payload type is not "+wantType)
	}
	if len(env.Signatures) != 1 {
		return nil, refuse(ReasonSignatureCount, fmt.Sprintf("envelope carries %d signatures, not 1", len(env.Signatures)))
	}
	if env.Payload == "" {
		return nil, refuse(ReasonEmptyPayload, "envelope carries no payload")
	}
	body, ok := decodeCanonical(env.Payload)
	if !ok {
		return nil, refuse(ReasonPayloadEncoding, "payload is not canonical standard base64")
	}
	signature, ok := decodeCanonical(env.Signatures[0].Sig)
	if !ok || len(signature) == 0 {
		return nil, refuse(ReasonSignatureEncoding, "signature is empty or is not canonical standard base64")
	}
	if err := keys.VerifySignature(env.Signatures[0].KeyID, pae(wantType, body), signature); err != nil {
		if errors.Is(err, ErrUnknownKey) {
			return nil, &Refusal{Reason: ReasonUnknownKey, detail: "the verifier holds no key by the signature's key identifier", cause: err}
		}
		return nil, &Refusal{Reason: ReasonSignatureInvalid, detail: "signature does not verify as " + wantType, cause: err}
	}
	return body, nil
}

// decodeCanonical decodes standard base64 and accepts only the one encoding of
// the decoded bytes: padding present, no line breaks, zero trailing bits. The
// decoder alone skips line breaks, so the result is encoded again and compared.
func decodeCanonical(s string) ([]byte, bool) {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil || base64.StdEncoding.EncodeToString(decoded) != s {
		return nil, false
	}
	return decoded, true
}
