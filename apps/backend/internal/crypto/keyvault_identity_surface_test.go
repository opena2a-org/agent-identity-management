package crypto

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// masterKeyLineFormat is the format of the line the server prints once per
// start for its master key (cmd/server/main.go).
const masterKeyLineFormat = "KeyVault: master key source=%s storageKeyId=%d masterKeyIdentity=%s"

// identityLabel is the label masterKeyIdentity MACs under the master key.
const identityLabel = "opena2a-aim/keyvault-master-key/v1/key-id"

// identitySurface is what masterKeyIdentitySurface found in a source tree.
type identitySurface struct {
	findings  []string // each way the tree breaks the identity's one surface, as path:line: what
	read      []string // the production files read, relative to the root
	printFile string   // the file of the one print, when there is exactly one
}

// identitySourceFile is one parsed production file.
type identitySourceFile struct {
	rel     string // path relative to the root, with forward slashes
	dir     string // the package directory, relative to the root
	file    *ast.File
	imports map[string]string // local name to import path
}

var importVersionSuffix = regexp.MustCompile(`^v[0-9]+$`)

// importNames maps each name a file refers to an import by to its path: the
// explicit name, or else the last element of the path (the element before a
// /vN suffix). Blank and dot imports bind no name.
func importNames(f *ast.File) map[string]string {
	names := make(map[string]string, len(f.Imports))
	for _, spec := range f.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(p)
		if importVersionSuffix.MatchString(name) && path.Dir(p) != "." {
			name = path.Base(path.Dir(p))
		}
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "_" || name == "." {
			continue
		}
		names[name] = p
	}
	return names
}

// refersToImport reports whether expr is a name the file binds to importPath.
func (f *identitySourceFile) refersToImport(expr ast.Expr, importPath string) bool {
	id, ok := expr.(*ast.Ident)
	return ok && f.imports[id.Name] == importPath
}

// standardStream reports whether expr is os.Stdout or os.Stderr.
func (f *identitySourceFile) standardStream(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	return ok && f.refersToImport(sel.X, "os") && (sel.Sel.Name == "Stdout" || sel.Sel.Name == "Stderr")
}

// printFormatArg returns the argument of call that holds the printed text when
// call prints to standard output, standard error or the standard log package,
// decided by the import path of the called function: fmt.Print, Printf and
// Println; fmt.Fprint, Fprintf and Fprintln to os.Stdout or os.Stderr; and the
// log package's Print, Printf and Println. Any other call returns ok=false.
func (f *identitySourceFile) printFormatArg(call *ast.CallExpr) (ast.Expr, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 {
		return nil, false
	}
	switch {
	case f.refersToImport(sel.X, "fmt"):
		switch sel.Sel.Name {
		case "Print", "Printf", "Println":
			return call.Args[0], true
		case "Fprint", "Fprintf", "Fprintln":
			if len(call.Args) > 1 && f.standardStream(call.Args[0]) {
				return call.Args[1], true
			}
		}
	case f.refersToImport(sel.X, "log"):
		switch sel.Sel.Name {
		case "Print", "Printf", "Println":
			return call.Args[0], true
		}
	}
	return nil, false
}

// calleeName returns the name a call is made by: f in f(...) and x.f(...).
func calleeName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}

// inspectWithParents walks node and calls visit with each node and the nodes
// that enclose it, outermost first.
func inspectWithParents(node ast.Node, visit func(n ast.Node, parents []ast.Node)) {
	var stack []ast.Node
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		visit(n, stack)
		stack = append(stack, n)
		return true
	})
}

// readIdentitySources parses every Go file under root that is not a test,
// skipping vendor, node_modules, testdata and hidden directories.
func readIdentitySources(root string) ([]*identitySourceFile, *token.FileSet, error) {
	fset := token.NewFileSet()
	var files []*identitySourceFile
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); p != root && (name == "vendor" || name == "node_modules" || name == "testdata" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		files = append(files, &identitySourceFile{rel: rel, dir: path.Dir(rel), file: f, imports: importNames(f)})
		return nil
	})
	return files, fset, err
}

