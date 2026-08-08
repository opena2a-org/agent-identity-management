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
	// The federated revocation list. Unauthenticated and cross-organization by
	// design: a verifier must be able to check revocation WITHOUT holding a
	// credential, which is what makes offline verification possible. The control
	// is minimization, not scoping — the payload carries an agent id and a closed
	// reason code and nothing else.
	//
	// This entry previously justified itself with "the SDK auth middleware has
	// already verified the caller IS this agent" and "iterates all agents". Both
	// were wrong: the route is registered on the v1 group with a rate limiter and
	// no auth, and the handler iterated nothing because it asked the repository
	// for LIMIT 0. It was the only place this endpoint's cross-org nature was ever
	// reasoned about, and it was wrong about the code in both directions.
	"LifecycleHandler.GetRevocationList": "unauthenticated federated CRL; cross-org by design, minimized to agentId + closed-set reason (see GetRevocationList doc comment)",

	// Scoped, but not through LoadOwned — each verified individually. The lint recognises
	// one mechanism; these use others, and "does not call the helper" is not "unscoped".
	"A2AHandler.ListTasks":          "org-scoped in SQL: the handler passes RequireOrganizationID(c) to ListA2ATasks, and the repository query requires a row whose client or remote agent belongs to that org (a2a_tasks has no organization_id to filter on directly)",
	"A2AHandler.GetPublicAgentCard": "intentionally public: serves /.well-known/agent.json, whose whole purpose is to be readable without credentials",
	"APIKeyHandler.ListAPIKeys":     "org-scoped at the service layer: orgID comes from Locals and is passed to ListAPIKeys(ctx, orgID); the agent_id query parameter only filters that already-scoped result set in Go, so a foreign id yields an empty filter rather than a foreign key",

	// ---------------------------------------------------------------
	// AUDIT-BASELINE: pre-existing handlers that read c.Params("id"|
	// "agent_id") without any visible OrganizationID reference. These
	// PREDATE the LoadOwned helper. They are not known-safe — they require
	// manual review. The lint allowlists them here so the structural fix for
	// the 8 cited defects (#18-25) could ship without blocking on the audit.
	// Each entry must be removed from this section as it is reviewed; the
	// reviewer either confirms the handler is intentionally cross-tenant and
	// moves it to the section above with a per-entry justification, or wires
	// LoadOwned into the handler. "needs review" is not an end state.
	//
	// An earlier version of this comment claimed 71 handlers. This map has
	// never held that many — verify the real number rather than restating
	// one, since a count in a comment cannot be checked by anything:
	//
	//	grep -c "audit-baseline: needs review" cmd/tenantscope-lint/main.go
	//
	// Tracking: opena2a-org/agent-identity-management#358.
	// ---------------------------------------------------------------
	"A2AHandler.DeleteSkill":                       "stub-handler: returns 204 No Content with no service dispatch. Path :id is a skill UUID; once a DeleteSkill service method exists, scope at handler layer (A3d-vii.c follow-up). Not exploitable today.",
	"AdminHandler.AcknowledgeAlert":                "service-layer scoping: AlertService.AcknowledgeAlert performs Load → caller-org check → ErrAlertNotFound (collapses cross-tenant + not-found + uuid.Nil mismatch) and the handler maps the sentinel to a fixed 404 body (A3d-v R7 closed in PR #190).",
	"AdminHandler.ResolveAlert":                    "service-layer scoping: AlertService.ResolveAlert mirrors AcknowledgeAlert — Load → caller-org check → ErrAlertNotFound → fixed 404 handler mapping (A3d-v R7 closed in PR #190).",
	"DetectionHandler.GetDetectionStatus":          "audit-baseline: needs review",
	"DetectionHandler.GetLatestCapabilityReport":   "audit-baseline: needs review",
	"DetectionHandler.ReportCapabilities":          "audit-baseline: needs review",
	"DetectionHandler.ReportDetection":             "audit-baseline: needs review",
	"MCPGraphHandler.GetMCPServerConnections":      "audit-baseline: needs review",
	"MCPHandler.GetConnectedAgents":                "audit-baseline: needs review",
	"MCPHandler.VerifyMCPCapability":               "audit-baseline: needs review",
	"PublicMCPHandler.VerifyMCPAction":             "audit-baseline: needs review",
	"SDKTokenHandler.RevokeToken":                  "service-layer scoping: SDKTokenService.RevokeToken collapses both not-found and cross-user mismatch into ErrSDKTokenNotFound; the handler maps the sentinel to a fixed 404. SDK tokens are user-scoped (not org-scoped), so the gate lives at the service layer.",
	"TagHandler.UpdateTag":                         "audit-baseline: needs review (service-layer scoping exists but has existence side channel via error string; A3d-ii follow-up — see todo/2026-05-21-a3d-ii-tag-mcp-scoping.md)",
	"VerificationHandler.SubmitVerificationResult": "audit-baseline: needs review",
	"VerificationHandler.UpdateExecutionStatus":    "audit-baseline: needs review",

	// ---------------------------------------------------------------
	// Service-layer-enforced tenant scoping. The lint's AST scan cannot
	// see across the handler/service boundary, so handlers that push
	// the org check into the service (rather than wrapping the path
	// load in LoadOwned at the handler layer) need an allowlist entry
	// with the enforcement mechanism cited.
	// ---------------------------------------------------------------
	"A2AHandler.ListUserConsents":             "service-layer scoping: A2AService.ListUserConsents requires callerOrgID and the repo query filters by organization_id (A3c #42 closed in PR TBD).",
	"MCPAttestationHandler.RevokeAttestation": "service-layer scoping: MCPAttestationService.RevokeAttestation requires callerOrgID and returns application.ErrAttestationNotFound for both not-found and cross-tenant (A3c #47 closed in PR TBD).",

	// ---------------------------------------------------------------
	// Historical notes:
	// - A3c-FLAGGED defects #42, #43, #44, #45, #46, #47 were
	//   surfaced 2026-05-20 by broadening paramKeys (PR #144).
	//   Defects #43, #44 now satisfy the lint structurally via
	//   LoadOwned/agent.OrganizationID checks at the handler layer.
	//   Defects #45, #46 were dead-handler removals (handlers were
	//   never mounted). Defects #42 and #47 require allowlist
	//   entries above because their fix lives at the service layer.
	// - Defect #48 (RegisterCapability) lived here from PR #144 until
	//   PR #145 wrapped the handler with LoadOwned. The audit doc
	//   #48 retains the full historical record.
	// ---------------------------------------------------------------
}

