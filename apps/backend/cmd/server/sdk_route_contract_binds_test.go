package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Bind structs: the server side, read with go/parser.

// bindIndex parses the backend's packages on demand to find the struct a
// handler binds its JSON body into.
type bindIndex struct {
	backend string
	module  string
	fset    *token.FileSet
	pkgs    map[string]*bindPkg
}

type bindPkg struct {
	types    map[string]bindDecl[*ast.TypeSpec]
	funcs    map[string]bindDecl[*ast.FuncDecl] // "Type.Method", or "Func"
	unmarshs map[string]bool                    // types with their own UnmarshalJSON
}

type bindDecl[T any] struct {
	decl T
	file *ast.File
}

// bindTarget is one variable a handler decodes the request body into, with
// the types declared inside the function that declares it.
type bindTarget struct {
	typ      ast.Expr
	file     *ast.File
	pkg      *bindPkg
	locals   map[string]*ast.TypeSpec
	flexible bool
}

func newBindIndex(t *testing.T, backend string) *bindIndex {
	t.Helper()
	gomod, err := os.ReadFile(filepath.Join(backend, "go.mod")) //nolint:gosec // G304: the repository's own go.mod
	require.NoError(t, err)
	m := regexp.MustCompile(`(?m)^module\s+(\S+)`).FindSubmatch(gomod)
	require.NotNil(t, m, "apps/backend/go.mod names no module")
	return &bindIndex{backend: backend, module: string(m[1]), fset: token.NewFileSet(), pkgs: map[string]*bindPkg{}}
}

func (ix *bindIndex) pkg(importPath string) (*bindPkg, error) {
	if p, ok := ix.pkgs[importPath]; ok {
		return p, nil
	}
	rel, ok := strings.CutPrefix(importPath, ix.module)
	if !ok {
		return nil, fmt.Errorf("package %s is outside the backend module", importPath)
	}
	names, err := filepath.Glob(filepath.Join(ix.backend, filepath.FromSlash(rel), "*.go"))
	if err != nil || len(names) == 0 {
		return nil, fmt.Errorf("package %s has no Go files", importPath)
	}
	p := &bindPkg{
		types:    map[string]bindDecl[*ast.TypeSpec]{},
		funcs:    map[string]bindDecl[*ast.FuncDecl]{},
		unmarshs: map[string]bool{},
	}
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(ix.fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						p.types[ts.Name.Name] = bindDecl[*ast.TypeSpec]{ts, file}
					}
				}
			case *ast.FuncDecl:
				key := d.Name.Name
				if d.Recv != nil && len(d.Recv.List) == 1 {
					recv := receiverTypeName(d.Recv.List[0].Type)
					key = recv + "." + key
					if d.Name.Name == "UnmarshalJSON" {
						p.unmarshs[recv] = true
					}
				}
				p.funcs[key] = bindDecl[*ast.FuncDecl]{d, file}
			}
		}
	}
	ix.pkgs[importPath] = p
	return p, nil
}

func receiverTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(e.X)
	case *ast.IndexExpr:
		return receiverTypeName(e.X)
	case *ast.Ident:
		return e.Name
	}
	return ""
}

// bodyFields returns the JSON names of the fields of each struct the handler
// decodes the body into; none when the handler decodes no body.
func (ix *bindIndex) bodyFields(handler string) (boundBody, error) {
	m := boundHandlerName.FindStringSubmatch(handler)
	if m == nil {
		return nil, fmt.Errorf("%s is not a handler method", handler)
	}
	p, err := ix.pkg(m[1])
	if err != nil {
		return nil, err
	}
	fn, ok := p.funcs[m[2]+"."+m[3]]
	if !ok {
		return nil, fmt.Errorf("no declaration of %s.%s in %s", m[2], m[3], m[1])
	}
	targets, err := ix.boundTargets(p, fn, 0)
	if err != nil {
		return nil, fmt.Errorf("%s.%s: %w", m[2], m[3], err)
	}
	var bound boundBody
	for _, target := range targets {
		fields := map[string]bool{}
		if err := ix.typeFields(target.typ, target.file, target.pkg, target.locals, fields, 0); err != nil {
			return nil, fmt.Errorf("%s.%s binds a type the test cannot read: %w", m[2], m[3], err)
		}
		bound = append(bound, boundStruct{fields, target.flexible})
	}
	return bound, nil
}