// masterKeyIdentitySurface reads the production Go files under root and
// reports each way they break the master key identity's one surface:
//
//   - exactly one function holds identityLabel (directly, or through a
//     package-level name bound to it), and it is the one that computes the
//     identity;
//   - the identity, from that function or from a function that returns it, is
//     used in exactly one place, as an argument of one print to standard
//     output, standard error or the standard log package (printFormatArg),
//     and that print is the masterKeyLineFormat line;
//   - no exported function returns the identity when the print is in the
//     package that computes it; otherwise exactly one does, and the print is
//     its one caller;
//   - no file calls slog.SetDefault, log.SetOutput or log.Default().SetOutput,
//     or assigns os.Stdout or os.Stderr, since each re-routes that print;
//   - no file under internal/interfaces, a repository or telemetry refers to
//     the identity.
//
// Since the one permitted use is the print, the identity also reaches no
// cipher, MAC, seed, signing call, JWK Set or error.
func masterKeyIdentitySurface(root string) (identitySurface, error) {
	files, fset, err := readIdentitySources(root)
	if err != nil {
		return identitySurface{}, err
	}
	var s identitySurface
	for _, f := range files {
		s.read = append(s.read, f.rel)
	}
	at := func(f *identitySourceFile, n ast.Node) string {
		return fmt.Sprintf("%s:%d", f.rel, fset.Position(n.Pos()).Line)
	}
	report := func(format string, args ...any) {
		s.findings = append(s.findings, fmt.Sprintf(format, args...))
	}

	// Output re-routing.
	for _, f := range files {
		ast.Inspect(f.file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				sel, ok := n.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch {
				case sel.Sel.Name == "SetDefault" && f.refersToImport(sel.X, "log/slog"):
					report("%s: calls slog.SetDefault, which re-routes the standard log package", at(f, n))
				case sel.Sel.Name == "SetOutput" && f.refersToImport(sel.X, "log"):
					report("%s: calls log.SetOutput, which re-routes the standard log package", at(f, n))
				case sel.Sel.Name == "SetOutput":
					if inner, ok := sel.X.(*ast.CallExpr); ok {
						if innerSel, ok := inner.Fun.(*ast.SelectorExpr); ok && innerSel.Sel.Name == "Default" && f.refersToImport(innerSel.X, "log") {
							report("%s: calls log.Default().SetOutput, which re-routes the standard log package", at(f, n))
						}
					}
				}
			case *ast.AssignStmt:
				for _, lhs := range n.Lhs {
					if f.standardStream(lhs) {
						report("%s: assigns %s", at(f, n), "os."+lhs.(*ast.SelectorExpr).Sel.Name)
					}
				}
			}
			return true
		})
	}

	// The label, and the functions that hold it.
	type funcRef struct {
		f    *identitySourceFile
		decl *ast.FuncDecl
	}
	labelLiterals := 0
	labelNames := map[string]string{} // package-level name bound to the label, to its package directory
	holders := map[*ast.FuncDecl]funcRef{}
	for _, f := range files {
		for _, decl := range f.file.Decls {
			ast.Inspect(decl, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil || !strings.Contains(value, identityLabel) {
					return true
				}
				labelLiterals++
				switch decl := decl.(type) {
				case *ast.FuncDecl:
					holders[decl] = funcRef{f, decl}
				case *ast.GenDecl:
					for _, spec := range decl.Specs {
						if vs, ok := spec.(*ast.ValueSpec); ok {
							for _, name := range vs.Names {
								labelNames[name.Name] = f.dir
							}
						}
					}
				}
				return true
			})
		}
	}
	for _, f := range files {
		for _, decl := range f.file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					if dir, ok := labelNames[id.Name]; ok && dir == f.dir {
						holders[fd] = funcRef{f, fd}
					}
				}
				return true
			})
		}
	}
	if labelLiterals != 1 {
		report("the identity label occurs %d times in production files; want 1", labelLiterals)
	}
	if len(holders) != 1 {
		var names []string
		for _, h := range holders {
			names = append(names, at(h.f, h.decl)+" "+h.decl.Name.Name)
		}
		report("%d functions hold the identity label, want 1: %s", len(holders), strings.Join(names, ", "))
		return s, nil
	}
	var compute funcRef
	for _, h := range holders {
		compute = h
	}

	// Providers: the computing function, and each function that returns a
	// call of a provider.
	providers := map[string]funcRef{compute.decl.Name.Name: compute}
	for changed := true; changed; {
		changed = false
		for _, f := range files {
			for _, decl := range f.file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				if _, ok := providers[fd.Name.Name]; ok {
					continue
				}
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					ret, ok := n.(*ast.ReturnStmt)
					if !ok {
						return true
					}
					for _, result := range ret.Results {
						if call, ok := result.(*ast.CallExpr); ok {
							if _, ok := providers[calleeName(call)]; ok {
								providers[fd.Name.Name] = funcRef{f, fd}
								changed = true
							}
						}
					}
					return true
				})
			}
		}
	}

	// Every reference to a provider.
	type printUse struct {
		f    *identitySourceFile
		call *ast.CallExpr
	}
	var prints []printUse
	for _, f := range files {
		inspectWithParents(f.file, func(n ast.Node, parents []ast.Node) {
			id, ok := n.(*ast.Ident)
			if !ok {
				return
			}
			if _, ok := providers[id.Name]; !ok || len(parents) == 0 {
				return
			}
			parent := parents[len(parents)-1]
			if fd, ok := parent.(*ast.FuncDecl); ok && fd.Name == id {
				return
			}
			if strings.HasPrefix(f.rel, "internal/interfaces/") || strings.Contains(f.rel, "repository") || strings.Contains(f.rel, "telemetry") {
				report("%s: refers to the identity (%s)", at(f, id), id.Name)
			}

			// The call made by this name, and what encloses that call.
			var call *ast.CallExpr
			var outer []ast.Node
			switch p := parent.(type) {
			case *ast.CallExpr:
				if p.Fun == id {
					call, outer = p, parents[:len(parents)-1]
				}
			case *ast.SelectorExpr:
				if p.Sel == id && len(parents) > 1 {
					if c, ok := parents[len(parents)-2].(*ast.CallExpr); ok && c.Fun == p {
						call, outer = c, parents[:len(parents)-2]
					}
				}
			}
			if call == nil {
				report("%s: refers to %s without calling it", at(f, id), id.Name)
				return
			}
			if len(outer) == 0 {
				report("%s: uses the identity outside a print", at(f, call))
				return
			}
			switch encl := outer[len(outer)-1].(type) {
			case *ast.ReturnStmt:
				for i := len(outer) - 1; i >= 0; i-- {
					if fd, ok := outer[i].(*ast.FuncDecl); ok {
						if _, ok := providers[fd.Name.Name]; ok {
							return
						}
						break
					}
				}
				report("%s: returns the identity from a function that is not a provider", at(f, call))
			case *ast.CallExpr:
				if _, ok := f.printFormatArg(encl); ok {
					prints = append(prints, printUse{f, encl})
					return
				}
				report("%s: passes the identity to a call that is not a print to standard output, standard error or the standard log package", at(f, encl))
			default:
				report("%s: uses the identity outside a print", at(f, call))
			}
		})
	}

	if len(prints) != 1 {
		var where []string
		for _, p := range prints {
			where = append(where, at(p.f, p.call))
		}
		report("the identity is printed %d times, want 1: %s", len(prints), strings.Join(where, ", "))
		return s, nil
	}
	p := prints[0]
	s.printFile = p.f.rel
	arg, _ := p.f.printFormatArg(p.call)
	format := ""
	if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		format, _ = strconv.Unquote(lit.Value)
	}
	if format != masterKeyLineFormat {
		report("%s: the print of the identity is not the %q line", at(p.f, p.call), masterKeyLineFormat)
	}

	var exported []string
	for name, ref := range providers {
		if ast.IsExported(name) {
			exported = append(exported, at(ref.f, ref.decl)+" "+name)
		}
	}
	switch {
	case p.f.dir == compute.f.dir && len(exported) > 0:
		report("exported functions return the identity while it is printed in the package that computes it: %s", strings.Join(exported, ", "))
	case p.f.dir != compute.f.dir && len(exported) != 1:
		report("%d exported functions return the identity, want exactly 1, the print's accessor: %s", len(exported), strings.Join(exported, ", "))
	}
	return s, nil
}

