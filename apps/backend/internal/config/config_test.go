package config

import (
	"crypto/sha256"
	"encoding/hex"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// knownDevSecrets is the canonical set of plaintexts whose SHA-256 digests
// appear in insecureDevSecretDigests. Keeping these in the test file (and only
// in the test file) lets us verify the digest list is complete and accurate
// without leaking the plaintexts into shipped code.
var knownDevSecrets = []string{
	"dev-only-jwt-secret-do-not-use-in-production-abc123",
	"dev-only-keyvault-key-do-not-use-in-prod==",
	"XfnA4gOf872btnIv8pMyaTBu0bpi4AfbAFDfisQBUt0=",
	"pR1fz62Vd+uDpfdXOzZRx5XXbwsFIbyxhwHZmbRqGmk=",
	"YsOb1gouG02SWoGY3v7VfnuGlSc6zI3f0IWjLbeVw+w=",
	"aim-super-secret-jwt-key-2025-development",
}

func baseValidConfig() *Config {
	return &Config{
		Server: ServerConfig{Environment: "development"},
		Database: DatabaseConfig{
			Host:     "localhost",
			Password: "anything",
		},
		JWT: JWTConfig{
			Secret: strings.Repeat("a", 64),
		},
	}
}

// TestValidate_RejectsKnownDevSecrets_InAllEnvironments closes defect #36:
// before this fix the dev-secret blocklist gated on Server.Environment ==
// "production", so a laptop with ENVIRONMENT=development accepted every known
// bad string for JWT_SECRET. This test fails on master and passes after the
// fix moves the check into Validate() unconditionally.
func TestValidate_RejectsKnownDevSecrets_InAllEnvironments(t *testing.T) {
	t.Parallel()

	// Includes "" and "test" alongside the canonical three so that a future
	// regression that re-introduces any environment gate is caught even when
	// the gate uses an unusual or empty env value.
	environments := []string{"", "development", "test", "staging", "production"}

	for _, env := range environments {
		for _, secret := range knownDevSecrets {
			env := env
			secret := secret
			name := env + "/" + secret[:min(16, len(secret))]
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				cfg := baseValidConfig()
				cfg.Server.Environment = env
				cfg.JWT.Secret = secret
				if err := cfg.Validate(); err == nil {
					t.Fatalf("Validate accepted known dev secret in env=%s; want error", env)
				}
			})
		}
	}
}

// TestValidate_RejectsKnownDevKeyvaultMasterKey closes the KEYVAULT_MASTER_KEY
// half of defect #36 + #17. Same gating bug, separate env var. Before this
// fix the check at config.go:225 only ran when Environment == "production"; a
// dev-env operator with KEYVAULT_MASTER_KEY set to the docker-compose default
// (Xfn...UQBUt0=) silently accepted it.
func TestValidate_RejectsKnownDevKeyvaultMasterKey(t *testing.T) {
	// Not t.Parallel(): mutates a process-wide env var.
	for _, secret := range knownDevSecrets {
		t.Run(secret[:min(16, len(secret))], func(t *testing.T) {
			t.Setenv("KEYVAULT_MASTER_KEY", secret)
			cfg := baseValidConfig()
			cfg.Server.Environment = "development"
			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate accepted known dev KEYVAULT_MASTER_KEY in development; want error")
			}
		})
	}
}

// TestValidate_RejectsKnownDevSecrets_WithWhitespacePadding closes the
// adversarial-review HIGH on the whitespace-paste bypass. Before the
// strings.TrimSpace in isKnownDevSecret, a value pasted from a clipboard with
// a trailing newline or surrounding spaces hashed to a different digest and
// bypassed the blocklist.
func TestValidate_RejectsKnownDevSecrets_WithWhitespacePadding(t *testing.T) {
	t.Parallel()
	paddings := []struct {
		name   string
		wrap   func(string) string
	}{
		{"trailing-newline", func(s string) string { return s + "\n" }},
		{"leading-space", func(s string) string { return " " + s }},
		{"both-sides-whitespace", func(s string) string { return "\t" + s + " \r\n" }},
	}
	for _, p := range paddings {
		for _, secret := range knownDevSecrets {
			padded := p.wrap(secret)
			t.Run(p.name+"/"+secret[:min(16, len(secret))], func(t *testing.T) {
				cfg := baseValidConfig()
				cfg.JWT.Secret = padded
				if err := cfg.Validate(); err == nil {
					t.Fatalf("Validate accepted %s-padded dev secret %q; want error", p.name, padded)
				}
			})
		}
	}
}

