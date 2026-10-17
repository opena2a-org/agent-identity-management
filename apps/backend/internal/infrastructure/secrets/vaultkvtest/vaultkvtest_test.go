package vaultkvtest

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// rawRequest sends one request to s and returns the status and the decoded
// JSON body, nil for an empty body. An empty token sends no X-Vault-Token.
func rawRequest(t *testing.T, s *Server, method, path, token, body string) (int, map[string]interface{}) {
	t.Helper()
	req, err := http.NewRequest(method, s.URL+"/v1/"+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("X-Vault-Token", token)
	}
	resp, err := s.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		return resp.StatusCode, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("answer is not JSON: %v: %s", err, raw)
	}
	return resp.StatusCode, out
}

func errorsOf(t *testing.T, out map[string]interface{}) []string {
	t.Helper()
	raw, ok := out["errors"].([]interface{})
	if !ok {
		t.Fatalf("answer carries no errors list: %v", out)
	}
	var errs []string
	for _, e := range raw {
		errs = append(errs, e.(string))
	}
	return errs
}

func TestServer_RefusesARequestWithoutItsToken(t *testing.T) {
	s := NewServer(t)
	path := Mount + "/data/aim-secrets/x"

	status, out := rawRequest(t, s, http.MethodGet, path, "", "")
	if status != http.StatusForbidden || !slices.Equal(errorsOf(t, out), []string{"permission denied"}) {
		t.Fatalf("no token: answer = %d %v, want 403 [permission denied]", status, out)
	}

	status, out = rawRequest(t, s, http.MethodGet, path, "vaultkvtest-another-token", "")
	if status != http.StatusForbidden || !slices.Equal(errorsOf(t, out), []string{wrongTokenError}) {
		t.Fatalf("another token: answer = %d %v, want 403 with Vault's permission denied and invalid token message", status, out)
	}

	if n := len(s.Requests()); n != 0 {
		t.Fatalf("%d refused requests were recorded", n)
	}
}

func TestServer_AnswersPathsOutsideTheEngineAsVaultDoes(t *testing.T) {
	s := NewServer(t)

	status, out := rawRequest(t, s, http.MethodGet, "kv/data/aim-secrets/x", s.Token, "")
	want := []string{`no handler for route "kv/data/aim-secrets/x". route entry not found.`}
	if status != http.StatusNotFound || !slices.Equal(errorsOf(t, out), want) {
		t.Fatalf("another mount: answer = %d %v, want 404 %v", status, out, want)
	}

	for _, path := range []string{Mount + "/data/aim-secrets/x/", Mount + "/other/aim-secrets/x", Mount + "/data"} {
		status, out := rawRequest(t, s, http.MethodGet, path, s.Token, "")
		warnings, _ := out["warnings"].([]interface{})
		if status != http.StatusNotFound || out["data"] != nil || len(warnings) != 1 || warnings[0] != invalidPathWarning {
			t.Fatalf("%s: answer = %d %v, want 404 with data null and the invalid path warning", path, status, out)
		}
	}

	status, out = rawRequest(t, s, http.MethodPatch, Mount+"/data/aim-secrets/x", s.Token, `{"data":{"a":"b"}}`)
	if status != http.StatusMethodNotAllowed || !slices.Equal(errorsOf(t, out), []string{"unsupported operation"}) {
		t.Fatalf("PATCH: answer = %d %v, want 405 [unsupported operation]", status, out)
	}

	if n := len(s.Requests()); n != 0 {
		t.Fatalf("%d refused requests were recorded", n)
	}
}

