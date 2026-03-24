package application

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// VerificationResult represents the result of an action verification
type VerificationResult struct {
	IsValid       bool    `json:"isValid"`
	IsAuthorized  bool    `json:"isAuthorized"`
	InScope       bool    `json:"inScope"`
	TrustScore    float64 `json:"trustScore"`
	ExecutionMode string  `json:"executionMode,omitempty"`
	DenialReason  string  `json:"denialReason,omitempty"`
	Message       string  `json:"message,omitempty"`
}

// CapabilityService handles capability verification and management
type CapabilityService struct {
	capabilityRepo domain.CapabilityRepository
	agentRepo      domain.AgentRepository
	auditRepo      domain.AuditLogRepository
	alertRepo      domain.AlertRepository
	trustCalc      domain.TrustScoreCalculator
	trustScoreRepo domain.TrustScoreRepository
}

// NewCapabilityService creates a new capability service
func NewCapabilityService(
	capabilityRepo domain.CapabilityRepository,
	agentRepo domain.AgentRepository,
	auditRepo domain.AuditLogRepository,
	alertRepo domain.AlertRepository,
	trustCalc domain.TrustScoreCalculator,
	trustScoreRepo domain.TrustScoreRepository,
) *CapabilityService {
	return &CapabilityService{
		capabilityRepo: capabilityRepo,
		agentRepo:      agentRepo,
		auditRepo:      auditRepo,
		alertRepo:      alertRepo,
		trustCalc:      trustCalc,
		trustScoreRepo: trustScoreRepo,
	}
}

