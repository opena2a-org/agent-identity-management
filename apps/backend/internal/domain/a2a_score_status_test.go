package domain

import "testing"

// A2AScoreStatusOf is derived on read, never stored: a nil composite is the
// unscored state and anything else is measured. Both branches are pinned so
// the status can never drift from the presence of the number it describes.
func TestA2AScoreStatusOf(t *testing.T) {
	t.Run("nil score is unscored", func(t *testing.T) {
		if got := A2AScoreStatusOf(nil); got != A2AScoreUnscored {
			t.Fatalf("A2AScoreStatusOf(nil) = %q, want %q", got, A2AScoreUnscored)
		}
	})

	t.Run("any number is measured, including zero", func(t *testing.T) {
		for _, v := range []float64{0, 0.5, 1} {
			score := v
			if got := A2AScoreStatusOf(&score); got != A2AScoreMeasured {
				t.Fatalf("A2AScoreStatusOf(&%v) = %q, want %q", v, got, A2AScoreMeasured)
			}
		}
	})

	t.Run("the two states are distinct wire values", func(t *testing.T) {
		if A2AScoreMeasured == A2AScoreUnscored {
			t.Fatal("measured and unscored must not share a value")
		}
		if A2AScoreMeasured != "measured" || A2AScoreUnscored != "unscored" {
			t.Fatalf("wire values changed: %q / %q", A2AScoreMeasured, A2AScoreUnscored)
		}
	})
}
