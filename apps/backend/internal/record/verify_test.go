package record

import (
	"bytes"
	"context"
	"encoding/base64"
	"testing"
)

// replaceInPayload returns the payload with one occurrence of old replaced.
func replaceInPayload(t *testing.T, payload []byte, old, new string) []byte {
	t.Helper()
	if bytes.Count(payload, []byte(old)) != 1 {
		t.Fatalf("%q does not occur exactly once in %s", old, payload)
	}
	return bytes.Replace(payload, []byte(old), []byte(new), 1)
}

func TestVerify_AcceptsAnIntactChain(t *testing.T) {
	records, kp := signedChain(t)
	res := mustVerify(t, records, kp.key(t))
	if !res.OK() || res.Failure != nil {
		t.Fatalf("an intact chain fails: %+v", res.Failure)
	}
	if res.Verified != len(records) {
		t.Fatalf("verified %d of %d records", res.Verified, len(records))
	}
	last := records[len(records)-1]
	if want := (Head{ChainID: fixtureChainID, Seq: int64(len(records) - 1), Hash: last.RecordHash}); res.Head == nil || *res.Head != want {
		t.Fatalf("head %+v; want %+v", res.Head, want)
	}
	for n := 1; n <= len(records); n++ {
		if prefix := mustVerify(t, records[:n], kp.key(t)); !prefix.OK() || prefix.Verified != n {
			t.Fatalf("the first %d records fail: %+v", n, prefix.Failure)
		}
	}
}

func TestVerify_AnEmptySequenceIsNotAVerifiedChain(t *testing.T) {
	kp := newMemoryKeyProvider(t)
	res := mustVerify(t, nil, kp.key(t))
	if res.OK() || res.Failure != nil || res.Verified != 0 || res.Head != nil {
		t.Fatalf("an empty sequence reports %+v", res)
	}
}

func TestVerify_ReportsAModifiedRecord(t *testing.T) {
	records, kp := signedChain(t)
	key := kp.key(t)

	t.Run("one byte of the canonical bytes changed", func(t *testing.T) {
		for position := range records {
			tampered := cloneRecords(records)
			payload := payloadBytes(t, tampered[position])
			payload[len(payload)/2] ^= 0x01
			setPayload(&tampered[position], payload)
			wantFailure(t, mustVerify(t, tampered, key), position, FailureModified)
		}
	})

	t.Run("a value changed and the stored hash recomputed", func(t *testing.T) {
		tampered := cloneRecords(records)
		payload := replaceInPayload(t, payloadBytes(t, tampered[2]), `"outcome":"allowed"`, `"outcome":"refused"`)
		setPayload(&tampered[2], payload)
		tampered[2].RecordHash = hashHex(payload)
		wantFailure(t, mustVerify(t, tampered, key), 2, FailureModified)
	})

	t.Run("a value changed, the stored hash recomputed, and no later record", func(t *testing.T) {
		// With no successor to consult, the changed bytes are seen as bytes
		// their signature does not cover.
		tampered := cloneRecords(records)
		last := len(tampered) - 1
		payload := replaceInPayload(t, payloadBytes(t, tampered[last]), `"admin_action":"tag_created"`, `"admin_action":"tag_deleted"`)
		setPayload(&tampered[last], payload)
		tampered[last].RecordHash = hashHex(payload)
		wantFailure(t, mustVerify(t, tampered, key), last, FailureBadSignature)
	})

	t.Run("the stored hash changed", func(t *testing.T) {
		tampered := cloneRecords(records)
		tampered[1].RecordHash = records[2].RecordHash
		wantFailure(t, mustVerify(t, tampered, key), 1, FailureModified)
	})

	t.Run("an erasable part changed", func(t *testing.T) {
		tampered := cloneRecords(records)
		tampered[1].PersonalPart = replaceInPayload(t, tampered[1].PersonalPart, `"case_number":4711`, `"case_number":4712`)
		wantFailure(t, mustVerify(t, tampered, key), 1, FailureModified)

		tampered = cloneRecords(records)
		tampered[2].TenantPart = replaceInPayload(t, tampered[2].TenantPart, fixtureAgent, fixtureOrg)
		wantFailure(t, mustVerify(t, tampered, key), 2, FailureModified)
	})

	t.Run("a salt changed or left behind", func(t *testing.T) {
		tampered := cloneRecords(records)
		tampered[1].TenantSalt[0] ^= 0x01
		wantFailure(t, mustVerify(t, tampered, key), 1, FailureModified)

		tampered = cloneRecords(records)
		tampered[1].PersonalPart = nil
		wantFailure(t, mustVerify(t, tampered, key), 1, FailureModified)
	})

	t.Run("an erasable part added to a record that commits to none", func(t *testing.T) {
		tampered := cloneRecords(records)
		tampered[2].PersonalPart = []byte(`{"actor":"user:someone"}`)
		tampered[2].PersonalSalt = fixtureSalt(0x55)
		wantFailure(t, mustVerify(t, tampered, key), 2, FailureModified)
	})
}