// VerifyAction verifies if an agent is authorized to perform a specific action
func (s *CapabilityService) VerifyAction(
	ctx context.Context,
	agentID uuid.UUID,
	requestedCapability string,
	signature []byte,
	payload []byte,
	sourceIP *string,
	metadata map[string]interface{},
) (*VerificationResult, error) {
	// 1. Get agent information
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return &VerificationResult{
			IsValid:      false,
			IsAuthorized: false,
			InScope:      false,
			Message:      "Agent not found",
		}, err
	}

	// 2. Verify signature (identity verification)
	if agent.PublicKey != nil && len(signature) > 0 && len(payload) > 0 {
		valid := s.verifySignature(*agent.PublicKey, agent.KeyAlgorithm, signature, payload)
		if !valid {
			return &VerificationResult{
				IsValid:      false,
				IsAuthorized: false,
				InScope:      false,
				TrustScore:   agent.TrustScore,
				Message:      "Invalid signature",
			}, nil
		}
	}

	// 3. Check if agent has the requested capability
	capabilities, err := s.capabilityRepo.GetActiveCapabilitiesByAgentID(agentID)
	if err != nil {
		return nil, err
	}

	inScope := s.hasCapability(capabilities, requestedCapability)

	if !inScope {
		// 4. Record violation
		violation := &domain.CapabilityViolation{
			AgentID:             agentID,
			AttemptedCapability: requestedCapability,
			RegisteredCapabilities: s.capabilitiesToMap(capabilities),
			Severity:            s.calculateSeverity(agent),
			TrustScoreImpact:    -10,
			IsBlocked:           false,
			SourceIP:            sourceIP,
			RequestMetadata:     metadata,
		}

		if err := s.capabilityRepo.CreateViolation(violation); err != nil {
			return nil, err
		}

		// 5. Decrease trust score
		// IMPORTANT: trust_score is stored as 0.0-1.0 (representing 0-100%), not 0-100
		// Subtract 10% as a decimal (0.10), not as integer 10
		trustScoreDecrease := 0.10 // 10% penalty
		newTrustScore := agent.TrustScore - trustScoreDecrease
		if newTrustScore < 0 {
			newTrustScore = 0
		}

		// Update violation count and trust score
		newViolationCount := agent.CapabilityViolationCount + 1
		if err := s.agentRepo.UpdateTrustScore(agentID, newTrustScore); err != nil {
			return nil, err
		}
		if err := s.agentRepo.IncrementViolationCount(agentID); err != nil {
			return nil, err
		}

		// Check if agent should be marked as compromised
		// IMPORTANT: trust_score is 0.0-1.0 scale, so 30% = 0.30
		if newViolationCount >= 3 || newTrustScore < 0.30 {
			if err := s.agentRepo.MarkAsCompromised(agentID); err != nil {
				return nil, err
			}
		}

		// 6. Log to audit trail
		ipAddr := ""
		if sourceIP != nil {
			ipAddr = *sourceIP
		}
		metadataWithViolation := metadata
		if metadataWithViolation == nil {
			metadataWithViolation = make(map[string]interface{})
		}
		metadataWithViolation["capability"] = requestedCapability
		metadataWithViolation["severity"] = "high"
		metadataWithViolation["description"] = fmt.Sprintf("Agent attempted capability '%s' which is not registered. Trust score decreased by 10 points.", requestedCapability)

		auditLog := &domain.AuditLog{
			OrganizationID: agent.OrganizationID,
			AgentID:        &agentID, // Agent-initiated capability check
			Action:         "capability_violation",
			ResourceType:   "agent",
			ResourceID:     agentID,
			IPAddress:      ipAddr,
			UserAgent:      "",
			Metadata:       metadataWithViolation,
		}

		if err := s.auditRepo.Create(auditLog); err != nil {
			return nil, err
		}

		return &VerificationResult{
			IsValid:      true,
			IsAuthorized: false,
			InScope:      false,
			TrustScore:   newTrustScore,
			Message:      fmt.Sprintf("Action denied: capability '%s' not registered for this agent", requestedCapability),
		}, nil
	}

	// Action is within scope - check per-capability trust threshold
	namespace, action, parseErr := domain.ParseCapability(requestedCapability)
	if parseErr == nil {
		capDef, defErr := s.capabilityRepo.GetCapabilityDefinition(namespace, action, &agent.OrganizationID)
		if defErr == nil && capDef.MinTrustScore > 0 && agent.TrustScore < capDef.MinTrustScore {
			return &VerificationResult{
				IsValid:      true,
				IsAuthorized: false,
				InScope:      true,
				TrustScore:   agent.TrustScore,
				DenialReason: "trust_below_capability_threshold",
				Message:      fmt.Sprintf("Action denied: agent trust score %.0f%% is below the %.0f%% required for capability '%s'", agent.TrustScore*100, capDef.MinTrustScore*100, requestedCapability),
			}, nil
		}
	}

	// Find the matched capability for execution mode check
	matchedCapability := s.findMatchedCapability(capabilities, requestedCapability)
	executionMode := domain.ExecutionModeAuto
	if matchedCapability != nil && matchedCapability.ExecutionMode != "" {
		executionMode = matchedCapability.ExecutionMode
	}

	// Enforce execution mode
	switch executionMode {
	case domain.ExecutionModeNotify:
		// Execute but create an info alert
		if s.alertRepo != nil {
			alert := &domain.Alert{
				OrganizationID: agent.OrganizationID,
				AlertType:      domain.AlertUnusualActivity,
				Severity:       domain.AlertSeverityInfo,
				Title:          fmt.Sprintf("Capability executed (notify mode): %s", requestedCapability),
				Description:    fmt.Sprintf("Agent '%s' executed capability '%s' in notify mode", agent.DisplayName, requestedCapability),
				ResourceType:   "agent",
				ResourceID:     agentID,
				AgentName:      agent.DisplayName,
			}
			if err := s.alertRepo.Create(alert); err != nil {
				fmt.Printf("Warning: failed to create notify alert: %v\n", err)
			}
		}
	case domain.ExecutionModeReview:
		// Deny - requires human approval
		return &VerificationResult{
			IsValid:       true,
			IsAuthorized:  false,
			InScope:       true,
			TrustScore:    agent.TrustScore,
			ExecutionMode: executionMode,
			DenialReason:  "pending_human_review",
			Message:       fmt.Sprintf("Action requires human approval: capability '%s' is set to review mode", requestedCapability),
		}, nil
	}

	// Update last capability check timestamp
	now := time.Now()
	agent.LastCapabilityCheckAt = &now
	if err := s.agentRepo.Update(agent); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Warning: failed to update last_capability_check_at: %v\n", err)
	}

	return &VerificationResult{
		IsValid:       true,
		IsAuthorized:  true,
		InScope:       true,
		TrustScore:    agent.TrustScore,
		ExecutionMode: executionMode,
		Message:       "Action authorized",
	}, nil
}

// findMatchedCapability finds the capability that matches the requested capability type
func (s *CapabilityService) findMatchedCapability(capabilities []*domain.AgentCapability, requestedCapability string) *domain.AgentCapability {
	for _, cap := range capabilities {
		if cap.CapabilityType == requestedCapability {
			return cap
		}
	}
	return nil
}