// The backend's production code computes the master key identity in one
// function, prints it in one place, the server's master key line, and does
// nothing else with it.
func TestMasterKeyIdentity_HasOneSurface(t *testing.T) {
	root := filepath.Join("..", "..")
	s, err := masterKeyIdentitySurface(root)
	require.NoError(t, err)
	for _, file := range []string{"cmd/server/main.go", "internal/crypto/keyvault.go", "internal/crypto/signing_keys.go"} {
		require.Contains(t, s.read, file, "the walk did not read %s", file)
	}
	assert.Empty(t, s.findings, "the master key identity has more than its one surface:\n%s", strings.Join(s.findings, "\n"))
	assert.Equal(t, "cmd/server/main.go", s.printFile)

	// The line follows the KeyVault init line directly and comes before the
	// signing keys load, which reads the same master key.
	src, err := os.ReadFile(filepath.Join(root, "cmd", "server", "main.go"))
	require.NoError(t, err)
	var statements []string
	for _, line := range strings.Split(string(src), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "//") {
			statements = append(statements, line)
		}
	}
	initAt, printAt, loadAt := -1, -1, -1
	for i, line := range statements {
		switch {
		case line == `log.Println("KeyVault initialized for automatic key generation")`:
			initAt = i
		case strings.HasPrefix(line, "log.Printf("+strconv.Quote(masterKeyLineFormat)+","):
			printAt = i
		case strings.Contains(line, "crypto.LoadSigningKeyRing("):
			loadAt = i
		}
	}
	require.NotEqual(t, -1, initAt, "the KeyVault init line is missing")
	assert.Equal(t, initAt+1, printAt, "the master key line is not the statement after the KeyVault init line")
	assert.Greater(t, loadAt, printAt, "the signing keys load does not come after the master key line")
}

