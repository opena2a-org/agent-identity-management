package secrets

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	vault "github.com/hashicorp/vault/api"
)

// These tests run the backend against a real Vault server: `vault server -dev`,
// started by the test on a loopback port and stopped in t.Cleanup. They skip
// when no vault binary is on PATH.

// vaultDevPort returns a free loopback port whose next port is free as well:
// the dev server also listens for cluster traffic on the port after its API
// port.
func vaultDevPort(t *testing.T) int {
	t.Helper()
	for range 20 {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("no free loopback port: %v", err)
		}
		port := l.Addr().(*net.TCPAddr).Port
		next, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port+1))
		_ = l.Close()
		if err != nil {
			continue
		}
		_ = next.Close()
		return port
	}
	t.Fatal("no free pair of loopback ports for the vault dev server")
	return 0
}

// startVaultDevServer starts `vault server -dev` with a root token generated
// here and returns its address and that token once /v1/sys/health answers 200.
// The server keeps everything in memory, runs with an empty HOME and stores no
// token on disk.
func startVaultDevServer(t *testing.T) (addr, rootToken string) {
	t.Helper()
	bin, err := exec.LookPath("vault")
	if err != nil {
		t.Skip("vault binary not on PATH: this test runs a Vault dev server (vault server -dev) and skips without one")
	}

	listen := fmt.Sprintf("127.0.0.1:%d", vaultDevPort(t))
	rootToken = uuid.NewString()
	home := t.TempDir()
	logPath := filepath.Join(home, "vault-dev.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, bin, "server", "-dev",
		"-dev-listen-address="+listen,
		"-dev-root-token-id="+rootToken,
		"-dev-no-store-token")
	cmd.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		cancel()
		_ = logFile.Close()
		t.Fatalf("start vault dev server: %v", err)
	}
	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()
	t.Cleanup(func() {
		cancel()
		<-exited
		_ = logFile.Close()
	})

	addr = "http://" + listen
	probe := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(30 * time.Second)
	for {
		resp, err := probe.Get(addr + "/v1/sys/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return addr, rootToken
			}
		}
		select {
		case <-exited:
			out, _ := os.ReadFile(logPath)
			t.Fatalf("vault dev server exited before it was healthy; its output:\n%s", out)
		default:
		}
		if time.Now().After(deadline) {
			out, _ := os.ReadFile(logPath)
			t.Fatalf("vault dev server at %s was not healthy within 30s; its output:\n%s", addr, out)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func vaultTestClient(t *testing.T, addr, token string) *vault.Client {
	t.Helper()
	cfg := vault.DefaultConfig()
	cfg.Address = addr
	client, err := vault.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	client.SetToken(token)
	return client
}

// assertVaultRoundTrip stores a blob and reads it back through the backend.
func assertVaultRoundTrip(t *testing.T, b *HashiCorpVaultBackend, nsID uuid.UUID, blob []byte, alg string, wantVersion int) {
	t.Helper()
	got, err := b.Retrieve(nsID)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if !bytes.Equal(got.EncryptedBlob, blob) {
		t.Fatalf("Retrieve blob = %x, want %x", got.EncryptedBlob, blob)
	}
	if got.EncryptionAlg != alg {
		t.Errorf("Retrieve encryptionAlg = %q, want %q", got.EncryptionAlg, alg)
	}
	if got.Version != wantVersion {
		t.Errorf("Retrieve version = %d, want %d", got.Version, wantVersion)
	}
	if got.NamespaceID != nsID {
		t.Errorf("Retrieve namespaceID = %s, want %s", got.NamespaceID, nsID)
	}
}

func TestHashiCorpVaultBackend_DevServerStoreRetrieveRotateDelete(t *testing.T) {
	addr, rootToken := startVaultDevServer(t)
	client := vaultTestClient(t, addr, rootToken)
	b := NewHashiCorpVaultBackendFromClient(client, "secret")
	nsID := uuid.New()

	// Bytes that are not valid UTF-8, so a blob returned as text fails.
	first := []byte{0x00, 0x01, 0x7f, 0x80, 0xfe, 0xff, 'a', 'i', 'm'}
	if err := b.Store(nsID, first, "AES-256-GCM"); err != nil {
		t.Fatalf("Store: %v", err)
	}
	assertVaultRoundTrip(t, b, nsID, first, "AES-256-GCM", 1)

	kvPath := "secret/data/aim-secrets/" + nsID.String()
	if entry, err := client.Logical().Read(kvPath); err != nil || entry == nil {
		t.Fatalf("no KV v2 entry at %s after Store (err %v)", kvPath, err)
	}

	second := []byte{0xff, 0x00, 0xc3, 0x28, 'r', 'o', 't'}
	if err := b.Rotate(nsID, second, "X25519-ChaCha20-Poly1305"); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	assertVaultRoundTrip(t, b, nsID, second, "X25519-ChaCha20-Poly1305", 2)

	if err := b.Delete(nsID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got, err := b.Retrieve(nsID); err == nil {
		t.Fatalf("Retrieve after Delete returned %+v, want an error", got)
	}
	metaPath := "secret/metadata/aim-secrets/" + nsID.String()
	if meta, err := client.Logical().Read(metaPath); err != nil || meta != nil {
		t.Fatalf("metadata at %s after Delete = %v (err %v), want none", metaPath, meta, err)
	}
}

// vaultJWTRole is the jwt auth setup the login test configures: a role bound
// to the issuer, subject, audience and one run-specific claim of the test's
// JWT.
type vaultJWTRole struct {
	issuer, subject, audience, runClaim string
}

func (r vaultJWTRole) mint(t *testing.T, key *ecdsa.PrivateKey) string {
	t.Helper()
	now := time.Now()
	token, err := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss":     r.issuer,
		"sub":     r.subject,
		"aud":     r.audience,
		"aim_run": r.runClaim,
		"iat":     now.Unix(),
		"nbf":     now.Add(-time.Minute).Unix(),
		"exp":     now.Add(5 * time.Minute).Unix(),
	}).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func configureVaultJWTAuth(t *testing.T, admin *vault.Client, pub *ecdsa.PublicKey, role vaultJWTRole) {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})

	if err := admin.Sys().EnableAuthWithOptions("jwt", &vault.EnableAuthOptions{Type: "jwt"}); err != nil {
		t.Fatalf("enable jwt auth: %v", err)
	}
	if _, err := admin.Logical().Write("auth/jwt/config", map[string]interface{}{
		"jwt_validation_pubkeys": []string{string(pubPEM)},
		"bound_issuer":           role.issuer,
	}); err != nil {
		t.Fatalf("configure jwt auth: %v", err)
	}
	policy := `
path "secret/data/aim-secrets/*" {
  capabilities = ["create", "read", "update"]
}
path "secret/metadata/aim-secrets/*" {
  capabilities = ["read", "delete"]
}
`
	if err := admin.Sys().PutPolicy("aim-secrets", policy); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	if _, err := admin.Logical().Write("auth/jwt/role/aim-backend", map[string]interface{}{
		"role_type":       "jwt",
		"user_claim":      "sub",
		"bound_subject":   role.subject,
		"bound_audiences": []string{role.audience},
		"bound_claims":    map[string]interface{}{"aim_run": role.runClaim},
		"token_policies":  []string{"aim-secrets"},
		"token_ttl":       "5m",
	}); err != nil {
		t.Fatalf("write jwt role: %v", err)
	}
}

