package record

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const (
	// SchemaV1 is the value of opena2a.schema in every record of format
	// version 1. Its version is the one PayloadTypeRecordV1 names.
	SchemaV1 = "opena2a-audit-record/1"

	// TypeChainGenesis is the type of the first record of a chain.
	TypeChainGenesis = "opena2a.chain_genesis"

	// SaltSize is the length in bytes of the salt of an erasable part.
	SaltSize = 32

	// extensionMember is the one member that holds everything outside the
	// draft-defined member names.
	extensionMember = "opena2a"
)

// ErrInvalidRecord is wrapped by every error that refuses a draft, a genesis
// or a chain head.
var ErrInvalidRecord = errors.New("record: invalid record")

// Draft is a record before it takes a place in a chain. A part with no member
// is absent; a part that exists needs a salt of SaltSize bytes from a
// cryptographic random source, and an absent part takes none.
type Draft struct {
	// EventID is event_id: a UUID in lowercase hyphenated form.
	EventID string
	// Type is the record type.
	Type string
	// Timestamp is rendered by FormatTimestamp.
	Timestamp time.Time
	// Retained holds the further members of the retained part. Members under
	// "opena2a" sit in one map[string]any. The members this package sets
	// (event_id, type, timestamp, and schema, chain, commitments, sig_algs,
	// first_key and pre_genesis under "opena2a") are refused here.
	Retained map[string]any
	// Tenant and Personal are the erasable parts.
	Tenant       map[string]any
	TenantSalt   []byte
	Personal     map[string]any
	PersonalSalt []byte
}

// Head names the newest record of a chain: what the next record links to.
type Head struct {
	// ChainID is opena2a.chain.id.
	ChainID string
	// Seq is opena2a.chain.seq of the newest record.
	Seq int64
	// Hash is the newest record's hash: lowercase hex SHA-256 of its canonical
	// bytes.
	Hash string
}

// Body is a record with its place in a chain and its canonical bytes fixed,
// not yet signed. Only NewGenesis and NewRecord produce one, so the bytes Sign
// hands to a key provider always come from this package's serializer.
type Body struct {
	canonical    []byte
	head         Head
	recordType   string
	firstKeyID   string
	tenantPart   []byte
	tenantSalt   []byte
	personalPart []byte
	personalSalt []byte
}

// Canonical returns the canonical bytes: the only bytes the chain and the
// signatures cover.
func (b Body) Canonical() []byte { return append([]byte(nil), b.canonical...) }

// Head returns the head of the chain once this record is its newest.
func (b Body) Head() Head { return b.head }

// Genesis is the input of the first record of a chain.
type Genesis struct {
	// ChainID is a UUID minted for this chain, in lowercase hyphenated form.
	ChainID string
	// FirstKey is the key the chain's records are signed with from the start.
	FirstKey PublicKey
	// PreGenesis names every row that existed before the chain started.
	PreGenesis []PreGenesisRow
	// Draft carries the genesis record's own members. Its Type is
	// TypeChainGenesis or empty.
	Draft Draft
}

// NewGenesis builds the first record of a chain: sequence 0 and a null
// predecessor hash. It names the chain's first key and pins the rows that
// existed before the chain.
func NewGenesis(g Genesis) (Body, error) {
	d := g.Draft
	if d.Type == "" {
		d.Type = TypeChainGenesis
	}
	if d.Type != TypeChainGenesis {
		return Body{}, fmt.Errorf("%w: a genesis has type %s, not %q", ErrInvalidRecord, TypeChainGenesis, d.Type)
	}
	if err := g.FirstKey.validate(); err != nil {
		return Body{}, fmt.Errorf("%w: first key: %v", ErrInvalidRecord, err)
	}
	pre, err := preGenesisMember(g.PreGenesis)
	if err != nil {
		return Body{}, err
	}
	own := map[string]any{
		"first_key": map[string]any{
			"keyid":      g.FirstKey.KeyID(),
			"alg":        g.FirstKey.Alg,
			"public_key": hex.EncodeToString(g.FirstKey.Key),
		},
		"pre_genesis": pre,
	}
	b, err := seal(g.ChainID, 0, nil, d, own)
	if err != nil {
		return Body{}, err
	}
	b.firstKeyID = g.FirstKey.KeyID()
	return b, nil
}

