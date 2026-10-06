package application

// Census of every path that writes an agent's registered key material: the
// agents table's public_key and encrypted_private_key columns, and the
// previous_public_key, pqc_public_key and previous_pqc_public_key columns
// stored beside them.
//
// AgentRepository.Update rewrites every key column on each call, so a
// function that assigns a key field on a domain.Agent and then reaches
// agentRepo.Update or agentRepo.Create writes the agent's key, whatever its
// name says it does. Each census row names that function, the routes that
// reach it with their route-level role middleware, who generates the new key,
// what proves possession of the new private key, and what the request records.
// A function outside the repository that rewrites a key column through its own
// SQL at server startup is listed separately, with the key columns it writes
// and the one cmd/server function that runs it.
//
// The tests derive the writer set from the source (go/parser over every
// non-test Go file in the backend module) and compare it with the census in
// both directions: a new writer turns them red, and so does a listed writer
// that no longer writes. The planted-control tests hand the same scanner one
// more writer in each shape it detects and assert that it is reported.
//
// Changing the census is a deliberate act: when a path gains a role gate, a
// proof of possession or a transition record, update its row in the same
// change and say why in the commit.
//
// Known limits, stated so a green run is not read as more than it is:
//   - a key written through a pointer taken elsewhere (p := &agent.PublicKey;
//     *p = k), through reflection, or by JSON-decoding into a domain.Agent is
//     not seen;
//   - SQL is matched one string literal at a time: a statement assembled at run
//     time from fragments that do not each name the agents table and a key
//     column is not seen;
//   - in the repository package, a function that reads rows (calls Scan) is
//     treated as a row read; its key-field assignments are not reported, and
//     its writes are found only through their SQL;
//   - SQL migration files and seed SQL are deploy-time writers and are out of
//     scope, as is any writer outside this module; a Go function in this
//     module that rewrites a key column at startup is in scope;
//   - the route check reads a registration of the form
//     router.Method("path", ..., h.Field.Method) and names it by its full
//     path, following router back through X.Group("prefix", ...) and
//     one-argument wrappers such as bindAgentRoutes(group) to the
//     application, and through the call sites of a cmd/server function that
//     takes a router (setupRoutes(v1, ...)). A route mounted with a path
//     constant, on a router built any other way, or with a handler passed
//     through a struct is not read, so the bootstrap token exchange route is
//     named in a comment on the registration row, not in its entries.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// agentKeyWriter is one census row.
type agentKeyWriter struct {
	label    string // what the path is for
	file     string // relative to the backend module root
	function string // Receiver.Method that assigns the key field
	table    string // the table the assigned struct is stored in; tableNone for a request or response struct
	// reaches is, for a tableNone row, the agents census row the function
	// hands the agent to, if any; the test checks that it calls it.
	reaches string
	// The remaining fields describe rows whose table is "agents".
	sinks     []string // agentRepo calls in the function; each rewrites every key column
	generates bool     // the deployment generates the new key pair (custody: server); otherwise the caller supplies it
	replaces  bool     // can overwrite a key the agent already has (read from the code, not computed)
	proof     string   // what proves possession of the new private key
	// transition is the transition package's Trigger constant the function
	// records the key change with when a transition recorder is set; "" when
	// it records none.
	transition string
	entries    []agentKeyWriterEntry
}

// agentKeyWriterEntry is one route that reaches a census row.
type agentKeyWriterEntry struct {
	method     string // Get, Post, Put, ...
	path       string // full path as served, without a trailing slash; the Go variable holding the group is not part of it
	middleware string // route-level middleware, comma-separated; "" when only the group's authentication applies
	handler    string // <field>.<method> as registered on the handlers struct
	handlerFn  string // Receiver.Method in the handlers package
	record     string // what the handler records: "audit log" or "none"
}

// proofNone is the proof column for a path that verifies no signature by the
// new private key before writing it.
const proofNone = "none"

// tableNone is the table column for a struct that is not stored: a request
// body or a response.
const tableNone = "none"

