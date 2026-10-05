package agentauth

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The census of agent-status reads, and the shape of every signature authenticator, as
// tests. Neither list of files below is the input: the files are found by walking the
// tree, and the lists only name the exceptions and the admitted sites, so a new status
// read or a new authenticator is checked without anyone remembering to add it.

// internalRoot is apps/backend/internal, from this package's directory.
const internalRoot = ".."

var httpLayerDirs = []string{
	"interfaces/http/middleware",
	"interfaces/http/handlers",
}

// verifierCall matches a call that verifies an agent's signature: the primitives, and the
// Keys methods that wrap them.
var verifierCall = regexp.MustCompile(`ed25519\.Verify\(|pqc\.VerifyMLDSA\(|crypto\.VerifySignature\(|\.\s*Verify(Ed25519|MLDSA|Hybrid)\(`)

// authenticatorExceptions are authenticator files that do not follow the shape, each with
// the reason. Anything else that verifies a signature must.
var authenticatorExceptions = map[string]string{
	"interfaces/http/handlers/public_mcp_handler.go": "not mounted: nothing outside the file constructs PublicMCPHandler " +
		"(asserted below); it reads the agent row before its signature check and is outside the admitted census",
}

type parsedFile struct {
	rel  string
	fset *token.FileSet
	file *ast.File
	src  string
}

// goFiles parses every non-test .go file under dir (relative to internalRoot).
func goFiles(t *testing.T, root, dir string) []parsedFile {
	t.Helper()
	var out []parsedFile
	base := filepath.Join(root, dir)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, raw, 0)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		out = append(out, parsedFile{rel: filepath.ToSlash(rel), fset: fset, file: f, src: string(raw)})
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", base, err)
	}
	return out
}

// authenticatorFiles are the HTTP-layer files that verify an agent's signature.
func authenticatorFiles(t *testing.T) []parsedFile {
	t.Helper()
	var out []parsedFile
	for _, dir := range httpLayerDirs {
		for _, pf := range goFiles(t, internalRoot, dir) {
			if verifierCall.MatchString(pf.src) {
				out = append(out, pf)
			}
		}
	}
	return out
}

func funcName(fd *ast.FuncDecl) string {
	if fd.Recv != nil && len(fd.Recv.List) == 1 {
		return types.ExprString(fd.Recv.List[0].Type) + "." + fd.Name.Name
	}
	return fd.Name.Name
}

// isAgentRowRead reports whether call reads an agent row directly: any GetAgent, or a
// GetByID on something named for agents, or anything called on an agentRepo.
func isAgentRowRead(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	recv := strings.ToLower(receiverName(sel.X))
	switch {
	case sel.Sel.Name == "GetAgent":
		return true
	case sel.Sel.Name == "GetByID" && strings.Contains(recv, "agent"):
		return true
	case strings.Contains(recv, "agentrepo"):
		return true
	}
	return false
}

// receiverName is the last name in a receiver chain: agentRepo for h.agentRepo, and
// getAgentService for h.getAgentService(). Call arguments are not part of it.
func receiverName(x ast.Expr) string {
	switch r := x.(type) {
	case *ast.Ident:
		return r.Name
	case *ast.SelectorExpr:
		return r.Sel.Name
	case *ast.CallExpr:
		return receiverName(r.Fun)
	case *ast.StarExpr:
		return receiverName(r.X)
	case *ast.ParenExpr:
		return receiverName(r.X)
	}
	return ""
}

// The authenticators reach the agent row only through KeySet and LoadVerifiedAgent.
func TestAuthenticatorsReadTheAgentRowOnlyThroughTheKeySet(t *testing.T) {
	files := authenticatorFiles(t)
	if len(files) == 0 {
		t.Fatal("found no authenticator files: the walk is broken, not the tree")
	}

	var checked []string
	for _, pf := range files {
		if _, excepted := authenticatorExceptions[pf.rel]; excepted {
			continue
		}
		checked = append(checked, pf.rel)
		ast.Inspect(pf.file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if ok && isAgentRowRead(call) {
				t.Errorf("%s: %s reads the agent row directly; use agentauth.KeySet, then "+
					"agentauth.LoadVerifiedAgent after the signature verifies",
					pf.fset.Position(call.Pos()), types.ExprString(call.Fun))
			}
			return true
		})
	}

	// The control: the walk finds the authenticators this shape was built for. A rename
	// that hid one from the verifier pattern would otherwise pass silently.
	for _, want := range []string{
		"interfaces/http/middleware/pqc_agent_auth.go",
		"interfaces/http/middleware/ed25519_agent_auth.go",
		"interfaces/http/handlers/oauth_token_handler.go",
		"interfaces/http/handlers/verification_handler.go",
	} {
		found := false
		for _, got := range checked {
			found = found || got == want
		}
		if !found {
			t.Errorf("%s no longer matches the verifier pattern; checked %v", want, checked)
		}
	}
}

// Each excepted file stays unreachable: wiring it fails here, so it is converted first.
func TestExceptedAuthenticatorsAreNotMounted(t *testing.T) {
	var callers []string
	for _, pf := range goFiles(t, internalRoot, ".") {
		if pf.rel != "interfaces/http/handlers/public_mcp_handler.go" && strings.Contains(pf.src, "PublicMCPHandler") {
			callers = append(callers, pf.rel)
		}
	}
	for _, pf := range goFiles(t, filepath.Join(internalRoot, ".."), "cmd") {
		if strings.Contains(pf.src, "PublicMCPHandler") {
			callers = append(callers, pf.rel)
		}
	}
	if len(callers) > 0 {
		t.Errorf("PublicMCPHandler is now referenced in %v; convert it to agentauth.KeySet "+
			"and remove its exception before mounting it", callers)
	}
}