// NewRecord builds the record that follows prev: sequence prev.Seq plus one,
// committing to prev.Hash.
func NewRecord(prev Head, d Draft) (Body, error) {
	if !isCanonicalUUID(prev.ChainID) {
		return Body{}, fmt.Errorf("%w: the head's chain id is not a lowercase hyphenated UUID", ErrInvalidRecord)
	}
	if prev.Seq < 0 {
		return Body{}, fmt.Errorf("%w: the head's sequence number is negative", ErrInvalidRecord)
	}
	if !isLowerHexDigest(prev.Hash) {
		return Body{}, fmt.Errorf("%w: the head's hash is not 64 lowercase hex characters", ErrInvalidRecord)
	}
	if d.Type == TypeChainGenesis {
		return Body{}, fmt.Errorf("%w: a chain has one genesis, at sequence 0", ErrInvalidRecord)
	}
	return seal(prev.ChainID, prev.Seq+1, prev.Hash, d, nil)
}

// seal fixes a draft's place in a chain and serializes its retained part. own
// holds the "opena2a" members only this package may set on this record.
func seal(chainID string, seq int64, prevHash any, d Draft, own map[string]any) (Body, error) {
	if !isCanonicalUUID(chainID) {
		return Body{}, fmt.Errorf("%w: the chain id is not a lowercase hyphenated UUID", ErrInvalidRecord)
	}
	if !isCanonicalUUID(d.EventID) {
		return Body{}, fmt.Errorf("%w: event_id is not a lowercase hyphenated UUID", ErrInvalidRecord)
	}
	if d.Type == "" {
		return Body{}, fmt.Errorf("%w: type is not set", ErrInvalidRecord)
	}
	ts, err := FormatTimestamp(d.Timestamp)
	if err != nil {
		return Body{}, fmt.Errorf("%w: %v", ErrInvalidRecord, err)
	}

	retained := make(map[string]any, len(d.Retained)+4)
	for k, v := range d.Retained {
		retained[k] = v
	}
	for _, name := range []string{"event_id", "type", "timestamp"} {
		if _, set := retained[name]; set {
			return Body{}, fmt.Errorf("%w: %s is set by the record package, not by the caller", ErrInvalidRecord, name)
		}
	}
	ext := map[string]any{}
	if raw, present := retained[extensionMember]; present {
		given, ok := raw.(map[string]any)
		if !ok {
			return Body{}, fmt.Errorf("%w: %s in the retained part is not an object", ErrInvalidRecord, extensionMember)
		}
		for k, v := range given {
			ext[k] = v
		}
	}
	for _, name := range []string{"schema", "chain", "commitments", "sig_algs", "first_key", "pre_genesis"} {
		if _, set := ext[name]; set {
			return Body{}, fmt.Errorf("%w: %s.%s is set by the record package, not by the caller", ErrInvalidRecord, extensionMember, name)
		}
	}

	tenantPart, tenantCommitment, err := sealPart("tenant", d.Tenant, d.TenantSalt)
	if err != nil {
		return Body{}, err
	}
	personalPart, personalCommitment, err := sealPart("personal", d.Personal, d.PersonalSalt)
	if err != nil {
		return Body{}, err
	}
	commitments := map[string]any{}
	if tenantPart != nil {
		commitments["tenant"] = tenantCommitment
	}
	if personalPart != nil {
		commitments["personal"] = personalCommitment
	}

	retained["event_id"] = d.EventID
	retained["type"] = d.Type
	retained["timestamp"] = ts
	ext["schema"] = SchemaV1
	ext["chain"] = map[string]any{"id": chainID, "seq": seq, "prev_hash": prevHash}
	ext["commitments"] = commitments
	ext["sig_algs"] = []any{AlgEd25519}
	for k, v := range own {
		ext[k] = v
	}
	retained[extensionMember] = ext

	if err := checkDisjoint(retained, d.Tenant, d.Personal); err != nil {
		return Body{}, err
	}
	canonical, err := canonicalJSON(retained)
	if err != nil {
		return Body{}, fmt.Errorf("%w: retained part: %v", ErrInvalidRecord, err)
	}
	return Body{
		canonical:    canonical,
		head:         Head{ChainID: chainID, Seq: seq, Hash: hashHex(canonical)},
		recordType:   d.Type,
		tenantPart:   tenantPart,
		tenantSalt:   append([]byte(nil), d.TenantSalt...),
		personalPart: personalPart,
		personalSalt: append([]byte(nil), d.PersonalSalt...),
	}, nil
}

