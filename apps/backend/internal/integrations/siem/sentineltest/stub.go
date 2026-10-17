// Package sentineltest is a loopback Microsoft Entra token endpoint and a
// loopback Azure Monitor Logs Ingestion endpoint for tests. Only tests import
// it.
//
// The stubs enforce the request formats Microsoft publishes, read on
// 2026-10-16 from:
//
//	https://learn.microsoft.com/en-us/azure/azure-monitor/logs/logs-ingestion-api-overview
//	https://learn.microsoft.com/en-us/azure/azure-monitor/logs/tutorial-logs-ingestion-code
//	https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-client-creds-grant-flow
//
// They are written from those pages, not from the adapter they test, so a
// request the adapter gets wrong is refused here. Where a page names no
// answer for a case, the answer is the stub's own and says so.
package sentineltest

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
)

// scopeSuffix ends the scope of a client credentials request: the resource
// with /.default.
const scopeSuffix = ".default"

// MonitorAudience is the audience of a Logs Ingestion API token.
const MonitorAudience = "https://monitor.azure.com/"

// APIVersion is the api-version of an upload request.
const APIVersion = "2023-01-01"

// ExpiresIn is the lifetime, in seconds, the token stub gives each token.
const ExpiresIn = 3599

// maxBody bounds the request body a stub reads.
const maxBody = 8 << 20

// Refusal is one request a stub did not accept: its HTTP status and, for the
// token stub, the error code it answered.
type Refusal struct {
	Status int
	Error  string
}

// ============================================================================
// Token endpoint
// ============================================================================

// TokenServer is a loopback Microsoft Entra token endpoint for one tenant and
// one application. It issues a bearer token to a client credentials request
// that carries the application's client id and secret and a scope for
// https://monitor.azure.com.
type TokenServer struct {
	server       *httptest.Server
	tenant       string
	clientID     string
	clientSecret string

	mu       sync.Mutex
	requests int
	issued   map[string]bool
	refusals []Refusal
}

// NewTokenServer starts a token endpoint on loopback. Close stops it.
func NewTokenServer(tenant, clientID, clientSecret string) *TokenServer {
	s := &TokenServer{
		tenant:       tenant,
		clientID:     clientID,
		clientSecret: clientSecret,
		issued:       map[string]bool{},
	}
	s.server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

// AuthorityHost is the stub's base URL: the token endpoint is
// AuthorityHost/<tenant>/oauth2/v2.0/token.
func (s *TokenServer) AuthorityHost() string { return s.server.URL }

// Close stops the stub.
func (s *TokenServer) Close() { s.server.Close() }

// Requests is the number of requests the stub has received, accepted or not.
func (s *TokenServer) Requests() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests
}

// Refusals returns the requests refused so far, in arrival order.
func (s *TokenServer) Refusals() []Refusal {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Refusal(nil), s.refusals...)
}

// Valid reports whether token is one the stub issued and has not revoked.
func (s *TokenServer) Valid(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.issued[token]
}

// RevokeAll makes every token issued so far invalid, as an expired or
// revoked token is.
func (s *TokenServer) RevokeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.issued = map[string]bool{}
}

func (s *TokenServer) serve(w http.ResponseWriter, req *http.Request) {
	s.mu.Lock()
	s.requests++
	s.mu.Unlock()

	// "POST /{tenant}/oauth2/v2.0/token". A wrong path or method is answered
	// by the stub's own 404 or 405.
	if req.URL.Path != "/"+s.tenant+"/oauth2/v2.0/token" {
		s.refuse(w, http.StatusNotFound, "not_found", "the stub serves only /"+s.tenant+"/oauth2/v2.0/token")
		return
	}
	if req.Method != http.MethodPost {
		s.refuse(w, http.StatusMethodNotAllowed, "method_not_allowed", "the token endpoint takes POST")
		return
	}
	// "Content-Type: application/x-www-form-urlencoded". The page names
	// error codes, not the status of each; the 400 answers below are the
	// stub's own, and only the 401 for the client is quoted.
	if mediaType(req.Header.Get("Content-Type")) != "application/x-www-form-urlencoded" {
		s.refuse(w, http.StatusBadRequest, "invalid_request", "the body must be application/x-www-form-urlencoded")
		return
	}
	req.Body = http.MaxBytesReader(w, req.Body, maxBody)
	if err := req.ParseForm(); err != nil {
		s.refuse(w, http.StatusBadRequest, "invalid_request", "the body does not parse as a form")
		return
	}
	form := req.PostForm
	if form.Get("grant_type") != "client_credentials" {
		s.refuse(w, http.StatusBadRequest, "unsupported_grant_type", "grant_type must be client_credentials")
		return
	}
	scope := form.Get("scope")
	if !strings.HasPrefix(scope, MonitorAudience) || !strings.HasSuffix(scope, scopeSuffix) {
		s.refuse(w, http.StatusBadRequest, "invalid_scope", "scope must be https://monitor.azure.com/.default")
		return
	}
	if form.Get("client_id") != s.clientID || form.Get("client_secret") != s.clientSecret {
		s.refuse(w, http.StatusUnauthorized, "invalid_client", "The client id or secret is not valid.")
		return
	}

	token := randomToken()
	s.mu.Lock()
	s.issued[token] = true
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"token_type":   "Bearer",
		"expires_in":   ExpiresIn,
		"access_token": token,
	})
}

