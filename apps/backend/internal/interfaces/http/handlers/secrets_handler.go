package handlers

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// SecretsHandler handles HTTP requests for the identity-native secrets
// management API.
//
// SECURITY (A3d-iii): the handler holds an agentRepo so the four
// namespace-path-id routes (GetNamespace, DeleteNamespace,
// StoreCredential, RotateCredential) can verify the namespace's owning
// agent belongs to the caller's organization BEFORE invoking the
// service. SecretNamespace has no OrganizationID column; ownership is
// established via the namespace.AgentID -> Agent.OrganizationID chain,
// so the LoadOwnedViaAgent helper is the correct guard. Cross-tenant
// requests return 404 with body {"error":"not found"} (existence
// secrecy). See tenant_scope.go:84-121.
type SecretsHandler struct {
	secretsService *application.SecretsService
	agentRepo      domain.AgentRepository
	// jwtService reads the acting session's revocation before a credential is
	// rotated; auditService records a refused rotation.
	jwtService   *auth.JWTService
	auditService *application.AuditService
}

func NewSecretsHandler(secretsService *application.SecretsService, agentRepo domain.AgentRepository, jwtService *auth.JWTService, auditService *application.AuditService) *SecretsHandler {
	return &SecretsHandler{
		secretsService: secretsService,
		agentRepo:      agentRepo,
		jwtService:     jwtService,
		auditService:   auditService,
	}
}

// --- Request/Response types ---

type resolveRequest struct {
	Namespace      string `json:"namespace"`
	Operation      string `json:"operation"`
	Nonce          string `json:"nonce"`
	Signature      string `json:"signature"`
	AgentPublicKey string `json:"agentPublicKey"`
	ATCID          string `json:"atcId,omitempty"`
}

type resolveResponse struct {
	EncryptedBlob   string `json:"encryptedBlob"`
	EphemeralPubKey string `json:"ephemeralPubKey"`
	EncryptionAlg   string `json:"encryptionAlg"`
}

type createNamespaceRequest struct {
	AgentID     uuid.UUID `json:"agentId"`
	Namespace   string    `json:"namespace"`
	BackendType string    `json:"backendType,omitempty"`
	Operations  []string  `json:"operations"`
	URLPatterns []string  `json:"urlPatterns,omitempty"`
}

type storeCredentialRequest struct {
	EncryptedBlob string `json:"encryptedBlob"`
	EncryptionAlg string `json:"encryptionAlg,omitempty"`
}

type rotateCredentialRequest struct {
	EncryptedBlob string `json:"encryptedBlob"`
	EncryptionAlg string `json:"encryptionAlg,omitempty"`
}

// --- Handlers ---

// ResolveCredential handles POST /api/v1/secrets/resolve
// Auth: ATCAuthMiddleware (ATC) or PQCAgentMiddleware (Ed25519 signature)
//
// When authenticated via ATC:
//   - ATC capabilities are checked instead of AIM capability table
//   - Body signature + nonce still required (defense in depth)
//
// When authenticated via Ed25519/JWT:
//   - Falls back to existing flow (body signature + capability table)
//
// On both routes the credential is encrypted to the agent's registered key,
// agentPublicKey must be that key, and a signed request is answered once.
// When the single-use check cannot run the answer is 503 and nothing is
// resolved. When the revocation state of an ATC sent in the body cannot be
// read the answer is 503 atc_revocation_unknown with Retry-After.
func (h *SecretsHandler) ResolveCredential(c fiber.Ctx) error {
	agentIDValue := c.Locals("agent_id")
	if agentIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Agent authentication required"})
	}
	agentID := agentIDValue.(uuid.UUID)

	var req resolveRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Namespace == "" || req.Operation == "" || req.Nonce == "" || req.Signature == "" || req.AgentPublicKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing required fields: namespace, operation, nonce, signature, agentPublicKey"})
	}

	sig, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid signature encoding"})
	}

	pubKey, err := base64.StdEncoding.DecodeString(req.AgentPublicKey)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid public key encoding"})
	}

	domainReq := &secrets.ResolutionRequest{
		Namespace:      req.Namespace,
		Operation:      req.Operation,
		Nonce:          req.Nonce,
		Signature:      sig,
		AgentPublicKey: pubKey,
		ATCID:          req.ATCID,
	}

	// Pass ATC claims if agent authenticated via ATC middleware
	if atcClaimsVal := c.Locals("atc_claims"); atcClaimsVal != nil {
		if atcClaims, ok := atcClaimsVal.(*atcdomain.ATCClaims); ok {
			domainReq.ATCClaims = atcClaims
		}
	}

	result, err := h.secretsService.Resolve(agentID, domainReq)
	if err != nil {
		if errors.Is(err, application.ErrSecretsNonceAdmissionUnavailable) {
			log.Printf("secrets resolve refused: nonce check unavailable (error_class=%s)", freshnessErrorClass(err))
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Nonce check unavailable; retry the request with a new nonce"})
		}
		// The body ATC's revocation state could not be read: the answer the
		// ATC middleware gives a header ATC in the same case, as stated lines.
		if errors.Is(err, application.ErrResolveATCRevocationUnknown) {
			log.Printf("secrets resolve refused for agent %s: %v", agentID, err)
			c.Set(fiber.HeaderRetryAfter, strconv.Itoa(int(atcdomain.RevocationUnknownRetryAfter/time.Second)))
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error":   atcdomain.ErrCodeRevocationUnknown,
				"message": atcdomain.RevocationUnknownMessage,
			})
		}
		log.Printf("secrets resolve refused for agent %s: %v", agentID, err)
		refusal := resolveRefusalLine(err)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": refusal})
	}

	return c.Status(fiber.StatusOK).JSON(resolveResponse{
		EncryptedBlob:   base64.StdEncoding.EncodeToString(result.EncryptedBlob),
		EphemeralPubKey: base64.StdEncoding.EncodeToString(result.EphemeralPubKey),
		EncryptionAlg:   result.EncryptionAlg,
	})
}

