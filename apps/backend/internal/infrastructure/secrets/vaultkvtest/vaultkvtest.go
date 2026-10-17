// Package vaultkvtest is an in-memory HashiCorp Vault server for tests that
// drive the github.com/hashicorp/vault/api client. It serves plain HTTP on
// 127.0.0.1 only and holds one KV secrets engine version 2, mounted at Mount
// ("secret", where vault server -dev mounts its own).
//
// Routes and answers follow HashiCorp's published KV secrets engine version 2
// API reference, https://developer.hashicorp.com/vault/api-docs/secret/kv/kv-v2,
// for the calls the AIM backend makes. Each answer below was compared with a
// vault server -dev of Vault 1.21.1, and a test in the parent package repeats
// that comparison where a vault binary is on PATH:
//   - GET /v1/secret/data/<path>: 200 with data.data, the stored map as it was
//     sent, and data.metadata (created_time, custom_metadata null,
//     deletion_time "", destroyed false, version); 404 {"errors":[]} when
//     nothing is stored at the path.
//   - PUT or POST /v1/secret/data/<path> with {"data": {...}}: 200 with the new
//     version's metadata; the version number counts from 1 per path. The body
//     is parsed as JSON, as Vault parses a request without a form Content-Type:
//     a body that is not JSON is 400 {"errors":["error parsing JSON: failed to
//     parse JSON input: <the decoder's message>"]}, a body without a data
//     field, an empty body included, is 400 {"errors":["no data provided"]},
//     a data field that is not an object is 400 with Vault's "error converting
//     input for field" message, which names the kind it got, and a null data
//     field is stored as a version whose data is null.
//   - GET /v1/secret/metadata/<path>: 200 with current_version, created_time,
//     updated_time and a versions map keyed by version number; 404
//     {"errors":[]} when nothing is stored at the path.
//   - DELETE /v1/secret/metadata/<path>: 204, and the path holds nothing
//     afterwards; 204 as well for a path that holds nothing already.
//   - Every request carries X-Vault-Token. No token is 403
//     {"errors":["permission denied"]}; a token other than the server's is 403
//     with Vault's two-line "permission denied" and "invalid token" message.
//   - A path under another mount is 404 {"errors":["no handler for route
//     \"<path>\". route entry not found."]}. A path under the mount that is
//     neither data/<path> nor metadata/<path>, a trailing slash included, is
//     404 with data null and Vault's "Invalid path for a versioned K/V secrets
//     engine" warning.
//
// Answers of 200 carry Vault's envelope: request_id, lease_id "", renewable
// false, lease_duration 0, data, wrap_info null, warnings null, auth null and
// mount_type "kv". The stub's own choices, not compared with Vault: the token
// is checked before the route, a method the stub does not serve on a route
// (PATCH or LIST on data, DELETE on data, PUT on metadata) is 405
// {"errors":["unsupported operation"]}, and a request body over one MiB is
// refused with 400.
package vaultkvtest

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	vault "github.com/hashicorp/vault/api"
)

const (
	// Mount is the path of the stub's KV secrets engine version 2.
	Mount = "secret"

	tokenHeader     = "X-Vault-Token"
	maxRequestBytes = 1 << 20

	// wrongTokenError is the one-element errors list Vault answers for a token
	// it does not know, as a dev server of Vault 1.21.1 words it.
	wrongTokenError = "2 errors occurred:\n\t* permission denied\n\t* invalid token\n\n"

	// invalidPathWarning is the warning Vault answers for a path under a KV
	// version 2 mount that is not one of the engine's endpoints.
	invalidPathWarning = "Invalid path for a versioned K/V secrets engine. See the API docs for the " +
		"appropriate API endpoints to use. If using the Vault CLI, use 'vault kv get' for this operation."
)

// Request is one request the stub routed to the engine, as it arrived: the
// method and the path under /v1/.
type Request struct {
	Method string
	Path   string
}

type version struct {
	data    map[string]interface{}
	created time.Time
}

type entry struct {
	versions []version
}

// Server is the stub. Its URL is the Address a Vault client points at, and
// Token is the one token it accepts.
type Server struct {
	*httptest.Server
	Token string

	mu       sync.Mutex
	entries  map[string]*entry
	requests []Request
}

// NewServer starts a stub on a loopback port, with a token generated for this
// call, and closes it when t ends.
func NewServer(t testing.TB) *Server {
	t.Helper()
	s := &Server{
		Token:   "vaultkvtest-" + randomHex(16),
		entries: make(map[string]*entry),
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.Close)
	return s
}

// NewClient returns a Vault client pointed at s and holding its token. The
// client is built from an explicit configuration, so VAULT_ADDR,
// VAULT_AGENT_ADDR and VAULT_TOKEN in the environment do not redirect it.
func NewClient(t testing.TB, s *Server) *vault.Client {
	t.Helper()
	client, err := vault.NewClient(&vault.Config{
		Address:    s.URL,
		HttpClient: s.Client(),
		Timeout:    10 * time.Second,
	})
	if err != nil {
		t.Fatalf("vaultkvtest: new Vault client: %v", err)
	}
	client.SetToken(s.Token)
	return client
}

// Requests returns the requests the stub routed to the engine, in order. A
// request refused for its token, its mount or its method is not among them.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