var agentKeyWriterCensus = []agentKeyWriter{
	{
		label:    "registration (creation, not a replacement)",
		file:     "internal/application/agent_service.go",
		function: "AgentService.CreateAgent",
		table:    "agents",
		sinks:    []string{"agentRepo.Create", "agentRepo.Update"},
		// Generates a pair when the request carries no public key; stores the
		// caller's key otherwise. POST /api/v1/onboarding/bootstrap-tokens/exchange
		// reaches it too, through BootstrapTokenService.Exchange (its own row),
		// with the bootstrap token as the only credential.
		generates: true,
		replaces:  false,
		proof:     proofNone,
		entries: []agentKeyWriterEntry{
			{method: "Post", path: "/api/v1/agents", middleware: "MemberOrAPIKeyMiddleware",
				handler: "Agent.CreateAgent", handlerFn: "AgentHandler.CreateAgent", record: "audit log"},
			{method: "Post", path: "/api/v1/public/agents/register", middleware: "",
				handler: "PublicAgent.Register", handlerFn: "PublicAgentHandler.Register", record: "none"},
		},
	},
	{
		label:    "credential rotation (the deployment generates the pair and returns the private half)",
		file:     "internal/application/agent_service.go",
		function: "AgentService.RotateCredentials",
		table:    "agents",
		// With a transition recorder set, the rotation writes the key through
		// repository.RotateAgentKeyTx with its transition record instead.
		sinks:      []string{"agentRepo.Update"},
		generates:  true,
		replaces:   true,
		proof:      proofNone,
		transition: "TriggerKeyRotated",
		entries: []agentKeyWriterEntry{
			{method: "Post", path: "/api/v1/agents/:id/rotate-credentials", middleware: "MemberMiddleware",
				handler: "Agent.RotateCredentials", handlerFn: "AgentHandler.RotateCredentials", record: "audit log"},
		},
	},
	{
		label:     "public key update (the caller supplies the key)",
		file:      "internal/application/agent_service.go",
		function:  "AgentService.UpdateAgentPublicKey",
		table:     "agents",
		sinks:     []string{"agentRepo.Update"},
		generates: false,
		// Refuses an agent-authenticated caller when the agent already has a key.
		replaces: true,
		proof:    proofNone,
		entries: []agentKeyWriterEntry{
			{method: "Put", path: "/api/v1/agents/:id/keys", middleware: "MemberMiddleware",
				handler: "Agent.UpdateAgentKeys", handlerFn: "AgentHandler.UpdateAgentKeys", record: "audit log"},
		},
	},
	{
		label:     "A2A request signing (generates a server-held pair when the agent has none)",
		file:      "internal/application/a2a_service.go",
		function:  "A2AService.SignA2ARequest",
		table:     "agents",
		sinks:     []string{"agentRepo.Update"},
		generates: true,
		replaces:  true,
		proof:     proofNone,
		entries: []agentKeyWriterEntry{
			{method: "Post", path: "/api/v1/a2a/agents/:id/sign", middleware: "",
				handler: "A2A.SignRequest", handlerFn: "A2AHandler.SignRequest", record: "none"},
			{method: "Post", path: "/api/v1/a2a/sign", middleware: "",
				handler: "A2A.SignRequestAlt", handlerFn: "A2AHandler.SignRequestAlt", record: "none"},
		},
	},
	{
		label:     "PQC key registration",
		file:      "internal/application/agent_service.go",
		function:  "AgentService.UpdateAgentPQCKey",
		table:     "agents",
		sinks:     []string{"agentRepo.Update"},
		generates: false,
		// The handler refuses with 409 when the agent already has a PQC key.
		replaces: false,
		proof:    proofNone,
		entries: []agentKeyWriterEntry{
			{method: "Post", path: "/api/v1/agents/:id/pqc-key", middleware: "MemberMiddleware",
				handler: "Agent.RegisterPQCKey", handlerFn: "AgentHandler.RegisterPQCKey", record: "audit log"},
		},
	},
	{
		label:     "PQC key rotation (the caller supplies the key)",
		file:      "internal/application/agent_service.go",
		function:  "AgentService.RotateAgentPQCKey",
		table:     "agents",
		sinks:     []string{"agentRepo.Update"},
		generates: false,
		replaces:  true,
		proof:     proofNone,
		entries: []agentKeyWriterEntry{
			{method: "Put", path: "/api/v1/agents/:id/pqc-key", middleware: "MemberMiddleware",
				handler: "Agent.RotatePQCKey", handlerFn: "AgentHandler.RotatePQCKey", record: "audit log"},
		},
	},
	{
		// Same field name, different table: listed so that it is not mistaken
		// for an agent key writer and so that a change to it is noticed.
		label:    "MCP server update",
		file:     "internal/application/mcp_service.go",
		function: "MCPService.UpdateMCPServer",
		table:    "mcp_servers",
	},
	{
		// Assigns PublicKey on its response struct. The agent and its key are
		// written by AgentService.CreateAgent, which it calls through
		// BootstrapAgentRegistrar.
		label:    "bootstrap token exchange",
		file:     "internal/application/bootstrap_token_service.go",
		function: "BootstrapTokenService.Exchange",
		table:    tableNone,
		reaches:  "AgentService.CreateAgent",
	},
	{
		// Trims the publicKey of the exchange request body.
		label:    "bootstrap token exchange request validation",
		file:     "internal/interfaces/http/handlers/bootstrap_token_handler.go",
		function: "validateBootstrapExchangeBody",
		table:    tableNone,
	},
}

// agentKeyColumnSinks are the functions whose SQL writes the agents table's
// key columns. Every census row with table "agents" reaches one of them.
var agentKeyColumnSinks = []string{
	"internal/infrastructure/repository/agent_repository.go:AgentRepository.Create",
	"internal/infrastructure/repository/agent_repository.go:AgentRepository.Update",
	// A credential rotation's key columns, written in the transaction of its
	// authorization transition record.
	"internal/infrastructure/repository/agent_transition_statements.go:RotateAgentKeyTx",
}

// agentKeyStartupWriter is a function outside the repository whose own SQL
// rewrites an agents key column at server startup, before any route is
// served. No route reaches it, so it has no entries, role or record.
type agentKeyStartupWriter struct {
	label    string   // what the rewrite is for
	file     string   // relative to the backend module root
	function string   // the function that holds the SQL
	columns  []string // the agents key columns its SQL writes, sorted
	caller   string   // file:function in cmd/server that runs it; its only caller in the module
}

