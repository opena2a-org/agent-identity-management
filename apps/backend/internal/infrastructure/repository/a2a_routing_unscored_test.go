package repository

import (
	"strings"
	"testing"
)

// The routing queries once COALESCEd a missing A2A composite to 0.5: an agent
// nobody had measured passed every threshold up to 0.5 and ranked level with
// measured agents. The queries are pinned here without a database: no default
// stands in for a score, a positive threshold is applied to the real column,
// and an unscored agent sorts last.
func TestRoutingQueriesNeverDefaultAnUnscoredAgent(t *testing.T) {
	for name, q := range map[string]string{"searchByIntent": searchByIntentSQL, "countByIntent": countByIntentSQL} {
		if strings.Contains(q, "0.5") {
			t.Errorf("%s: a default score is back in the query", name)
		}
		if !strings.Contains(q, "($2 <= 0 OR t.a2a_trust_score >= $2)") {
			t.Errorf("%s: a positive threshold must be applied to the real score column", name)
		}
	}
	if !strings.Contains(searchByIntentSQL, "t.a2a_trust_score as trust_score") {
		t.Error("searchByIntent must select the real score, null when unscored")
	}
	if !strings.Contains(searchByIntentSQL, "COALESCE(t.a2a_trust_score, 0) DESC") {
		t.Error("searchByIntent must rank an unscored agent last, not as 0.5")
	}
}
