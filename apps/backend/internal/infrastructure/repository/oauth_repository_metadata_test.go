package repository

import (
	"strings"
	"testing"
)

func TestMarshalRegistrationMetadata_EmptyIsNull(t *testing.T) {
	for _, m := range []map[string]interface{}{nil, {}} {
		v, err := marshalRegistrationMetadata(m)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != nil {
			t.Fatalf("expected nil (SQL NULL), got %v", v)
		}
	}
}

func TestMarshalRegistrationMetadata_RoundTrip(t *testing.T) {
	in := map[string]interface{}{
		"signupProfile": map[string]interface{}{
			"role":           "developer",
			"primaryUseCase": "securing-production-agents",
			"referralSource": "github",
		},
	}

	v, err := marshalRegistrationMetadata(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	raw, ok := v.([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", v)
	}

	out, err := unmarshalRegistrationMetadata(raw)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	profile, ok := out["signupProfile"].(map[string]interface{})
	if !ok {
		t.Fatalf("signupProfile missing or wrong type: %v", out)
	}
	if profile["role"] != "developer" ||
		profile["primaryUseCase"] != "securing-production-agents" ||
		profile["referralSource"] != "github" {
		t.Fatalf("round-trip mismatch: %v", profile)
	}
}

func TestMarshalRegistrationMetadata_RejectsOversize(t *testing.T) {
	m := map[string]interface{}{
		"reason": strings.Repeat("a", maxRegistrationMetadataBytes+1),
	}
	if _, err := marshalRegistrationMetadata(m); err == nil {
		t.Fatal("expected oversize metadata to be rejected")
	}
}

func TestUnmarshalRegistrationMetadata_NullColumn(t *testing.T) {
	out, err := unmarshalRegistrationMetadata(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil map for NULL column, got %v", out)
	}
}
