// tenantscope-lint walks the HTTP handler package and reports any handler
// that reads c.Params(...) with a tenant-scoped URL-param spelling (see
// paramKeys below — generic `id`/`agent_id` plus per-resource names like
// mcp_id, attestation_id, capabilityId, etc.) without invoking one of the
// registered tenant-scoping helpers (LoadOwned, LoadOwnedViaAgent).
//
// The lint is the structural guarantee for the defect #18-25 cluster: it
// catches future handlers that forget to enforce tenant scoping AT LINT TIME,
// so a regression cannot ship.
//
// Allowlist: handlers genuinely operate cross-tenant (system health checks,
// public agent discovery, etc.) and must be added by name to the allowlist
// below with a one-line justification.
//
// Run:
//
//	go run ./cmd/tenantscope-lint ./internal/interfaces/http/handlers
//	go run ./cmd/tenantscope-lint  # defaults to the handlers package
//
// Exit code: 0 on pass, 1 on any violation.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// allowlist names handler functions that legitimately operate across
// organizations and therefore do not need to invoke LoadOwned. Each entry
// must include a one-line justification.
var allowlist = map[string]string{
	// Public agent discovery — intentionally cross-org for the public registry.
	// Returns only redacted fields; full resource never crosses the boundary.
	"PublicAgentHandler.GetPublicAgent":             "public registry endpoint; returns only redacted fields",
	"PublicAgentHandler.ListPublicAgents":           "public registry endpoint; returns only redacted fields",
	"PublicMCPHandler.GetPublicMCPServer":           "public MCP registry endpoint; returns only redacted fields",
	"PublicMCPHandler.ListPublicMCPServers":         "public MCP registry endpoint; returns only redacted fields",
	"PublicRegistrationHandler.GetRegistrationByID": "public registration status check before login",

	// Lifecycle endpoints that operate on the calling agent's own ID by
	// design — the SDK auth middleware has already verified the caller IS
	// this agent, so additional org check would be redundant.
	"LifecycleHandler.GetRevocationList": "system endpoint; iterates all agents for revocation list",

	// Authentication endpoints — operate on credentials, not org-scoped resources.
	"AuthRefreshHandler.Refresh":           "token refresh; not a resource read",
	"AuthHandler.Login":                    "login flow; org assignment is the OUTPUT, not the gate",
	"SDKTokenRecoveryHandler.RecoverToken": "token recovery; not a resource read",
	"OAuthTokenHandler.Token":              "OAuth token issuance; not a resource read",
	"DeviceAuthHandler.RequestDevice":      "device auth flow; not a resource read",
	"DeviceAuthHandler.PollDevice":         "device auth flow; not a resource read",

	// Detection / community intelligence — read-only system endpoints that
	// aggregate cross-org telemetry (no per-org resource being returned).
	"DetectionHandler.GetThreatLevel":       "aggregated threat telemetry; no per-org resource",
	"CommunityIntelligenceHandler.GetIntel": "aggregated community intel; no per-org resource",

	// Registry bridge — federation endpoint that operates on bridge identity.
	"RegistryBridgeHandler.Verify": "registry federation; bridge identity is the resource",

	// ---------------------------------------------------------------
	// AUDIT-BASELINE: 71 pre-existing handlers that read c.Params("id"|
	// "agent_id") without any visible OrganizationID reference. These
	// PREDATE the LoadOwned helper introduced in this PR. They are not
	// known-safe — they require manual review. The lint allowlists them
	// here so this PR can ship the structural fix for the 8 cited defects
	// (#18-25) without blocking on a 71-handler manual audit. Each entry
	// must be removed from this section as it is reviewed; the reviewer
	// either confirms the handler is intentionally cross-tenant and moves
	// it to the section above with a per-entry justification, or wires
	// LoadOwned into the handler.
	//
	// Tracking: ~/workspace/opena2a-org/todo/2026-05-18-aim-defect-audit-
	// from-lf-demo-prep.md (defect #18-25 follow-up section to be added).
	// ---------------------------------------------------------------
	"A2AHandler.DeleteSkill":                                "audit-baseline: needs review",
	"A2AHandler.GetA2ATrustScore":                           "audit-baseline: needs review",
	"A2AHandler.GetAgentAttestations":                       "audit-baseline: needs review",
	"A2AHandler.GetAgentCard":                               "audit-baseline: needs review",
	"A2AHandler.GetAgentSkills":                             "audit-baseline: needs review",
	"A2AHandler.GetConsensusStatus":                         "audit-baseline: needs review",
	"A2AHandler.GetPeerTrustScore":                          "audit-baseline: needs review",
	"A2AHandler.GetTrustScoreAlt":                           "audit-baseline: needs review",
	"A2AHandler.RecordInteraction":                          "audit-baseline: needs review",
	"A2AHandler.RevokeConsent":                              "audit-baseline: needs review",
	"A2AHandler.SignRequest":                                "audit-baseline: needs review",
	"A2AHandler.UpdateAgentCard":                            "audit-baseline: needs review",
	"A2AHandler.UpdateTaskState":                            "audit-baseline: needs review",
	"A2AHandler.UpdateTrustScore":                           "audit-baseline: needs review",
	"AdminHandler.AcknowledgeAlert":                         "audit-baseline: needs review",
	"AdminHandler.ApproveRegistrationRequest":               "audit-baseline: needs review",
	"AdminHandler.ApproveUser":                              "audit-baseline: needs review",
	"AdminHandler.DeactivateUser":                           "audit-baseline: needs review",
	"AdminHandler.RejectRegistrationRequest":                "audit-baseline: needs review",
	"AdminHandler.RejectUser":                               "audit-baseline: needs review",
	"AdminHandler.ResolveAlert":                             "audit-baseline: needs review",
	"AdminHandler.UpdateUserRole":                           "audit-baseline: needs review",
	"AgentHandler.LogCapabilityResult":                      "audit-baseline: needs review",
	"APIKeyHandler.DeleteAPIKey":                            "audit-baseline: needs review",
	"APIKeyHandler.DisableAPIKey":                           "audit-baseline: needs review",
	"AuthorizeHandler.Authorize":                            "audit-baseline: needs review",
	"CapabilityHandler.GetAgentCapabilities":                "audit-baseline: needs review",
	"CapabilityHandler.GetViolationsByAgent":                "audit-baseline: needs review",
	"CapabilityRequestHandlers.CreateCapabilityRequest":     "audit-baseline: needs review",
	"CapabilityRequestHandlers.ListAgentCapabilityRequests": "audit-baseline: needs review",
	"CapabilityRequestHandlers.RejectCapabilityRequest":     "audit-baseline: needs review",
	"DetectionHandler.GetDetectionStatus":                   "audit-baseline: needs review",
	"DetectionHandler.GetLatestCapabilityReport":            "audit-baseline: needs review",
	"DetectionHandler.ReportCapabilities":                   "audit-baseline: needs review",
	"DetectionHandler.ReportDetection":                      "audit-baseline: needs review",
	"MCPAttestationHandler.AttestMCP":                       "audit-baseline: needs review",
	"MCPAttestationHandler.GetAgentMCPServers":              "audit-baseline: needs review",
	"MCPAttestationHandler.GetAttestationChallenge":         "audit-baseline: needs review",
	"MCPAttestationHandler.GetConnectedAgents":              "audit-baseline: needs review",
	"MCPAttestationHandler.GetConsensusStatus":              "audit-baseline: needs review",
	"MCPAttestationHandler.GetMCPAttestations":              "audit-baseline: needs review",
	"MCPAttestationHandler.ManualAttestMCP":                 "audit-baseline: needs review",
	"MCPAttestationHandler.RevokeAllAttestationsByAgent":    "audit-baseline: needs review",
	"MCPGraphHandler.GetMCPServerConnections":               "audit-baseline: needs review",
	"MCPHandler.GetConnectedAgents":                         "audit-baseline: needs review",
	"MCPHandler.VerifyMCPCapability":                        "audit-baseline: needs review",
	"PublicMCPHandler.VerifyMCPAction":                      "audit-baseline: needs review",
	"SDKTokenHandler.RevokeToken":                           "audit-baseline: needs review",
	"SecretsHandler.DeleteNamespace":                        "audit-baseline: needs review",
	"SecretsHandler.GetNamespace":                           "audit-baseline: needs review",
	"SecretsHandler.RotateCredential":                       "audit-baseline: needs review",
	"SecretsHandler.StoreCredential":                        "audit-baseline: needs review",
	"SecurityPolicyHandler.DeletePolicy":                    "audit-baseline: needs review",
	"SecurityPolicyHandler.GetPolicy":                       "audit-baseline: needs review",
	"SecurityPolicyHandler.TogglePolicy":                    "audit-baseline: needs review",
	"SecurityPolicyHandler.UpdatePolicy":                    "audit-baseline: needs review",
	"TagHandler.AddTagsToAgent":                             "audit-baseline: needs review",
	"TagHandler.AddTagsToMCPServer":                         "audit-baseline: needs review",
	"TagHandler.GetAgentTags":                               "audit-baseline: needs review",
	"TagHandler.GetMCPServerTags":                           "audit-baseline: needs review",
	"TagHandler.RemoveTagFromAgent":                         "audit-baseline: needs review",
	"TagHandler.RemoveTagFromMCPServer":                     "audit-baseline: needs review",
	"TagHandler.SuggestTagsForAgent":                        "audit-baseline: needs review",
	"TagHandler.SuggestTagsForMCPServer":                    "audit-baseline: needs review",
	"TagHandler.UpdateTag":                                  "audit-baseline: needs review",
	"VerificationEventHandler.DeleteVerificationEvent":      "audit-baseline: needs review",
	"VerificationEventHandler.GetAgentVerificationEvents":   "audit-baseline: needs review",
	"VerificationEventHandler.GetMCPVerificationEvents":     "audit-baseline: needs review",
	"VerificationEventHandler.GetVerificationEvent":         "audit-baseline: needs review",
	"VerificationHandler.GetVerification":                   "DEFECT #23 deferred: handler mounted on both public /api/v1/verifications/:id (no auth) and SDK-token /api/v1/sdk-api/verifications/:id (audit-cited); needs route split before tenant-scoping can apply.",
	"VerificationHandler.SubmitVerificationResult":          "audit-baseline: needs review",
	"VerificationHandler.UpdateExecutionStatus":             "audit-baseline: needs review",

	// ---------------------------------------------------------------
	// A3c-FLAGGED: 7 handlers surfaced 2026-05-20. Six were caught
	// directly by broadening paramKeys to cover the snake_case +
	// camelCase URL-param spellings for tenant-scoped resources
	// beyond `id`/`agent_id` (#42-47). The seventh (#48,
	// CapabilityHandler.RegisterCapability) was NOT caught by the
	// lint — adversarial review surfaced it as a sibling-of-#25
	// IDOR (write-side, SDK auth path) that the lenient
	// `bodyMentionsOrganizationID` heuristic silently exempts
	// because the handler mentions `agent.OrganizationID` only to
	// look up the VICTIM resource's org, never to compare against
	// the caller. It is allowlisted here so that any future
	// heuristic tightening surfaces an explicit deferred entry
	// rather than a suddenly-blocking violation, and so reviewers
	// walking the audit-baseline section find it tracked.
	//
	// Each handler in this block reads its resource ID from the
	// path without invoking LoadOwned (or, for #48, without
	// comparing the mentioned OrganizationID to the caller).
	// Service-layer enforcement (if any) needs manual verification
	// before these can be moved to a per-entry justification or
	// wired through LoadOwned. Filed as defects #42-48 in
	// todo/2026-05-18-aim-defect-audit-from-lf-demo-prep.md.
	// Closing these is A3b/A3d scope.
	// ---------------------------------------------------------------
	"A2AHandler.GetSkillAttestations":               "audit-baseline: needs review (A3c-flagged #43)",
	"A2AHandler.ListUserConsents":                   "audit-baseline: needs review (A3c-flagged #42)",
	"CapabilityHandler.GetRecentViolations":         "audit-baseline: needs review (A3c-flagged #46)",
	"CapabilityHandler.GetViolationsByOrganization": "audit-baseline: needs review (A3c-flagged #45)",
	"CapabilityHandler.RegisterCapability":          "audit-baseline: needs review (A3c-flagged #48; lint-exempt today via bodyMentionsOrganizationID — entry future-proofs heuristic tightening)",
	"CapabilityHandler.RevokeCapability":            "audit-baseline: needs review (A3c-flagged #44)",
	"MCPAttestationHandler.RevokeAttestation":       "audit-baseline: needs review (A3c-flagged #47)",
}