// sealPart serializes one erasable part and computes its commitment. It
// returns nil bytes for an absent part.
func sealPart(name string, part map[string]any, salt []byte) ([]byte, string, error) {
	if len(part) == 0 {
		if len(salt) != 0 {
			return nil, "", fmt.Errorf("%w: the %s part is absent but has a salt", ErrInvalidRecord, name)
		}
		return nil, "", nil
	}
	if len(salt) != SaltSize {
		return nil, "", fmt.Errorf("%w: the %s salt is %d bytes, not %d", ErrInvalidRecord, name, len(salt), SaltSize)
	}
	if raw, present := part[extensionMember]; present {
		if _, ok := raw.(map[string]any); !ok {
			return nil, "", fmt.Errorf("%w: %s in the %s part is not an object", ErrInvalidRecord, extensionMember, name)
		}
	}
	bytes, err := canonicalJSON(part)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %s part: %v", ErrInvalidRecord, name, err)
	}
	return bytes, commitment(salt, bytes), nil
}

// commitment is the lowercase hex SHA-256 of the salt followed by the part's
// canonical bytes.
func commitment(salt, part []byte) string {
	h := sha256.New()
	h.Write(salt)
	h.Write(part)
	return hex.EncodeToString(h.Sum(nil))
}

// checkDisjoint refuses a member name that sits in more than one part. Names
// under "opena2a" are compared with each other, not with top-level names.
func checkDisjoint(retained, tenant, personal map[string]any) error {
	parts := []struct {
		name    string
		members map[string]any
	}{{"retained", retained}, {"tenant", tenant}, {"personal", personal}}
	seenTop := map[string]string{}
	seenExt := map[string]string{}
	for _, p := range parts {
		for k, v := range p.members {
			if k != extensionMember {
				if other, dup := seenTop[k]; dup {
					return fmt.Errorf("%w: %s sits in both the %s and the %s part", ErrInvalidRecord, k, other, p.name)
				}
				seenTop[k] = p.name
				continue
			}
			ext, _ := v.(map[string]any)
			for ek := range ext {
				if other, dup := seenExt[ek]; dup {
					return fmt.Errorf("%w: %s.%s sits in both the %s and the %s part", ErrInvalidRecord, extensionMember, ek, other, p.name)
				}
				seenExt[ek] = p.name
			}
		}
	}
	return nil
}

func hashHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// isLowerHexDigest reports whether s is a SHA-256 digest in lowercase hex.
func isLowerHexDigest(s string) bool {
	if len(s) != 2*sha256.Size {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isLowerHex(s[i]) {
			return false
		}
	}
	return true
}

// isCanonicalUUID reports whether s is a UUID in lowercase hyphenated form.
func isCanonicalUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch i {
		case 8, 13, 18, 23:
			if s[i] != '-' {
				return false
			}
		default:
			if !isLowerHex(s[i]) {
				return false
			}
		}
	}
	return true
}

func isLowerHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
}
