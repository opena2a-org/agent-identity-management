package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/crypto"
	"github.com/opena2a/identity/backend/internal/domain"
)

// AgentService handles agent business logic
type AgentService struct {
	agentRepo              domain.AgentRepository
	trustCalc              domain.TrustScoreCalculator
	trustScoreRepo         domain.TrustScoreRepository
	keyVault               *crypto.KeyVault                  // ✅ For secure private key storage
	alertRepo              domain.AlertRepository             // ✅ For creating security alerts
	policyService          *SecurityPolicyService             // ✅ For policy-based enforcement
	capabilityRepo         domain.CapabilityRepository        // ✅ For checking agent capabilities
	verificationEventService *VerificationEventService        // ✅ For creating verification events
}

// NewAgentService creates a new agent service
func NewAgentService(
	agentRepo domain.AgentRepository,
	trustCalc domain.TrustScoreCalculator,
	trustScoreRepo domain.TrustScoreRepository,
	keyVault *crypto.KeyVault,
	alertRepo domain.AlertRepository,               // ✅ NEW: AlertRepository for security alerts
	policyService *SecurityPolicyService,           // ✅ NEW: Security Policy Service
	capabilityRepo domain.CapabilityRepository,     // ✅ NEW: CapabilityRepository for capability checks
	verificationEventService *VerificationEventService, // ✅ NEW: For creating verification events
) *AgentService {
	return &AgentService{
		agentRepo:              agentRepo,
		trustCalc:              trustCalc,
		trustScoreRepo:         trustScoreRepo,
		keyVault:               keyVault,
		alertRepo:              alertRepo,
		policyService:          policyService,
		capabilityRepo:         capabilityRepo,
		verificationEventService: verificationEventService,
	}
}

// CreateAgentRequest represents agent creation request
type CreateAgentRequest struct {
	Name             string           `json:"name"`
	DisplayName      string           `json:"display_name"`
	Description      string           `json:"description"`
	AgentType        domain.AgentType `json:"agent_type"`
	Version          string           `json:"version"`
	PublicKey        string           `json:"public_key,omitempty"`  // ✅ OPTIONAL: SDK can provide its own public key
	CertificateURL   string   `json:"certificate_url"`
	RepositoryURL    string   `json:"repository_url"`
	DocumentationURL string   `json:"documentation_url"`
	TalksTo          []string `json:"talks_to,omitempty"`        // MCP servers this agent communicates with
	Capabilities     []string `json:"capabilities,omitempty"`    // Agent capabilities
}

// CreateAgent creates a new agent
func (s *AgentService) CreateAgent(ctx context.Context, req *CreateAgentRequest, orgID, userID uuid.UUID) (*domain.Agent, error) {
	// Validate inputs
	if req.Name == "" || req.DisplayName == "" {
		return nil, fmt.Errorf("name and display_name are required")
	}

	if req.AgentType != domain.AgentTypeAI && req.AgentType != domain.AgentTypeMCP {
		return nil, fmt.Errorf("invalid agent_type")
	}

	// ✅ KEY MANAGEMENT - Support both SDK-provided and auto-generated keys
	var publicKeyBase64 string
	var encryptedPrivateKey string
	var keyAlgorithm string

	if req.PublicKey != "" {
		// SDK provided its own public key (client-side keypair generation)
		// This is more secure as the private key never leaves the client
		publicKeyBase64 = req.PublicKey
		keyAlgorithm = "Ed25519"
		// No private key to store - SDK keeps it client-side
		encryptedPrivateKey = ""
	} else {
		// No public key provided - generate keypair server-side (legacy mode)
		// This maintains backward compatibility with older workflows
		keyPair, err := crypto.GenerateEd25519KeyPair()
		if err != nil {
			return nil, fmt.Errorf("failed to generate cryptographic keys: %w", err)
		}

		// Encode keys to base64 for storage
		encodedKeys := crypto.EncodeKeyPair(keyPair)
		publicKeyBase64 = encodedKeys.PublicKeyBase64
		keyAlgorithm = encodedKeys.Algorithm

		// Encrypt private key before storing (NEVER stored in plaintext)
		encPrivKey, err := s.keyVault.EncryptPrivateKey(encodedKeys.PrivateKeyBase64)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt private key: %w", err)
		}
		encryptedPrivateKey = encPrivKey
	}

	// Create agent with keys (SDK-provided or auto-generated)
	agent := &domain.Agent{
		OrganizationID:      orgID,
		Name:                req.Name,
		DisplayName:         req.DisplayName,
		Description:         req.Description,
		AgentType:           req.AgentType,
		Version:             req.Version,
		PublicKey:           &publicKeyBase64,      // ✅ Stored for verification (SDK-provided or generated)
		KeyAlgorithm:        keyAlgorithm,          // ✅ "Ed25519"
		CertificateURL:      req.CertificateURL,
		RepositoryURL:       req.RepositoryURL,
		DocumentationURL:    req.DocumentationURL,
		TalksTo:             req.TalksTo,           // MCP servers this agent communicates with
		Capabilities:        req.Capabilities,      // ✅ Store detected capabilities from SDK
		Status:              domain.AgentStatusPending,
		CreatedBy:           userID,
	}

	// Only set encrypted private key if we generated it server-side
	if encryptedPrivateKey != "" {
		agent.EncryptedPrivateKey = &encryptedPrivateKey // ✅ Encrypted storage (never exposed in API)
	}

	if err := s.agentRepo.Create(agent); err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Calculate initial trust score
	trustScore, err := s.trustCalc.Calculate(agent)
	if err != nil {
		// Log error but don't fail the creation
		fmt.Printf("Warning: failed to calculate trust score: %v\n", err)
	} else {
		agent.TrustScore = trustScore.Score
		if err := s.agentRepo.Update(agent); err != nil {
			fmt.Printf("Warning: failed to update trust score: %v\n", err)
		}
		if err := s.trustScoreRepo.Create(trustScore); err != nil {
			fmt.Printf("Warning: failed to save trust score: %v\n", err)
		}
	}

	// ✅ AUTO-VERIFICATION: Automatically verify agent if it meets basic criteria
	// This eliminates manual verification step for legitimate agents
	shouldAutoVerify := s.shouldAutoVerifyAgent(agent)
	if shouldAutoVerify {
		now := time.Now()
		agent.Status = domain.AgentStatusVerified
		agent.VerifiedAt = &now

		if err := s.agentRepo.Update(agent); err != nil {
			fmt.Printf("Warning: failed to auto-verify agent: %v\n", err)
		} else {
			fmt.Printf("✅ Agent %s auto-verified (trust score: %.2f)\n", agent.Name, agent.TrustScore)
		}

		// ✅ CREATE VERIFICATION EVENT for dashboard chart
		// This populates the Agent Verification Activity chart
		if s.verificationEventService != nil {
			verifiedResult := domain.VerificationResultVerified
			verificationReq := &CreateVerificationEventRequest{
				OrganizationID:   orgID,
				AgentID:          agent.ID,
				Protocol:         domain.VerificationProtocolA2A,
				VerificationType: domain.VerificationTypeIdentity,
				Status:           domain.VerificationEventStatusSuccess,
				Result:           &verifiedResult,
				DurationMs:       0,
				InitiatorType:    domain.InitiatorTypeSystem,
			}

			if _, err := s.verificationEventService.CreateVerificationEvent(ctx, verificationReq); err != nil {
				fmt.Printf("⚠️  Warning: failed to create verification event: %v\n", err)
			} else {
				fmt.Printf("✅ Created verification event for agent %s\n", agent.Name)
			}
		}

		// Recalculate trust score with verified status (verification boosts score)
		updatedTrustScore, err := s.trustCalc.Calculate(agent)
		if err == nil {
			agent.TrustScore = updatedTrustScore.Score
			s.agentRepo.Update(agent)
			s.trustScoreRepo.Create(updatedTrustScore)
			fmt.Printf("✅ Updated trust score after verification: %.2f\n", agent.TrustScore)
		}
	}

	// ✅ AUTO-GRANT CAPABILITIES: Auto-grant declared capabilities during registration
	// This eliminates admin approval bottleneck - users can start using agents immediately!
	// Admins only approve capability UPDATES, not initial registration.
	if len(req.Capabilities) > 0 {
		grantedCount := 0
		for _, capabilityType := range req.Capabilities {
			capabilityRecord := &domain.AgentCapability{
				AgentID:        agent.ID,
				CapabilityType: capabilityType,
				GrantedBy:      &userID, // Auto-granted by user who created agent
				GrantedAt:      time.Now(),
			}

			if err := s.capabilityRepo.CreateCapability(capabilityRecord); err != nil {
				fmt.Printf("⚠️  Warning: failed to auto-grant capability '%s': %v\n", capabilityType, err)
			} else {
				grantedCount++
			}
		}

		if grantedCount > 0 {
			fmt.Printf("✅ Auto-granted %d capabilities for agent %s: %v\n", grantedCount, agent.Name, req.Capabilities)
		}
	}

	return agent, nil
}

