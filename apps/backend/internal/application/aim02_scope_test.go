package application

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AIM-02 — bounded scope, held as tests rather than as a promise in a PR body.
//
// AIM-02 ships two read-side gates on trust factor 9 and the column a future
// verifier will write into. Three things it deliberately does NOT do, each of
// which is easy to do by accident while in the neighbourhood:
//
//	no verified write path — the ceiling is worth nothing if anything can set
//	                         verified = true, so nothing may, in any layer.
//	signing keys untouched — keyvault.go and KEYVAULT_MASTER_KEY are adjacent to
//	                         "attestation" by vocabulary only.
//	route mismatch left alone — both SDKs POST /isolation-attestation while the
//	                         backend registered /isolation, so the reporting
//	                         surface 404'd. That was CA's lane; fixing it here
//	                         would have folded an unrelated behaviour change into
//	                         a scoring change and made both harder to review.
//	                         CLOSED by AIM-03 ([CHIEF-CA] 2026-08-29 ruling): the
//	                         backend now registers the canonical SDK path and
//	                         keeps /isolation as a deprecated alias, so the
//	                         assertion that the mismatch is STILL BROKEN was
//	                         deleted by the lane that fixed it, as its own
//	                         comment said it would be. The parity guard lives at
//	                         apps/backend/cmd/server/aim03_sdk_api_routes_test.go.
//
// These are source-level assertions. They are coarse by design: a scan cannot
// prove the absence of every conceivable write, but it does catch the shapes a
// later change would actually take, and it fails loudly next to the code it
// constrains rather than in a reviewer's memory.

// aim02RepoRoot resolves the repository root from this file's own location, so
// the scan does not depend on the working directory `go test` was invoked from.
func aim02RepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller must resolve this test file's path")

	dir := filepath.Dir(thisFile)
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "apps")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate repository root above %s", thisFile)
	return ""
}

func aim02ReadFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	require.NoError(t, err, "expected %s to exist", rel)
	return string(b)
}

// aim02StripLineComments removes `//` and `--` line comments so that prose
// ABOUT a write ("nothing sets verified = TRUE") is not mistaken for a write.
// It is not a parser: a `//` inside a string literal would be stripped too.
// That direction is safe here — it can only hide a violation that is inside a
// string, and no SQL this repo issues is assembled that way.
func aim02StripLineComments(src string) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// verificationWrites are the shapes a write of `verified` would take in Go or
// in SQL. The struct-literal and assignment forms cover the Go side; the SQL
// forms cover a bound parameter, a TRUE literal, and an UPDATE of the table.
var verificationWrites = []*regexp.Regexp{
	regexp.MustCompile(`(?i)Verified\s*:\s*true`),
	regexp.MustCompile(`(?i)\.Verified\s*=\s*true`),
	regexp.MustCompile(`(?i)VerifiedBy\s*:\s*[^n\s]`), // anything but nil ([^n] alone eats the space after the colon and flags `VerifiedBy: nil` itself)
	regexp.MustCompile(`(?i)\bverified\s*=\s*(true|\$\d+)`),
	regexp.MustCompile(`(?i)UPDATE\s+isolation_attestations`),
}

