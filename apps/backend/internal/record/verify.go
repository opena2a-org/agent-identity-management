package record

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// FailureKind names why verification stopped.
type FailureKind string

const (
	// FailurePayloadType: the envelope's payload type is not, byte for byte,
	// the record payload type. Decided before the payload is parsed.
	FailurePayloadType FailureKind = "payload_type"
	// FailureModified: stored bytes are not the bytes that were recorded. The
	// stored hash does not match the canonical bytes, the next record commits
	// to a different hash, or an erasable part does not match its commitment.
	FailureModified FailureKind = "modified_record"
	// FailureBadSignature: the envelope does not carry exactly one signature,
	// or its signature names another key or does not verify over bytes the
	// chain otherwise accepts.
	FailureBadSignature FailureKind = "bad_signature"
	// FailureMalformed: the payload is not a record this package can read.
	FailureMalformed FailureKind = "malformed_record"
	// FailureSchema: opena2a.schema names a format version other than the one
	// in the payload type.
	FailureSchema FailureKind = "schema_version"
	// FailureRemoved: the record expected at this position is nowhere in the
	// rest of the sequence.
	FailureRemoved FailureKind = "removed_record"
	// FailureReordered: the record expected at this position sits later in
	// the sequence.
	FailureReordered FailureKind = "reordered_records"
	// FailureBrokenLink: the record does not continue the chain: another
	// chain's record, a repeated sequence number, or a predecessor hash that
	// is not the previous record's.
	FailureBrokenLink FailureKind = "broken_link"
	// FailureGenesis: the record at sequence 0 does not follow the genesis
	// rule, or a genesis sits after the start.
	FailureGenesis FailureKind = "genesis"
)

// Failure is the first point at which a sequence fails verification.
type Failure struct {
	// Position is the zero-based index in the sequence given to Verify.
	Position int
	Kind     FailureKind
	Detail   string
}

func (f Failure) Error() string {
	return fmt.Sprintf("record: verification failed at position %d: %s: %s", f.Position, f.Kind, f.Detail)
}

// Result is the outcome of walking a sequence of records.
type Result struct {
	// Verified is the number of records that verified before the walk ended.
	Verified int
	// Head is the newest verified record, nil when none verified.
	Head *Head
	// Failure is the first failure, nil when the walk reached the end.
	Failure *Failure
}

// OK reports whether at least one record verified and none failed. An empty
// sequence is no chain started, which is not a verified chain.
func (r Result) OK() bool { return r.Failure == nil && r.Verified > 0 }

// Verify walks records from the genesis of a chain, in sequence order, and
// stops at the first position where the chain or a signature fails. key is the
// chain's signing key as the verifier holds it; the records' own claims about
// keys are checked against it and never trusted in its place.
//
// Verification reads the stored canonical bytes and never serializes a record
// again. Records removed from the end of the sequence leave nothing to see
// here; compare Result.Head with a head held elsewhere to find that.
func Verify(records []Record, key PublicKey) (Result, error) {
	if err := key.validate(); err != nil {
		return Result{}, fmt.Errorf("record: the verifier's key is unusable: %v", err)
	}
	var res Result
	for i := range records {
		head, failure := verifyAt(records, i, res.Head, key)
		if failure != nil {
			res.Failure = failure
			return res, nil
		}
		res.Verified = i + 1
		res.Head = head
	}
	return res, nil
}

