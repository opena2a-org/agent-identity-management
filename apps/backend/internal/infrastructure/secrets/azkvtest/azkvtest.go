// Package azkvtest is an in-memory Azure Key Vault secrets endpoint for tests
// that drive the azsecrets client library. It serves HTTPS on 127.0.0.1 only.
//
// Request and response shapes follow Microsoft's published REST reference:
//   - Set Secret: https://learn.microsoft.com/en-us/rest/api/keyvault/secrets/set-secret/set-secret
//     PUT {vaultBaseUrl}/secrets/{secret-name}?api-version=..., secret-name
//     matching ^[0-9a-zA-Z-]+$, a JSON body whose "value" is required, and a
//     200 SecretBundle answer. A secret that already exists gets a new version.
//   - Get Secret: https://learn.microsoft.com/en-us/rest/api/keyvault/secrets/get-secret/get-secret
//     GET {vaultBaseUrl}/secrets/{secret-name}/{secret-version}; the version
//     fragment is optional and its absence means the latest version.
//   - Delete Secret: https://learn.microsoft.com/en-us/rest/api/keyvault/secrets/delete-secret/delete-secret
//     DELETE {vaultBaseUrl}/secrets/{secret-name}, answered 200 with a
//     DeletedSecretBundle (recoveryId, deletedDate, scheduledPurgeDate).
//   - Authentication, requests and responses:
//     https://learn.microsoft.com/en-us/azure/key-vault/general/authentication-requests-and-responses
//     Every request carries an api-version query parameter (there is no
//     default), a request body is application/json, Accept names
//     application/json, and a request without an accepted
//     "Authorization: Bearer <token>" is answered 401 with
//     `WWW-Authenticate: Bearer authorization="...", resource="..."`.
//     Errors use the KeyVaultError envelope {"error":{"code","message"}}.
//
// The 404 answer for a missing or deleted secret matches the azsecrets
// library's own live tests, which poll for 404 after DeleteSecret. The error
// code strings (Unauthorized, BadParameter, NotFound, SecretNotFound) are not on the
// pages above and are not verified against the service. The soft-delete rule
// that a deleted name cannot be reused until it is purged is not modelled.
package azkvtest

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// Resource is the resource the 401 challenge names, as on the authentication page.
const Resource = "https://vault.azure.net"

// AuthorityHost is the authority host the 401 challenge names.
const AuthorityHost = "https://login.microsoftonline.com"

var secretNamePattern = regexp.MustCompile(`^[0-9a-zA-Z-]+$`)

// Request is one request the stub received, recorded after it was answered.
type Request struct {
	Method string
	Path   string
	// APIVersion is the api-version query parameter as sent.
	APIVersion string
	// HadAuthorization reports whether an Authorization header was present.
	HadAuthorization bool
	// Authorized reports whether the header carried the stub's bearer token.
	Authorized bool
	// BodyBytes is the length of the request body the stub read.
	BodyBytes int
	Status    int
}

type version struct {
	id      string
	value   string
	created int64
}

type secret struct {
	versions []version
}

// Server is the stub vault. Use URL as the vault URL and Client as the
// transport that trusts its certificate.
type Server struct {
	// URL is the vault base URL, https://127.0.0.1:<port>.
	URL string
	// TenantID is the tenant the 401 challenge names in its authorization URL.
	TenantID string

	srv   *httptest.Server
	token string

	mu       sync.Mutex
	secrets  map[string]*secret
	requests []Request
}

// New starts a stub vault that accepts only the given bearer token. It is
// closed when the test ends.
func New(t testing.TB, token string) *Server {
	t.Helper()
	s := &Server{
		TenantID: randomHex(16),
		token:    token,
		secrets:  make(map[string]*secret),
	}
	s.srv = httptest.NewUnstartedServer(http.HandlerFunc(s.serve))
	// A client that does not trust the certificate makes the server log a
	// handshake error; that is an expected outcome of a test, not noise to print.
	s.srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	s.srv.StartTLS()
	s.URL = s.srv.URL
	t.Cleanup(s.srv.Close)
	return s
}

// Client returns an HTTP client that trusts the stub's certificate.
func (s *Server) Client() *http.Client {
	return s.srv.Client()
}

// Requests returns a copy of the requests received so far.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Request, len(s.requests))
	copy(out, s.requests)
	return out
}

// NewToken returns a bearer token built at runtime for one test.
func NewToken() string {
	return "azkvtest-" + randomHex(24)
}

// Challenge is the WWW-Authenticate value of a 401 answer.
func (s *Server) Challenge() string {
	return fmt.Sprintf(`Bearer authorization="%s/%s", resource="%s"`, AuthorityHost, s.TenantID, Resource)
}

type secretAttributes struct {
	Enabled       bool   `json:"enabled"`
	Created       int64  `json:"created"`
	Updated       int64  `json:"updated"`
	RecoveryLevel string `json:"recoveryLevel"`
}

type secretBundle struct {
	Value      string           `json:"value"`
	ID         string           `json:"id"`
	Attributes secretAttributes `json:"attributes"`
}

type deletedSecretBundle struct {
	RecoveryID         string           `json:"recoveryId"`
	DeletedDate        int64            `json:"deletedDate"`
	ScheduledPurgeDate int64            `json:"scheduledPurgeDate"`
	ID                 string           `json:"id"`
	Attributes         secretAttributes `json:"attributes"`
}

type keyVaultError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Recovery level of a soft-delete vault with purge allowed, as in the page samples.
const recoveryLevel = "Recoverable+Purgeable"

