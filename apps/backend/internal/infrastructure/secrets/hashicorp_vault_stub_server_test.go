package secrets

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/secrets/vaultkvtest"
)

// These tests run the backend through the Vault client against vaultkvtest, an
// in-memory server of the KV secrets engine version 2 HTTP API on a loopback
// port. They need no vault binary. The last one, where a vault binary is on
// PATH, sends the same requests to vault server -dev and to the stub and
// requires the same answers.

func TestHashiCorpVaultBackend_StubServerStoreRetrieveRotateDelete(t *testing.T) {
	srv := vaultkvtest.NewServer(t)
	b := NewHashiCorpVaultBackendFromClient(vaultkvtest.NewClient(t, srv), vaultkvtest.Mount)
	nsID := uuid.New()
	key := "aim-secrets/" + nsID.String()
	data, meta := vaultkvtest.Mount+"/data/"+key, vaultkvtest.Mount+"/metadata/"+key

	// Bytes that are not valid UTF-8, so a blob returned as text fails.
	first := []byte{0x00, 0x01, 0x7f, 0x80, 0xfe, 0xff, 'a', 'i', 'm'}
	if err := b.Store(nsID, first, "AES-256-GCM"); err != nil {
		t.Fatalf("Store: %v", err)
	}
	// The client sends the []byte blob as a standard base64 string and the
	// server keeps it as one: the form Retrieve decodes.
	held, number, ok := srv.Entry(key)
	if !ok || number != 1 {
		t.Fatalf("Entry after Store = %v %d %v, want version 1", held, number, ok)
	}
	if held["blob"] != base64.StdEncoding.EncodeToString(first) || held["encryptionAlg"] != "AES-256-GCM" {
		t.Fatalf("the server holds %v, want the blob as its base64 string and the algorithm", held)
	}
	assertVaultRoundTrip(t, b, nsID, first, "AES-256-GCM", 1)

	second := []byte{0xff, 0x00, 0xc3, 0x28, 'r', 'o', 't'}
	if err := b.Rotate(nsID, second, "X25519-ChaCha20-Poly1305"); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	assertVaultRoundTrip(t, b, nsID, second, "X25519-ChaCha20-Poly1305", 2)
	if _, number, _ := srv.Entry(key); number != 2 {
		t.Fatalf("the server holds version %d after Rotate, want 2", number)
	}

	if err := b.Delete(nsID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, err := b.Retrieve(nsID)
	if err == nil {
		t.Fatalf("Retrieve after Delete returned %+v, want an error", got)
	}
	if !strings.Contains(err.Error(), "no credential found in vault for namespace "+nsID.String()) {
		t.Fatalf("Retrieve after Delete: %v, want the not-found error", err)
	}
	if _, _, ok := srv.Entry(key); ok {
		t.Fatal("the server still holds the path after Delete")
	}

	want := []vaultkvtest.Request{
		{Method: http.MethodPut, Path: data},    // Store
		{Method: http.MethodGet, Path: data},    // the first round trip
		{Method: http.MethodGet, Path: data},    // Rotate reads the current version
		{Method: http.MethodPut, Path: data},    // Rotate writes the next one
		{Method: http.MethodGet, Path: data},    // the second round trip
		{Method: http.MethodDelete, Path: meta}, // Delete removes every version
		{Method: http.MethodGet, Path: data},    // Retrieve after Delete
	}
	if got := srv.Requests(); !slices.Equal(got, want) {
		t.Fatalf("requests = %v, want %v", got, want)
	}
}

func TestHashiCorpVaultBackend_StubServerRefusesAnotherToken(t *testing.T) {
	srv := vaultkvtest.NewServer(t)
	client := vaultkvtest.NewClient(t, srv)
	client.SetToken("vaultkvtest-not-the-servers-token")
	b := NewHashiCorpVaultBackendFromClient(client, vaultkvtest.Mount)
	nsID := uuid.New()

	if err := b.Store(nsID, []byte("blob"), "AES-256-GCM"); err == nil || !strings.Contains(err.Error(), "vault store failed") || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("Store with another token: %v, want the store failure with Vault's permission denied", err)
	}
	if _, err := b.Retrieve(nsID); err == nil || !strings.Contains(err.Error(), "vault retrieve failed") || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("Retrieve with another token: %v, want the retrieve failure with Vault's permission denied", err)
	}
	if err := b.Delete(nsID); err == nil || !strings.Contains(err.Error(), "vault delete failed") || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("Delete with another token: %v, want the delete failure with Vault's permission denied", err)
	}
	if n := len(srv.Requests()); n != 0 {
		t.Fatalf("%d requests with another token reached the engine", n)
	}
}

