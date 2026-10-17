// Package onepasswordtest is an in-memory 1Password Connect server for tests
// that drive the github.com/1Password/connect-sdk-go client library. It serves
// plain HTTP on 127.0.0.1 only and holds one vault.
//
// Routes and answers follow 1Password's published Connect API reference,
// https://developer.1password.com/docs/connect/connect-api-reference/ (now
// served at https://www.1password.dev/connect/connect-api-reference/), for the
// vault item calls the AIM backend makes:
//   - GET /v1/vaults/{vaultUUID}/items, filtered by `title eq "..."` or
//     `tag eq "..."`: 200 with an array of Item objects that do not include
//     sections and fields; 404 "Vault not found".
//   - GET /v1/vaults/{vaultUUID}/items/{itemUUID}: 200 with the Item; 404
//     "Item not found".
//   - POST /v1/vaults/{vaultUUID}/items: 200 with the new Item; 400 "Unable to
//     create item due to invalid input". Items cannot be created in the CUSTOM
//     or DOCUMENT categories.
//   - PUT /v1/vaults/{vaultUUID}/items/{itemUUID}: 200 with the Item; 400 for
//     invalid input; 404 "Item not found".
//   - DELETE /v1/vaults/{vaultUUID}/items/{itemUUID}: 204; 404 "Item not found".
//   - Every request carries "Authorization: Bearer <access_token>"; a missing
//     or invalid token is answered 401 "Invalid or missing token".
//   - Errors are ErrorResponse objects, {"status": <code>, "message": "..."}.
//
// Item IDs are 26 lowercase letters and digits, as in the page's examples and
// as the client library requires. The stub answers 403 "Unauthorized access"
// for an item route on a vault other than its own, and 400 for a list filter
// that is neither form above; those two choices are the stub's own.
package onepasswordtest

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

const idAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

var filterPattern = regexp.MustCompile(`^(title|tag) eq "(.*)"$`)

// Categories an item may be created in. The page also lists CUSTOM and
// DOCUMENT, which cannot be used to create an item.
var creatableCategories = map[string]bool{
	"LOGIN": true, "PASSWORD": true, "API_CREDENTIAL": true, "SERVER": true,
	"DATABASE": true, "CREDIT_CARD": true, "MEMBERSHIP": true, "PASSPORT": true,
	"SOFTWARE_LICENSE": true, "OUTDOOR_LICENSE": true, "SECURE_NOTE": true,
	"WIRELESS_ROUTER": true, "BANK_ACCOUNT": true, "DRIVER_LICENSE": true,
	"IDENTITY": true, "REWARD_PROGRAM": true, "EMAIL_ACCOUNT": true,
	"SOCIAL_SECURITY_NUMBER": true, "MEDICAL_RECORD": true, "SSH_KEY": true,
}

// Field types and purposes the page lists.
var (
	fieldTypes    = map[string]bool{"STRING": true, "EMAIL": true, "CONCEALED": true, "URL": true, "OTP": true, "DATE": true, "MONTH_YEAR": true, "MENU": true}
	fieldPurposes = map[string]bool{"USERNAME": true, "PASSWORD": true, "NOTES": true}
)

// Request is one request the stub received, recorded after it was answered.
type Request struct {
	Method string
	Path   string
	// Filter is the filter query parameter, decoded.
	Filter string
	// HadAuthorization reports whether an Authorization header was present.
	HadAuthorization bool
	// Authorized reports whether it carried the stub's bearer token.
	Authorized bool
	Status     int
}

type itemVault struct {
	ID string `json:"id"`
}

type itemField struct {
	ID      string          `json:"id"`
	Section json.RawMessage `json:"section,omitempty"`
	Type    string          `json:"type,omitempty"`
	Purpose string          `json:"purpose,omitempty"`
	Label   string          `json:"label,omitempty"`
	Value   string          `json:"value,omitempty"`
}