// identitySurfaceTree is a small backend: a crypto package that computes the
// identity, a server that prints it in the master key line, and test files
// that the scan leaves alone.
func identitySurfaceTree() map[string]string {
	return map[string]string{
		"internal/crypto/keyvault.go": `package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

type KeyVault struct{ masterKey []byte }

func (kv *KeyVault) MasterKeySource() string { return "environment" }

func (kv *KeyVault) StorageKeyID() byte { return 1 }

func (kv *KeyVault) MasterKeyIdentity() string {
	return masterKeyIdentity(kv.masterKey)
}

func masterKeyIdentity(masterKey []byte) string {
	mac := hmac.New(sha256.New, masterKey)
	mac.Write([]byte("` + identityLabel + `"))
	return hex.EncodeToString(mac.Sum(nil)[:16])
}
`,
		"internal/crypto/keyvault_test.go": `package crypto

import "fmt"

func show(kv *KeyVault) {
	fmt.Println(masterKeyIdentity(kv.masterKey), "` + identityLabel + `")
}
`,
		"cmd/server/main.go": `package main

import (
	"log"

	"example.com/backend/internal/crypto"
)

func initServices(keyVault *crypto.KeyVault) {
	log.Println("KeyVault initialized for automatic key generation")
	log.Printf("` + masterKeyLineFormat + `", keyVault.MasterKeySource(), keyVault.StorageKeyID(), keyVault.MasterKeyIdentity())
}
`,
	}
}

func writeIdentitySurfaceTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	}
	return root
}