// TestValidate_AcceptsFreshlyGeneratedSecrets is the positive control: with
// random secrets, Validate() must succeed in every environment. Catches
// regressions where a future change accidentally rejects valid input.
func TestValidate_AcceptsFreshlyGeneratedSecrets(t *testing.T) {
	t.Parallel()
	for _, env := range []string{"development", "staging", "production"} {
		env := env
		t.Run(env, func(t *testing.T) {
			t.Parallel()
			cfg := baseValidConfig()
			cfg.Server.Environment = env
			cfg.JWT.Secret = strings.Repeat("z", 64) // 64-char placeholder, not on blocklist
			cfg.Database.Password = "anything-non-default"
			if err := cfg.Validate(); err != nil {
				t.Fatalf("Validate rejected fresh secret in env=%s: %v", env, err)
			}
		})
	}
}

// TestValidate_RejectsEmptyJWTSecret guards getEnvRequired's invariant at the
// Validate() layer too. Defensive: covers the case where a future refactor
// constructs Config directly without going through Load().
func TestValidate_RejectsEmptyJWTSecret(t *testing.T) {
	t.Parallel()
	cfg := baseValidConfig()
	cfg.JWT.Secret = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted empty JWT_SECRET; want error")
	}
}

// TestValidate_RejectsShortJWTSecret keeps the existing >=32-char floor.
func TestValidate_RejectsShortJWTSecret(t *testing.T) {
	t.Parallel()
	cfg := baseValidConfig()
	cfg.JWT.Secret = "too-short"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted JWT_SECRET shorter than 32 chars; want error")
	}
}

// TestInsecureDevSecretDigests_MatchKnownPlaintexts is the round-trip that
// keeps the production blocklist accurate. If anyone changes a digest by hand
// or drops one, this test catches it. Closes defect #36's "are the hashes
// actually right?" review surface.
func TestInsecureDevSecretDigests_MatchKnownPlaintexts(t *testing.T) {
	t.Parallel()
	if len(insecureDevSecretDigests) != len(knownDevSecrets) {
		t.Fatalf("digest count drift: %d production digests vs %d known plaintexts", len(insecureDevSecretDigests), len(knownDevSecrets))
	}
	want := make(map[string]struct{}, len(knownDevSecrets))
	for _, s := range knownDevSecrets {
		sum := sha256.Sum256([]byte(s))
		want[hex.EncodeToString(sum[:])] = struct{}{}
	}
	for _, got := range insecureDevSecretDigests {
		if _, ok := want[got]; !ok {
			t.Fatalf("production digest %s does not match any known plaintext; either a digest was edited by hand or a plaintext was removed from the test file", got)
		}
	}
}

// TestConfigSource_HasNoPlaintextDevSecrets closes defect #36 directly: walks
// the AST of config.go and asserts no string literal decrypts to one of the
// known dev secrets. Fails on master because lines 13-20 of config.go (pre-
// fix) carried the plaintexts inline. Passes after the fix replaces them with
// SHA-256 hex digests, which do not match the plaintext set.
func TestConfigSource_HasNoPlaintextDevSecrets(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("config.go")
	if err != nil {
		t.Fatalf("read config.go: %v", err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "config.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse config.go: %v", err)
	}

	banned := make(map[string]struct{}, len(knownDevSecrets))
	for _, s := range knownDevSecrets {
		banned[s] = struct{}{}
	}

	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		// lit.Value includes the surrounding quotes.
		raw := lit.Value
		if len(raw) < 2 {
			return true
		}
		unquoted := raw[1 : len(raw)-1]
		if _, isBanned := banned[unquoted]; isBanned {
			pos := fset.Position(lit.Pos())
			t.Errorf("config.go:%d carries plaintext dev secret %q in shipped source; replace with a SHA-256 digest in insecureDevSecretDigests", pos.Line, unquoted)
		}
		return true
	})
}
