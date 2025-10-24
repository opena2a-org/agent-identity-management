package handlers

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

// VerificationHandler handles agent action verification requests
type VerificationHandler struct {
	agentService              *application.AgentService
	auditService              *application.AuditService
	trustService              *application.TrustCalculator
	verificationEventService  *application.VerificationEventService
}

// NewVerificationHandler creates a new verification handler
func NewVerificationHandler(
	agentService *application.AgentService,
	auditService *application.AuditService,
	trustService *application.TrustCalculator,
	verificationEventService *application.VerificationEventService,
) *VerificationHandler {
	return &VerificationHandler{
		agentService:             agentService,
		auditService:             auditService,
		trustService:             trustService,
		verificationEventService: verificationEventService,
	}
}

// VerificationRequest represents an action verification request from an agent
type VerificationRequest struct {
	AgentID    string                 `json:"agent_id" validate:"required"`
	ActionType string                 `json:"action_type" validate:"required"`
	Resource   string                 `json:"resource"`
	Context    map[string]interface{} `json:"context"`
	Timestamp  string                 `json:"timestamp" validate:"required"`
	RiskLevel  string                 `json:"risk_level,omitempty"` // Optional risk assessment
	Signature  string                 `json:"signature" validate:"required"`
	PublicKey  string                 `json:"public_key" validate:"required"`
}

