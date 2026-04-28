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
// Per @nanomind/daemon 0.1.1 (CHANGELOG), the daemon always emits
// attackClass as a string (default ""). Per fga_engine.go::checkIntentSync,
// the FGA blocking criterion is:
//
//	blocked := attackClass != "" && confidence > 0.8
//
// These tests exercise checkIntentSync directly against an httptest server
// that mimics three daemon response shapes. They are deliberately scoped to
// the contract under test and do not depend on the running backend, the
// database, or a real daemon — that is the point.
func TestFGAIntentContract(t *testing.T) {
	cases := []struct {
		name              string
		daemonAttackClass string
		daemonConfidence  float64
		wantBlocked       bool
		wantIntentClass   string
		wantConfidence    float64
	}{
		{
			// Case (a): production classifier flags a malicious intent at
			// high confidence — FGA Step 5 must block.
			name:              "non-empty class with high confidence blocks",
			daemonAttackClass: "exfiltration_pattern",
			daemonConfidence:  0.91,
			wantBlocked:       true,
			wantIntentClass:   "exfiltration_pattern",
			wantConfidence:    0.91,
		},
		{
			// Case (b): the always-emitted default. This is what every
			// response carries today (until Bug 2 wires the production
			// classifier). FGA Step 5 must allow.
			name:              "empty class with high confidence allows",
			daemonAttackClass: "",
			daemonConfidence:  0.91,
			wantBlocked:       false,
			wantIntentClass:   "",
			wantConfidence:    0.91,
		},
		{
			// Case (c): classifier flagged something but confidence is below
			// the 0.8 threshold — FGA Step 5 must allow.
			name:              "non-empty class with low confidence allows",
			daemonAttackClass: "exfiltration_pattern",
			daemonConfidence:  0.7,
			wantBlocked:       false,
			wantIntentClass:   "exfiltration_pattern",
			wantConfidence:    0.7,
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
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"intent":       "INTENT_CHECK",
					"result":       "stub",
					"confidence":   tc.daemonConfidence,
					"attackClass":  tc.daemonAttackClass,
					"latencyMs":    1,
					"modelVersion": "test-stub",
				})
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
		})
	}
}
