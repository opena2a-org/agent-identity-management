package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
)

// TestAgentRequestAudienceStartupLine_QuotesTheOriginAsSet checks that the
// line names AIM_PUBLIC_ORIGIN as config.Load holds it, quoted: its case, its
// spaces, its port and its path are kept, and a line feed or a carriage return
// in it is escaped, so the value cannot start another line of the log. The
// environment table records the line for each input config.Load resolves.
func TestAgentRequestAudienceStartupLine_QuotesTheOriginAsSet(t *testing.T) {
	const prefix = "Agent requests signed as agent-request-v2: no route accepts them yet (AIM_PUBLIC_ORIGIN, the audience they must name, is "
	for _, tc := range []struct{ origin, quoted string }{
		{"https://aim.example.com", `"https://aim.example.com"`},
		{"https://aim.example.com:8443", `"https://aim.example.com:8443"`},
		{"https://AIM.example.com/", `"https://AIM.example.com/"`},
		{" https://aim.example.com ", `" https://aim.example.com "`},
		{"https://aim.example.com\nforged line", `"https://aim.example.com\nforged line"`},
		{"https://aim.example.com\rforged line", `"https://aim.example.com\rforged line"`},
	} {
		line := agentRequestAudienceStartupLine(config.ServerConfig{PublicOrigin: tc.origin})
		assert.Equal(t, prefix+tc.quoted+")", line)
		assert.NotContains(t, line, "\n")
		assert.NotContains(t, line, "\r")
	}
	assert.Equal(t,
		"Agent requests signed as agent-request-v2: no route accepts them yet (AIM_PUBLIC_ORIGIN, the audience they must name, is not set)",
		agentRequestAudienceStartupLine(config.ServerConfig{}))
}

// identifierUses returns each use of name in file, as file:line, leaving out
// the name of a function or method declared with it. A name in a comment is
// not a use.
func identifierUses(fset *token.FileSet, file *ast.File, name string) []string {
	declared := map[token.Pos]bool{}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			declared[fn.Name.Pos()] = true
		}
	}
	var uses []string
	ast.Inspect(file, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name && !declared[id.Pos()] {
			position := fset.Position(id.Pos())
			uses = append(uses, fmt.Sprintf("%s:%d", position.Filename, position.Line))
		}
		return true
	})
	return uses
}

// backendUses returns, for each name, its uses in the backend's Go files, the
// directory two above this package: every file but test files and those under
// testdata, vendor, node_modules or a directory whose name starts with a dot.
func backendUses(t *testing.T, names ...string) map[string][]string {
	t.Helper()
	const root = "../.."
	fset := token.NewFileSet()
	uses := map[string][]string{}
	parsed := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if path != root && (base == "testdata" || base == "vendor" || base == "node_modules" || strings.HasPrefix(base, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		parsed++
		for _, name := range names {
			uses[name] = append(uses[name], identifierUses(fset, file, name)...)
		}
		return nil
	})
	require.NoError(t, err)
	require.NotZero(t, parsed, "the census parsed no Go file under %s", root)
	return uses
}

// TestIdentifierUses_ReportsAPlantedMountAndNotTheDeclaration checks the
// census on planted source: each use of the middleware is reported at its
// line, and its declaration and a comment naming it are not.
func TestIdentifierUses_ReportsAPlantedMountAndNotTheDeclaration(t *testing.T) {
	const src = `package main

func AgentRequestV2Middleware() {}

// AgentRequestV2Middleware named in a comment is not a use.
func routes() {
	app.Use(middleware.AgentRequestV2Middleware(agents, nonces, cfg.Server.PublicOrigin))
	group.Use(AgentRequestV2Middleware)
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "planted.go", src, parser.SkipObjectResolution)
	require.NoError(t, err)
	assert.Equal(t, []string{"planted.go:7", "planted.go:8"}, identifierUses(fset, file, "AgentRequestV2Middleware"))
}

// TestAgentRequestAudienceStartupLine_SaysNoRouteAcceptsThemWhileNoneDoes
// holds the line's "no route accepts them yet" to the backend's source: the
// line says it, for a set and an unset AIM_PUBLIC_ORIGIN, while no Go file of
// the backend outside the tests uses AgentRequestV2Middleware, and does not
// once one does.
func TestAgentRequestAudienceStartupLine_SaysNoRouteAcceptsThemWhileNoneDoes(t *testing.T) {
	uses := backendUses(t, "PQCAgentMiddleware", "AgentRequestV2Middleware")
	require.NotEmpty(t, uses["PQCAgentMiddleware"],
		"the census finds no use of PQCAgentMiddleware, which main mounts, so it does not read the code that mounts routes")
	mounts := uses["AgentRequestV2Middleware"]
	t.Logf("uses outside the tests: PQCAgentMiddleware %d, AgentRequestV2Middleware %d", len(uses["PQCAgentMiddleware"]), len(mounts))
	for _, origin := range []string{"", "https://aim.example.com"} {
		line := agentRequestAudienceStartupLine(config.ServerConfig{PublicOrigin: origin})
		saysNone := strings.Contains(line, "no route accepts them yet")
		if len(mounts) > 0 {
			assert.False(t, saysNone, "AgentRequestV2Middleware is used at %s, and the startup line for AIM_PUBLIC_ORIGIN=%q still says no route accepts the requests: %q",
				strings.Join(mounts, ", "), origin, line)
		} else {
			assert.True(t, saysNone, "no Go file of the backend uses AgentRequestV2Middleware, and the startup line for AIM_PUBLIC_ORIGIN=%q does not say that no route accepts the requests: %q",
				origin, line)
		}
	}
}

// TestMainLogsTheAgentRequestAudienceLineOnce pins main to logging the line
// the tests above check, once, for the server settings config.Load read.
func TestMainLogsTheAgentRequestAudienceLineOnce(t *testing.T) {
	mainSrc := aim03ReadRepoFile(t, "apps/backend/cmd/server/main.go")
	assert.Equal(t, 1, strings.Count(mainSrc, "agentRequestAudienceStartupLine("))
	assert.Equal(t, 1, strings.Count(mainSrc, "log.Print(agentRequestAudienceStartupLine(cfg.Server))"))
}
