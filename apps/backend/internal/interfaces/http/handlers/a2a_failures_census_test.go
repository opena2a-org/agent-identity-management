package handlers

// Census of the responses a2a_handler.go builds. An A2A handler answers a
// failure by naming it to respondA2AFailure or one of its siblings in
// a2a_failures.go, which hold the status and reason of every failure; the
// handler file builds only its success responses. The census fails on a
// Status or SendStatus call whose status is not a 2xx fiber constant, and on
// a call to respondServerError, respondResourceNotFound or fiber.NewError.

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var a2aSuccessStatuses = map[string]bool{
	"StatusOK":        true,
	"StatusCreated":   true,
	"StatusAccepted":  true,
	"StatusNoContent": true,
}

var a2aFailureWriters = map[string]bool{
	"respondA2AFailure":          true,
	"respondA2AFailureCause":     true,
	"respondA2AVerifyRefused":    true,
	"respondA2ANoAgentForIntent": true,
	"respondAgentCardError":      true,
}

// a2aResponsesOutsideTheMapping returns each response src builds outside the
// failure mapping, and the number of failures it answers through it.
func a2aResponsesOutsideTheMapping(t *testing.T, filename string, src any) ([]string, int) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, 0)
	require.NoError(t, err)

	var found []string
	mapped := 0
	report := func(call *ast.CallExpr) {
		var text bytes.Buffer
		_ = printer.Fprint(&text, fset, call.Fun)
		found = append(found, fmt.Sprintf("%s: %s(...)", fset.Position(call.Pos()), text.String()))
	}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			switch fn.Sel.Name {
			case "Status", "SendStatus":
				if len(call.Args) != 1 || !isA2ASuccessStatus(call.Args[0]) {
					report(call)
				}
			case "NewError":
				if isIdent(fn.X, "fiber") {
					report(call)
				}
			}
		case *ast.Ident:
			switch {
			case fn.Name == "respondServerError" || fn.Name == "respondResourceNotFound":
				report(call)
			case a2aFailureWriters[fn.Name]:
				mapped++
			}
		}
		return true
	})
	return found, mapped
}

func isA2ASuccessStatus(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	return ok && isIdent(sel.X, "fiber") && a2aSuccessStatuses[sel.Sel.Name]
}

func TestA2AHandlerAnswersEveryFailureThroughTheMapping(t *testing.T) {
	found, mapped := a2aResponsesOutsideTheMapping(t, "a2a_handler.go", nil)
	for _, f := range found {
		t.Errorf("%s builds a failure response outside the mapping; name the failure to respondA2AFailure (a2a_failures.go)", f)
	}
	require.NotZero(t, mapped, "no failure answers found; the census is not reading the handler file")
	if !t.Failed() {
		t.Logf("a2a_handler.go answers %d failures through the mapping and builds none itself", mapped)
	}
}

// The census refuses each way a handler can build a failure response itself,
// and admits the success responses.
func TestA2AFailureCensusRefusesAResponseBuiltOutsideTheMapping(t *testing.T) {
	src := `package handlers

func f(c fiber.Ctx, code int, err error) error {
	_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid agent ID"})
	_ = c.SendStatus(fiber.StatusNotFound)
	_ = c.Status(code).JSON(fiber.Map{})
	_ = respondServerError(c, fiber.StatusInternalServerError, err)
	respondResourceNotFound(c)
	_ = fiber.NewError(fiber.StatusUnauthorized, "no")
	_ = c.Status(fiber.StatusCreated).JSON(fiber.Map{})
	_ = c.SendStatus(fiber.StatusNoContent)
	return respondA2AFailure(c, a2aInvalidAgentID)
}
`
	found, mapped := a2aResponsesOutsideTheMapping(t, "planted.go", src)
	assert.Len(t, found, 6, "found: %v", found)
	assert.Equal(t, 1, mapped)
}

func TestEveryA2AFailureHasAnAnswer(t *testing.T) {
	assert.Len(t, a2aAnswers, int(a2aFailureEnd)-1, "every kind has one answer and every answer a kind")
	for kind := a2aFailure(1); kind < a2aFailureEnd; kind++ {
		answer, ok := a2aAnswers[kind]
		if !assert.Truef(t, ok, "failure %d has no answer", kind) {
			continue
		}
		assert.GreaterOrEqualf(t, answer.status, 400, "failure %d", kind)
		assert.Lessf(t, answer.status, 600, "failure %d", kind)
		if kind != a2aVerifyRefused {
			assert.NotEmptyf(t, answer.reason, "failure %d states no reason", kind)
		}
		if answer.verdict {
			assert.Lessf(t, answer.status, 500, "failure %d: a 500 says nothing about the request", kind)
		}
	}
}
