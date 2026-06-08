package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// Registration errors returned by CreateAgent for known, client-correctable
// conditions. They are sentinels so HTTP handlers can map them to the right
// status code with errors.Is instead of fragile string comparison. The Error()
// strings double as the safe, user-facing messages (no DB internals leak).
var (
	// ErrAgentNameExists is returned when an agent with the same name already
	// exists in the organization (unique-constraint violation). Maps to 409.
	ErrAgentNameExists = errors.New("an agent with this name already exists")

	// ErrInvalidOrgOrUser is returned when the registration references an
	// organization or user that does not exist (foreign-key violation) — a bad
	// credential or stale token, not a server fault. Maps to 400.
	ErrInvalidOrgOrUser = errors.New("invalid organization or user for this registration")
)

// AgentService handles agent business logic
type AgentService struct {
	agentRepo                domain.AgentRepository
	trustCalc                domain.TrustScoreCalculator
	trustScoreRepo           domain.TrustScoreRepository
	keyVault                 *crypto.KeyVault              // ✅ For secure private key storage
	alertRepo                domain.AlertRepository        // ✅ For creating security alerts
	policyService            *SecurityPolicyService        // ✅ For policy-based enforcement
	capabilityRepo           domain.CapabilityRepository   // ✅ For checking agent capabilities
	verificationEventService *VerificationEventService     // ✅ For creating verification events
	tagRepo                  domain.TagRepository          // ✅ For tagging agents during registration
	userRepo                 domain.UserRepository         // ✅ For looking up user details (audit trail)
	orgRepo                  domain.OrganizationRepository // ✅ For checking enforcement mode
	capabilityRequestService *CapabilityRequestService     // For routing re-registration capability adds through the mode-aware approval workflow (monitoring auto-approves, strict creates pending request)
}

// NewAgentService creates a new agent service
func NewAgentService(
	agentRepo domain.AgentRepository,
	trustCalc domain.TrustScoreCalculator,
	trustScoreRepo domain.TrustScoreRepository,
	keyVault *crypto.KeyVault,
	alertRepo domain.AlertRepository, // ✅ NEW: AlertRepository for security alerts
	policyService *SecurityPolicyService, // ✅ NEW: Security Policy Service
	capabilityRepo domain.CapabilityRepository, // ✅ NEW: CapabilityRepository for capability checks
	verificationEventService *VerificationEventService, // ✅ NEW: For creating verification events
	tagRepo domain.TagRepository, // ✅ NEW: For tagging agents during registration
	userRepo domain.UserRepository, // ✅ NEW: For audit trail (creator/updater info)
	orgRepo domain.OrganizationRepository, // ✅ NEW: For checking enforcement mode
	capabilityRequestService *CapabilityRequestService, // NEW: For mode-aware capability approval workflow on re-registration
) *AgentService {
	return &AgentService{
		agentRepo:                agentRepo,
		trustCalc:                trustCalc,
		trustScoreRepo:           trustScoreRepo,
		keyVault:                 keyVault,
		alertRepo:                alertRepo,
		policyService:            policyService,
		capabilityRepo:           capabilityRepo,
		verificationEventService: verificationEventService,
		tagRepo:                  tagRepo,
		userRepo:                 userRepo,
		orgRepo:                  orgRepo,
		capabilityRequestService: capabilityRequestService,
	}
}

// isValidAgentType checks if the given agent type is valid
func isValidAgentType(agentType domain.AgentType) bool {
	validTypes := map[domain.AgentType]bool{
		// LLM Providers
		domain.AgentTypeClaude:  true,
		domain.AgentTypeGPT:     true,
		domain.AgentTypeGemini:  true,
		domain.AgentTypeLlama:   true,
		domain.AgentTypeMistral: true,
		domain.AgentTypeCohere:  true,
		// Frameworks
		domain.AgentTypeLangChain:      true,
		domain.AgentTypeLlamaIndex:     true,
		domain.AgentTypeAutoGen:        true,
		domain.AgentTypeCrewAI:         true,
		domain.AgentTypeLangGraph:      true,
		domain.AgentTypeHaystack:       true,
		domain.AgentTypeSemanticKernel: true,
		// Copilots & Assistants
		domain.AgentTypeCopilot:   true,
		domain.AgentTypeAssistant: true,
		domain.AgentTypeChatbot:   true,
		// Autonomous
		domain.AgentTypeAutoGPT: true,
		domain.AgentTypeBabyAGI: true,
		// Generic
		domain.AgentTypeCustom: true,
		// Legacy support
		domain.AgentTypeAI: true,
	}
	return validTypes[agentType]
}

// sanitizeTalksTo validates and cleans up TalksTo entries
// - Splits comma-separated values into individual entries
// - Trims whitespace from each entry
// - Removes empty entries
// - Returns a clean slice of MCP server names
func sanitizeTalksTo(talksTo []string) []string {
	if len(talksTo) == 0 {
		return talksTo
	}

	sanitized := make([]string, 0, len(talksTo))
	seen := make(map[string]bool)

	for _, entry := range talksTo {
		// Trim whitespace
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		// Check if entry contains comma (malformed: "memory,aws-terraform" instead of ["memory", "aws-terraform"])
		if strings.Contains(entry, ",") {
			// Split and add individual entries
			parts := strings.Split(entry, ",")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part != "" && !seen[part] {
					sanitized = append(sanitized, part)
					seen[part] = true
					fmt.Printf("⚠️  Sanitized malformed talks_to entry: split '%s' into '%s'\n", entry, part)
				}
			}
		} else {
			// Valid single entry
			if !seen[entry] {
				sanitized = append(sanitized, entry)
				seen[entry] = true
			}
		}
	}

	return sanitized
}

// CreateAgentRequest represents agent creation request
type CreateAgentRequest struct {
	Name             string                 `json:"name"`
	DisplayName      string                 `json:"displayName"`
	Description      string                 `json:"description"`
	AgentType        domain.AgentType       `json:"agentType"`
	Version          string                 `json:"version"`
	PublicKey        string                 `json:"publicKey,omitempty"` // ✅ OPTIONAL: SDK can provide its own public key
	CertificateURL   string                 `json:"certificateUrl"`
	RepositoryURL    string                 `json:"repositoryUrl"`
	DocumentationURL string                 `json:"documentationUrl"`
	TalksTo          []string               `json:"talksTo,omitempty"`      // MCP servers this agent communicates with
	Capabilities     []string               `json:"capabilities,omitempty"` // Agent capabilities
	TagIds           []string               `json:"tagIds,omitempty"`       // ✅ Tags to apply during registration
	Metadata         map[string]interface{} `json:"metadata,omitempty"`     // Custom agent metadata (model, department, etc.)
	// DeclaredPurpose is the optional structured "what is this agent for" declaration
	// (atx-spec core.md §1.5). Identity/attestation + offline detection signal only;
	// never an authorization input.
	DeclaredPurpose *domain.DeclaredPurpose `json:"declaredPurpose,omitempty"`
}

