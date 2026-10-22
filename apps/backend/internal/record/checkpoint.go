package record

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// SchemaCheckpointV1 is the value of opena2a.schema in a checkpoint of format
// version 1. Its version is the one PayloadTypeCheckpointV1 names.
const SchemaCheckpointV1 = "opena2a-audit-checkpoint/1"

// Checkpoint is a chain's head at one moment, held outside the sequence of
// records: the chain, its newest record's sequence number and hash, and when
// the checkpoint was taken. A chain's records show nothing of records removed
// from their end, and nothing of a chain replaced from some record on by one
// its own key signed; a checkpoint taken before either shows it.
type Checkpoint struct {
	// Head names the chain and its newest record when the checkpoint was
	// taken.
	Head Head
	// Timestamp is when the checkpoint was taken, to the microsecond.
	Timestamp time.Time
	// KeyID identifies the key whose signature the checkpoint's envelope
	// carries. OpenCheckpoint sets it; a checkpoint built by NewCheckpoint
	// has none yet.
	KeyID string
}

// CheckpointBody is a checkpoint with its canonical bytes fixed, not yet
// signed. Only NewCheckpoint produces one, so the bytes SignCheckpoint hands
// to a key provider come from this package's serializer.
type CheckpointBody struct {
	canonical []byte
	head      Head
}

// Canonical returns the canonical bytes: the bytes the signature covers.
func (b CheckpointBody) Canonical() []byte { return append([]byte(nil), b.canonical...) }

// Head returns the head the checkpoint names.
func (b CheckpointBody) Head() Head { return b.head }

// NewCheckpoint builds the checkpoint of a chain whose newest record is head,
// taken at takenAt. Its bytes are the canonical serialization of the chain
// id, the sequence number and the hash under opena2a.chain, the schema under
// opena2a.schema, and the time under timestamp.
func NewCheckpoint(head Head, takenAt time.Time) (CheckpointBody, error) {
	if err := checkHead(head); err != nil {
		return CheckpointBody{}, err
	}
	ts, err := FormatTimestamp(takenAt)
	if err != nil {
		return CheckpointBody{}, fmt.Errorf("%w: checkpoint: %v", ErrInvalidRecord, err)
	}
	canonical, err := canonicalJSON(map[string]any{
		"timestamp": ts,
		extensionMember: map[string]any{
			"schema": SchemaCheckpointV1,
			"chain":  map[string]any{"id": head.ChainID, "seq": head.Seq, "hash": head.Hash},
		},
	})
	if err != nil {
		return CheckpointBody{}, fmt.Errorf("%w: checkpoint: %v", ErrInvalidRecord, err)
	}
	return CheckpointBody{canonical: canonical, head: head}, nil
}

// SignCheckpoint seals a checkpoint in its envelope as a payload of
// ClassCheckpointV1, signed by the provider's key. The key that signs a
// checkpoint is the key in force after the checkpoint's head: the key that
// signs the chain's next record. A verifier accepts a checkpoint under that
// key alone, so after a key rotation the provider of the next key signs the
// checkpoint; a verifier refuses one the retired key signs.
func SignCheckpoint(ctx context.Context, b CheckpointBody, kp SigningKeyProvider) (Envelope, error) {
	if len(b.canonical) == 0 {
		return Envelope{}, fmt.Errorf("%w: the checkpoint was not built by NewCheckpoint", ErrSigning)
	}
	key, err := providerKey(ctx, kp)
	if err != nil {
		return Envelope{}, err
	}
	return sealUnder(ctx, ClassCheckpointV1, Payload{canonical: b.canonical}, kp, key)
}

// OpenCheckpoint checks env as a checkpoint signed by a key that keys holds
// and reads it. The payload type is compared before anything else is read, so
// a record offered as a checkpoint is refused with ReasonPayloadTypeMismatch.
// A payload that is not a checkpoint of this format in canonical form is
// refused with ReasonMalformedPayload, and one whose opena2a.schema names
// another format version with ReasonSchemaVersion.
func OpenCheckpoint(env Envelope, keys SignatureVerifier) (Checkpoint, error) {
	payload, err := Open(env, ClassCheckpointV1, keys)
	if err != nil {
		return Checkpoint{}, err
	}
	cp, err := parseCheckpoint(payload)
	if err != nil {
		return Checkpoint{}, err
	}
	cp.KeyID = env.Signatures[0].KeyID
	return cp, nil
}