// vaultAnswer is what one request observed of a server's answer: the status,
// the errors and warnings lists, the stored map of a read, and the version
// number a write, a read or a metadata read reports.
type vaultAnswer struct {
	Step     string
	Status   int
	Errors   []string
	Warnings []string
	HasData  bool
	Data     map[string]interface{}
	Version  float64
	Versions []string
}

type vaultStep struct {
	name, method, path, token, body string
}

func observeVault(t *testing.T, client *http.Client, baseURL, token string, step vaultStep) vaultAnswer {
	t.Helper()
	req, err := http.NewRequest(step.method, baseURL+"/v1/"+step.path, strings.NewReader(step.body))
	if err != nil {
		t.Fatal(err)
	}
	switch step.token {
	case "right":
		req.Header.Set("X-Vault-Token", token)
	case "wrong":
		req.Header.Set("X-Vault-Token", "vaultkvtest-not-the-servers-token")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s: %v", step.name, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	a := vaultAnswer{Step: step.name, Status: resp.StatusCode}
	if len(raw) == 0 {
		return a
	}
	var body map[string]interface{}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("%s: answer is not JSON: %v: %s", step.name, err, raw)
	}
	for _, e := range toInterfaceSlice(body["errors"]) {
		a.Errors = append(a.Errors, e.(string))
	}
	for _, w := range toInterfaceSlice(body["warnings"]) {
		a.Warnings = append(a.Warnings, w.(string))
	}
	d, ok := body["data"].(map[string]interface{})
	if !ok {
		return a
	}
	a.HasData = true
	if inner, ok := d["data"].(map[string]interface{}); ok {
		a.Data = inner
	}
	if meta, ok := d["metadata"].(map[string]interface{}); ok {
		a.Version, _ = meta["version"].(float64)
	}
	if v, ok := d["version"].(float64); ok {
		a.Version = v
	}
	if v, ok := d["current_version"].(float64); ok {
		a.Version = v
	}
	if versions, ok := d["versions"].(map[string]interface{}); ok {
		for k := range versions {
			a.Versions = append(a.Versions, k)
		}
		sort.Strings(a.Versions)
	}
	return a
}

func toInterfaceSlice(v interface{}) []interface{} {
	s, _ := v.([]interface{})
	return s
}

// The stub answers the requests the backend makes, and the refusals around
// them, as a Vault dev server does. The dev server is the measurement; the
// test skips without a vault binary.
func TestVaultKVStub_AnswersAsADevServerDoes(t *testing.T) {
	addr, rootToken := startVaultDevServer(t)
	stub := vaultkvtest.NewServer(t)

	key := "aim-secrets/" + uuid.NewString()
	data, meta := vaultkvtest.Mount+"/data/"+key, vaultkvtest.Mount+"/metadata/"+key
	steps := []vaultStep{
		{"read without a token", http.MethodGet, data, "none", ""},
		{"read with another token", http.MethodGet, data, "wrong", ""},
		{"read before any write", http.MethodGet, data, "right", ""},
		{"first write", http.MethodPut, data, "right", `{"data":{"blob":"AAF/gP7/YWlt","encryptionAlg":"AES-256-GCM","version":1}}`},
		{"read the first version", http.MethodGet, data, "right", ""},
		{"second write by POST", http.MethodPost, data, "right", `{"data":{"blob":"/wDDKHJvdA==","encryptionAlg":"X25519-ChaCha20-Poly1305","version":2}}`},
		{"read the second version", http.MethodGet, data, "right", ""},
		{"read the metadata", http.MethodGet, meta, "right", ""},
		{"delete the metadata", http.MethodDelete, meta, "right", ""},
		{"read after the delete", http.MethodGet, data, "right", ""},
		{"read the metadata after the delete", http.MethodGet, meta, "right", ""},
		{"delete the metadata again", http.MethodDelete, meta, "right", ""},
		{"read under another mount", http.MethodGet, "nosuchmount/data/" + key, "right", ""},
		{"write without a data field", http.MethodPut, data, "right", `{"blob":"x"}`},
		{"write an empty body", http.MethodPut, data, "right", ``},
		{"write a body that is not JSON", http.MethodPut, data, "right", `not json`},
		{"write a data field that is not an object", http.MethodPut, data, "right", `{"data":"x"}`},
		{"read with a trailing slash", http.MethodGet, data + "/", "right", ""},
		{"read a path that is neither data nor metadata", http.MethodGet, vaultkvtest.Mount + "/other/" + key, "right", ""},
	}

	client := &http.Client{}
	var fromVault, fromStub []vaultAnswer
	for _, step := range steps {
		fromVault = append(fromVault, observeVault(t, client, addr, rootToken, step))
		fromStub = append(fromStub, observeVault(t, stub.Client(), stub.URL, stub.Token, step))
	}
	require.Equal(t, fromVault, fromStub, "the stub's answers differ from the Vault dev server's")
	require.Len(t, fromVault, len(steps))
}
