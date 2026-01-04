package handlers

import (
	"encoding/base64"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// ========================================
// Post-Quantum Cryptography (PQC) Endpoints
// ========================================

// RegisterPQCKeyRequest represents a request to register a PQC public key
type RegisterPQCKeyRequest struct {
	PublicKey    string `json:"publicKey"`    // Base64-encoded ML-DSA public key
	Algorithm    string `json:"algorithm"`    // ML-DSA-44, ML-DSA-65, ML-DSA-87
	EnableHybrid bool   `json:"enableHybrid"` // Enable hybrid mode (Ed25519 + ML-DSA)
}

// RotatePQCKeyRequest represents a request to rotate a PQC key
type RotatePQCKeyRequest struct {
	NewPublicKey string `json:"newPublicKey"` // Base64-encoded new ML-DSA public key
	Algorithm    string `json:"algorithm"`    // ML-DSA variant (must match existing or be an upgrade)
}

// EnableHybridModeRequest represents a request to enable hybrid mode
type EnableHybridModeRequest struct {
	Enable bool `json:"enable"` // Enable or disable hybrid mode
}

// RegisterPQCKey registers a post-quantum cryptographic public key for an agent
// @Summary Register PQC public key
// @Description Register an ML-DSA post-quantum public key for an agent. Supports ML-DSA-44, ML-DSA-65, and ML-DSA-87.
// @Tags agents,pqc
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param request body RegisterPQCKeyRequest true "PQC key registration request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Failure 403 {object} ErrorResponse "Access denied"
// @Router /agents/{id}/pqc-key [post]
func (h *AgentHandler) RegisterPQCKey(c fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}
	userID, _ := c.Locals("user_id").(uuid.UUID)

	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	var req RegisterPQCKeyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate algorithm
	alg := pqc.Algorithm(req.Algorithm)
	if !pqc.IsPQCAlgorithm(alg) && !pqc.IsHybridAlgorithm(alg) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "Invalid PQC algorithm",
			"supportedValues":   []string{"ML-DSA-44", "ML-DSA-65", "ML-DSA-87"},
			"recommendedValue":  "ML-DSA-65",
		})
	}

	// Normalize to pure ML-DSA algorithm for storage
	pureAlg := pqc.GetPureAlgorithm(alg)

	// Validate public key format and size
	publicKeyBytes, err := base64.StdEncoding.DecodeString(req.PublicKey)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid public key format: must be base64 encoded",
		})
	}

	expectedSize, err := pqc.GetExpectedPublicKeySize(pureAlg)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if len(publicKeyBytes) != expectedSize {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":        "Invalid public key size",
			"expectedSize": expectedSize,
			"actualSize":   len(publicKeyBytes),
		})
	}

	// Load agent and verify ownership
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}
	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Check if agent already has a PQC key
	if agent.PQCPublicKey != nil && *agent.PQCPublicKey != "" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error":   "Agent already has a PQC key registered",
			"hint":    "Use PUT /agents/{id}/pqc-key to rotate the existing key",
		})
	}

	// Update agent with PQC key
	now := time.Now()
	algStr := string(pureAlg)

	if err := h.agentService.UpdateAgentPQCKey(c.Context(), agentID, req.PublicKey, algStr, req.EnableHybrid); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to register PQC key",
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionCreate,
		"agent_pqc_key",
		agentID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"agentName":   agent.Name,
			"algorithm":   algStr,
			"hybridMode":  req.EnableHybrid,
		},
	)

	return c.JSON(fiber.Map{
		"agentId":           agentID.String(),
		"pqcKeyAlgorithm":   algStr,
		"hybridModeEnabled": agent.HybridModeEnabled,
		"pqcKeyCreatedAt":   now,
		"message":           "PQC key registered successfully",
	})
}

