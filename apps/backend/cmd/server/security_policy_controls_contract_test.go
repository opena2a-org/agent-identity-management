package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
)

// securityPolicyText returns SECURITY.md with every run of whitespace folded to
// one space, so a statement wrapped across lines matches as one sentence.
func securityPolicyText(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(aim03RepoRoot(t), "SECURITY.md"))
	require.NoError(t, err)
	return strings.Join(strings.Fields(string(body)), " ")
}

// TestSecurityPolicyNamesNoControlTheCodeLacks fails on a SECURITY.md statement
// about a control the code does not have. The page said CSRF was prevented with
// tokens, that the SDK encrypted its credentials with Fernet and a keyring key,
// that required secrets fell back to weak defaults, and that all personal data
// was encrypted at rest.
func TestSecurityPolicyNamesNoControlTheCodeLacks(t *testing.T) {
	policy := securityPolicyText(t)
	for _, claim := range []struct{ pattern, fact string }{
		{`(?i)token-based CSRF`, "the server issues and checks no CSRF token"},
		{`(?i)Fernet|keyring`, "the Python SDK writes its credential files as plain JSON"},
		{`(?i)fall back to weak defaults`, "the server refuses to start without JWT_SECRET, and without KEYVAULT_MASTER_KEY outside development and test"},
		{`(?i)all PII (is )?encrypted`, "users' email addresses and names are plain database columns"},
	} {
		if loc := regexp.MustCompile(claim.pattern).FindStringIndex(policy); loc != nil {
			t.Errorf("SECURITY.md says %q, but %s", policy[loc[0]:loc[1]], claim.fact)
		}
	}
}

// TestSecurityPolicyCSRFStatementHoldsForTheServer pins what SECURITY.md says
// in place of a CSRF token: no server code reads a credential from a cookie or
// installs a CSRF middleware.
func TestSecurityPolicyCSRFStatementHoldsForTheServer(t *testing.T) {
	require.Contains(t, securityPolicyText(t), "The server reads no credential from a cookie")

	backend := filepath.Join(aim03RepoRoot(t), "apps", "backend")
	cookieRead := regexp.MustCompile(`\.Cookies\(`)
	checked := 0
	err := filepath.WalkDir(backend, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		checked++
		rel, _ := filepath.Rel(backend, path)
		if cookieRead.Match(src) {
			t.Errorf("%s reads a cookie; SECURITY.md says the server reads no credential from one", rel)
		}
		if strings.Contains(string(src), "middleware/csrf") {
			t.Errorf("%s installs a CSRF middleware; SECURITY.md says no CSRF token is issued or checked", rel)
		}
		return nil
	})
	require.NoError(t, err)
	require.Greater(t, checked, 100, "the backend has far more Go files than the walk found; the walk is broken")
}

// TestSecurityPolicySDKCredentialStatementHoldsForThePythonSDK pins what
// SECURITY.md says about the Python SDK's credential files: one writer stores
// both the login tokens and the agent keys as JSON and sets file mode 0600.
func TestSecurityPolicySDKCredentialStatementHoldsForThePythonSDK(t *testing.T) {
	require.Contains(t, securityPolicyText(t), "as plain JSON with file mode 0600")

	src, err := os.ReadFile(filepath.Join(aim03RepoRoot(t), "sdk", "python", "aim_sdk", "credentials.py"))
	require.NoError(t, err)
	module := string(src)
	start := strings.Index(module, "\ndef _write_private_json(")
	require.GreaterOrEqual(t, start, 0, "credentials.py has no _write_private_json")
	writer := module[start+1:]
	if end := strings.Index(writer, "\ndef "); end >= 0 {
		writer = writer[:end]
	}
	require.Contains(t, writer, "json.dump(data, f")
	require.Contains(t, writer, "os.chmod(path, 0o600)")
	require.Contains(t, module, "_write_private_json(SDK_CREDENTIALS_FILE, credentials)")
	require.Contains(t, module, "_write_private_json(agent_file, credentials)")
	require.NotContains(t, module, "Fernet")
}

