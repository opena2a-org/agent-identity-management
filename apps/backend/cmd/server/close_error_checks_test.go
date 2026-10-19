package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

// TestServerTestsCheckTheErrorFromClose fails on a Close call whose returned
// error is dropped in the server test files listed below.
//
// errcheck, one of golangci-lint's default linters, reports such a call, and
// scripts/sync-to-cloud.sh copies the .go files under apps/backend that are
// not protected into aim-cloud. This repository's .golangci.yml turns errcheck
// off, so this test is what holds these files to that rule here.
//
// The check reads syntax only: a call to a method named Close with no
// arguments, written as a statement (plain, deferred or after go), is
// reported. A Close that returns nothing, httptest.Server's for one, is
// reported as well; extend the check before one of these files calls it.
func TestServerTestsCheckTheErrorFromClose(t *testing.T) {
	files := []string{
		"aim07_health_ready_test.go",
		"registry_routes_drift_test.go",
		"verification_event_routes_test.go",
	}
	fset := token.NewFileSet()
	for _, name := range files {
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			var call *ast.CallExpr
			switch stmt := n.(type) {
			case *ast.ExprStmt:
				call, _ = stmt.X.(*ast.CallExpr)
			case *ast.DeferStmt:
				call = stmt.Call
			case *ast.GoStmt:
				call = stmt.Call
			}
			if call == nil {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Close" && len(call.Args) == 0 {
				t.Errorf("%s: %s drops the error it returns; check it, as require.NoError(t, %s) does",
					fset.Position(call.Pos()), types.ExprString(call), types.ExprString(call))
			}
			return true
		})
	}
}