// RotatePQCKey rotates an agent's post-quantum cryptographic key
// @Summary Rotate PQC key
// @Description Rotate an agent's ML-DSA post-quantum public key. The previous key is kept for a grace period.
// @Tags agents,pqc
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param request body RotatePQCKeyRequest true "PQC key rotation request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 404 {object} ErrorResponse "Agent not found or no PQC key to rotate"
// @Failure 403 {object} ErrorResponse "Access denied"
// @Router /agents/{id}/pqc-key [put]
func (h *AgentHandler) RotatePQCKey(c fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}
	userID, _ := c.Locals("user_id").(uuid.UUID)

	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	var req RotatePQCKeyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate algorithm
	alg := pqc.Algorithm(req.Algorithm)
	if !pqc.IsPQCAlgorithm(alg) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":            "Invalid PQC algorithm",
			"supportedValues":  []string{"ML-DSA-44", "ML-DSA-65", "ML-DSA-87"},
		})
	}

	// Validate public key format and size
	publicKeyBytes, err := base64.StdEncoding.DecodeString(req.NewPublicKey)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid public key format: must be base64 encoded",
		})
	}

	expectedSize, err := pqc.GetExpectedPublicKeySize(alg)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
	if len(publicKeyBytes) != expectedSize {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":        "Invalid public key size",
			"expectedSize": expectedSize,
			"actualSize":   len(publicKeyBytes),
		})
	}

	// Load agent and verify ownership
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}
	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Check if agent has a PQC key to rotate
	if agent.PQCPublicKey == nil || *agent.PQCPublicKey == "" {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent does not have a PQC key to rotate",
			"hint":  "Use POST /agents/{id}/pqc-key to register a new key",
		})
	}

	// Rotate the PQC key
	now := time.Now()
	algStr := string(alg)

	if err := h.agentService.RotateAgentPQCKey(c.Context(), agentID, req.NewPublicKey, algStr); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to rotate PQC key",
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionUpdate,
		"agent_pqc_key_rotation",
		agentID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"agentName":     agent.Name,
			"newAlgorithm":  algStr,
			"rotationCount": agent.RotationCount + 1,
		},
	)

	return c.JSON(fiber.Map{
		"agentId":              agentID.String(),
		"pqcKeyAlgorithm":      algStr,
		"pqcKeyCreatedAt":      now,
		"hasPreviousPqcKey":    true,
		"message":              "PQC key rotated successfully",
	})
}

// SetHybridMode enables or disables hybrid authentication mode for an agent
// @Summary Set hybrid mode
// @Description Enable or disable hybrid authentication mode (Ed25519 + ML-DSA) for an agent.
// @Tags agents,pqc
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param request body EnableHybridModeRequest true "Hybrid mode request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid request or missing PQC key"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Failure 403 {object} ErrorResponse "Access denied"
// @Router /agents/{id}/hybrid-mode [post]
func (h *AgentHandler) SetHybridMode(c fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}
	userID, _ := c.Locals("user_id").(uuid.UUID)

	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	var req EnableHybridModeRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Load agent and verify ownership
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}
	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Validate requirements for enabling hybrid mode
	if req.Enable {
		// Must have Ed25519 key
		if agent.PublicKey == nil || *agent.PublicKey == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Cannot enable hybrid mode: agent does not have an Ed25519 key",
				"hint":  "Register an Ed25519 key first",
			})
		}
		// Must have PQC key
		if agent.PQCPublicKey == nil || *agent.PQCPublicKey == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Cannot enable hybrid mode: agent does not have a PQC key",
				"hint":  "Register a PQC key using POST /agents/{id}/pqc-key first",
			})
		}
	}

	// Update hybrid mode
	if err := h.agentService.SetAgentHybridMode(c.Context(), agentID, req.Enable); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update hybrid mode",
		})
	}

	// Log audit
	action := "enabled"
	if !req.Enable {
		action = "disabled"
	}
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionUpdate,
		"agent_hybrid_mode",
		agentID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"agentName":   agent.Name,
			"hybridMode":  req.Enable,
			"action":      action,
		},
	)

	return c.JSON(fiber.Map{
		"agentId":           agentID.String(),
		"hybridModeEnabled": req.Enable,
		"message":           "Hybrid mode " + action + " successfully",
	})
}

