package main

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// passwordLeakWindow is the shortest run of password characters that counts
// as a leak: a log line that prints four characters of a password leaks it.
const passwordLeakWindow = 4

// captureProcessOutput runs fn with the standard logger, os.Stdout and
// os.Stderr all writing to one pipe, and returns what was written.
func captureProcessOutput(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	read := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		r.Close()
		read <- buf.String()
	}()

	stdout, stderr, logOut := os.Stdout, os.Stderr, log.Writer()
	func() {
		defer func() {
			os.Stdout, os.Stderr = stdout, stderr
			log.SetOutput(logOut)
			w.Close()
		}()
		os.Stdout, os.Stderr = w, w
		log.SetOutput(w)
		fn()
	}()
	return <-read
}

// drawPassword returns n characters of alphabet chosen with crypto/rand,
// redrawn until keep accepts them.
func drawPassword(t *testing.T, alphabet string, n int, keep func(string) bool) string {
	t.Helper()
	for range 1000 {
		raw := make([]byte, n)
		if _, err := rand.Read(raw); err != nil {
			t.Fatalf("crypto/rand: %v", err)
		}
		out := make([]byte, n)
		for i, b := range raw {
			out[i] = alphabet[int(b)%len(alphabet)]
		}
		if keep(string(out)) {
			return string(out)
		}
	}
	t.Fatal("no planted password was drawn in 1000 attempts")
	return ""
}

// leakedOffsets returns the offsets of every window of password that occurs
// in text. It never returns the characters themselves.
func leakedOffsets(text, password string) []int {
	var offsets []int
	for i := 0; i+passwordLeakWindow <= len(password); i++ {
		if strings.Contains(text, password[i:i+passwordLeakWindow]) {
			offsets = append(offsets, i)
		}
	}
	return offsets
}

