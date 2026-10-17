package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests need no backend. They start a local server that answers
// GET /health as the AIM backend does and refuses the admin's sign-in, as a
// development instance with an admin password of its own does, and check when
// the two probes a live test calls before its first request, WaitForBackend
// and ensureAIMBackendRunning, let that test go on.

// aimLookalike answers GET /health with the AIM backend's service name and a
// sign-in with 401, and records each request it receives.
type aimLookalike struct {
	mu    sync.Mutex
	calls []string // "METHOD path", in arrival order
}

func (s *aimLookalike) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.calls = append(s.calls, r.Method+" "+r.URL.Path)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/health":
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "healthy", "service": "agent-identity-management"})
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/public/login":
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid email or password", "success": false})
	default:
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "not found"})
	}
}

// recorded returns a copy of the calls, read under the lock the handler holds.
func (s *aimLookalike) recorded() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.calls...)
}

func startAIMLookalike(t *testing.T) (*aimLookalike, string) {
	t.Helper()
	s := &aimLookalike{}
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	return s, srv.URL
}

// backendProbes are the two probes the live tests in this package call before
// their first request.
var backendProbes = []struct {
	name  string
	probe func(t *testing.T)
}{
	{"WaitForBackend", func(t *testing.T) { _ = NewTestContext(t).WaitForBackend() }},
	{"ensureAIMBackendRunning", ensureAIMBackendRunning},
}

// With neither BACKEND_URL nor TEST_BASE_URL set, a live test is skipped before
// it sends a request, though an AIM backend answers on the address getBaseURL
// falls back to. The live test here does what TestAuthorize_RejectsBadAgentID
// does first: the probe, then the admin sign-in, which that backend refuses.
func TestBackendProbes_SkipWithoutARequestWhenNoAddressIsSet(t *testing.T) {
	for _, p := range backendProbes {
		t.Run(p.name, func(t *testing.T) {
			server, url := startAIMLookalike(t)
			previous := defaultBaseURL
			defaultBaseURL = url
			t.Cleanup(func() { defaultBaseURL = previous })
			t.Setenv("BACKEND_URL", "")
			t.Setenv("TEST_BASE_URL", "")

			var live *testing.T
			t.Run("live test", func(t *testing.T) {
				live = t
				p.probe(t)
				require.NoError(t, NewTestContext(t).LoginAsAdmin())
			})

			require.NotNil(t, live, "the live test ran")
			assert.True(t, live.Skipped(), "the probe skips the live test")
			assert.Empty(t, server.recorded(), "the server on the default address receives no request")
		})
	}
}

// A live test goes on against the address BACKEND_URL names, or TEST_BASE_URL
// when BACKEND_URL is unset, once GET /health there answers as the AIM backend.
func TestBackendProbes_LetTheTestRunAgainstTheAddressTheEnvironmentNames(t *testing.T) {
	for _, p := range backendProbes {
		t.Run(p.name, func(t *testing.T) {
			for _, variable := range []string{"BACKEND_URL", "TEST_BASE_URL"} {
				t.Run(variable, func(t *testing.T) {
					server, url := startAIMLookalike(t)
					t.Setenv("BACKEND_URL", "")
					t.Setenv("TEST_BASE_URL", "")
					t.Setenv(variable, url)

					var live *testing.T
					wentOn := false
					t.Run("live test", func(t *testing.T) {
						live = t
						p.probe(t)
						wentOn = true
					})

					require.NotNil(t, live, "the live test ran")
					assert.False(t, live.Skipped(), "the probe lets the live test go on")
					assert.True(t, wentOn, "the live test went on past the probe")
					assert.Equal(t, []string{"GET /health"}, server.recorded())
				})
			}
		})
	}
}

// With BACKEND_URL set, a live test is skipped when GET /health at that address
// fails, or answers without the AIM backend's service name.
func TestBackendProbes_SkipWhenTheNamedAddressIsNotAnAIMBackend(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL
	closed.Close()

	otherService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "healthy", "service": "another-service"})
	}))
	t.Cleanup(otherService.Close)

	notJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body>not an API</body></html>"))
	}))
	t.Cleanup(notJSON.Close)

	addresses := []struct{ name, url string }{
		{"nothing listens", closedURL},
		{"another service answers", otherService.URL},
		{"the answer is not JSON", notJSON.URL},
	}
	for _, p := range backendProbes {
		t.Run(p.name, func(t *testing.T) {
			for _, a := range addresses {
				t.Run(a.name, func(t *testing.T) {
					t.Setenv("BACKEND_URL", a.url)
					t.Setenv("TEST_BASE_URL", "")

					var live *testing.T
					t.Run("live test", func(t *testing.T) {
						live = t
						p.probe(t)
					})

					require.NotNil(t, live, "the live test ran")
					assert.True(t, live.Skipped(), "the probe skips the live test")
				})
			}
		})
	}
}
