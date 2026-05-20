package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type fakeResource struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
}

func resourceOrgID(r *fakeResource) uuid.UUID { return r.OrganizationID }

// TestLoadOwned_HappyPath: resource exists and is owned by the caller's org →
// helper returns the resource, no HTTP response written, no error.
func TestLoadOwned_HappyPath(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	callerOrg := uuid.New()
	resourceID := uuid.New()
	resource := &fakeResource{ID: resourceID, OrganizationID: callerOrg}

	loader := func(id uuid.UUID) (*fakeResource, error) {
		if id != resourceID {
			t.Fatalf("loader called with unexpected id %s", id)
		}
		return resource, nil
	}

	var got *fakeResource
	app.Get("/x", func(c fiber.Ctx) error {
		got = LoadOwned(c, loader, resourceID, callerOrg, resourceOrgID)
		if got != nil {
			return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": true})
		}
		return nil
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if got != resource {
		t.Fatalf("LoadOwned returned %v, want %v", got, resource)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("happy path got status %d, want 200", resp.StatusCode)
	}
}

// TestLoadOwned_CrossTenantReturns404: resource exists but belongs to a
// different org → helper writes 404 and returns nil. This is the structural
// fix for defects #18-25.
func TestLoadOwned_CrossTenantReturns404(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	callerOrg := uuid.New()
	otherOrg := uuid.New()
	resourceID := uuid.New()
	resource := &fakeResource{ID: resourceID, OrganizationID: otherOrg}

	loader := func(id uuid.UUID) (*fakeResource, error) {
		return resource, nil
	}

	var got *fakeResource
	app.Get("/x", func(c fiber.Ctx) error {
		got = LoadOwned(c, loader, resourceID, callerOrg, resourceOrgID)
		if got == nil {
			return nil
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": true})
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if got != nil {
		t.Fatalf("cross-tenant returned non-nil resource %v; want nil to prevent caller from using it", got)
	}
	if resp.StatusCode != 404 {
		t.Fatalf("cross-tenant got status %d, want 404 (NOT 403 — see SECURITY note in LoadOwned)", resp.StatusCode)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	var parsed map[string]any
	_ = json.Unmarshal(bodyBytes, &parsed)
	if parsed["error"] != "not found" {
		t.Fatalf("cross-tenant body = %v, want {error: not found} (no leak of org/resource shape)", parsed)
	}
}

// TestLoadOwned_NotFoundReturns404: loader returns (nil, ErrXxx) → 404.
// Same external observation as cross-tenant — no side channel.
func TestLoadOwned_NotFoundReturns404(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	callerOrg := uuid.New()
	resourceID := uuid.New()

	loader := func(id uuid.UUID) (*fakeResource, error) {
		return nil, errors.New("agent not found")
	}

	var got *fakeResource
	app.Get("/x", func(c fiber.Ctx) error {
		got = LoadOwned(c, loader, resourceID, callerOrg, resourceOrgID)
		if got == nil {
			return nil
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": true})
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if got != nil {
		t.Fatalf("loader error returned non-nil resource; want nil")
	}
	if resp.StatusCode != 404 {
		t.Fatalf("loader error got status %d, want 404", resp.StatusCode)
	}
}

// TestLoadOwned_NilResourceReturns404: loader returns (nil, nil) — buggy
// loader, but the helper must not panic and must not let the caller
// dereference a nil pointer.
func TestLoadOwned_NilResourceReturns404(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	callerOrg := uuid.New()
	resourceID := uuid.New()

	loader := func(id uuid.UUID) (*fakeResource, error) {
		return nil, nil
	}

	var got *fakeResource
	app.Get("/x", func(c fiber.Ctx) error {
		got = LoadOwned(c, loader, resourceID, callerOrg, resourceOrgID)
		if got == nil {
			return nil
		}
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"ok": true})
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/x", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if got != nil {
		t.Fatalf("nil resource path returned non-nil; want nil")
	}
	if resp.StatusCode != 404 {
		t.Fatalf("nil resource got status %d, want 404", resp.StatusCode)
	}
}

// TestLoadOwned_DoesNotLeakOrgIDOnCrossTenant: response body must be
// indistinguishable between not-found and cross-tenant. Defensive check that
// extends TestLoadOwned_CrossTenantReturns404.
func TestLoadOwned_DoesNotLeakOrgIDOnCrossTenant(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	callerOrg := uuid.New()
	otherOrg := uuid.New()
	resourceID := uuid.New()
	resource := &fakeResource{ID: resourceID, OrganizationID: otherOrg}

	loader := func(id uuid.UUID) (*fakeResource, error) {
		return resource, nil
	}

	app.Get("/cross", func(c fiber.Ctx) error {
		_ = LoadOwned(c, loader, resourceID, callerOrg, resourceOrgID)
		return nil
	})
	app.Get("/missing", func(c fiber.Ctx) error {
		_ = LoadOwned(c, func(uuid.UUID) (*fakeResource, error) { return nil, errors.New("nope") }, resourceID, callerOrg, resourceOrgID)
		return nil
	})

	cross, _ := app.Test(httptest.NewRequest("GET", "/cross", nil))
	missing, _ := app.Test(httptest.NewRequest("GET", "/missing", nil))

	if cross.StatusCode != missing.StatusCode {
		t.Fatalf("status code leaks cross-tenant signal: cross=%d, missing=%d", cross.StatusCode, missing.StatusCode)
	}
	crossBody, _ := io.ReadAll(cross.Body)
	missingBody, _ := io.ReadAll(missing.Body)
	if string(crossBody) != string(missingBody) {
		t.Fatalf("response body leaks cross-tenant signal:\n  cross=%q\n  missing=%q", crossBody, missingBody)
	}
}

// TestLoadOwned_HandlerReturningNilPreserves404: this is the regression test
// for the bug I caught during pre-push gate — when the handler returns the
// helper's err sentinel (the OLD contract), Fiber's default error handler
// overwrites the 404 with 500. The new contract (helper returns *T only;
// caller returns nil after seeing nil resource) is verified here end-to-end.
func TestLoadOwned_HandlerReturningNilPreserves404(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	callerOrg := uuid.New()
	otherOrg := uuid.New()
	resourceID := uuid.New()
	resource := &fakeResource{ID: resourceID, OrganizationID: otherOrg}

	app.Get("/probe", func(c fiber.Ctx) error {
		got := LoadOwned(c,
			func(uuid.UUID) (*fakeResource, error) { return resource, nil },
			resourceID, callerOrg, resourceOrgID,
		)
		if got == nil {
			return nil // <-- the contract; do NOT return an error
		}
		return c.JSON(got)
	})

	resp, _ := app.Test(httptest.NewRequest("GET", "/probe", nil))
	if resp.StatusCode != 404 {
		t.Fatalf("regression: got %d, want 404. Fiber's default error handler may be overwriting the helper's 404", resp.StatusCode)
	}
}