// parseCheckpoint reads authenticated checkpoint bytes. It refuses a member it
// does not know, a missing member, a value outside its form, and bytes that
// are not the canonical serialization of what they read as.
func parseCheckpoint(payload []byte) (Checkpoint, error) {
	var doc struct {
		Timestamp *string `json:"timestamp"`
		OpenA2A   *struct {
			Schema *string `json:"schema"`
			Chain  *struct {
				ID   *string `json:"id"`
				Seq  *int64  `json:"seq"`
				Hash *string `json:"hash"`
			} `json:"chain"`
		} `json:"opena2a"`
	}
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return Checkpoint{}, refuse(ReasonMalformedPayload, "the payload is not a JSON checkpoint with the members of this format")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Checkpoint{}, refuse(ReasonMalformedPayload, "the payload carries bytes after the checkpoint")
	}
	switch {
	case doc.OpenA2A == nil:
		return Checkpoint{}, refuse(ReasonMalformedPayload, "the checkpoint has no opena2a member")
	case doc.OpenA2A.Schema == nil:
		return Checkpoint{}, refuse(ReasonMalformedPayload, "the checkpoint has no opena2a.schema")
	case *doc.OpenA2A.Schema != SchemaCheckpointV1:
		return Checkpoint{}, refuse(ReasonSchemaVersion, "opena2a.schema is not "+SchemaCheckpointV1+", the version of the payload type")
	case doc.OpenA2A.Chain == nil || doc.OpenA2A.Chain.ID == nil || doc.OpenA2A.Chain.Seq == nil || doc.OpenA2A.Chain.Hash == nil:
		return Checkpoint{}, refuse(ReasonMalformedPayload, "the checkpoint has no opena2a.chain id, sequence number and hash")
	case doc.Timestamp == nil:
		return Checkpoint{}, refuse(ReasonMalformedPayload, "the checkpoint has no timestamp")
	case !isCanonicalTimestamp(*doc.Timestamp):
		return Checkpoint{}, refuse(ReasonMalformedPayload, "timestamp is not a timestamp in canonical form")
	}
	head := Head{ChainID: *doc.OpenA2A.Chain.ID, Seq: *doc.OpenA2A.Chain.Seq, Hash: *doc.OpenA2A.Chain.Hash}
	takenAt, err := time.Parse(timestampLayout, *doc.Timestamp)
	if err != nil {
		return Checkpoint{}, refuse(ReasonMalformedPayload, "timestamp is not a timestamp in canonical form")
	}
	rebuilt, err := NewCheckpoint(head, takenAt)
	if err != nil {
		return Checkpoint{}, refuse(ReasonMalformedPayload, "opena2a.chain does not name a head a record can link to")
	}
	if !bytes.Equal(rebuilt.canonical, payload) {
		return Checkpoint{}, refuse(ReasonMalformedPayload, "the payload is not the canonical serialization of the checkpoint it reads as")
	}
	return Checkpoint{Head: head, Timestamp: takenAt}, nil
}

// ChainKeys returns a verifier of the keys the walk saw: the key given to
// Verify and the next key of each key rotation that verified. A checkpoint
// of the chain opens under one of these keys; one that does not is another
// chain's, or was signed by a key that a record missing from the sequence
// named.
func (r Result) ChainKeys() SignatureVerifier { return r.ring }

// keyInForceAfter returns the key that signs the record after sequence seq,
// as the walk found it. ok is false when seq is beyond the verified records.
func (r Result) keyInForceAfter(seq int64) (PublicKey, bool) {
	if seq < 0 || seq >= int64(r.Verified) {
		return PublicKey{}, false
	}
	for _, old := range r.ring.retired {
		if old.seq > seq {
			return old.key, true
		}
	}
	return r.ring.current, true
}

// VerifyCheckpoint checks a held checkpoint against the records that
// verified. It opens env under ChainKeys, accepts the checkpoint only when it
// names the walked chain and is signed by the key in force after its
// sequence, and then compares it with the walk:
//
//   - a checkpoint whose sequence the verified records do not reach is
//     reported as FailureTruncated: records were removed from the end of the
//     chain, or the walk ended at a failure before the checkpoint;
//   - a checkpoint whose sequence verified under a different hash is reported
//     as FailureCheckpointMismatch: the chain verifies on its own, so from
//     that record on it was replaced by one the chain's key signed;
//   - a checkpoint that does not open under a key the walk saw, names another
//     chain, or is signed by a key that was not in force after its sequence
//     is reported as FailureCheckpoint.
//
// A checkpoint that agrees with the walk is returned with a nil failure. The
// records themselves are not read again: the walk kept the hash of each
// record it verified.
func (r Result) VerifyCheckpoint(env Envelope) (Checkpoint, *Failure) {
	cp, err := OpenCheckpoint(env, r.ring)
	if err != nil {
		reason, _ := ReasonOf(err)
		detail := "the checkpoint does not open: " + string(reason)
		if reason == ReasonUnknownKey {
			detail = "the checkpoint is signed by a key that no verified record of the chain named: it is another chain's, or the records that named its key are missing"
		}
		return Checkpoint{}, &Failure{Position: r.Verified, Kind: FailureCheckpoint, Detail: detail}
	}
	seq := cp.Head.Seq
	position := int(seq)
	if r.Head != nil && cp.Head.ChainID != r.Head.ChainID {
		return cp, &Failure{Position: position, Kind: FailureCheckpoint,
			Detail: fmt.Sprintf("the checkpoint names chain %s, not %s", cp.Head.ChainID, r.Head.ChainID)}
	}
	if seq >= int64(r.Verified) {
		detail := fmt.Sprintf("the checkpoint names sequence %d of a chain of which no record verified", seq)
		if r.Verified > 0 {
			detail = fmt.Sprintf("the checkpoint names sequence %d, and the records that verified end at sequence %d", seq, r.Verified-1)
		}
		if r.Failure != nil {
			detail += fmt.Sprintf("; verification failed at position %d", r.Failure.Position)
		}
		return cp, &Failure{Position: position, Kind: FailureTruncated, Detail: detail}
	}
	key, _ := r.keyInForceAfter(seq)
	if cp.KeyID != key.KeyID() {
		return cp, &Failure{Position: position, Kind: FailureCheckpoint,
			Detail: fmt.Sprintf("the checkpoint is signed by key %s, which was not in force after sequence %d", cp.KeyID, seq)}
	}
	if r.hashes[seq] != cp.Head.Hash {
		return cp, &Failure{Position: position, Kind: FailureCheckpointMismatch,
			Detail: fmt.Sprintf("the record at sequence %d verified, and the checkpoint names a different hash for it", seq)}
	}
	return cp, nil
}
