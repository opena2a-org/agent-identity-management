package handlers

import (
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

type SecurityPolicyHandler struct {
	policyService *application.SecurityPolicyService
}

func NewSecurityPolicyHandler(policyService *application.SecurityPolicyService) *SecurityPolicyHandler {
	return &SecurityPolicyHandler{
		policyService: policyService,
	}
}

// ListPolicies lists all security policies for the organization (admin only)
func (h *SecurityPolicyHandler) ListPolicies(c fiber.Ctx) error {
	// 🔍 Safe type assertion with error checking
	orgIDValue := c.Locals("organization_id")
	if orgIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID not found in context",
		})
	}

	orgID, ok := orgIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid organization ID type in context",
		})
	}

	// 🔍 DEBUG: Log the organization ID being queried
	fmt.Printf("🔍 DEBUG ListPolicies: Querying policies for org_id=%s\n", orgID)

	policies, err := h.policyService.ListPolicies(c.Context(), orgID)
	if err != nil {
		fmt.Printf("❌ DEBUG ListPolicies: Error fetching policies: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve policies",
		})
	}

	fmt.Printf("✅ DEBUG ListPolicies: Found %d policies for org_id=%s\n", len(policies), orgID)
	return c.JSON(policies)
}

// GetPolicy retrieves a specific security policy by ID (admin only)
func (h *SecurityPolicyHandler) GetPolicy(c fiber.Ctx) error {
	policyID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid policy ID",
		})
	}

	policy, err := h.policyService.GetPolicy(c.Context(), policyID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Policy not found",
		})
	}

	return c.JSON(policy)
}

// CreatePolicyRequest represents request body for creating a policy
type CreatePolicyRequest struct {
	Name              string                   `json:"name" validate:"required"`
	Description       string                   `json:"description"`
	PolicyType        domain.PolicyType        `json:"policyType" validate:"required"`
	EnforcementAction domain.EnforcementAction `json:"enforcementAction" validate:"required"`
	SeverityThreshold domain.AlertSeverity     `json:"severityThreshold" validate:"required"`
	Rules             map[string]interface{}   `json:"rules"`
	AppliesTo         string                   `json:"appliesTo" validate:"required"`
	IsEnabled         bool                     `json:"isEnabled"`
	Priority          int                      `json:"priority" validate:"required"`
}

// CreatePolicy creates a new security policy (admin only)
func (h *SecurityPolicyHandler) CreatePolicy(c fiber.Ctx) error {
	userID := c.Locals("user_id").(uuid.UUID)
	orgID := c.Locals("organization_id").(uuid.UUID)

	var req CreatePolicyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	policy := &domain.SecurityPolicy{
		OrganizationID:    orgID,
		Name:              req.Name,
		Description:       req.Description,
		PolicyType:        req.PolicyType,
		EnforcementAction: req.EnforcementAction,
		SeverityThreshold: req.SeverityThreshold,
		Rules:             req.Rules,
		AppliesTo:         req.AppliesTo,
		IsEnabled:         req.IsEnabled,
		Priority:          req.Priority,
		CreatedBy:         userID,
	}

	if err := h.policyService.CreatePolicy(c.Context(), policy); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create policy",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(policy)
}

// UpdatePolicy updates an existing security policy (admin only)
func (h *SecurityPolicyHandler) UpdatePolicy(c fiber.Ctx) error {
	policyID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid policy ID",
		})
	}

	var req CreatePolicyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	policy, err := h.policyService.GetPolicy(c.Context(), policyID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Policy not found",
		})
	}

	// Update fields
	policy.Name = req.Name
	policy.Description = req.Description
	policy.PolicyType = req.PolicyType
	policy.EnforcementAction = req.EnforcementAction
	policy.SeverityThreshold = req.SeverityThreshold
	policy.Rules = req.Rules
	policy.AppliesTo = req.AppliesTo
	policy.IsEnabled = req.IsEnabled
	policy.Priority = req.Priority

	if err := h.policyService.UpdatePolicy(c.Context(), policy); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update policy",
		})
	}

	return c.JSON(policy)
}

// DeletePolicy deletes a security policy (admin only)
func (h *SecurityPolicyHandler) DeletePolicy(c fiber.Ctx) error {
	policyID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid policy ID",
		})
	}

	if err := h.policyService.DeletePolicy(c.Context(), policyID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete policy",
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// TogglePolicyRequest represents request body for toggling a policy
type TogglePolicyRequest struct {
	IsEnabled bool `json:"isEnabled"`
}

// TogglePolicy enables or disables a security policy (admin only)
func (h *SecurityPolicyHandler) TogglePolicy(c fiber.Ctx) error {
	policyID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid policy ID",
		})
	}

	var req TogglePolicyRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.IsEnabled {
		if err := h.policyService.EnablePolicy(c.Context(), policyID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to enable policy",
			})
		}
	} else {
		if err := h.policyService.DisablePolicy(c.Context(), policyID); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to disable policy",
			})
		}
	}

	policy, _ := h.policyService.GetPolicy(c.Context(), policyID)
	return c.JSON(policy)
}