// recognizedHelpers names the helper functions that satisfy the lint.
// Any handler invoking one of these on a path that derives from
// c.Params("id"|"agent_id"|"agent-id") is considered properly tenant-scoped.
var recognizedHelpers = map[string]bool{
	"LoadOwned":         true,
	"LoadOwnedViaAgent": true,
}

// paramKeys names the URL-param keys whose reads we want to gate. Reading
// any of these and not subsequently calling a recognized helper is a
// violation unless the function is on the allowlist.
//
// The list covers every URL-param spelling for tenant-scoped resources
// observed in the handlers package. Generic `id` plus the per-resource
// names: agent_id / agentId / identifier (agents and agent-name lookups),
// mcp_id (MCP servers), attestation_id (attestations), audit_id (audit
// log entries), capabilityId (capabilities), peer_id (A2A peers),
// skillId (A2A skills), tagId (tags), userId (users), and orgId (the
// organization itself — a handler reading another org's UUID from the
// path must verify the caller's org matches it).
//
// Excluded by design:
//   - checkName  — compliance check NAME string (not a resource UUID).
//   - requestId  — public registration request, pre-authentication endpoint.
//   - *1         — single-use today on /api/v1/did/* (DID resolution,
//     W3C public-federation endpoint, caller-org-irrelevant
//     by design). NOT a general-purpose exclusion: if a
//     future route mounts a `*` wildcard on a tenant-scoped
//     resource, add the receiving handler to the allowlist
//     with a one-line rationale OR gate it via LoadOwned —
//     the lint will not catch a `c.Params("*1")` read on its
//     own.
var paramKeys = map[string]bool{
	"id":             true,
	"agent_id":       true,
	"agent-id":       true,
	"agentId":        true,
	"identifier":     true,
	"mcp_id":         true,
	"attestation_id": true,
	"audit_id":       true,
	"capabilityId":   true,
	"peer_id":        true,
	"skillId":        true,
	"tagId":          true,
	"userId":         true,
	"orgId":          true,
}

