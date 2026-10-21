package handlers

// Census of the credential route guards' call sites in this package. A
// route's own check (refuseIfFamilyRevoked, refuseUnlessLiveSignIn) and the
// session middleware, which refuses a revoked session before the route runs,
// record a refusal at the route under one label: the call names it with a
// middleware.CredentialRoute* constant, and the middleware recognises the
// route through middleware.CredentialRoutes. Every call names a label in
// that table, and every label there is named by exactly one call.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

var credentialRouteGuards = map[string]bool{
	"refuseIfFamilyRevoked":  true,
	"refuseUnlessLiveSignIn": true,
}

// credentialRouteConstants reads the string constants the middleware package
// declares beside its credential route table, by name.
func credentialRouteConstants(t *testing.T) map[string]string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("..", "middleware", "credential_route_refusal.go"), nil, 0)
	require.NoError(t, err)
	values := map[string]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range valueSpec.Names {
				if i >= len(valueSpec.Values) {
					continue
				}
				lit, ok := valueSpec.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				value, err := strconv.Unquote(lit.Value)
				require.NoError(t, err)
				values[name.Name] = value
			}
		}
	}
	return values
}

// credentialRouteGuardCalls returns the labels the guard calls in file name,
// each with the call sites that name it, and a finding for each call that
// does not name its route with one of the middleware package's label
// constants.
func credentialRouteGuardCalls(fset *token.FileSet, file *ast.File, constants map[string]string) (named map[string][]string, findings []string) {
	named = map[string][]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fn, ok := call.Fun.(*ast.Ident)
		if !ok || !credentialRouteGuards[fn.Name] {
			return true
		}
		site := fset.Position(call.Pos()).String()
		if len(call.Args) != 4 {
			findings = append(findings, site+": "+fn.Name+" takes 4 arguments")
			return true
		}
		sel, _ := call.Args[3].(*ast.SelectorExpr)
		var pkg *ast.Ident
		if sel != nil {
			pkg, _ = sel.X.(*ast.Ident)
		}
		if pkg == nil || pkg.Name != "middleware" {
			findings = append(findings, site+": "+fn.Name+" names its route with something other than a middleware.CredentialRoute constant")
			return true
		}
		label, known := constants[sel.Sel.Name]
		if !known {
			findings = append(findings, site+": middleware."+sel.Sel.Name+" is not a label constant of the credential route table")
			return true
		}
		named[label] = append(named[label], site)
		return true
	})
	return named, findings
}

func TestCredentialRouteGuardCallsFindings(t *testing.T) {
	src := `package handlers

func guarded() {
	refuseIfFamilyRevoked(c, jwtService, audit, middleware.CredentialRouteSDKDownload)
	refuseUnlessLiveSignIn(c, jwtService, audit, "api_key_create")
	refuseIfFamilyRevoked(c, jwtService, audit, middleware.CredentialRouteUnknown)
	refuseIfFamilyRevoked(c, jwtService, audit, other.CredentialRouteSDKDownload)
	refuseIfFamilyRevoked(c, jwtService, audit)
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "sample.go", src, 0)
	require.NoError(t, err)
	named, findings := credentialRouteGuardCalls(fset, file, map[string]string{"CredentialRouteSDKDownload": "sdk_download"})
	assert.Equal(t, map[string][]string{"sdk_download": {"sample.go:4:2"}}, named)
	assert.Equal(t, []string{
		"sample.go:5:2: refuseUnlessLiveSignIn names its route with something other than a middleware.CredentialRoute constant",
		"sample.go:6:2: middleware.CredentialRouteUnknown is not a label constant of the credential route table",
		"sample.go:7:2: refuseIfFamilyRevoked names its route with something other than a middleware.CredentialRoute constant",
		"sample.go:8:2: refuseIfFamilyRevoked takes 4 arguments",
	}, findings)
}

func TestCredentialRouteGuardsNameTheSessionMiddlewaresRoutes(t *testing.T) {
	constants := credentialRouteConstants(t)
	require.NotEmpty(t, constants, "the middleware's label constants were read")

	paths, err := filepath.Glob("*.go")
	require.NoError(t, err)
	fset := token.NewFileSet()
	named := map[string][]string{}
	var findings []string
	for _, path := range paths {
		// credential_mint_family.go defines the guards; refuseUnlessLiveSignIn
		// passes its own route on to refuseIfFamilyRevoked.
		if strings.HasSuffix(path, "_test.go") || path == "credential_mint_family.go" {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err)
		fileNamed, fileFindings := credentialRouteGuardCalls(fset, file, constants)
		for label, sites := range fileNamed {
			named[label] = append(named[label], sites...)
		}
		findings = append(findings, fileFindings...)
	}
	assert.Empty(t, findings)
	require.NotEmpty(t, named, "the census found the guard calls")

	for _, route := range middleware.CredentialRoutes() {
		assert.Len(t, named[route.Label], 1, "%s %s (%s) is named by exactly one guard call: %v", route.Method, route.Path, route.Label, named[route.Label])
		delete(named, route.Label)
	}
	assert.Empty(t, named, "guard calls name labels middleware.CredentialRoutes does not list, so the session middleware would not record a refusal there")
}