func TestVerify_ReportsARemovedRecord(t *testing.T) {
	records, kp := signedChain(t)
	key := kp.key(t)

	t.Run("from the middle", func(t *testing.T) {
		for removed := 1; removed < len(records)-1; removed++ {
			tampered := append(cloneRecords(records[:removed]), cloneRecords(records[removed+1:])...)
			// The failure is at the record that now sits where the removed
			// one was.
			wantFailure(t, mustVerify(t, tampered, key), removed, FailureRemoved)
		}
	})

	t.Run("the genesis", func(t *testing.T) {
		wantFailure(t, mustVerify(t, cloneRecords(records[1:]), key), 0, FailureRemoved)
	})

	t.Run("two in a row", func(t *testing.T) {
		tampered := append(cloneRecords(records[:1]), cloneRecords(records[3:])...)
		wantFailure(t, mustVerify(t, tampered, key), 1, FailureRemoved)
	})
}

func TestVerify_ReportsAReorderedPair(t *testing.T) {
	records, kp := signedChain(t)
	key := kp.key(t)
	for first := 0; first < len(records)-1; first++ {
		tampered := cloneRecords(records)
		tampered[first], tampered[first+1] = tampered[first+1], tampered[first]
		wantFailure(t, mustVerify(t, tampered, key), first, FailureReordered)
	}

	t.Run("a record moved to the end", func(t *testing.T) {
		tampered := cloneRecords(records)
		moved := tampered[1]
		tampered = append(append(tampered[:1], tampered[2:]...), moved)
		wantFailure(t, mustVerify(t, tampered, key), 1, FailureReordered)
	})

	t.Run("a forged later record does not turn a removal into a reorder", func(t *testing.T) {
		tampered := append(cloneRecords(records[:1]), cloneRecords(records[2:])...)
		forged := cloneRecords(records[1:2])[0]
		flipSignatureByte(t, &forged.Envelope.Signatures[0], 0)
		tampered = append(tampered, forged)
		wantFailure(t, mustVerify(t, tampered, key), 1, FailureRemoved)
	})
}

func TestVerify_ReportsABadSignature(t *testing.T) {
	records, kp := signedChain(t)
	key := kp.key(t)

	t.Run("one byte of the signature changed", func(t *testing.T) {
		for position := range records {
			tampered := cloneRecords(records)
			flipSignatureByte(t, &tampered[position].Envelope.Signatures[0], 7)
			wantFailure(t, mustVerify(t, tampered, key), position, FailureBadSignature)
		}
	})

	t.Run("signed by another key", func(t *testing.T) {
		other := newMemoryKeyProvider(t)
		tampered := cloneRecords(records)
		resigned := signRaw(t, other, ClassRecordV1, payloadBytes(t, tampered[2]))
		tampered[2].Envelope.Signatures = resigned.Envelope.Signatures
		wantFailure(t, mustVerify(t, tampered, key), 2, FailureBadSignature)

		// The other key's signature under the chain key's name.
		tampered[2].Envelope.Signatures[0].KeyID = key.KeyID()
		wantFailure(t, mustVerify(t, tampered, key), 2, FailureBadSignature)
	})

	t.Run("no signature", func(t *testing.T) {
		tampered := cloneRecords(records)
		tampered[1].Envelope.Signatures = nil
		wantFailure(t, mustVerify(t, tampered, key), 1, FailureBadSignature)
	})

	t.Run("a second signature that does not verify", func(t *testing.T) {
		tampered := cloneRecords(records)
		extra := tampered[1].Envelope.Signatures[0]
		flipSignatureByte(t, &extra, 0)
		tampered[1].Envelope.Signatures = append(tampered[1].Envelope.Signatures, extra)
		wantFailure(t, mustVerify(t, tampered, key), 1, FailureBadSignature)
	})

	t.Run("a signature over the bare payload", func(t *testing.T) {
		tampered := cloneRecords(records)
		_, bare, err := kp.SignPayload(context.Background(), ClassCheckpointV1, Payload{canonical: payloadBytes(t, tampered[1])})
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		tampered[1].Envelope.Signatures[0].Sig = base64.StdEncoding.EncodeToString(bare)
		wantFailure(t, mustVerify(t, tampered, key), 1, FailureBadSignature)
	})

	t.Run("a listed algorithm with no signature", func(t *testing.T) {
		tampered := cloneRecords(records)
		last := len(tampered) - 1
		payload := replaceInPayload(t, payloadBytes(t, tampered[last]), `"sig_algs":["Ed25519"]`, `"sig_algs":["Ed25519","ML-DSA-65"]`)
		resigned := signRaw(t, kp, ClassRecordV1, payload)
		resigned.TenantPart, resigned.TenantSalt = tampered[last].TenantPart, tampered[last].TenantSalt
		tampered[last] = resigned
		wantFailure(t, mustVerify(t, tampered, key), last, FailureBadSignature)
	})

	t.Run("a verifier that holds another key", func(t *testing.T) {
		other := newMemoryKeyProvider(t)
		wantFailure(t, mustVerify(t, cloneRecords(records), other.key(t)), 0, FailureBadSignature)
	})
}