// CreateNamespace handles POST /api/v1/secrets/namespaces
// Auth: AuthMiddleware + MemberMiddleware (JWT)
func (h *SecretsHandler) CreateNamespace(c fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var req createNamespaceRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Namespace == "" || req.AgentID == uuid.Nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing required fields: agentId, namespace"})
	}

	// SECURITY: agentId arrives in the REQUEST BODY and was previously used without
	// checking who owns it, so any authenticated user of any organization could attach a
	// secrets namespace to another organization's agent by naming its UUID. Registration
	// is open, so the only prerequisite was an account and a target agent id.
	//
	// GetNamespace in this same file already scopes via the agent; this is the same check
	// applied to the write path. LoadOwned answers 404 for a cross-tenant or unknown id,
	// so it does not disclose whether the agent exists.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if LoadOwned(c, h.agentRepo.GetByID, req.AgentID, orgID, agentOrgID) == nil {
		return nil
	}

	ns := &secrets.SecretNamespace{
		AgentID:     req.AgentID,
		Namespace:   req.Namespace,
		BackendType: secrets.BackendType(req.BackendType),
		Operations:  req.Operations,
		URLPatterns: req.URLPatterns,
		CreatedBy:   userID,
	}

	if err := h.secretsService.CreateNamespace(ns); err != nil {
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}
	h.recordSecretsChange(c, orgID, domain.AuditActionCreate, secretNamespaceResource, ns)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":          ns.ID,
		"agentId":     ns.AgentID,
		"namespace":   ns.Namespace,
		"backendType": ns.BackendType,
		"operations":  ns.Operations,
		"urlPatterns": ns.URLPatterns,
		"status":      ns.Status,
		"createdAt":   ns.CreatedAt,
	})
}

// ListNamespaces handles GET /api/v1/secrets/namespaces
// Auth: AuthMiddleware (JWT)
func (h *SecretsHandler) ListNamespaces(c fiber.Ctx) error {
	agentIDStr := c.Query("agentId")
	if agentIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "agentId query parameter required"})
	}

	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid agentId format"})
	}

	// SECURITY: agentId arrives as a QUERY PARAMETER and was previously used without any
	// org check — this handler did not even read the caller's organization. Any
	// authenticated user could list another organization's agent's secret namespaces by
	// naming its UUID.
	//
	// The tenant-scope lint watches c.Params, not c.Query, which is why this route did not
	// trip it. That is selection bias in the lint, not evidence the handler was safe.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if LoadOwned(c, h.agentRepo.GetByID, agentID, orgID, agentOrgID) == nil {
		return nil
	}

	namespaces, err := h.secretsService.ListNamespaces(agentID)
	if err != nil {
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"namespaces": namespaces})
}

// GetNamespace handles GET /api/v1/secrets/namespaces/:id
// Auth: AuthMiddleware (JWT)
func (h *SecretsHandler) GetNamespace(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid namespace ID"})
	}

	// SECURITY (A3d-iii): verify the namespace's owning agent belongs to
	// the caller's org before returning namespace metadata. Returns 404
	// for any cross-tenant or non-existent ID (existence secrecy).
	ns := LoadOwnedViaAgent(c, h.secretsService.GetNamespace, id, orgID,
		func(n *secrets.SecretNamespace) uuid.UUID { return n.AgentID },
		h.agentRepo)
	if ns == nil {
		return nil
	}

	return c.Status(fiber.StatusOK).JSON(ns)
}