var agentKeyStartupWriters = []agentKeyStartupWriter{
	{
		// Re-encrypts each stored private key in place, bound to its own row
		// ID, with a compare-and-swap on the ciphertext it read. The key pair
		// is unchanged: no key is generated, supplied or replaced, and the
		// public key column is not written.
		label:    "stored private key conversion to the format bound to the agent's ID",
		file:     "internal/application/agent_private_key_migration.go",
		function: "MigrateAgentPrivateKeysToV2",
		columns:  []string{"encrypted_private_key"},
		caller:   "cmd/server/main.go:main",
	},
}

// agentKeyFields are the domain.Agent fields stored in the agents table's key columns.
var agentKeyFields = map[string]bool{
	"PublicKey":            true,
	"EncryptedPrivateKey":  true,
	"PreviousPublicKey":    true,
	"PQCPublicKey":         true,
	"PreviousPQCPublicKey": true,
}

var (
	agentsTableWrite = regexp.MustCompile(`(?i)\b(insert\s+into|update)\s+(public\.)?agents\b`)
	agentsKeyColumn  = regexp.MustCompile(`(?i)\b(previous_)?(pqc_)?public_key\b|\bencrypted_private_key\b`)
	// A call that verifies a signature: ed25519.Verify, mldsa.Verify,
	// VerifySignature, VerifyEd25519Signature, ...
	signatureVerifyCall = regexp.MustCompile(`^Verify$|^Verify\w*Signature$`)
)

// The repository package assigns key fields when it reads rows; those
// assignments are reads, and its writes are found through their SQL.
const agentRepositoryDir = "internal/infrastructure/repository/"

type parsedGoFile struct {
	rel  string // slash-separated, relative to the backend module root
	file *ast.File
}

// keyWriteSite is what the scanner found in one function.
type keyWriteSite struct {
	pos       token.Position
	field     bool // assigns a key field, or builds a domain.Agent literal with one
	sql       bool // holds an INSERT INTO / UPDATE agents literal naming a key column
	generates bool
	sinks     map[string]bool
}

var (
	backendSourceOnce  sync.Once
	backendSourceFset  *token.FileSet
	backendSourceFiles []parsedGoFile
	backendSourceErr   error
)

// backendSource parses every non-test Go file in the backend module once.
func backendSource(t *testing.T) (*token.FileSet, []parsedGoFile) {
	t.Helper()
	backendSourceOnce.Do(func() {
		_, thisFile, _, ok := runtime.Caller(0)
		if !ok {
			backendSourceErr = fmt.Errorf("runtime.Caller failed")
			return
		}
		root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
		backendSourceFset = token.NewFileSet()
		backendSourceErr = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				name := d.Name()
				if path != root && (name == "testdata" || name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, err := parser.ParseFile(backendSourceFset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			backendSourceFiles = append(backendSourceFiles, parsedGoFile{rel: filepath.ToSlash(rel), file: f})
			return nil
		})
	})
	require.NoError(t, backendSourceErr, "parse the backend module")
	require.NotEmpty(t, backendSourceFiles, "the backend module has Go files")
	return backendSourceFset, backendSourceFiles
}

// declName returns Receiver.Method for a method and the bare name for a function.
func declName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	t := fn.Recv.List[0].Type
	for {
		switch x := t.(type) {
		case *ast.StarExpr:
			t = x.X
			continue
		case *ast.ParenExpr:
			t = x.X
			continue
		case *ast.IndexExpr:
			t = x.X
			continue
		case *ast.IndexListExpr:
			t = x.X
			continue
		}
		break
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name + "." + fn.Name.Name
	}
	return fn.Name.Name
}

func unwrapAssignTarget(e ast.Expr) ast.Expr {
	for {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		default:
			return e
		}
	}
}

func isDomainAgentType(t ast.Expr, pkg string) bool {
	switch x := t.(type) {
	case *ast.SelectorExpr:
		id, ok := x.X.(*ast.Ident)
		return ok && id.Name == "domain" && x.Sel.Name == "Agent"
	case *ast.Ident:
		return pkg == "domain" && x.Name == "Agent"
	}
	return false
}

func isAgentRepo(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.SelectorExpr:
		return x.Sel.Name == "agentRepo"
	case *ast.Ident:
		return x.Name == "agentRepo"
	}
	return false
}

// callName returns the called function's name: Sel for a selector, the
// identifier otherwise.
func callName(call *ast.CallExpr) string {
	switch f := call.Fun.(type) {
	case *ast.SelectorExpr:
		return f.Sel.Name
	case *ast.Ident:
		return f.Name
	}
	return ""
}