// GetSupportedAlgorithms returns the list of supported cryptographic algorithms
// @Summary List supported algorithms
// @Description Get the list of supported cryptographic algorithms for agent authentication
// @Tags crypto
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /crypto/algorithms [get]
func (h *AgentHandler) GetSupportedAlgorithms(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"classical": []fiber.Map{
			{
				"algorithm":   "Ed25519",
				"type":        "classical",
				"description": "Edwards-curve Digital Signature Algorithm (EdDSA) using Curve25519",
				"keySize":     32,
				"signatureSize": 64,
				"status":      "recommended",
			},
		},
		"postQuantum": []fiber.Map{
			{
				"algorithm":     "ML-DSA-44",
				"type":          "post-quantum",
				"description":   "Module-Lattice Digital Signature Algorithm (NIST FIPS 204) - Security Level 2",
				"publicKeySize": 1312,
				"signatureSize": 2420,
				"securityLevel": 2,
				"status":        "supported",
			},
			{
				"algorithm":     "ML-DSA-65",
				"type":          "post-quantum",
				"description":   "Module-Lattice Digital Signature Algorithm (NIST FIPS 204) - Security Level 3",
				"publicKeySize": 1952,
				"signatureSize": 3309,
				"securityLevel": 3,
				"status":        "recommended",
			},
			{
				"algorithm":     "ML-DSA-87",
				"type":          "post-quantum",
				"description":   "Module-Lattice Digital Signature Algorithm (NIST FIPS 204) - Security Level 5",
				"publicKeySize": 2592,
				"signatureSize": 4627,
				"securityLevel": 5,
				"status":        "supported",
			},
		},
		"hybrid": []fiber.Map{
			{
				"algorithm":   "Ed25519+ML-DSA-44",
				"type":        "hybrid",
				"description": "Hybrid mode combining Ed25519 and ML-DSA-44 for defense in depth",
				"status":      "supported",
			},
			{
				"algorithm":   "Ed25519+ML-DSA-65",
				"type":        "hybrid",
				"description": "Hybrid mode combining Ed25519 and ML-DSA-65 for defense in depth",
				"status":      "recommended",
			},
			{
				"algorithm":   "Ed25519+ML-DSA-87",
				"type":        "hybrid",
				"description": "Hybrid mode combining Ed25519 and ML-DSA-87 for defense in depth",
				"status":      "supported",
			},
		},
		"recommended": fiber.Map{
			"classical":   "Ed25519",
			"postQuantum": "ML-DSA-65",
			"hybrid":      "Ed25519+ML-DSA-65",
		},
	})
}

// GetPQCKeyVault returns an agent's PQC key vault information
// @Summary Get agent PQC key vault
// @Description Get agent's post-quantum cryptographic key vault information
// @Tags agents,pqc
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse "Invalid agent ID"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Failure 403 {object} ErrorResponse "Access denied"
// @Router /agents/{id}/pqc-key [get]
func (h *AgentHandler) GetPQCKeyVault(c fiber.Ctx) error {
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}
	userID, _ := c.Locals("user_id").(uuid.UUID)

	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Load agent and verify ownership
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}
	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionView,
		"agent_pqc_key_vault",
		agentID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"agentName": agent.Name,
		},
	)

	// Determine PQC status
	pqcStatus := "not_configured"
	if agent.PQCPublicKey != nil && *agent.PQCPublicKey != "" {
		if agent.HybridModeEnabled {
			pqcStatus = "hybrid_mode"
		} else {
			pqcStatus = "pqc_only"
		}
	}

	return c.JSON(fiber.Map{
		"agentId":              agentID.String(),
		"agentName":            agent.Name,
		"pqcStatus":            pqcStatus,
		"pqcPublicKey":         agent.PQCPublicKey,
		"pqcKeyAlgorithm":      agent.PQCKeyAlgorithm,
		"hybridModeEnabled":    agent.HybridModeEnabled,
		"pqcKeyCreatedAt":      agent.PQCKeyCreatedAt,
		"pqcKeyExpiresAt":      agent.PQCKeyExpiresAt,
		"hasPreviousPqcKey":    agent.PreviousPQCPublicKey != nil && *agent.PreviousPQCPublicKey != "",
		// Classical key info for reference
		"hasClassicalKey":      agent.PublicKey != nil && *agent.PublicKey != "",
		"classicalKeyAlgorithm": agent.KeyAlgorithm,
	})
}