// runSeedPaths drives seedAdminFromEnv down every path it has, logging each
// returned error through the standard logger the way main does.
func runSeedPaths(t *testing.T, strong, weak string) {
	t.Helper()
	t.Setenv("ADMIN_EMAIL", "")
	t.Setenv("ADMIN_NAME", "")
	logSeedErr := func(err error) {
		if err != nil {
			log.Printf("seed: %v", err)
		}
	}
	expectAdminOrg := func(mock sqlmock.Sqlmock) {
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM organizations WHERE domain = $1`)).
			WithArgs(adminSeedOrgDomain).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("11111111-1111-1111-1111-111111111111"))
	}

	t.Setenv("ADMIN_PASSWORD", strong)

	// An administrator already exists.
	db, mock := newSeedMock(t)
	expectAdminExists(mock, true)
	logSeedErr(seedAdminFromEnv(db))

	// No administrator yet: one is seeded.
	db, mock = newSeedMock(t)
	expectAdminExists(mock, false)
	expectAdminOrg(mock)
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO users`)).WillReturnResult(sqlmock.NewResult(0, 1))
	logSeedErr(seedAdminFromEnv(db))

	// The insert fails.
	db, mock = newSeedMock(t)
	expectAdminExists(mock, false)
	expectAdminOrg(mock)
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO users`)).WillReturnError(errors.New("insert refused"))
	logSeedErr(seedAdminFromEnv(db))

	// The admin organization is missing.
	db, mock = newSeedMock(t)
	expectAdminExists(mock, false)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id FROM organizations WHERE domain = $1`)).
		WithArgs(adminSeedOrgDomain).
		WillReturnError(errors.New("sql: no rows in result set"))
	logSeedErr(seedAdminFromEnv(db))

	// The existence check fails.
	db, mock = newSeedMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS`)).WillReturnError(errors.New("connection reset"))
	logSeedErr(seedAdminFromEnv(db))

	// A weak password is refused.
	t.Setenv("ADMIN_PASSWORD", weak)
	db, mock = newSeedMock(t)
	expectAdminExists(mock, false)
	logSeedErr(seedAdminFromEnv(db))
}

// No log line the first-administrator seed writes, on any of its paths and
// including the errors main logs, carries any four characters of the password.
// A canary written through the same logger and streams in the same run must be
// found, so an empty capture cannot pass.
func TestSeedAdminFromEnv_LogsNoPartOfThePassword(t *testing.T) {
	const strongAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!#"
	const weakAlphabet = "abcdefghijklmnopqrstuvwxyz"
	const plantedLength = 32
	hasher := auth.NewPasswordHasher()
	isStrong := func(p string) bool { return hasher.ValidatePassword(p) == nil }
	isWeak := func(p string) bool { return errors.Is(hasher.ValidatePassword(p), auth.ErrPasswordTooWeak) }

	canaries := map[string]string{
		"standard logger": rand.Text(),
		"os.Stdout":       rand.Text(),
		"os.Stderr":       rand.Text(),
	}

	// What the same paths log for a different password, plus the canaries.
	// The planted passwords share no window with it, so a window found later
	// can only have come from a planted password.
	background := captureProcessOutput(t, func() {
		runSeedPaths(t,
			drawPassword(t, strongAlphabet, plantedLength, isStrong),
			drawPassword(t, weakAlphabet, plantedLength, isWeak))
	})
	for _, canary := range canaries {
		background += "\n" + canary
	}
	unseen := func(p string) bool { return len(leakedOffsets(background, p)) == 0 }
	strong := drawPassword(t, strongAlphabet, plantedLength, func(p string) bool { return isStrong(p) && unseen(p) })
	weak := drawPassword(t, weakAlphabet, plantedLength, func(p string) bool { return isWeak(p) && unseen(p) })

	captured := captureProcessOutput(t, func() {
		log.Printf("capture canary %s", canaries["standard logger"])
		fmt.Fprintf(os.Stdout, "capture canary %s\n", canaries["os.Stdout"])
		fmt.Fprintf(os.Stderr, "capture canary %s\n", canaries["os.Stderr"])
		runSeedPaths(t, strong, weak)
	})

	for writer, canary := range canaries {
		if !strings.Contains(captured, canary) {
			t.Fatalf("the canary written through %s is missing from the capture, so the capture proves nothing", writer)
		}
	}
	if !strings.Contains(captured, "Seeded administrator") {
		t.Fatal("the seed path's own log line is missing from the capture")
	}
	for name, password := range map[string]string{"accepted": strong, "refused": weak} {
		if offsets := leakedOffsets(captured, password); len(offsets) > 0 {
			t.Errorf("the log carries %d-character windows of the %s planted password at offsets %v", passwordLeakWindow, name, offsets)
		}
	}
}

// Tripwire, not the proof (the capture test above is): no logging or printing
// call in this package's non-test files passes an identifier whose name says
// it holds a password.
func TestNoLogCallPassesAPasswordNamedValue(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)
	names, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	isOutputCall := func(call *ast.CallExpr) bool {
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return false
		}
		switch pkg.Name {
		case "log":
			return true
		case "slog":
			return true
		case "fmt":
			return strings.HasPrefix(sel.Sel.Name, "Print") || strings.HasPrefix(sel.Sel.Name, "Fprint")
		}
		return false
	}

	fset := token.NewFileSet()
	parsed := 0
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", filepath.Base(name), err)
		}
		parsed++
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isOutputCall(call) {
				return true
			}
			for _, arg := range call.Args {
				ast.Inspect(arg, func(n ast.Node) bool {
					if id, ok := n.(*ast.Ident); ok && strings.Contains(strings.ToLower(id.Name), "password") {
						pos := fset.Position(id.Pos())
						t.Errorf("%s:%d passes %s to a logging call", filepath.Base(pos.Filename), pos.Line, id.Name)
					}
					return true
				})
			}
			return true
		})
	}
	if parsed == 0 {
		t.Fatal("no non-test source file was parsed")
	}
}