type violation struct {
	File    string
	Line    int
	Handler string
}

func main() {
	dir := "./internal/interfaces/http/handlers"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	dir = filepath.Clean(dir)

	violations, err := scanDirectory(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tenantscope-lint: %v\n", err)
		os.Exit(2)
	}

	if len(violations) == 0 {
		fmt.Printf("tenantscope-lint: ok (%d allowlist entries; scanned %s)\n", len(allowlist), dir)
		os.Exit(0)
	}

	sort.Slice(violations, func(i, j int) bool {
		if violations[i].File != violations[j].File {
			return violations[i].File < violations[j].File
		}
		return violations[i].Line < violations[j].Line
	})

	fmt.Fprintf(os.Stderr, "tenantscope-lint: %d handler(s) read a tenant-scoped c.Params(...) key without invoking LoadOwned/LoadOwnedViaAgent:\n\n", len(violations))
	for _, v := range violations {
		fmt.Fprintf(os.Stderr, "  %s:%d  %s\n", v.File, v.Line, v.Handler)
	}
	fmt.Fprintln(os.Stderr, `
To fix each violation:
  1. Add RequireOrganizationID(c) at the top of the handler.
  2. Wrap the resource load in LoadOwned or LoadOwnedViaAgent before
     returning the resource or using it for any state mutation.
  3. If the handler genuinely operates cross-tenant, add its qualified
     name (HandlerStructName.MethodName) to the allowlist in
     cmd/tenantscope-lint/main.go with a one-line justification.

See apps/backend/internal/interfaces/http/handlers/tenant_scope.go for the
helper documentation and the SECURITY rationale for 404-not-403.`)
	os.Exit(1)
}