// GrantCapability grants a new capability to an agent
func (s *CapabilityService) GrantCapability(
	ctx context.Context,
	agentID uuid.UUID,
	capabilityType string,
	scope map[string]interface{},
	grantedBy *uuid.UUID,
	executionMode string,
) (*domain.AgentCapability, error) {
	// Verify agent exists
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	// Default execution mode based on risk level if not specified
	if executionMode == "" {
		executionMode = domain.ExecutionModeAuto
		namespace, action, parseErr := domain.ParseCapability(capabilityType)
		if parseErr == nil {
			capDef, defErr := s.capabilityRepo.GetCapabilityDefinition(namespace, action, &agent.OrganizationID)
			if defErr == nil {
				executionMode = domain.DefaultExecutionModeForRiskLevel(capDef.RiskLevel)
			}
		}
	}

	// Create capability
	capability := &domain.AgentCapability{
		AgentID:         agentID,
		CapabilityType:  capabilityType,
		CapabilityScope: scope,
		ExecutionMode:   executionMode,
		GrantedBy:       grantedBy,
		GrantedAt:       time.Now(),
	}

	if err := s.capabilityRepo.CreateCapability(capability); err != nil {
		return nil, err
	}

	// Log to audit trail
	description := fmt.Sprintf("Capability '%s' granted to agent %s", capabilityType, agent.DisplayName)
	auditLog := &domain.AuditLog{
		OrganizationID: agent.OrganizationID,
		UserID:         grantedBy, // grantedBy is already a *uuid.UUID
		Action:         "capability_granted",
		ResourceType:   "agent",
		ResourceID:     agentID,
		IPAddress:      "",
		UserAgent:      "",
		Metadata: map[string]interface{}{
			"capabilityType": capabilityType,
			"capabilityId":   capability.ID.String(),
			"description":    description,
		},
	}

	if err := s.auditRepo.Create(auditLog); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Warning: failed to create audit log: %v\n", err)
	}

	// Automatically recalculate trust score after capability is granted
	trustScore, err := s.trustCalc.Calculate(agent)
	if err == nil {
		agent.TrustScore = trustScore.Score
		s.agentRepo.Update(agent)
		s.trustScoreRepo.Create(trustScore)
	}

	return capability, nil
}

// RevokeCapability revokes a capability from an agent
func (s *CapabilityService) RevokeCapability(
	ctx context.Context,
	capabilityID uuid.UUID,
	revokedBy *uuid.UUID,
) error {
	// Get capability details before revocation
	capability, err := s.capabilityRepo.GetCapabilityByID(capabilityID)
	if err != nil {
		return fmt.Errorf("capability not found: %w", err)
	}

	// Get agent for audit log
	agent, err := s.agentRepo.GetByID(capability.AgentID)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	// Revoke capability
	if err := s.capabilityRepo.RevokeCapability(capabilityID, time.Now()); err != nil {
		return err
	}

	// Log to audit trail
	description := fmt.Sprintf("Capability '%s' revoked from agent %s", capability.CapabilityType, agent.DisplayName)
	auditLog := &domain.AuditLog{
		OrganizationID: agent.OrganizationID,
		UserID:         revokedBy, // revokedBy is already a *uuid.UUID
		Action:         "capability_revoked",
		ResourceType:   "agent",
		ResourceID:     capability.AgentID,
		IPAddress:      "",
		UserAgent:      "",
		Metadata: map[string]interface{}{
			"capabilityType": capability.CapabilityType,
			"capabilityId":   capabilityID.String(),
			"description":    description,
		},
	}

	if err := s.auditRepo.Create(auditLog); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Warning: failed to create audit log: %v\n", err)
	}

	// Automatically recalculate trust score after capability is revoked
	trustScore, err := s.trustCalc.Calculate(agent)
	if err == nil {
		agent.TrustScore = trustScore.Score
		s.agentRepo.Update(agent)
		s.trustScoreRepo.Create(trustScore)
	}

	return nil
}