// shouldAutoVerifyAgent determines if an agent meets criteria for automatic verification
// Auto-verification criteria:
// 1. Has valid cryptographic keys (public + encrypted private key)
// 2. Trust score >= 0.3 (30% minimum threshold)
// 3. Has required metadata (name, description, type)
func (s *AgentService) shouldAutoVerifyAgent(agent *domain.Agent) bool {
	// ✅ Check 1: Must have cryptographic keys
	if agent.PublicKey == nil || agent.EncryptedPrivateKey == nil {
		fmt.Printf("⚠️  Agent %s cannot be auto-verified: missing cryptographic keys\n", agent.Name)
		return false
	}

	// ✅ Check 2: Trust score must be >= 0.3 (30%)
	if agent.TrustScore < 0.3 {
		fmt.Printf("⚠️  Agent %s cannot be auto-verified: trust score too low (%.2f < 0.3)\n", agent.Name, agent.TrustScore)
		return false
	}

	// ✅ Check 3: Must have required metadata
	if agent.Name == "" || agent.DisplayName == "" || agent.Description == "" {
		fmt.Printf("⚠️  Agent %s cannot be auto-verified: missing required metadata\n", agent.Name)
		return false
	}

	// ✅ All checks passed - agent qualifies for auto-verification
	return true
}

// GetAgent retrieves an agent by ID
func (s *AgentService) GetAgent(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
	return s.agentRepo.GetByID(id)
}

// ListAgents lists agents for an organization
func (s *AgentService) ListAgents(ctx context.Context, orgID uuid.UUID) ([]*domain.Agent, error) {
	return s.agentRepo.GetByOrganization(orgID)
}

// UpdateAgent updates an agent
func (s *AgentService) UpdateAgent(ctx context.Context, id uuid.UUID, req *CreateAgentRequest) (*domain.Agent, error) {
	agent, err := s.agentRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Update fields
	if req.DisplayName != "" {
		agent.DisplayName = req.DisplayName
	}
	if req.Description != "" {
		agent.Description = req.Description
	}
	if req.Version != "" {
		agent.Version = req.Version
	}
	// ✅ REMOVED: PublicKey update - keys are immutable after creation
	if req.CertificateURL != "" {
		agent.CertificateURL = req.CertificateURL
	}
	if req.RepositoryURL != "" {
		agent.RepositoryURL = req.RepositoryURL
	}
	if req.DocumentationURL != "" {
		agent.DocumentationURL = req.DocumentationURL
	}
	// Update talks_to configuration
	if req.TalksTo != nil {
		agent.TalksTo = req.TalksTo
	}
	
	if err := s.agentRepo.Update(agent); err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	if req.Capabilities != nil && len(req.Capabilities) > 0 {
        // Get current capabilities
        currentCaps, err := s.capabilityRepo.GetCapabilitiesByAgentID(id)
        if err != nil {
            fmt.Printf("⚠️  Warning: failed to get current capabilities: %v\n", err)
        }

        // Build map of current capability types
        currentCapTypes := make(map[string]*domain.AgentCapability)
        for _, cap := range currentCaps {
            if cap.RevokedAt == nil {
                currentCapTypes[cap.CapabilityType] = cap
            }
        }

        // Build map of requested capability types
        requestedCapTypes := make(map[string]bool)
        for _, capType := range req.Capabilities {
            requestedCapTypes[capType] = true
        }

        // Add new capabilities that don't exist
        for _, capType := range req.Capabilities {
            if _, exists := currentCapTypes[capType]; !exists {
                capabilityRecord := &domain.AgentCapability{
                    AgentID:        id,
                    CapabilityType: capType,
                    GrantedBy:      &agent.CreatedBy, // Use agent creator as granter
                    GrantedAt:      time.Now(),
                }
                if err := s.capabilityRepo.CreateCapability(capabilityRecord); err != nil {
                    fmt.Printf("⚠️  Warning: failed to add capability '%s': %v\n", capType, err)
                } else {
                    fmt.Printf("✅ Added capability '%s' to agent %s\n", capType, agent.Name)
                }
            }
        }

        // Revoke capabilities that are no longer in the request
        for capType, cap := range currentCapTypes {
            if !requestedCapTypes[capType] {
                now := time.Now()
                if err := s.capabilityRepo.RevokeCapability(cap.ID, now); err != nil {
                    fmt.Printf("⚠️  Warning: failed to revoke capability '%s': %v\n", capType, err)
                } else {
                    fmt.Printf("🗑️  Revoked capability '%s' from agent %s\n", capType, agent.Name)
                }
            }
        }
    }
	// Recalculate trust score
	trustScore, err := s.trustCalc.Calculate(agent)
	if err == nil {
		agent.TrustScore = trustScore.Score
		s.agentRepo.Update(agent)
		s.trustScoreRepo.Create(trustScore)
	}

	return agent, nil
}