// recognizedHelpers names the helper functions that satisfy the lint.
// Any handler invoking one of these on a path that derives from
// c.Params("id"|"agent_id"|"agent-id") is considered properly tenant-scoped.
//
// `loadOwnedAgent` is the A2A-specific wrapper (a2a_handler.go) that
// hides the agentService.GetAgent->LoadOwned closure boilerplate. The
// lint recognizes it the same as LoadOwned because the unwrapping is
// a one-line forwarder; treating it as a recognized helper avoids
// every A2A handler having to repeat the closure inline.
var recognizedHelpers = map[string]bool{
	"LoadOwned":         true,
	"LoadOwnedViaAgent": true,
	"loadOwnedAgent":    true,
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

// serviceParamAllowlist names application/service methods that legitimately
// accept an `orgID` / `organizationID` parameter but do not reference it in
// their body. The lint catches the "service param accepted-but-unused"
// class-#3 IDOR (see feedback_tenantscope_lint_three_blindspot_classes in
// the memory): a service whose signature looks tenant-aware but whose
// repository call runs `WHERE id = $1` system-wide.
//
// Each entry must include a one-line justification. The allowlist must
// stay small; the goal is to refactor or remove the unused parameter
// rather than to add entries.
// collectionScopeAllowlist names service methods that legitimately return a
// collection across every organization. Each entry must carry a one-line
// justification naming what makes the cross-tenant read safe — minimization,
// an admin-only route gate, or a genuinely public dataset. "It is an admin
// endpoint" is only a justification if a role middleware actually enforces it;
// both of the methods that motivated this check were documented as admin
// endpoints in their doc comments and were mounted with no role middleware at
// all.
var collectionScopeAllowlist = map[string]string{}

var serviceParamAllowlist = map[string]string{
	// AUDIT-BASELINE: pre-existing methods discovered when the
	// class-#3 scan first landed. Each entry MUST cite the rationale
	// or the tracking todo. Reviewers fixing the underlying defect
	// should remove the entry; the allowlist must NOT grow over time.

	// Stub: documented no-op kept for future expansion. orgID is the
	// future-API surface; no current callsite. (alert_service.go:108-117)
	"AlertService.CheckAPIKeyExpiry": "stub no-op; orgID reserved for future API key expiry implementation",

	// Pure function on the passed-in baseline argument; no DB call.
	// orgID is vestigial — kept because callers still pass it but the
	// method's logic uses only `baseline.CapabilityUsage`. Not an IDOR
	// risk. Refactor candidate: remove the parameter and update the
	// (private) callsite in behavior_analysis_service.go.
	"BehaviorAnalysisService.detectCapabilityDrift": "pure baseline analysis; orgID vestigial; refactor candidate",
}

// classifyServiceDir treats a directory as the service scan target if
// its trailing component is "application". Heuristic, but works for
// every layout this lint targets.
func classifyServiceDir(dir string) bool {
	return strings.HasSuffix(filepath.Clean(dir), "/application") ||
		strings.HasSuffix(filepath.Clean(dir), string(filepath.Separator)+"application")
}

func main() {
	// Default behavior: scan BOTH the handlers directory (handler
	// param-id-without-LoadOwned class) AND the application services
	// directory (orgID-param-accepted-but-unused class). Either scan
	// failing makes the whole invocation fail; allowlists are
	// independent.
	handlersDir := "./internal/interfaces/http/handlers"
	servicesDir := "./internal/application"
	if len(os.Args) > 1 {
		// Single-directory mode: route based on path. Preserves the
		// legacy invocation `go run ./cmd/tenantscope-lint
		// ./internal/interfaces/http/handlers`.
		dir := filepath.Clean(os.Args[1])
		if classifyServiceDir(dir) {
			handlersDir = ""
			servicesDir = dir
		} else {
			handlersDir = dir
			servicesDir = ""
		}
	}

	failed := false

	if handlersDir != "" {
		violations, err := scanDirectory(handlersDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tenantscope-lint: %v\n", err)
			os.Exit(2)
		}
		if len(violations) > 0 {
			failed = true
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

See apps/backend/internal/interfaces/http/handlers/tenant_scope.go for
the helper documentation and the SECURITY rationale for 404-not-403.`)
		} else {
			fmt.Printf("tenantscope-lint: ok (%d allowlist entries; scanned %s)\n", len(allowlist), handlersDir)
		}

		// An allowlist entry that resolves to no real method is a silent hole: it
		// presents as a reviewed exemption while covering nothing. Checked after the
		// scan so discoveredHandlers is fully populated.
		if unresolved := checkAllowlistResolves(); len(unresolved) > 0 {
			failed = true
			fmt.Fprintf(os.Stderr, "\ntenantscope-lint: %d allowlist entr(ies) name a method that does not exist:\n\n", len(unresolved))
			for _, key := range unresolved {
				fmt.Fprintf(os.Stderr, "  %s\n", key)
			}
			fmt.Fprintln(os.Stderr, `
Each of these exempts nothing. Either the method was renamed, or it lives on a
different receiver, or it never existed. Fix by finding the real handler:

  grep -rn "func (.* \*<ReceiverType>) " ./internal/interfaces/http/handlers

Then either correct the key to the real Receiver.Method and keep the
justification, or delete the entry if the handler is gone. Do not silence this
by deleting the check.`)
		}
	}

	if servicesDir != "" {
		serviceViolations, err := scanServiceDirectory(servicesDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tenantscope-lint: %v\n", err)
			os.Exit(2)
		}
		if len(serviceViolations) > 0 {
			failed = true
			sort.Slice(serviceViolations, func(i, j int) bool {
				if serviceViolations[i].File != serviceViolations[j].File {
					return serviceViolations[i].File < serviceViolations[j].File
				}
				return serviceViolations[i].Line < serviceViolations[j].Line
			})
			fmt.Fprintf(os.Stderr, "\nserviceparam-lint: %d service method(s) accept an orgID/organizationID parameter but never reference it in the body (class-#3 IDOR — likely repository call runs WHERE id = $1 system-wide):\n\n", len(serviceViolations))
			for _, v := range serviceViolations {
				fmt.Fprintf(os.Stderr, "  %s:%d  %s\n", v.File, v.Line, v.Handler)
			}
			fmt.Fprintln(os.Stderr, `
To fix each violation:
  1. Use orgID in the repository call's WHERE clause (e.g. add it to
     a "WHERE id = $1 AND organization_id = $2" query).
  2. If the orgID parameter is genuinely unused (e.g. legacy callsite
     that no longer needs it), REMOVE the parameter — do not add to
     the allowlist.
  3. If the orgID is used through a struct field accessor (e.g. it
     was unpacked into a local), refactor to reference orgID directly
     OR add the method to serviceParamAllowlist in
     cmd/tenantscope-lint/main.go with a one-line justification.

See feedback_tenantscope_lint_three_blindspot_classes (memory) for
the broader taxonomy. False-negative class: orgID referenced only in
logging without flowing to the repo call still passes this check.`)
		} else {
			fmt.Printf("serviceparam-lint: ok (%d allowlist entries; scanned %s)\n", len(serviceParamAllowlist), servicesDir)
		}

		collectionViolations, err := scanCollectionDirectory(servicesDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tenantscope-lint: %v\n", err)
			os.Exit(2)
		}
		if len(collectionViolations) > 0 {
			failed = true
			sort.Slice(collectionViolations, func(i, j int) bool {
				if collectionViolations[i].File != collectionViolations[j].File {
					return collectionViolations[i].File < collectionViolations[j].File
				}
				return collectionViolations[i].Line < collectionViolations[j].Line
			})
			fmt.Fprintf(os.Stderr, "\ncollectionscope-lint: %d service method(s) return a collection with no organization parameter to scope it by:\n\n", len(collectionViolations))
			for _, v := range collectionViolations {
				fmt.Fprintf(os.Stderr, "  %s:%d  %s\n", v.File, v.Line, v.Handler)
			}
			fmt.Fprintln(os.Stderr, `
A ListAll* method that takes no organization returns every tenant's rows to
whoever calls it. Fix by adding an orgID uuid.UUID parameter, threading it to
the repository, and putting the predicate in the SQL — including the COUNT,
since an unscoped total discloses other tenants' cardinality on its own.

If the method is genuinely global, add it to collectionScopeAllowlist in
cmd/tenantscope-lint/main.go with a one-line justification naming the control
that makes it safe. A doc comment calling it an "admin endpoint" is not that
control; verify a role middleware is actually mounted on the route.`)
		} else {
			fmt.Printf("collectionscope-lint: ok (%d allowlist entries; scanned %s)\n", len(collectionScopeAllowlist), servicesDir)
		}
	}

	if failed {
		os.Exit(1)
	}
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

// discoveredHandlers collects every Receiver.Method seen while scanning, so the
// allowlist can be checked against reality rather than trusted.
//
// An allowlist key that matches no real method exempts nothing. It is worse than a
// missing entry: it reads as a reviewed, justified exemption while the handler it was
// meant to cover is either unexempted or named something else entirely. Fourteen of the
// entries here were in that state — `AuthHandler.Login` (the type has `LocalLogin`; the
// real `Login` is on `PublicRegistrationHandler`), `RegistryBridgeHandler.Verify` (the
// type has only `TriggerPush`), `OAuthTokenHandler.Token` (it is `HandleTokenRequest`),
// and eleven more.
var discoveredHandlers = map[string]bool{}

// checkAllowlistResolves reports allowlist keys that name no method the scan found.
func checkAllowlistResolves() []string {
	var unresolved []string
	for key := range allowlist {
		if !discoveredHandlers[key] {
			unresolved = append(unresolved, key)
		}
	}
	sort.Strings(unresolved)
	return unresolved
}

func scanFile(fset *token.FileSet, file *ast.File, path string) []violation {
	var out []violation
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Recv == nil {
			continue
		}
		handlerName := receiverTypeName(fn) + "." + fn.Name.Name
		discoveredHandlers[handlerName] = true
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
		// Watch Query as well as Params.
		//
		// The lint only ever looked at c.Params, and that was selection bias rather than a
		// scoping decision: every cross-tenant handler found in the 2026 audit read its
		// tenant identifier from somewhere else. SecretsHandler.ListNamespaces and
		// GetAuditLog took `agentId` from c.Query; A2AHandler.ListTasks took it from
		// c.Query and had no organization predicate anywhere in the stack;
		// SecretsHandler.CreateNamespace and LifecycleHandler.BulkStatus took it from the
		// request body. The lint reported green on all of them, for years, because it was
		// looking at the one channel none of them used.
		//
		// A guard that watches the channel the defects do not use is not a weak guard, it
		// is a guard that reports on something else. Body-bound fields are a third channel
		// and are not covered here — see bodyBindsTenantField below.
		if sel.Sel == nil || (sel.Sel.Name != "Params" && sel.Sel.Name != "Query") {
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

// orgParamNames are the parameter names recognized as carrying a
// caller-org UUID into a service method. These are the names we expect
// to appear in the body if the method is genuinely tenant-scoped.
//
// Spelling variants reflect the codebase's current mix; the lint should
// tolerate any reasonable casing. Names not in this set (e.g. `tenantID`)
// are excluded because they introduce false-positive risk on unrelated
// methods.
var orgParamNames = map[string]bool{
	"orgID":          true,
	"OrgID":          true,
	"organizationID": true,
	"OrganizationID": true,
	"adminOrgID":     true,
	"callerOrgID":    true,
}

// scanServiceDirectory walks the application/services directory and
// returns class-#3 violations: function declarations whose signature
// includes an `orgID`-shaped parameter (per orgParamNames) that the
// body never references. This catches methods like the historical
// `AlertService.AcknowledgeAlert(ctx, alertID, orgID, userID uuid.UUID)`
// where orgID is accepted in the signature but the repo call ran
// `WHERE id = $1` with no tenant filter — making it a system-wide IDOR.
//
// Allowlist is `serviceParamAllowlist` (qualified by ReceiverType.Method).
//
// False-negative class to be aware of: a method that references orgID
// only inside a log statement (without flowing it to a repo call) will
// pass this check. The narrower "is orgID actually used in the repo
// query?" check requires cross-function flow analysis beyond what the
// AST scan provides here. Reviewers should still verify, but the
// structural pattern catches the common case where orgID is forgotten
// outright.
func scanServiceDirectory(dir string) ([]violation, error) {
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
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		violations = append(violations, scanServiceFile(fset, file, path)...)
	}
	return violations, nil
}

// scanCollectionDirectory walks the service package for unscoped collection
// readers. Kept separate from scanServiceDirectory so the two failure modes
// report distinct remediation text; they scan the same files.
func scanCollectionDirectory(dir string) ([]violation, error) {
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
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		violations = append(violations, scanCollectionFile(fset, file, path)...)
	}
	return violations, nil
}

// scanServiceFile walks one parsed Go file for class-#3 service-method
// violations. A method qualifies for inspection when:
//   - It has a receiver (i.e. it's a method, not a free function).
//   - It has at least one parameter whose name is in orgParamNames and
//     whose type is uuid.UUID.
//
// For each qualifying parameter, the body is searched for any *ast.Ident
// matching the parameter name. Zero matches → violation.
// scanCollectionFile catches the class that both of the other two checks are
// structurally blind to: a service method that returns a WHOLE COLLECTION and
// never takes an organization to scope it by.
//
// The handler check triggers on reading a tenant-scoped c.Params(...) key, and
// the service check triggers on accepting an orgID and not using it. A method
// that asks for no identifier at all and returns everything trips neither. That
// is exactly how A2AService.ListAllConsents and ListAllTrustScores served every
// tenant's rows to any authenticated caller for seven months while this lint
// reported ok on every run.
//
// The predicate is deliberately mechanical: a method whose name begins with
// "ListAll" must accept a uuid.UUID parameter with an organization-ish name. A
// genuinely global lister goes on collectionScopeAllowlist with a written
// justification, the same as every other exemption here.
//
// KNOWN LIMIT, stated so nobody reads a green run as more than it is. The
// "ListAll" prefix is a NAMING convention, not the shape of the defect, so this
// check is a regression guard for the two methods above and not yet a guard for
// the class. Every same-shape method that happens to be named differently is
// invisible to it -- A2AService.ListAgentCards and A2AService.ListA2ATasks are
// both on this very service, both return a whole collection with no
// organization parameter, and both pass this check today.
//
// The predicate that would cover the class is "returns a slice or map AND takes
// no organization parameter". It is not enabled here because it fires across
// the package and every hit needs triage into either a fix or an allowlist
// entry -- that is its own change, not a rider on this one.
func scanCollectionFile(fset *token.FileSet, file *ast.File, path string) []violation {
	var out []violation
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Recv == nil {
			continue
		}
		if !strings.HasPrefix(fn.Name.Name, "ListAll") {
			continue
		}
		methodName := receiverTypeName(fn) + "." + fn.Name.Name
		if _, allowed := collectionScopeAllowlist[methodName]; allowed {
			continue
		}
		if fn.Type == nil || fn.Type.Params == nil {
			continue
		}
		scoped := false
		for _, field := range fn.Type.Params.List {
			if !isUUIDType(field.Type) {
				continue
			}
			for _, name := range field.Names {
				if orgParamNames[name.Name] {
					scoped = true
				}
			}
		}
		if !scoped {
			out = append(out, violation{
				File:    path,
				Line:    fset.Position(fn.Pos()).Line,
				Handler: methodName,
			})
		}
	}
	return out
}

func scanServiceFile(fset *token.FileSet, file *ast.File, path string) []violation {
	var out []violation
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		// Service-layer methods always have a receiver. Skip free
		// functions (constructors, helpers).
		if fn.Recv == nil {
			continue
		}
		methodName := receiverTypeName(fn) + "." + fn.Name.Name
		if _, allowed := serviceParamAllowlist[methodName]; allowed {
			continue
		}
		if fn.Type == nil || fn.Type.Params == nil {
			continue
		}
		for _, field := range fn.Type.Params.List {
			if !isUUIDType(field.Type) {
				continue
			}
			for _, name := range field.Names {
				if !orgParamNames[name.Name] {
					continue
				}
				if bodyReferencesIdent(fn.Body, name.Name) {
					continue
				}
				out = append(out, violation{
					File:    path,
					Line:    fset.Position(name.Pos()).Line,
					Handler: methodName + " (param " + name.Name + ")",
				})
			}
		}
	}
	return out
}

// isUUIDType returns true if the parameter type expression is a
// `uuid.UUID` selector. Recognizes the typical `uuid.UUID` spelling;
// does NOT match pointer types or slices (callers passing a *uuid.UUID
// or []uuid.UUID are not in scope for this lint — those are
// non-canonical and rare in this codebase).
func isUUIDType(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return pkg.Name == "uuid" && sel.Sel != nil && sel.Sel.Name == "UUID"
}

// bodyReferencesIdent returns true if any *ast.Ident in the function
// body — EXCLUDING selector right-hand sides (`Sel`) — matches the
// given parameter name. Parameter declarations are outside the body,
// so the lookup is purely usage-based.
//
// SECURITY: a naive `ast.Inspect` walk visits the `Sel` field of every
// `*ast.SelectorExpr`. If we didn't skip it, expressions like
// `agent.OrganizationID` or `req.OrgID` would match parameter names
// `OrganizationID`/`OrgID`, silently making the lint pass for any
// method that touches a domain object with the same field name. This
// is the exact class-of-bug the lint was designed to catch on the
// OPPOSITE side — Phase 4.5 review caught it as a HIGH (H1) before
// landing.
//
// Implementation: a custom `ast.Visitor` intercepts `*ast.SelectorExpr`
// and recurses only into the LHS (`X`) — `Sel` is never visited as
// an ident match candidate.
func bodyReferencesIdent(body *ast.BlockStmt, name string) bool {
	var found bool
	ast.Walk(identMatchVisitor{name: name, found: &found}, body)
	return found
}

// identMatchVisitor walks a Go AST node tree counting how many times a
// top-level (non-selector-RHS) identifier matches a given name. It
// short-circuits as soon as a match is found.
type identMatchVisitor struct {
	name  string
	found *bool
}

func (v identMatchVisitor) Visit(n ast.Node) ast.Visitor {
	if *v.found || n == nil {
		return nil
	}
	switch x := n.(type) {
	case *ast.SelectorExpr:
		// Walk the LHS (e.g. `agent` in `agent.OrganizationID`).
		// Sel is intentionally NOT walked: a field name match on
		// `OrganizationID` should not be mistaken for a param match.
		ast.Walk(v, x.X)
		return nil
	case *ast.KeyValueExpr:
		// Struct literal: `domain.Alert{OrganizationID: orgID}`. The
		// Key is a field name (skip); the Value is a real expression
		// (walk). Treating Key as a potential ident match would
		// produce the same false-negative as the selector case.
		ast.Walk(v, x.Value)
		return nil
	case *ast.Ident:
		if x.Name == v.name {
			*v.found = true
		}
		return nil
	}
	return v
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