func TestVerify_RefusesAnyOtherPayloadTypeBeforeParsing(t *testing.T) {
	records, kp := signedChain(t)
	key := kp.key(t)

	t.Run("a checkpoint envelope offered as a record", func(t *testing.T) {
		// Validly signed as a checkpoint, with a payload that is not JSON: a
		// verifier that parsed first would report a malformed record.
		checkpoint := signRaw(t, kp, ClassCheckpointV1, []byte("not a record"))
		tampered := append(cloneRecords(records[:2]), checkpoint)
		wantFailure(t, mustVerify(t, tampered, key), 2, FailurePayloadType)
	})

	t.Run("a record's bytes signed as a checkpoint", func(t *testing.T) {
		tampered := cloneRecords(records)
		tampered[1] = signRaw(t, kp, ClassCheckpointV1, payloadBytes(t, tampered[1]))
		wantFailure(t, mustVerify(t, tampered, key), 1, FailurePayloadType)
	})

	t.Run("the payload type without a version", func(t *testing.T) {
		tampered := cloneRecords(records)
		tampered[1].Envelope.PayloadType = "application/vnd.opena2a.audit-record+json"
		wantFailure(t, mustVerify(t, tampered, key), 1, FailurePayloadType)
	})

	t.Run("the payload type in another case", func(t *testing.T) {
		tampered := cloneRecords(records)
		tampered[0].Envelope.PayloadType = "Application/vnd.opena2a.audit-record.v1+json"
		wantFailure(t, mustVerify(t, tampered, key), 0, FailurePayloadType)
	})
}

func TestVerify_RefusesASchemaVersionOtherThanThePayloadTypes(t *testing.T) {
	records, kp := signedChain(t)
	last := len(records) - 1
	payload := replaceInPayload(t, payloadBytes(t, records[last]), `"schema":"opena2a-audit-record/1"`, `"schema":"opena2a-audit-record/2"`)
	resigned := signRaw(t, kp, ClassRecordV1, payload)
	resigned.TenantPart, resigned.TenantSalt = records[last].TenantPart, records[last].TenantSalt
	tampered := append(cloneRecords(records[:last]), resigned)
	wantFailure(t, mustVerify(t, tampered, kp.key(t)), last, FailureSchema)
}

func TestVerify_ReportsARecordThatDoesNotContinueTheChain(t *testing.T) {
	records, kp := signedChain(t)
	key := kp.key(t)
	sign := func(t *testing.T, b Body) Record {
		t.Helper()
		rec, err := Sign(context.Background(), b, kp)
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		return rec
	}

	t.Run("a record of another chain", func(t *testing.T) {
		// The same place and the same predecessor hash, under another chain id.
		const otherChain = "ffffffff-5d0c-4c1e-9d55-2f6a3c1b7e10"
		foreign, err := NewRecord(Head{ChainID: otherChain, Seq: 1, Hash: records[1].RecordHash}, fixtureDrafts()[1])
		if err != nil {
			t.Fatalf("NewRecord: %v", err)
		}
		tampered := cloneRecords(records)
		tampered[2] = sign(t, foreign)
		wantFailure(t, mustVerify(t, tampered, key), 2, FailureBrokenLink)
	})

	t.Run("a record repeated", func(t *testing.T) {
		tampered := append(cloneRecords(records[:2]), cloneRecords(records[1:])...)
		wantFailure(t, mustVerify(t, tampered, key), 2, FailureBrokenLink)
	})

	t.Run("a signed record that names another predecessor", func(t *testing.T) {
		fork, err := NewRecord(Head{ChainID: fixtureChainID, Seq: 1, Hash: records[0].RecordHash}, fixtureDrafts()[1])
		if err != nil {
			t.Fatalf("NewRecord: %v", err)
		}
		tampered := cloneRecords(records)
		tampered[2] = sign(t, fork)
		wantFailure(t, mustVerify(t, tampered, key), 2, FailureBrokenLink)
	})
}

