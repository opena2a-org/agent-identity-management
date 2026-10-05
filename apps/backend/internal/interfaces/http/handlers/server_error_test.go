package handlers

// A 5xx means the server failed, so the error behind it describes the
// server: a repository error names tables, constraints and query fragments,
// and a wrapped error carries every layer's prefix. None of that is for the
// client. Each 5xx answer is the fixed body respondServerError writes, and
// the error goes to the server log:
//
//	if err := h.tagService.DeleteTag(ctx, id); err != nil {
//		return respondServerError(c, fiber.StatusInternalServerError, err)
//	}
//
// A route that states its own outcome on a 5xx, such as a failed agent delete,
// answers a fixed sentence of its own instead; the census holds it to the same
// rule.
//
// The census reads every non-test file in the package and fails on a
// response that can be sent at a 5xx status (c.Status(s).JSON(...),
// .SendString, fiber.NewError(s, ...)) whose body mentions an error value: a
// call to .Error(), or an identifier named err, fooErr or errFoo. A status
// can be 5xx when it is a 5xx constant, a local variable the enclosing
// function sets to one, or a call to a function of this package that returns
// one.

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var serverErrorStatusNames = map[string]bool{
	"StatusInternalServerError":           true,
	"StatusNotImplemented":                true,
	"StatusBadGateway":                    true,
	"StatusServiceUnavailable":            true,
	"StatusGatewayTimeout":                true,
	"StatusHTTPVersionNotSupported":       true,
	"StatusVariantAlsoNegotiates":         true,
	"StatusInsufficientStorage":           true,
	"StatusLoopDetected":                  true,
	"StatusNotExtended":                   true,
	"StatusNetworkAuthenticationRequired": true,
}

// repositoryFailure is the text of a real foreign-key violation, the kind of
// error a 5xx used to hand to the browser.
var repositoryFailure = errors.New(`failed to delete agent: pq: update or delete on table "agents" violates foreign key constraint "fk_verification_events_agent" on table "verification_events"`)

func decodeErrorBody(t *testing.T, app *fiber.App, method, path string) (int, map[string]any) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(method, path, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return resp.StatusCode, body
}

func TestRespondServerErrorWritesTheFixedLine(t *testing.T) {
	app := fiber.New()
	app.Get("/probe", func(c fiber.Ctx) error {
		return respondServerError(c, fiber.StatusServiceUnavailable, repositoryFailure)
	})

	status, body := decodeErrorBody(t, app, "GET", "/probe")
	assert.Equal(t, fiber.StatusServiceUnavailable, status)
	assert.Equal(t, map[string]any{"error": ServerErrorMessage}, body)
}

func TestDeleteAgentRepositoryFailureAnswersItsStatedLine(t *testing.T) {
	orgID, userID, agentID := uuid.New(), uuid.New(), uuid.New()
	handler := NewAgentHandlerWithInterfaces(
		&MockAgentServiceImpl{
			GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
				return &domain.Agent{ID: agentID, OrganizationID: orgID}, nil
			},
			DeleteAgentFunc: func(ctx context.Context, id uuid.UUID) error {
				return repositoryFailure
			},
		},
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)
	app := createTestAppWithAuth(handler, orgID, userID)
	app.Delete("/api/v1/agents/:id", handler.DeleteAgent)

	status, body := decodeErrorBody(t, app, "DELETE", "/api/v1/agents/"+agentID.String())
	assert.Equal(t, fiber.StatusInternalServerError, status)
	assert.Equal(t, map[string]any{"error": agentDeleteFailedMessage}, body)
}

func TestServerErrorResponsesCarryNoErrorText(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	require.NoError(t, err)

	fset := token.NewFileSet()
	var files []*ast.File
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err)
		files = append(files, file)
	}
	returns5xx := functionsReturningServerErrorStatus(files)

	total, leaking := 0, 0
	for _, file := range files {
		var stack []ast.Node
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			status, body, ok := responseStatusAndBody(call)
			if !ok || !canBeServerErrorStatus(status, enclosingFuncBody(stack), returns5xx) {
				return true
			}
			total++
			for _, arg := range body {
				if mentionsErrorValue(arg) {
					leaking++
					t.Errorf("%s: a 5xx response body is built from an error value; return respondServerError(c, status, err) instead",
						fset.Position(call.Pos()))
					break
				}
			}
			return true
		})
	}

	require.NotZero(t, total, "no 5xx responses found; the census is not reading the handler files")
	if leaking > 0 {
		t.Logf("%d of %d 5xx responses carry error text", leaking, total)
	} else {
		t.Logf("%d 5xx responses, none built from an error value", total)
	}
}