func TestHashiCorpVaultBackend_DevServerJWTLogin(t *testing.T) {
	addr, rootToken := startVaultDevServer(t)
	admin := vaultTestClient(t, addr, rootToken)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	role := vaultJWTRole{
		issuer:   "https://aim-test.invalid",
		subject:  "aim-backend",
		audience: "aim-vault-test",
		runClaim: uuid.NewString(),
	}
	configureVaultJWTAuth(t, admin, &key.PublicKey, role)

	cfg := &VaultConfig{Address: addr, MountPath: "secret", AuthPath: "jwt", Role: "aim-backend"}
	t.Setenv("VAULT_TOKEN", "")

	t.Run("a JWT signed by the configured key logs in", func(t *testing.T) {
		t.Setenv("VAULT_JWT", role.mint(t, key))
		b, err := NewHashiCorpVaultBackend(cfg)
		if err != nil {
			t.Fatalf("NewHashiCorpVaultBackend with JWT auth: %v", err)
		}

		self, err := b.client.Auth().Token().LookupSelf()
		if err != nil {
			t.Fatalf("look up the login token: %v", err)
		}
		policies, err := self.TokenPolicies()
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(policies, "aim-secrets") || slices.Contains(policies, "root") {
			t.Fatalf("login token policies = %v, want the role's aim-secrets policy and not root", policies)
		}

		nsID := uuid.New()
		blob := []byte{0x00, 0xff, 'j', 'w', 't'}
		if err := b.Store(nsID, blob, "AES-256-GCM"); err != nil {
			t.Fatalf("Store with the JWT login token: %v", err)
		}
		assertVaultRoundTrip(t, b, nsID, blob, "AES-256-GCM", 1)
	})

	t.Run("a JWT signed by another key is refused at login", func(t *testing.T) {
		other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("VAULT_JWT", role.mint(t, other))
		b, err := NewHashiCorpVaultBackend(cfg)
		if err == nil {
			t.Fatal("NewHashiCorpVaultBackend logged in with a JWT signed by a key Vault was not given")
		}
		if b != nil {
			t.Fatal("a backend was returned for a refused login")
		}
		if !strings.Contains(err.Error(), "vault JWT auth failed") {
			t.Fatalf("error = %q, want the login refusal", err)
		}
	})
}
