// jsonarray-lint enforces the slice-initialization invariant for handlers and
// repositories: a `var xs []T` declaration produces a nil slice that marshals
// to JSON `null`, which crashes the React dashboard when it tries to iterate
// the field with `.forEach` / `.map` / `.filter`. Use `xs := make([]T, 0)`
// instead so empty lists serialize as `[]`.
//
// The lint walks every non-test .go file under the supplied directories,
// parses each, and flags any function-body `var xs []T` declaration whose
// element type is NOT one of the known scratch types:
//
//   - []byte         (JSON marshal/unmarshal target, pq Scan target)
//   - []rune         (string-builder scratch)
//   - []interface{}  (sql query args slice)
//   - []map[K]V      (json.Unmarshal scratch target)
//
// Findings are reported as file:line: var <name> []<type> ...; replace with
// <name> := make([]<type>, 0). Exit code 1 if any findings; 0 otherwise.
//
// Baseline support: pass --baseline <path> to a file of file:line entries to
// allowlist during migration. Currently empty; all known offenders were fixed
// in PR (hardening/p0-array-null) alongside this lint.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type finding struct {
	File string
	Line int
	Name string
	Type string
}

func (f finding) Location() string {
	return fmt.Sprintf("%s:%d", f.File, f.Line)
}

func main() {
	baselinePath := flag.String("baseline", "", "path to baseline file of file:line entries to allowlist")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: jsonarray-lint [--baseline path] <dir> [<dir>...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	dirs := flag.Args()
	if len(dirs) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	baseline, err := loadBaseline(*baselinePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "jsonarray-lint: load baseline: %v\n", err)
		os.Exit(2)
	}

	var findings []finding
	for _, dir := range dirs {
		fs, err := scanDir(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "jsonarray-lint: scan %s: %v\n", dir, err)
			os.Exit(2)
		}
		findings = append(findings, fs...)
	}

	var reported []finding
	for _, f := range findings {
		if _, ok := baseline[f.Location()]; ok {
			continue
		}
		reported = append(reported, f)
	}
	sort.Slice(reported, func(i, j int) bool {
		if reported[i].File != reported[j].File {
			return reported[i].File < reported[j].File
		}
		return reported[i].Line < reported[j].Line
	})

	for _, f := range reported {
		fmt.Printf("%s: var %s []%s; replace with %s := make([]%s, 0)\n",
			f.Location(), f.Name, f.Type, f.Name, f.Type)
	}

	if len(reported) > 0 {
		fmt.Fprintf(os.Stderr, "\njsonarray-lint: %d nil-slice declarations found.\n", len(reported))
		fmt.Fprintf(os.Stderr, "Replace with make([]T, 0) so empty lists marshal as [] not null.\n")
		os.Exit(1)
	}
}

func loadBaseline(path string) (map[string]struct{}, error) {
	if path == "" {
		return map[string]struct{}{}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]struct{}{}, nil
		}
		return nil, err
	}
	defer f.Close()

	entries := map[string]struct{}{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries[line] = struct{}{}
	}
	return entries, scanner.Err()
}

func scanDir(dir string) ([]finding, error) {
	var findings []finding
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fs, err := scanFile(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		findings = append(findings, fs...)
		return nil
	})
	return findings, err
}

func scanFile(path string) ([]finding, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}

	var findings []finding
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}
		for _, stmt := range fn.Body.List {
			collectVarSliceDecls(stmt, fset, path, &findings)
		}
		return true
	})
	return findings, nil
}

func collectVarSliceDecls(stmt ast.Stmt, fset *token.FileSet, path string, out *[]finding) {
	switch s := stmt.(type) {
	case *ast.DeclStmt:
		gen, ok := s.Decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			return
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			if vs.Type == nil || len(vs.Values) > 0 {
				continue
			}
			arr, ok := vs.Type.(*ast.ArrayType)
			if !ok || arr.Len != nil {
				continue
			}
			if isExemptElement(arr.Elt) {
				continue
			}
			for _, name := range vs.Names {
				if name.Name == "_" {
					continue
				}
				pos := fset.Position(name.Pos())
				*out = append(*out, finding{
					File: path,
					Line: pos.Line,
					Name: name.Name,
					Type: exprString(arr.Elt),
				})
			}
		}
	case *ast.IfStmt:
		if s.Body != nil {
			for _, c := range s.Body.List {
				collectVarSliceDecls(c, fset, path, out)
			}
		}
		if s.Else != nil {
			collectVarSliceDecls(s.Else, fset, path, out)
		}
	case *ast.BlockStmt:
		for _, c := range s.List {
			collectVarSliceDecls(c, fset, path, out)
		}
	case *ast.ForStmt:
		if s.Body != nil {
			for _, c := range s.Body.List {
				collectVarSliceDecls(c, fset, path, out)
			}
		}
	case *ast.RangeStmt:
		if s.Body != nil {
			for _, c := range s.Body.List {
				collectVarSliceDecls(c, fset, path, out)
			}
		}
	case *ast.SwitchStmt:
		if s.Body != nil {
			for _, c := range s.Body.List {
				if cc, ok := c.(*ast.CaseClause); ok {
					for _, cs := range cc.Body {
						collectVarSliceDecls(cs, fset, path, out)
					}
				}
			}
		}
	case *ast.TypeSwitchStmt:
		if s.Body != nil {
			for _, c := range s.Body.List {
				if cc, ok := c.(*ast.CaseClause); ok {
					for _, cs := range cc.Body {
						collectVarSliceDecls(cs, fset, path, out)
					}
				}
			}
		}
	case *ast.SelectStmt:
		if s.Body != nil {
			for _, c := range s.Body.List {
				if cc, ok := c.(*ast.CommClause); ok {
					for _, cs := range cc.Body {
						collectVarSliceDecls(cs, fset, path, out)
					}
				}
			}
		}
	}
}

func isExemptElement(elt ast.Expr) bool {
	switch e := elt.(type) {
	case *ast.Ident:
		return e.Name == "byte" || e.Name == "rune"
	case *ast.InterfaceType:
		return true
	case *ast.MapType:
		return true
	}
	return false
}

func exprString(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return "*" + exprString(x.X)
	case *ast.SelectorExpr:
		return exprString(x.X) + "." + x.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprString(x.Elt)
	case *ast.MapType:
		return "map[" + exprString(x.Key) + "]" + exprString(x.Value)
	case *ast.InterfaceType:
		return "interface{}"
	}
	return "?"
}