// AutoDetectCapabilities attempts to automatically detect and register capabilities for MCP servers
// This is called during MCP registration to capture capabilities without user input
func (s *CapabilityService) AutoDetectCapabilities(
	ctx context.Context,
	agentID uuid.UUID,
	mcpMetadata map[string]interface{},
) error {
	// Extract tools/capabilities from MCP metadata
	// MCP servers typically declare their tools in the registration payload
	if tools, ok := mcpMetadata["tools"].([]interface{}); ok {
		for _, tool := range tools {
			if toolMap, ok := tool.(map[string]interface{}); ok {
				// Extract tool name and convert to capability type
				toolName, _ := toolMap["name"].(string)
				capabilityType := s.mcpToolToCapabilityType(toolName)

				// Create capability with tool metadata as scope
				capability := &domain.AgentCapability{
					AgentID:         agentID,
					CapabilityType:  capabilityType,
					CapabilityScope: toolMap,
					GrantedBy:       nil, // Auto-detected, not manually granted
					GrantedAt:       time.Now(),
				}

				if err := s.capabilityRepo.CreateCapability(capability); err != nil {
					// Log error but continue with other capabilities
					fmt.Printf("Warning: failed to auto-register capability %s: %v\n", capabilityType, err)
				}
			}
		}
	}

	return nil
}

// mcpToolToCapabilityType maps MCP tool names to standard capability types
func (s *CapabilityService) mcpToolToCapabilityType(toolName string) string {
	// Map common MCP tool patterns to capability types
	toolMap := map[string]string{
		"read_file":       domain.CapabilityFileRead,
		"write_file":      domain.CapabilityFileWrite,
		"delete_file":     domain.CapabilityFileDelete,
		"execute_command": domain.CapabilitySystemAdmin,
		"query_database":  domain.CapabilityDBQuery,
		"write_database":  domain.CapabilityDBWrite,
		"call_api":        domain.CapabilityAPICall,
		"export_data":     domain.CapabilityDataExport,
	}

	// Check for exact matches first
	if capType, ok := toolMap[toolName]; ok {
		return capType
	}

	// Fallback to mcp:tool_use with tool name as suffix
	return fmt.Sprintf("%s:%s", domain.CapabilityMCPToolUse, toolName)
}

// GetAgentCapabilities retrieves all capabilities for an agent
func (s *CapabilityService) GetAgentCapabilities(
	ctx context.Context,
	agentID uuid.UUID,
	activeOnly bool,
) ([]*domain.AgentCapability, error) {
	if activeOnly {
		return s.capabilityRepo.GetActiveCapabilitiesByAgentID(agentID)
	}
	return s.capabilityRepo.GetCapabilitiesByAgentID(agentID)
}