func TestVerify_HoldsTheGenesisRule(t *testing.T) {
	records, kp := signedChain(t)
	key := kp.key(t)

	t.Run("a genesis that names a key the verifier does not hold", func(t *testing.T) {
		genesis, err := NewGenesis(fixtureGenesis(keylessKey(), fixturePreGenesis()))
		if err != nil {
			t.Fatalf("NewGenesis: %v", err)
		}
		forged := signRaw(t, kp, ClassRecordV1, genesis.Canonical())
		forged.TenantPart, forged.TenantSalt = records[0].TenantPart, records[0].TenantSalt
		wantFailure(t, mustVerify(t, []Record{forged}, key), 0, FailureGenesis)
	})

	t.Run("a first record that is not a genesis", func(t *testing.T) {
		payload := replaceInPayload(t, payloadBytes(t, records[0]), `"type":"opena2a.chain_genesis"`, `"type":"action"`)
		forged := signRaw(t, kp, ClassRecordV1, payload)
		forged.TenantPart, forged.TenantSalt = records[0].TenantPart, records[0].TenantSalt
		wantFailure(t, mustVerify(t, []Record{forged}, key), 0, FailureGenesis)
	})

	t.Run("a genesis that names a predecessor", func(t *testing.T) {
		payload := replaceInPayload(t, payloadBytes(t, records[0]), `"prev_hash":null`, `"prev_hash":"`+records[1].RecordHash+`"`)
		forged := signRaw(t, kp, ClassRecordV1, payload)
		forged.TenantPart, forged.TenantSalt = records[0].TenantPart, records[0].TenantSalt
		wantFailure(t, mustVerify(t, []Record{forged}, key), 0, FailureGenesis)
	})
}

func TestVerify_ReportsAPayloadThatIsNotARecord(t *testing.T) {
	records, kp := signedChain(t)
	for name, payload := range map[string]string{
		"not JSON":       "not a record",
		"no opena2a":     `{"type":"action"}`,
		"no chain":       `{"opena2a":{"schema":"opena2a-audit-record/1"},"type":"action"}`,
		"fractional seq": `{"opena2a":{"chain":{"id":"` + fixtureChainID + `","prev_hash":null,"seq":1.5},"schema":"opena2a-audit-record/1"},"type":"action"}`,
		"negative seq":   `{"opena2a":{"chain":{"id":"` + fixtureChainID + `","prev_hash":null,"seq":-1},"schema":"opena2a-audit-record/1"},"type":"action"}`,
	} {
		tampered := append(cloneRecords(records[:1]), signRaw(t, kp, ClassRecordV1, []byte(payload)))
		res := mustVerify(t, tampered, kp.key(t))
		if res.Failure == nil || res.Failure.Position != 1 || res.Failure.Kind != FailureMalformed {
			t.Errorf("%s: got %+v; want %s at position 1", name, res.Failure, FailureMalformed)
		}
	}
}

func TestVerify_ErasedPartsLeaveTheChainVerifying(t *testing.T) {
	records, kp := signedChain(t)
	erased := cloneRecords(records)
	for i := range erased {
		erased[i].TenantPart, erased[i].TenantSalt = nil, nil
		erased[i].PersonalPart, erased[i].PersonalSalt = nil, nil
	}
	res := mustVerify(t, erased, kp.key(t))
	if !res.OK() || res.Verified != len(records) {
		t.Fatalf("a chain whose erasable parts are gone fails: %+v", res.Failure)
	}
	if res.Head.Hash != records[len(records)-1].RecordHash {
		t.Fatalf("erasure changed the head")
	}
}

func TestVerify_RefusesAKeyItCannotUse(t *testing.T) {
	records, kp := signedChain(t)
	key := kp.key(t)
	for name, bad := range map[string]PublicKey{
		"no key":            {},
		"short key":         {Alg: AlgEd25519, Key: key.Key[:16]},
		"another algorithm": {Alg: "ES256", Key: key.Key},
	} {
		if _, err := Verify(records, bad); err == nil {
			t.Errorf("%s: Verify accepted the key", name)
		}
	}
}