// verifyAt checks the record at position i against the head of the verified
// records before it.
func verifyAt(records []Record, i int, prev *Head, key PublicKey) (*Head, *Failure) {
	rec := records[i]
	fail := func(kind FailureKind, format string, args ...any) (*Head, *Failure) {
		return nil, &Failure{Position: i, Kind: kind, Detail: fmt.Sprintf(format, args...)}
	}

	if rec.Envelope.PayloadType != PayloadTypeRecordV1 {
		return fail(FailurePayloadType, "payload type %q is not %q", rec.Envelope.PayloadType, PayloadTypeRecordV1)
	}
	payload, ok := decodeCanonical(rec.Envelope.Payload)
	if !ok {
		return fail(FailureMalformed, "the payload is not canonical standard base64")
	}
	digest := hashHex(payload)
	if rec.RecordHash != digest {
		return fail(FailureModified, "the stored record hash is not the hash of the stored canonical bytes")
	}
	if reason := checkSignatures(rec.Envelope, key); reason != "" {
		// The bytes fail their signature. When the next record is authentic
		// and commits to a different hash, the bytes changed after they were
		// chained; otherwise the signature is what is wrong.
		if i+1 < len(records) {
			if next, ok := authenticHeader(records[i+1], key); ok && next.seq == int64(i)+1 && next.prevHash != nil && *next.prevHash != digest {
				return fail(FailureModified, "%s, and the next record commits to a different hash", reason)
			}
		}
		return fail(FailureBadSignature, "%s", reason)
	}

	h, err := parseHeader(payload)
	if err != nil {
		return fail(FailureMalformed, "%v", err)
	}
	if h.schema != SchemaV1 {
		return fail(FailureSchema, "opena2a.schema %q is not %q, the version of the payload type", h.schema, SchemaV1)
	}
	// The envelope carries exactly one signature, and it is the chain key's,
	// so the one algorithm a record can list is that key's.
	for _, alg := range h.sigAlgs {
		if alg != key.Alg {
			return fail(FailureBadSignature, "no signature for the listed algorithm %q", alg)
		}
	}
	if len(h.sigAlgs) == 0 {
		return fail(FailureBadSignature, "opena2a.sig_algs lists no algorithm")
	}

	if prev != nil && h.chainID != prev.ChainID {
		return fail(FailureBrokenLink, "the record belongs to chain %s, not %s", h.chainID, prev.ChainID)
	}
	want := int64(i)
	switch {
	case h.seq > want:
		if laterHolds(records, i, want, h.chainID, key) {
			return fail(FailureReordered, "sequence number %d sits where %d belongs, and %d sits later", h.seq, want, want)
		}
		return fail(FailureRemoved, "sequence number %d sits where %d belongs, and %d is nowhere after it", h.seq, want, want)
	case h.seq < want:
		return fail(FailureBrokenLink, "sequence number %d repeats or runs backwards where %d belongs", h.seq, want)
	}
	if prev == nil {
		if h.recordType != TypeChainGenesis {
			return fail(FailureGenesis, "the record at sequence 0 has type %q, not %q", h.recordType, TypeChainGenesis)
		}
		if h.prevHash != nil {
			return fail(FailureGenesis, "the genesis names a predecessor hash")
		}
		if h.firstKey == nil || h.firstKey.keyID != key.KeyID() || h.firstKey.alg != key.Alg || h.firstKey.publicKey != hex.EncodeToString(key.Key) {
			return fail(FailureGenesis, "the genesis does not name the verifier's key as its first key")
		}
	} else {
		if h.recordType == TypeChainGenesis {
			return fail(FailureGenesis, "a genesis sits after the start of the chain")
		}
		if h.prevHash == nil || *h.prevHash != prev.Hash {
			return fail(FailureBrokenLink, "the predecessor hash is not the hash of the previous record")
		}
	}

	if reason := checkPart("tenant", rec.TenantPart, rec.TenantSalt, h.commitments); reason != "" {
		return fail(FailureModified, "%s", reason)
	}
	if reason := checkPart("personal", rec.PersonalPart, rec.PersonalSalt, h.commitments); reason != "" {
		return fail(FailureModified, "%s", reason)
	}
	return &Head{ChainID: h.chainID, Seq: h.seq, Hash: digest}, nil
}

// checkSignatures returns why the envelope does not open as a record under the
// chain's key, or "". The envelope must carry exactly one signature, by the
// verifier's key, that verifies.
func checkSignatures(env Envelope, key PublicKey) string {
	if _, err := Open(env, ClassRecordV1, chainKey{key}); err != nil {
		var refusal *Refusal
		if errors.As(err, &refusal) {
			return refusal.detail
		}
		return err.Error()
	}
	return ""
}

// chainKey is the SignatureVerifier of a chain's one key.
type chainKey struct{ key PublicKey }

func (c chainKey) VerifySignature(keyID string, message, signature []byte) error {
	if keyID != c.key.KeyID() {
		return ErrUnknownKey
	}
	if !c.key.verify(message, signature) {
		return errors.New("record: the signature does not verify")
	}
	return nil
}

