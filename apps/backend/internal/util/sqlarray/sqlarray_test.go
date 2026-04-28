package sqlarray

import "testing"

func TestNonNilStrings_NilBecomesEmpty(t *testing.T) {
	got := NonNilStrings(nil)
	if got == nil {
		t.Fatal("NonNilStrings(nil) returned nil; want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("NonNilStrings(nil) returned len=%d; want 0", len(got))
	}
}

func TestNonNilStrings_EmptyPassesThrough(t *testing.T) {
	in := []string{}
	got := NonNilStrings(in)
	if got == nil {
		t.Fatal("NonNilStrings([]string{}) returned nil; want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("got len=%d; want 0", len(got))
	}
}

func TestNonNilStrings_PopulatedPreserved(t *testing.T) {
	in := []string{"read", "write", "delete"}
	got := NonNilStrings(in)
	if len(got) != len(in) {
		t.Fatalf("len mismatch: got %d want %d", len(got), len(in))
	}
	for i := range in {
		if got[i] != in[i] {
			t.Fatalf("element %d: got %q want %q", i, got[i], in[i])
		}
	}
}

// TestNonNilStrings_AliasesInput documents that NonNilStrings does NOT
// copy the slice — it returns the input directly when non-nil. Callers
// relying on this invariant (e.g. avoiding an extra allocation in hot
// paths) should not be broken by future "defensive copy" refactors.
func TestNonNilStrings_AliasesInput(t *testing.T) {
	in := []string{"a", "b"}
	got := NonNilStrings(in)
	if &in[0] != &got[0] {
		t.Fatal("NonNilStrings copied the slice; expected aliasing of input")
	}
}