// boundTargets finds every variable fn decodes the request body into: through
// c.Bind().JSON, c.Bind().Body, json.Unmarshal(c.Body(), ...) or
// utils.UnmarshalFlexibleJSON(c.Body(), ...), in fn or in a function of the
// same package fn hands its fiber.Ctx to.
func (ix *bindIndex) boundTargets(p *bindPkg, fn bindDecl[*ast.FuncDecl], depth int) ([]bindTarget, error) {
	if fn.decl.Body == nil {
		return nil, nil
	}
	ctxNames := map[string]bool{}
	for _, field := range fn.decl.Type.Params.List {
		if sel, ok := field.Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "Ctx" {
			for _, n := range field.Names {
				ctxNames[n.Name] = true
			}
		}
	}
	recvName := ""
	if fn.decl.Recv != nil && len(fn.decl.Recv.List) == 1 && len(fn.decl.Recv.List[0].Names) == 1 {
		recvName = fn.decl.Recv.List[0].Names[0].Name
	}
	recvType := ""
	if fn.decl.Recv != nil && len(fn.decl.Recv.List) == 1 {
		recvType = receiverTypeName(fn.decl.Recv.List[0].Type)
	}

	locals := map[string]*ast.TypeSpec{}
	ast.Inspect(fn.decl.Body, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok {
			locals[ts.Name.Name] = ts
		}
		return true
	})

	var out []bindTarget
	var firstErr error
	ast.Inspect(fn.decl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || firstErr != nil {
			return firstErr == nil
		}
		if arg, flexible := bodyBindArgument(call); arg != nil {
			if u, ok := arg.(*ast.UnaryExpr); ok && u.Op == token.AND {
				arg = u.X
			}
			id, ok := arg.(*ast.Ident)
			if !ok {
				firstErr = fmt.Errorf("decodes the body into %T, not a variable", arg)
				return false
			}
			typ := variableType(fn.decl, id.Name)
			if typ == nil {
				firstErr = fmt.Errorf("decodes the body into %s, whose type the test cannot find", id.Name)
				return false
			}
			out = append(out, bindTarget{typ, fn.file, p, locals, flexible})
			return true
		}
		if depth >= 3 || !passesCtx(call, ctxNames) {
			return true
		}
		var callee bindDecl[*ast.FuncDecl]
		found := false
		switch f := call.Fun.(type) {
		case *ast.SelectorExpr:
			if x, ok := f.X.(*ast.Ident); ok && x.Name == recvName && recvName != "" {
				callee, found = p.funcs[recvType+"."+f.Sel.Name]
			}
		case *ast.Ident:
			callee, found = p.funcs[f.Name]
		}
		if found {
			more, err := ix.boundTargets(p, callee, depth+1)
			if err != nil {
				firstErr = err
				return false
			}
			out = append(out, more...)
		}
		return true
	})
	return out, firstErr
}

// bodyBindArgument returns the destination of a call that decodes the request
// body, and whether the decoder turns snake_case keys into camelCase; nil when
// the call decodes no request body.
func bodyBindArgument(call *ast.CallExpr) (ast.Expr, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 {
		return nil, false
	}
	readsBody := func(arg ast.Expr) bool {
		src, ok := arg.(*ast.CallExpr)
		if !ok {
			return false
		}
		s, ok := src.Fun.(*ast.SelectorExpr)
		return ok && (s.Sel.Name == "Body" || s.Sel.Name == "BodyRaw")
	}
	switch sel.Sel.Name {
	case "JSON", "Body":
		if inner, ok := sel.X.(*ast.CallExpr); ok {
			if s, ok := inner.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "Bind" && len(call.Args) == 1 {
				return call.Args[0], false
			}
		}
	case "Unmarshal":
		if x, ok := sel.X.(*ast.Ident); ok && x.Name == "json" && len(call.Args) == 2 && readsBody(call.Args[0]) {
			return call.Args[1], false
		}
	case "UnmarshalFlexibleJSON":
		if len(call.Args) == 2 && readsBody(call.Args[0]) {
			return call.Args[1], true
		}
	}
	return nil, false
}

func passesCtx(call *ast.CallExpr, ctxNames map[string]bool) bool {
	for _, arg := range call.Args {
		if id, ok := arg.(*ast.Ident); ok && ctxNames[id.Name] {
			return true
		}
	}
	return false
}

