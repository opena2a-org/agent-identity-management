package record

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// chainVectorFile is one golden vector file of the chain. Vectors pin the bytes a record is made
// of; they carry no signature, because every signing key is generated when a
// test runs and none is ever stored. The first key they name is a public key
// with no private half.
type chainVectorFile struct {
	Description   string         `json:"description"`
	PayloadType   string         `json:"payloadType"`
	FirstKey      vectorKey      `json:"firstKey"`
	PreGenesisSet string         `json:"preGenesisSet"`
	Records       []vectorRecord `json:"records"`
}

type vectorKey struct {
	Alg       string `json:"alg"`
	PublicKey string `json:"publicKey"`
	KeyID     string `json:"keyid"`
}

// vectorRecord holds one record: canonical, tenantPart, personalPart and
// signingInput are the exact bytes as text; hashes and salts are lowercase
// hex.
type vectorRecord struct {
	Canonical    string `json:"canonical"`
	RecordHash   string `json:"recordHash"`
	SigningInput string `json:"signingInput"`
	TenantPart   string `json:"tenantPart,omitempty"`
	TenantSalt   string `json:"tenantSalt,omitempty"`
	PersonalPart string `json:"personalPart,omitempty"`
	PersonalSalt string `json:"personalSalt,omitempty"`
}