// VerificationResponse represents the verification result
type VerificationResponse struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"` // "approved", "denied", "pending"
	ApprovedBy  string    `json:"approved_by,omitempty"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
	DenialReason string   `json:"denial_reason,omitempty"`
	TrustScore  float64   `json:"trust_score"`
}

// CreateVerification handles POST /api/v1/verifications
// @Summary Request verification for an agent action
// @Description Verify agent identity and approve/deny action based on trust score
// @Tags verifications
// @Accept json
// @Produce json
// @Param request body VerificationRequest true "Verification request"
// @Success 201 {object} VerificationResponse "Verification created"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Invalid signature"
// @Failure 403 {object} ErrorResponse "Action denied"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/verifications [post]
func (h *VerificationHandler) CreateVerification(c fiber.Ctx) error {
	var req VerificationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.AgentID == "" || req.ActionType == "" || req.Signature == "" || req.PublicKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "agent_id, action_type, signature, and public_key are required",
		})
	}

	// Parse agent ID
	agentID, err := uuid.Parse(req.AgentID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent_id format",
		})
	}

	// Get agent from database
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}

	// Verify agent is active
	if agent.Status != domain.AgentStatusVerified && agent.Status != domain.AgentStatusPending {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": fmt.Sprintf("Agent status is %s, cannot perform actions", agent.Status),
		})
	}

	// Verify public key matches
	if agent.PublicKey == nil || *agent.PublicKey != req.PublicKey {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Public key mismatch",
		})
	}

	// Verify signature
	if err := h.verifySignature(req); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fmt.Sprintf("Signature verification failed: %v", err),
		})
	}

	// Calculate trust score for this action
	trustScore := h.calculateActionTrustScore(agent, req.ActionType, req.Resource)

	// Determine auto-approval based on trust score and action type
	status, denialReason := h.determineVerificationStatus(agent, req.ActionType, trustScore)

	// Create verification ID
	verificationID := uuid.New()

	// ✅ CHECK FOR CAPABILITY VIOLATIONS - Create alert if agent doesn't have permission
	shouldCreateAlert := false
	if status == "approved" {
		// Check if agent has the capability for this action
		hasCapability, err := h.agentService.HasCapability(c.Context(), agentID, req.ActionType, req.Resource)
		if err != nil {
			fmt.Printf("⚠️  Error checking capability: %v\n", err)
		} else if !hasCapability {
			// Agent is attempting an action without proper capability - CREATE ALERT
			shouldCreateAlert = true
			fmt.Printf("🚨 CAPABILITY VIOLATION: Agent %s attempting unauthorized action: %s\n", agent.Name, req.ActionType)
		}
	}

	// Create audit log entry
	auditEntry := &domain.AuditLog{
		ID:             uuid.New(),
		OrganizationID: agent.OrganizationID,
		UserID:         agent.CreatedBy, // Creator of the agent
		Action:         domain.AuditAction(req.ActionType),
		ResourceType:   "agent_action",
		ResourceID:     agentID,
		IPAddress:      c.IP(),
		UserAgent:      c.Get("User-Agent"),
		Metadata: map[string]interface{}{
			"verification_id": verificationID.String(),
			"trust_score":     trustScore,
			"auto_approved":   status == "approved",
			"action_type":     req.ActionType,
			"resource":        req.Resource,
			"context":         req.Context,
		},
		Timestamp: time.Now(),
	}

	if status == "denied" {
		auditEntry.Metadata["denial_reason"] = denialReason
	}

	// Save audit log
	if err := h.auditService.Log(c.Context(), auditEntry); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to create audit log: %v\n", err)
	}

	// ✅ CREATE SECURITY ALERT if capability violation detected
	if shouldCreateAlert {
		alertTitle := fmt.Sprintf("Unauthorized Action Detected: %s", agent.Name)
		alertDescription := fmt.Sprintf(
			"Agent '%s' (ID: %s) attempted unauthorized action '%s' on resource '%s' without proper capability. "+
			"This action was logged but allowed for monitoring purposes. "+
			"Trust Score: %.2f. Verification ID: %s",
			agent.Name, agent.ID.String(), req.ActionType, req.Resource,
			trustScore, verificationID.String(),
		)

		alert := &domain.Alert{
			ID:             uuid.New(),
			OrganizationID: agent.OrganizationID,
			AlertType:      domain.AlertSecurityBreach,
			Severity:       domain.AlertSeverityHigh,
			Title:          alertTitle,
			Description:    alertDescription,
			ResourceType:   "agent",
			ResourceID:     agentID,
			IsAcknowledged: false,
			CreatedAt:      time.Now(),
		}

		// Save alert to database using AgentService's alert repository
		if err := h.agentService.CreateSecurityAlert(c.Context(), alert); err != nil {
			fmt.Printf("⚠️  Warning: failed to create security alert: %v\n", err)
		} else {
			fmt.Printf("✅ Security alert created: %s\n", alert.ID.String())
		}
	}

	// ✅ Create verification event for dashboard visibility
	startTime := time.Now()
	verificationDurationMs := 10 // Estimate: signature verification + trust calculation

	// Determine verification protocol based on action type
	protocol := domain.VerificationProtocolA2A // Default to A2A (Agent-to-Agent)
	if strings.Contains(req.ActionType, "mcp") || strings.Contains(req.ActionType, "azure_openai") {
		protocol = domain.VerificationProtocolMCP
	}

	// Determine verification type
	verificationType := domain.VerificationTypeIdentity // Default to identity verification
	if strings.Contains(req.ActionType, "capability") {
		verificationType = domain.VerificationTypeCapability
	} else if strings.Contains(req.ActionType, "permission") {
		verificationType = domain.VerificationTypePermission
	}

	// Map status to verification event status
	var eventStatus domain.VerificationEventStatus
	var result *domain.VerificationResult
	if status == "approved" {
		eventStatus = domain.VerificationEventStatusSuccess
		verifiedResult := domain.VerificationResultVerified
		result = &verifiedResult
	} else if status == "denied" {
		eventStatus = domain.VerificationEventStatusFailed
		deniedResult := domain.VerificationResultDenied
		result = &deniedResult
	} else {
		eventStatus = domain.VerificationEventStatusPending
	}

	// Create verification event metadata
	eventMetadata := map[string]interface{}{
		"verification_id": verificationID.String(),
		"action_type":     req.ActionType,
		"resource":        req.Resource,
		"context":         req.Context,
		"trust_score":     trustScore,
		"auto_approved":   status == "approved",
	}
	if status == "denied" {
		eventMetadata["denial_reason"] = denialReason
	}

	// Create verification event using service
	var errorReasonPtr *string
	if status == "denied" {
		errorReasonPtr = &denialReason
	}

	completedAt := startTime
	verificationEventReq := &application.CreateVerificationEventRequest{
		OrganizationID:   agent.OrganizationID,
		AgentID:          agentID,
		Protocol:         protocol,
		VerificationType: verificationType,
		Status:           eventStatus,
		Result:           result,
		Signature:        &req.Signature,
		PublicKey:        &req.PublicKey,
		DurationMs:       verificationDurationMs,
		ErrorReason:      errorReasonPtr,
		InitiatorType:    domain.InitiatorTypeAgent,
		InitiatorID:      &agentID,
		InitiatorName:    &agent.DisplayName,
		Action:           &req.ActionType,
		ResourceType:     &req.Resource,
		StartedAt:        startTime.Add(-time.Duration(verificationDurationMs) * time.Millisecond),
		CompletedAt:      &completedAt,
		Metadata:         eventMetadata,
	}

	// Save verification event using service
	event, err := h.verificationEventService.CreateVerificationEvent(c.Context(), verificationEventReq)
	if err != nil {
		// Log error but don't fail the request
		fmt.Printf("❌ Failed to create verification event: %v\n", err)
	} else {
		fmt.Printf("✅ Verification event created: ID=%s, OrgID=%s, AgentID=%s\n",
			event.ID, event.OrganizationID, *event.AgentID)
	}

	// Build response
	response := VerificationResponse{
		ID:         verificationID.String(),
		Status:     status,
		TrustScore: trustScore,
	}

	if status == "approved" {
		response.ApprovedBy = "system" // Auto-approved
		response.ExpiresAt = time.Now().Add(24 * time.Hour)
	} else if status == "denied" {
		response.DenialReason = denialReason
	}

	statusCode := fiber.StatusCreated
	if status == "denied" {
		statusCode = fiber.StatusForbidden
	}

	return c.Status(statusCode).JSON(response)
}

// verifySignature verifies the Ed25519 signature
func (h *VerificationHandler) verifySignature(req VerificationRequest) error {
	// Recreate the signature message (same as SDK)
	// MUST use same approach as Python SDK: json.dumps(sort_keys=True)

	// Build payload in Go map (will be sorted by json.Marshal)
	signaturePayload := make(map[string]interface{})
	signaturePayload["action_type"] = req.ActionType
	signaturePayload["agent_id"] = req.AgentID

	// Handle context carefully
	if req.Context != nil && len(req.Context) > 0 {
		signaturePayload["context"] = req.Context
	} else {
		signaturePayload["context"] = make(map[string]interface{})
	}

	signaturePayload["resource"] = req.Resource
	signaturePayload["timestamp"] = req.Timestamp

	// Include risk_level if provided (must match SDK signature)
	if req.RiskLevel != "" {
		signaturePayload["risk_level"] = req.RiskLevel
	}

	// Create deterministic JSON matching Python's json.dumps(sort_keys=True)
	// Python adds spaces after colons and commas, Go doesn't by default
	// We need to use MarshalIndent with empty prefix and indent to get spaces
	buffer := new(bytes.Buffer)
	encoder := json.NewEncoder(buffer)
	encoder.SetIndent("", "")  // No indentation, but spaces after colons/commas
	encoder.SetEscapeHTML(false)  // Don't escape HTML characters

	if err := encoder.Encode(signaturePayload); err != nil {
		return fmt.Errorf("failed to marshal signature payload: %w", err)
	}

	// Remove the trailing newline that Encode() adds
	messageBytes := bytes.TrimRight(buffer.Bytes(), "\n")

	// Still doesn't match - Python uses "key": "value" (space after colon)
	// Go uses "key":"value" (no space)
	// Let's manually add spaces after colons
	messageStr := string(messageBytes)
	// Replace ": with ": " (add space after colon before value)
	messageStr = strings.ReplaceAll(messageStr, "\":", "\": ")
	messageStr = strings.ReplaceAll(messageStr, ",", ", ")
	messageBytes = []byte(messageStr)

	// Decode public key
	publicKeyBytes, err := base64.StdEncoding.DecodeString(req.PublicKey)
	if err != nil {
		return fmt.Errorf("invalid public key encoding: %w", err)
	}

	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key size: expected %d, got %d", ed25519.PublicKeySize, len(publicKeyBytes))
	}

	// Decode signature
	signatureBytes, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		return fmt.Errorf("invalid signature encoding: %w", err)
	}

	// Verify signature
	publicKey := ed25519.PublicKey(publicKeyBytes)
	if !ed25519.Verify(publicKey, messageBytes, signatureBytes) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// calculateActionTrustScore calculates trust score for specific action
func (h *VerificationHandler) calculateActionTrustScore(agent *domain.Agent, actionType, resource string) float64 {
	// Start with agent's base trust score
	score := agent.TrustScore

	// Adjust based on action type (high-risk actions reduce effective trust)
	riskAdjustment := h.getActionRiskAdjustment(actionType)
	score = score * riskAdjustment

	return score
}

// getActionRiskAdjustment returns multiplier based on action risk
func (h *VerificationHandler) getActionRiskAdjustment(actionType string) float64 {
	riskLevels := map[string]float64{
		// Low risk (read-only)
		"read_database":   1.0,
		"read_file":       1.0,
		"query_api":       1.0,
		// Medium risk (modifications)
		"write_database":  0.8,
		"write_file":      0.8,
		"send_email":      0.8,
		"modify_config":   0.7,
		// High risk (destructive)
		"delete_data":     0.5,
		"delete_file":     0.5,
		"execute_command": 0.3,
		"admin_action":    0.3,
	}

	if adjustment, ok := riskLevels[actionType]; ok {
		return adjustment
	}

	// Default: medium risk
	return 0.8
}

// determineVerificationStatus determines if action should be auto-approved
func (h *VerificationHandler) determineVerificationStatus(
	agent *domain.Agent,
	actionType string,
	trustScore float64,
) (status string, denialReason string) {
	// Minimum trust score thresholds
	const (
		MinTrustForLowRisk    = 0.3  // 30%
		MinTrustForMediumRisk = 0.5  // 50%
		MinTrustForHighRisk   = 0.7  // 70%
	)

	// Determine required trust based on action type
	var requiredTrust float64
	switch actionType {
	case "read_database", "read_file", "query_api":
		requiredTrust = MinTrustForLowRisk
	case "delete_data", "delete_file", "execute_command", "admin_action":
		requiredTrust = MinTrustForHighRisk
	default:
		requiredTrust = MinTrustForMediumRisk
	}

	// Check if trust score meets requirement
	if trustScore < requiredTrust {
		return "denied", fmt.Sprintf("Trust score %.2f below required %.2f for action %s", trustScore, requiredTrust, actionType)
	}

	// Auto-approve
	return "approved", ""
}

// GetVerification retrieves verification status by ID
// @Summary Get verification status
// @Description Retrieve the status of a verification request by ID
// @Tags verifications
// @Produce json
// @Param id path string true "Verification ID (UUID)"
// @Success 200 {object} VerificationResponse "Verification found"
// @Failure 400 {object} ErrorResponse "Invalid verification ID"
// @Failure 404 {object} ErrorResponse "Verification not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/verifications/{id} [get]
func (h *VerificationHandler) GetVerification(c fiber.Ctx) error {
	verificationID := c.Params("id")
	if verificationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "verification_id is required",
		})
	}

	// Parse UUID
	vid, err := uuid.Parse(verificationID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid verification_id format",
		})
	}

	// Query verification event from database
	event, err := h.verificationEventService.GetVerificationEvent(c.Context(), vid)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Verification not found or expired",
		})
	}

	// Build response
	response := VerificationResponse{
		ID:         event.ID.String(),
		TrustScore: event.TrustScore,
	}

	// Map event status and result to verification status
	if event.Result != nil {
		switch *event.Result {
		case domain.VerificationResultVerified:
			response.Status = "approved"
			response.ApprovedBy = "system"
			response.ExpiresAt = event.CreatedAt.Add(24 * time.Hour)
		case domain.VerificationResultDenied:
			response.Status = "denied"
			if event.ErrorReason != nil {
				response.DenialReason = *event.ErrorReason
			}
		case domain.VerificationResultExpired:
			response.Status = "expired"
		default:
			response.Status = "pending"
		}
	} else {
		response.Status = "pending"
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

// SubmitVerificationResult handles verification result submission
// @Summary Submit verification result
// @Description Submit the result of a verification request (success/failure)
// @Tags verifications
// @Accept json
// @Produce json
// @Param id path string true "Verification ID (UUID)"
// @Param result body object true "Verification result"
// @Success 200 {object} map[string]interface{} "Result recorded"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 404 {object} ErrorResponse "Verification not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/verifications/{id}/result [post]
func (h *VerificationHandler) SubmitVerificationResult(c fiber.Ctx) error {
	verificationID := c.Params("id")
	if verificationID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "verification_id is required",
		})
	}

	var req struct {
		Result   string                 `json:"result"` // "success", "failure"
		Reason   string                 `json:"reason,omitempty"`
		Metadata map[string]interface{} `json:"metadata,omitempty"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Parse UUID
	vid, err := uuid.Parse(verificationID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid verification_id format",
		})
	}

	// Validate result value
	if req.Result != "success" && req.Result != "failure" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "result must be either 'success' or 'failure'",
		})
	}

	// Map result string to VerificationResult type
	var result domain.VerificationResult
	if req.Result == "success" {
		result = domain.VerificationResultVerified
	} else {
		result = domain.VerificationResultDenied
	}

	// Prepare reason pointer
	var reasonPtr *string
	if req.Reason != "" {
		reasonPtr = &req.Reason
	}

	// Update verification event in database
	err = h.verificationEventService.UpdateVerificationResult(c.Context(), vid, result, reasonPtr, req.Metadata)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Verification not found or update failed",
		})
	}

	// Return success response
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"id":     vid.String(),
		"status": "result_recorded",
		"result": req.Result,
	})
}