// variableType returns the declared type of a variable in fn: a parameter,
// "var x T", "x := T{}", "x := &T{}" or "x := new(T)".
func variableType(fn *ast.FuncDecl, name string) ast.Expr {
	for _, field := range fn.Type.Params.List {
		for _, n := range field.Names {
			if n.Name == name {
				return field.Type
			}
		}
	}
	var typ ast.Expr
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if typ != nil {
			return false
		}
		switch s := n.(type) {
		case *ast.ValueSpec:
			for i, id := range s.Names {
				if id.Name != name {
					continue
				}
				if s.Type != nil {
					typ = s.Type
				} else if i < len(s.Values) {
					typ = typeOfValue(s.Values[i])
				}
			}
		case *ast.AssignStmt:
			if s.Tok != token.DEFINE {
				return true
			}
			for i, lhs := range s.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && id.Name == name && i < len(s.Rhs) {
					typ = typeOfValue(s.Rhs[i])
				}
			}
		}
		return true
	})
	return typ
}

func typeOfValue(v ast.Expr) ast.Expr {
	switch e := v.(type) {
	case *ast.CompositeLit:
		return e.Type
	case *ast.UnaryExpr:
		if lit, ok := e.X.(*ast.CompositeLit); ok && e.Op == token.AND {
			return lit.Type
		}
	case *ast.CallExpr:
		if id, ok := e.Fun.(*ast.Ident); ok && id.Name == "new" && len(e.Args) == 1 {
			return e.Args[0]
		}
	}
	return nil
}

// typeFields adds the JSON names encoding/json decodes into the type: a
// field's json tag name, or its Go name when it has none, with the fields of
// untagged embedded structs promoted.
func (ix *bindIndex) typeFields(expr ast.Expr, file *ast.File, p *bindPkg, locals map[string]*ast.TypeSpec, out map[string]bool, depth int) error {
	if depth > 8 {
		return fmt.Errorf("types nest too deep")
	}
	switch t := expr.(type) {
	case *ast.StarExpr:
		return ix.typeFields(t.X, file, p, locals, out, depth+1)
	case *ast.ParenExpr:
		return ix.typeFields(t.X, file, p, locals, out, depth+1)
	case *ast.StructType:
		for _, field := range t.Fields.List {
			name := ""
			if field.Tag != nil {
				tag, err := strconv.Unquote(field.Tag.Value)
				if err != nil {
					return err
				}
				name, _, _ = strings.Cut(reflect.StructTag(tag).Get("json"), ",")
			}
			switch {
			case name == "-":
				// json:"-" is never decoded.
			case name != "":
				out[name] = true
			case len(field.Names) == 0:
				if err := ix.typeFields(field.Type, file, p, locals, out, depth+1); err != nil {
					return err
				}
			default:
				for _, n := range field.Names {
					if n.IsExported() {
						out[n.Name] = true
					}
				}
			}
		}
		return nil
	case *ast.Ident:
		if local, ok := locals[t.Name]; ok {
			return ix.typeFields(local.Type, file, p, nil, out, depth+1)
		}
		decl, ok := p.types[t.Name]
		if !ok {
			return fmt.Errorf("%s is not a struct type of its package", t.Name)
		}
		if p.unmarshs[t.Name] {
			return fmt.Errorf("%s decodes itself with UnmarshalJSON", t.Name)
		}
		return ix.typeFields(decl.decl.Type, decl.file, p, nil, out, depth+1)
	case *ast.SelectorExpr:
		alias, ok := t.X.(*ast.Ident)
		if !ok {
			return fmt.Errorf("unsupported type %T", t.X)
		}
		other, err := ix.pkg(importPathFor(file, alias.Name))
		if err != nil {
			return err
		}
		decl, ok := other.types[t.Sel.Name]
		if !ok {
			return fmt.Errorf("%s.%s is not declared", alias.Name, t.Sel.Name)
		}
		if other.unmarshs[t.Sel.Name] {
			return fmt.Errorf("%s.%s decodes itself with UnmarshalJSON", alias.Name, t.Sel.Name)
		}
		return ix.typeFields(decl.decl.Type, decl.file, other, nil, out, depth+1)
	}
	return fmt.Errorf("the body is decoded into a %s, not a struct", strings.TrimPrefix(fmt.Sprintf("%T", expr), "*ast."))
}

func importPathFor(file *ast.File, alias string) string {
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		name := path[strings.LastIndex(path, "/")+1:]
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name == alias {
			return path
		}
	}
	return alias
}