func TestAIM02_BoundedScope(t *testing.T) {
	root := aim02RepoRoot(t)

	t.Run("AIM-02.AC4 no backend source writes verified on an isolation attestation", func(t *testing.T) {
		// Scan every non-test Go and SQL file in the backend that mentions the
		// isolation attestation at all. Test files are excluded on purpose:
		// they construct verified rows to exercise the READ side, which is the
		// only way to prove the read side treats them differently.
		var offenders []string
		backend := filepath.Join(root, "apps", "backend")

		err := filepath.Walk(backend, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if info.Name() == "node_modules" || info.Name() == "vendor" {
					return filepath.SkipDir
				}
				return nil
			}
			ext := filepath.Ext(path)
			if ext != ".go" && ext != ".sql" {
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			src := string(raw)
			if !strings.Contains(src, "IsolationAttestation") && !strings.Contains(src, "isolation_attestations") {
				return nil
			}
			body := aim02StripLineComments(src)
			for _, re := range verificationWrites {
				if loc := re.FindString(body); loc != "" {
					rel, _ := filepath.Rel(root, path)
					offenders = append(offenders, rel+": "+strings.TrimSpace(loc))
				}
			}
			return nil
		})
		require.NoError(t, err)

		assert.Empty(t, offenders,
			"no write path may set verified on an isolation attestation: the unverified ceiling "+
				"is the only thing bounding a self-report, and a single writable TRUE returns the "+
				"full 1.0 it exists to deny. TEE attestation and orchestrator metadata may write "+
				"this in Phase 2; an HMA static scan may not, and the SDK never.")
	})

	t.Run("AIM-02.AC4 the isolation path does not touch signing-key handling", func(t *testing.T) {
		// Bounded scope in both directions: the isolation change must not reach
		// into the key vault, and the key vault must not have grown an
		// isolation dependency while this landed.
		for _, rel := range []string{
			"apps/backend/internal/domain/isolation.go",
			"apps/backend/internal/application/trust_calculator.go",
			"apps/backend/internal/infrastructure/repository/isolation_attestation_repository.go",
			"apps/backend/migrations/108_add_isolation_attestation_verification.sql",
		} {
			src := strings.ToLower(aim02ReadFile(t, root, rel))
			assert.NotContains(t, src, "keyvault", "%s must not reach into signing-key handling", rel)
			assert.NotContains(t, src, "master_key", "%s must not reach into signing-key handling", rel)
		}

		keyvault := strings.ToLower(aim02ReadFile(t, root, "apps/backend/internal/crypto/keyvault.go"))
		assert.NotContains(t, keyvault, "isolationattestation",
			"keyvault.go is out of scope for AIM-02 and must not have acquired an isolation dependency")
	})

	t.Run("AIM-02.AC4 the user-facing surfaces describe the ceiling and the expiry", func(t *testing.T) {
		// A scoring change an operator cannot see is a scoring change they will
		// file a bug about. Each surface has to say what the factor now does.
		breakdown := aim02ReadFile(t, root, "apps/web/components/agent/trust-score-breakdown.tsx")
		assert.Contains(t, breakdown, "0.65",
			"the breakdown copy must name the ceiling an unverified report is held to")
		assert.Contains(t, breakdown, "90 days",
			"the breakdown copy must name the expiry")

		doc := aim02ReadFile(t, root, "docs/sdk/trust-scoring.md")
		assert.Contains(t, doc, "0.65", "factor 9 documentation must name the ceiling")
		assert.Contains(t, doc, "90-day", "factor 9 documentation must name the expiry")

		changelog := aim02ReadFile(t, root, "CHANGELOG.md")
		assert.Contains(t, changelog, "isolation_attestations",
			"the changelog must record the schema change")
		assert.Contains(t, changelog, "90-day", "the changelog must record the expiry")
	})

	t.Run("AIM-02.AC4 no deployment artifact carries this change", func(t *testing.T) {
		// AIM-02 is code and schema only; the prod deploy for
		// aim-cloud/agent-identity-management is gated in another lane. If a
		// deployment manifest names anything this change introduced, something
		// was shipped from here that should not have been.
		markers := []string{
			"UnverifiedIsolationCeiling",
			"IsolationAttestationTTL",
			"108_add_isolation_attestation_verification",
		}
		for _, dir := range []string{"infrastructure", filepath.Join("apps", "backend", "deployments")} {
			base := filepath.Join(root, dir)
			if _, err := os.Stat(base); os.IsNotExist(err) {
				continue
			}
			err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				raw, readErr := os.ReadFile(path)
				if readErr != nil {
					return nil // binary or unreadable: not a manifest we care about
				}
				for _, m := range markers {
					if strings.Contains(string(raw), m) {
						rel, _ := filepath.Rel(root, path)
						t.Errorf("deployment file %s references %q; AIM-02 ships no deploy artifact", rel, m)
					}
				}
				return nil
			})
			require.NoError(t, err)
		}
	})
}