// scanAgentKeyWriters returns every function, keyed file:Receiver.Method, that
// assigns an agent key field or holds SQL writing an agents key column.
func scanAgentKeyWriters(fset *token.FileSet, files []parsedGoFile) map[string]*keyWriteSite {
	sites := map[string]*keyWriteSite{}
	for _, pf := range files {
		for _, decl := range pf.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			site := &keyWriteSite{pos: fset.Position(fn.Pos()), sinks: map[string]bool{}}
			readsRows := false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.AssignStmt:
					for _, lhs := range x.Lhs {
						if sel, ok := unwrapAssignTarget(lhs).(*ast.SelectorExpr); ok && agentKeyFields[sel.Sel.Name] {
							site.field = true
						}
					}
				case *ast.CompositeLit:
					if isDomainAgentType(x.Type, pf.file.Name.Name) {
						for _, elt := range x.Elts {
							if kv, ok := elt.(*ast.KeyValueExpr); ok {
								if id, ok := kv.Key.(*ast.Ident); ok && agentKeyFields[id.Name] {
									site.field = true
								}
							}
						}
					}
				case *ast.BasicLit:
					if x.Kind == token.STRING {
						if s, err := strconv.Unquote(x.Value); err == nil && agentsTableWrite.MatchString(s) && agentsKeyColumn.MatchString(s) {
							site.sql = true
						}
					}
				case *ast.CallExpr:
					switch name := callName(x); name {
					case "Update", "Create":
						if sel, ok := x.Fun.(*ast.SelectorExpr); ok && isAgentRepo(sel.X) {
							site.sinks["agentRepo."+name] = true
						}
					case "Scan":
						readsRows = true
					case "GenerateEd25519KeyPair":
						site.generates = true
					}
				}
				return true
			})
			if site.field && readsRows && strings.HasPrefix(pf.rel, agentRepositoryDir) {
				site.field = false
			}
			if site.field || site.sql {
				sites[pf.rel+":"+declName(fn)] = site
			}
		}
	}
	return sites
}

// diffAgentKeyWriterCensus compares the scanner's result with the census and
// the column sinks: unlisted are found but not listed, missing are listed but
// no longer found.
func diffAgentKeyWriterCensus(sites map[string]*keyWriteSite) (unlisted, missing []string) {
	listed := map[string]bool{}
	for _, row := range agentKeyWriterCensus {
		listed[row.file+":"+row.function] = true
	}
	for _, sink := range agentKeyColumnSinks {
		listed[sink] = true
	}
	for _, w := range agentKeyStartupWriters {
		listed[w.file+":"+w.function] = true
	}
	for key := range sites {
		if !listed[key] {
			unlisted = append(unlisted, key)
		}
	}
	for key := range listed {
		if sites[key] == nil {
			missing = append(missing, key)
		}
	}
	sort.Strings(unlisted)
	sort.Strings(missing)
	return unlisted, missing
}

// agentKeyWriterRow reports whether function names a census row whose table is agents.
func agentKeyWriterRow(function string) bool {
	for _, row := range agentKeyWriterCensus {
		if row.function == function && row.table == "agents" {
			return true
		}
	}
	return false
}

// agentsKeyColumnsWritten returns the agents key columns named by fn's SQL
// literals that write the agents table, lowercased and sorted.
func agentsKeyColumnsWritten(fn *ast.FuncDecl) []string {
	columns := map[string]bool{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if s, err := strconv.Unquote(lit.Value); err == nil && agentsTableWrite.MatchString(s) {
			for _, c := range agentsKeyColumn.FindAllString(s, -1) {
				columns[strings.ToLower(c)] = true
			}
		}
		return true
	})
	return sortedKeys(columns)
}