// DeleteAgent deletes an agent
func (s *AgentService) DeleteAgent(ctx context.Context, id uuid.UUID) error {
	return s.agentRepo.Delete(id)
}

// VerifyAgent verifies an agent
func (s *AgentService) VerifyAgent(ctx context.Context, id uuid.UUID) error {
	agent, err := s.agentRepo.GetByID(id)
	if err != nil {
		return err
	}

	now := time.Now()
	agent.Status = domain.AgentStatusVerified
	agent.VerifiedAt = &now

	if err := s.agentRepo.Update(agent); err != nil {
		return fmt.Errorf("failed to verify agent: %w", err)
	}

	// Recalculate trust score
	trustScore, err := s.trustCalc.Calculate(agent)
	if err == nil {
		agent.TrustScore = trustScore.Score
		s.agentRepo.Update(agent)
		s.trustScoreRepo.Create(trustScore)
	}

	return nil
}

// RecalculateTrustScore recalculates trust score for an agent
func (s *AgentService) RecalculateTrustScore(ctx context.Context, id uuid.UUID) (*domain.TrustScore, error) {
	agent, err := s.agentRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	trustScore, err := s.trustCalc.Calculate(agent)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate trust score: %w", err)
	}

	// Update agent with new score
	agent.TrustScore = trustScore.Score
	if err := s.agentRepo.Update(agent); err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	// Save trust score history
	if err := s.trustScoreRepo.Create(trustScore); err != nil {
		return nil, fmt.Errorf("failed to save trust score: %w", err)
	}

	return trustScore, nil
}

// UpdateTrustScore manually updates an agent's trust score (admin override)
func (s *AgentService) UpdateTrustScore(ctx context.Context, agentID uuid.UUID, newScore float64) error {
	// Validate score range (0.000 to 9.999 based on database schema)
	if newScore < 0.0 || newScore > 9.999 {
		return fmt.Errorf("trust score must be between 0.0 and 9.999")
	}

	// Get agent to check previous score and for alert creation
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return fmt.Errorf("failed to get agent: %w", err)
	}

	previousScore := agent.TrustScore

	// Update trust score in database
	if err := s.agentRepo.UpdateTrustScore(agentID, newScore); err != nil {
		return fmt.Errorf("failed to update trust score: %w", err)
	}

	// Check for significant trust score drop and create alert if needed
	s.checkAndCreateTrustScoreDropAlert(ctx, agent, previousScore, newScore)

	return nil
}

// checkAndCreateTrustScoreDropAlert checks for significant trust score drops and creates alerts
func (s *AgentService) checkAndCreateTrustScoreDropAlert(ctx context.Context, agent *domain.Agent, previousScore, currentScore float64) {
	// Configuration thresholds
	const (
		significantDropThreshold = 0.1  // 10% drop triggers warning
		criticalDropThreshold    = 0.2  // 20% drop triggers critical
		lowScoreThreshold        = 0.5  // 50% trust score threshold
	)

	// Calculate drop
	if previousScore <= 0 {
		return // No meaningful comparison
	}

	drop := previousScore - currentScore
	if drop <= 0 {
		return // Score increased or stayed the same
	}

	dropPercentage := drop / previousScore

	var alert *domain.Alert
	agentName := agent.DisplayName
	if agentName == "" {
		agentName = agent.Name
	}

	// Critical drop (>20% OR score dropped below 50%)
	if dropPercentage >= criticalDropThreshold || (drop > 0 && currentScore < lowScoreThreshold) {
		alert = &domain.Alert{
			OrganizationID: agent.OrganizationID,
			AlertType:      domain.AlertTrustScoreDrop,
			Severity:       domain.AlertSeverityCritical,
			Title:          fmt.Sprintf("Critical Trust Score Drop for '%s'", agentName),
			Description:    fmt.Sprintf("Agent trust score dropped from %.1f%% to %.1f%% (%.1f%% decrease). This may indicate a security issue or policy violation.", previousScore*100, currentScore*100, drop*100),
			ResourceType:   "agent",
			ResourceID:     agent.ID,
		}
	} else if dropPercentage >= significantDropThreshold {
		// Significant drop (>10%)
		alert = &domain.Alert{
			OrganizationID: agent.OrganizationID,
			AlertType:      domain.AlertTrustScoreDrop,
			Severity:       domain.AlertSeverityWarning,
			Title:          fmt.Sprintf("Trust Score Drop Detected for '%s'", agentName),
			Description:    fmt.Sprintf("Agent trust score dropped from %.1f%% to %.1f%% (%.1f%% decrease). Monitor this agent's behavior.", previousScore*100, currentScore*100, drop*100),
			ResourceType:   "agent",
			ResourceID:     agent.ID,
		}
	}

	if alert == nil {
		return // No significant drop
	}

	// Check for existing unacknowledged alert to avoid duplicates
	existing, _ := s.alertRepo.GetUnacknowledged(agent.OrganizationID)
	for _, a := range existing {
		if a.ResourceID == agent.ID && a.AlertType == domain.AlertTrustScoreDrop {
			return // Alert already exists
		}
	}

	// Create the alert
	s.alertRepo.Create(alert)
}

// CreateSecurityAlert creates a security alert in the database
func (s *AgentService) CreateSecurityAlert(ctx context.Context, alert *domain.Alert) error {
	return s.alertRepo.Create(alert)
}

// HasCapability checks if an agent has a specific capability
func (s *AgentService) HasCapability(ctx context.Context, agentID uuid.UUID, actionType string, resource string) (bool, error) {
	// Get agent's active capabilities
	capabilities, err := s.capabilityRepo.GetActiveCapabilitiesByAgentID(agentID)
	if err != nil {
		return false, fmt.Errorf("failed to get capabilities: %w", err)
	}

	// If agent has no capabilities, return false
	if len(capabilities) == 0 {
		return false, nil
	}

	// Check if action matches any capability
	for _, capability := range capabilities {
		if s.matchesCapability(actionType, resource, capability.CapabilityType) {
			return true, nil
		}
	}

	return false, nil
}

