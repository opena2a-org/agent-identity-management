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

// apiKeyOrgID extracts OrganizationID from a *domain.APIKey. Used by
// APIKeyHandler.DisableAPIKey + DeleteAPIKey to gate cross-tenant
// API-key revoke + delete BEFORE the service is invoked. The
// service-layer check (api_key_service.go:102, 117) already returns
// a "does not belong to organization" error for cross-tenant, but
// the handler echoes err.Error() in the response body — an
// existence side channel.
func apiKeyOrgID(k *domain.APIKey) uuid.UUID { return k.OrganizationID }

// securityPolicyOrgID extracts OrganizationID from a
// *domain.SecurityPolicy. Used by SecurityPolicyHandler's
// Get/Update/Delete/Toggle paths to gate cross-tenant policy reads +
// mutations (HIGH IDOR cluster — any authenticated user could
// otherwise read/modify/delete any other tenant's policies).
func securityPolicyOrgID(p *domain.SecurityPolicy) uuid.UUID { return p.OrganizationID }

// consentOrgID extracts OrganizationID from a *domain.A2AConsentRecord.
// A2AConsentRecord.OrganizationID is *uuid.UUID (nullable); a nil pointer
// surfaces as uuid.Nil. LoadOwned treats both uuid.Nil resource org and
// uuid.Nil caller org as cross-tenant (404), so unassigned consent
// records are unreachable via LoadOwned. This is intentional: an
// unassigned consent has no tenant owner authorized to mutate it.
func consentOrgID(c *domain.A2AConsentRecord) uuid.UUID {
	if c.OrganizationID == nil {
		return uuid.Nil
	}
	return *c.OrganizationID
}

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
	// Defense-in-depth: refuse to compare against a uuid.Nil caller or
	// resource org. A uuid.Nil callerOrgID would indicate a bug in the
	// middleware chain (auth pipeline failed to populate Locals but the
	// handler proceeded); a uuid.Nil resource org indicates an
	// unassigned record (e.g. an A2AConsentRecord with OrganizationID
	// nil — see consentOrgID in this file). Matching uuid.Nil to
	// uuid.Nil would let either bug surface as cross-tenant access.
	if callerOrgID == uuid.Nil || orgIDOf == nil {
		respondResourceNotFound(c)
		return nil
	}
	resource, err := loader(resourceID)
	if err != nil || resource == nil {
		respondResourceNotFound(c)
		return nil
	}
	resourceOrgID := orgIDOf(resource)
	if resourceOrgID == uuid.Nil || resourceOrgID != callerOrgID {
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
	// Defense-in-depth: refuse to compare against a uuid.Nil caller.
	// See LoadOwned for the rationale.
	if callerOrgID == uuid.Nil {
		respondResourceNotFound(c)
		return nil
	}
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
	if agent.OrganizationID == uuid.Nil || agent.OrganizationID != callerOrgID {
		respondResourceNotFound(c)
		return nil
	}
	return resource
}

// CallerAgentID returns the agent UUID established by an agent-authenticating
// middleware. A principal that set no "agent_id" local, or set it as some other
// type, yields uuid.Nil.
//
// SCOPE, and read this before mounting a handler that uses it on another group:
// several middlewares in this package set a uuid.UUID under "agent_id" —
// pqc_agent_auth.go:162, ed25519_agent_auth.go:215, api_key.go:130 and :224,
// atc_auth.go:97, service_principal.go:121. This helper does NOT distinguish
// between them. It is safe on the sdkAPI group specifically because that chain
// is PQC → JWT → activity-touch → rate limit (cmd/server/main.go), so the only
// principal that can set agent_id there is the signature-authenticated one.
// Mount a handler that relies on this under an API-key, ATC or service-principal
// group and the ownership check silently begins accepting those principals as
// the agent.
//
// NOTE for anyone diffing this against aim-cloud: the two trees mount DIFFERENT
// middleware on this group — public runs PQCAgentMiddleware, cloud runs
// Ed25519AgentMiddleware. Both set agent_id as a uuid.UUID and both fall through
// to JWT, so the reasoning holds in both, but the file to read differs.
//
// SECURITY: this exists so the ownership check cannot be written as
//
//	if id, ok := c.Locals("agent_id").(uuid.UUID); ok && id != want { reject }
//
// which is the idiom two files away at trust_score_handler.go:415. That shape
// PERMITS when the assertion fails, and on the sdkAPI group the assertion fails
// for every human caller: PQCAgentMiddleware hands off to the JWT middleware
// whenever an Authorization header is present (pqc_agent_auth.go:39-41) or the
// signature headers are absent (:50-52), and AuthMiddleware sets user_id /
// organization_id / email / role and never agent_id (auth.go:132-135). Absence
// of an agent principal is a rejection, not a reason to skip the check.
//
// Returning uuid.Nil rather than an ok flag is deliberate: LoadOwnedByAgent
// refuses uuid.Nil before it touches the loader, so the fail-closed behaviour is
// a property of the pair rather than of the caller remembering to check.
func CallerAgentID(c fiber.Ctx) uuid.UUID {
	if agentID, ok := c.Locals("agent_id").(uuid.UUID); ok {
		return agentID
	}
	return uuid.Nil
}

// LoadOwnedByAgent is the agent-principal variant of LoadOwned: ownership is
// established against the calling AGENT, not the calling organization.
//
// The sdkAPI group authenticates *an* agent; it does not establish that this
// agent owns *this* resource. Organization scoping is not a substitute — two
// agents in one organization are distinct principals, and the verification
// events this guards are per-agent authorization records.
//
// Every failure path THIS HELPER takes — non-agent caller, loader error, missing
// resource, row with no agent, different agent — writes the same 404 body and
// returns nil, so the response cannot be used to test for the existence of
// another agent's row. Same 404-not-403 rationale as LoadOwned; the caller must
// `return nil`.
//
// Note the qualifier: a caller may emit a DIFFERENT 404 body later, after this
// helper has already returned the resource (UpdateExecutionStatus does, when the
// write itself affects no row). That is not an oracle — by then the caller has
// been proven to own the row — but it does mean "every 404 from this route is
// byte-identical" is false, and no code should rely on it.
//
// agentIDOf returns a *uuid.UUID because the owning column is nullable on the
// domain types that need this (domain.VerificationEvent.AgentID is *uuid.UUID —
// an event may target an MCP server instead of an agent). A nil owner is
// refused rather than treated as a wildcard.
func LoadOwnedByAgent[T any](
	c fiber.Ctx,
	loader func(uuid.UUID) (*T, error),
	resourceID uuid.UUID,
	callerAgentID uuid.UUID,
	agentIDOf func(*T) *uuid.UUID,
) *T {
	// Defense-in-depth: a uuid.Nil caller is a non-agent principal (see
	// CallerAgentID) or a middleware-chain bug. Refuse before the loader runs
	// so no existence signal is produced. Matching uuid.Nil to uuid.Nil would
	// let either case surface as ownership.
	if callerAgentID == uuid.Nil || agentIDOf == nil {
		respondResourceNotFound(c)
		return nil
	}
	resource, err := loader(resourceID)
	if err != nil || resource == nil {
		respondResourceNotFound(c)
		return nil
	}
	ownerAgentID := agentIDOf(resource)
	if ownerAgentID == nil || *ownerAgentID == uuid.Nil || *ownerAgentID != callerAgentID {
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
