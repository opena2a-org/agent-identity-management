package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// agentOrgID extracts OrganizationID from a *domain.Agent for use with
// LoadOwned. Defined once at package scope to keep handler call sites concise
// and to give the AST lint a stable symbol name to recognize.
func agentOrgID(a *domain.Agent) uuid.UUID { return a.OrganizationID }

// tagOrgID extracts OrganizationID from a *domain.Tag.
func tagOrgID(t *domain.Tag) uuid.UUID { return t.OrganizationID }

// auditLogOrgID extracts OrganizationID from a *domain.AuditLog.
func auditLogOrgID(a *domain.AuditLog) uuid.UUID { return a.OrganizationID }

// verificationEventOrgID extracts OrganizationID from a
// *domain.VerificationEvent.
func verificationEventOrgID(v *domain.VerificationEvent) uuid.UUID { return v.OrganizationID }

// mcpServerOrgID extracts OrganizationID from a *domain.MCPServer.
func mcpServerOrgID(m *domain.MCPServer) uuid.UUID { return m.OrganizationID }

// userOrgID extracts OrganizationID from a *domain.User. Used by
// AdminHandler approval flows (A3d-v) to gate cross-tenant approve /
// reject on a target user before the service is invoked.
func userOrgID(u *domain.User) uuid.UUID { return u.OrganizationID }

// LoadOwned loads a resource by ID via the provided loader and verifies the
// caller's organization owns it. On any failure path (loader error,
// cross-tenant mismatch, nil resource) the helper writes HTTP 404 to the
// response and returns nil; on success it returns the resource.
//
// Calling pattern:
//
//	resource := LoadOwned(c, loader, resourceID, callerOrgID, orgIDOf)
//	if resource == nil {
//	    return nil // helper already wrote the 404 response
//	}
//	// use resource
//
// SECURITY: 404 — not 403 — is intentional for cross-tenant access. Returning
// 403 would leak the existence of the resource in another organization, a
// cross-tenant side channel that lets an attacker enumerate valid resource
// IDs across the system. By returning 404 for both "row does not exist" and
// "row exists in another org" the response is indistinguishable from the
// caller's perspective.
//
// The helper returns nil rather than an error sentinel because Fiber's
// default error handler overrides response status when a handler returns a
// non-nil error. Writing 404 and then returning a sentinel would let Fiber
// rewrite the response to 500, defeating the security guarantee. The caller
// must `return nil` after seeing a nil resource so Fiber considers the
// response complete.
//
// The orgIDOf accessor is provided by the caller because Go generics cannot
// reach into struct fields by name. Every AIM domain type that participates
// in tenant scoping has an OrganizationID uuid.UUID field; the accessor is
// usually a one-line lambda like `func(a *domain.Agent) uuid.UUID { return a.OrganizationID }`.
//
// Loader-error distinction (not-found vs database failure) is intentionally
// collapsed here. The repository layer in this codebase returns a plain
// error("...not found") string for missing rows rather than a sentinel, so a
// type-based check would be fragile. A future PR can introduce
// domain.ErrNotFound and distinguish 404 from 500 here.
func LoadOwned[T any](
	c fiber.Ctx,
	loader func(uuid.UUID) (*T, error),
	resourceID uuid.UUID,
	callerOrgID uuid.UUID,
	orgIDOf func(*T) uuid.UUID,
) *T {
	resource, err := loader(resourceID)
	if err != nil || resource == nil {
		respondResourceNotFound(c)
		return nil
	}
	if orgIDOf(resource) != callerOrgID {
		respondResourceNotFound(c)
		return nil
	}
	return resource
}

// LoadOwnedViaAgent is the FK-lookup variant of LoadOwned for resources whose
// tenant ownership is determined by their associated agent rather than their
// own OrganizationID field. Used for capability requests, verification
// events that reference an agent, and other agent-scoped child resources.
//
// The flow: load the resource → load the referenced agent via agentRepo →
// verify agent.OrganizationID == callerOrgID. Any failure along the way
// produces the same 404 as LoadOwned, preserving the no-side-channel
// guarantee.
func LoadOwnedViaAgent[T any](
	c fiber.Ctx,
	loader func(uuid.UUID) (*T, error),
	resourceID uuid.UUID,
	callerOrgID uuid.UUID,
	agentIDOf func(*T) uuid.UUID,
	agentRepo domain.AgentRepository,
) *T {
	resource, err := loader(resourceID)
	if err != nil || resource == nil {
		respondResourceNotFound(c)
		return nil
	}
	agentID := agentIDOf(resource)
	if agentID == uuid.Nil {
		respondResourceNotFound(c)
		return nil
	}
	agent, err := agentRepo.GetByID(agentID)
	if err != nil || agent == nil {
		respondResourceNotFound(c)
		return nil
	}
	if agent.OrganizationID != callerOrgID {
		respondResourceNotFound(c)
		return nil
	}
	return resource
}

func respondResourceNotFound(c fiber.Ctx) {
	_ = c.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"error": "not found",
	})
}