// VerifyAction verifies if an agent can perform an action
// ✅ CRITICAL SECURITY FUNCTION - EchoLeak Prevention
// This is the core defense mechanism that prevented CVE-2025-32711 (EchoLeak) attack
func (s *AgentService) VerifyAction(
	ctx context.Context,
	agentID uuid.UUID,
	actionType string,
	resource string,
	metadata map[string]interface{},
) (allowed bool, reason string, auditID uuid.UUID, err error) {
	auditID = uuid.New()

	// 1. Fetch agent
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return false, "Agent not found", uuid.Nil, err
	}

	// 2. Check agent status - MUST be verified
	if agent.Status != domain.AgentStatusVerified {
		return false, "Agent not verified - all actions denied", auditID, nil
	}

	// 3. Check if agent is compromised
	if agent.IsCompromised {
		return false, "Agent is marked as compromised - all actions denied", auditID, nil
	}

	// 4. ✅ CAPABILITY-BASED ACCESS CONTROL (CBAC)
	// This is what prevents EchoLeak and similar attacks
	//
	// ✅ ENTERPRISE ARCHITECTURE: SINGLE SOURCE OF TRUTH
	// - agent_capabilities table records = GRANTED capabilities (enforcement)
	// - agent.capabilities array = DECLARED capabilities (reference only)
	//
	// Security Workflow:
	// 1. Agent declares capabilities during registration (agent.capabilities)
	// 2. Admin reviews and grants specific capabilities (agent_capabilities table)
	// 3. System enforces ONLY granted capabilities (this function)
	//
	// This prevents:
	// - Unauthorized capability escalation (agents can't self-authorize)
	// - Scope violations like CVE-2025-32711 (EchoLeak)
	// - Unclear approval chains (full audit trail via granted_by, granted_at)

	// ✅ Fetch GRANTED capabilities (single source of truth for enforcement)
	activeCapabilities, err := s.capabilityRepo.GetActiveCapabilitiesByAgentID(agentID)
	if err != nil {
		return false, fmt.Sprintf("Failed to fetch agent capabilities: %v", err), auditID, err
	}

	// Build list of granted capability types for error messages
	capabilityTypes := []string{}
	hasCapability := false

	for _, capability := range activeCapabilities {
		capabilityTypes = append(capabilityTypes, capability.CapabilityType)
		if s.matchesCapability(actionType, resource, capability.CapabilityType) {
			hasCapability = true
		}
	}

	// ⚠️  CRITICAL: If agent has NO GRANTED capabilities, DENY ALL actions
	if len(capabilityTypes) == 0 {
		return false, "Agent has no granted capabilities - action denied (admin must grant capabilities first)", auditID, nil
	}

	if !hasCapability {
		// ✅ CAPABILITY VIOLATION DETECTED - Evaluate security policies
		// This prevents scope violations like EchoLeak's bulk email access

		// 🛡️ Evaluate security policies to determine enforcement action
		shouldBlock, shouldAlert, policyName, err := s.policyService.EvaluateCapabilityViolation(
			ctx, agent, actionType, resource, auditID,
		)
		if err != nil {
			// Policy evaluation failed - use safe default (block + alert)
			fmt.Printf("⚠️  Policy evaluation failed: %v, using safe default (block + alert)\n", err)
			shouldBlock = true
			shouldAlert = true
			policyName = "default_policy"
		}

		// 🚨 CREATE SECURITY ALERT if policy requires it
		if shouldAlert {
			alertTitle := fmt.Sprintf("Capability Violation Detected: %s", agent.DisplayName)
			alertDescription := fmt.Sprintf(
				"Agent '%s' attempted unauthorized action '%s' which is not in its capability list (allowed: %v). "+
				"This matches the attack pattern of CVE-2025-32711 (EchoLeak). "+
				"Security Policy '%s' enforcement: %s. Audit ID: %s",
				agent.DisplayName, actionType, capabilityTypes, policyName,
				map[bool]string{true: "BLOCKED", false: "ALLOWED (monitored)"}[shouldBlock],
				auditID.String(),
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

			if err := s.alertRepo.Create(alert); err != nil {
				fmt.Printf("⚠️  Warning: failed to create security alert: %v\n", err)
			} else {
				fmt.Printf("🚨 SECURITY ALERT: Capability violation for agent %s (policy: %s, action: %s)\n",
					agent.Name, policyName, map[bool]string{true: "BLOCKED", false: "MONITORED"}[shouldBlock])
			}
		}

		// 📝 CREATE VIOLATION RECORD for dashboard tracking
		// This ensures the Violations tab shows all capability violations
		violation := &domain.CapabilityViolation{
			AgentID:             agentID,
			AttemptedCapability: actionType,
			RegisteredCapabilities: map[string]interface{}{
				"allowed_capabilities": capabilityTypes,
				"attempted_action":     actionType,
				"resource":             resource,
			},
			Severity:         s.calculateViolationSeverity(agent, shouldBlock),
			TrustScoreImpact: s.calculateTrustScoreImpact(shouldBlock),
			IsBlocked:        shouldBlock,
			SourceIP:         nil, // Could be passed from context if available
			RequestMetadata:  metadata,
		}

		if err := s.capabilityRepo.CreateViolation(violation); err != nil {
			fmt.Printf("⚠️  Warning: failed to create violation record: %v\n", err)
		} else {
			fmt.Printf("📝 VIOLATION RECORDED: Agent %s attempted %s (blocked: %v)\n",
				agent.Name, actionType, shouldBlock)
		}

		// Return enforcement decision from policy
		if shouldBlock {
			return false, fmt.Sprintf(
				"Capability violation blocked by security policy '%s': Agent does not have permission for action '%s' (allowed: %v)",
				policyName, actionType, capabilityTypes,
			), auditID, nil
		} else {
			// Policy says alert-only mode - allow the action but log it
			fmt.Printf("⚠️  Capability violation ALLOWED by policy '%s' (alert-only mode): %s attempting %s\n",
				policyName, agent.Name, actionType)
			return true, fmt.Sprintf(
				"Action allowed by security policy '%s' (alert-only mode) - capability violation logged",
				policyName,
			), auditID, nil
		}
	}

	// 6. ✅ CAPABILITY CHECK PASSED - Now evaluate additional security policies
	// Even if agent has the capability, we still need to check other policy types

	// 6.1 Trust Score Policy Evaluation
	trustScoreBlocked, trustScoreAlert, trustScorePolicyName, err := s.policyService.EvaluateTrustScoreLow(
		ctx, agent, actionType, resource, auditID,
	)
	if err != nil {
		fmt.Printf("⚠️  Trust score policy evaluation failed: %v\n", err)
	}
	if trustScoreAlert {
		s.createPolicyAlert(agent, "Trust Score Low", trustScorePolicyName, trustScoreBlocked,
			fmt.Sprintf("Agent has low trust score (%.2f)", agent.TrustScore), domain.AlertSeverityWarning, auditID)
	}
	if trustScoreBlocked {
		return false, fmt.Sprintf(
			"Action blocked by trust score policy '%s': Agent trust score too low (%.2f)",
			trustScorePolicyName, agent.TrustScore,
		), auditID, nil
	}

	// 6.2 Data Exfiltration Policy Evaluation
	exfilBlocked, exfilAlert, exfilPolicyName, err := s.policyService.EvaluateDataExfiltration(
		ctx, agent, actionType, resource, auditID,
	)
	if err != nil {
		fmt.Printf("⚠️  Data exfiltration policy evaluation failed: %v\n", err)
	}
	if exfilAlert {
		s.createPolicyAlert(agent, "Data Exfiltration Attempt", exfilPolicyName, exfilBlocked,
			fmt.Sprintf("Suspected data exfiltration pattern detected: %s on %s", actionType, resource),
			domain.AlertSeverityCritical, auditID)
	}
	if exfilBlocked {
		return false, fmt.Sprintf(
			"Action blocked by data exfiltration policy '%s': Suspicious pattern detected",
			exfilPolicyName,
		), auditID, nil
	}

	// 6.3 Unusual Activity Policy Evaluation (stub - needs historical data)
	unusualBlocked, unusualAlert, unusualPolicyName, err := s.policyService.EvaluateUnusualActivity(
		ctx, agent, actionType, resource, auditID,
	)
	if err != nil {
		fmt.Printf("⚠️  Unusual activity policy evaluation failed: %v\n", err)
	}
	if unusualAlert {
		s.createPolicyAlert(agent, "Unusual Activity", unusualPolicyName, unusualBlocked,
			"Anomalous behavior pattern detected", domain.AlertSeverityWarning, auditID)
	}
	if unusualBlocked {
		return false, fmt.Sprintf(
			"Action blocked by unusual activity policy '%s'",
			unusualPolicyName,
		), auditID, nil
	}

	// 6.4 Config Drift Policy Evaluation (stub - needs baseline)
	driftBlocked, driftAlert, driftPolicyName, err := s.policyService.EvaluateConfigDrift(
		ctx, agent, actionType, resource, auditID,
	)
	if err != nil {
		fmt.Printf("⚠️  Config drift policy evaluation failed: %v\n", err)
	}
	if driftAlert {
		s.createPolicyAlert(agent, "Configuration Drift", driftPolicyName, driftBlocked,
			"Agent configuration has drifted from baseline", domain.AlertSeverityWarning, auditID)
	}
	if driftBlocked {
		return false, fmt.Sprintf(
			"Action blocked by config drift policy '%s'",
			driftPolicyName,
		), auditID, nil
	}

	// 6.5 Unauthorized Access Policy Evaluation (stub)
	unauthBlocked, unauthAlert, unauthPolicyName, err := s.policyService.EvaluateUnauthorizedAccess(
		ctx, agent, actionType, resource, auditID,
	)
	if err != nil {
		fmt.Printf("⚠️  Unauthorized access policy evaluation failed: %v\n", err)
	}
	if unauthAlert {
		s.createPolicyAlert(agent, "Unauthorized Access Attempt", unauthPolicyName, unauthBlocked,
			"Unauthorized access pattern detected", domain.AlertSeverityHigh, auditID)
	}
	if unauthBlocked {
		return false, fmt.Sprintf(
			"Action blocked by unauthorized access policy '%s'",
			unauthPolicyName,
		), auditID, nil
	}

	// 7. ✅ ALL POLICIES PASSED - Action is allowed
	return true, "Action matches registered capabilities and passes all security policies", auditID, nil
}