// Each change below, applied once to a tree the scan accepts, breaks the one
// surface, and the scan reports it.
func TestMasterKeyIdentitySurface_ReportsEachBreak(t *testing.T) {
	base := identitySurfaceTree()
	s, err := masterKeyIdentitySurface(writeIdentitySurfaceTree(t, base))
	require.NoError(t, err)
	require.Empty(t, s.findings, "the unchanged tree must pass, or the cases below prove nothing")
	require.Equal(t, "cmd/server/main.go", s.printFile)
	require.ElementsMatch(t, []string{"cmd/server/main.go", "internal/crypto/keyvault.go"}, s.read)

	mainGo := base["cmd/server/main.go"]
	keyvaultGo := base["internal/crypto/keyvault.go"]
	printLine := "\tlog.Printf(\"" + masterKeyLineFormat + "\", keyVault.MasterKeySource(), keyVault.StorageKeyID(), keyVault.MasterKeyIdentity())\n"
	accessor := "func (kv *KeyVault) MasterKeyIdentity() string {\n\treturn masterKeyIdentity(kv.masterKey)\n}\n"
	require.Contains(t, mainGo, printLine)
	require.Contains(t, keyvaultGo, accessor)

	cases := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name: "a second function that prints the identity",
			files: map[string]string{"cmd/server/main.go": mainGo + `
func again(keyVault *crypto.KeyVault) {
	log.Printf("identity %s", keyVault.MasterKeyIdentity())
}
`},
			want: "the identity is printed 2 times",
		},
		{
			name: "the print moved to telemetry.Logger",
			files: map[string]string{"cmd/server/main.go": strings.Replace(
				strings.Replace(mainGo, printLine, "\ttelemetry.Logger(\"aim\").Info(\"KeyVault: master key\", \"masterKeyIdentity\", keyVault.MasterKeyIdentity())\n", 1),
				"\t\"example.com/backend/internal/crypto\"\n", "\t\"example.com/backend/internal/crypto\"\n\t\"example.com/backend/internal/telemetry\"\n", 1)},
			want: "passes the identity to a call that is not a print",
		},
		{
			name: "a production slog.SetDefault call",
			files: map[string]string{"cmd/server/logging.go": `package main

import (
	"log/slog"
	"os"
)

func init() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
}
`},
			want: "calls slog.SetDefault",
		},
		{
			name: "an exported method that returns the identity",
			files: map[string]string{"internal/crypto/keyvault.go": keyvaultGo + `
func (kv *KeyVault) Identity() string {
	return kv.MasterKeyIdentity()
}
`},
			want: "2 exported functions return the identity",
		},
		{
			name: "the identity function exported while the print stays in its package",
			files: map[string]string{
				"internal/crypto/keyvault.go": strings.Replace(
					strings.Replace(strings.Replace(keyvaultGo, accessor, "", 1), "masterKeyIdentity(", "MasterKeyIdentityOf(", -1),
					"\t\"encoding/hex\"\n", "\t\"encoding/hex\"\n\t\"log\"\n", 1) + `
func printMasterKey(kv *KeyVault) {
	log.Printf("` + masterKeyLineFormat + `", "environment", 1, MasterKeyIdentityOf(kv.masterKey))
}
`,
				"cmd/server/main.go": strings.Replace(mainGo, printLine, "", 1),
			},
			want: "exported functions return the identity while it is printed in the package that computes it",
		},
		{
			name: "an assignment to os.Stdout",
			files: map[string]string{"cmd/server/quiet.go": `package main

import "os"

func quiet() { os.Stdout = os.Stderr }
`},
			want: "assigns os.Stdout",
		},
		{
			name: "a second function that holds the label",
			files: map[string]string{"internal/crypto/label.go": `package crypto

func otherLabel() []byte { return []byte("` + identityLabel + `") }
`},
			want: "2 functions hold the identity label",
		},
		{
			name: "a print through a *log.Logger from log.New",
			files: map[string]string{"cmd/server/main.go": strings.Replace(
				strings.Replace(mainGo, "log.Printf(", "logger.Printf(", 1),
				"\tlog.Println(", "\tlogger := log.New(os.Stderr, \"\", 0)\n\tlog.Println(", 1)},
			want: "passes the identity to a call that is not a print",
		},
		{
			name:  "a print through a package named log that is not the standard log package",
			files: map[string]string{"cmd/server/main.go": strings.Replace(mainGo, "\t\"log\"\n", "\tlog \"github.com/sirupsen/logrus\"\n", 1)},
			want:  "passes the identity to a call that is not a print",
		},
		{
			name: "a print to a writer other than standard output or standard error",
			files: map[string]string{"cmd/server/main.go": strings.Replace(
				strings.Replace(mainGo, "log.Printf(\"", "fmt.Fprintf(w, \"", 1),
				"\t\"log\"\n", "\t\"fmt\"\n\t\"log\"\n", 1)},
			want: "passes the identity to a call that is not a print",
		},
		{
			name:  "the identity printed in a line other than the master key line",
			files: map[string]string{"cmd/server/main.go": strings.Replace(mainGo, masterKeyLineFormat, "KeyVault ready %s %d %s", 1)},
			want:  "the print of the identity is not the",
		},
		{
			name: "the identity in an error",
			files: map[string]string{"internal/crypto/errors.go": `package crypto

import "fmt"

func refuse(kv *KeyVault) error { return fmt.Errorf("refused for %s", kv.MasterKeyIdentity()) }
`},
			want: "passes the identity to a call that is not a print",
		},
		{
			name: "the identity in a repository",
			files: map[string]string{"internal/infrastructure/repository/keys.go": `package repository

import "example.com/backend/internal/crypto"

func save(kv *crypto.KeyVault) string {
	id := kv.MasterKeyIdentity()
	return id
}
`},
			want: "refers to the identity (MasterKeyIdentity)",
		},
		{
			name: "a log.SetOutput call",
			files: map[string]string{"cmd/server/logging.go": `package main

import (
	"io"
	"log"
)

func init() { log.SetOutput(io.Discard) }
`},
			want: "calls log.SetOutput",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := identitySurfaceTree()
			for rel, body := range tc.files {
				files[rel] = body
			}
			s, err := masterKeyIdentitySurface(writeIdentitySurfaceTree(t, files))
			require.NoError(t, err)
			require.NotEmpty(t, s.findings, "the scan accepted the change")
			assert.Contains(t, strings.Join(s.findings, "\n"), tc.want)
		})
	}
}