// callersOf returns every function, keyed file:Receiver.Method, whose body
// calls a function or method named name.
func callersOf(files []parsedGoFile, name string) []string {
	var callers []string
	for _, pf := range files {
		for _, decl := range pf.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if len(callsNamed(fn, func(n string) bool { return n == name })) > 0 {
				callers = append(callers, pf.rel+":"+declName(fn))
			}
		}
	}
	sort.Strings(callers)
	return callers
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k, v := range m {
		if v {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

func TestAgentKeyWriterCensus_WriterSetMatchesSource(t *testing.T) {
	fset, files := backendSource(t)
	sites := scanAgentKeyWriters(fset, files)

	unlisted, missing := diffAgentKeyWriterCensus(sites)
	assert.Empty(t, unlisted,
		"these functions write an agent key field or an agents key column and are not in the census. "+
			"AgentRepository.Update rewrites every key column, so each is a way to replace an agent's "+
			"registered key: add a row naming its routes, role, custody, proof and record (a startup "+
			"rewrite outside the repository goes in agentKeyStartupWriters), or remove the write")
	assert.Empty(t, missing,
		"these census rows no longer write what the census says; update or remove the row deliberately")

	for _, sink := range agentKeyColumnSinks {
		if site := sites[sink]; site != nil {
			assert.True(t, site.sql, "%s is listed as a key-column sink but holds no agents key-column SQL", sink)
		}
	}
	for _, row := range agentKeyWriterCensus {
		site := sites[row.file+":"+row.function]
		if site == nil {
			continue // reported as missing above
		}
		assert.True(t, site.field, "%s: listed as a key-field writer", row.function)
		if row.table != "agents" {
			assert.Empty(t, sortedKeys(site.sinks), "%s writes %s and must not reach agentRepo", row.function, row.table)
			if row.reaches != "" {
				assert.True(t, agentKeyWriterRow(row.reaches), "%s reaches %s, which is not an agents census row", row.function, row.reaches)
				fn := findFuncDecl(files, filepath.ToSlash(filepath.Dir(row.file))+"/", row.function)
				method := row.reaches[strings.LastIndex(row.reaches, ".")+1:]
				if assert.NotNil(t, fn, "%s not found", row.function) {
					assert.NotEmpty(t, callsNamed(fn, func(n string) bool { return n == method }),
						"%s must reach %s", row.function, row.reaches)
				}
			}
			continue
		}
		assert.Equal(t, row.sinks, sortedKeys(site.sinks), "%s: agentRepo calls", row.function)
		assert.Equal(t, row.generates, site.generates, "%s: custody (does the deployment generate the pair?)", row.function)
		assert.NotEmpty(t, row.entries, "%s: a key writer row names the routes that reach it", row.function)
	}
	for _, w := range agentKeyStartupWriters {
		site := sites[w.file+":"+w.function]
		if site == nil {
			continue // reported as missing above
		}
		assert.True(t, site.sql, "%s: listed as a startup rewrite of an agents key column", w.function)
		assert.False(t, site.field, "%s: a startup rewrite assigns no domain.Agent key field", w.function)
		assert.Empty(t, sortedKeys(site.sinks), "%s: a startup rewrite writes through its own SQL, not agentRepo", w.function)
		assert.False(t, site.generates, "%s: a startup rewrite generates no key pair", w.function)
		fn := findFuncDecl(files, filepath.ToSlash(filepath.Dir(w.file))+"/", w.function)
		if assert.NotNil(t, fn, "%s not found", w.function) {
			assert.Equal(t, w.columns, agentsKeyColumnsWritten(fn), "%s: the agents key columns its SQL writes", w.function)
		}
		assert.Equal(t, []string{w.caller}, callersOf(files, w.function),
			"%s runs at startup only: another caller (a handler, a job) is a path the census does not name", w.function)
	}
}

type routeRegistration struct {
	middleware string
	handler    string
	pos        token.Position
}

// handlerRef returns <field>.<method> for an expression of the form x.<field>.<method>.
func handlerRef(e ast.Expr) string {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	inner, ok := sel.X.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	return inner.Sel.Name + "." + sel.Sel.Name
}

var routeMethods = map[string]bool{"Get": true, "Post": true, "Put": true, "Patch": true, "Delete": true, "All": true}

// routeMount is where a router variable puts its routes: below path, on the
// application when param is -1, otherwise on the router that function fn
// receives as its param-th argument.
type routeMount struct {
	fn    string
	param int
	path  string
}

// joinRoutePath joins a group prefix and a path the way fiber does.
func joinRoutePath(prefix, path string) string {
	if path == "" {
		return prefix
	}
	if path[0] != '/' {
		path = "/" + path
	}
	return strings.TrimRight(prefix, "/") + path
}

func stringLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

// isFiberRouterType reports whether t is fiber.Router, *fiber.Group or *fiber.App.
func isFiberRouterType(t ast.Expr) bool {
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	sel, ok := t.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "fiber" && (sel.Sel.Name == "Router" || sel.Sel.Name == "Group" || sel.Sel.Name == "App")
}

// collectRoutes returns the route registrations of cmd/server keyed
// "method /full/path" (no trailing slash), and how often each handler
// reference appears there. A router variable is followed back to the
// application through X.Group("prefix", ...), through a one-argument
// cmd/server function that wraps the router it is given, and through the
// call sites of a cmd/server function that takes a router parameter, so a
// route keeps its key when the variable that holds its group is renamed.
func collectRoutes(fset *token.FileSet, files []parsedGoFile) (map[string][]routeRegistration, map[string]int) {
	type pendingRoute struct {
		mount  routeMount // path here is the route's own path below the mount's root
		method string
		reg    routeRegistration
	}
	var pending []pendingRoute
	refs := map[string]int{}
	// passed[fn][i] lists the routers that call sites hand fn as its i-th argument.
	passed := map[string]map[int][]routeMount{}

	var decls []*ast.FuncDecl
	funcs := map[string]bool{}
	for _, pf := range files {
		if !strings.HasPrefix(pf.rel, "cmd/server/") {
			continue
		}
		ast.Inspect(pf.file, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if ref := handlerRef(sel); ref != "" {
					refs[ref]++
				}
			}
			return true
		})
		for _, decl := range pf.file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil && fn.Recv == nil {
				decls = append(decls, fn)
				funcs[fn.Name.Name] = true
			}
		}
	}

	for _, fn := range decls {
		name := fn.Name.Name
		mounts := map[string]routeMount{}
		i := 0
		for _, field := range fn.Type.Params.List {
			if len(field.Names) == 0 {
				i++
				continue
			}
			for _, p := range field.Names {
				if isFiberRouterType(field.Type) {
					mounts[p.Name] = routeMount{fn: name, param: i}
				}
				i++
			}
		}
		mountOf := func(e ast.Expr) (routeMount, bool) {
			id, ok := e.(*ast.Ident)
			if !ok {
				return routeMount{}, false
			}
			m, ok := mounts[id.Name]
			return m, ok
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				if len(n.Lhs) != 1 || len(n.Rhs) != 1 {
					return true
				}
				lhs, ok := n.Lhs[0].(*ast.Ident)
				call, isCall := n.Rhs[0].(*ast.CallExpr)
				if !ok || !isCall {
					return true
				}
				switch fun := call.Fun.(type) {
				case *ast.SelectorExpr:
					if pkg, ok := fun.X.(*ast.Ident); ok && pkg.Name == "fiber" && fun.Sel.Name == "New" {
						mounts[lhs.Name] = routeMount{fn: name, param: -1}
					} else if parent, ok := mountOf(fun.X); ok && fun.Sel.Name == "Group" && len(call.Args) > 0 {
						if prefix, ok := stringLit(call.Args[0]); ok {
							mounts[lhs.Name] = routeMount{fn: name, param: parent.param, path: joinRoutePath(parent.path, prefix)}
						}
					}
				case *ast.Ident:
					if len(call.Args) == 1 && funcs[fun.Name] {
						if parent, ok := mountOf(call.Args[0]); ok {
							mounts[lhs.Name] = parent
						}
					}
				}
			case *ast.CallExpr:
				if callee, ok := n.Fun.(*ast.Ident); ok && funcs[callee.Name] {
					for i, arg := range n.Args {
						if m, ok := mountOf(arg); ok {
							if passed[callee.Name] == nil {
								passed[callee.Name] = map[int][]routeMount{}
							}
							passed[callee.Name][i] = append(passed[callee.Name][i], m)
						}
					}
				}
				fun, ok := n.Fun.(*ast.SelectorExpr)
				if !ok || !routeMethods[fun.Sel.Name] || len(n.Args) < 2 {
					return true
				}
				m, ok := mountOf(fun.X)
				if !ok {
					return true
				}
				path, ok := stringLit(n.Args[0])
				if !ok {
					return true
				}
				var mws []string
				for _, arg := range n.Args[1 : len(n.Args)-1] {
					if mc, ok := arg.(*ast.CallExpr); ok {
						mws = append(mws, callName(mc))
					}
				}
				m.path = joinRoutePath(m.path, path)
				pending = append(pending, pendingRoute{mount: m, method: fun.Sel.Name, reg: routeRegistration{
					middleware: strings.Join(mws, ","),
					handler:    handlerRef(n.Args[len(n.Args)-1]),
					pos:        fset.Position(n.Pos()),
				}})
			}
			return true
		})
	}

	// prefixes returns every path the root of m is mounted at.
	var prefixes func(m routeMount, depth int) []string
	prefixes = func(m routeMount, depth int) []string {
		if m.param < 0 {
			return []string{""}
		}
		if depth > 8 {
			return nil
		}
		var out []string
		for _, caller := range passed[m.fn][m.param] {
			for _, p := range prefixes(caller, depth+1) {
				out = append(out, joinRoutePath(p, caller.path))
			}
		}
		return out
	}

	routes := map[string][]routeRegistration{}
	for _, r := range pending {
		for _, prefix := range prefixes(r.mount, 0) {
			full := strings.TrimRight(joinRoutePath(prefix, r.mount.path), "/")
			if full == "" {
				full = "/"
			}
			key := r.method + " " + full
			routes[key] = append(routes[key], r.reg)
		}
	}
	return routes, refs
}