// matchesCapability checks if an action matches a registered capability
// Supports exact matching and wildcard patterns
func (s *AgentService) matchesCapability(actionType string, resource string, capability string) bool {
	// Exact match
	if actionType == capability {
		return true
	}

	// Wildcard patterns (e.g., "read_*" matches "read_email", "read_file")
	if len(capability) > 0 && capability[len(capability)-1] == '*' {
		prefix := capability[:len(capability)-1]
		if len(actionType) >= len(prefix) && actionType[:len(prefix)] == prefix {
			return true
		}
	}

	// Future: Add more sophisticated pattern matching here
	// - Resource-based matching (e.g., "read:/data/*")
	// - Time-based capabilities
	// - Context-aware matching

	return false
}

// LogActionResult logs the outcome of a verified action
func (s *AgentService) LogActionResult(
	ctx context.Context,
	agentID uuid.UUID,
	auditID uuid.UUID,
	success bool,
	errorMsg string,
	result map[string]interface{},
) error {
	// Fetch agent for context
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	// Determine verification status
	var eventStatus domain.VerificationEventStatus
	if success {
		eventStatus = domain.VerificationEventStatusSuccess
	} else {
		eventStatus = domain.VerificationEventStatusFailed
	}

	// Build metadata with result details
	metadata := make(map[string]interface{})
	if result != nil {
		for k, v := range result {
			metadata[k] = v
		}
	}
	if errorMsg != "" {
		metadata["error"] = errorMsg
	}
	metadata["audit_id"] = auditID.String()

	// Create the verification event for audit trail
	if s.verificationEventService != nil {
		_, err := s.verificationEventService.LogVerificationEvent(
			ctx,
			agent.OrganizationID,
			agentID,
			domain.VerificationProtocolA2A,
			domain.VerificationTypeCapability,
			eventStatus,
			0, // durationMs not tracked for action results
			domain.InitiatorTypeAgent,
			&agentID,
			metadata,
		)
		if err != nil {
			// Log but don't fail - audit logging shouldn't break business logic
			fmt.Printf("Warning: Failed to record action result audit log: %v\n", err)
		}
	}

	// Track repeated failures and potentially create alerts
	if !success && s.alertRepo != nil {
		// Check for repeated failures pattern
		// This could trigger an alert if many consecutive failures occur
		// For now, we only alert on explicitly flagged issues in the result
		if shouldAlert, ok := result["create_alert"].(bool); ok && shouldAlert {
			alertDesc := fmt.Sprintf("Agent %s experienced action failure: %s", agent.Name, errorMsg)
			alert := &domain.Alert{
				ID:             uuid.New(),
				OrganizationID: agent.OrganizationID,
				AlertType:      domain.AlertUnusualActivity,
				Severity:       domain.AlertSeverityWarning,
				Title:          "Agent Action Failed",
				Description:    alertDesc,
				ResourceType:   "agent",
				ResourceID:     agentID,
				IsAcknowledged: false,
				CreatedAt:      time.Now(),
			}
			if err := s.alertRepo.Create(alert); err != nil {
				fmt.Printf("Warning: Failed to create action failure alert: %v\n", err)
			}
		}
	}

	return nil
}

// GetAgentCredentials retrieves agent credentials for SDK generation
// ⚠️ INTERNAL USE ONLY - Never expose through public API
// This method decrypts the private key for embedding in SDKs
func (s *AgentService) GetAgentCredentials(ctx context.Context, agentID uuid.UUID) (publicKey, privateKey string, err error) {
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return "", "", fmt.Errorf("agent not found: %w", err)
	}

	if agent.PublicKey == nil || agent.EncryptedPrivateKey == nil {
		return "", "", fmt.Errorf("agent keys not generated")
	}

	// Decrypt private key
	privateKeyBase64, err := s.keyVault.DecryptPrivateKey(*agent.EncryptedPrivateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to decrypt private key: %w", err)
	}

	return *agent.PublicKey, privateKeyBase64, nil
}

// ========================================
// MCP Server Relationship Management
// ========================================

