package application

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// VerificationResult represents the result of an action verification
type VerificationResult struct {
	IsValid      bool    `json:"isValid"`
	IsAuthorized bool    `json:"isAuthorized"`
	InScope      bool    `json:"inScope"`
	TrustScore   float64 `json:"trustScore"`
	Message      string  `json:"message,omitempty"`
}

// CapabilityService handles capability verification and management
type CapabilityService struct {
	capabilityRepo domain.CapabilityRepository
	agentRepo      domain.AgentRepository
	auditRepo      domain.AuditLogRepository
	trustCalc      domain.TrustScoreCalculator
	trustScoreRepo domain.TrustScoreRepository
}

// NewCapabilityService creates a new capability service
func NewCapabilityService(
	capabilityRepo domain.CapabilityRepository,
	agentRepo domain.AgentRepository,
	auditRepo domain.AuditLogRepository,
	trustCalc domain.TrustScoreCalculator,
	trustScoreRepo domain.TrustScoreRepository,
) *CapabilityService {
	return &CapabilityService{
		capabilityRepo: capabilityRepo,
		agentRepo:      agentRepo,
		auditRepo:      auditRepo,
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

		// Update violation count
		newViolationCount := agent.CapabilityViolationCount + 1
		if err := s.agentRepo.UpdateTrustScore(agentID, newTrustScore); err != nil {
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
		zeroUUID := uuid.Nil
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
			UserID:         zeroUUID,
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

	// Action is within scope - update last capability check timestamp
	now := time.Now()
	agent.LastCapabilityCheckAt = &now
	if err := s.agentRepo.Update(agent); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Warning: failed to update last_capability_check_at: %v\n", err)
	}

	return &VerificationResult{
		IsValid:      true,
		IsAuthorized: true,
		InScope:      true,
		TrustScore:   agent.TrustScore,
		Message:      "Action authorized",
	}, nil
}

// GrantCapability grants a new capability to an agent
func (s *CapabilityService) GrantCapability(
	ctx context.Context,
	agentID uuid.UUID,
	capabilityType string,
	scope map[string]interface{},
	grantedBy *uuid.UUID,
) (*domain.AgentCapability, error) {
	// Verify agent exists
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	// Create capability
	capability := &domain.AgentCapability{
		AgentID:         agentID,
		CapabilityType:  capabilityType,
		CapabilityScope: scope,
		GrantedBy:       grantedBy,
		GrantedAt:       time.Now(),
	}

	if err := s.capabilityRepo.CreateCapability(capability); err != nil {
		return nil, err
	}

	// Log to audit trail
	description := fmt.Sprintf("Capability '%s' granted to agent %s", capabilityType, agent.DisplayName)
	grantedByID := uuid.Nil
	if grantedBy != nil {
		grantedByID = *grantedBy
	}
	auditLog := &domain.AuditLog{
		OrganizationID: agent.OrganizationID,
		UserID:         grantedByID,
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
	revokedByID := uuid.Nil
	if revokedBy != nil {
		revokedByID = *revokedBy
	}
	auditLog := &domain.AuditLog{
		OrganizationID: agent.OrganizationID,
		UserID:         revokedByID,
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
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	RiskLevel   string `json:"riskLevel"`
}

// ListCapabilities lists all available capability types in the system
func (s *CapabilityService) ListCapabilities(ctx context.Context, orgID uuid.UUID) ([]CapabilityDefinition, error) {
	// Return the standard set of capabilities available in AIM
	capabilities := []CapabilityDefinition{
		{
			Type:        domain.CapabilityFileRead,
			Name:        "File Read",
			Description: "Read files from the file system",
			Category:    "file_system",
			RiskLevel:   "low",
		},
		{
			Type:        domain.CapabilityFileWrite,
			Name:        "File Write",
			Description: "Write files to the file system",
			Category:    "file_system",
			RiskLevel:   "medium",
		},
		{
			Type:        domain.CapabilityFileDelete,
			Name:        "File Delete",
			Description: "Delete files from the file system",
			Category:    "file_system",
			RiskLevel:   "high",
		},
		{
			Type:        domain.CapabilityNetworkAccess,
			Name:        "Network Access",
			Description: "Make network requests and access external services",
			Category:    "network",
			RiskLevel:   "medium",
		},
		{
			Type:        domain.CapabilityDBQuery,
			Name:        "Database Query",
			Description: "Query databases (read operations)",
			Category:    "database",
			RiskLevel:   "low",
		},
		{
			Type:        domain.CapabilityDBWrite,
			Name:        "Database Write",
			Description: "Modify databases (write operations)",
			Category:    "database",
			RiskLevel:   "high",
		},
		{
			Type:        domain.CapabilityAPICall,
			Name:        "API Call",
			Description: "Call external APIs",
			Category:    "network",
			RiskLevel:   "medium",
		},
		{
			Type:        domain.CapabilityDataExport,
			Name:        "Data Export",
			Description: "Export data from the system",
			Category:    "data",
			RiskLevel:   "high",
		},
		{
			Type:        domain.CapabilitySystemAdmin,
			Name:        "System Administration",
			Description: "Execute system commands and administrative actions",
			Category:    "system",
			RiskLevel:   "critical",
		},
		{
			Type:        domain.CapabilityMCPToolUse,
			Name:        "MCP Tool Use",
			Description: "Use Model Context Protocol tools",
			Category:    "mcp",
			RiskLevel:   "medium",
		},
	}

	return capabilities, nil
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