func findFuncDecl(files []parsedGoFile, dirPrefix, name string) *ast.FuncDecl {
	for _, pf := range files {
		if !strings.HasPrefix(pf.rel, dirPrefix) {
			continue
		}
		for _, decl := range pf.file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil && declName(fn) == name {
				return fn
			}
		}
	}
	return nil
}

func callsNamed(fn *ast.FuncDecl, match func(string) bool) []string {
	var found []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if name := callName(call); match(name) {
				found = append(found, name)
			}
		}
		return true
	})
	return found
}

const handlersDir = "internal/interfaces/http/handlers/"

func TestAgentKeyWriterCensus_RoutesRolesAndRecordsMatchSource(t *testing.T) {
	fset, files := backendSource(t)
	routes, refs := collectRoutes(fset, files)

	listedPerHandler := map[string]int{}
	for _, row := range agentKeyWriterCensus {
		serviceMethod := row.function[strings.LastIndex(row.function, ".")+1:]
		for _, e := range row.entries {
			listedPerHandler[e.handler]++
			key := e.method + " " + e.path
			regs := routes[key]
			if !assert.Len(t, regs, 1, "%s: exactly one registration of %q in cmd/server", row.function, key) {
				continue
			}
			assert.Equal(t, e.handler, regs[0].handler, "%q (%s): handler", key, regs[0].pos)
			assert.Equal(t, e.middleware, regs[0].middleware, "%q (%s): route-level role middleware", key, regs[0].pos)

			hfn := findFuncDecl(files, handlersDir, e.handlerFn)
			if !assert.NotNil(t, hfn, "handler %s not found", e.handlerFn) {
				continue
			}
			assert.NotEmpty(t, callsNamed(hfn, func(n string) bool { return n == serviceMethod }),
				"%s must reach %s", e.handlerFn, row.function)
			record := "none"
			if len(callsNamed(hfn, func(n string) bool { return n == "LogAction" })) > 0 {
				record = "audit log"
			}
			assert.Equal(t, e.record, record, "%s: what the request records", e.handlerFn)
		}
	}

	// A listed handler mounted anywhere else in cmd/server (a second route,
	// a route table entry) is a path the census does not name.
	for handler, listed := range listedPerHandler {
		assert.Equal(t, listed, refs[handler],
			"%s is referenced %d times in cmd/server but the census lists %d routes for it",
			handler, refs[handler], listed)
	}
}

