package main

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

// recordingDrainer stands in for the FGA engine and records the drain.
type recordingDrainer struct {
	mu          sync.Mutex
	calls       int
	hadDeadline bool
	order       *[]string
}

func (d *recordingDrainer) Shutdown(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls++
	_, d.hadDeadline = ctx.Deadline()
	if d.order != nil {
		*d.order = append(*d.order, "fga")
	}
	return nil
}

// failingAPI stands in for the API server and fails its shutdown.
type failingAPI struct {
	order *[]string
}

func (a failingAPI) ShutdownWithTimeout(time.Duration) error {
	*a.order = append(*a.order, "api")
	return errors.New("listener close failed")
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return &buf
}

// A request that never finishes must not hold shutdown open: the API shutdown
// gives up at its timeout, the timeout is logged, and the FGA drain still runs.
func TestShutdownBoundsAStalledRequestAndStillDrainsFGA(t *testing.T) {
	logs := captureLog(t)

	app := fiber.New()
	started := make(chan struct{})
	release := make(chan struct{})
	app.Get("/stall", func(c fiber.Ctx) error {
		close(started)
		<-release
		return c.SendString("ok")
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	served := make(chan struct{})
	go func() {
		defer close(served)
		_ = app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true})
	}()
	client := make(chan struct{})
	go func() {
		defer close(client)
		resp, err := http.Get("http://" + ln.Addr().String() + "/stall") //nolint:noctx // test client
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the stalled request never reached the handler")
	}
	t.Cleanup(func() {
		close(release)
		<-client
		<-served
	})

	drainer := &recordingDrainer{}
	const apiTimeout = 300 * time.Millisecond
	done := make(chan struct{})
	start := time.Now()
	go func() {
		defer close(done)
		shutdownGracefully(app, apiTimeout, drainer, time.Second)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("shutdown still blocked after 5s with one stalled request; the API timeout was %s", apiTimeout)
	}
	if elapsed := time.Since(start); elapsed < apiTimeout {
		t.Errorf("shutdown returned after %s, before the %s API timeout, so the stalled request was not waited on", elapsed, apiTimeout)
	}
	if !strings.Contains(logs.String(), context.DeadlineExceeded.Error()) {
		t.Errorf("the timed-out API shutdown was not logged; log output:\n%s", logs.String())
	}
	if drainer.calls != 1 {
		t.Errorf("FGA drain ran %d times after a timed-out API shutdown, want 1", drainer.calls)
	}
	if !drainer.hadDeadline {
		t.Error("FGA drain ran without a deadline")
	}
}

// A failed API shutdown is logged and the FGA drain runs after it. Exiting the
// process here instead (log.Fatal) would end this test binary.
func TestShutdownDrainsFGAAfterAFailedAPIShutdown(t *testing.T) {
	logs := captureLog(t)

	var order []string
	drainer := &recordingDrainer{order: &order}
	shutdownGracefully(failingAPI{order: &order}, time.Second, drainer, time.Second)

	if got := strings.Join(order, ","); got != "api,fga" {
		t.Errorf("shutdown order = %q, want %q (stop the listener, then drain FGA)", got, "api,fga")
	}
	if !strings.Contains(logs.String(), "listener close failed") {
		t.Errorf("the API shutdown error was not logged; log output:\n%s", logs.String())
	}

	// Without an FGA engine the API shutdown still completes.
	order = nil
	shutdownGracefully(failingAPI{order: &order}, time.Second, nil, time.Second)
	if got := strings.Join(order, ","); got != "api" {
		t.Errorf("shutdown order without FGA = %q, want %q", got, "api")
	}
}

// Source guard over main.go and shutdown.go (parsed, so comments do not count):
// after the shutdown signal, nothing calls log.Fatal*, log.Panic* or os.Exit,
// main hands off to shutdownGracefully, and nothing calls the untimed
// Shutdown() of the API server.
func TestShutdownPathNeverExitsTheProcess(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(thisFile)
	fset := token.NewFileSet()
	parse := func(name string) *ast.File {
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		return f
	}
	mainFile, shutdownFile := parse("main.go"), parse("shutdown.go")

	findFunc := func(f *ast.File, name string) *ast.FuncDecl {
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name {
				return fn
			}
		}
		t.Fatalf("func %s not found", name)
		return nil
	}
	isExit := func(call *ast.CallExpr) (string, bool) {
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return "", false
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return "", false
		}
		name := pkg.Name + "." + sel.Sel.Name
		switch {
		case pkg.Name == "log" && (strings.HasPrefix(sel.Sel.Name, "Fatal") || strings.HasPrefix(sel.Sel.Name, "Panic")):
			return name, true
		case name == "os.Exit":
			return name, true
		}
		return "", false
	}
	exitsIn := func(n ast.Node) []string {
		var found []string
		ast.Inspect(n, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if name, ok := isExit(call); ok {
					found = append(found, fset.Position(call.Pos()).String()+" "+name)
				}
			}
			return true
		})
		return found
	}

	// The shutdown sequence in main starts at the statement that receives the signal.
	mainFn := findFunc(mainFile, "main")
	signalAt := -1
	for i, stmt := range mainFn.Body.List {
		if es, ok := stmt.(*ast.ExprStmt); ok {
			if u, ok := es.X.(*ast.UnaryExpr); ok && u.Op == token.ARROW {
				signalAt = i
			}
		}
	}
	if signalAt < 0 {
		t.Fatal("main no longer waits on a shutdown signal; update this guard")
	}
	handsOff := false
	for _, stmt := range mainFn.Body.List[signalAt+1:] {
		for _, exit := range exitsIn(stmt) {
			t.Errorf("%s runs after the shutdown signal; it would skip the FGA drain and the deferred cleanup", exit)
		}
		ast.Inspect(stmt, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "shutdownGracefully" {
					handsOff = true
				}
			}
			return true
		})
	}
	if !handsOff {
		t.Error("main does not call shutdownGracefully after the shutdown signal")
	}

	for _, exit := range exitsIn(findFunc(shutdownFile, "shutdownGracefully")) {
		t.Errorf("%s in shutdownGracefully would skip the FGA drain and the deferred cleanup", exit)
	}

	// Shutdown() with no arguments is the untimed form on *fiber.App.
	ast.Inspect(mainFile, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && len(call.Args) == 0 {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Shutdown" {
				t.Errorf("%s calls the untimed Shutdown(); use shutdownGracefully", fset.Position(call.Pos()))
			}
		}
		return true
	})
}
