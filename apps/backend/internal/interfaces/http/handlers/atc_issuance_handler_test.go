package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// These tests are a no-DB route-registration smoke for POST /agents/:id/atc.
// They exercise the authz + validation paths that return before any service or
// repository call, so the handler can be wired with nil dependencies. Live
// end-to-end verification (against Postgres + the Registry) is Phase D.

func TestATCIssuanceHandler_MissingOrg_Unauthorized(t *testing.T) {
	h := NewATCIssuanceHandler(nil, nil, nil)
	app := fiber.New()
	app.Post("/agents/:id/atc", h.IssueATC)

	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/atc", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 when organization_id is absent", resp.StatusCode)
	}
}

func TestATCIssuanceHandler_NilOrg_Unauthorized(t *testing.T) {
	h := NewATCIssuanceHandler(nil, nil, nil)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		// A malformed token could set a zero-value org. It must not authorize: a
		// Nil orgID would compare equal to a Nil-org agent and collapse the tenant
		// check.
		c.Locals("organization_id", uuid.Nil)
		c.Locals("user_id", uuid.New())
		return c.Next()
	})
	app.Post("/agents/:id/atc", h.IssueATC)

	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/atc", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 when organization_id is the zero UUID", resp.StatusCode)
	}
}

func TestATCIssuanceHandler_BadAgentID_BadRequest(t *testing.T) {
	h := NewATCIssuanceHandler(nil, nil, nil)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return c.Next()
	})
	app.Post("/agents/:id/atc", h.IssueATC)

	req := httptest.NewRequest("POST", "/agents/not-a-uuid/atc", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for an invalid agent ID", resp.StatusCode)
	}
}
