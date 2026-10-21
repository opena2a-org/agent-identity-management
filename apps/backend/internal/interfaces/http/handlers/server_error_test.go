package handlers

// A response body carries stated lines, never an error's text. A 5xx means
// the server failed, so the error behind it describes the server: a
// repository error names tables, constraints and query fragments, and a
// wrapped error carries every layer's prefix. Each 5xx answer is the fixed
// body respondServerError writes, and the error goes to the server log:
//
//	if err := h.tagService.DeleteTag(ctx, id); err != nil {
//		return respondServerError(c, fiber.StatusInternalServerError, err)
//	}
//
// A route that states its own outcome on a 5xx, such as a failed agent delete,
// answers a fixed sentence of its own instead; the census holds it to the same
// rule.
//
// A refusal the caller can act on is matched (errors.Is, errors.As) and
// answered with a line written for the caller that names the member and the
// rule it breaks; the error's own text is not the answer.
//
// The census reads every non-test file in the package and fails on a
// response at any status (c.JSON, c.Status(s).JSON, .Send, .SendString,
// fiber.NewError) or a fiber.Map or ErrorResponse literal whose body mentions
// an error value: a call to .Error(), an identifier named err, fooErr or
// errFoo, or a local variable that was assigned an error value or its text
// under another name (errorValueVariables), such as detail := err.Error().

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
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