// Entry returns the current version stored at path (the part after data/):
// the data map as the client sent it and the version number. ok is false
// when nothing is stored at the path.
func (s *Server) Entry(path string) (data map[string]interface{}, number int, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[path]
	if !ok {
		return nil, 0, false
	}
	v := e.versions[len(e.versions)-1]
	data = make(map[string]interface{}, len(v.data))
	for k, val := range v.data {
		data[k] = val
	}
	return data, len(e.versions), true
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	switch token := r.Header.Get(tokenHeader); {
	case token == "":
		writeErrors(w, http.StatusForbidden, "permission denied")
		return
	case token != s.Token:
		writeErrors(w, http.StatusForbidden, wrongTokenError)
		return
	}

	path, ok := strings.CutPrefix(r.URL.Path, "/v1/")
	if !ok {
		path = strings.TrimPrefix(r.URL.Path, "/")
	}
	mount, rest, _ := strings.Cut(path, "/")
	if !ok || mount != Mount {
		writeErrors(w, http.StatusNotFound, fmt.Sprintf("no handler for route %q. route entry not found.", path))
		return
	}
	kind, key, found := strings.Cut(rest, "/")
	if !found || key == "" || strings.HasSuffix(key, "/") || (kind != "data" && kind != "metadata") {
		writeEnvelope(w, http.StatusNotFound, nil, []string{invalidPathWarning}, "")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case kind == "data" && r.Method == http.MethodGet:
		s.readData(w, key)
	case kind == "data" && (r.Method == http.MethodPut || r.Method == http.MethodPost):
		s.writeData(w, r, key)
	case kind == "metadata" && r.Method == http.MethodGet:
		s.readMetadata(w, key)
	case kind == "metadata" && r.Method == http.MethodDelete:
		s.deleteMetadata(w, key)
	default:
		writeErrors(w, http.StatusMethodNotAllowed, "unsupported operation")
		return
	}
	s.requests = append(s.requests, Request{Method: r.Method, Path: path})
}

func (s *Server) readData(w http.ResponseWriter, key string) {
	e, ok := s.entries[key]
	if !ok {
		writeErrors(w, http.StatusNotFound)
		return
	}
	v := e.versions[len(e.versions)-1]
	writeEnvelope(w, http.StatusOK, map[string]interface{}{
		"data":     v.data,
		"metadata": versionMetadata(v, len(e.versions)),
	}, nil, "kv")
}

func (s *Server) writeData(w http.ResponseWriter, r *http.Request, key string) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBytes))
	if err != nil {
		writeErrors(w, http.StatusBadRequest, "request body could not be read: "+err.Error())
		return
	}
	// Vault decodes the body into a map with json.Decoder.UseNumber, so a
	// number is kept as written; an empty body decodes to no fields.
	var fields map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&fields); err != nil && err != io.EOF {
		writeErrors(w, http.StatusBadRequest, "error parsing JSON: failed to parse JSON input: "+err.Error())
		return
	}
	raw, present := fields["data"]
	if !present {
		writeErrors(w, http.StatusBadRequest, "no data provided")
		return
	}
	var data map[string]interface{}
	switch d := raw.(type) {
	case nil:
	case map[string]interface{}:
		data = d
	default:
		writeErrors(w, http.StatusBadRequest,
			fmt.Sprintf("error converting input for field %q: '' expected a map, got '%s'", "data", reflect.TypeOf(raw).Kind()))
		return
	}
	e := s.entries[key]
	if e == nil {
		e = &entry{}
		s.entries[key] = e
	}
	v := version{data: data, created: time.Now()}
	e.versions = append(e.versions, v)
	writeEnvelope(w, http.StatusOK, versionMetadata(v, len(e.versions)), nil, "kv")
}

func (s *Server) readMetadata(w http.ResponseWriter, key string) {
	e, ok := s.entries[key]
	if !ok {
		writeErrors(w, http.StatusNotFound)
		return
	}
	versions := make(map[string]interface{}, len(e.versions))
	for i, v := range e.versions {
		versions[strconv.Itoa(i+1)] = map[string]interface{}{
			"created_time":  vaultTime(v.created),
			"deletion_time": "",
			"destroyed":     false,
		}
	}
	first, last := e.versions[0], e.versions[len(e.versions)-1]
	writeEnvelope(w, http.StatusOK, map[string]interface{}{
		"cas_required":         false,
		"created_time":         vaultTime(first.created),
		"current_version":      len(e.versions),
		"custom_metadata":      nil,
		"delete_version_after": "0s",
		"max_versions":         0,
		"oldest_version":       0,
		"updated_time":         vaultTime(last.created),
		"versions":             versions,
	}, nil, "kv")
}

func (s *Server) deleteMetadata(w http.ResponseWriter, key string) {
	delete(s.entries, key)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// versionMetadata is the metadata object of one version, as the data route
// answers it for a read and for a write.
func versionMetadata(v version, number int) map[string]interface{} {
	return map[string]interface{}{
		"created_time":    vaultTime(v.created),
		"custom_metadata": nil,
		"deletion_time":   "",
		"destroyed":       false,
		"version":         number,
	}
}

func vaultTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// writeErrors answers {"errors": [...]}; with no message the list is empty,
// as Vault answers a read of a path that holds nothing.
func writeErrors(w http.ResponseWriter, status int, messages ...string) {
	if messages == nil {
		messages = []string{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"errors": messages})
}

func writeEnvelope(w http.ResponseWriter, status int, data interface{}, warnings []string, mountType string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"request_id":     uuid.NewString(),
		"lease_id":       "",
		"renewable":      false,
		"lease_duration": 0,
		"data":           data,
		"wrap_info":      nil,
		"warnings":       warnings,
		"auth":           nil,
		"mount_type":     mountType,
	})
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
