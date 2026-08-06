package main

import (
	"os"
	"sort"
	"strings"
	"testing"
)

// TestScanDirectoryFlagsExpectedHandlers exercises scanDirectory on a
// pair of fixtures under testdata/ and asserts that the violating
// fixture is flagged while the clean fixture is not.
//
// The fixtures live under testdata/ so the Go toolchain ignores them
// during a regular build. The lint parses them via go/parser without
// needing them to compile.
func TestScanDirectoryFlagsExpectedHandlers(t *testing.T) {
	violations, err := scanDirectory("testdata")
	if err != nil {
		t.Fatalf("scanDirectory(testdata): %v", err)
	}

	gotHandlers := make([]string, 0, len(violations))
	for _, v := range violations {
		gotHandlers = append(gotHandlers, v.Handler)
	}
	sort.Strings(gotHandlers)

	wantHandlers := []string{
		"LintTestViolatingHandler.GetThing",
	}

	if !equal(gotHandlers, wantHandlers) {
		t.Fatalf("scanDirectory(testdata) flagged %v, want %v",
			gotHandlers, wantHandlers)
	}

	for _, v := range violations {
		if !strings.HasSuffix(v.File, "violating.go") {
			t.Errorf("violation in unexpected file: %s", v.File)
		}
		if v.Line == 0 {
			t.Errorf("violation has zero line number: %+v", v)
		}
	}
}

// TestParamKeysCoversTenantScopedResources is a structural guard
// against accidentally removing one of the URL-param spellings the
// A3c broadening committed to gating. If a key in the want set
// disappears, this test fails — keeping the lint's scope honest
// without depending on the much larger handlers corpus.
func TestParamKeysCoversTenantScopedResources(t *testing.T) {
	mustHave := []string{
		// PR #136 originals.
		"id", "agent_id", "agent-id",
		// A3c additions.
		"agentId", "identifier", "mcp_id", "attestation_id",
		"audit_id", "capabilityId", "peer_id", "skillId",
		"tagId", "userId", "orgId",
	}
	for _, key := range mustHave {
		if _, ok := paramKeys[key]; !ok {
			t.Errorf("paramKeys is missing %q", key)
		}
	}
}