// responseStatusAndBody matches X.Status(s).Method(...) and
// fiber.NewError(s, ...) and returns the status and the expressions that
// form the body.
func responseStatusAndBody(call *ast.CallExpr) (ast.Expr, []ast.Expr, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, nil, false
	}
	if sel.Sel.Name == "NewError" && isIdent(sel.X, "fiber") {
		if len(call.Args) > 1 {
			return call.Args[0], call.Args[1:], true
		}
		return nil, nil, false
	}
	inner, ok := sel.X.(*ast.CallExpr)
	if !ok || len(inner.Args) != 1 {
		return nil, nil, false
	}
	innerSel, ok := inner.Fun.(*ast.SelectorExpr)
	if !ok || innerSel.Sel.Name != "Status" {
		return nil, nil, false
	}
	return inner.Args[0], call.Args, true
}

func canBeServerErrorStatus(status ast.Expr, fnBody *ast.BlockStmt, returns5xx map[string]bool) bool {
	switch s := status.(type) {
	case *ast.Ident:
		return fnBody != nil && assignsServerErrorStatus(fnBody, s.Name)
	case *ast.CallExpr:
		fn, ok := s.Fun.(*ast.Ident)
		return ok && returns5xx[fn.Name]
	}
	return isServerErrorStatus(status)
}

func isServerErrorStatus(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.SelectorExpr:
		return serverErrorStatusNames[e.Sel.Name]
	case *ast.BasicLit:
		return e.Kind == token.INT && len(e.Value) == 3 && e.Value[0] == '5'
	}
	return false
}

// assignsServerErrorStatus reports whether body sets the variable name to a
// 5xx constant (name := 500, name = fiber.StatusInternalServerError, var name = ...).
func assignsServerErrorStatus(body *ast.BlockStmt, name string) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			if len(s.Lhs) == len(s.Rhs) {
				for i, lhs := range s.Lhs {
					if isIdent(lhs, name) && isServerErrorStatus(s.Rhs[i]) {
						found = true
					}
				}
			}
		case *ast.ValueSpec:
			if len(s.Names) == len(s.Values) {
				for i, ident := range s.Names {
					if ident.Name == name && isServerErrorStatus(s.Values[i]) {
						found = true
					}
				}
			}
		}
		return !found
	})
	return found
}

// functionsReturningServerErrorStatus names the package-level functions with
// a return statement whose first result is a 5xx constant.
func functionsReturningServerErrorStatus(files []*ast.File) map[string]bool {
	names := map[string]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if _, nested := n.(*ast.FuncLit); nested {
					return false
				}
				if ret, ok := n.(*ast.ReturnStmt); ok && len(ret.Results) > 0 && isServerErrorStatus(ret.Results[0]) {
					names[fn.Name.Name] = true
				}
				return true
			})
		}
	}
	return names
}

func enclosingFuncBody(stack []ast.Node) *ast.BlockStmt {
	for i := len(stack) - 1; i >= 0; i-- {
		switch fn := stack[i].(type) {
		case *ast.FuncLit:
			return fn.Body
		case *ast.FuncDecl:
			return fn.Body
		}
	}
	return nil
}

// mentionsErrorValue reports whether expr calls .Error() or names a variable
// that by this package's naming holds an error. Composite-literal keys and
// selector names (ErrorResponse{Error: ...}, x.Err) are names of fields, not
// values, and are skipped.
func mentionsErrorValue(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if found {
			return false
		}
		switch e := n.(type) {
		case *ast.KeyValueExpr:
			if _, isName := e.Key.(*ast.Ident); isName {
				found = mentionsErrorValue(e.Value)
				return false
			}
		case *ast.CallExpr:
			if sel, ok := e.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Error" && len(e.Args) == 0 {
				found = true
				return false
			}
		case *ast.SelectorExpr:
			found = mentionsErrorValue(e.X)
			return false
		case *ast.Ident:
			found = isErrorVariableName(e.Name)
		}
		return true
	})
	return found
}

func isErrorVariableName(name string) bool {
	if name == "err" || strings.HasSuffix(name, "Err") {
		return true
	}
	rest := strings.TrimPrefix(name, "err")
	return rest != name && rest != "" && unicode.IsUpper(rune(rest[0]))
}
