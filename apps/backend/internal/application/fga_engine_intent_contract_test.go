package application

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

// TestFGAIntentContract pins the wire contract between the NanoMind daemon's
// /v1/infer response and FGA Step 5's blocking decision.
//
// Per @nanomind/daemon 0.4.0 the daemon emits an explicit `classification`
// field ("classified" | "abstain") alongside `attackClass`. Per
// fga_engine.go::checkIntentSync, the FGA blocking criterion is:
//
//	blocked := classification == "classified" && attackClass != "" && confidence > 0.8
//
// A pre-0.4.0 daemon omits `classification`; the consumer then falls back to the
// legacy heuristic (empty attackClass == abstain). Both the explicit-field and
// the legacy-fallback shapes are exercised below (daemonClassification == ""
// means "omit the field", i.e. an older daemon).
//
// These tests exercise checkIntentSync directly against an httptest server that
// mimics the daemon response shapes. They are deliberately scoped to the
// contract under test and do not depend on the running backend, the database,
// or a real daemon — that is the point.
func TestFGAIntentContract(t *testing.T) {
	cases := []struct {
		name                 string
		daemonAttackClass    string
		daemonConfidence     float64
		daemonClassification string // "" => omit the field (pre-0.4.0 daemon)
		wantBlocked          bool
		wantIntentClass      string
		wantConfidence       float64
		wantStatus           string
	}{
		{
			// Case (a): classifier flags a malicious intent at high confidence
			// with an explicit "classified" — FGA Step 5 must block.
			name:                 "classified non-empty class with high confidence blocks",
			daemonAttackClass:    "exfiltration_pattern",
			daemonConfidence:     0.91,
			daemonClassification: "classified",
			wantBlocked:          true,
			wantIntentClass:      "exfiltration_pattern",
			wantConfidence:       0.91,
			wantStatus:           intentStatusClassified,
		},
		{
			// Case (b): a confident benign — explicit "classified" with an empty
			// attack class. This must NOT be conflated with abstain (the #131
			// fix): status is classified, decision is allow.
			name:                 "classified empty class is a confident benign allow",
			daemonAttackClass:    "",
			daemonConfidence:     0.91,
			daemonClassification: "classified",
			wantBlocked:          false,
			wantIntentClass:      "",
			wantConfidence:       0.91,
			wantStatus:           intentStatusClassified,
		},
		{
			// Case (c): the model could not produce a usable verdict — explicit
			// "abstain" (attackClass forced to "" by the daemon). Allow, but
			// recorded as abstain, not benign.
			name:                 "explicit abstain allows and is recorded as abstain",
			daemonAttackClass:    "",
			daemonConfidence:     0.31,
			daemonClassification: "abstain",
			wantBlocked:          false,
			wantIntentClass:      "",
			wantConfidence:       0.31,
			wantStatus:           intentStatusAbstain,
		},
		{
			// Case (d): classified but confidence below the 0.8 block threshold
			// — FGA Step 5 must allow.
			name:                 "classified non-empty class with low confidence allows",
			daemonAttackClass:    "exfiltration_pattern",
			daemonConfidence:     0.7,
			daemonClassification: "classified",
			wantBlocked:          false,
			wantIntentClass:      "exfiltration_pattern",
			wantConfidence:       0.7,
			wantStatus:           intentStatusClassified,
		},
		{
			// Case (e): legacy daemon (no classification field), non-empty class
			// at high confidence — the fallback heuristic still blocks.
			name:                 "legacy daemon non-empty class blocks via fallback",
			daemonAttackClass:    "prompt_injection",
			daemonConfidence:     0.95,
			daemonClassification: "",
			wantBlocked:          true,
			wantIntentClass:      "prompt_injection",
			wantConfidence:       0.95,
			wantStatus:           intentStatusClassified,
		},
		{
			// Case (f): legacy daemon, empty class — the fallback heuristic
			// records abstain (the pre-#131-Stage-1 behavior, preserved for
			// mixed-version deploys).
			name:                 "legacy daemon empty class is abstain via fallback",
			daemonAttackClass:    "",
			daemonConfidence:     0.95,
			daemonClassification: "",
			wantBlocked:          false,
			wantIntentClass:      "",
			wantConfidence:       0.95,
			wantStatus:           intentStatusAbstain,
		},
		{
			// Case (g): self-contradictory daemon response — "abstain" label but
			// a live high-confidence attack class. Evidence beats label: Step 5
			// fails closed (blocks) rather than dropping the attack. A
			// well-behaved 0.4.0 daemon never sends this; the guard is for a
			// buggy / downgraded / mixed-version daemon.
			name:                 "abstain label with live high-confidence attack fails closed",
			daemonAttackClass:    "exfiltration_pattern",
			daemonConfidence:     0.99,
			daemonClassification: "abstain",
			wantBlocked:          true,
			wantIntentClass:      "exfiltration_pattern",
			wantConfidence:       0.99,
			wantStatus:           intentStatusClassified,
		},
		{
			// Case (h): malformed classification value (unknown string) with a
			// live high-confidence attack class — must fall through to the
			// fallback and still block, never silently allow a real attack.
			name:                 "malformed classification with live attack still blocks",
			daemonAttackClass:    "prompt_injection",
			daemonConfidence:     0.95,
			daemonClassification: "CLASSIFIED-typo",
			wantBlocked:          true,
			wantIntentClass:      "prompt_injection",
			wantConfidence:       0.95,
			wantStatus:           intentStatusClassified,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/infer" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				body := map[string]interface{}{
					"intent":       "INTENT_CHECK",
					"result":       "stub",
					"confidence":   tc.daemonConfidence,
					"attackClass":  tc.daemonAttackClass,
					"latencyMs":    1,
					"modelVersion": "test-stub",
				}
				if tc.daemonClassification != "" {
					body["classification"] = tc.daemonClassification
				}
				_ = json.NewEncoder(w).Encode(body)
			}))
			t.Cleanup(srv.Close)

			engine := NewFGAEngine(nil, nil, nil)
			engine.SetDaemonURL(srv.URL)

			req := &FGARequest{
				AgentID:    uuid.New(),
				Capability: "file:read",
				Resource:   "/tmp/intent-contract-test",
				Action:     "read",
			}

			got := engine.checkIntentSync(context.Background(), req)
			if got == nil {
				t.Fatal("checkIntentSync returned nil; expected an IntentCheckResult")
			}

			if got.Blocked != tc.wantBlocked {
				t.Errorf("Blocked: got %v, want %v", got.Blocked, tc.wantBlocked)
			}
			if got.IntentClass != tc.wantIntentClass {
				t.Errorf("IntentClass: got %q, want %q", got.IntentClass, tc.wantIntentClass)
			}
			if got.Confidence != tc.wantConfidence {
				t.Errorf("Confidence: got %v, want %v", got.Confidence, tc.wantConfidence)
			}
			if got.Status != tc.wantStatus {
				t.Errorf("Status: got %q, want %q", got.Status, tc.wantStatus)
			}
		})
	}
}
