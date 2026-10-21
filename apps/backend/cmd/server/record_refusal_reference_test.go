package main

// docs/API.md and the dashboard's API reference (apps/web/lib/api-documentation.ts)
// describe the 503 recordUnavailable refusal of a verification, a reactivation, an
// agent delete, a capability request approval and an addition of MCP servers. The
// services make those changes only together with an audit record when a transition
// recorder is set on them, so the reference states the refusal under that condition,
// and says that this server answers none of them for as long as no non-test file of
// the backend builds or sets a recorder.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// transitionRecorderWiring returns "path: function" for each call of
// transition.NewRecorder, or of a method named SetTransitionRecorder, in the Go
// files under apps/backend, test files only when withTests is set. A path is
// relative to apps/backend, and a call outside a function is in "package scope".
// Directories named testdata or vendor are not read.
func transitionRecorderWiring(t *testing.T, withTests bool) []string {
	t.Helper()
	backend := filepath.Join(aim03RepoRoot(t), "apps", "backend")
	var found []string
	err := filepath.WalkDir(backend, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || (!withTests && strings.HasSuffix(path, "_test.go")) {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(backend, path)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			scope := "package scope"
			if fn, ok := decl.(*ast.FuncDecl); ok {
				scope = fn.Name.Name
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				builds := false
				switch fun := call.Fun.(type) {
				case *ast.SelectorExpr:
					builds = fun.Sel.Name == "SetTransitionRecorder" ||
						(fun.Sel.Name == "NewRecorder" && identName(fun.X) == "transition")
				case *ast.Ident:
					builds = fun.Name == "NewRecorder" && file.Name.Name == "transition"
				}
				if builds {
					found = append(found, filepath.ToSlash(rel)+": "+scope)
				}
				return true
			})
		}
		return nil
	})
	require.NoError(t, err)
	return found
}

// The reference's statements that this server answers no record refusal, each
// with the file that carries it.
var recordRefusalNotAnswered = []struct{ file, statement string }{
	{"docs/API.md", "This version of the server writes no audit record for a verification or a reactivation, so neither route answers this `503`."},
	{"docs/API.md", "This version of the server writes no audit record for a delete, so this route does not answer this `503`."},
	{"apps/web/lib/api-documentation.ts", "This version of the server writes no audit record for an approval, so it does not answer this 503."},
	{"apps/web/lib/api-documentation.ts", "This version of the server writes no audit record for an addition, only an audit log entry, so it does not answer this 503."},
}

// The sentences with which the reference promised a record refusal before it
// stated the refusal's condition.
var recordRefusalUnconditionalPromises = []struct{ file, sentence string }{
	{"docs/API.md", "A verification is made only together with its audit record"},
	{"docs/API.md", "A delete is made only together with its audit record."},
	{"apps/web/lib/api-documentation.ts", "The approval and its grant are made only together with their audit record;"},
	{"apps/web/lib/api-documentation.ts", "An addition is made only together with its audit record;"},
}

func TestAPIReference_StatesTheRecordRefusalOnlyForAServerThatWritesTheRecord(t *testing.T) {
	requirePublicRepositoryFiles(t)
	wiring := transitionRecorderWiring(t, false)

	for _, p := range recordRefusalUnconditionalPromises {
		assert.False(t, strings.Contains(aim03ReadRepoFile(t, p.file), p.sentence),
			"%s promises the record refusal without the condition under which the server answers it: %q", p.file, p.sentence)
	}
	for _, s := range recordRefusalNotAnswered {
		stated := strings.Contains(aim03ReadRepoFile(t, s.file), s.statement)
		if len(wiring) == 0 {
			assert.True(t, stated,
				"no non-test file builds or sets a transition recorder, and %s does not say %q", s.file, s.statement)
			continue
		}
		assert.False(t, stated,
			"a transition recorder is built or set in %s, and %s says %q", strings.Join(wiring, ", "), s.file, s.statement)
	}
}

// The negative control: the test files build and set transition recorders, and
// the search finds those calls.
func TestTransitionRecorderWiring_FindsTheCallsInTheTestFiles(t *testing.T) {
	found := transitionRecorderWiring(t, true)
	assert.Contains(t, found, "internal/interfaces/http/handlers/capability_request_record_refusal_test.go: approveCapabilityRequestApp")
}