// checkPart returns why an erasable part does not match its commitment, or "".
// An absent part with no salt is an erased or never-present part and passes.
func checkPart(name string, part, salt []byte, commitments map[string]string) string {
	if len(part) == 0 {
		if len(salt) != 0 {
			return fmt.Sprintf("the %s part is absent but its salt is present", name)
		}
		return ""
	}
	if len(salt) != SaltSize {
		return fmt.Sprintf("the %s salt is %d bytes, not %d", name, len(salt), SaltSize)
	}
	committed, ok := commitments[name]
	if !ok {
		return fmt.Sprintf("the %s part is present but the record commits to none", name)
	}
	if commitment(salt, part) != committed {
		return fmt.Sprintf("the %s part does not match its commitment", name)
	}
	return ""
}

// laterHolds reports whether an authentic record of the chain with sequence
// number seq sits after position i.
func laterHolds(records []Record, i int, seq int64, chainID string, key PublicKey) bool {
	for j := i + 1; j < len(records); j++ {
		if h, ok := authenticHeader(records[j], key); ok && h.seq == seq && h.chainID == chainID {
			return true
		}
	}
	return false
}

// authenticHeader returns the header of a record whose type, hash and
// signatures hold, whatever its place in the chain.
func authenticHeader(rec Record, key PublicKey) (header, bool) {
	if rec.Envelope.PayloadType != PayloadTypeRecordV1 {
		return header{}, false
	}
	payload, ok := decodeCanonical(rec.Envelope.Payload)
	if !ok || rec.RecordHash != hashHex(payload) {
		return header{}, false
	}
	if checkSignatures(rec.Envelope, key) != "" {
		return header{}, false
	}
	h, err := parseHeader(payload)
	return h, err == nil
}

// header is what the verifier reads from a record's canonical bytes.
type header struct {
	recordType  string
	schema      string
	chainID     string
	seq         int64
	prevHash    *string
	sigAlgs     []string
	commitments map[string]string
	firstKey    *firstKey
}

type firstKey struct {
	keyID     string
	alg       string
	publicKey string
}

func parseHeader(payload []byte) (header, error) {
	var doc struct {
		Type    *string `json:"type"`
		OpenA2A *struct {
			Schema *string `json:"schema"`
			Chain  *struct {
				ID       *string `json:"id"`
				Seq      *int64  `json:"seq"`
				PrevHash *string `json:"prev_hash"`
			} `json:"chain"`
			SigAlgs     []string          `json:"sig_algs"`
			Commitments map[string]string `json:"commitments"`
			FirstKey    *struct {
				KeyID     string `json:"keyid"`
				Alg       string `json:"alg"`
				PublicKey string `json:"public_key"`
			} `json:"first_key"`
		} `json:"opena2a"`
	}
	if err := json.Unmarshal(payload, &doc); err != nil {
		return header{}, fmt.Errorf("the payload is not a JSON record: %v", err)
	}
	switch {
	case doc.Type == nil:
		return header{}, errors.New("the record has no type")
	case doc.OpenA2A == nil:
		return header{}, errors.New("the record has no opena2a member")
	case doc.OpenA2A.Schema == nil:
		return header{}, errors.New("the record has no opena2a.schema")
	case doc.OpenA2A.Chain == nil || doc.OpenA2A.Chain.ID == nil || doc.OpenA2A.Chain.Seq == nil:
		return header{}, errors.New("the record has no opena2a.chain id and sequence number")
	case *doc.OpenA2A.Chain.Seq < 0:
		return header{}, errors.New("the record's sequence number is negative")
	}
	h := header{
		recordType:  *doc.Type,
		schema:      *doc.OpenA2A.Schema,
		chainID:     *doc.OpenA2A.Chain.ID,
		seq:         *doc.OpenA2A.Chain.Seq,
		prevHash:    doc.OpenA2A.Chain.PrevHash,
		sigAlgs:     doc.OpenA2A.SigAlgs,
		commitments: doc.OpenA2A.Commitments,
	}
	if k := doc.OpenA2A.FirstKey; k != nil {
		h.firstKey = &firstKey{keyID: k.KeyID, alg: k.Alg, publicKey: k.PublicKey}
	}
	return h, nil
}
