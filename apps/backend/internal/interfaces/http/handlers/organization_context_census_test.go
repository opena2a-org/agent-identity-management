package handlers

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// uncheckedOrganizationAssertions returns the position of every single-value
// assertion Locals("organization_id").(T) in file. The two-value form
// `orgID, ok := c.Locals("organization_id").(uuid.UUID)` is checked by its
// caller; the single-value form panics when the request carries no
// organization.
func uncheckedOrganizationAssertions(fset *token.FileSet, file *ast.File) []string {
	var found []string
	var stack []ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		parent := ast.Node(nil)
		if len(stack) > 0 {
			parent = stack[len(stack)-1]
		}
		stack = append(stack, n)

		assertion, ok := n.(*ast.TypeAssertExpr)
		if !ok || assertion.Type == nil || !isOrganizationLocals(assertion.X) {
			return true
		}
		switch p := parent.(type) {
		case *ast.AssignStmt:
			if len(p.Lhs) == 2 && len(p.Rhs) == 1 {
				return true
			}
		case *ast.ValueSpec:
			if len(p.Names) == 2 && len(p.Values) == 1 {
				return true
			}
		}
		found = append(found, fset.Position(assertion.Pos()).String())
		return true
	})
	return found
}

func isOrganizationLocals(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Locals" {
		return false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	key, err := strconv.Unquote(lit.Value)
	return err == nil && key == "organization_id"
}

// A handler reached without an organization in its context answers the 401
// that RequireOrganizationID returns. A single-value assertion on
// c.Locals("organization_id") panics there instead.
func TestHandlersReadTheOrganizationWithoutAnUncheckedAssertion(t *testing.T) {
	// Planted control: the census must catch each single-value shape and pass
	// the two-value one, or an empty result proves nothing.
	const control = `package p
func a(c ctx) { orgID := c.Locals("organization_id").(uuid.UUID); _ = orgID }
func b(c ctx) { use(c.Locals("organization_id").(uuid.UUID)) }
func d(c ctx) { orgID, ok := c.Locals("organization_id").(uuid.UUID); _, _ = orgID, ok }
func e(c ctx) { var orgID, ok = c.Locals("organization_id").(uuid.UUID); _, _ = orgID, ok }
`
	fset := token.NewFileSet()
	controlFile, err := parser.ParseFile(fset, "control.go", control, 0)
	if err != nil {
		t.Fatalf("parse control: %v", err)
	}
	if got := uncheckedOrganizationAssertions(fset, controlFile); len(got) != 2 {
		t.Fatalf("the census found %d of the 2 planted single-value assertions: %v", len(got), got)
	}

	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	scanned := 0
	var unchecked []string
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		scanned++
		unchecked = append(unchecked, uncheckedOrganizationAssertions(fset, file)...)
	}
	if scanned == 0 {
		t.Fatal("no handler source files were scanned")
	}
	if len(unchecked) > 0 {
		t.Errorf("%d handler line(s) read the organization with a single-value assertion; use RequireOrganizationID:\n%s",
			len(unchecked), strings.Join(unchecked, "\n"))
	}
}

// A request that reaches one of these handlers with no organization in its
// context is answered 401, not a panic.
func TestHandlersWithoutAnOrganizationAnswer401(t *testing.T) {
	for name, handler := range map[string]fiber.Handler{
		"TrustScoreHandler.CalculateTrustScore": (&TrustScoreHandler{}).CalculateTrustScore,
		"WebhookHandler.ListWebhooks":           (&WebhookHandler{}).ListWebhooks,
		"ComplianceHandler.GetComplianceStatus": (&ComplianceHandler{}).GetComplianceStatus,
		"MCPHandler.ListMCPServers":             (&MCPHandler{}).ListMCPServers,
	} {
		t.Run(name, func(t *testing.T) {
			app := fiber.New()
			app.Post("/probe/:id", handler)
			resp, err := app.Test(httptest.NewRequest("POST", "/probe/"+uuid.NewString(), nil))
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != fiber.StatusUnauthorized {
				t.Fatalf("status %d; want 401", resp.StatusCode)
			}
		})
	}
}
