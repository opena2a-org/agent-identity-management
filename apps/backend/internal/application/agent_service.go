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
	agentRepo      domain.AgentRepository
	trustCalc      domain.TrustScoreCalculator
	trustScoreRepo domain.TrustScoreRepository
	keyVault       *crypto.KeyVault   // ✅ For secure private key storage
	alertRepo      domain.AlertRepository // ✅ For creating security alerts
}

// NewAgentService creates a new agent service
func NewAgentService(
	agentRepo domain.AgentRepository,
	trustCalc domain.TrustScoreCalculator,
	trustScoreRepo domain.TrustScoreRepository,
	keyVault *crypto.KeyVault,
	alertRepo domain.AlertRepository, // ✅ NEW: AlertRepository for security alerts
) *AgentService {
	return &AgentService{
		agentRepo:      agentRepo,
		trustCalc:      trustCalc,
		trustScoreRepo: trustScoreRepo,
		keyVault:       keyVault,
		alertRepo:      alertRepo,
	}
}

// CreateAgentRequest represents agent creation request
type CreateAgentRequest struct {
	Name             string           `json:"name"`
	DisplayName      string           `json:"display_name"`
	Description      string           `json:"description"`
	AgentType        domain.AgentType `json:"agent_type"`
	Version          string           `json:"version"`
	// ✅ REMOVED: PublicKey - AIM generates this automatically
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

	// ✅ AUTOMATIC KEY GENERATION - Zero effort for developers
	// Generate Ed25519 key pair automatically
	keyPair, err := crypto.GenerateEd25519KeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate cryptographic keys: %w", err)
	}

	// Encode keys to base64 for storage
	encodedKeys := crypto.EncodeKeyPair(keyPair)

	// Encrypt private key before storing (NEVER stored in plaintext)
	encryptedPrivateKey, err := s.keyVault.EncryptPrivateKey(encodedKeys.PrivateKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt private key: %w", err)
	}

	// Create agent with auto-generated keys
	agent := &domain.Agent{
		OrganizationID:      orgID,
		Name:                req.Name,
		DisplayName:         req.DisplayName,
		Description:         req.Description,
		AgentType:           req.AgentType,
		Version:             req.Version,
		PublicKey:           &encodedKeys.PublicKeyBase64, // ✅ Stored for verification
		EncryptedPrivateKey: &encryptedPrivateKey,         // ✅ Encrypted storage (never exposed in API)
		KeyAlgorithm:        encodedKeys.Algorithm,        // ✅ "Ed25519"
		CertificateURL:      req.CertificateURL,
		RepositoryURL:       req.RepositoryURL,
		DocumentationURL:    req.DocumentationURL,
		TalksTo:             req.TalksTo,       // MCP servers this agent communicates with
		Capabilities:        req.Capabilities,  // ✅ Store detected capabilities from SDK
		Status:              domain.AgentStatusPending,
		CreatedBy:           userID,
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

		// Recalculate trust score with verified status (verification boosts score)
		updatedTrustScore, err := s.trustCalc.Calculate(agent)
		if err == nil {
			agent.TrustScore = updatedTrustScore.Score
			s.agentRepo.Update(agent)
			s.trustScoreRepo.Create(updatedTrustScore)
			fmt.Printf("✅ Updated trust score after verification: %.2f\n", agent.TrustScore)
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
	if agent.Capabilities == nil || len(agent.Capabilities) == 0 {
		// ⚠️  CRITICAL: If agent has NO capabilities, DENY ALL actions
		// This is the default-deny security posture
		return false, "Agent has no registered capabilities - action denied (capability violation)", auditID, nil
	}

	// 5. Check if requested action matches any registered capability
	hasCapability := false
	for _, capability := range agent.Capabilities {
		if s.matchesCapability(actionType, resource, capability) {
			hasCapability = true
			break
		}
	}

	if !hasCapability {
		// ✅ BLOCK THE ATTACK - Action not in capability list
		// This prevents scope violations like EchoLeak's bulk email access

		// 🚨 CREATE SECURITY ALERT for capability violation
		alert := &domain.Alert{
			ID:             uuid.New(),
			OrganizationID: agent.OrganizationID,
			AlertType:      domain.AlertSecurityBreach, // Security breach / unauthorized action attempt
			Severity:       domain.AlertSeverityHigh,
			Title:          fmt.Sprintf("Capability Violation Detected: %s", agent.DisplayName),
			Description:    fmt.Sprintf("Agent '%s' attempted unauthorized action '%s' which is not in its capability list (allowed: %v). This matches the attack pattern of CVE-2025-32711 (EchoLeak). The action was BLOCKED by AIM's capability-based access control. Audit ID: %s", agent.DisplayName, actionType, agent.Capabilities, auditID.String()),
			ResourceType:   "agent",
			ResourceID:     agentID,
			IsAcknowledged: false,
			CreatedAt:      time.Now(),
		}

		// Save the alert (fire-and-forget - don't fail the verification if alert creation fails)
		if err := s.alertRepo.Create(alert); err != nil {
			fmt.Printf("⚠️  Warning: failed to create security alert for capability violation: %v\n", err)
		} else {
			fmt.Printf("🚨 SECURITY ALERT CREATED: Capability violation detected for agent %s (action: %s)\n", agent.Name, actionType)
		}

		return false, fmt.Sprintf("Capability violation: Agent does not have permission for action '%s' (allowed: %v)", actionType, agent.Capabilities), auditID, nil
	}

	// 6. ✅ ACTION ALLOWED - Agent has proper capability
	return true, "Action matches registered capabilities", auditID, nil
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
	// TODO: Implement proper audit logging
	// For now, we'll just return nil
	// In production, this should:
	// 1. Verify the audit ID exists
	// 2. Update the audit log with the action result
	// 3. Track success/failure metrics
	// 4. Alert on repeated failures

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

			_, err := mcpService.CreateMCPServer(ctx, registerReq, orgID, userID)
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