// TestRecognizedHelpersUnchanged is a structural guard against
// silently dropping a helper recognition. A3c does not add helpers,
// but a regression here would change which handlers count as scoped.
func TestRecognizedHelpersUnchanged(t *testing.T) {
	mustHave := []string{"LoadOwned", "LoadOwnedViaAgent"}
	for _, name := range mustHave {
		if !recognizedHelpers[name] {
			t.Errorf("recognizedHelpers is missing %q", name)
		}
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestScanServiceDirectoryFlagsAcceptedButUnusedOrgID runs the new
// class-#3 service-param scan against the testdata fixtures. The
// violating fixture (service_violating.go) MUST be flagged; the clean
// fixtures (service_clean.go and the handler files) MUST NOT.
func TestScanServiceDirectoryFlagsAcceptedButUnusedOrgID(t *testing.T) {
	violations, err := scanServiceDirectory("testdata")
	if err != nil {
		t.Fatalf("scanServiceDirectory(testdata): %v", err)
	}

	gotMethods := make([]string, 0, len(violations))
	for _, v := range violations {
		gotMethods = append(gotMethods, v.Handler)
	}
	sort.Strings(gotMethods)

	wantMethods := []string{
		"LintTestViolatingSelectorService.AcknowledgeAlert (param OrganizationID)",
		"LintTestViolatingService.AcknowledgeAlert (param orgID)",
	}

	if !equal(gotMethods, wantMethods) {
		t.Fatalf("scanServiceDirectory(testdata) flagged %v, want %v",
			gotMethods, wantMethods)
	}

	for _, v := range violations {
		if !strings.HasSuffix(v.File, "service_violating.go") &&
			!strings.HasSuffix(v.File, "service_violating_selector.go") {
			t.Errorf("violation in unexpected file: %s", v.File)
		}
		if v.Line == 0 {
			t.Errorf("violation has zero line number: %+v", v)
		}
	}
}

// TestOrgParamNamesCoversCommonSpellings is a structural guard against
// silently dropping a parameter-name spelling. If callsites in this
// codebase introduce a new spelling, add it here AND walk the existing
// service surface for references.
func TestOrgParamNamesCoversCommonSpellings(t *testing.T) {
	mustHave := []string{"orgID", "OrgID", "organizationID", "OrganizationID", "adminOrgID", "callerOrgID"}
	for _, name := range mustHave {
		if !orgParamNames[name] {
			t.Errorf("orgParamNames is missing %q", name)
		}
	}
}

// TestClassifyServiceDirRouting confirms the path-classifier sends
// `./internal/application` to the service scanner and any other path
// (including the default handlers path) to the handler scanner.
func TestClassifyServiceDirRouting(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"./internal/application", true},
		{"internal/application", true},
		{"/abs/path/to/internal/application", true},
		{"./internal/interfaces/http/handlers", false},
		{"./cmd/tenantscope-lint", false},
		{".", false},
	}
	for _, c := range cases {
		if got := classifyServiceDir(c.path); got != c.want {
			t.Errorf("classifyServiceDir(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// Every allowlist key must name a method that actually exists.
//
// An entry that resolves to nothing exempts nothing, and it is worse than a missing
// entry: it reads as a reviewed, justified exemption while the handler it was meant to
// cover is either unexempted or named something else. Fourteen of the then-32 entries
// were in that state — `AuthHandler.Login` (the type has `LocalLogin`; the real `Login`
// is on `PublicRegistrationHandler`), `RegistryBridgeHandler.Verify` (the type has only
// `TriggerPush`), `OAuthTokenHandler.Token` (it is `HandleTokenRequest`), and eleven
// more. All fourteen were removed and the lint still passes, which is the evidence that
// none of the real handlers needed exempting.
//
// This runs the real scan over the real handlers package rather than a fixture, because
// the property under test is agreement between the allowlist and the shipped code — a
// fixture cannot express that.
func TestAllowlistKeysAllResolveToRealHandlers(t *testing.T) {
	const handlersDir = "../../internal/interfaces/http/handlers"

	if _, err := os.Stat(handlersDir); err != nil {
		t.Fatalf("handlers package not found at %s: %v", handlersDir, err)
	}

	// Populates discoveredHandlers as a side effect.
	if _, err := scanDirectory(handlersDir); err != nil {
		t.Fatalf("scanDirectory: %v", err)
	}
	if len(discoveredHandlers) == 0 {
		t.Fatal("scan discovered no handlers at all; this test would pass vacuously")
	}

	if unresolved := checkAllowlistResolves(); len(unresolved) > 0 {
		t.Errorf("%d allowlist entr(ies) name a method that does not exist: %v\n"+
			"Each exempts nothing. Correct the key to the real Receiver.Method and keep "+
			"the justification, or delete the entry.", len(unresolved), unresolved)
	}
}

// The check above must be capable of failing. Without this, a bug that left
// discoveredHandlers over-populated would make it pass for every possible allowlist.
func TestAllowlistResolutionCheckDetectsAMissingMethod(t *testing.T) {
	discoveredHandlers = map[string]bool{"RealHandler.RealMethod": true}
	t.Cleanup(func() { discoveredHandlers = map[string]bool{} })

	original := allowlist
	allowlist = map[string]string{
		"RealHandler.RealMethod":     "exists",
		"GhostHandler.VanishedThing": "does not exist",
	}
	t.Cleanup(func() { allowlist = original })

	unresolved := checkAllowlistResolves()

	if len(unresolved) != 1 || unresolved[0] != "GhostHandler.VanishedThing" {
		t.Errorf("expected exactly the nonexistent key to be reported, got %v", unresolved)
	}
}