func TestRespondServerErrorLogsWhatTheBodyLeavesOut(t *testing.T) {
	var logged bytes.Buffer
	defer log.SetOutput(log.Writer())
	log.SetOutput(&logged)

	planted := "planted-" + uuid.NewString()
	app := fiber.New()
	app.Get("/probe", func(c fiber.Ctx) error {
		return respondServerError(c, fiber.StatusInternalServerError, fmt.Errorf("pq: relation %q does not exist", planted))
	})
	resp, err := app.Test(httptest.NewRequest("GET", "/probe", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
	assert.NotContains(t, string(body), planted)
	assert.Contains(t, logged.String(), planted)
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

func TestResponsesCarryNoErrorText(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	require.NoError(t, err)

	fset := token.NewFileSet()
	total, leaking := 0, 0
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err)
		responses, found := responsesBuiltFromErrorValues(file)
		total += responses
		for _, pos := range found {
			leaking++
			t.Errorf("%s: a response body is built from an error value; answer a stated line (respondServerError for a 5xx)",
				fset.Position(pos))
		}
	}

	require.NotZero(t, total, "no responses found; the census is not reading the handler files")
	if leaking > 0 {
		t.Logf("%d of %d response bodies carry error text", leaking, total)
	} else {
		t.Logf("%d response bodies, none built from an error value", total)
	}
}

// The census reports a body built from an error's text whatever status the
// response carries and whatever the variable holding the text is named.
func TestResponsesBuiltFromErrorValuesFindsTheTextInAnyVariableAtAnyStatus(t *testing.T) {
	const probe = `package handlers

func probeStatus() int { return fiber.StatusInternalServerError }

func statusFromAFunction(c fiber.Ctx, err error) error {
	status := probeStatus()
	return c.Status(status).JSON(fiber.Map{"error": err.Error()})
}

func textCopiedIntoAVariable(c fiber.Ctx, err error) error {
	detail := err.Error()
	message := "lookup failed: " + detail
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": message})
}

func textFormattedIntoAVariable(c fiber.Ctx, lookupErr error) error {
	var reason string
	reason = fmt.Sprintf("lookup failed: %v", lookupErr)
	return c.Status(probeStatus()).JSON(ErrorResponse{Error: reason})
}

func errorCopiedIntoAVariable(c fiber.Ctx, err error) error {
	cause := err
	return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "lookup failed", "cause": cause})
}

func textTrimmedIntoAVariable(c fiber.Ctx, err error) error {
	detail := strings.TrimPrefix(err.Error(), "lookup: ")
	return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": detail})
}

func statedLine(c fiber.Ctx, err error) error {
	message := "lookup failed"
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": message})
	}
	return c.JSON(fiber.Map{"error": message})
}

func statedLineChosenForTheError(c fiber.Ctx, err error) error {
	refusal := refusalLine(err)
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": refusal})
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "probe.go", probe, 0)
	require.NoError(t, err)

	responses, found := responsesBuiltFromErrorValues(file)
	lines := make([]int, 0, len(found))
	for _, pos := range found {
		lines = append(lines, fset.Position(pos).Line)
	}
	assert.Equal(t, []int{7, 13, 19, 24, 29}, lines,
		"the five bodies built from an error value, and none of the three that carry a stated line")
	assert.Equal(t, 11, responses, "eight response calls, and the literals of the three that carry a stated line")
}

// responsesBuiltFromErrorValues counts the responses in file and returns the
// position of each one whose body is built from an error value. A response is
// a response call (responseBody) or a fiber.Map or ErrorResponse literal
// (isResponseLiteral); a literal inside a leaking call is not counted again.
func responsesBuiltFromErrorValues(file *ast.File) (responses int, leaking []token.Pos) {
	errorValues := errorValueVariables(file)
	ast.Inspect(file, func(n ast.Node) bool {
		var body []ast.Expr
		switch e := n.(type) {
		case *ast.CallExpr:
			args, ok := responseBody(e)
			if !ok {
				return true
			}
			body = args
		case *ast.CompositeLit:
			if !isResponseLiteral(e) {
				return true
			}
			body = []ast.Expr{e}
		default:
			return true
		}
		responses++
		for _, expr := range body {
			if mentionsErrorValue(expr, errorValues) {
				leaking = append(leaking, n.Pos())
				return false
			}
		}
		return true
	})
	return responses, leaking
}

// errorValueVariables returns the local variables of file that hold an error
// value or its text under a name isErrorVariableName does not recognise: each
// variable assigned (=, :=, += or var) an expression that hands one on
// (handsOnErrorValue), such as detail := err.Error(), reason :=
// fmt.Sprintf("...: %v", err) or cause := err, and each variable assigned from
// such a variable in turn. Variables are told apart by the parser's resolution
// of identifiers within the file, so a variable that shares a name with one of
// these elsewhere is not included. A value reaching a response through a
// function's result or parameters, a struct field or a map element is not
// followed.
func errorValueVariables(file *ast.File) map[*ast.Object]bool {
	errorValues := make(map[*ast.Object]bool)
	mark := func(lhs, rhs ast.Expr) bool {
		name, ok := lhs.(*ast.Ident)
		if !ok || name.Obj == nil || errorValues[name.Obj] || !handsOnErrorValue(rhs, errorValues, true) {
			return false
		}
		errorValues[name.Obj] = true
		return true
	}
	for changed := true; changed; {
		changed = false
		ast.Inspect(file, func(n ast.Node) bool {
			switch s := n.(type) {
			case *ast.AssignStmt:
				if len(s.Lhs) == len(s.Rhs) {
					for i := range s.Lhs {
						changed = mark(s.Lhs[i], s.Rhs[i]) || changed
					}
				}
			case *ast.ValueSpec:
				if len(s.Names) == len(s.Values) {
					for i := range s.Names {
						changed = mark(s.Names[i], s.Values[i]) || changed
					}
				}
			}
			return true
		})
	}
	return errorValues
}

// textForwardingCalls are the calls whose result carries the text of an error
// passed to them.
var textForwardingCalls = map[string]map[string]bool{
	"fmt":    {"Sprintf": true, "Sprint": true, "Sprintln": true, "Errorf": true},
	"errors": {"Join": true, "Unwrap": true},
}

// handsOnErrorValue reports whether an expression assigned to a variable hands
// on an error value or its text: it calls .Error(), passes an error value to
// one of textForwardingCalls, calls a method of an error value, or, where
// values count, mentions an error value as mentionsErrorValue does. An error
// value passed to any other function does not count: a function such as
// resolveRefusalLine(err) maps the error to a line of its own, and a response
// literal it builds is read where it is built. .Error() counts wherever it is
// called, inside such a function's arguments too.
func handsOnErrorValue(expr ast.Expr, errorValues map[*ast.Object]bool, valuesCount bool) bool {
	switch e := expr.(type) {
	case nil:
		return false
	case *ast.Ident:
		return valuesCount && (isErrorVariableName(e.Name) || (e.Obj != nil && errorValues[e.Obj]))
	case *ast.SelectorExpr:
		return handsOnErrorValue(e.X, errorValues, valuesCount)
	case *ast.KeyValueExpr:
		if _, isName := e.Key.(*ast.Ident); isName {
			return handsOnErrorValue(e.Value, errorValues, valuesCount)
		}
		return handsOnErrorValue(e.Key, errorValues, valuesCount) || handsOnErrorValue(e.Value, errorValues, valuesCount)
	case *ast.FuncLit:
		return false
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Error" && len(e.Args) == 0 {
			return true
		}
		forwards := false
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Obj == nil {
				forwards = textForwardingCalls[pkg.Name][sel.Sel.Name]
			}
		}
		if !forwards && handsOnErrorValue(e.Fun, errorValues, valuesCount) {
			return true
		}
		for _, arg := range e.Args {
			if handsOnErrorValue(arg, errorValues, forwards) {
				return true
			}
		}
		return false
	}
	// Any other expression hands on what one of its operands hands on.
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if found || n == nil {
			return false
		}
		if n == ast.Node(expr) {
			return true
		}
		if operand, ok := n.(ast.Expr); ok {
			found = handsOnErrorValue(operand, errorValues, valuesCount)
			return false
		}
		return true
	})
	return found
}

// responseBody returns the expressions that form the body of a response
// call: X.JSON, X.Send, X.SendString (with or without .Status(s) before it)
// and fiber.NewError(s, ...). Decoding a request (c.Bind().JSON) is not a
// response.
func responseBody(call *ast.CallExpr) ([]ast.Expr, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	switch sel.Sel.Name {
	case "NewError":
		if isIdent(sel.X, "fiber") && len(call.Args) > 1 {
			return call.Args[1:], true
		}
	case "JSON", "Send", "SendString":
		if inner, ok := sel.X.(*ast.CallExpr); ok {
			if innerSel, ok := inner.Fun.(*ast.SelectorExpr); ok && innerSel.Sel.Name == "Bind" {
				return nil, false
			}
		}
		return call.Args, true
	}
	return nil, false
}

// isResponseLiteral reports whether lit is a fiber.Map or an ErrorResponse,
// the two shapes a handler builds a body from.
func isResponseLiteral(lit *ast.CompositeLit) bool {
	switch typ := lit.Type.(type) {
	case *ast.SelectorExpr:
		return typ.Sel.Name == "Map" && isIdent(typ.X, "fiber")
	case *ast.Ident:
		return typ.Name == "ErrorResponse"
	}
	return false
}

// mentionsErrorValue reports whether expr calls .Error(), names a variable
// that by this package's naming holds an error, or names one of errorValues
// (errorValueVariables). Composite-literal keys and selector names
// (ErrorResponse{Error: ...}, x.Err) are names of fields, not values, and are
// skipped.
func mentionsErrorValue(expr ast.Expr, errorValues map[*ast.Object]bool) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if found {
			return false
		}
		switch e := n.(type) {
		case *ast.KeyValueExpr:
			if _, isName := e.Key.(*ast.Ident); isName {
				found = mentionsErrorValue(e.Value, errorValues)
				return false
			}
		case *ast.CallExpr:
			if sel, ok := e.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Error" && len(e.Args) == 0 {
				found = true
				return false
			}
		case *ast.SelectorExpr:
			found = mentionsErrorValue(e.X, errorValues)
			return false
		case *ast.Ident:
			found = isErrorVariableName(e.Name) || (e.Obj != nil && errorValues[e.Obj])
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