func vectorOf(t *testing.T, description string, pre []PreGenesisRow, bodies []Body) []byte {
	t.Helper()
	key := keylessKey()
	set, err := preGenesisSet(pre)
	if err != nil {
		t.Fatalf("pre-genesis set: %v", err)
	}
	file := chainVectorFile{
		Description:   description,
		PayloadType:   PayloadTypeRecordV1,
		FirstKey:      vectorKey{Alg: key.Alg, PublicKey: hex.EncodeToString(key.Key), KeyID: key.KeyID()},
		PreGenesisSet: string(set),
	}
	for _, b := range bodies {
		signingInput, err := SigningInput(ClassRecordV1, Payload{canonical: b.canonical})
		if err != nil {
			t.Fatalf("signing input: %v", err)
		}
		file.Records = append(file.Records, vectorRecord{
			Canonical:    string(b.canonical),
			RecordHash:   b.head.Hash,
			SigningInput: string(signingInput),
			TenantPart:   string(b.tenantPart),
			TenantSalt:   hex.EncodeToString(b.tenantSalt),
			PersonalPart: string(b.personalPart),
			PersonalSalt: hex.EncodeToString(b.personalSalt),
		})
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(file); err != nil {
		t.Fatalf("encode vector: %v", err)
	}
	return out.Bytes()
}

// TestChainGoldenVectors regenerates the chain vectors. update rewrites them.
// The same input must give the same bytes for as long as the schema version
// stands, so a rewrite belongs with a change of schema version and nowhere
// else.
func TestChainGoldenVectors(t *testing.T) {
	genesis, err := NewGenesis(fixtureGenesis(keylessKey(), nil))
	if err != nil {
		t.Fatalf("NewGenesis: %v", err)
	}
	vectors := []struct {
		file string
		got  []byte
	}{
		{"genesis.json", vectorOf(t,
			"The genesis record of a chain that starts with no earlier rows.",
			nil, []Body{genesis})},
		{"chain.json", vectorOf(t,
			"A genesis that pins three earlier rows, then three records: one with a tenant and a personal part, two with a tenant part alone.",
			fixturePreGenesis(), buildChain(t, keylessKey()))},
	}
	for _, v := range vectors {
		t.Run(v.file, func(t *testing.T) {
			path := filepath.Join("testdata", v.file)
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatalf("testdata: %v", err)
				}
				if err := os.WriteFile(path, v.got, 0o644); err != nil {
					t.Fatalf("write %s: %v", path, err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			if !bytes.Equal(v.got, want) {
				at := 0
				for at < len(v.got) && at < len(want) && v.got[at] == want[at] {
					at++
				}
				t.Fatalf("%s: the bytes regenerated from the fixed inputs differ from the committed vector, first at byte %d.\n"+
					"The same input must give the same bytes unless the schema version changes.\n"+
					"regenerated: %d bytes\ncommitted:   %d bytes", path, at, len(v.got), len(want))
			}
			checkVectorFile(t, want)
		})
	}
}

// checkVectorFile reads a committed vector with nothing but a JSON decoder and
// SHA-256, and checks what it claims: each hash, each signing input, each
// commitment and each link to the record before.
func checkVectorFile(t *testing.T, raw []byte) {
	t.Helper()
	var file chainVectorFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("the vector is not JSON: %v", err)
	}
	if len(file.Records) == 0 {
		t.Fatalf("the vector holds no record")
	}
	sha := func(parts ...[]byte) string {
		h := sha256.New()
		for _, p := range parts {
			h.Write(p)
		}
		return hex.EncodeToString(h.Sum(nil))
	}
	type body struct {
		Type    string `json:"type"`
		OpenA2A struct {
			Schema string `json:"schema"`
			Chain  struct {
				ID       string  `json:"id"`
				Seq      int     `json:"seq"`
				PrevHash *string `json:"prev_hash"`
			} `json:"chain"`
			Commitments map[string]string `json:"commitments"`
			FirstKey    *vectorFirstKey   `json:"first_key"`
			PreGenesis  *struct {
				SetDigest string `json:"set_digest"`
			} `json:"pre_genesis"`
		} `json:"opena2a"`
	}
	for i, r := range file.Records {
		if got := sha([]byte(r.Canonical)); got != r.RecordHash {
			t.Fatalf("record %d: recordHash is not the SHA-256 of canonical", i)
		}
		wantInput := "DSSEv1 " + strconv.Itoa(len(file.PayloadType)) + " " + file.PayloadType + " " +
			strconv.Itoa(len(r.Canonical)) + " " + r.Canonical
		if r.SigningInput != wantInput {
			t.Fatalf("record %d: signingInput is not the pre-authentication encoding of canonical", i)
		}
		var b body
		if err := json.Unmarshal([]byte(r.Canonical), &b); err != nil {
			t.Fatalf("record %d: canonical is not JSON: %v", i, err)
		}
		if b.OpenA2A.Schema != "opena2a-audit-record/1" {
			t.Fatalf("record %d: schema %q", i, b.OpenA2A.Schema)
		}
		if b.OpenA2A.Chain.Seq != i {
			t.Fatalf("record %d: sequence number %d", i, b.OpenA2A.Chain.Seq)
		}
		if i == 0 {
			if b.Type != "opena2a.chain_genesis" || b.OpenA2A.Chain.PrevHash != nil {
				t.Fatalf("record 0 is not a genesis with a null predecessor hash")
			}
			if b.OpenA2A.FirstKey == nil || *b.OpenA2A.FirstKey != (vectorFirstKey{Alg: file.FirstKey.Alg, KeyID: file.FirstKey.KeyID, PublicKey: file.FirstKey.PublicKey}) {
				t.Fatalf("the genesis does not name the vector's first key")
			}
			publicKey, err := hex.DecodeString(file.FirstKey.PublicKey)
			if err != nil || sha(publicKey) != file.FirstKey.KeyID {
				t.Fatalf("the first key's id is not the SHA-256 of its bytes")
			}
			if b.OpenA2A.PreGenesis == nil || b.OpenA2A.PreGenesis.SetDigest != sha([]byte(file.PreGenesisSet)) {
				t.Fatalf("set_digest is not the SHA-256 of preGenesisSet")
			}
		} else if b.OpenA2A.Chain.PrevHash == nil || *b.OpenA2A.Chain.PrevHash != file.Records[i-1].RecordHash {
			t.Fatalf("record %d does not commit to the hash of record %d", i, i-1)
		}
		for name, part := range map[string][2]string{
			"tenant":   {r.TenantSalt, r.TenantPart},
			"personal": {r.PersonalSalt, r.PersonalPart},
		} {
			committed, has := b.OpenA2A.Commitments[name]
			if part[1] == "" {
				if has {
					t.Fatalf("record %d commits to a %s part the vector does not hold", i, name)
				}
				continue
			}
			salt, err := hex.DecodeString(part[0])
			if err != nil || len(salt) != 32 {
				t.Fatalf("record %d: the %s salt is not 32 bytes of hex", i, name)
			}
			if committed != sha(salt, []byte(part[1])) {
				t.Fatalf("record %d: the %s commitment is not SHA-256(salt || part)", i, name)
			}
		}
	}
}

type vectorFirstKey struct {
	Alg       string `json:"alg"`
	KeyID     string `json:"keyid"`
	PublicKey string `json:"public_key"`
}