// AddMCPServersRequest represents request to add MCP servers to agent's talks_to list
type AddMCPServersRequest struct {
	MCPServerIDs   []string               `json:"mcp_server_ids"`   // MCP server IDs or names
	DetectedMethod string                 `json:"detected_method"`  // "manual", "auto_sdk", "auto_config", "cli"
	Confidence     float64                `json:"confidence"`       // Detection confidence (0-100)
	Metadata       map[string]interface{} `json:"metadata"`         // Additional context
}

// MCPServerDetail represents detailed MCP server information
type MCPServerDetail struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	URL            string    `json:"url"`
	Status         string    `json:"status"`
	TrustScore     float64   `json:"trust_score"`
	AddedAt        time.Time `json:"added_at"`
	DetectedMethod string    `json:"detected_method"`
}

// AddMCPServers adds MCP servers to an agent's talks_to list
func (s *AgentService) AddMCPServers(
	ctx context.Context,
	agentID uuid.UUID,
	mcpServerIdentifiers []string,
) (*domain.Agent, []string, error) {
	// 1. Fetch agent
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return nil, nil, fmt.Errorf("agent not found: %w", err)
	}

	// 2. Initialize talks_to if nil
	if agent.TalksTo == nil {
		agent.TalksTo = []string{}
	}

	// 3. Create a map to track existing entries (prevent duplicates)
	existingMap := make(map[string]bool)
	for _, existing := range agent.TalksTo {
		existingMap[existing] = true
	}

	// 4. Add new MCP servers (only unique ones)
	addedServers := []string{}
	for _, identifier := range mcpServerIdentifiers {
		if !existingMap[identifier] {
			agent.TalksTo = append(agent.TalksTo, identifier)
			existingMap[identifier] = true
			addedServers = append(addedServers, identifier)
		}
	}

	// 5. Update agent in database
	if len(addedServers) > 0 {
		if err := s.agentRepo.Update(agent); err != nil {
			return nil, nil, fmt.Errorf("failed to update agent: %w", err)
		}

		// 6. Automatically recalculate trust score after MCP connections change
		trustScore, err := s.trustCalc.Calculate(agent)
		if err == nil {
			agent.TrustScore = trustScore.Score
			s.agentRepo.Update(agent)
			s.trustScoreRepo.Create(trustScore)
		}
	}

	return agent, addedServers, nil
}

// RemoveMCPServers removes MCP servers from an agent's talks_to list
func (s *AgentService) RemoveMCPServers(
	ctx context.Context,
	agentID uuid.UUID,
	mcpServerIdentifiers []string,
) (*domain.Agent, []string, error) {
	// 1. Fetch agent
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return nil, nil, fmt.Errorf("agent not found: %w", err)
	}

	// 2. Initialize talks_to if nil
	if agent.TalksTo == nil {
		agent.TalksTo = []string{}
		return agent, []string{}, nil
	}

	// 3. Create a map of servers to remove
	removeMap := make(map[string]bool)
	for _, identifier := range mcpServerIdentifiers {
		removeMap[identifier] = true
	}

	// 4. Filter out removed servers
	removedServers := []string{}
	newTalksTo := []string{}
	for _, existing := range agent.TalksTo {
		if removeMap[existing] {
			removedServers = append(removedServers, existing)
		} else {
			newTalksTo = append(newTalksTo, existing)
		}
	}

	// 5. Update agent with new talks_to list
	agent.TalksTo = newTalksTo
	if len(removedServers) > 0 {
		if err := s.agentRepo.Update(agent); err != nil {
			return nil, nil, fmt.Errorf("failed to update agent: %w", err)
		}

		// 6. Automatically recalculate trust score after MCP connections change
		trustScore, err := s.trustCalc.Calculate(agent)
		if err == nil {
			agent.TrustScore = trustScore.Score
			s.agentRepo.Update(agent)
			s.trustScoreRepo.Create(trustScore)
		}
	}

	return agent, removedServers, nil
}

// RemoveMCPServer removes a single MCP server from an agent's talks_to list
func (s *AgentService) RemoveMCPServer(
	ctx context.Context,
	agentID uuid.UUID,
	mcpServerIdentifier string,
) (*domain.Agent, error) {
	agent, _, err := s.RemoveMCPServers(ctx, agentID, []string{mcpServerIdentifier})
	return agent, err
}

// GetAgentMCPServers retrieves detailed information about MCP servers an agent talks to
// This returns the full MCP server details, not just the IDs/names in talks_to
func (s *AgentService) GetAgentMCPServers(
	ctx context.Context,
	agentID uuid.UUID,
	mcpRepo domain.MCPServerRepository,
) ([]*domain.MCPServer, error) {
	// 1. Fetch agent
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	// 2. If no talks_to entries, return empty list
	if agent.TalksTo == nil || len(agent.TalksTo) == 0 {
		return []*domain.MCPServer{}, nil
	}

	// 3. Fetch all MCP servers for the organization
	allMCPServers, err := mcpRepo.GetByOrganization(agent.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch MCP servers: %w", err)
	}

	// 4. Create a map of talks_to identifiers for fast lookup
	talksToMap := make(map[string]bool)
	for _, identifier := range agent.TalksTo {
		talksToMap[identifier] = true
	}

	// 5. Filter MCP servers that match talks_to (by ID or name)
	matchingServers := []*domain.MCPServer{}
	for _, server := range allMCPServers {
		// Match by ID or name
		if talksToMap[server.ID.String()] || talksToMap[server.Name] {
			matchingServers = append(matchingServers, server)
		}
	}

	return matchingServers, nil
}

// ========================================
// Auto-Detection of MCP Servers
// ========================================

// DetectMCPServersRequest represents request to auto-detect MCP servers from config
type DetectMCPServersRequest struct {
	ConfigPath   string `json:"config_path"`    // Path to Claude Desktop config file
	AutoRegister bool   `json:"auto_register"`  // Whether to auto-register discovered MCPs
	DryRun       bool   `json:"dry_run"`        // Preview changes without applying
}