// CapabilityDefinition represents a capability type available in the system
type CapabilityDefinition struct {
	Type           string `json:"type"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Category       string `json:"category"`
	RiskLevel      string `json:"riskLevel"`
	CapabilityType string `json:"capabilityType,omitempty"` // "core" or "custom"
}

// ListCapabilitiesResponse is the response for listing capabilities
type ListCapabilitiesResponse struct {
	Capabilities       []CapabilityDefinition `json:"capabilities"`
	ReservedNamespaces []string               `json:"reservedNamespaces"`
	ValidationPattern  string                 `json:"validationPattern"`
}

// ListCapabilities lists all available capability types from the database
func (s *CapabilityService) ListCapabilities(ctx context.Context, orgID uuid.UUID) ([]CapabilityDefinition, error) {
	// Fetch from database
	definitions, err := s.capabilityRepo.ListCapabilityDefinitions(&orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list capability definitions: %w", err)
	}

	// Convert to response format
	capabilities := make([]CapabilityDefinition, len(definitions))
	for i, def := range definitions {
		capabilities[i] = CapabilityDefinition{
			Type:           def.Type(),
			Name:           def.DisplayName,
			Description:    def.Description,
			Category:       def.Category,
			RiskLevel:      def.RiskLevel,
			CapabilityType: def.CapabilityType,
		}
	}

	return capabilities, nil
}

// ListCapabilitiesWithMetadata returns capabilities with reserved namespaces and validation pattern
func (s *CapabilityService) ListCapabilitiesWithMetadata(ctx context.Context, orgID uuid.UUID) (*ListCapabilitiesResponse, error) {
	capabilities, err := s.ListCapabilities(ctx, orgID)
	if err != nil {
		return nil, err
	}

	return &ListCapabilitiesResponse{
		Capabilities:       capabilities,
		ReservedNamespaces: domain.ReservedNamespaces,
		ValidationPattern:  domain.CapabilityValidationPattern,
	}, nil
}

// ValidateAndRegisterCapability validates a capability string and auto-registers custom capabilities
func (s *CapabilityService) ValidateAndRegisterCapability(ctx context.Context, capability string, orgID uuid.UUID) error {
	// Validate format
	if err := domain.ValidateCapabilityFormat(capability); err != nil {
		return err
	}

	// Parse namespace and action
	namespace, action, err := domain.ParseCapability(capability)
	if err != nil {
		return err
	}

	// Check if it's a reserved namespace
	if domain.IsReservedNamespace(namespace) {
		// Must be an existing core capability
		_, err := s.capabilityRepo.GetCapabilityDefinition(namespace, action, nil)
		if err != nil {
			return &domain.CapabilityError{
				Capability: capability,
				Message:    fmt.Sprintf("namespace '%s' is reserved; capability '%s' is not a valid core capability", namespace, capability),
			}
		}
		return nil
	}

	// Custom capability - check if it exists, if not create it
	_, err = s.capabilityRepo.GetCapabilityDefinition(namespace, action, &orgID)
	if err != nil {
		// Create new custom capability
		riskLevel := domain.RiskLevelMedium
		def := &domain.CapabilityDefinition{
			Namespace:      namespace,
			Action:         action,
			CapabilityType: "custom",
			RiskLevel:      riskLevel,
			MinTrustScore:  domain.DefaultMinTrustScoreForRiskLevel(riskLevel),
			DisplayName:    fmt.Sprintf("%s:%s", namespace, action),
			Description:    fmt.Sprintf("Custom capability: %s:%s", namespace, action),
			Category:       "custom",
			OrganizationID: &orgID,
		}
		if err := s.capabilityRepo.CreateCapabilityDefinition(def); err != nil {
			return fmt.Errorf("failed to register custom capability: %w", err)
		}
	}

	return nil
}

// GetViolationsByAgent retrieves violations for a specific agent
func (s *CapabilityService) GetViolationsByAgent(
	ctx context.Context,
	agentID uuid.UUID,
	limit, offset int,
) ([]*domain.CapabilityViolation, int, error) {
	return s.capabilityRepo.GetViolationsByAgentID(agentID, limit, offset)
}

// GetViolationsByOrganization retrieves all violations for an organization
func (s *CapabilityService) GetViolationsByOrganization(
	ctx context.Context,
	orgID uuid.UUID,
	limit, offset int,
) ([]*domain.CapabilityViolation, int, error) {
	return s.capabilityRepo.GetViolationsByOrganization(orgID, limit, offset)
}

// GetRecentViolations retrieves violations from the last N minutes
func (s *CapabilityService) GetRecentViolations(
	ctx context.Context,
	orgID uuid.UUID,
	minutes int,
) ([]*domain.CapabilityViolation, error) {
	return s.capabilityRepo.GetRecentViolations(orgID, minutes)
}

// Helper: Verify cryptographic signature
func (s *CapabilityService) verifySignature(publicKeyStr string, algorithm string, signature []byte, payload []byte) bool {
	// Decode public key from base64
	publicKey, err := base64.StdEncoding.DecodeString(publicKeyStr)
	if err != nil {
		return false
	}

	// For now, only Ed25519 is implemented
	// TODO: Add support for RSA and ECDSA
	if algorithm == "Ed25519" || algorithm == "" {
		if len(publicKey) != ed25519.PublicKeySize {
			return false
		}
		return ed25519.Verify(publicKey, payload, signature)
	}

	// Unsupported algorithm
	return false
}

// Helper: Check if agent has a specific capability
func (s *CapabilityService) hasCapability(capabilities []*domain.AgentCapability, requestedCapability string) bool {
	for _, cap := range capabilities {
		if cap.CapabilityType == requestedCapability {
			return true
		}
	}
	return false
}

// Helper: Convert capabilities to map for JSON storage
func (s *CapabilityService) capabilitiesToMap(capabilities []*domain.AgentCapability) map[string]interface{} {
	result := make(map[string]interface{})
	capList := make([]string, 0, len(capabilities))
	for _, cap := range capabilities {
		capList = append(capList, cap.CapabilityType)
	}
	result["capabilities"] = capList
	return result
}

// Helper: Calculate violation severity based on agent's history
func (s *CapabilityService) calculateSeverity(agent *domain.Agent) string {
	if agent.CapabilityViolationCount == 0 {
		return domain.ViolationSeverityLow
	} else if agent.CapabilityViolationCount == 1 {
		return domain.ViolationSeverityMedium
	} else if agent.CapabilityViolationCount == 2 {
		return domain.ViolationSeverityHigh
	}
	return domain.ViolationSeverityCritical
}
