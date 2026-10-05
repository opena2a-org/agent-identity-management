package handlers

// Census of the Require* context-helper call sites in this package. Each
// helper fails with a 401 *fiber.Error that the app's error handler turns
// into the response, so a principal that passed authentication without an
// organization or user gets that 401 only if the call site hands the error
// back unchanged:
//
//	orgID, err := RequireOrganizationID(c)
//	if err != nil {
//		return err
//	}
//
// inside a function whose only result is error. Wrapping or replacing the
// error turns the 401 into a 500; dropping it lets the handler run on with
// uuid.Nil as the organization.

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var contextHelperNames = map[string]bool{
	"RequireOrganizationID": true,
	"RequireUserID":         true,
	"RequireOrgAndUserID":   true,
}

func TestContextHelpersFailWith401FiberError(t *testing.T) {
	app := fiber.New()
	var errs []error
	app.Get("/probe", func(c fiber.Ctx) error {
		_, err := RequireOrganizationID(c)
		errs = append(errs, err)
		_, err = RequireUserID(c)
		errs = append(errs, err)
		_, _, err = RequireOrgAndUserID(c)
		errs = append(errs, err)
		c.Locals("organization_id", uuid.New())
		_, _, err = RequireOrgAndUserID(c)
		errs = append(errs, err)
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/probe", nil))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Len(t, errs, 4)
	for i, err := range errs {
		var fe *fiber.Error
		require.Truef(t, errors.As(err, &fe), "helper error %d is %T, want *fiber.Error", i, err)
		assert.Equalf(t, fiber.StatusUnauthorized, fe.Code, "helper error %d", i)
	}
}

func TestContextHelperCallSitesReturnTheHelperError(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	require.NoError(t, err)

	fset := token.NewFileSet()
	total := 0
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") || path == "context_helpers.go" {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err)

		conforming := conformingContextHelperCalls(file)
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			ident, ok := call.Fun.(*ast.Ident)
			if !ok || !contextHelperNames[ident.Name] {
				return true
			}
			total++
			if !conforming[call] {
				t.Errorf("%s: %s must be assigned to an err variable that the next statement returns unchanged (if err != nil { return err }) in a function whose only result is error",
					fset.Position(call.Pos()), ident.Name)
			}
			return true
		})
	}

	require.NotZero(t, total, "no context-helper call sites found; the census is not reading the handler files")
	if !t.Failed() {
		t.Logf("%d context-helper call sites, each returns the helper's 401 unchanged", total)
	}
}

// conformingContextHelperCalls returns the helper calls in file that are
// followed by `if err != nil { return err }` inside a function returning
// only error.
func conformingContextHelperCalls(file *ast.File) map[*ast.CallExpr]bool {
	conforming := map[*ast.CallExpr]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		var fnType *ast.FuncType
		var body *ast.BlockStmt
		switch fn := n.(type) {
		case *ast.FuncDecl:
			fnType, body = fn.Type, fn.Body
		case *ast.FuncLit:
			fnType, body = fn.Type, fn.Body
		default:
			return true
		}
		if body == nil || !returnsOnlyError(fnType) {
			return true
		}
		ast.Inspect(body, func(m ast.Node) bool {
			if _, nested := m.(*ast.FuncLit); nested {
				return false // checked against its own signature
			}
			block, ok := m.(*ast.BlockStmt)
			if !ok {
				return true
			}
			for i := 0; i+1 < len(block.List); i++ {
				call, errName := contextHelperAssignment(block.List[i])
				if call != nil && returnsErrUnchanged(block.List[i+1], errName) {
					conforming[call] = true
				}
			}
			return true
		})
		return true
	})
	return conforming
}

func returnsOnlyError(fnType *ast.FuncType) bool {
	if fnType.Results == nil || len(fnType.Results.List) != 1 || len(fnType.Results.List[0].Names) > 1 {
		return false
	}
	ident, ok := fnType.Results.List[0].Type.(*ast.Ident)
	return ok && ident.Name == "error"
}

// contextHelperAssignment matches `..., err := RequireX(c)` (or `=`) and
// returns the call and the name the error is bound to.
func contextHelperAssignment(stmt ast.Stmt) (*ast.CallExpr, string) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || len(assign.Rhs) != 1 || len(assign.Lhs) == 0 {
		return nil, ""
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok {
		return nil, ""
	}
	fn, ok := call.Fun.(*ast.Ident)
	if !ok || !contextHelperNames[fn.Name] {
		return nil, ""
	}
	errIdent, ok := assign.Lhs[len(assign.Lhs)-1].(*ast.Ident)
	if !ok || errIdent.Name == "_" {
		return nil, ""
	}
	return call, errIdent.Name
}

// returnsErrUnchanged matches `if errName != nil { return errName }`.
func returnsErrUnchanged(stmt ast.Stmt, errName string) bool {
	ifStmt, ok := stmt.(*ast.IfStmt)
	if !ok || ifStmt.Init != nil || ifStmt.Else != nil || len(ifStmt.Body.List) != 1 {
		return false
	}
	cond, ok := ifStmt.Cond.(*ast.BinaryExpr)
	if !ok || cond.Op != token.NEQ || !isIdent(cond.X, errName) || !isIdent(cond.Y, "nil") {
		return false
	}
	ret, ok := ifStmt.Body.List[0].(*ast.ReturnStmt)
	return ok && len(ret.Results) == 1 && isIdent(ret.Results[0], errName)
}

func isIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}