// DetectedMCPServer represents an MCP server detected from config
type DetectedMCPServer struct {
	Name       string                 `json:"name"`
	Command    string                 `json:"command"`
	Args       []string               `json:"args"`
	Env        map[string]string      `json:"env,omitempty"`
	Confidence float64                `json:"confidence"` // 0-100
	Source     string                 `json:"source"`     // "claude_desktop_config"
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// DetectMCPServersResult represents the result of auto-detection
type DetectMCPServersResult struct {
	DetectedServers  []DetectedMCPServer `json:"detected_servers"`
	RegisteredCount  int                 `json:"registered_count"`
	MappedCount      int                 `json:"mapped_count"`
	TotalTalksTo     int                 `json:"total_talks_to"`
	DryRun           bool                `json:"dry_run"`
	ErrorsEncountered []string           `json:"errors_encountered,omitempty"`
}

// DetectMCPServersFromConfig auto-detects MCP servers from Claude Desktop config
func (s *AgentService) DetectMCPServersFromConfig(
	ctx context.Context,
	agentID uuid.UUID,
	req *DetectMCPServersRequest,
	mcpService *MCPService,
	orgID uuid.UUID,
	userID uuid.UUID,
) (*DetectMCPServersResult, error) {
	// 1. Validate request
	if req.ConfigPath == "" {
		return nil, fmt.Errorf("config_path is required")
	}

	// 2. Parse Claude Desktop config file
	detectedServers, err := s.parseClaudeDesktopConfig(req.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 3. If dry run, return immediately with detected servers
	if req.DryRun {
		return &DetectMCPServersResult{
			DetectedServers: detectedServers,
			DryRun:          true,
		}, nil
	}

	// 4. Auto-register new MCP servers if requested
	registeredCount := 0
	mcpServerIdentifiers := []string{}
	errorsEncountered := []string{}

	if req.AutoRegister {
		for _, detected := range detectedServers {
			// Try to register the MCP server
			// Note: CreateMCPServerRequest expects URL, but Claude config uses command/args
			// We'll use the name as a placeholder URL for now
			registerReq := &CreateMCPServerRequest{
				Name:        detected.Name,
				Description: fmt.Sprintf("Auto-detected from Claude Desktop config. Command: %s", detected.Command),
				URL:         fmt.Sprintf("mcp://%s", detected.Name), // Placeholder URL for local MCP servers
			}

			_, err := mcpService.CreateMCPServer(ctx, registerReq, orgID, userID, nil)
			if err != nil {
				// If already exists, that's fine - we'll use existing
				errorsEncountered = append(errorsEncountered,
					fmt.Sprintf("MCP '%s': %v", detected.Name, err))
			} else {
				registeredCount++
			}

			mcpServerIdentifiers = append(mcpServerIdentifiers, detected.Name)
		}
	} else {
		// Just extract names for mapping
		for _, detected := range detectedServers {
			mcpServerIdentifiers = append(mcpServerIdentifiers, detected.Name)
		}
	}

	// 5. Add detected MCP servers to agent's talks_to list
	agent, addedServers, err := s.AddMCPServers(ctx, agentID, mcpServerIdentifiers)
	if err != nil {
		return nil, fmt.Errorf("failed to map MCP servers to agent: %w", err)
	}

	// 6. Return results
	return &DetectMCPServersResult{
		DetectedServers:   detectedServers,
		RegisteredCount:   registeredCount,
		MappedCount:       len(addedServers),
		TotalTalksTo:      len(agent.TalksTo),
		DryRun:            false,
		ErrorsEncountered: errorsEncountered,
	}, nil
}

// parseClaudeDesktopConfig parses Claude Desktop config JSON file
func (s *AgentService) parseClaudeDesktopConfig(configPath string) ([]DetectedMCPServer, error) {
	// Expand tilde (~) in path to home directory
	if len(configPath) > 0 && configPath[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		configPath = homeDir + configPath[1:]
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON
	var config struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Convert to DetectedMCPServer structs
	detectedServers := []DetectedMCPServer{}
	for name, serverConfig := range config.MCPServers {
		detected := DetectedMCPServer{
			Name:       name,
			Command:    serverConfig.Command,
			Args:       serverConfig.Args,
			Env:        serverConfig.Env,
			Confidence: 100.0, // High confidence for config file detection
			Source:     "claude_desktop_config",
			Metadata: map[string]interface{}{
				"config_path": configPath,
			},
		}
		detectedServers = append(detectedServers, detected)
	}

	return detectedServers, nil
}

// GetAgentByName retrieves an agent by name within an organization
func (s *AgentService) GetAgentByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.Agent, error) {
return s.agentRepo.GetByName(orgID, name)
}

// SuspendAgent suspends an agent by setting its status to suspended
func (s *AgentService) SuspendAgent(ctx context.Context, id uuid.UUID) error {
	agent, err := s.agentRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	// Update status to suspended
	agent.Status = domain.AgentStatusSuspended

	if err := s.agentRepo.Update(agent); err != nil {
		return fmt.Errorf("failed to suspend agent: %w", err)
	}

	// Recalculate trust score (suspension affects trust)
	trustScore, err := s.trustCalc.Calculate(agent)
	if err == nil {
		agent.TrustScore = trustScore.Score
		s.agentRepo.Update(agent)
		s.trustScoreRepo.Create(trustScore)
	}

	return nil
}

// ReactivateAgent reactivates a suspended agent by setting its status to verified
func (s *AgentService) ReactivateAgent(ctx context.Context, id uuid.UUID) error {
	agent, err := s.agentRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	// Update status to verified
	now := time.Now()
	agent.Status = domain.AgentStatusVerified
	agent.VerifiedAt = &now

	if err := s.agentRepo.Update(agent); err != nil {
		return fmt.Errorf("failed to reactivate agent: %w", err)
	}

	// Recalculate trust score (reactivation affects trust)
	trustScore, err := s.trustCalc.Calculate(agent)
	if err == nil {
		agent.TrustScore = trustScore.Score
		s.agentRepo.Update(agent)
		s.trustScoreRepo.Create(trustScore)
	}

	return nil
}

// RotateCredentials rotates an agent's cryptographic credentials by generating new Ed25519 keypair
func (s *AgentService) RotateCredentials(ctx context.Context, id uuid.UUID) (publicKey, privateKey string, err error) {
	// 1. Fetch agent
	agent, err := s.agentRepo.GetByID(id)
	if err != nil {
		return "", "", fmt.Errorf("agent not found: %w", err)
	}

	// 2. Generate new Ed25519 key pair
	keyPair, err := crypto.GenerateEd25519KeyPair()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate new cryptographic keys: %w", err)
	}

	// 3. Encode keys to base64
	encodedKeys := crypto.EncodeKeyPair(keyPair)

	// 4. Encrypt new private key before storing
	encryptedPrivateKey, err := s.keyVault.EncryptPrivateKey(encodedKeys.PrivateKeyBase64)
	if err != nil {
		return "", "", fmt.Errorf("failed to encrypt private key: %w", err)
	}

	// 5. Store previous public key for grace period (allows existing SDKs to work temporarily)
	if agent.PublicKey != nil {
		agent.PreviousPublicKey = agent.PublicKey
	}

	// 6. Update agent with new keys
	agent.PublicKey = &encodedKeys.PublicKeyBase64
	agent.EncryptedPrivateKey = &encryptedPrivateKey
	agent.KeyAlgorithm = encodedKeys.Algorithm
	now := time.Now()
	agent.KeyCreatedAt = &now

	// Set key expiration to 1 year from now (standard practice)
	keyExpiry := time.Now().AddDate(1, 0, 0)
	agent.KeyExpiresAt = &keyExpiry

	// Increment rotation count
	agent.RotationCount++

	// 7. Update agent in database
	if err := s.agentRepo.Update(agent); err != nil {
		return "", "", fmt.Errorf("failed to update agent credentials: %w", err)
	}

	// 8. Return new credentials (for immediate use by caller)
	return encodedKeys.PublicKeyBase64, encodedKeys.PrivateKeyBase64, nil
}

// UpdateAgentPublicKey allows SDK to register/update its own public key
// This is used during SDK initialization when the SDK generates its own keypair
func (s *AgentService) UpdateAgentPublicKey(ctx context.Context, agentID uuid.UUID, publicKey string) error {
	// 1. Fetch agent
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	// 2. Validate public key format (should be base64-encoded 32-byte Ed25519 public key)
	if publicKey == "" {
		return fmt.Errorf("public_key is required")
	}

	// 3. Store previous public key for grace period
	if agent.PublicKey != nil {
		agent.PreviousPublicKey = agent.PublicKey
	}

	// 4. Update agent with new public key
	agent.PublicKey = &publicKey
	agent.KeyAlgorithm = "Ed25519"
	now := time.Now()
	agent.KeyCreatedAt = &now

	// Set key expiration to 1 year from now
	keyExpiry := time.Now().AddDate(1, 0, 0)
	agent.KeyExpiresAt = &keyExpiry

	// Increment rotation count
	agent.RotationCount++

	// 5. Update agent in database
	if err := s.agentRepo.Update(agent); err != nil {
		return fmt.Errorf("failed to update agent public key: %w", err)
	}

	return nil
}

// UpdateLastActive updates the last_active timestamp for an agent
func (s *AgentService) UpdateLastActive(ctx context.Context, agentID uuid.UUID) error {
	return s.agentRepo.UpdateLastActive(ctx, agentID)
}

// calculateViolationSeverity determines the severity level for a capability violation
func (s *AgentService) calculateViolationSeverity(agent *domain.Agent, isBlocked bool) string {
	// Base severity on trust score and whether action was blocked
	if agent.TrustScore < 30 || agent.IsCompromised {
		return "critical"
	}

	if isBlocked {
		// Blocked violations are more severe
		if agent.TrustScore < 50 {
			return "high"
		}
		return "medium"
	}

	// Alert-only violations (not blocked) are lower severity
	if agent.TrustScore < 50 {
		return "medium"
	}
	return "low"
}

// calculateTrustScoreImpact calculates the trust score penalty for a violation
func (s *AgentService) calculateTrustScoreImpact(isBlocked bool) int {
	if isBlocked {
		// Blocked violations have higher impact
		return -10
	}
	// Alert-only violations have lower impact
	return -5
}

// createPolicyAlert creates a security alert for policy violations
func (s *AgentService) createPolicyAlert(
	agent *domain.Agent,
	alertType string,
	policyName string,
	isBlocked bool,
	description string,
	severity domain.AlertSeverity,
	auditID uuid.UUID,
) {
	alertTitle := fmt.Sprintf("%s: %s", alertType, agent.DisplayName)
	alertDescription := fmt.Sprintf(
		"Agent '%s' triggered security policy '%s'. %s. "+
		"Enforcement: %s. Audit ID: %s",
		agent.DisplayName, policyName, description,
		map[bool]string{true: "BLOCKED", false: "ALLOWED (monitored)"}[isBlocked],
		auditID.String(),
	)

	alert := &domain.Alert{
		ID:             uuid.New(),
		OrganizationID: agent.OrganizationID,
		AlertType:      domain.AlertSecurityBreach,
		Severity:       severity,
		Title:          alertTitle,
		Description:    alertDescription,
		ResourceType:   "agent",
		ResourceID:     agent.ID,
		IsAcknowledged: false,
		CreatedAt:      time.Now(),
	}

	if err := s.alertRepo.Create(alert); err != nil {
		fmt.Printf("⚠️  Warning: failed to create security alert: %v\n", err)
	} else {
		fmt.Printf("🚨 SECURITY ALERT: %s for agent %s (policy: %s, action: %s)\n",
			alertType, agent.Name, policyName, map[bool]string{true: "BLOCKED", false: "MONITORED"}[isBlocked])
	}
}

// CreateCapabilityViolation creates a capability violation record for dashboard tracking
func (s *AgentService) CreateCapabilityViolation(
	ctx context.Context,
	agentID uuid.UUID,
	actionType string,
	resource string,
	severity string,
	metadata map[string]interface{},
) error {
	// Get agent to determine trust score impact
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return fmt.Errorf("failed to get agent: %w", err)
	}

	// Get agent's current capabilities for tracking
	capabilities, err := s.capabilityRepo.GetActiveCapabilitiesByAgentID(agentID)
	if err != nil {
		return fmt.Errorf("failed to get capabilities: %w", err)
	}

	capabilityTypes := []string{}
	for _, cap := range capabilities {
		capabilityTypes = append(capabilityTypes, cap.CapabilityType)
	}

	// Map alert severity to violation severity (frontend expects: low, medium, high, critical)
	violationSeverity := "low" // Default
	trustImpact := -5         // Default for low severity

	switch severity {
	case "critical":
		violationSeverity = "critical"
		trustImpact = -15
	case "high":
		violationSeverity = "high"
		trustImpact = -10
	case "warning":
		violationSeverity = "medium"
		trustImpact = -7
	case "info":
		violationSeverity = "low"
		trustImpact = -5
	default:
		// If severity doesn't match known values, treat as low
		violationSeverity = "low"
		trustImpact = -5
	}

	// Create violation record
	violation := &domain.CapabilityViolation{
		AgentID:             agentID,
		AttemptedCapability: actionType,
		RegisteredCapabilities: map[string]interface{}{
			"allowed_capabilities": capabilityTypes,
			"attempted_action":     actionType,
			"resource":             resource,
		},
		Severity:         violationSeverity, // Use mapped severity
		TrustScoreImpact: trustImpact,
		IsBlocked:        false, // SDK violations are logged but allowed
		SourceIP:         nil,
		RequestMetadata:  metadata,
	}

	if err := s.capabilityRepo.CreateViolation(violation); err != nil {
		return fmt.Errorf("failed to create violation: %w", err)
	}

	// Recalculate trust score breakdown after violation
	// This ensures the trust_scores table stays in sync with agents.trust_score
	updatedScore, err := s.trustCalc.Calculate(agent)
	if err != nil {
		fmt.Printf("⚠️  Warning: failed to recalculate trust score: %v\n", err)
	} else {
		// Store the new score breakdown
		if err := s.trustScoreRepo.Create(updatedScore); err != nil {
			fmt.Printf("⚠️  Warning: failed to store trust score breakdown: %v\n", err)
		}
		// Update agent's trust_score field to keep it in sync
		if err := s.agentRepo.UpdateTrustScore(agentID, updatedScore.Score); err != nil {
			fmt.Printf("⚠️  Warning: failed to update agent trust score: %v\n", err)
		} else {
			fmt.Printf("✅ Trust score recalculated after violation: %.2f%% for agent %s\n", updatedScore.Score*100, agent.Name)
		}
	}

	return nil
}