type item struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Vault     itemVault       `json:"vault"`
	Category  string          `json:"category"`
	Tags      []string        `json:"tags,omitempty"`
	Version   int             `json:"version"`
	Sections  json.RawMessage `json:"sections,omitempty"`
	Fields    []itemField     `json:"fields,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type errorResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

// Server is the stub Connect server.
type Server struct {
	// URL is the Connect host, http://127.0.0.1:<port>.
	URL string
	// VaultID is the UUID of the one vault the stub holds.
	VaultID string

	token string

	mu       sync.Mutex
	items    map[string]*item
	order    []string
	requests []Request
}

// New starts a stub Connect server that accepts only the given bearer token.
// It is closed when the test ends.
func New(t testing.TB, token string) *Server {
	t.Helper()
	s := &Server{VaultID: NewID(), token: token, items: make(map[string]*item)}
	srv := httptest.NewServer(http.HandlerFunc(s.serve))
	s.URL = srv.URL
	t.Cleanup(srv.Close)
	return s
}

// NewID returns a random 26-character item or vault UUID.
func NewID() string {
	b := make([]byte, 26)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("onepasswordtest: crypto/rand: %v", err))
	}
	for i := range b {
		b[i] = idAlphabet[int(b[i])%len(idAlphabet)]
	}
	return string(b)
}

// NewToken returns a bearer token built at runtime for one test.
func NewToken() string {
	return "onepasswordtest-" + NewID() + NewID()
}

// Requests returns a copy of the requests received so far.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Request, len(s.requests))
	copy(out, s.requests)
	return out
}

// FieldValues returns label -> value for the stored item with the given
// title, or nil when there is none.
func (s *Server) FieldValues(title string) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range s.order {
		it := s.items[id]
		if it.Title != title {
			continue
		}
		out := make(map[string]string, len(it.Fields))
		for _, f := range it.Fields {
			out[f.Label] = f.Value
		}
		return out
	}
	return nil
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	rec := Request{
		Method:           r.Method,
		Path:             r.URL.Path,
		Filter:           r.URL.Query().Get("filter"),
		HadAuthorization: r.Header.Get("Authorization") != "",
		Authorized:       r.Header.Get("Authorization") == "Bearer "+s.token,
	}
	rec.Status = s.handle(w, r, rec)
	s.mu.Lock()
	s.requests = append(s.requests, rec)
	s.mu.Unlock()
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request, rec Request) int {
	if !rec.Authorized {
		return writeError(w, http.StatusUnauthorized, "Invalid or missing token")
	}
	// /v1/vaults/{vaultUUID}/items[/{itemUUID}]
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 || len(parts) > 5 || parts[0] != "v1" || parts[1] != "vaults" || parts[3] != "items" {
		return writeError(w, http.StatusNotFound, "Not found")
	}
	vaultID := parts[2]

	if len(parts) == 4 {
		if vaultID != s.VaultID {
			return writeError(w, http.StatusNotFound, "Vault not found")
		}
		switch r.Method {
		case http.MethodGet:
			return s.listItems(w, rec.Filter)
		case http.MethodPost:
			return s.createItem(w, r)
		}
		return writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}

	if vaultID != s.VaultID {
		return writeError(w, http.StatusForbidden, "Unauthorized access")
	}
	itemID := parts[4]
	switch r.Method {
	case http.MethodGet:
		return s.getItem(w, itemID)
	case http.MethodPut:
		return s.replaceItem(w, r, itemID)
	case http.MethodDelete:
		return s.deleteItem(w, itemID)
	}
	return writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
}

func (s *Server) listItems(w http.ResponseWriter, filter string) int {
	var field, want string
	if filter != "" {
		m := filterPattern.FindStringSubmatch(filter)
		if m == nil {
			return writeError(w, http.StatusBadRequest, "Invalid filter: items can only be filtered by title or tag")
		}
		field, want = m[1], m[2]
	}
	s.mu.Lock()
	out := []item{}
	for _, id := range s.order {
		it := *s.items[id]
		switch field {
		case "title":
			if it.Title != want {
				continue
			}
		case "tag":
			if !contains(it.Tags, want) {
				continue
			}
		}
		// List answers do not include sections and fields.
		it.Sections, it.Fields = nil, nil
		out = append(out, it)
	}
	s.mu.Unlock()
	return writeJSON(w, http.StatusOK, out)
}

func (s *Server) getItem(w http.ResponseWriter, itemID string) int {
	s.mu.Lock()
	it, ok := s.items[itemID]
	var cp item
	if ok {
		cp = *it
	}
	s.mu.Unlock()
	if !ok {
		return writeError(w, http.StatusNotFound, "Item not found")
	}
	return writeJSON(w, http.StatusOK, cp)
}

func (s *Server) createItem(w http.ResponseWriter, r *http.Request) int {
	in, msg := s.decodeItem(r)
	if msg != "" {
		return writeError(w, http.StatusBadRequest, "Unable to create item due to invalid input: "+msg)
	}
	now := time.Now().UTC()
	in.ID = NewID()
	in.Vault = itemVault{ID: s.VaultID}
	in.Version = 1
	in.CreatedAt, in.UpdatedAt = now, now
	assignFieldIDs(in)

	s.mu.Lock()
	s.items[in.ID] = in
	s.order = append(s.order, in.ID)
	cp := *in
	s.mu.Unlock()
	return writeJSON(w, http.StatusOK, cp)
}

func (s *Server) replaceItem(w http.ResponseWriter, r *http.Request, itemID string) int {
	in, msg := s.decodeItem(r)
	if msg == "" && in.ID != "" && in.ID != itemID {
		msg = "item id in the body does not match the path"
	}
	if msg != "" {
		return writeError(w, http.StatusBadRequest, "Unable to create item due to invalid input: "+msg)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.items[itemID]
	if !ok {
		return writeError(w, http.StatusNotFound, "Item not found")
	}
	in.ID = itemID
	in.Vault = itemVault{ID: s.VaultID}
	in.Version = old.Version + 1
	in.CreatedAt = old.CreatedAt
	in.UpdatedAt = time.Now().UTC()
	assignFieldIDs(in)
	s.items[itemID] = in
	return writeJSON(w, http.StatusOK, *in)
}

func (s *Server) deleteItem(w http.ResponseWriter, itemID string) int {
	s.mu.Lock()
	_, ok := s.items[itemID]
	if ok {
		delete(s.items, itemID)
		for i, id := range s.order {
			if id == itemID {
				s.order = append(s.order[:i], s.order[i+1:]...)
				break
			}
		}
	}
	s.mu.Unlock()
	if !ok {
		return writeError(w, http.StatusNotFound, "Item not found")
	}
	w.WriteHeader(http.StatusNoContent)
	return http.StatusNoContent
}

// decodeItem reads an Item body and returns a non-empty message when it is
// not valid input.
func (s *Server) decodeItem(r *http.Request) (*item, string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, "body could not be read"
	}
	var in item
	if err := json.Unmarshal(body, &in); err != nil {
		return nil, "body is not an Item object"
	}
	if in.Vault.ID != "" && in.Vault.ID != s.VaultID {
		return nil, "vault id in the body does not match the path"
	}
	if !creatableCategories[in.Category] {
		return nil, fmt.Sprintf("category %q cannot be used to create an item", in.Category)
	}
	for _, f := range in.Fields {
		if f.Type != "" && !fieldTypes[f.Type] {
			return nil, fmt.Sprintf("field type %q is not a field type", f.Type)
		}
		if f.Purpose != "" && !fieldPurposes[f.Purpose] {
			return nil, fmt.Sprintf("field purpose %q is not a field purpose", f.Purpose)
		}
	}
	return &in, ""
}

func assignFieldIDs(it *item) {
	for i := range it.Fields {
		if it.Fields[i].ID == "" {
			it.Fields[i].ID = NewID()
		}
	}
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func writeError(w http.ResponseWriter, status int, message string) int {
	return writeJSON(w, status, errorResponse{Status: status, Message: message})
}

func writeJSON(w http.ResponseWriter, status int, v any) int {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
	return status
}