// DeleteNamespace handles DELETE /api/v1/secrets/namespaces/:id
// Auth: AuthMiddleware + MemberMiddleware (JWT)
func (h *SecretsHandler) DeleteNamespace(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid namespace ID"})
	}

	// SECURITY (A3d-iii): verify the namespace's owning agent belongs to
	// the caller's org before deletion. Returns 404 for any cross-tenant
	// or non-existent ID (existence secrecy + delete-protection).
	ns := LoadOwnedViaAgent(c, h.secretsService.GetNamespace, id, orgID,
		func(n *secrets.SecretNamespace) uuid.UUID { return n.AgentID },
		h.agentRepo)
	if ns == nil {
		return nil
	}

	if err := h.secretsService.DeleteNamespace(id); err != nil {
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}
	h.recordSecretsChange(c, orgID, domain.AuditActionDelete, secretNamespaceResource, ns)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Namespace deleted"})
}

// StoreCredential handles POST /api/v1/secrets/namespaces/:id/credentials
// Auth: AuthMiddleware + MemberMiddleware (JWT)
func (h *SecretsHandler) StoreCredential(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	nsID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid namespace ID"})
	}

	var req storeCredentialRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.EncryptedBlob == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "encryptedBlob is required"})
	}

	blob, err := base64.StdEncoding.DecodeString(req.EncryptedBlob)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid encryptedBlob encoding"})
	}

	alg := req.EncryptionAlg
	if alg == "" {
		alg = "X25519-ChaCha20-Poly1305"
	}

	// SECURITY (A3d-iii): verify the namespace's owning agent belongs to
	// the caller's org before writing credential material. Without this
	// check, a caller in org A could plant credentials inside a namespace
	// owned by an agent in org B. Returns 404 on cross-tenant.
	ns := LoadOwnedViaAgent(c, h.secretsService.GetNamespace, nsID, orgID,
		func(n *secrets.SecretNamespace) uuid.UUID { return n.AgentID },
		h.agentRepo)
	if ns == nil {
		return nil
	}

	if err := h.secretsService.StoreCredential(nsID, blob, alg); err != nil {
		if errors.Is(err, application.ErrCredentialBlobTooLarge) {
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"error": credentialTooLargeLine})
		}
		if errors.Is(err, secrets.ErrBackendReadOnly) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": readOnlyBackendLine})
		}
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}
	h.recordSecretsChange(c, orgID, domain.AuditActionCreate, secretCredentialResource, ns)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "Credential stored"})
}

// RotateCredential handles POST /api/v1/secrets/namespaces/:id/rotate
// Auth: AuthMiddleware + MemberMiddleware (JWT)
func (h *SecretsHandler) RotateCredential(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	// A revoked session cannot rotate a credential with its still-valid access token.
	if !refuseIfFamilyRevoked(c, h.jwtService, h.auditService, "secret_rotate") {
		return nil
	}

	nsID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid namespace ID"})
	}

	var req rotateCredentialRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.EncryptedBlob == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "encryptedBlob is required"})
	}

	blob, err := base64.StdEncoding.DecodeString(req.EncryptedBlob)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid encryptedBlob encoding"})
	}

	alg := req.EncryptionAlg
	if alg == "" {
		alg = "X25519-ChaCha20-Poly1305"
	}

	// SECURITY (A3d-iii): verify the namespace's owning agent belongs to
	// the caller's org before rotating credentials. Without this check, a
	// caller in org A could overwrite credentials inside a namespace
	// owned by an agent in org B. Returns 404 on cross-tenant.
	ns := LoadOwnedViaAgent(c, h.secretsService.GetNamespace, nsID, orgID,
		func(n *secrets.SecretNamespace) uuid.UUID { return n.AgentID },
		h.agentRepo)
	if ns == nil {
		return nil
	}

	if err := h.secretsService.RotateCredential(nsID, blob, alg); err != nil {
		if errors.Is(err, application.ErrCredentialBlobTooLarge) {
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"error": credentialTooLargeLine})
		}
		if errors.Is(err, secrets.ErrBackendReadOnly) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": readOnlyBackendLine})
		}
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}
	h.recordSecretsChange(c, orgID, domain.AuditActionUpdate, secretCredentialResource, ns)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Credential rotated"})
}

// The resource types of the audit entries the secrets management routes write.
const (
	secretNamespaceResource  = "secret_namespace"
	secretCredentialResource = "secret_credential"
)