// Soft-delete retention interval the DeletionRecoveryLevel definitions name.
const retention = 90 * 24 * time.Hour

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	rec := Request{
		Method:           r.Method,
		Path:             r.URL.Path,
		APIVersion:       r.URL.Query().Get("api-version"),
		HadAuthorization: r.Header.Get("Authorization") != "",
		Authorized:       r.Header.Get("Authorization") == "Bearer "+s.token,
	}
	status := s.handle(w, r, &rec)
	rec.Status = status
	s.mu.Lock()
	s.requests = append(s.requests, rec)
	s.mu.Unlock()
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request, rec *Request) int {
	if !rec.Authorized {
		w.Header().Set("WWW-Authenticate", s.Challenge())
		return writeError(w, http.StatusUnauthorized, "Unauthorized", "Request is missing a Bearer token or the token is not accepted.")
	}
	if rec.APIVersion == "" {
		return writeError(w, http.StatusBadRequest, "BadParameter", "The api-version query parameter is required.")
	}
	if !strings.Contains(r.Header.Get("Accept"), "application/json") {
		return writeError(w, http.StatusBadRequest, "BadParameter", "The Accept header must contain application/json.")
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 || len(parts) > 3 || parts[0] != "secrets" || parts[1] == "" {
		return writeError(w, http.StatusNotFound, "NotFound", "Unknown path "+r.URL.Path)
	}
	name := parts[1]
	versionID := ""
	if len(parts) == 3 {
		versionID = parts[2]
	}

	switch {
	case r.Method == http.MethodPut && len(parts) == 2:
		return s.setSecret(w, r, rec, name)
	case r.Method == http.MethodGet:
		return s.getSecret(w, name, versionID)
	case r.Method == http.MethodDelete && len(parts) == 2:
		return s.deleteSecret(w, name)
	default:
		return writeError(w, http.StatusMethodNotAllowed, "BadParameter", r.Method+" is not supported on "+r.URL.Path)
	}
}

func (s *Server) setSecret(w http.ResponseWriter, r *http.Request, rec *Request, name string) int {
	if !secretNamePattern.MatchString(name) {
		return writeError(w, http.StatusBadRequest, "BadParameter", "The secret name must match ^[0-9a-zA-Z-]+$.")
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return writeError(w, http.StatusBadRequest, "BadParameter", "The request body content type must be application/json.")
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return writeError(w, http.StatusBadRequest, "BadParameter", "The request body could not be read.")
	}
	rec.BodyBytes = len(body)
	var params struct {
		Value *string `json:"value"`
	}
	if err := json.Unmarshal(body, &params); err != nil {
		return writeError(w, http.StatusBadRequest, "BadParameter", "The request body is not a JSON object.")
	}
	if params.Value == nil {
		return writeError(w, http.StatusBadRequest, "BadParameter", "The value field is required.")
	}

	now := time.Now().Unix()
	v := version{id: randomHex(16), value: *params.Value, created: now}
	s.mu.Lock()
	sec, ok := s.secrets[name]
	if !ok {
		sec = &secret{}
		s.secrets[name] = sec
	}
	sec.versions = append(sec.versions, v)
	s.mu.Unlock()

	return writeJSON(w, http.StatusOK, s.bundle(name, v))
}

func (s *Server) getSecret(w http.ResponseWriter, name, versionID string) int {
	s.mu.Lock()
	sec, ok := s.secrets[name]
	var found *version
	if ok && len(sec.versions) > 0 {
		if versionID == "" {
			v := sec.versions[len(sec.versions)-1]
			found = &v
		} else {
			for i := range sec.versions {
				if sec.versions[i].id == versionID {
					v := sec.versions[i]
					found = &v
					break
				}
			}
		}
	}
	s.mu.Unlock()
	if found == nil {
		return writeError(w, http.StatusNotFound, "SecretNotFound",
			fmt.Sprintf("A secret with (name/id) %s was not found in this key vault.", name))
	}
	return writeJSON(w, http.StatusOK, s.bundle(name, *found))
}

func (s *Server) deleteSecret(w http.ResponseWriter, name string) int {
	s.mu.Lock()
	sec, ok := s.secrets[name]
	if ok {
		delete(s.secrets, name)
	}
	s.mu.Unlock()
	if !ok || len(sec.versions) == 0 {
		return writeError(w, http.StatusNotFound, "SecretNotFound",
			fmt.Sprintf("A secret with (name/id) %s was not found in this key vault.", name))
	}
	latest := sec.versions[len(sec.versions)-1]
	deleted := time.Now()
	b := s.bundle(name, latest)
	return writeJSON(w, http.StatusOK, deletedSecretBundle{
		RecoveryID:         s.URL + "/deletedsecrets/" + name,
		DeletedDate:        deleted.Unix(),
		ScheduledPurgeDate: deleted.Add(retention).Unix(),
		ID:                 b.ID,
		Attributes:         b.Attributes,
	})
}

func (s *Server) bundle(name string, v version) secretBundle {
	return secretBundle{
		Value: v.value,
		ID:    s.URL + "/secrets/" + name + "/" + v.id,
		Attributes: secretAttributes{
			Enabled:       true,
			Created:       v.created,
			Updated:       v.created,
			RecoveryLevel: recoveryLevel,
		},
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) int {
	var e keyVaultError
	e.Error.Code = code
	e.Error.Message = message
	return writeJSON(w, status, e)
}

func writeJSON(w http.ResponseWriter, status int, v any) int {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
	return status
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("azkvtest: crypto/rand: %v", err))
	}
	return hex.EncodeToString(b)
}