// CreateAgent creates a new agent
// sdkTokenID is optional - if provided, it tracks which SDK token was used to create this agent
// apiKeyID is optional - if provided, it tracks which API key was used to create this agent
// userEmail is optional - used as fallback for audit trail if user lookup fails
// This enables admins to easily revoke compromised SDK tokens or API keys
func (s *AgentService) CreateAgent(ctx context.Context, req *CreateAgentRequest, orgID, userID uuid.UUID, sdkTokenID *uuid.UUID, apiKeyID *uuid.UUID, userEmail string) (*domain.Agent, error) {
	// Validate inputs
	if req.Name == "" || req.DisplayName == "" {
		return nil, fmt.Errorf("name and display_name are required")
	}

	if !isValidAgentType(req.AgentType) {
		return nil, fmt.Errorf("invalid agent_type: %s", req.AgentType)
	}

	// ✅ Validate and sanitize TalksTo entries
	// Reject malformed entries (e.g., comma-separated values in a single string)
	req.TalksTo = sanitizeTalksTo(req.TalksTo)

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
	// ✅ AUTO-VERIFIED: Agents created via authenticated channels (SDK/API/Dashboard)
	// are automatically verified since the creator is already authenticated.
	// This removes friction from the development workflow while maintaining security.
	// Admins can still suspend/revoke agents if needed.
	now := time.Now()
	keyExpiresAt := now.AddDate(1, 0, 0) // Keys expire in 1 year by default

	// ✅ Look up user details for audit trail
	var createdByName, createdByEmail string
	if s.userRepo != nil {
		if user, err := s.userRepo.GetByID(userID); err == nil && user != nil {
			createdByName = user.Name
			createdByEmail = user.Email
		}
	}
	// ✅ Fallback: Use email from JWT claims if user lookup failed
	if createdByEmail == "" && userEmail != "" {
		createdByEmail = userEmail
	}

	agent := &domain.Agent{
		OrganizationID:      orgID,
		Name:                req.Name,
		DisplayName:         req.DisplayName,
		Description:         req.Description,
		AgentType:           req.AgentType,
		Version:             req.Version,
		PublicKey:           &publicKeyBase64, // ✅ Stored for verification (SDK-provided or generated)
		KeyAlgorithm:        keyAlgorithm,     // ✅ "Ed25519"
		KeyCreatedAt:        &now,             // ✅ Track when key was created
		KeyExpiresAt:        &keyExpiresAt,    // ✅ Keys expire in 1 year
		CertificateURL:      req.CertificateURL,
		RepositoryURL:       req.RepositoryURL,
		DocumentationURL:    req.DocumentationURL,
		TalksTo:             req.TalksTo,      // MCP servers this agent communicates with
		Capabilities:        req.Capabilities, // ✅ Store detected capabilities from SDK
		Metadata:            req.Metadata,     // ✅ Custom metadata (model, department, owner, etc.)
		DeclaredPurpose:     req.DeclaredPurpose,
		Status:              domain.AgentStatusPending, // Agents start as pending, verification is earned
		CreatedBy:           userID,
		CreatedByName:       createdByName,   // ✅ Denormalized for audit trail
		CreatedByEmail:      createdByEmail,  // ✅ Denormalized for audit trail
		CreatedBySDKTokenID: sdkTokenID,      // ✅ Track SDK token for easy revocation if compromised
		CreatedByAPIKeyID:   apiKeyID,        // ✅ Track API key for easy revocation if compromised
	}

	// Only set encrypted private key if we generated it server-side
	if encryptedPrivateKey != "" {
		agent.EncryptedPrivateKey = &encryptedPrivateKey // ✅ Encrypted storage (never exposed in API)
	}

	// Validate the optional declared purpose (atx-spec core.md §1.5). Hard
	// structural errors reject the request; coherence issues are issuance-time
	// hygiene logged warn-only during the optional phase. declaredPurpose is never
	// an authorization input — this is identity validation + a detection signal.
	if agent.DeclaredPurpose != nil {
		if err := agent.DeclaredPurpose.Validate(agent.Capabilities); err != nil {
			return nil, fmt.Errorf("invalid declaredPurpose: %w", err)
		}
		for _, w := range agent.DeclaredPurpose.CoherenceWarnings(agent.Capabilities) {
			fmt.Printf("Warning: agent %q declaredPurpose coherence: %s\n", agent.Name, w)
		}
	}

	if err := s.agentRepo.Create(agent); err != nil {
		// Map known database constraint violations to safe, user-facing messages.
		// Raw PostgreSQL driver errors (the `pq:` prefix, table/constraint names like
		// `agents_organization_id_fkey`) must never reach the API response — surfacing
		// them leaks the schema and is an information-disclosure bug.
		msg := err.Error()
		switch {
		case strings.Contains(msg, "duplicate key value"), strings.Contains(msg, "unique constraint"):
			return nil, ErrAgentNameExists
		case strings.Contains(msg, "foreign key constraint"):
			return nil, ErrInvalidOrgOrUser
		case strings.Contains(msg, "pq:"):
			// Any other PostgreSQL driver error: log the detail server-side, return generic.
			fmt.Printf("agent creation failed (database error): %v\n", err)
			return nil, fmt.Errorf("failed to create agent")
		default:
			return nil, fmt.Errorf("failed to create agent: %w", err)
		}
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

	// AUTO-VERIFICATION: Only in monitoring mode. In strict mode, agents must be manually verified.
	// This ensures strict environments require explicit approval before agents can operate.
	var isStrictMode bool
	if s.orgRepo != nil {
		org, orgErr := s.orgRepo.GetByID(orgID)
		isStrictMode = orgErr == nil && org != nil && org.EnforcementMode == domain.EnforcementModeStrict
	}
	shouldAutoVerify := !isStrictMode && s.shouldAutoVerifyAgent(agent)
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

	// First-registration baseline. Declared capabilities are granted directly
	// in BOTH monitoring and strict mode because first registration IS the
	// baseline declaration moment: the trust boundary is the authenticated
	// user who has permission to create the agent, not the capability list
	// they declare for it. The strict-mode protection kicks in on
	// re-registration (AgentService.UpdateAgent), which routes new caps
	// through CapabilityRequestService for admin approval.
	if len(req.Capabilities) > 0 {
		grantedCount := 0
		for _, capabilityType := range req.Capabilities {
			capabilityRecord := &domain.AgentCapability{
				AgentID:        agent.ID,
				CapabilityType: capabilityType,
				GrantedBy:      &userID,
				GrantedAt:      time.Now(),
			}

			if err := s.capabilityRepo.CreateCapability(capabilityRecord); err != nil {
				fmt.Printf("⚠️  Warning: failed to auto-grant capability '%s': %v\n", capabilityType, err)
			} else {
				grantedCount++
			}
		}

		if grantedCount > 0 {
			fmt.Printf("✅ Granted %d baseline capabilities for agent %s: %v\n", grantedCount, agent.Name, req.Capabilities)
		}
	}

	// ✅ AUTO-APPLY TAGS: Apply tags during registration (no separate API call needed)
	// Supports both tag UUIDs and tag names (keys). If a name is provided, it will
	// find an existing tag or create a new one automatically.
	if len(req.TagIds) > 0 && s.tagRepo != nil {
		tagUUIDs := make([]uuid.UUID, 0, len(req.TagIds))
		for _, tagIDStr := range req.TagIds {
			// Try to parse as UUID first
			tagID, err := uuid.Parse(tagIDStr)
			if err == nil {
				// It's a valid UUID, use it directly
				tagUUIDs = append(tagUUIDs, tagID)
				continue
			}

			// Not a UUID - treat as tag name/key. Find existing or create new tag.
			existingTags, searchErr := s.tagRepo.SearchTags(ctx, orgID, tagIDStr, nil)
			if searchErr != nil {
				fmt.Printf("⚠️  Warning: failed to search for tag '%s': %v\n", tagIDStr, searchErr)
				continue
			}

			// Check if we found an exact match by key
			var foundTag *domain.Tag
			for _, t := range existingTags {
				if t.Key == tagIDStr {
					foundTag = t
					break
				}
			}

			if foundTag != nil {
				// Found existing tag by name
				tagUUIDs = append(tagUUIDs, foundTag.ID)
			} else {
				// Create new tag with this name
				newTag := &domain.Tag{
					OrganizationID: orgID,
					Key:            tagIDStr,
					Value:          tagIDStr, // Use key as value for simple tags
					Category:       domain.TagCategoryCustom,
					Description:    fmt.Sprintf("Auto-created tag from SDK registration"),
					Color:          "#6b7280", // Gray color for auto-created tags
					CreatedBy:      userID,
				}
				if createErr := s.tagRepo.Create(ctx, newTag); createErr != nil {
					fmt.Printf("⚠️  Warning: failed to create tag '%s': %v\n", tagIDStr, createErr)
					continue
				}
				tagUUIDs = append(tagUUIDs, newTag.ID)
				fmt.Printf("✅ Auto-created tag '%s' for agent %s\n", tagIDStr, agent.Name)
			}
		}

		if len(tagUUIDs) > 0 {
			if err := s.tagRepo.AddTagsToAgent(ctx, agent.ID, tagUUIDs); err != nil {
				fmt.Printf("⚠️  Warning: failed to apply tags to agent %s: %v\n", agent.Name, err)
			} else {
				fmt.Printf("✅ Applied %d tags to agent %s\n", len(tagUUIDs), agent.Name)
			}
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

// UpdateAgent updates an agent. requestedBy is the authenticated user ID
// driving the update; used as the RequestedBy on any capability_requests
// rows created for new capability declarations in strict mode. A zero UUID
// is acceptable for system-initiated paths but should be avoided for SDK /
// dashboard calls where the user identity is known.
func (s *AgentService) UpdateAgent(ctx context.Context, id uuid.UUID, req *CreateAgentRequest, requestedBy uuid.UUID) (*domain.Agent, error) {
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
	if req.AgentType != "" {
		agent.AgentType = req.AgentType
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
	// Update metadata
	if req.Metadata != nil {
		agent.Metadata = req.Metadata
	}
	// Update the optional declared purpose (atx-spec core.md §1.5). Validate
	// against the effective granted capabilities (the incoming set if the request
	// also changes capabilities, otherwise the stored set). Hard errors reject;
	// coherence issues are warn-only hygiene. Never an authorization input.
	if req.DeclaredPurpose != nil {
		agent.DeclaredPurpose = req.DeclaredPurpose
		grantedCaps := agent.Capabilities
		if len(req.Capabilities) > 0 {
			grantedCaps = req.Capabilities
		}
		if err := agent.DeclaredPurpose.Validate(grantedCaps); err != nil {
			return nil, fmt.Errorf("invalid declaredPurpose: %w", err)
		}
		for _, w := range agent.DeclaredPurpose.CoherenceWarnings(grantedCaps) {
			fmt.Printf("Warning: agent %q declaredPurpose coherence: %s\n", agent.Name, w)
		}
	}

	if err := s.agentRepo.Update(agent); err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	if req.Capabilities != nil && len(req.Capabilities) > 0 {
		// Get current capabilities
		currentCaps, err := s.capabilityRepo.GetCapabilitiesByAgentID(id)
		if err != nil {
			fmt.Printf("Warning: failed to get current capabilities: %v\n", err)
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

		// Check organization enforcement mode
		var enforcementMode domain.EnforcementMode = domain.EnforcementModeStrict // Default to strict
		if s.orgRepo != nil {
			org, orgErr := s.orgRepo.GetByID(agent.OrganizationID)
			if orgErr == nil && org != nil {
				enforcementMode = org.EnforcementMode
			}
		}

		// Identify new capabilities (potential escalations)
		newCaps := make([]string, 0)
		for _, capType := range req.Capabilities {
			if _, exists := currentCapTypes[capType]; !exists {
				newCaps = append(newCaps, capType)
			}
		}

		// Route every new (re-registration-declared) capability through
		// CapabilityRequestService.CreateRequest, which encapsulates the
		// mode-aware approval policy in one place:
		//
		//   monitoring mode -> creates a capability_requests row with
		//     status=auto_approved AND grants the capability. Audit trail
		//     preserved.
		//
		//   strict mode -> creates a capability_requests row with
		//     status=pending. Capability is NOT granted; admin must approve
		//     via /api/v1/capability-requests/:id/approve before the agent
		//     can use it. Agent's verify_capability calls for the requested
		//     cap will DENY until approval.
		//
		// capabilityRequestService is REQUIRED for this branch to execute.
		// Production wiring (cmd/server/main.go) always injects it; if a
		// caller constructs AgentService without it AND tries to add new
		// caps, we fail loudly rather than silently fall back to a
		// strict-mode-bypassing direct grant.
		if len(newCaps) > 0 {
			if s.capabilityRequestService == nil {
				return nil, fmt.Errorf("capabilityRequestService is required to add new capabilities to existing agents; received nil (agent=%s, new_caps=%v)", agent.Name, newCaps)
			}
			for _, capType := range newCaps {
				input := &domain.CreateCapabilityRequestInput{
					AgentID:        id,
					CapabilityType: capType,
					Reason:         fmt.Sprintf("Re-registration declared new capability for agent %s.", agent.Name),
					RequestedBy:    requestedBy,
				}
				createdRequest, err := s.capabilityRequestService.CreateRequest(ctx, input)
				if err != nil {
					fmt.Printf("Warning: failed to create capability request for '%s' (agent %s, mode=%s): %v\n",
						capType, agent.Name, enforcementMode, err)
					continue
				}
				// High-signal log line for SIEM rules. Distinguishes
				// monitoring auto-approve (already-granted) from strict
				// pending (admin action required).
				if createdRequest != nil && createdRequest.Status == domain.CapabilityRequestStatusPending {
					fmt.Printf("SECURITY: agent %s (%s) re-registration declared new capability '%s' in strict mode; pending admin approval (request_id=%s, requested_by=%s)\n",
						agent.Name, id, capType, createdRequest.ID, requestedBy)
				}
			}
		}

		// Capability removal is always allowed (safe direction - reducing privileges)
		revokedCaps := make([]string, 0)
		for capType, cap := range currentCapTypes {
			if !requestedCapTypes[capType] {
				now := time.Now()
				if err := s.capabilityRepo.RevokeCapability(cap.ID, now); err != nil {
					fmt.Printf("Warning: failed to revoke capability '%s': %v\n", capType, err)
				} else {
					revokedCaps = append(revokedCaps, capType)
				}
			}
		}

		if len(revokedCaps) > 0 {
			fmt.Printf("Agent %s: revoked capabilities %v (self-requested reduction)\n", agent.Name, revokedCaps)
		}
	}
	// Recalculate trust score. Guarded for test setups that construct
	// AgentService without a trust calculator; production wiring always
	// injects one.
	if s.trustCalc != nil {
		trustScore, err := s.trustCalc.Calculate(agent)
		if err == nil {
			agent.TrustScore = trustScore.Score
			s.agentRepo.Update(agent)
			if s.trustScoreRepo != nil {
				s.trustScoreRepo.Create(trustScore)
			}
		}
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

	previousScore := agent.TrustScore

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

	// Evaluate trust score policy enforcement (alerts and suspension)
	if _, err := s.policyService.EvaluateTrustScoreOnUpdate(ctx, agent, previousScore, trustScore.Score); err != nil {
		fmt.Printf("⚠️  Trust score policy evaluation failed: %v\n", err)
	}

	return trustScore, nil
}

// UpdateTrustScore manually updates an agent's trust score (admin override)
func (s *AgentService) UpdateTrustScore(ctx context.Context, agentID uuid.UUID, newScore float64) error {
	// Validate score range (0.0 to 100.0 matching trust calculator output and DB schema DECIMAL(5,2))
	if newScore < 0.0 || newScore > 100.0 {
		return fmt.Errorf("trust score must be between 0.0 and 100.0")
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

	// Update agent object for policy evaluation
	agent.TrustScore = newScore

	// Check for significant trust score drop and create alert if needed
	s.checkAndCreateTrustScoreDropAlert(ctx, agent, previousScore, newScore)

	// Evaluate trust score policy enforcement (alerts and suspension)
	if _, err := s.policyService.EvaluateTrustScoreOnUpdate(ctx, agent, previousScore, newScore); err != nil {
		fmt.Printf("⚠️  Trust score policy evaluation failed: %v\n", err)
	}

	return nil
}

// checkAndCreateTrustScoreDropAlert checks for significant trust score drops and creates alerts
func (s *AgentService) checkAndCreateTrustScoreDropAlert(ctx context.Context, agent *domain.Agent, previousScore, currentScore float64) {
	// Configuration thresholds
	const (
		significantDropThreshold = 0.1 // 10% drop triggers warning
		criticalDropThreshold    = 0.2 // 20% drop triggers critical
		lowScoreThreshold        = 0.5 // 50% trust score threshold
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
			AgentName:      agentName, // Denormalized for display in alerts
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
			AgentName:      agentName, // Denormalized for display in alerts
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
func (s *AgentService) HasCapability(ctx context.Context, agentID uuid.UUID, capabilityToCheck string, resource string) (bool, error) {
	// Get agent's active capabilities
	capabilities, err := s.capabilityRepo.GetActiveCapabilitiesByAgentID(agentID)
	if err != nil {
		return false, fmt.Errorf("failed to get capabilities: %w", err)
	}

	// If agent has no capabilities, return false
	if len(capabilities) == 0 {
		return false, nil
	}

	// Check if capability matches any granted capability
	for _, cap := range capabilities {
		if s.matchesCapability(capabilityToCheck, resource, cap.CapabilityType) {
			return true, nil
		}
	}

	return false, nil
}

// VerifyCapability verifies if an agent can use a capability
// ✅ CRITICAL SECURITY FUNCTION - EchoLeak Prevention
// This is the core defense mechanism that prevented CVE-2025-32711 (EchoLeak) attack
func (s *AgentService) VerifyCapability(
	ctx context.Context,
	agentID uuid.UUID,
	capability string,
	resource string,
	metadata map[string]interface{},
	sourceIP string,
) (allowed bool, reason string, auditID uuid.UUID, err error) {
	auditID = uuid.New()

	// 1. Fetch agent
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return false, "Agent not found", uuid.Nil, err
	}

	// 2. Check agent status - MUST be verified (unless MONITORING mode)
	if agent.Status != domain.AgentStatusVerified {
		// Check enforcement mode before blocking
		enforcementMode := domain.EnforcementModeMonitoring // Default to monitoring
		if s.orgRepo != nil {
			org, orgErr := s.orgRepo.GetByID(agent.OrganizationID)
			if orgErr == nil && org != nil {
				enforcementMode = org.EnforcementMode
			}
		}

		if enforcementMode == domain.EnforcementModeMonitoring {
			fmt.Printf("✅ MONITORING MODE: Allowing action '%s' for unverified agent %s (status: %s) - logged for review\n",
				capability, agent.Name, agent.Status)

			// Log violation for visibility
			violation := &domain.CapabilityViolation{
				AgentID:             agentID,
				AttemptedCapability: capability,
				RegisteredCapabilities: map[string]interface{}{
					"attemptedCapability": capability,
					"resource":            resource,
					"enforcementMode":     "monitoring",
					"agentStatus":         string(agent.Status),
					"reason":              "agent not verified - allowed by monitoring mode",
				},
				Severity:         "medium",
				TrustScoreImpact: -2,
				IsBlocked:        false,
				SourceIP:         func() *string { if sourceIP != "" { return &sourceIP }; return nil }(),
				RequestMetadata:  metadata,
			}
			if err := s.capabilityRepo.CreateViolation(violation); err != nil {
				fmt.Printf("⚠️  Warning: failed to create monitoring violation record: %v\n", err)
			}

			// Create alert so admin sees unverified agent activity
			alertTitle := fmt.Sprintf("Unverified Agent Action (Monitoring Mode): %s", agent.DisplayName)
			alertDescription := fmt.Sprintf(
				"Agent '%s' (status: %s) used capability '%s' without being verified. "+
					"Enforcement mode: MONITORING. Action was ALLOWED but logged. Audit ID: %s",
				agent.DisplayName, agent.Status, capability, auditID.String(),
			)
			alert := &domain.Alert{
				ID:             uuid.New(),
				OrganizationID: agent.OrganizationID,
				AlertType:      domain.AlertSecurityBreach,
				Severity:       domain.AlertSeverityWarning,
				Title:          alertTitle,
				Description:    alertDescription,
				ResourceType:   "agent",
				ResourceID:     agentID,
				AgentName:      agent.DisplayName,
				SourceIP:       sourceIP,
				IsAcknowledged: false,
				CreatedAt:      time.Now(),
			}
			if err := s.alertRepo.Create(alert); err != nil {
				fmt.Printf("⚠️  Warning: failed to create monitoring alert: %v\n", err)
			}

			return true, fmt.Sprintf(
				"Action allowed by MONITORING mode (agent status: %s, not yet verified) - logged for review",
				agent.Status,
			), auditID, nil
		}

		// STRICT mode: deny unverified agents
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

	for _, cap := range activeCapabilities {
		capabilityTypes = append(capabilityTypes, cap.CapabilityType)
		if s.matchesCapability(capability, resource, cap.CapabilityType) {
			hasCapability = true
		}
	}

	// ⚠️  Check enforcement mode for agents with NO GRANTED capabilities
	// In MONITORING mode: Allow actions but log violations
	// In STRICT mode: Block actions (require capabilities to be granted first)
	if len(capabilityTypes) == 0 {
		// 🔍 Get organization enforcement mode
		enforcementMode := domain.EnforcementModeMonitoring // Default to monitoring (lenient)
		if s.orgRepo != nil {
			org, err := s.orgRepo.GetByID(agent.OrganizationID)
			if err == nil && org != nil {
				enforcementMode = org.EnforcementMode
			}
		}

		// 🛡️ Also check security policies for capability violations
		// Note: In MONITORING mode, we ignore shouldBlock and always allow (alert only)
		_, shouldAlert, policyName, policyErr := s.policyService.EvaluateCapabilityViolation(
			ctx, agent, capability, resource, auditID,
		)
		if policyErr != nil {
			fmt.Printf("⚠️  Policy evaluation failed for no-capabilities check: %v\n", policyErr)
			// Policy failed - use enforcement mode as fallback
			shouldAlert = true
			policyName = "enforcement_mode_fallback"
		}

		// In MONITORING mode, ALWAYS allow the action (policies only affect alerting, not blocking)
		// This is the key difference: MONITORING mode overrides individual policy blocking decisions
		// If you want to block, switch to STRICT mode in Global Enforcement settings
		if enforcementMode == domain.EnforcementModeMonitoring {
			fmt.Printf("✅ MONITORING MODE: Allowing action '%s' for agent %s (no granted capabilities) - will be logged\n",
				capability, agent.Name)
			fmt.Printf("   Policy '%s' would block in STRICT mode, but MONITORING mode allows all actions\n", policyName)

			// Still log the violation for visibility (non-blocking)
			violation := &domain.CapabilityViolation{
				AgentID:             agentID,
				AttemptedCapability: capability,
				RegisteredCapabilities: map[string]interface{}{
					"allowedCapabilities": []string{}, // No capabilities granted
					"attemptedCapability": capability,
					"resource":            resource,
					"enforcementMode":     "monitoring",
				},
				Severity:         "medium", // Lower severity in monitoring mode
				TrustScoreImpact: -2,       // Minimal impact in monitoring mode
				IsBlocked:        false,    // NOT blocked
				SourceIP:         func() *string { if sourceIP != "" { return &sourceIP }; return nil }(),
				RequestMetadata:  metadata,
			}

			if err := s.capabilityRepo.CreateViolation(violation); err != nil {
				fmt.Printf("⚠️  Warning: failed to create monitoring violation record: %v\n", err)
			}

			// Create alert if policy requires it
			if shouldAlert {
				alertTitle := fmt.Sprintf("Unregistered Capability Used (Monitoring Mode): %s", agent.DisplayName)
				alertDescription := fmt.Sprintf(
					"Agent '%s' used capability '%s' without pre-registration. "+
						"No capabilities granted to this agent. Enforcement mode: MONITORING. "+
						"Action was ALLOWED but logged. Policy: %s. Audit ID: %s",
					agent.DisplayName, capability, policyName, auditID.String(),
				)
				alert := &domain.Alert{
					ID:             uuid.New(),
					OrganizationID: agent.OrganizationID,
					AlertType:      domain.AlertSecurityBreach,
					Severity:       domain.AlertSeverityWarning,
					Title:          alertTitle,
					Description:    alertDescription,
					ResourceType:   "agent",
					ResourceID:     agentID,
					AgentName:      agent.DisplayName,
					SourceIP:       sourceIP,
					IsAcknowledged: false,
					CreatedAt:      time.Now(),
				}
				if err := s.alertRepo.Create(alert); err != nil {
					fmt.Printf("⚠️  Warning: failed to create monitoring alert: %v\n", err)
				}
			}

			return true, fmt.Sprintf(
				"Action allowed by MONITORING mode (no capabilities registered yet) - logged for review. Policy: %s",
				policyName,
			), auditID, nil
		}

		// STRICT mode or policy says block - deny the action
		// 📝 Record violation and increment count for tracking
		violation := &domain.CapabilityViolation{
			AgentID:             agentID,
			AttemptedCapability: capability,
			RegisteredCapabilities: map[string]interface{}{
				"allowedCapabilities": []string{}, // No capabilities granted
				"attemptedCapability": capability,
				"resource":            resource,
				"enforcementMode":     string(enforcementMode),
			},
			Severity:         "high", // No capabilities = serious issue
			TrustScoreImpact: -10,    // Standard violation impact
			IsBlocked:        true,
			SourceIP:         func() *string { if sourceIP != "" { return &sourceIP }; return nil }(),
			RequestMetadata:  metadata,
		}

		if err := s.capabilityRepo.CreateViolation(violation); err != nil {
			fmt.Printf("⚠️  Warning: failed to create violation record: %v\n", err)
		} else {
			fmt.Printf("📝 VIOLATION RECORDED: Agent %s attempted %s with no granted capabilities (mode: %s)\n",
				agent.Name, capability, enforcementMode)
		}

		// capability_violation_count is bumped by an AFTER INSERT trigger
		// on capability_violations (migration 091); no application-layer
		// increment needed.

		return false, fmt.Sprintf(
			"Agent has no granted capabilities - action denied by %s mode (admin must grant capabilities first)",
			enforcementMode,
		), auditID, nil
	}

	if !hasCapability {
		// ✅ CAPABILITY VIOLATION DETECTED - Evaluate security policies
		// This prevents scope violations like EchoLeak's bulk email access

		// 🔍 Get organization enforcement mode (check if MONITORING overrides)
		orgEnforcementMode := domain.EnforcementModeMonitoring // Default to monitoring
		if s.orgRepo != nil {
			org, err := s.orgRepo.GetByID(agent.OrganizationID)
			if err == nil && org != nil {
				orgEnforcementMode = org.EnforcementMode
			}
		}

		// 🛡️ Evaluate security policies to determine enforcement action
		shouldBlock, shouldAlert, policyName, err := s.policyService.EvaluateCapabilityViolation(
			ctx, agent, capability, resource, auditID,
		)
		if err != nil {
			// Policy evaluation failed - use safe default (block + alert)
			fmt.Printf("⚠️  Policy evaluation failed: %v, using safe default (block + alert)\n", err)
			shouldBlock = true
			shouldAlert = true
			policyName = "default_policy"
		}

		// 🔄 MONITORING MODE OVERRIDE: In monitoring mode, never block, only alert
		// This allows all actions to proceed for observation purposes
		if orgEnforcementMode == domain.EnforcementModeMonitoring && shouldBlock {
			fmt.Printf("✅ MONITORING MODE OVERRIDE: Policy '%s' would block '%s' but MONITORING mode allows all actions\n",
				policyName, capability)
			shouldBlock = false // Override to allow
			shouldAlert = true  // Still alert for visibility
		}

		// 🚨 CREATE SECURITY ALERT if policy requires it
		if shouldAlert {
			alertTitle := fmt.Sprintf("Capability Violation Detected: %s", agent.DisplayName)
			alertDescription := fmt.Sprintf(
				"Agent '%s' attempted unauthorized capability '%s' which is not in its capability list (allowed: %v). "+
					"This matches the attack pattern of CVE-2025-32711 (EchoLeak). "+
					"Security Policy '%s' enforcement: %s. Audit ID: %s",
				agent.DisplayName, capability, capabilityTypes, policyName,
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
				AgentName:      agent.DisplayName, // Denormalized for display in alerts
				SourceIP:       sourceIP,
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
			AttemptedCapability: capability,
			RegisteredCapabilities: map[string]interface{}{
				"allowedCapabilities":  capabilityTypes,
				"attemptedCapability":  capability,
				"resource":             resource,
			},
			Severity:         s.calculateViolationSeverity(agent, shouldBlock),
			TrustScoreImpact: s.calculateTrustScoreImpact(shouldBlock),
			IsBlocked:        shouldBlock,
			SourceIP:         func() *string { if sourceIP != "" { return &sourceIP }; return nil }(),
			RequestMetadata:  metadata,
		}

		if err := s.capabilityRepo.CreateViolation(violation); err != nil {
			fmt.Printf("⚠️  Warning: failed to create violation record: %v\n", err)
		} else {
			fmt.Printf("📝 VIOLATION RECORDED: Agent %s attempted %s (blocked: %v)\n",
				agent.Name, capability, shouldBlock)
		}

		// ✅ APPLY TRUST SCORE IMPACT directly from violation
		// The trust_score_impact is a percentage to subtract (e.g., -10 means subtract 10%)
		// This provides immediate, cumulative impact from violations
		impactPercent := float64(violation.TrustScoreImpact) / 100.0 // Convert -10 to -0.10
		newScore := agent.TrustScore + impactPercent                 // Subtract (impact is negative)

		// Ensure score stays within bounds [0, 1]
		if newScore < 0 {
			newScore = 0
		} else if newScore > 1 {
			newScore = 1
		}

		// Update agent's trust score in database
		if err := s.agentRepo.UpdateTrustScore(agentID, newScore); err != nil {
			fmt.Printf("⚠️  Warning: failed to update agent trust score: %v\n", err)
		} else {
			fmt.Printf("✅ Trust score updated after violation: %.2f%% → %.2f%% (impact: %d%%) for agent %s\n",
				agent.TrustScore*100, newScore*100, violation.TrustScoreImpact, agent.Name)
		}

		// Also update trust_scores table to keep it in sync with agents.trust_score
		// This ensures the Trust Score tab shows the correct value
		if err := s.trustScoreRepo.UpdateScore(agentID, newScore); err != nil {
			fmt.Printf("⚠️  Warning: failed to update trust_scores table: %v\n", err)
		}

		// capability_violation_count is bumped by an AFTER INSERT trigger
		// on capability_violations (migration 091); no application-layer
		// increment needed.

		// Evaluate trust score policy enforcement (alerts and suspension)
		previousScore := agent.TrustScore  // Store before updating
		agent.TrustScore = newScore        // Update agent object for policy evaluation
		if _, err := s.policyService.EvaluateTrustScoreOnUpdate(ctx, agent, previousScore, newScore); err != nil {
			fmt.Printf("⚠️  Trust score policy evaluation failed: %v\n", err)
		}

		// Return enforcement decision from policy
		if shouldBlock {
			return false, fmt.Sprintf(
				"Capability violation blocked by security policy '%s': Agent does not have permission for capability '%s' (allowed: %v)",
				policyName, capability, capabilityTypes,
			), auditID, nil
		} else {
			// Policy says alert-only mode - allow the action but log it
			fmt.Printf("⚠️  Capability violation ALLOWED by policy '%s' (alert-only mode): %s attempting %s\n",
				policyName, agent.Name, capability)
			return true, fmt.Sprintf(
				"Action allowed by security policy '%s' (alert-only mode) - capability violation logged",
				policyName,
			), auditID, nil
		}
	}

	// 6. ✅ CAPABILITY CHECK PASSED - Now evaluate additional security policies
	// Even if agent has the capability, we still need to check other policy types

	// 🔍 Get organization enforcement mode for policy overrides
	policyEnforcementMode := domain.EnforcementModeMonitoring // Default to monitoring
	if s.orgRepo != nil {
		org, err := s.orgRepo.GetByID(agent.OrganizationID)
		if err == nil && org != nil {
			policyEnforcementMode = org.EnforcementMode
		}
	}

	// 6.1 Trust Score Policy Evaluation
	trustScoreBlocked, trustScoreAlert, trustScorePolicyName, err := s.policyService.EvaluateTrustScoreLow(
		ctx, agent, capability, resource, auditID,
	)
	if err != nil {
		fmt.Printf("⚠️  Trust score policy evaluation failed: %v\n", err)
	}
	if trustScoreAlert {
		s.createPolicyAlert(agent, "Trust Score Low", trustScorePolicyName, trustScoreBlocked,
			fmt.Sprintf("Agent has low trust score (%.2f)", agent.TrustScore), domain.AlertSeverityWarning, auditID)
	}
	// Apply MONITORING mode override
	if trustScoreBlocked && policyEnforcementMode == domain.EnforcementModeMonitoring {
		fmt.Printf("✅ MONITORING MODE: Trust score policy '%s' would block but allowing (mode: %s)\n",
			trustScorePolicyName, policyEnforcementMode)
		trustScoreBlocked = false
	}
	if trustScoreBlocked {
		return false, fmt.Sprintf(
			"Action blocked by trust score policy '%s': Agent trust score too low (%.2f)",
			trustScorePolicyName, agent.TrustScore,
		), auditID, nil
	}

	// 6.2 Data Exfiltration Policy Evaluation
	exfilBlocked, exfilAlert, exfilPolicyName, err := s.policyService.EvaluateDataExfiltration(
		ctx, agent, capability, resource, auditID,
	)
	if err != nil {
		fmt.Printf("⚠️  Data exfiltration policy evaluation failed: %v\n", err)
	}
	if exfilAlert {
		s.createPolicyAlert(agent, "Data Exfiltration Attempt", exfilPolicyName, exfilBlocked,
			fmt.Sprintf("Suspected data exfiltration pattern detected: %s on %s", capability, resource),
			domain.AlertSeverityCritical, auditID)
	}
	// Apply MONITORING mode override
	if exfilBlocked && policyEnforcementMode == domain.EnforcementModeMonitoring {
		fmt.Printf("✅ MONITORING MODE: Data exfiltration policy '%s' would block but allowing (mode: %s)\n",
			exfilPolicyName, policyEnforcementMode)
		exfilBlocked = false
	}
	if exfilBlocked {
		return false, fmt.Sprintf(
			"Action blocked by data exfiltration policy '%s': Suspicious pattern detected",
			exfilPolicyName,
		), auditID, nil
	}

	// 6.3 Unusual Activity Policy Evaluation (stub - needs historical data)
	unusualBlocked, unusualAlert, unusualPolicyName, err := s.policyService.EvaluateUnusualActivity(
		ctx, agent, capability, resource, auditID,
	)
	if err != nil {
		fmt.Printf("⚠️  Unusual activity policy evaluation failed: %v\n", err)
	}
	if unusualAlert {
		s.createPolicyAlert(agent, "Unusual Activity", unusualPolicyName, unusualBlocked,
			"Anomalous behavior pattern detected", domain.AlertSeverityWarning, auditID)
	}
	// Apply MONITORING mode override
	if unusualBlocked && policyEnforcementMode == domain.EnforcementModeMonitoring {
		fmt.Printf("✅ MONITORING MODE: Unusual activity policy '%s' would block but allowing (mode: %s)\n",
			unusualPolicyName, policyEnforcementMode)
		unusualBlocked = false
	}
	if unusualBlocked {
		return false, fmt.Sprintf(
			"Action blocked by unusual activity policy '%s'",
			unusualPolicyName,
		), auditID, nil
	}

	// 6.4 Config Drift Policy Evaluation (stub - needs baseline)
	driftBlocked, driftAlert, driftPolicyName, err := s.policyService.EvaluateConfigDrift(
		ctx, agent, capability, resource, auditID,
	)
	if err != nil {
		fmt.Printf("⚠️  Config drift policy evaluation failed: %v\n", err)
	}
	if driftAlert {
		s.createPolicyAlert(agent, "Configuration Drift", driftPolicyName, driftBlocked,
			"Agent configuration has drifted from baseline", domain.AlertSeverityWarning, auditID)
	}
	// Apply MONITORING mode override
	if driftBlocked && policyEnforcementMode == domain.EnforcementModeMonitoring {
		fmt.Printf("✅ MONITORING MODE: Config drift policy '%s' would block but allowing (mode: %s)\n",
			driftPolicyName, policyEnforcementMode)
		driftBlocked = false
	}
	if driftBlocked {
		return false, fmt.Sprintf(
			"Action blocked by config drift policy '%s'",
			driftPolicyName,
		), auditID, nil
	}

	// 6.5 Unauthorized Access Policy Evaluation (stub)
	unauthBlocked, unauthAlert, unauthPolicyName, err := s.policyService.EvaluateUnauthorizedAccess(
		ctx, agent, capability, resource, auditID,
	)
	if err != nil {
		fmt.Printf("⚠️  Unauthorized access policy evaluation failed: %v\n", err)
	}
	if unauthAlert {
		s.createPolicyAlert(agent, "Unauthorized Access Attempt", unauthPolicyName, unauthBlocked,
			"Unauthorized access pattern detected", domain.AlertSeverityHigh, auditID)
	}
	// Apply MONITORING mode override
	if unauthBlocked && policyEnforcementMode == domain.EnforcementModeMonitoring {
		fmt.Printf("✅ MONITORING MODE: Unauthorized access policy '%s' would block but allowing (mode: %s)\n",
			unauthPolicyName, policyEnforcementMode)
		unauthBlocked = false
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

// matchesCapability checks if a requested capability matches a registered capability
// Supports exact matching and wildcard patterns
func (s *AgentService) matchesCapability(requestedCapability string, resource string, grantedCapability string) bool {
	// Exact match
	if requestedCapability == grantedCapability {
		return true
	}

	// Wildcard patterns (e.g., "file:*" matches "file:read", "file:write")
	if len(grantedCapability) > 0 && grantedCapability[len(grantedCapability)-1] == '*' {
		prefix := grantedCapability[:len(grantedCapability)-1]
		if len(requestedCapability) >= len(prefix) && requestedCapability[:len(prefix)] == prefix {
			return true
		}
	}

	// Future: Add more sophisticated pattern matching here
	// - Resource-based matching (e.g., "file:read:/data/*")
	// - Time-based capabilities
	// - Context-aware matching

	return false
}

// LogCapabilityResult logs the outcome of a verified capability usage
func (s *AgentService) LogCapabilityResult(
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
	metadata["auditId"] = auditID.String()

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
	MCPServerIDs   []string               `json:"mcpServerIds"`    // MCP server IDs or names
	DetectedMethod string                 `json:"detectedMethod"`  // "manual", "auto_sdk", "auto_config", "cli"
	Confidence     float64                `json:"confidence"`      // Detection confidence (0-100)
	Metadata       map[string]interface{} `json:"metadata"`        // Additional context
}

// MCPServerDetail represents detailed MCP server information
type MCPServerDetail struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	URL            string    `json:"url"`
	Status         string    `json:"status"`
	TrustScore     float64   `json:"trustScore"`
	AddedAt        time.Time `json:"addedAt"`
	DetectedMethod string    `json:"detectedMethod"`
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

	// 2. Sanitize input identifiers (handle malformed entries like "memory,aws-terraform")
	mcpServerIdentifiers = sanitizeTalksTo(mcpServerIdentifiers)

	// 3. Initialize talks_to if nil, and sanitize existing entries
	if agent.TalksTo == nil {
		agent.TalksTo = []string{}
	} else {
		// Sanitize any existing malformed entries
		agent.TalksTo = sanitizeTalksTo(agent.TalksTo)
	}

	// 4. Create a map to track existing entries (prevent duplicates)
	existingMap := make(map[string]bool)
	for _, existing := range agent.TalksTo {
		existingMap[existing] = true
	}

	// 5. Add new MCP servers (only unique ones)
	addedServers := []string{}
	for _, identifier := range mcpServerIdentifiers {
		if !existingMap[identifier] {
			agent.TalksTo = append(agent.TalksTo, identifier)
			existingMap[identifier] = true
			addedServers = append(addedServers, identifier)
		}
	}

	// 6. Update agent in database
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
	ConfigPath   string `json:"configPath"`   // Path to Claude Desktop config file
	AutoRegister bool   `json:"autoRegister"` // Whether to auto-register discovered MCPs
	DryRun       bool   `json:"dryRun"`       // Preview changes without applying
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
	DetectedServers   []DetectedMCPServer `json:"detectedServers"`
	RegisteredCount   int                 `json:"registeredCount"`
	MappedCount       int                 `json:"mappedCount"`
	TotalTalksTo      int                 `json:"totalTalksTo"`
	DryRun            bool                `json:"dryRun"`
	ErrorsEncountered []string            `json:"errorsEncountered,omitempty"`
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

			// Note: agentID, sdkTokenID, and apiKeyID are nil for auto-detected MCP servers during agent registration
			_, err := mcpService.CreateMCPServer(ctx, registerReq, orgID, userID, nil, nil, nil)
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

// RevokeAgent permanently revokes an agent. Data is retained for 30 days before cleanup.
// During the retention period, the agent can be reactivated with ReactivateAgent.
func (s *AgentService) RevokeAgent(ctx context.Context, id uuid.UUID) error {
	agent, err := s.agentRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	if agent.Status == domain.AgentStatusRevoked {
		return fmt.Errorf("agent is already revoked")
	}

	// Set status to revoked
	agent.Status = domain.AgentStatusRevoked

	if err := s.agentRepo.Update(agent); err != nil {
		return fmt.Errorf("failed to revoke agent: %w", err)
	}

	// Recalculate trust score (revocation drops trust to 0)
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
// SECURITY: If agent already has a key, the caller must authenticate via JWT (not Ed25519)
// to prevent an attacker with a stolen key from replacing it with their own.
func (s *AgentService) UpdateAgentPublicKey(ctx context.Context, agentID uuid.UUID, publicKey string, authMethod string) error {
	// 1. Fetch agent
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	// 2. Validate public key format (should be base64-encoded 32-byte Ed25519 public key)
	if publicKey == "" {
		return fmt.Errorf("public_key is required")
	}

	// 3. SECURITY: If agent already has a registered key, require JWT auth (not Ed25519)
	// This prevents an attacker who compromised the old key from replacing it.
	if agent.PublicKey != nil && *agent.PublicKey != "" {
		if authMethod == "ed25519" || authMethod == "mldsa" || authMethod == "hybrid" {
			return fmt.Errorf("key replacement requires JWT authentication, not cryptographic agent auth")
		}
	}

	// 4. Store previous public key for grace period
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

// UpdateAgentPQCKey updates an agent's PQC key information
func (s *AgentService) UpdateAgentPQCKey(ctx context.Context, agentID uuid.UUID, pqcPublicKey string, algorithm string, hybridMode bool) error {
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	now := time.Now()
	agent.PQCPublicKey = &pqcPublicKey
	agent.PQCKeyAlgorithm = &algorithm
	agent.PQCKeyCreatedAt = &now
	agent.HybridModeEnabled = hybridMode
	agent.UpdatedAt = now

	if err := s.agentRepo.Update(agent); err != nil {
		return fmt.Errorf("failed to update agent PQC key: %w", err)
	}

	return nil
}

// RotateAgentPQCKey rotates an agent's PQC key, storing the previous key for grace period
func (s *AgentService) RotateAgentPQCKey(ctx context.Context, agentID uuid.UUID, newPQCPublicKey string, algorithm string) error {
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	now := time.Now()
	// Store previous key for grace period verification
	agent.PreviousPQCPublicKey = agent.PQCPublicKey
	agent.PQCPublicKey = &newPQCPublicKey
	agent.PQCKeyAlgorithm = &algorithm
	agent.PQCKeyCreatedAt = &now
	agent.UpdatedAt = now

	// Bump the rotation counter (#129) — matches RotateCredentials and
	// UpdateAgentPublicKey, which both already increment. Without this, the
	// dashboard's rotationCount field stays at 0 for any agent that has only
	// rotated its PQC key, hiding rotation activity from operators.
	agent.RotationCount++

	if err := s.agentRepo.Update(agent); err != nil {
		return fmt.Errorf("failed to rotate agent PQC key: %w", err)
	}

	return nil
}

// SetAgentHybridMode enables or disables hybrid authentication mode for an agent
func (s *AgentService) SetAgentHybridMode(ctx context.Context, agentID uuid.UUID, enabled bool) error {
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}

	agent.HybridModeEnabled = enabled
	agent.UpdatedAt = time.Now()

	if err := s.agentRepo.Update(agent); err != nil {
		return fmt.Errorf("failed to update agent hybrid mode: %w", err)
	}

	return nil
}

// calculateViolationSeverity determines the severity level for a capability violation
func (s *AgentService) calculateViolationSeverity(agent *domain.Agent, isBlocked bool) string {
	// Base severity on trust score and whether action was blocked
	// Trust score is stored as 0-1 in database (e.g., 0.30 = 30%)
	if agent.TrustScore < 0.30 || agent.IsCompromised {
		return "critical"
	}

	if isBlocked {
		// Blocked violations are more severe
		if agent.TrustScore < 0.50 {
			return "high"
		}
		return "medium"
	}

	// Alert-only violations (not blocked) are lower severity
	if agent.TrustScore < 0.50 {
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
	enforcement := map[bool]string{true: "BLOCKED", false: "ALLOWED (monitored)"}[isBlocked]
	alertDescription := fmt.Sprintf(
		"Agent '%s' triggered security policy '%s'. %s. Enforcement: %s.",
		agent.DisplayName, policyName, description, enforcement,
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
		AuditID:        &auditID, // Link to triggering audit log
		AgentName:      agent.DisplayName,
		Metadata: map[string]interface{}{
			"policyName":  policyName,
			"policyType":  alertType,
			"enforcement": enforcement,
			"isBlocked":   isBlocked,
			"trustScore":  agent.TrustScore,
		},
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
	// Verify agent exists
	_, err := s.agentRepo.GetByID(agentID)
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
	trustImpact := -5          // Default for low severity

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

	// NOTE: We intentionally do NOT recalculate trust score here.
	// The violation already has a trust_score_impact that is applied directly.
	// Recalculating would cause double-counting since the 8-factor algorithm
	// also considers security alerts (which includes this violation).
	// Trust score recalculation should only happen on positive events or scheduled intervals.

	return nil
}

// EnforceKeyExpiry suspends agents with expired keys past grace period
func (s *AgentService) EnforceKeyExpiry(ctx context.Context) (int, error) {
	now := time.Now()
	agents, err := s.agentRepo.List(0, 0) // Get all agents
	if err != nil {
		return 0, fmt.Errorf("failed to list agents: %w", err)
	}

	suspended := 0
	for _, agent := range agents {
		if agent.Status == domain.AgentStatusRevoked || agent.Status == domain.AgentStatusSuspended {
			continue
		}
		// Check if key is expired and past grace period
		if agent.KeyExpiresAt != nil && agent.KeyExpiresAt.Before(now) {
			// If there's a grace period, check if we're past it
			if agent.KeyRotationGraceUntil != nil && agent.KeyRotationGraceUntil.After(now) {
				continue // Still in grace period
			}
			// Suspend the agent
			agent.Status = domain.AgentStatusSuspended
			agent.UpdatedAt = now
			if err := s.agentRepo.Update(agent); err != nil {
				continue // Log but don't fail the whole batch
			}
			suspended++
		}
	}

	return suspended, nil
}

// RecordHeartbeat updates the heartbeat timestamp for an agent
func (s *AgentService) RecordHeartbeat(ctx context.Context, agentID uuid.UUID) (*domain.Agent, error) {
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	now := time.Now()
	agent.LastHeartbeat = &now
	agent.UpdatedAt = now
	if err := s.agentRepo.Update(agent); err != nil {
		return nil, fmt.Errorf("failed to update heartbeat: %w", err)
	}

	return agent, nil
}

// GetAgentsByIDs returns agents matching the given IDs with status and trust info
func (s *AgentService) GetAgentsByIDs(ctx context.Context, ids []uuid.UUID) ([]*domain.Agent, error) {
	return s.agentRepo.GetByIDs(ctx, ids)
}