// A route is named by the path it is served at: renaming the variable that
// holds its group, wrapping the group in a one-argument helper, or mounting
// it through a function that takes a router leaves its key unchanged.
func TestAgentKeyWriterCensus_RouteKeyIsTheServedPath(t *testing.T) {
	const rel = "cmd/server/planted_routes.go"
	const src = `package main

func main() {
	app := fiber.New(fiber.Config{})
	v1 := app.Group("/api/v1")
	mountPlanted(v1, nil)
}

func mountPlanted(v1 fiber.Router, h *Handlers) {
	a2aRenamed := v1.Group("/a2a")
	a2aWrapped := wrapPlanted(a2aRenamed)
	a2aWrapped.Post("/agents/:id/sign", middleware.MemberMiddleware(), h.Planted.Sign)
	a2aRenamed.Post("/", h.Planted.Root)
}

func wrapPlanted(group fiber.Router) plantedRouter {
	return plantedRouter{group: group}
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, parser.SkipObjectResolution)
	require.NoError(t, err)
	routes, refs := collectRoutes(fset, []parsedGoFile{{rel: rel, file: f}})

	keys := map[string]bool{}
	for k := range routes {
		keys[k] = true
	}
	sign := routes["Post /api/v1/a2a/agents/:id/sign"]
	if assert.Len(t, sign, 1, "registered routes: %v", sortedKeys(keys)) {
		assert.Equal(t, "Planted.Sign", sign[0].handler)
		assert.Equal(t, "MemberMiddleware", sign[0].middleware)
	}
	assert.Len(t, routes["Post /api/v1/a2a"], 1, "registered routes: %v", sortedKeys(keys))
	assert.Len(t, routes, 2, "registered routes: %v", sortedKeys(keys))
	assert.Equal(t, 1, refs["Planted.Sign"])
}

func TestAgentKeyWriterCensus_ProofColumnMatchesSource(t *testing.T) {
	_, files := backendSource(t)
	isVerify := func(n string) bool { return signatureVerifyCall.MatchString(n) }

	for _, row := range agentKeyWriterCensus {
		if row.table != "agents" || row.proof != proofNone {
			continue
		}
		fn := findFuncDecl(files, filepath.ToSlash(filepath.Dir(row.file))+"/", row.function)
		if !assert.NotNil(t, fn, "%s not found", row.function) {
			continue
		}
		assert.Empty(t, callsNamed(fn, isVerify),
			"%s now verifies a signature: if it proves possession of the new key, name the proof in its census row", row.function)
		for _, e := range row.entries {
			hfn := findFuncDecl(files, handlersDir, e.handlerFn)
			if !assert.NotNil(t, hfn, "handler %s not found", e.handlerFn) {
				continue
			}
			assert.Empty(t, callsNamed(hfn, isVerify),
				"%s now verifies a signature: if it proves possession of the new key, name the proof in the census row for %s",
				e.handlerFn, row.function)
		}
	}
}

// agentKeyTransitionTriggers are the transition package's triggers that
// record a change of an agent's key, by constant name and value.
var agentKeyTransitionTriggers = map[string]string{
	"TriggerKeyRotated": "key_rotated",
	"TriggerKeyUpdated": "key_updated",
}

// agentKeyTransitionVocabulary is the one file that spells the key triggers'
// values.
const agentKeyTransitionVocabulary = "internal/record/transition/transition.go"

// Each agents census row records its key change as an authorization
// transition with the trigger its transition column names, and a row whose
// column is empty records none. The key triggers' values are spelled only in
// the transition vocabulary, so a path cannot record a key change under a
// value the census does not know.
func TestAgentKeyWriterCensus_TransitionColumnMatchesSource(t *testing.T) {
	fset, files := backendSource(t)
	for _, pf := range files {
		ast.Inspect(pf.file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if s, err := strconv.Unquote(lit.Value); err == nil {
				for _, value := range agentKeyTransitionTriggers {
					if strings.Contains(s, value) {
						assert.Equal(t, agentKeyTransitionVocabulary, pf.rel,
							"%s spells the key trigger %q outside the transition vocabulary", fset.Position(lit.Pos()), value)
					}
				}
			}
			return true
		})
	}
	for _, row := range agentKeyWriterCensus {
		if row.table != "agents" {
			continue
		}
		fn := findFuncDecl(files, filepath.ToSlash(filepath.Dir(row.file))+"/", row.function)
		if !assert.NotNil(t, fn, "%s not found", row.function) {
			continue
		}
		recorded := map[string]bool{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "transition" {
				if _, key := agentKeyTransitionTriggers[sel.Sel.Name]; key {
					recorded[sel.Sel.Name] = true
				}
			}
			return true
		})
		want := []string{}
		if row.transition != "" {
			want = []string{row.transition}
		}
		assert.Equal(t, want, sortedKeys(recorded),
			"%s: the key trigger it records as an authorization transition; update its census row deliberately", row.function)
	}
}

func TestAgentKeyWriterCensus_PlantedWriterIsReported(t *testing.T) {
	cases := []struct {
		name     string
		rel      string
		function string
		src      string
	}{
		{
			name:     "a sixth agentRepo.Update caller assigning PublicKey",
			rel:      "internal/application/planted_key_writer.go",
			function: "AgentService.PlantedKeyWriter",
			src: `package application

func (s *AgentService) PlantedKeyWriter(agent *domain.Agent, key string) error {
	agent.PublicKey = &key
	return s.agentRepo.Update(agent)
}
`,
		},
		{
			name:     "a write through the field's pointer",
			rel:      "internal/application/planted_key_writer.go",
			function: "AgentService.PlantedKeyWriter",
			src: `package application

func (s *AgentService) PlantedKeyWriter(agent *domain.Agent, key string) error {
	*agent.EncryptedPrivateKey = key
	return s.agentRepo.Update(agent)
}
`,
		},
		{
			name:     "a domain.Agent literal carrying a key",
			rel:      "internal/application/planted_key_writer.go",
			function: "plantedAgent",
			src: `package application

func plantedAgent(key string) *domain.Agent {
	return &domain.Agent{PQCPublicKey: &key}
}
`,
		},
		{
			name:     "SQL writing an agents key column",
			rel:      "internal/infrastructure/repository/planted_key_writer.go",
			function: "AgentRepository.PlantedKeyWriter",
			src: "package repository\n\n" +
				"func (r *AgentRepository) PlantedKeyWriter(id, key string) error {\n" +
				"\t_, err := r.db.Exec(`UPDATE agents\n\t\tSET public_key = $1 WHERE id = $2`, key, id)\n" +
				"\treturn err\n}\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fset, files := backendSource(t)
			planted, err := parser.ParseFile(fset, tc.rel, tc.src, parser.SkipObjectResolution)
			require.NoError(t, err)
			withPlanted := append(append([]parsedGoFile{}, files...), parsedGoFile{rel: tc.rel, file: planted})

			unlisted, missing := diffAgentKeyWriterCensus(scanAgentKeyWriters(fset, withPlanted))
			assert.Equal(t, []string{tc.rel + ":" + tc.function}, unlisted, "the planted writer is reported")
			assert.Empty(t, missing)
		})
	}
}

// A listed writer that stops writing is reported too, so a row cannot outlive
// the path it describes.
func TestAgentKeyWriterCensus_RemovedWriterIsReported(t *testing.T) {
	fset, files := backendSource(t)
	const removed = "internal/application/mcp_service.go"
	var without []parsedGoFile
	for _, pf := range files {
		if pf.rel != removed {
			without = append(without, pf)
		}
	}
	require.Len(t, without, len(files)-1, "%s is part of the parsed source", removed)

	unlisted, missing := diffAgentKeyWriterCensus(scanAgentKeyWriters(fset, without))
	assert.Empty(t, unlisted)
	assert.Equal(t, []string{removed + ":MCPService.UpdateMCPServer"}, missing)
}

// The census is of key writers, not of every agentRepo.Update caller: a
// caller that leaves the key fields as read, and SQL updating other agents
// columns, are not reported.
func TestAgentKeyWriterCensus_NonKeyWriteIsNotReported(t *testing.T) {
	fset, files := backendSource(t)
	src := "package application\n\n" +
		"func (s *AgentService) PlantedTrustUpdate(agent *domain.Agent, score float64) error {\n" +
		"\tagent.TrustScore = score\n" +
		"\t_ = `UPDATE agents SET trust_score = $1 WHERE key_expires_at < NOW()`\n" +
		"\treturn s.agentRepo.Update(agent)\n}\n"
	planted, err := parser.ParseFile(fset, "internal/application/planted_trust_update.go", src, parser.SkipObjectResolution)
	require.NoError(t, err)
	withPlanted := append(append([]parsedGoFile{}, files...),
		parsedGoFile{rel: "internal/application/planted_trust_update.go", file: planted})

	unlisted, missing := diffAgentKeyWriterCensus(scanAgentKeyWriters(fset, withPlanted))
	assert.Empty(t, unlisted)
	assert.Empty(t, missing)
}

// A startup rewrite that gains a second caller, such as a handler, is
// reported, so it cannot become a request path without a census row.
func TestAgentKeyWriterCensus_StartupWriterSecondCallerIsReported(t *testing.T) {
	_, files := backendSource(t)
	require.NotEmpty(t, agentKeyStartupWriters)
	w := agentKeyStartupWriters[0]
	require.Equal(t, []string{w.caller}, callersOf(files, w.function), "%s: its caller in the parsed source", w.function)

	const rel = handlersDir + "planted_migration_handler.go"
	src := "package handlers\n\n" +
		"func (h *AgentHandler) PlantedMigrate(c fiber.Ctx) error {\n" +
		"\t_, err := application." + w.function + "(c.Context(), h.db, h.keyVault)\n" +
		"\treturn err\n}\n"
	fset := token.NewFileSet()
	planted, err := parser.ParseFile(fset, rel, src, parser.SkipObjectResolution)
	require.NoError(t, err)
	withPlanted := append(append([]parsedGoFile{}, files...), parsedGoFile{rel: rel, file: planted})

	assert.Equal(t, []string{w.caller, rel + ":AgentHandler.PlantedMigrate"}, callersOf(withPlanted, w.function))
}