// The scan decides a print by the import path of the called function, not by
// its name, and accepts the print in the computing package while the
// identity function stays unexported.
func TestMasterKeyIdentitySurface_AcceptsEquivalentPrints(t *testing.T) {
	base := identitySurfaceTree()
	mainGo := base["cmd/server/main.go"]
	keyvaultGo := base["internal/crypto/keyvault.go"]
	printLine := "\tlog.Printf(\"" + masterKeyLineFormat + "\", keyVault.MasterKeySource(), keyVault.StorageKeyID(), keyVault.MasterKeyIdentity())\n"
	accessor := "func (kv *KeyVault) MasterKeyIdentity() string {\n\treturn masterKeyIdentity(kv.masterKey)\n}\n"
	require.Contains(t, mainGo, printLine)
	require.Contains(t, keyvaultGo, accessor)

	cases := []struct {
		name      string
		files     map[string]string
		printFile string
	}{
		{
			name:      "the standard log package under another name",
			files:     map[string]string{"cmd/server/main.go": strings.Replace(strings.Replace(mainGo, "\t\"log\"\n", "\tstdlog \"log\"\n", 1), "log.Print", "stdlog.Print", -1)},
			printFile: "cmd/server/main.go",
		},
		{
			name: "fmt.Fprintf to os.Stderr",
			files: map[string]string{"cmd/server/main.go": strings.Replace(
				strings.Replace(mainGo, "log.Printf(\"", "fmt.Fprintf(os.Stderr, \"", 1),
				"\t\"log\"\n", "\t\"fmt\"\n\t\"log\"\n\t\"os\"\n", 1)},
			printFile: "cmd/server/main.go",
		},
		{
			name: "the print in the computing package, the identity function unexported",
			files: map[string]string{
				"internal/crypto/keyvault.go": strings.Replace(
					strings.Replace(keyvaultGo, accessor, "", 1),
					"\t\"encoding/hex\"\n", "\t\"encoding/hex\"\n\t\"log\"\n", 1) + `
func printMasterKey(kv *KeyVault) {
	log.Printf("` + masterKeyLineFormat + `", "environment", 1, masterKeyIdentity(kv.masterKey))
}
`,
				"cmd/server/main.go": strings.Replace(mainGo, printLine, "", 1),
			},
			printFile: "internal/crypto/keyvault.go",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := identitySurfaceTree()
			for rel, body := range tc.files {
				files[rel] = body
			}
			s, err := masterKeyIdentitySurface(writeIdentitySurfaceTree(t, files))
			require.NoError(t, err)
			assert.Empty(t, s.findings)
			assert.Equal(t, tc.printFile, s.printFile)
		})
	}
}