// refuse records a refusal and sends {"error":...,"error_description":...}.
func (s *TokenServer) refuse(w http.ResponseWriter, status int, code, description string) {
	s.mu.Lock()
	s.refusals = append(s.refusals, Refusal{Status: status, Error: code})
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "error_description": description})
}

func randomToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// ============================================================================
// Logs Ingestion endpoint
// ============================================================================

// Record is one record the ingestion stub accepted: its columns by name.
type Record map[string]json.RawMessage

// String returns the string value of column name, and false when the record
// has no such column or its value is not a string.
func (r Record) String(name string) (string, bool) {
	raw, ok := r[name]
	if !ok {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}

// IngestionServer is a loopback Logs Ingestion endpoint for one data
// collection rule and stream. It accepts an upload that carries a bearer
// token the TokenServer issued and a JSON array of records whose every
// column name the page allows.
type IngestionServer struct {
	server *httptest.Server
	dcr    string
	stream string
	tokens *TokenServer

	mu       sync.Mutex
	requests int
	accepted []Record
	refusals []Refusal
}

// NewIngestionServer starts an ingestion endpoint on loopback for the rule's
// immutable id dcr and the stream, accepting the tokens that tokens issues.
// Close stops it.
func NewIngestionServer(dcr, stream string, tokens *TokenServer) *IngestionServer {
	s := &IngestionServer{dcr: dcr, stream: stream, tokens: tokens}
	s.server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

// Endpoint is the stub's base URL: uploads go to
// Endpoint/dataCollectionRules/<dcr>/streams/<stream>.
func (s *IngestionServer) Endpoint() string { return s.server.URL }

// Close stops the stub.
func (s *IngestionServer) Close() { s.server.Close() }

// Requests is the number of requests the stub has received, accepted or not.
func (s *IngestionServer) Requests() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests
}

// Accepted returns the records accepted so far, in arrival order.
func (s *IngestionServer) Accepted() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Record(nil), s.accepted...)
}

// Refusals returns the requests refused so far, in arrival order.
func (s *IngestionServer) Refusals() []Refusal {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Refusal(nil), s.refusals...)
}

// columnName is the page's rule for a column name: it starts with a letter
// and has up to 45 alphanumerics or underscores.
var columnName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,44}$`)

// reservedColumns are reserved names the page lists. The page lists more;
// these three are the ones read for the stub. They are compared without
// regard to case, which refuses more than the page may.
var reservedColumns = []string{"id", "TenantId", "Type"}

func (s *IngestionServer) serve(w http.ResponseWriter, req *http.Request) {
	s.mu.Lock()
	s.requests++
	s.mu.Unlock()

	// "POST {endpoint}/dataCollectionRules/{dcrImmutableId}/streams/{streamName}".
	// A wrong path or method is answered by the stub's own 404 or 405.
	if req.URL.Path != "/dataCollectionRules/"+s.dcr+"/streams/"+s.stream {
		s.refuse(w, http.StatusNotFound)
		return
	}
	if req.Method != http.MethodPost {
		s.refuse(w, http.StatusMethodNotAllowed)
		return
	}
	// "Authorization: Bearer <token>". The success status is not verified
	// from the pages; the stub answers 204 to an accepted upload, 401 to a
	// missing or wrong bearer and 400 to any other malformed request.
	authorization := req.Header.Get("Authorization")
	token, isBearer := strings.CutPrefix(authorization, "Bearer ")
	if !isBearer || token == "" || !s.tokens.Valid(token) {
		s.refuse(w, http.StatusUnauthorized)
		return
	}
	if req.URL.Query().Get("api-version") != APIVersion {
		s.refuse(w, http.StatusBadRequest)
		return
	}
	if mediaType(req.Header.Get("Content-Type")) != "application/json" {
		s.refuse(w, http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, maxBody))
	if err != nil {
		s.refuse(w, http.StatusBadRequest)
		return
	}
	records, ok := parseRecords(body)
	if !ok {
		s.refuse(w, http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.accepted = append(s.accepted, records...)
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *IngestionServer) refuse(w http.ResponseWriter, status int) {
	s.mu.Lock()
	s.refusals = append(s.refusals, Refusal{Status: status})
	s.mu.Unlock()
	w.WriteHeader(status)
}

// parseRecords reads a body that is one JSON array of objects, each of whose
// keys is a column name the page allows. An upload is accepted or refused
// whole.
func parseRecords(body []byte) ([]Record, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	var objects []json.RawMessage
	if err := dec.Decode(&objects); err != nil {
		return nil, false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, false
	}
	if len(objects) == 0 {
		return nil, false
	}
	records := make([]Record, 0, len(objects))
	for _, object := range objects {
		var record Record
		if err := json.Unmarshal(object, &record); err != nil || record == nil {
			return nil, false
		}
		for name := range record {
			if !AllowedColumn(name) {
				return nil, false
			}
		}
		records = append(records, record)
	}
	return records, true
}

// AllowedColumn reports whether name obeys the page's column name rule and is
// not one of the reserved names the stub knows.
func AllowedColumn(name string) bool {
	if !columnName.MatchString(name) {
		return false
	}
	for _, reserved := range reservedColumns {
		if strings.EqualFold(name, reserved) {
			return false
		}
	}
	return true
}

func mediaType(contentType string) string {
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	return mt
}