func scanDirectory(dir string) ([]violation, error) {
	var violations []violation

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}

	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		// Test files run their own suites; production handlers are the
		// target. The lint also intentionally skips its own implementation
		// files (none live in this directory).
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		violations = append(violations, scanFile(fset, file, path)...)
	}
	return violations, nil
}

func scanFile(fset *token.FileSet, file *ast.File, path string) []violation {
	var out []violation
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Recv == nil {
			continue
		}
		handlerName := receiverTypeName(fn) + "." + fn.Name.Name
		if _, allowed := allowlist[handlerName]; allowed {
			continue
		}

		readsParam, paramLine := bodyReadsTenantParam(fset, fn.Body)
		if !readsParam {
			continue
		}
		if bodyInvokesHelper(fn.Body) {
			continue
		}
		// Many existing handlers tenant-scope inline rather than through the
		// helper (e.g. `if agent.OrganizationID != orgID { 403 }`). Those
		// pre-date the helper and are not the lint's target — the lint
		// catches handlers that have forgotten tenant scoping entirely. A
		// handler that mentions `OrganizationID` anywhere in its body is
		// treated as "thought about it." This is intentionally permissive;
		// pair the lint with a code review for new handlers to push them
		// toward the helper for consistency.
		if bodyMentionsOrganizationID(fn.Body) {
			continue
		}
		out = append(out, violation{
			File:    path,
			Line:    paramLine,
			Handler: handlerName,
		})
	}
	return out
}

