package handlers

import (
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/secrets"
)

type SecretsHandler struct {
	secretsService *application.SecretsService
}

func NewSecretsHandler(secretsService *application.SecretsService) *SecretsHandler {
	return &SecretsHandler{secretsService: secretsService}
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
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
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

	ns := &secrets.SecretNamespace{
		AgentID:     req.AgentID,
		Namespace:   req.Namespace,
		BackendType: secrets.BackendType(req.BackendType),
		Operations:  req.Operations,
		URLPatterns: req.URLPatterns,
		CreatedBy:   userID,
	}

	if err := h.secretsService.CreateNamespace(ns); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

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

	namespaces, err := h.secretsService.ListNamespaces(agentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"namespaces": namespaces})
}

// GetNamespace handles GET /api/v1/secrets/namespaces/:id
// Auth: AuthMiddleware (JWT)
func (h *SecretsHandler) GetNamespace(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid namespace ID"})
	}

	ns, err := h.secretsService.GetNamespace(id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if ns == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Namespace not found"})
	}

	return c.Status(fiber.StatusOK).JSON(ns)
}

// DeleteNamespace handles DELETE /api/v1/secrets/namespaces/:id
// Auth: AuthMiddleware + MemberMiddleware (JWT)
func (h *SecretsHandler) DeleteNamespace(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid namespace ID"})
	}

	if err := h.secretsService.DeleteNamespace(id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Namespace deleted"})
}

// StoreCredential handles POST /api/v1/secrets/namespaces/:id/credentials
// Auth: AuthMiddleware + MemberMiddleware (JWT)
func (h *SecretsHandler) StoreCredential(c fiber.Ctx) error {
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

	if err := h.secretsService.StoreCredential(nsID, blob, alg); err != nil {
		if strings.Contains(err.Error(), "exceeds maximum size") {
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "Credential stored"})
}

// RotateCredential handles POST /api/v1/secrets/namespaces/:id/rotate
// Auth: AuthMiddleware + MemberMiddleware (JWT)
func (h *SecretsHandler) RotateCredential(c fiber.Ctx) error {
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

	if err := h.secretsService.RotateCredential(nsID, blob, alg); err != nil {
		if strings.Contains(err.Error(), "exceeds maximum size") {
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Credential rotated"})
}

// GetAuditLog handles GET /api/v1/secrets/audit
// Auth: AuthMiddleware (JWT)
func (h *SecretsHandler) GetAuditLog(c fiber.Ctx) error {
	agentIDStr := c.Query("agentId")
	if agentIDStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "agentId query parameter required"})
	}

	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid agentId format"})
	}

	var since *time.Time
	if sinceStr := c.Query("since"); sinceStr != "" {
		t, err := time.Parse(time.RFC3339, sinceStr)
		if err == nil {
			since = &t
		}
	}

	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	entries, err := h.secretsService.GetAuditLog(agentID, since, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"entries": entries})
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
