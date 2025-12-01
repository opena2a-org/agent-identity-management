package handlers

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

// VerificationHandler handles agent action verification requests
type VerificationHandler struct {
	agentService             *application.AgentService
	auditService             *application.AuditService
	alertService             *application.AlertService
	trustService             *application.TrustCalculator
	verificationEventService *application.VerificationEventService
}

// NewVerificationHandler creates a new verification handler
func NewVerificationHandler(
	agentService *application.AgentService,
	auditService *application.AuditService,
	alertService *application.AlertService,
	trustService *application.TrustCalculator,
	verificationEventService *application.VerificationEventService,
) *VerificationHandler {
	return &VerificationHandler{
		agentService:             agentService,
		auditService:             auditService,
		alertService:             alertService,
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
	ID           string    `json:"id"`
	Status       string    `json:"status"` // "approved", "denied", "pending"
	ApprovedBy   string    `json:"approved_by,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	DenialReason string    `json:"denial_reason,omitempty"`
	TrustScore   float64   `json:"trust_score"`
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
	publicKeyMatched := agent.PublicKey != nil && *agent.PublicKey == req.PublicKey
	if !publicKeyMatched {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Public key mismatch",
		})
	}

	// Verify signature
	signatureVerified := false
	if err := h.verifySignature(req); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": fmt.Sprintf("Signature verification failed: %v", err),
		})
	}
	signatureVerified = true

	// Calculate trust score for this action
	trustScore := h.calculateActionTrustScore(agent, req.ActionType, req.Resource)

	// ============================================================================
	// ✅ CRITICAL: Use AgentService.VerifyAction for PROPER capability-based access control
	// This enforces the security policies configured in the dashboard
	// The old determineVerificationStatus() only checked trust scores, not capabilities!
	// ============================================================================
	allowed, denialReason, auditIDFromVerify, err := h.agentService.VerifyAction(
		c.Context(),
		agentID,
		req.ActionType,
		req.Resource,
		req.Context,
	)

	var status string
	if err != nil {
		// Error during verification - deny by default for security
		fmt.Printf("⚠️  VerifyAction error: %v\n", err)
		status = "denied"
		if denialReason == "" {
			denialReason = fmt.Sprintf("Verification error: %v", err)
		}
	} else if allowed {
		status = "approved"
	} else {
		status = "denied"
	}
	_ = auditIDFromVerify // Audit ID already created by VerifyAction

	// Create verification ID
	verificationID := uuid.New()

	// ✅ CHECK FOR CAPABILITY VIOLATIONS - Create alert based on risk level
	// Low-risk actions: No alerts needed, just tracking (better UX for demos)
	// Medium/High-risk actions: Alert if action is denied or lacks capability
	shouldCreateAlert := false
	hasCapability, err := h.agentService.HasCapability(c.Context(), agentID, req.ActionType, req.Resource)
	if err != nil {
		fmt.Printf("⚠️  Error checking capability: %v\n", err)
	} else if !hasCapability {
		// Determine if this action warrants an alert based on risk level and approval status
		// Low-risk actions approved by trust score: No alert (good UX for demos)
		// Medium-risk actions without capability: Alert only if denied
		// High-risk actions without capability: Always alert
		isLowRisk := isLowRiskAction(req.ActionType)
		isDenied := status == "denied"

		if isDenied {
			// Always alert on denied actions
			shouldCreateAlert = true
			fmt.Printf("🚨 DENIED ACTION: Agent %s denied for action: %s\n", agent.Name, req.ActionType)
		} else if !isLowRisk && req.RiskLevel != "low" {
			// Alert for medium/high risk actions without capability (even if approved)
			shouldCreateAlert = true
			fmt.Printf("🚨 CAPABILITY VIOLATION: Agent %s attempting %s-risk action without capability: %s\n", agent.Name, req.RiskLevel, req.ActionType)
		} else {
			// Low-risk approved actions: Just log, no alert
			fmt.Printf("📝 TRACKED: Agent %s performed low-risk action: %s (approved by trust score)\n", agent.Name, req.ActionType)
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
			"trustScore":      trustScore,
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
		// Determine alert type and messaging based on action type
		var alertTitle, alertDescription string
		var alertType domain.AlertType
		var severity domain.AlertSeverity

		if isDemoHighRiskAction(req.ActionType) {
			// Demo high-risk actions get informational monitoring alerts (not scary breach alerts)
			alertType = domain.AlertUnusualActivity // Info-level, not breach
			severity = domain.AlertSeverityInfo
			alertTitle = fmt.Sprintf("High-Risk Action Monitored: %s", agent.Name)
			alertDescription = fmt.Sprintf(
				"Agent '%s' performed high-risk action '%s' on resource '%s'. "+
					"This action was approved (trust score: %.2f) and logged for monitoring. "+
					"Verification ID: %s. Consider granting explicit capability for production use.",
				agent.Name, req.ActionType, req.Resource,
				trustScore, verificationID.String(),
			)
		} else {
			// Real security concern - create breach alert
			alertType = domain.AlertSecurityBreach
			severity = h.determineAlertSeverity(req.ActionType, req.Context, req.RiskLevel)
			alertTitle = fmt.Sprintf("Unauthorized Action Detected: %s", agent.Name)
			alertDescription = fmt.Sprintf(
				"Agent '%s' (ID: %s) attempted unauthorized action '%s' on resource '%s' without proper capability. "+
					"This action was logged but allowed for monitoring purposes. "+
					"Trust Score: %.2f. Verification ID: %s",
				agent.Name, agent.ID.String(), req.ActionType, req.Resource,
				trustScore, verificationID.String(),
			)
		}

		alert := &domain.Alert{
			ID:             uuid.New(),
			OrganizationID: agent.OrganizationID,
			AlertType:      alertType,
			Severity:       severity,
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
			fmt.Printf("✅ Security alert created (severity: %s): %s\n", severity, alert.ID.String())
		}

		// NOTE: Violation record is already created by VerifyAction() with correct is_blocked and severity
		// Do NOT create a duplicate here - it was causing dashboard to show incorrect data
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
		"trustScore":      trustScore,
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

	// Calculate confidence score based on verification factors
	confidence := h.calculateVerificationConfidence(agent, status, signatureVerified, publicKeyMatched)

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
		Confidence:       confidence,
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
		// Use the actual database ID from the created event
		verificationID = event.ID
	}

	// ============================================================================
	// UNUSUAL ACCESS PATTERN DETECTION
	// Run anomaly detection after each verification to catch suspicious behavior
	// ============================================================================
	if h.alertService != nil {
		// Capture values needed for async operation - don't use c.Context() in goroutine
		// because the request context becomes invalid after the response is sent
		orgID := agent.OrganizationID
		agentIDCopy := agentID
		go func() {
			// Run async to not slow down verification response
			// Use background context since request context may be cancelled
			ctx := context.Background()
			_, err := h.alertService.DetectUnusualAccessPatterns(ctx, orgID, agentIDCopy)
			if err != nil {
				fmt.Printf("⚠️ Unusual access pattern detection failed: %v\n", err)
			}
		}()
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

// customJSONFormat adds spaces after colons and commas to match Python's json.dumps format
// This only adds spaces outside of string values to avoid changing string content
func customJSONFormat(jsonStr string) string {
	var result strings.Builder
	inString := false
	escape := false

	for i, char := range jsonStr {
		result.WriteRune(char)

		if escape {
			escape = false
			continue
		}

		if char == '\\' {
			escape = true
			continue
		}

		if char == '"' {
			inString = !inString
			continue
		}

		if !inString {
			// Add space after : if not already there
			if char == ':' && i+1 < len(jsonStr) && jsonStr[i+1] != ' ' {
				result.WriteRune(' ')
			}
			// Add space after , if not already there
			if char == ',' && i+1 < len(jsonStr) && jsonStr[i+1] != ' ' {
				result.WriteRune(' ')
			}
		}
	}

	return result.String()
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
	if len(req.Context) > 0 {
		signaturePayload["context"] = req.Context
	} else {
		signaturePayload["context"] = make(map[string]interface{})
	}

	// Handle resource carefully - Python SDK uses null, not empty string
	if req.Resource == "" {
		signaturePayload["resource"] = nil // Match Python's null
	} else {
		signaturePayload["resource"] = req.Resource
	}
	signaturePayload["timestamp"] = req.Timestamp

	// DEBUG: risk_level is NEVER sent as separate field by SDK - it's inside context
	// Don't include it in signature payload unless SDK changes
	// if req.RiskLevel != "" {
	// 	signaturePayload["risk_level"] = req.RiskLevel
	// }

	// Create deterministic JSON matching Python's json.dumps(sort_keys=True, separators=(', ', ': '))
	// Python's separators=(', ', ': ') adds space after comma and colon
	// Use json.MarshalIndent with SetIndent("", " ") BUT that doesn't work for spaces
	// Instead, use json.Marshal and then use a proper JSON formatter

	jsonBytes, err := json.Marshal(signaturePayload)
	if err != nil {
		return fmt.Errorf("failed to marshal signature payload: %w", err)
	}

	// Parse back and re-encode with proper spacing
	var parsed interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		return fmt.Errorf("failed to unmarshal for formatting: %w", err)
	}

	// Use custom encoder to match Python's format exactly
	buffer := new(bytes.Buffer)
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "")

	if err := encoder.Encode(parsed); err != nil {
		return fmt.Errorf("failed to encode with formatting: %w", err)
	}

	// Remove trailing newline
	messageBytes := bytes.TrimRight(buffer.Bytes(), "\n")

	// Manually add spaces to match Python's separators=(', ', ': ')
	// This is the ONLY reliable way to match Python's exact format
	messageStr := customJSONFormat(string(messageBytes))
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
		"read_database": 1.0,
		"read_file":     1.0,
		"query_api":     1.0,
		// Medium risk (modifications)
		"write_database": 0.8,
		"write_file":     0.8,
		"send_email":     0.8,
		"modify_config":  0.7,
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

// isLowRiskAction checks if an action is considered low-risk (read-only, informational)
// Low-risk actions don't generate security alerts when approved by trust score alone
func isLowRiskAction(actionType string) bool {
	lowRiskActions := map[string]bool{
		// Read-only database operations
		"read_database": true,
		"read_file":     true,
		"query_api":     true,
		// Demo-friendly actions (low and medium risk from demo_agent.py)
		"check_weather":    true,
		"search_products":  true,
		"get_user_profile": true, // Medium in demo but really just a read
		"query_orders":     true, // Medium in demo but really just a read
		// General read operations
		"fetch_data": true,
		"list_items": true,
		"get_status": true,
		"search":     true,
		"lookup":     true,
		"view":       true,
		"read":       true,
	}
	return lowRiskActions[actionType]
}

// isDemoHighRiskAction checks if this is a high-risk demo action that should
// generate informational alerts (not security breach alerts) to demonstrate monitoring
func isDemoHighRiskAction(actionType string) bool {
	demoHighRisk := map[string]bool{
		"send_notification": true,
		"process_refund":    true,
	}
	return demoHighRisk[actionType]
}

func normalizeVerificationStatus(status domain.VerificationEventStatus) string {
	switch status {
	case domain.VerificationEventStatusPending:
		return "pending"
	case domain.VerificationEventStatusSuccess:
		return "approved"
	case domain.VerificationEventStatusFailed:
		return "denied"
	default:
		return strings.ToLower(string(status))
	}
}

// determineVerificationStatus determines if action should be auto-approved
func (h *VerificationHandler) determineVerificationStatus(
	agent *domain.Agent,
	actionType string,
	trustScore float64,
) (status string, denialReason string) {
	// Minimum trust score thresholds
	const (
		MinTrustForLowRisk    = 0.3 // 30%
		MinTrustForMediumRisk = 0.5 // 50%
		MinTrustForHighRisk   = 0.7 // 70%
		MinTrustForCritical   = 0.9 // 90% - Critical actions require very high trust OR manual approval
	)

	// Define critical actions that ALWAYS require manual approval regardless of trust score
	criticalActions := map[string]bool{
		"delete_production_data": true,
		"drop_database":          true,
		"execute_shell_command":  true,
		"access_sensitive_data":  true,
		"modify_security_policy": true,
		"grant_admin_access":     true,
		"revoke_all_permissions": true,
		"export_all_data":        true,
		"system_shutdown":        true,
		"modify_authentication":  true,
	}

	// Define high-risk actions that require approval below certain trust thresholds
	highRiskActions := map[string]bool{
		"delete_data":        true,
		"delete_file":        true,
		"execute_command":    true,
		"admin_action":       true,
		"modify_permissions": true,
		"create_admin_user":  true,
		"access_audit_logs":  true,
		"modify_config":      true,
	}

	// ============================================================================
	// CRITICAL ACTION BLOCKING
	// These actions ALWAYS require manual admin approval regardless of trust score
	// ============================================================================
	if criticalActions[actionType] {
		fmt.Printf("🔴 CRITICAL ACTION BLOCKED: %s requires manual admin approval\n", actionType)
		return "pending", fmt.Sprintf("Critical action '%s' requires manual admin approval", actionType)
	}

	// ============================================================================
	// HIGH-RISK ACTION EVALUATION
	// High-risk actions with low trust score require manual approval
	// ============================================================================
	if highRiskActions[actionType] && trustScore < MinTrustForCritical {
		if trustScore < MinTrustForHighRisk {
			// Very low trust - deny outright
			return "denied", fmt.Sprintf("Trust score %.2f below required %.2f for high-risk action %s", trustScore, MinTrustForHighRisk, actionType)
		}
		// Medium-high trust but not high enough for auto-approval - require manual review
		fmt.Printf("⚠️ HIGH-RISK ACTION PENDING: %s with trust score %.2f requires review\n", actionType, trustScore)
		return "pending", fmt.Sprintf("High-risk action '%s' with trust score %.2f requires admin review (auto-approve threshold: %.2f)", actionType, trustScore, MinTrustForCritical)
	}

	// ============================================================================
	// STANDARD ACTION EVALUATION
	// Normal actions evaluated purely on trust score thresholds
	// ============================================================================
	var requiredTrust float64
	switch actionType {
	// Low-risk actions: read-only, informational, no side effects
	case "read_database", "read_file", "query_api",
		"check_weather", "search_products", "get_user_profile", "query_orders",
		"fetch_data", "list_items", "get_status", "search", "lookup":
		requiredTrust = MinTrustForLowRisk
	// Demo high-risk actions: Allow at medium threshold for demo UX
	// These create info alerts but are auto-approved to show monitoring value
	case "send_notification", "process_refund":
		requiredTrust = MinTrustForMediumRisk
	// High-risk actions: destructive or privileged operations
	case "delete_data", "delete_file", "execute_command", "admin_action":
		requiredTrust = MinTrustForHighRisk
	// Medium-risk: default for unknown actions that may have side effects
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
	// 📍 LOG 2: HANDLER ENTRY
	// requestID := c.Locals("request_id")
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

// determineAlertSeverity determines the alert severity based on action type and context
func (h *VerificationHandler) determineAlertSeverity(actionType string, context map[string]interface{}, riskLevel string) domain.AlertSeverity {
	// 1. Check explicit risk_level from context or request
	if riskLevel != "" {
		switch strings.ToLower(riskLevel) {
		case "critical":
			return domain.AlertSeverityCritical
		case "high":
			return domain.AlertSeverityHigh
		case "medium", "warning":
			return domain.AlertSeverityWarning
		case "low", "info":
			return domain.AlertSeverityInfo
		}
	}

	// Check context for risk_level
	if context != nil {
		if contextRiskLevel, ok := context["risk_level"].(string); ok {
			switch strings.ToLower(contextRiskLevel) {
			case "critical":
				return domain.AlertSeverityCritical
			case "high":
				return domain.AlertSeverityHigh
			case "medium", "warning":
				return domain.AlertSeverityWarning
			case "low", "info":
				return domain.AlertSeverityInfo
			}
		}
	}

	// 2. Determine severity based on action type patterns
	actionLower := strings.ToLower(actionType)

	// CRITICAL: Destructive operations, system access, credential operations
	criticalPatterns := []string{
		"delete", "drop", "truncate", "destroy", "remove",
		"admin", "root", "sudo", "execute", "exec", "run",
		"credential", "password", "secret", "key", "token",
		"privilege", "permission", "grant", "revoke",
		"system", "kernel", "process",
	}
	for _, pattern := range criticalPatterns {
		if strings.Contains(actionLower, pattern) {
			return domain.AlertSeverityCritical
		}
	}

	// HIGH: Write operations, modifications, sensitive data access
	highPatterns := []string{
		"write", "update", "modify", "edit", "change", "alter",
		"create", "insert", "add", "post", "put", "patch",
		"payment", "transaction", "financial", "billing",
		"user", "account", "profile",
		"config", "setting", "configuration",
	}
	for _, pattern := range highPatterns {
		if strings.Contains(actionLower, pattern) {
			return domain.AlertSeverityHigh
		}
	}

	// WARNING: Read operations on sensitive data
	warningPatterns := []string{
		"read", "get", "fetch", "retrieve", "query", "search",
		"list", "view", "show", "display",
		"download", "export",
	}
	for _, pattern := range warningPatterns {
		if strings.Contains(actionLower, pattern) {
			return domain.AlertSeverityWarning
		}
	}

	// INFO: Everything else (monitoring, logging, etc.)
	return domain.AlertSeverityInfo
}

// calculateVerificationConfidence calculates confidence score for verification events
// Returns a value between 0.0 and 1.0 based on multiple factors
func (h *VerificationHandler) calculateVerificationConfidence(agent *domain.Agent, status string, signatureVerified bool, publicKeyMatched bool) float64 {
	confidence := 0.0

	// Base confidence from successful verification steps (60% of total)
	if signatureVerified {
		confidence += 0.30 // Cryptographic signature verified
	}
	if publicKeyMatched {
		confidence += 0.30 // Public key matches registered key
	}

	// Agent status contributes to confidence (20% of total)
	switch agent.Status {
	case domain.AgentStatusVerified:
		confidence += 0.20 // Fully verified agent
	case domain.AgentStatusPending:
		confidence += 0.10 // Pending verification
	default:
		confidence += 0.0 // Suspended/revoked agents get no confidence bonus
	}

	// Trust score contributes to confidence (20% of total)
	// Trust score ranges from 0-100, normalize to 0-0.20
	trustScoreContribution := (agent.TrustScore / 100.0) * 0.20
	confidence += trustScoreContribution

	// Final verification status can reduce confidence
	if status == "denied" {
		confidence *= 0.5 // Reduce confidence by half for denied requests
	}

	// Ensure confidence is between 0.0 and 1.0
	if confidence < 0.0 {
		confidence = 0.0
	}
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// ============================================================================
// ADMIN VERIFICATION APPROVAL ENDPOINTS
// These enable the require_approval decorator in the SDK
// ============================================================================

// PendingVerificationResponse represents a pending verification for admin review
type PendingVerificationResponse struct {
	ID          string                 `json:"id"`
	AgentID     string                 `json:"agent_id"`
	AgentName   string                 `json:"agent_name"`
	ActionType  string                 `json:"action_type"`
	Resource    string                 `json:"resource"`
	Context     map[string]interface{} `json:"context"`
	RiskLevel   string                 `json:"risk_level"`
	TrustScore  float64                `json:"trust_score"`
	Status      string                 `json:"status"`
	RequestedAt time.Time              `json:"requested_at"`
	ExpiresAt   time.Time              `json:"expires_at"`
}

type PendingVerificationListResponse struct {
	Verifications []PendingVerificationResponse `json:"verifications"`
	Pagination    struct {
		Page       int `json:"page"`
		PageSize   int `json:"page_size"`
		Total      int `json:"total"`
		TotalPages int `json:"total_pages"`
	} `json:"pagination"`
	StatusCounts domain.VerificationStatusCounts `json:"status_counts"`
}

// ListPendingVerifications returns all pending verifications awaiting admin approval
// @Summary List pending verifications
// @Description Get all verification requests awaiting admin approval
// @Tags admin,verifications
// @Produce json
// @Success 200 {array} PendingVerificationResponse "List of pending verifications"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/admin/verifications/pending [get]
func (h *VerificationHandler) ListPendingVerifications(c fiber.Ctx) error {
	// Get organization ID from context (set by auth middleware)
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization context not found",
		})
	}

	pageParam := c.Query("page", "1")
	pageSizeParam := c.Query("page_size", "10")
	page, err := strconv.Atoi(pageParam)
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(pageSizeParam)
	if err != nil || pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	status := strings.ToLower(c.Query("status", "pending"))
	risk := strings.ToLower(c.Query("risk", "all"))
	search := c.Query("search", "")
	searchField := strings.ToLower(c.Query("search_field", "all"))

	params := domain.VerificationQueryParams{
		Status:      status,
		RiskLevel:   risk,
		Search:      search,
		SearchField: searchField,
		Limit:       pageSize,
		Offset:      (page - 1) * pageSize,
	}

	events, total, counts, err := h.verificationEventService.SearchVerifications(c.Context(), orgID, params)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to get verification requests: %v", err),
		})
	}

	totalPages := 1
	if pageSize > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(pageSize)))
		if totalPages == 0 {
			totalPages = 1
		}
	}

	var responseItems []PendingVerificationResponse
	for _, event := range events {
		agentName := ""
		if event.InitiatorName != nil {
			agentName = *event.InitiatorName
		}

		actionType := ""
		if event.Action != nil {
			actionType = *event.Action
		}

		resource := ""
		if event.ResourceType != nil {
			resource = *event.ResourceType
		}

		riskLevel := "medium"
		if event.Metadata != nil {
			if rl, ok := event.Metadata["risk_level"].(string); ok {
				riskLevel = rl
			}
			// Also check inside context
			if ctx, ok := event.Metadata["context"].(map[string]interface{}); ok {
				if rl, ok := ctx["risk_level"].(string); ok {
					riskLevel = rl
				}
			}
		}

		agentIDStr := ""
		if event.AgentID != nil {
			agentIDStr = event.AgentID.String()
		}

		responseItems = append(responseItems, PendingVerificationResponse{
			ID:          event.ID.String(),
			AgentID:     agentIDStr,
			AgentName:   agentName,
			ActionType:  actionType,
			Resource:    resource,
			Context:     event.Metadata,
			RiskLevel:   riskLevel,
			TrustScore:  event.TrustScore,
			Status:      normalizeVerificationStatus(event.Status),
			RequestedAt: event.CreatedAt,
			ExpiresAt:   event.CreatedAt.Add(1 * time.Hour), // Default 1 hour expiry
		})
	}

	var statusCounts domain.VerificationStatusCounts
	if counts != nil {
		statusCounts = *counts
	}

	payload := PendingVerificationListResponse{
		Verifications: responseItems,
		StatusCounts:  statusCounts,
	}
	payload.Pagination.Page = page
	payload.Pagination.PageSize = pageSize
	payload.Pagination.Total = total
	payload.Pagination.TotalPages = totalPages

	return c.Status(fiber.StatusOK).JSON(payload)
}

// ApproveVerificationRequest represents the request body for approving a verification
type ApproveVerificationRequest struct {
	Reason string `json:"reason,omitempty"`
}

// ApproveVerification approves a pending verification request
// @Summary Approve verification
// @Description Approve a pending verification request, allowing the agent action to proceed
// @Tags admin,verifications
// @Accept json
// @Produce json
// @Param id path string true "Verification ID (UUID)"
// @Param request body ApproveVerificationRequest false "Approval details"
// @Success 200 {object} map[string]interface{} "Verification approved"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 404 {object} ErrorResponse "Verification not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/admin/verifications/{id}/approve [post]
func (h *VerificationHandler) ApproveVerification(c fiber.Ctx) error {
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

	// Parse optional request body
	var req ApproveVerificationRequest
	_ = c.Bind().JSON(&req) // Ignore error - body is optional

	// Get admin user info from context
	userID, _ := c.Locals("user_id").(uuid.UUID)
	userName := "admin"
	if name, ok := c.Locals("user_name").(string); ok {
		userName = name
	}

	// Update verification to approved status
	result := domain.VerificationResultVerified
	metadata := map[string]interface{}{
		"approved_by":     userName,
		"approved_by_id":  userID.String(),
		"approved_at":     time.Now().Format(time.RFC3339),
		"approval_reason": req.Reason,
		"manual_approval": true,
	}

	err = h.verificationEventService.UpdateVerificationResult(c.Context(), vid, result, nil, metadata)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Verification not found or update failed",
		})
	}

	// Create audit log
	orgID, _ := c.Locals("organization_id").(uuid.UUID)
	auditEntry := &domain.AuditLog{
		ID:             uuid.New(),
		OrganizationID: orgID,
		UserID:         userID,
		Action:         domain.AuditActionUpdate,
		ResourceType:   "verification",
		ResourceID:     vid,
		IPAddress:      c.IP(),
		UserAgent:      c.Get("User-Agent"),
		Metadata: map[string]interface{}{
			"action":          "approve_verification",
			"verification_id": vid.String(),
			"approval_reason": req.Reason,
		},
		Timestamp: time.Now(),
	}
	_ = h.auditService.Log(c.Context(), auditEntry)

	fmt.Printf("✅ Verification %s APPROVED by %s\n", vid.String(), userName)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"id":          vid.String(),
		"status":      "approved",
		"approved_by": userName,
		"approved_at": time.Now().Format(time.RFC3339),
		"message":     "Verification approved - agent action can now proceed",
	})
}

// DenyVerificationRequest represents the request body for denying a verification
type DenyVerificationRequest struct {
	Reason string `json:"reason" validate:"required"`
}

// DenyVerification denies a pending verification request
// @Summary Deny verification
// @Description Deny a pending verification request, blocking the agent action
// @Tags admin,verifications
// @Accept json
// @Produce json
// @Param id path string true "Verification ID (UUID)"
// @Param request body DenyVerificationRequest true "Denial details"
// @Success 200 {object} map[string]interface{} "Verification denied"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 404 {object} ErrorResponse "Verification not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /api/v1/admin/verifications/{id}/deny [post]
func (h *VerificationHandler) DenyVerification(c fiber.Ctx) error {
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

	// Parse request body
	var req DenyVerificationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Reason == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "reason is required when denying a verification",
		})
	}

	// Get admin user info from context
	userID, _ := c.Locals("user_id").(uuid.UUID)
	userName := "admin"
	if name, ok := c.Locals("user_name").(string); ok {
		userName = name
	}

	// Update verification to denied status
	result := domain.VerificationResultDenied
	metadata := map[string]interface{}{
		"denied_by":     userName,
		"denied_by_id":  userID.String(),
		"denied_at":     time.Now().Format(time.RFC3339),
		"denial_reason": req.Reason,
		"manual_denial": true,
	}

	err = h.verificationEventService.UpdateVerificationResult(c.Context(), vid, result, &req.Reason, metadata)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Verification not found or update failed",
		})
	}

	// Create audit log
	orgID, _ := c.Locals("organization_id").(uuid.UUID)
	auditEntry := &domain.AuditLog{
		ID:             uuid.New(),
		OrganizationID: orgID,
		UserID:         userID,
		Action:         domain.AuditActionUpdate,
		ResourceType:   "verification",
		ResourceID:     vid,
		IPAddress:      c.IP(),
		UserAgent:      c.Get("User-Agent"),
		Metadata: map[string]interface{}{
			"action":          "deny_verification",
			"verification_id": vid.String(),
			"denial_reason":   req.Reason,
		},
		Timestamp: time.Now(),
	}
	_ = h.auditService.Log(c.Context(), auditEntry)

	fmt.Printf("❌ Verification %s DENIED by %s: %s\n", vid.String(), userName, req.Reason)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"id":            vid.String(),
		"status":        "denied",
		"denied_by":     userName,
		"denied_at":     time.Now().Format(time.RFC3339),
		"denial_reason": req.Reason,
		"message":       "Verification denied - agent action blocked",
	})
}