// recordSecretsChange writes the audit entry of a secrets management request
// that succeeded: in the caller's organization, under the caller's user id,
// with the namespace as its resource. The metadata names the namespace's
// backend and nothing else, never the credential, its length or its
// encryption parameters. The change has been made, so a failed write is
// logged and the answer is unchanged.
func (h *SecretsHandler) recordSecretsChange(c fiber.Ctx, orgID uuid.UUID, action domain.AuditAction, resourceType string, ns *secrets.SecretNamespace) {
	if h.auditService == nil {
		return
	}
	entry := &domain.AuditLog{
		OrganizationID: orgID,
		Action:         action,
		ResourceType:   resourceType,
		ResourceID:     ns.ID,
		IPAddress:      c.IP(),
		UserAgent:      c.Get("User-Agent"),
		Metadata:       map[string]interface{}{"backendType": string(ns.BackendType)},
	}
	if userID, err := getUserID(c); err == nil && userID != uuid.Nil {
		entry.UserID = &userID
	}
	if err := h.auditService.Log(c.Context(), entry); err != nil {
		log.Printf("secrets: audit entry for %s %s %s not written: %v", action, resourceType, ns.ID, err)
	}
}

// GetAuditLog handles GET /api/v1/secrets/audit
// Auth: AuthMiddleware (JWT)
// Filters: since, an RFC 3339 time, and limit and offset, integers of 0 or
// more. A filter the route cannot read is refused with 400 and an error line
// that names it; an absent or empty one reads as its default.
func (h *SecretsHandler) GetAuditLog(c fiber.Ctx) error {
	agentIDStr := c.Query("agentId")
	if agentIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "agentId query parameter required"})
	}

	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid agentId format"})
	}

	// The filters are read before the agent is, so a request the route cannot
	// read is refused with the parameter named and nothing is looked up for it.
	var since *time.Time
	if sinceStr := c.Query("since"); sinceStr != "" {
		t, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": auditSinceLine})
		}
		since = &t
	}
	limit, ok := auditCountParam(c, "limit", 50)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": auditCountLine("limit")})
	}
	offset, ok := auditCountParam(c, "offset", 0)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": auditCountLine("offset")})
	}

	// SECURITY: same defect as ListNamespaces — agentId from a query parameter, used with
	// no org check. This one returns the secrets AUDIT LOG for that agent, so it disclosed
	// another organization's credential-access history.
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	if LoadOwned(c, h.agentRepo.GetByID, agentID, orgID, agentOrgID) == nil {
		return nil
	}

	entries, err := h.secretsService.GetAuditLog(agentID, since, limit, offset)
	if err != nil {
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"entries": entries})
}

// auditSinceLine is the answer, with 400, to a since filter of
// GET /api/v1/secrets/audit that is not an RFC 3339 time.
const auditSinceLine = "since must be an RFC 3339 time such as 2026-10-17T09:30:00Z"

// auditCountLine is the answer, with 400, to the limit or offset filter of
// GET /api/v1/secrets/audit named name when it is not an integer of 0 or more.
func auditCountLine(name string) string {
	return name + " must be an integer of 0 or more"
}

// auditCountParam reads the query parameter name of GET /api/v1/secrets/audit
// as an integer of 0 or more. An absent or empty value reads as fallback; any
// other value that is not an integer of 0 or more is not ok, and the route
// answers it with auditCountLine.
func auditCountParam(c fiber.Ctx, name string, fallback int) (int, bool) {
	raw := c.Query(name)
	if raw == "" {
		return fallback, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// getUserID extracts the user ID from Fiber context (set by AuthMiddleware).
func getUserID(c fiber.Ctx) (uuid.UUID, error) {
	userIDValue := c.Locals("user_id")
	if userIDValue == nil {
		return uuid.Nil, fiber.ErrUnauthorized
	}
	switch v := userIDValue.(type) {
	case uuid.UUID:
		return v, nil
	case string:
		return uuid.Parse(v)
	default:
		return uuid.Nil, fiber.ErrUnauthorized
	}
}

// credentialTooLargeLine states the size rule an encryptedBlob breaks.
var credentialTooLargeLine = fmt.Sprintf("encryptedBlob must decode to at most %d bytes", application.MaxCredentialBlobSize)

// readOnlyBackendLine answers a store or a rotation in a namespace whose
// backend AIM reads and never writes, such as cyberark_ccp.
const readOnlyBackendLine = "the namespace's backend is read-only: store and rotate its credential in the backend"

// resolveRefusalLine is the answer to a refused resolve request. A refusal
// the caller can correct names the member and the rule; any other refusal is
// one line, and its reason is in the secrets audit entry and the server log.
func resolveRefusalLine(err error) string {
	switch {
	case errors.Is(err, application.ErrResolveKeyNotRegistered):
		return "agentPublicKey is not the registered key of this agent; omit it or send the registered key"
	case errors.Is(err, application.ErrResolveNonceMalformed):
		return "nonce must be <RFC 3339 time>:<random part>"
	case errors.Is(err, application.ErrResolveNonceOutsideWindow):
		return "nonce time is outside the accepted window; sign the request again with a fresh nonce"
	case errors.Is(err, application.ErrResolveNonceAlreadyUsed):
		return "nonce has already been used; sign the request again with a fresh nonce"
	}
	return "Secret resolution refused"
}
