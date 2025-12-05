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

// VerificationRequest represents a capability verification request from an agent
type VerificationRequest struct {
	AgentID    string                 `json:"agentId" validate:"required"`
	Capability string                 `json:"capability" validate:"required"`
	Resource   string                 `json:"resource"`
	Context    map[string]interface{} `json:"context"`
	Timestamp  string                 `json:"timestamp" validate:"required"`
	RiskLevel  string                 `json:"riskLevel,omitempty"` // Optional risk assessment
	Signature  string                 `json:"signature" validate:"required"`
	PublicKey  string                 `json:"publicKey" validate:"required"`
}

// VerificationResponse represents the verification result
type VerificationResponse struct {
	ID           string    `json:"id"`
	Status       string    `json:"status"` // "approved", "denied", "pending"
	ApprovedBy   string    `json:"approvedBy,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
	DenialReason string    `json:"denialReason,omitempty"`
	TrustScore   float64   `json:"trustScore"`
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
	if req.AgentID == "" || req.Capability == "" || req.Signature == "" || req.PublicKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "agentId, capability, signature, and publicKey are required",
		})
	}

	// Parse agent ID
	agentID, err := uuid.Parse(req.AgentID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agentId format",
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

	// Use agent's base trust score for display/storage (consistency across app)
	// The risk-adjusted calculation is used internally for security decisions
	trustScore := agent.TrustScore / 100.0 // Normalize to 0-1 range for frontend display

	// ============================================================================
	// ✅ CRITICAL: Use AgentService.VerifyCapability for PROPER capability-based access control
	// This enforces the security policies configured in the dashboard
	// The old determineVerificationStatus() only checked trust scores, not capabilities!
	// ============================================================================
	allowed, denialReason, auditIDFromVerify, err := h.agentService.VerifyCapability(
		c.Context(),
		agentID,
		req.Capability,
		req.Resource,
		req.Context,
		c.IP(), // Capture source IP for security tracking
	)

	// Check if this is a JIT (Just-In-Time) access request that needs admin approval
	isJITRequest := false
	if jitAccess, ok := req.Context["jit_access"].(bool); ok && jitAccess {
		isJITRequest = true
	}

	// SECURITY: Debug logging removed to prevent information leakage
	var status string
	if err != nil {
		// Error during verification - deny by default for security
		status = "denied"
		if denialReason == "" {
			denialReason = fmt.Sprintf("Verification error: %v", err)
		}
	} else if allowed {
		status = "approved"
	} else if isJITRequest {
		// JIT access request without capability - create pending record for admin approval
		status = "pending"
	} else {
		status = "denied"
	}
	// Create verification ID
	verificationID := uuid.New()

	// CHECK FOR CAPABILITY VIOLATIONS - Create alert based on risk level
	// Low-risk actions: No alerts needed, just tracking
	// Medium/High-risk actions: Alert if action is denied or lacks capability
	// SECURITY: No debug logging to prevent information leakage
	shouldCreateAlert := false
	hasCapability, err := h.agentService.HasCapability(c.Context(), agentID, req.Capability, req.Resource)
	if err == nil && !hasCapability {
		// Determine if this action warrants an alert based on risk level and approval status
		isLowRisk := isLowRiskCapability(req.Capability)
		isDenied := status == "denied"

		if isDenied {
			// Always alert on denied actions
			shouldCreateAlert = true
		} else if !isLowRisk && req.RiskLevel != "low" {
			// Alert for medium/high risk actions without capability (even if approved)
			shouldCreateAlert = true
		}
		// Low-risk approved actions: No alert, no logging needed
	}

	// Create audit log entry - Use the audit ID from VerifyCapability so alerts can link to it
	auditEntry := &domain.AuditLog{
		ID:             auditIDFromVerify, // Use same ID so alerts link correctly
		OrganizationID: agent.OrganizationID,
		UserID:         agent.CreatedBy, // Creator of the agent
		Action:         domain.AuditAction(req.Capability),
		ResourceType:   "agent_action",
		ResourceID:     agentID,
		IPAddress:      c.IP(),
		UserAgent:      c.Get("User-Agent"),
		Metadata: map[string]interface{}{
			"verificationId": verificationID.String(),
			"trustScore":     trustScore,
			"autoApproved":   status == "approved",
			"actionType":     req.Capability,
			"resource":       req.Resource,
			"context":        req.Context,
		},
		Timestamp: time.Now(),
	}

	if status == "denied" {
		auditEntry.Metadata["denialReason"] = denialReason
	}

	// Save audit log - errors don't block the request
	// SECURITY: No error logging to prevent information leakage
	_ = h.auditService.Log(c.Context(), auditEntry)

	// ✅ CREATE SECURITY ALERT if capability violation detected
	if shouldCreateAlert {
		// Determine alert type and messaging based on action type
		var alertTitle, alertDescription string
		var alertType domain.AlertType
		var severity domain.AlertSeverity

		if isDemoHighRiskCapability(req.Capability) {
			// Demo high-risk actions get informational monitoring alerts (not scary breach alerts)
			alertType = domain.AlertUnusualActivity // Info-level, not breach
			severity = domain.AlertSeverityInfo
			alertTitle = fmt.Sprintf("High-Risk Action Monitored: %s", agent.Name)
			alertDescription = fmt.Sprintf(
				"Agent '%s' performed high-risk action '%s' on resource '%s'. "+
					"This action was approved (trust score: %.2f) and logged for monitoring. "+
					"Consider granting explicit capability for production use.",
				agent.Name, req.Capability, req.Resource, trustScore,
			)
		} else {
			// Real security concern - create breach alert
			alertType = domain.AlertSecurityBreach
			severity = h.determineAlertSeverity(req.Capability, req.Context, req.RiskLevel)
			alertTitle = fmt.Sprintf("Unauthorized Action Detected: %s", agent.Name)
			alertDescription = fmt.Sprintf(
				"Agent '%s' attempted unauthorized action '%s' on resource '%s' without proper capability. "+
					"This action was logged but allowed for monitoring purposes.",
				agent.Name, req.Capability, req.Resource,
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
			AuditID:        &auditEntry.ID, // Link to the audit log entry
			AgentName:      agent.Name,
			Metadata: map[string]interface{}{
				"capability":     req.Capability,
				"resource":       req.Resource,
				"trustScore":     trustScore,
				"verificationId": verificationID.String(),
				"riskLevel":      req.RiskLevel,
				"status":         status,
				"ipAddress":      c.IP(),
			},
			IsAcknowledged: false,
			CreatedAt:      time.Now(),
		}

		// Save alert to database using AgentService's alert repository
		// Create security alert - errors don't block the request
		// SECURITY: No logging to prevent information leakage
		_ = h.agentService.CreateSecurityAlert(c.Context(), alert)

		// NOTE: Violation record is already created by VerifyAction() with correct is_blocked and severity
		// Do NOT create a duplicate here - it was causing dashboard to show incorrect data
	}

	// ✅ Create verification event for dashboard visibility
	startTime := time.Now()
	verificationDurationMs := 10 // Estimate: signature verification + trust calculation

	// Determine verification protocol based on action type
	protocol := domain.VerificationProtocolA2A // Default to A2A (Agent-to-Agent)
	if strings.Contains(req.Capability, "mcp") || strings.Contains(req.Capability, "azure_openai") {
		protocol = domain.VerificationProtocolMCP
	}

	// Determine verification type
	verificationType := domain.VerificationTypeIdentity // Default to identity verification
	if strings.Contains(req.Capability, "capability") {
		verificationType = domain.VerificationTypeCapability
	} else if strings.Contains(req.Capability, "permission") {
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
		"verificationId": verificationID.String(),
		"actionType":     req.Capability,
		"resource":       req.Resource,
		"context":        req.Context,
		"trustScore":     trustScore,
		"autoApproved":   status == "approved",
	}
	if status == "denied" {
		eventMetadata["denialReason"] = denialReason
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
		Action:           &req.Capability,
		ResourceType:     &req.Resource,
		StartedAt:        startTime.Add(-time.Duration(verificationDurationMs) * time.Millisecond),
		CompletedAt:      &completedAt,
		Metadata:         eventMetadata,
	}

	// Save verification event using service
	// SECURITY: No error logging to prevent information leakage
	event, err := h.verificationEventService.CreateVerificationEvent(c.Context(), verificationEventReq)
	if err == nil {
		// Use the actual database ID from the created event
		verificationID = event.ID
	}

	// ============================================================================
	// UNUSUAL ACCESS PATTERN DETECTION
	// Run anomaly detection after each verification to catch suspicious behavior
	// ============================================================================
	// UNUSUAL ACCESS PATTERN DETECTION (async, non-blocking)
	// SECURITY: No error logging to prevent information leakage
	if h.alertService != nil {
		orgID := agent.OrganizationID
		agentIDCopy := agentID
		go func() {
			ctx := context.Background()
			_, _ = h.alertService.DetectUnusualAccessPatterns(ctx, orgID, agentIDCopy)
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
	signaturePayload["action_type"] = req.Capability
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

// calculateCapabilityTrustScore calculates trust score for specific capability
func (h *VerificationHandler) calculateCapabilityTrustScore(agent *domain.Agent, capability, resource string) float64 {
	// Start with agent's base trust score
	score := agent.TrustScore

	// Adjust based on capability (high-risk capabilities reduce effective trust)
	riskAdjustment := h.getCapabilityRiskAdjustment(capability)
	score = score * riskAdjustment

	return score
}

// getCapabilityRiskAdjustment returns multiplier based on capability risk
func (h *VerificationHandler) getCapabilityRiskAdjustment(capability string) float64 {
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

	if adjustment, ok := riskLevels[capability]; ok {
		return adjustment
	}

	// Default: medium risk
	return 0.8
}

// isLowRiskCapability checks if a capability is considered low-risk (read-only, informational)
// Low-risk capabilities don't generate security alerts when approved by trust score alone
func isLowRiskCapability(capability string) bool {
	lowRiskCapabilities := map[string]bool{
		// Read-only database operations
		"db:read":   true,
		"file:read": true,
		"api:read":  true,
		// Demo-friendly capabilities (low and medium risk from demo_agent.py)
		"weather:check":   true,
		"products:search": true,
		"user:read":       true, // Medium in demo but really just a read
		"orders:read":     true, // Medium in demo but really just a read
		// General read operations
		"data:fetch":   true,
		"items:list":   true,
		"status:get":   true,
		"search":       true,
		"lookup":       true,
		"view":         true,
		"read":         true,
		// Legacy format support
		"read_database": true,
		"read_file":     true,
		"query_api":     true,
	}
	return lowRiskCapabilities[capability]
}

// isDemoHighRiskCapability checks if this is a high-risk demo capability that should
// generate informational alerts (not security breach alerts) to demonstrate monitoring
func isDemoHighRiskCapability(capability string) bool {
	demoHighRisk := map[string]bool{
		"notification:send": true,
		"refund:process":    true,
		// Legacy format support
		"send_notification": true,
		"process_refund":    true,
	}
	return demoHighRisk[capability]
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
	// SECURITY: No debug logging to prevent information leakage
	if criticalActions[actionType] {
		return "pending", fmt.Sprintf("Critical action '%s' requires manual admin approval", actionType)
	}

	// ============================================================================
	// HIGH-RISK ACTION EVALUATION
	// High-risk actions with low trust score require manual approval
	// ============================================================================
	// SECURITY: No debug logging to prevent information leakage
	if highRiskActions[actionType] && trustScore < MinTrustForCritical {
		if trustScore < MinTrustForHighRisk {
			// Very low trust - deny outright
			return "denied", fmt.Sprintf("Trust score %.2f below required %.2f for high-risk action %s", trustScore, MinTrustForHighRisk, actionType)
		}
		// Medium-high trust but not high enough for auto-approval - require manual review
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
			"error": "verificationId is required",
		})
	}

	// Parse UUID
	vid, err := uuid.Parse(verificationID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid verificationId format",
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
			"error": "verificationId is required",
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
			"error": "Invalid verificationId format",
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
	AgentID     string                 `json:"agentId"`
	AgentName   string                 `json:"agentName"`
	ActionType  string                 `json:"capability"`
	Resource    string                 `json:"resource"`
	Context     map[string]interface{} `json:"context"`
	RiskLevel   string                 `json:"riskLevel"`
	TrustScore  float64                `json:"trustScore"`
	Status      string                 `json:"status"`
	RequestedAt time.Time              `json:"requestedAt"`
	ExpiresAt   time.Time              `json:"expiresAt"`
}

type PendingVerificationListResponse struct {
	Verifications []PendingVerificationResponse `json:"verifications"`
	Pagination    struct {
		Page       int `json:"page"`
		PageSize   int `json:"pageSize"`
		Total      int `json:"total"`
		TotalPages int `json:"totalPages"`
	} `json:"pagination"`
	StatusCounts domain.VerificationStatusCounts `json:"statusCounts"`
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
		// Get agent name with fallbacks
		agentName := ""
		if event.InitiatorName != nil && *event.InitiatorName != "" {
			agentName = *event.InitiatorName
		} else if event.AgentName != nil && *event.AgentName != "" {
			agentName = *event.AgentName
		}

		// If still no name, try to get from agent repo
		if agentName == "" && event.AgentID != nil {
			if agent, err := h.agentService.GetAgent(c.Context(), *event.AgentID); err == nil {
				if agent.DisplayName != "" {
					agentName = agent.DisplayName
				} else {
					agentName = agent.Name
				}
			}
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

		// Get trust score with fallback to metadata
		trustScore := event.TrustScore
		if trustScore == 0 && event.Metadata != nil {
			if ts, ok := event.Metadata["trustScore"].(float64); ok {
				trustScore = ts
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
			TrustScore:  trustScore, // Use computed trustScore with fallback
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
			"error": "verificationId is required",
		})
	}

	// Parse UUID
	vid, err := uuid.Parse(verificationID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid verificationId format",
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
		"approvedBy":     userName,
		"approvedById":   userID.String(),
		"approvedAt":     time.Now().Format(time.RFC3339),
		"approvalReason": req.Reason,
		"manualApproval": true,
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
			"action":         "approve_verification",
			"verificationId": vid.String(),
			"approvalReason": req.Reason,
		},
		Timestamp: time.Now(),
	}
	_ = h.auditService.Log(c.Context(), auditEntry)

	// SECURITY: No logging of approval events to prevent information leakage

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"id":         vid.String(),
		"status":     "approved",
		"approvedBy": userName,
		"approvedAt": time.Now().Format(time.RFC3339),
		"message":    "Verification approved - agent action can now proceed",
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
			"error": "verificationId is required",
		})
	}

	// Parse UUID
	vid, err := uuid.Parse(verificationID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid verificationId format",
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
		"deniedBy":     userName,
		"deniedById":   userID.String(),
		"deniedAt":     time.Now().Format(time.RFC3339),
		"denialReason": req.Reason,
		"manualDenial": true,
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
			"action":         "deny_verification",
			"verificationId": vid.String(),
			"denialReason":   req.Reason,
		},
		Timestamp: time.Now(),
	}
	_ = h.auditService.Log(c.Context(), auditEntry)

	// SECURITY: No logging of denial events to prevent information leakage

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"id":           vid.String(),
		"status":       "denied",
		"deniedBy":     userName,
		"deniedAt":     time.Now().Format(time.RFC3339),
		"denialReason": req.Reason,
		"message":      "Verification denied - agent action blocked",
	})
}