// statusRead reports whether n reads an agent's status or another attribute that may only
// be read after verification: a .Status or .HybridModeEnabled field (not a c.Status(...)
// call), or a call to the status predicate.
func statusRead(n ast.Node, calledFuns map[ast.Expr]bool) (string, bool) {
	switch x := n.(type) {
	case *ast.SelectorExpr:
		if (x.Sel.Name == "Status" || x.Sel.Name == "HybridModeEnabled") && !calledFuns[x] {
			return types.ExprString(x), true
		}
	case *ast.CallExpr:
		name := ""
		switch f := x.Fun.(type) {
		case *ast.Ident:
			name = f.Name
		case *ast.SelectorExpr:
			name = f.Sel.Name
		}
		if name == "AgentStatusPermitsAuth" || name == "agentStatusPermitsAuth" {
			return name + "(...)", true
		}
	}
	return "", false
}

// In every function that verifies a signature, nothing reads the status (or the hybrid
// flag) before the first verification call. The type already forces this, since the row
// is reachable only through LoadVerifiedAgent; this catches a status read that arrives by
// some other path.
func TestAuthenticatorsReadStatusOnlyAfterTheSignatureVerifies(t *testing.T) {
	for _, pf := range authenticatorFiles(t) {
		if _, excepted := authenticatorExceptions[pf.rel]; excepted {
			continue
		}
		for _, decl := range pf.file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			body := pf.src[pf.fset.Position(fd.Body.Pos()).Offset:pf.fset.Position(fd.Body.End()).Offset]
			loc := verifierCall.FindStringIndex(body)
			if loc == nil {
				continue
			}
			firstVerify := fd.Body.Pos() + token.Pos(loc[0])

			calledFuns := map[ast.Expr]bool{}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					calledFuns[call.Fun] = true
				}
				return true
			})
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if n == nil {
					return false
				}
				if what, ok := statusRead(n, calledFuns); ok && n.Pos() < firstVerify {
					t.Errorf("%s: %s reads %s before the signature verifies",
						pf.fset.Position(n.Pos()), funcName(fd), what)
				}
				return true
			})
		}
	}
}

// admittedStatusReads is the census of every call to the status predicate outside its
// own definitions, by file and function, with the number of calls and why each is
// admitted. A new call fails this test until it is added here with its reason.
var admittedStatusReads = map[string]struct {
	calls  int
	reason string
}{
	"interfaces/http/middleware/pqc_agent_auth.go#PQCAgentMiddleware":         {1, "after the signature verifies, through LoadVerifiedAgent"},
	"interfaces/http/middleware/ed25519_agent_auth.go#Ed25519AgentMiddleware": {1, "after the signature verifies, through LoadVerifiedAgent"},
	"interfaces/http/handlers/oauth_token_handler.go#*OAuthTokenHandler.processTokenRequest": {
		1, "after the assertion's signature verifies, through LoadVerifiedAgent"},
	"interfaces/http/handlers/verification_handler.go#*VerificationHandler.CreateVerification": {
		1, "after the signature verifies, through LoadVerifiedAgent; the unknown-agent 404 is a named residual"},
	"interfaces/http/handlers/verification_handler.go#*VerificationHandler.GetVerificationSDK": {
		1, "after the signature verifies, through LoadVerifiedAgent"},
	"interfaces/http/handlers/action_request_statement.go#*VerificationHandler.createVerificationFromStatement": {
		1, "after the statement's signature verifies and its nonce is admitted, through LoadVerifiedAgent"},
	"interfaces/http/middleware/service_principal.go#ServicePrincipalMiddleware": {
		1, "verify-first: the service token's JWT signature is verified before the agent row is read"},
	"interfaces/http/middleware/api_key.go#APIKeyMiddleware": {
		1, "verify-first: the API-key hash lookup is the verification, and the status arrives with the key row"},
	"interfaces/http/middleware/api_key.go#OptionalAPIKeyMiddleware": {
		1, "verify-first: the API-key hash lookup is the verification, and the status arrives with the key row"},
	"interfaces/http/handlers/aip_handler.go#*AIPHandler.ResolveDID": {
		1, "the public DID resolver publishes deactivated for every agent that is not pending, by design"},
}

func TestAgentStatusReadCensus(t *testing.T) {
	got := map[string]int{}
	roots := []struct{ root, dir string }{
		{internalRoot, "."},
		{filepath.Join(internalRoot, ".."), "cmd"},
	}
	for _, r := range roots {
		for _, pf := range goFiles(t, r.root, r.dir) {
			for _, decl := range pf.file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				// The predicate's own definitions: domain's, and the middleware forwarder.
				if fd.Name.Name == "AgentStatusPermitsAuth" || fd.Name.Name == "agentStatusPermitsAuth" {
					continue
				}
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					if what, isRead := statusRead(call, nil); isRead && strings.HasSuffix(what, "(...)") {
						got[pf.rel+"#"+funcName(fd)]++
					}
					return true
				})
			}
		}
	}

	var keys []string
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		admitted, ok := admittedStatusReads[k]
		switch {
		case !ok:
			t.Errorf("%s calls the agent-status predicate %d time(s) and is not in the census; "+
				"a status read must follow the credential's verification and be admitted here with its reason", k, got[k])
		case admitted.calls != got[k]:
			t.Errorf("%s: %d status read(s), census admits %d", k, got[k], admitted.calls)
		}
	}
	for k := range admittedStatusReads {
		if got[k] == 0 {
			t.Errorf("%s is in the census but no longer reads the status; remove it", k)
		}
	}
}