func TestServer_StoresVersionsAndDeletesThemWithTheMetadata(t *testing.T) {
	s := NewServer(t)
	key := "aim-secrets/11111111-2222-3333-4444-555555555555"
	data, meta := Mount+"/data/"+key, Mount+"/metadata/"+key

	status, out := rawRequest(t, s, http.MethodGet, data, s.Token, "")
	if status != http.StatusNotFound || len(errorsOf(t, out)) != 0 {
		t.Fatalf("read before any write: answer = %d %v, want 404 with an empty errors list", status, out)
	}

	status, out = rawRequest(t, s, http.MethodPut, data, s.Token, `{"data":{"blob":"AAF/gP7/YWlt","encryptionAlg":"AES-256-GCM","version":1}}`)
	if status != http.StatusOK || out["data"].(map[string]interface{})["version"] != 1.0 {
		t.Fatalf("first write: answer = %d %v, want 200 with version 1", status, out)
	}

	status, out = rawRequest(t, s, http.MethodGet, data, s.Token, "")
	if status != http.StatusOK {
		t.Fatalf("read: status %d, want 200: %v", status, out)
	}
	got := out["data"].(map[string]interface{})
	stored := got["data"].(map[string]interface{})
	if stored["blob"] != "AAF/gP7/YWlt" || stored["encryptionAlg"] != "AES-256-GCM" || stored["version"] != 1.0 {
		t.Fatalf("read data.data = %v, want the map as it was written", stored)
	}
	if got["metadata"].(map[string]interface{})["version"] != 1.0 || out["mount_type"] != "kv" {
		t.Fatalf("read metadata = %v, mount_type = %v, want version 1 and kv", got["metadata"], out["mount_type"])
	}

	status, out = rawRequest(t, s, http.MethodPost, data, s.Token, `{"data":{"blob":"/wDDKHJvdA==","encryptionAlg":"X25519-ChaCha20-Poly1305","version":2}}`)
	if status != http.StatusOK || out["data"].(map[string]interface{})["version"] != 2.0 {
		t.Fatalf("second write by POST: answer = %d %v, want 200 with version 2", status, out)
	}
	held, number, ok := s.Entry(key)
	if !ok || number != 2 || held["blob"] != "/wDDKHJvdA==" {
		t.Fatalf("Entry = %v %d %v, want the second write as version 2", held, number, ok)
	}

	status, out = rawRequest(t, s, http.MethodGet, meta, s.Token, "")
	metadata := out["data"].(map[string]interface{})
	versions := metadata["versions"].(map[string]interface{})
	if status != http.StatusOK || metadata["current_version"] != 2.0 || len(versions) != 2 || versions["1"] == nil || versions["2"] == nil {
		t.Fatalf("metadata: answer = %d %v, want current_version 2 and versions 1 and 2", status, out)
	}

	if status, out = rawRequest(t, s, http.MethodDelete, meta, s.Token, ""); status != http.StatusNoContent || out != nil {
		t.Fatalf("delete metadata: answer = %d %v, want 204 with no body", status, out)
	}
	if status, out = rawRequest(t, s, http.MethodGet, data, s.Token, ""); status != http.StatusNotFound || len(errorsOf(t, out)) != 0 {
		t.Fatalf("read after delete: answer = %d %v, want 404 with an empty errors list", status, out)
	}
	if status, out = rawRequest(t, s, http.MethodGet, meta, s.Token, ""); status != http.StatusNotFound || len(errorsOf(t, out)) != 0 {
		t.Fatalf("metadata after delete: answer = %d %v, want 404 with an empty errors list", status, out)
	}
	if _, _, ok := s.Entry(key); ok {
		t.Fatal("Entry still finds the path after the metadata delete")
	}
	if status, _ = rawRequest(t, s, http.MethodDelete, meta, s.Token, ""); status != http.StatusNoContent {
		t.Fatalf("delete of a path that holds nothing: status %d, want 204", status)
	}

	refused := []struct{ body, want string }{
		{`{"blob":"x"}`, "no data provided"},
		{``, "no data provided"},
		{`not json`, "error parsing JSON: failed to parse JSON input: invalid character 'o' in literal null (expecting 'u')"},
		{`[]`, "error parsing JSON: failed to parse JSON input: json: cannot unmarshal array into Go value of type map[string]interface {}"},
		{`{"data":"x"}`, `error converting input for field "data": '' expected a map, got 'string'`},
	}
	for _, tc := range refused {
		status, out := rawRequest(t, s, http.MethodPut, data, s.Token, tc.body)
		if status != http.StatusBadRequest || !slices.Equal(errorsOf(t, out), []string{tc.want}) {
			t.Fatalf("write of %q: answer = %d %v, want 400 [%s]", tc.body, status, out, tc.want)
		}
	}
	if _, _, ok := s.Entry(key); ok {
		t.Fatal("a refused write stored a version")
	}

	want := []Request{
		{http.MethodGet, data}, {http.MethodPut, data}, {http.MethodGet, data}, {http.MethodPost, data},
		{http.MethodGet, meta}, {http.MethodDelete, meta}, {http.MethodGet, data}, {http.MethodGet, meta},
		{http.MethodDelete, meta}, {http.MethodPut, data}, {http.MethodPut, data}, {http.MethodPut, data},
		{http.MethodPut, data}, {http.MethodPut, data},
	}
	if got := s.Requests(); !slices.Equal(got, want) {
		t.Fatalf("requests = %v, want %v", got, want)
	}
}