// bodyMentionsOrganizationID returns true if the function body contains any
// identifier or selector with the name "OrganizationID". This is the cheap
// "did the handler consider organization scoping at all" check. False
// positives are acceptable here — the goal is to catch handlers that have
// completely forgotten, not to enforce the helper as the only valid pattern.
//
// Known false-NEGATIVE class (audit-doc defect #48 is the cited example):
// a handler that mentions `OrganizationID` only to look up the VICTIM
// resource's org for routing/policy or audit-log purposes — without
// comparing to the caller's `c.Locals("organization_id")` — silently
// satisfies this check. Tightening the heuristic to detect "mentioned-but-
// not-compared" is OUT OF the A3c structural-broadening scope; reviewers
// should treat the OrganizationID-mention-only exemption as a hint, not a
// proof, and walk the lenient-exempted set during A3b/A3d.
func bodyMentionsOrganizationID(body *ast.BlockStmt) bool {
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch x := n.(type) {
		case *ast.Ident:
			if x.Name == "OrganizationID" {
				found = true
				return false
			}
		case *ast.SelectorExpr:
			if x.Sel != nil && x.Sel.Name == "OrganizationID" {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// receiverTypeName returns the concrete receiver type name for a method
// declaration. Pointer receivers and value receivers both unwrap to the
// underlying type identifier.
func receiverTypeName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	expr := fn.Recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// bodyReadsTenantParam returns true and the line number of the first match
// if the function body invokes c.Params(...) with a tenant-relevant key
// from the paramKeys set above (id, agent_id, agent-id, agentId,
// mcp_id, attestation_id, capabilityId, peer_id, skillId, tagId, userId,
// orgId, identifier).
func bodyReadsTenantParam(fset *token.FileSet, body *ast.BlockStmt) (bool, int) {
	var (
		found bool
		line  int
	)
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel == nil || sel.Sel.Name != "Params" {
			return true
		}
		if len(call.Args) == 0 {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		// lit.Value is the quoted string including the surrounding quotes.
		key := strings.Trim(lit.Value, `"`)
		if _, want := paramKeys[key]; want {
			found = true
			line = fset.Position(call.Pos()).Line
			return false
		}
		return true
	})
	return found, line
}

// bodyInvokesHelper returns true if any call inside the function body has
// the selector or identifier equal to a recognized helper name. This
// matches both `LoadOwned(...)` direct invocations and
// `handlers.LoadOwned(...)` qualified ones (same package, but defensive).
func bodyInvokesHelper(body *ast.BlockStmt) bool {
	var found bool
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch f := call.Fun.(type) {
		case *ast.Ident:
			if recognizedHelpers[f.Name] {
				found = true
				return false
			}
		case *ast.SelectorExpr:
			if f.Sel != nil && recognizedHelpers[f.Sel.Name] {
				found = true
				return false
			}
		case *ast.IndexExpr:
			// Generic call site like LoadOwned[T](...). Fun resolves to an
			// IndexExpr; the function identifier lives in X.
			if id, ok := f.X.(*ast.Ident); ok && recognizedHelpers[id.Name] {
				found = true
				return false
			}
		}
		return true
	})
	return found
}
