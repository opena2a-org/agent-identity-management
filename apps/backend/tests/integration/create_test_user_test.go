package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests need no backend: they run CreateTestUser and CreateTestManager
// against a local server that stands in for the four routes they call. It
// issues a sign-in token for the role the user holds at sign-in time, as the
// backend's login does when it puts the user's role in the token.

type registrationServer struct {
	mu    sync.Mutex
	calls []string // "METHOD path", in arrival order
	role  string   // the registered user's role; approval makes it "viewer"
}

func (s *registrationServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, r.Method+" "+r.URL.Path)

	var body map[string]interface{}
	_ = json.NewDecoder(r.Body).Decode(&body)
	w.Header().Set("Content-Type", "application/json")

	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/public/login":
		if body["email"] == "admin@opena2a.org" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"accessToken": "admin-session-placeholder"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"accessToken": "session-with-role-" + s.role})
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/public/register":
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"requestId": "request-1"})
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/registration-requests/request-1/approve":
		s.role = "viewer"
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"user": map[string]interface{}{"id": "user-1", "role": s.role}})
	case r.Method == http.MethodPut && r.URL.Path == "/api/v1/admin/users/user-1/role":
		role, _ := body["role"].(string)
		s.role = role
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"user": map[string]interface{}{"id": "user-1", "role": s.role}})
	default:
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "not found"})
	}
}

// recorded returns a copy of the calls, read under the lock the handler holds.
func (s *registrationServer) recorded() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.calls...)
}

func startRegistrationServer(t *testing.T) *registrationServer {
	t.Helper()
	s := &registrationServer{}
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	t.Setenv("BACKEND_URL", srv.URL)
	t.Setenv("TEST_BASE_URL", "")
	return s
}

func TestCreateTestManager_SetsTheManagerRoleBeforeTheUserSignsIn(t *testing.T) {
	s := startRegistrationServer(t)
	tc := NewTestContext(t)

	token, err := tc.CreateTestManager("manager-user@example.com", "TestPass123!")
	require.NoError(t, err)
	assert.Equal(t, "session-with-role-manager", token, "the token is issued after the role is set")

	calls := s.recorded()
	approve := indexOfCall(calls, "POST /api/v1/admin/registration-requests/request-1/approve")
	setRole := indexOfCall(calls, "PUT /api/v1/admin/users/user-1/role")
	userLogin := lastIndexOfCall(calls, "POST /api/v1/public/login")
	require.NotEqual(t, -1, setRole, "calls: %s", strings.Join(calls, ", "))
	assert.Less(t, approve, setRole, "the role is set after the approval")
	assert.Less(t, setRole, userLogin, "the role is set before the user signs in")
}

func TestCreateTestUser_LeavesTheViewerRoleTheApprovalGives(t *testing.T) {
	s := startRegistrationServer(t)
	tc := NewTestContext(t)

	token, err := tc.CreateTestUser("viewer-user@example.com", "TestPass123!")
	require.NoError(t, err)
	assert.Equal(t, "session-with-role-viewer", token)
	calls := s.recorded()
	assert.Equal(t, -1, indexOfCall(calls, "PUT /api/v1/admin/users/user-1/role"), "calls: %s", strings.Join(calls, ", "))
}

func indexOfCall(calls []string, call string) int {
	for i, c := range calls {
		if c == call {
			return i
		}
	}
	return -1
}

func lastIndexOfCall(calls []string, call string) int {
	for i := len(calls) - 1; i >= 0; i-- {
		if calls[i] == call {
			return i
		}
	}
	return -1
}
