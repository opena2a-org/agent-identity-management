package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// environmentReaders are the functions that read the process environment.
var environmentReaders = map[string]map[string]bool{
	"os":      {"Getenv": true, "LookupEnv": true, "Environ": true, "ExpandEnv": true},
	"syscall": {"Getenv": true, "Environ": true},
}

// environmentReads returns every use of an environment reader in the Go
// source files matched by pattern, test files excepted, as file:line: name.
// A reader passed as a value counts, as a call does.
func environmentReads(t *testing.T, pattern string) (reads []string, parsed int) {
	t.Helper()
	names, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		parsed++
		// The name each reader package is imported under in this file.
		imported := map[string]string{}
		for _, spec := range file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil || environmentReaders[path] == nil {
				continue
			}
			local := path
			if spec.Name != nil {
				local = spec.Name.Name
			}
			imported[local] = path
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if path, ok := imported[pkg.Name]; ok && environmentReaders[path][sel.Sel.Name] {
				reads = append(reads, fmt.Sprintf("%s: %s.%s", fset.Position(sel.Pos()), pkg.Name, sel.Sel.Name))
			}
			return true
		})
	}
	return reads, parsed
}

// TestServerReadsTheEnvironmentOnlyThroughConfig fails when a non-test file
// of the server reads an environment variable itself. Every variable the
// server reads is a field of config.Config, read once by config.Load with one
// rule for empty and unset values, and pinned by the table in
// internal/config/testdata/environment_table.json; a second reader could give
// the same variable a second meaning.
func TestServerReadsTheEnvironmentOnlyThroughConfig(t *testing.T) {
	reads, parsed := environmentReads(t, "*.go")
	if parsed < 5 {
		t.Fatalf("parsed %d non-test files of cmd/server; expected main.go and its siblings", parsed)
	}
	if len(reads) > 0 {
		t.Errorf("cmd/server reads the environment outside internal/config; add the variable to config.Config, to config.Load, and to the environment table, then read the field:\n  %s",
			strings.Join(reads, "\n  "))
	}
}

// configDir is internal/config, relative to this package's directory.
const configDir = "../../internal/config"

// lookupEnvDefinition returns the position of the reader assigned to the
// package variable lookupEnv in internal/config/config.go, in the form
// environmentReads reports a read.
func lookupEnvDefinition(t *testing.T) string {
	t.Helper()
	name := filepath.Join(configDir, "config.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != 1 || value.Names[0].Name != "lookupEnv" || len(value.Values) != 1 {
				continue
			}
			sel, ok := value.Values[0].(*ast.SelectorExpr)
			if !ok {
				t.Fatalf("%s: lookupEnv is not assigned a reader of the os package", fset.Position(value.Pos()))
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				t.Fatalf("%s: lookupEnv is not assigned a reader of the os package", fset.Position(value.Pos()))
			}
			return fmt.Sprintf("%s: %s.%s", fset.Position(sel.Pos()), pkg.Name, sel.Sel.Name)
		}
	}
	t.Fatalf("%s declares no package variable lookupEnv", name)
	return ""
}

// TestConfigReadsTheEnvironmentOnlyThroughLookupEnv fails when a non-test
// file of internal/config reads an environment variable other than through
// the package variable lookupEnv. The environment table test records the
// names Load asks lookupEnv for, so a read that bypasses lookupEnv would be
// missing from the table without that test noticing.
func TestConfigReadsTheEnvironmentOnlyThroughLookupEnv(t *testing.T) {
	reads, parsed := environmentReads(t, filepath.Join(configDir, "*.go"))
	if parsed < 2 {
		t.Fatalf("parsed %d non-test files of internal/config; expected config.go and its siblings", parsed)
	}
	definition := lookupEnvDefinition(t)
	var outside []string
	found := false
	for _, read := range reads {
		if read == definition {
			found = true
			continue
		}
		outside = append(outside, read)
	}
	if !found {
		t.Errorf("the census found no read at the lookupEnv definition %s; reads found:\n  %s", definition, strings.Join(reads, "\n  "))
	}
	if len(outside) > 0 {
		t.Errorf("internal/config reads the environment other than through lookupEnv; read the variable through getenv, getEnv or lookupEnv and add it to the environment table:\n  %s",
			strings.Join(outside, "\n  "))
	}
}