// TestSecurityPolicyConfigurationStatementHoldsForTheServer pins what
// SECURITY.md says the server refuses at start: a missing or short JWT_SECRET,
// a missing KEYVAULT_MASTER_KEY outside development and test, and a weak
// POSTGRES_PASSWORD for a remote host in production.
func TestSecurityPolicyConfigurationStatementHoldsForTheServer(t *testing.T) {
	policy := securityPolicyText(t)
	require.Contains(t, policy, "The server refuses to start without `JWT_SECRET`, with one shorter than 32 characters")
	require.Contains(t, policy, "it also refuses to start without `KEYVAULT_MASTER_KEY` unless `ENVIRONMENT` is `development` or `test`")
	require.Contains(t, policy, "With `ENVIRONMENT=production` it refuses an empty or `postgres` `POSTGRES_PASSWORD` for a remote database host")

	t.Setenv("KEYVAULT_MASTER_KEY", "")
	secret := "security-policy-contract-test-secret-0123456789"

	require.ErrorContains(t, (&config.Config{}).Validate(), "JWT_SECRET is required")
	short := &config.Config{JWT: config.JWTConfig{Secret: secret[:31]}}
	require.ErrorContains(t, short.Validate(), "at least 32 characters")
	for _, password := range []string{"", "postgres"} {
		remote := &config.Config{
			Server:   config.ServerConfig{Environment: "production"},
			Database: config.DatabaseConfig{Host: "db.example.com", Password: password},
			JWT:      config.JWTConfig{Secret: secret},
		}
		require.ErrorContains(t, remote.Validate(), "POSTGRES_PASSWORD", "password %q", password)
	}
	valid := &config.Config{
		Server:   config.ServerConfig{Environment: "production"},
		Database: config.DatabaseConfig{Host: "db.example.com", Password: secret},
		JWT:      config.JWTConfig{Secret: secret},
	}
	require.NoError(t, valid.Validate())

	for _, environment := range []string{"", "production", "staging"} {
		t.Setenv("ENVIRONMENT", environment)
		_, err := crypto.NewKeyVaultFromEnv()
		require.ErrorContains(t, err, "KEYVAULT_MASTER_KEY is required", "ENVIRONMENT=%q", environment)
	}
}

// TestSecurityPolicyPersonalDataStatementHoldsForTheSchema pins what
// SECURITY.md says about personal data at rest: the users table keeps email
// and name in plain VARCHAR columns, and no migration changes either column.
func TestSecurityPolicyPersonalDataStatementHoldsForTheSchema(t *testing.T) {
	require.Contains(t, securityPolicyText(t), "Users' email addresses and names are stored in plain database columns")

	migrations := filepath.Join(aim03RepoRoot(t), "apps", "backend", "migrations")
	initial, err := os.ReadFile(filepath.Join(migrations, "001_initial_schema.sql"))
	require.NoError(t, err)
	users := regexp.MustCompile(`(?s)CREATE TABLE IF NOT EXISTS users \((.*?)\n\);`).FindSubmatch(initial)
	require.NotNil(t, users, "001_initial_schema.sql creates no users table")
	require.Regexp(t, `(?m)^\s*email VARCHAR\(255\) NOT NULL,`, string(users[1]))
	require.Regexp(t, `(?m)^\s*name VARCHAR\(255\) NOT NULL,`, string(users[1]))

	alter := regexp.MustCompile(`(?is)ALTER\s+TABLE\s+(IF\s+EXISTS\s+)?(ONLY\s+)?users\s[^;]*ALTER\s+COLUMN\s+(email|name)\b`)
	files, err := filepath.Glob(filepath.Join(migrations, "*.sql"))
	require.NoError(t, err)
	require.Greater(t, len(files), 50, "the migrations directory has far more files than the glob found")
	for _, file := range files {
		sql, err := os.ReadFile(file)
		require.NoError(t, err)
		if m := alter.Find(sql); m != nil {
			t.Errorf("%s changes a users column SECURITY.md calls plain: %s", filepath.Base(file), m)
		}
	}
}
