package application

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	infracrypto "github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/utils"
)

type MCPService struct {
	mcpRepo               *repository.MCPServerRepository
	verificationEventRepo domain.VerificationEventRepository
	userRepo              *repository.UserRepository
	cryptoService         *infracrypto.ED25519Service
	keyVault              *crypto.KeyVault       // ✅ For secure private key storage
	capabilityService     *MCPCapabilityService  // ✅ For automatic capability detection
	capabilityRepo        *repository.MCPServerCapabilityRepository // ✅ For creating SDK capabilities
	connectionRepo        *repository.AgentMCPConnectionRepository  // ✅ For tracking agent-MCP connections
	httpClient            *http.Client           // ✅ For real MCP server communication
	agentRepo             *repository.AgentRepository // ✅ For querying connected agents
	tagRepo               *repository.TagRepository   // ✅ For tagging MCP servers during registration
	// trustCalculator is the only source of an MCP server's trust score. This
	// service assigns no score of its own; it asks the calculator and stores
	// what it returns. Required, not optional: a nil calculator leaves every
	// server at the DB default of 0.0, which reads as "measured and maximally
	// untrusted" rather than "not scored", and fails every MinTrustScore
	// policy gate. See the [CHIEF-CDS] decision of 2026-08-04.
	trustCalculator *MCPTrustCalculator
	// In-memory challenge storage (in production, use Redis)
	challenges map[string]ChallengeData
}

// ChallengeData stores challenge information
type ChallengeData struct {
	Challenge string
	ServerID  uuid.UUID
	CreatedAt time.Time
	ExpiresAt time.Time
}

func NewMCPService(mcpRepo *repository.MCPServerRepository, verificationEventRepo domain.VerificationEventRepository, userRepo *repository.UserRepository, keyVault *crypto.KeyVault, capabilityService *MCPCapabilityService, capabilityRepo *repository.MCPServerCapabilityRepository, connectionRepo *repository.AgentMCPConnectionRepository, agentRepo *repository.AgentRepository, tagRepo *repository.TagRepository, trustCalculator *MCPTrustCalculator) *MCPService {
	return &MCPService{
		mcpRepo:               mcpRepo,
		verificationEventRepo: verificationEventRepo,
		userRepo:              userRepo,
		cryptoService:         infracrypto.NewED25519Service(),
		keyVault:              keyVault,
		capabilityService:     capabilityService,
		capabilityRepo:        capabilityRepo,
		connectionRepo:        connectionRepo,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // 30 second timeout for MCP server communication
		},
		challenges:      make(map[string]ChallengeData),
		agentRepo:       agentRepo,
		tagRepo:         tagRepo,
		trustCalculator: trustCalculator,
	}
}

// scoreServer runs the 8-factor trust calculation for a persisted MCP server
// and stores the result. `mcp_trust_scores` is the source of truth; the
// migration 094 trigger mirrors the score into `mcp_servers.trust_score`.
//
// The server row must already exist — the calculator re-reads it so that
// database-supplied columns (notably `created_at`, which Factor 6 buckets by
// age) hold their real values rather than Go zero values. A zero `time.Time`
// would score as "90+ days old" and hand a brand-new server the maximum age
// factor.
//
// The in-memory server is updated with the stored score, so the value this
// service returns to its caller is the value that was persisted rather than
// the zero the struct was constructed with.
//
// Scoring failure never fails the caller's operation: registration and
// verification are the user's action, the score is a derived value, and a
// server with no score yet is a state the system already represents. It is
// logged rather than swallowed so an unscored server is traceable.
func (s *MCPService) scoreServer(ctx context.Context, server *domain.MCPServer, occasion string) {
	if s.trustCalculator == nil {
		fmt.Printf("⚠️  No trust calculator configured; MCP server %s left unscored after %s\n", server.ID, occasion)
		return
	}
	score, err := s.trustCalculator.CalculateTrustScore(ctx, server.ID)
	if err != nil {
		fmt.Printf("⚠️  Failed to calculate trust score for MCP server %s after %s: %v\n", server.ID, occasion, err)
		return
	}
	server.TrustScore = score.Score
}

// CreateMCPServerRequest represents the request to create an MCP server
type CreateMCPServerRequest struct {
	Name              string   `json:"name" validate:"required"`
	Description       string   `json:"description"`
	URL               string   `json:"url" validate:"required,url"`
	Version           string   `json:"version"`
	PublicKey         string   `json:"publicKey"`
	VerificationURL   string   `json:"verificationUrl"`
	Capabilities      []string `json:"capabilities"`
	TagIds            []string `json:"tagIds,omitempty"`          // ✅ Tags to apply during registration
	RegisteredByAgent string   `json:"registeredByAgent,omitempty"` // ✅ Agent ID when SDK registers MCP
}

// UpdateMCPServerRequest represents the request to update an MCP server
type UpdateMCPServerRequest struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	URL             string   `json:"url"`
	Version         string   `json:"version"`
	PublicKey       string   `json:"publicKey"`
	VerificationURL string   `json:"verificationUrl"`
	Capabilities    []string `json:"capabilities"`
}

// AddPublicKeyRequest represents the request to add a public key
type AddPublicKeyRequest struct {
	PublicKey string `json:"publicKey" validate:"required"`
	KeyType   string `json:"keyType" validate:"required"` // e.g., "rsa", "ed25519"
}

// CreateMCPServer creates a new MCP server
// agentID is optional - if provided (SDK registration), creates agent-MCP connection automatically
// sdkTokenID is optional - tracks which SDK token was used to create this server
// apiKeyID is optional - tracks which API key was used to create this server
func (s *MCPService) CreateMCPServer(ctx context.Context, req *CreateMCPServerRequest, orgID, userID uuid.UUID, agentID *uuid.UUID, sdkTokenID *uuid.UUID, apiKeyID *uuid.UUID) (*domain.MCPServer, error) {
	// Check if THIS organization already has an MCP server at this URL.
	// SECURITY (defect #40): The lookup is org-scoped at the repository
	// layer. The mcp_servers table has UNIQUE(organization_id, url), so
	// the same URL can legally coexist across organizations; a global
	// lookup would leak cross-tenant existence via the 409 response
	// (returning the victim's UUID + name), allow attackers to create
	// agent-MCP connections to cross-org MCP servers, and let them
	// mutate the victim's capabilities/version in the update branch
	// below. With the org filter, cross-org collisions return nil here
	// and execution proceeds to create a same-URL MCP server in the
	// caller's org — which the DB constraint permits.
	existing, _ := s.mcpRepo.GetByURL(req.URL, orgID)
	if existing != nil {
		// ✅ Even if MCP server exists, create agent-MCP connection for THIS agent
		// This allows multiple agents to register connections to the same MCP server
		if agentID != nil && s.connectionRepo != nil {
			// Check if connection already exists
			existingConn, _ := s.connectionRepo.GetByAgentAndMCPServer(ctx, *agentID, existing.ID)
			if existingConn == nil {
				now := time.Now().UTC()
				connection := &domain.AgentMCPConnection{
					ID:               uuid.New(),
					AgentID:          *agentID,
					MCPServerID:      existing.ID,
					DetectionID:      nil,
					ConnectionType:   domain.ConnectionTypeUserRegistered,
					FirstConnectedAt: now,
					LastAttestedAt:   nil,
					AttestationCount: 0,
					IsActive:         true,
					CreatedAt:        now,
					UpdatedAt:        now,
				}

				if err := s.connectionRepo.Create(ctx, connection); err != nil {
					fmt.Printf("⚠️  Warning: Failed to create agent-MCP connection for agent %s and existing MCP %s: %v\n", agentID, existing.Name, err)
				} else {
					fmt.Printf("✅ Created agent-MCP connection for agent %s → existing MCP server %s\n", agentID, existing.Name)
				}
			}

			// ✅ Always ensure agent's talks_to field contains this MCP server
			// This handles both new connections and existing connections created before this fix
			s.updateAgentTalksTo(*agentID, existing.Name)
		}

		// ✅ Update capabilities and version if new ones were detected by the registering agent
		// This allows capability discovery and version info to improve over time as more agents attest
		needsUpdate := false

		if len(req.Capabilities) > 0 && len(existing.Capabilities) == 0 {
			existing.Capabilities = req.Capabilities
			needsUpdate = true
		}

		// Update version if a real version was discovered (not "1.0.0" placeholder or empty)
		if req.Version != "" && req.Version != "1.0.0" && (existing.Version == "" || existing.Version == "1.0.0") {
			existing.Version = req.Version
			needsUpdate = true
		}

		if needsUpdate {
			existing.UpdatedAt = time.Now().UTC()
			if err := s.mcpRepo.Update(existing); err != nil {
				fmt.Printf("⚠️  Warning: Failed to update existing MCP %s: %v\n", existing.Name, err)
			} else {
				fmt.Printf("✅ Updated existing MCP server %s (version: %s, capabilities: %d)\n", existing.Name, existing.Version, len(existing.Capabilities))
			}
		}

		// Return error with existing server ID for 409 response
		return existing, fmt.Errorf("mcp server with this URL already exists")
	}

	// ✅ AUTOMATIC KEY GENERATION - Zero effort for developers
	// If no public key provided, generate Ed25519 key pair automatically
	publicKey := req.PublicKey
	if publicKey == "" && s.keyVault != nil {
		// Generate Ed25519 key pair automatically
		keyPair, err := crypto.GenerateEd25519KeyPair()
		if err != nil {
			return nil, fmt.Errorf("failed to generate cryptographic keys: %w", err)
		}

		// Encode keys to base64 for storage
		encodedKeys := crypto.EncodeKeyPair(keyPair)
		publicKey = encodedKeys.PublicKeyBase64

		// Note: For MCP servers, we don't store the private key since the actual MCP server
		// will have its own private key. We just use this for initial verification testing.
		// In production, the real MCP server would sign challenges with its own private key.
		fmt.Printf("✅ Generated Ed25519 keys for MCP server %s\n", req.Name)
	}

	// ✅ Fetch user details for audit trail
	var createdByName, createdByEmail string
	if s.userRepo != nil {
		user, err := s.userRepo.GetByID(userID)
		if err == nil && user != nil {
			createdByName = user.Name
			createdByEmail = user.Email
		}
	}

	// ✅ AUTO-VERIFICATION: MCP servers registered via SDK are auto-verified
	// This matches agent behavior where SDK-registered agents are auto-verified
	// Manual registrations (no agentID) remain pending for admin review
	isSDKRegistration := agentID != nil
	var status domain.MCPServerStatus
	var isVerified bool
	var verificationMethod string
	var lastVerifiedAt *time.Time

	if isSDKRegistration {
		// SDK registration: auto-verify since agent is authenticated
		status = domain.MCPServerStatusVerified
		isVerified = true
		verificationMethod = "sdk_registration"
		now := time.Now()
		lastVerifiedAt = &now
		fmt.Printf("✅ Auto-verifying MCP server %s (registered via SDK by agent %s)\n", req.Name, agentID)
	} else {
		// Manual registration: requires admin verification
		status = domain.MCPServerStatusPending
		isVerified = false
		verificationMethod = "manual"
	}

	// The trust score is NOT set here. It is calculated from the server's real
	// inputs by the 8-factor calculator once the row exists (see scoreServer
	// below). Assigning a literal here — this line used to read
	// `trustScore = 75.0` for SDK registrations — published a number that
	// implied measurement without measuring anything, and, being on a 0-100
	// scale, sat above every [0,1] `MinTrustScore` policy threshold, so an
	// SDK-registered server passed any organization trust floor unconditionally.

	server := &domain.MCPServer{
		ID:                  uuid.New(),
		OrganizationID:      orgID,
		Name:                strings.TrimSpace(req.Name),
		Description:         req.Description,
		URL:                 strings.TrimSpace(req.URL), // ✅ Trim spaces from URL
		Version:             req.Version,
		PublicKey:           publicKey, // ✅ Auto-generated if not provided
		Status:              status,
		IsVerified:          isVerified,
		LastVerifiedAt:      lastVerifiedAt,
		VerificationURL:     strings.TrimSpace(req.VerificationURL), // ✅ Trim spaces
		Capabilities:        req.Capabilities,
		VerificationMethod:  verificationMethod,
		RegisteredByAgent:   agentID,         // ✅ Track which agent registered this MCP (for SDK registrations)
		CreatedBy:           userID,
		CreatedByName:       createdByName,   // ✅ Audit trail: creator name
		CreatedByEmail:      createdByEmail,  // ✅ Audit trail: creator email
		CreatedBySDKTokenID: sdkTokenID,      // ✅ SDK token tracking for revocation
		CreatedByAPIKeyID:   apiKeyID,        // ✅ API key tracking for revocation
	}

	if err := s.mcpRepo.Create(server); err != nil {
		return nil, err
	}

	// ✅ Parse capabilities array and store in mcp_server_capabilities table
	// SDK sends capabilities as string array like ["read_file", "write_file", "list_directory"]
	// We need to convert these to proper capability entries for the UI to display
	if len(req.Capabilities) > 0 && s.capabilityRepo != nil {
		for _, capName := range req.Capabilities {
			// For now, treat all as "tool" type since SDK doesn't specify
			// In future, SDK could send structured capabilities with types
			// Create capability schema as empty JSON for PostgreSQL
			emptySchema := []byte("{}")

			capability := &domain.MCPServerCapability{
				ID:               uuid.New(),
				MCPServerID:      server.ID,
				Name:             capName,
				CapabilityType:   "tool", // Default to "tool" for SDK-registered capabilities
				Description:      fmt.Sprintf("SDK-registered capability: %s", capName),
				CapabilitySchema: emptySchema, // Empty JSON object for SDK-registered capabilities
				IsActive:         true,
			}

			if err := s.capabilityRepo.Create(capability); err != nil {
				fmt.Printf("⚠️  Warning: Failed to create capability %s for MCP server %s: %v\n", capName, server.Name, err)
				// Don't fail the entire operation if capability creation fails
			} else {
				fmt.Printf("✅ Created capability %s for MCP server %s\n", capName, server.Name)
			}
		}
	}

	// ✅ Create agent-MCP connection when agent registers MCP server via SDK
	// This tracks which agents are using which MCP servers for security monitoring
	if agentID != nil && s.connectionRepo != nil {
		now := time.Now().UTC()
		connection := &domain.AgentMCPConnection{
			ID:               uuid.New(),
			AgentID:          *agentID,
			MCPServerID:      server.ID,
			DetectionID:      nil,
			ConnectionType:   domain.ConnectionTypeUserRegistered,
			FirstConnectedAt: now,
			LastAttestedAt:   nil,
			AttestationCount: 0,
			IsActive:         true,
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		if err := s.connectionRepo.Create(ctx, connection); err != nil {
			fmt.Printf("⚠️  Warning: Failed to create agent-MCP connection for agent %s and MCP %s: %v\n", agentID, server.Name, err)
			// Don't fail the entire operation if connection creation fails
		} else {
			fmt.Printf("✅ Created agent-MCP connection for agent %s → MCP server %s\n", agentID, server.Name)

			// ✅ Also update agent's talks_to field so GetMCPServerAgents can find this connection
			s.updateAgentTalksTo(*agentID, server.Name)
		}
	}

	// if server.PublicKey != "" {
	//	// Run verification asynchronously to avoid blocking the creation response
	//	go func() {
	//		// Use a background context for async operation
	//		bgCtx := context.Background()
	//		// Use localhost IP for system-initiated verification
	//		if err := s.VerifyMCPServer(bgCtx, server.ID, userID, "127.0.0.1"); err != nil {
	//			fmt.Printf("⚠️  Automatic verification failed for MCP server %s: %v\n", server.Name, err)
	//		} else {
	//			fmt.Printf("✅ Automatic verification succeeded for MCP server %s\n", server.Name)
	//		}
	//	}()
	// }
	// ✅ Manual verification required
	// MCP servers are created with status="pending" and is_verified=false
	// Admins must manually verify servers by clicking the "Verify" button in the UI
	// This ensures proper security review before servers are trusted

	// ✅ AUTO-APPLY TAGS: Apply tags during registration (no separate API call needed)
	if len(req.TagIds) > 0 && s.tagRepo != nil {
		tagUUIDs := make([]uuid.UUID, 0, len(req.TagIds))
		for _, tagIDStr := range req.TagIds {
			tagID, err := uuid.Parse(tagIDStr)
			if err != nil {
				fmt.Printf("⚠️  Warning: invalid tag ID '%s': %v\n", tagIDStr, err)
				continue
			}
			tagUUIDs = append(tagUUIDs, tagID)
		}

		if len(tagUUIDs) > 0 {
			if err := s.tagRepo.AddTagsToMCPServer(ctx, server.ID, tagUUIDs); err != nil {
				fmt.Printf("⚠️  Warning: failed to apply tags to MCP server %s: %v\n", server.Name, err)
			} else {
				fmt.Printf("✅ Applied %d tags to MCP server %s\n", len(tagUUIDs), server.Name)
			}
		}
	}

	// Score the server from its real inputs. This runs last, after the
	// capability rows and the agent-MCP connection have been written, because
	// Factor 3 (capability stability) reads `mcp_server_capabilities` and
	// Factor 7 (usage patterns) reads the connection count. Scoring before
	// those writes would measure a server that looks emptier than it is.
	//
	// A freshly registered server legitimately scores low — no attestations,
	// age bucket under 7 days — and that is the honest value, not a defect.
	s.scoreServer(ctx, server, "registration")

	return server, nil
}

// GetMCPServer retrieves an MCP server by ID
func (s *MCPService) GetMCPServer(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error) {
	return s.mcpRepo.GetByID(id)
}

// GetMCPServerByName retrieves an MCP server by name within an organization
// This is useful for SDK clients to check if capabilities are cached before running discovery
func (s *MCPService) GetMCPServerByName(ctx context.Context, orgID uuid.UUID, name string) (*domain.MCPServer, error) {
	return s.mcpRepo.GetByName(orgID, name)
}

// ListMCPServers lists all MCP servers for an organization
func (s *MCPService) ListMCPServers(ctx context.Context, orgID uuid.UUID) ([]*domain.MCPServer, error) {
	return s.mcpRepo.GetByOrganization(orgID)
}

// UpdateMCPServer updates an MCP server
func (s *MCPService) UpdateMCPServer(ctx context.Context, id uuid.UUID, req *UpdateMCPServerRequest) (*domain.MCPServer, error) {
	server, err := s.mcpRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Update fields
	if req.Name != "" {
		server.Name = strings.TrimSpace(req.Name)
	}
	if req.Description != "" {
		server.Description = req.Description
	}
	if req.URL != "" {
		server.URL = strings.TrimSpace(req.URL) // ✅ Trim spaces
	}
	if req.Version != "" {
		server.Version = req.Version
	}
	if req.PublicKey != "" {
		server.PublicKey = req.PublicKey
	}
	if req.VerificationURL != "" {
		server.VerificationURL = strings.TrimSpace(req.VerificationURL) // ✅ Trim spaces
	}
	if len(req.Capabilities) > 0 {
		server.Capabilities = req.Capabilities
	}

	if err := s.mcpRepo.Update(server); err != nil {
		return nil, err
	}

	return server, nil
}

// DeleteMCPServer deletes an MCP server
func (s *MCPService) DeleteMCPServer(ctx context.Context, id uuid.UUID) error {
	return s.mcpRepo.Delete(id)
}

// VerifyMCPServer performs cryptographic verification of an MCP server
func (s *MCPService) VerifyMCPServer(ctx context.Context, id uuid.UUID, userID uuid.UUID, userIP string) error {
	startTime := time.Now()

	server, err := s.mcpRepo.GetByID(id)
	if err != nil {
		return err
	}

	// Fetch user information for audit trail
	var initiatorName *string
	if s.userRepo != nil {
		user, err := s.userRepo.GetByID(userID)
		if err == nil && user != nil {
			initiatorName = &user.Email
		}
	}

	// Cryptographic verification workflow:
	// 1. Check if server has a public key
	if server.PublicKey == "" {
		return fmt.Errorf("server must have a public key for verification")
	}

	// 2. Generate challenge
	challenge, err := s.GenerateVerificationChallenge(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to generate challenge: %w", err)
	}

	// ✅ 3. REAL CRYPTOGRAPHIC VERIFICATION
	// Send challenge to server's verification URL and verify signed response
	var verificationSuccess bool

	if server.VerificationURL == "" {
		// If no verification URL provided, use the server's base URL + standard endpoint
		server.VerificationURL = server.URL + "/.well-known/mcp/verify"
	}

	// Clean up URL (trim spaces that might have been entered in the form)
	verificationURL := strings.TrimSpace(server.VerificationURL)

	// SECURITY: Validate URL to prevent SSRF attacks
	if err := utils.ValidateExternalURL(verificationURL); err != nil {
		return fmt.Errorf("invalid MCP verification URL: %w", err)
	}

	// Step 3a: Send challenge to MCP server
	challengeReq := map[string]string{
		"challenge": challenge,
		"server_id": id.String(),
	}
	reqBody, err := json.Marshal(challengeReq)
	if err != nil {
		return fmt.Errorf("failed to marshal challenge request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", verificationURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create verification request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to contact MCP server verification endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MCP server verification endpoint returned non-200 status: %d", resp.StatusCode)
	}

	// Step 3b: Parse signed challenge response (limit to 64KB)
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return fmt.Errorf("failed to read verification response: %w", err)
	}

	var verifyResp struct {
		SignedChallenge string `json:"signedChallenge"`
	}
	if err := json.Unmarshal(respBody, &verifyResp); err != nil {
		return fmt.Errorf("failed to parse verification response: %w", err)
	}

	// Step 3c: Verify the signed challenge using Ed25519
	if err := s.VerifyChallengeResponse(ctx, id, verifyResp.SignedChallenge); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	// ✅ Verification successful - cryptographic proof established
	verificationSuccess = true

	var verificationStatus domain.VerificationEventStatus
	var verificationResult domain.VerificationResult
	var confidence float64
	var trustScore float64

	if verificationSuccess {
		// Mark server as verified
		if err := s.mcpRepo.VerifyServer(ctx, id); err != nil {
			return err
		}

		now := time.Now()
		server.IsVerified = true
		server.Status = domain.MCPServerStatusVerified
		server.LastVerifiedAt = &now

		if err := s.mcpRepo.Update(server); err != nil {
			return err
		}

		// Recalculate from real inputs now that the server is verified.
		// Verification genuinely moves two factors — Security Posture credits
		// `IsVerified`, and Attestation Consensus stops applying the 0.5
		// pending-status multiplier — so the score rises on its own. It used
		// to be pinned here to the literal 75.0 regardless of what the server
		// actually looked like.
		s.scoreServer(ctx, server, "verification")

		// ✅ AUTOMATIC CAPABILITY DETECTION
		// After successful verification, automatically detect MCP server capabilities
		if s.capabilityService != nil {
			go func() {
				// Run asynchronously to avoid blocking verification; detached
				// context so the caller returning does not cancel detection.
				bgCtx := context.Background()
				if err := s.capabilityService.DetectCapabilities(bgCtx, id); err != nil {
					fmt.Printf("⚠️  Failed to detect capabilities for MCP server %s: %v\n", server.Name, err)
				}
			}()
		}

		verificationStatus = domain.VerificationEventStatusSuccess
		verificationResult = domain.VerificationResultVerified
		confidence = 0.95
		trustScore = server.TrustScore
	} else {
		verificationStatus = domain.VerificationEventStatusFailed
		verificationResult = domain.VerificationResultDenied
		confidence = 0.0
		trustScore = 0.0
	}

	// Create verification event for monitoring
	completedAt := time.Now()
	durationMs := int(completedAt.Sub(startTime).Milliseconds())

	mcpServerIDPtr := &id
	mcpServerNamePtr := &server.Name

	event := &domain.VerificationEvent{
		ID:               uuid.New(),
		OrganizationID:   server.OrganizationID,
		MCPServerID:      mcpServerIDPtr,
		MCPServerName:    mcpServerNamePtr,
		Protocol:         domain.VerificationProtocolMCP,
		VerificationType: domain.VerificationTypeIdentity,
		Status:           verificationStatus,
		Result:           &verificationResult,
		Confidence:       confidence,
		TrustScore:       trustScore,
		DurationMs:       durationMs,
		InitiatorType:    domain.InitiatorTypeUser,
		InitiatorID:      &userID,
		InitiatorName:    initiatorName,
		InitiatorIP:      &userIP,
		StartedAt:        startTime,
		CompletedAt:      &completedAt,
		CreatedAt:        time.Now(),
	}

	// Store the verification event
	if s.verificationEventRepo != nil {
		if err := s.verificationEventRepo.Create(event); err != nil {
			fmt.Printf("⚠️  Failed to create verification event: %v\n", err)
		}
	}

	if !verificationSuccess {
		return fmt.Errorf("cryptographic verification failed")
	}

	return nil
}

// AddPublicKey adds a public key to an MCP server
func (s *MCPService) AddPublicKey(ctx context.Context, serverID uuid.UUID, req *AddPublicKeyRequest) error {
	// Verify server exists
	_, err := s.mcpRepo.GetByID(serverID)
	if err != nil {
		return err
	}

	return s.mcpRepo.AddPublicKey(ctx, serverID, req.PublicKey, req.KeyType)
}

// GetVerificationStatus retrieves the verification status of an MCP server
func (s *MCPService) GetVerificationStatus(ctx context.Context, id uuid.UUID) (*domain.MCPServerVerificationStatus, error) {
	return s.mcpRepo.GetVerificationStatus(id)
}

// GenerateVerificationChallenge generates a challenge for server verification
func (s *MCPService) GenerateVerificationChallenge(ctx context.Context, serverID uuid.UUID) (string, error) {
	server, err := s.mcpRepo.GetByID(serverID)
	if err != nil {
		return "", err
	}

	// Verify server has a public key
	if server.PublicKey == "" {
		return "", fmt.Errorf("server must have a public key before verification")
	}

	// Generate cryptographic challenge
	challenge, err := s.cryptoService.GenerateChallenge()
	if err != nil {
		return "", fmt.Errorf("failed to generate challenge: %w", err)
	}

	// Store challenge with expiration (5 minutes)
	now := time.Now()
	s.challenges[serverID.String()] = ChallengeData{
		Challenge: challenge,
		ServerID:  serverID,
		CreatedAt: now,
		ExpiresAt: now.Add(5 * time.Minute),
	}

	return challenge, nil
}

// VerifyChallengeResponse verifies a signed challenge response
func (s *MCPService) VerifyChallengeResponse(ctx context.Context, serverID uuid.UUID, signedChallenge string) error {
	// Retrieve challenge
	challengeData, exists := s.challenges[serverID.String()]
	if !exists {
		return fmt.Errorf("no challenge found for server")
	}

	// Check if challenge has expired
	if time.Now().After(challengeData.ExpiresAt) {
		delete(s.challenges, serverID.String())
		return fmt.Errorf("challenge has expired")
	}

	// Get server details
	server, err := s.mcpRepo.GetByID(serverID)
	if err != nil {
		return err
	}

	// Verify server has a public key
	if server.PublicKey == "" {
		return fmt.Errorf("server does not have a public key")
	}

	// Verify the signed challenge
	valid, err := s.cryptoService.Verify(server.PublicKey, []byte(challengeData.Challenge), signedChallenge)
	if err != nil {
		return fmt.Errorf("failed to verify signature: %w", err)
	}

	if !valid {
		return fmt.Errorf("invalid signature")
	}

	// Clean up challenge after successful verification
	delete(s.challenges, serverID.String())

	return nil
}

// VerifyMCPCapability verifies if an MCP server can use a capability
func (s *MCPService) VerifyMCPCapability(
	ctx context.Context,
	mcpID uuid.UUID,
	capability string,
	resource string,
	targetService string,
	metadata map[string]interface{},
) (allowed bool, reason string, auditID uuid.UUID, err error) {
	// 1. Fetch MCP server
	mcp, err := s.mcpRepo.GetByID(mcpID)
	if err != nil {
		return false, "MCP server not found", uuid.Nil, err
	}

	// 2. Check MCP server status
	if mcp.Status != domain.MCPServerStatusVerified {
		return false, "MCP server not verified", uuid.Nil, nil
	}

	// 3. Verify capabilities (simplified for now)
	allowed = mcp.IsVerified
	if allowed {
		reason = "MCP server is verified and authorized"
	} else {
		reason = "MCP server not verified"
	}

	// 4. Audit log via verification event
	auditID = uuid.New()

	// Create verification event for MCP action audit trail
	now := time.Now()
	eventStatus := domain.VerificationEventStatusSuccess
	var eventResult *domain.VerificationResult
	if allowed {
		result := domain.VerificationResultVerified
		eventResult = &result
	} else {
		eventStatus = domain.VerificationEventStatusFailed
		result := domain.VerificationResultDenied
		eventResult = &result
	}

	verificationEvent := &domain.VerificationEvent{
		ID:               auditID,
		OrganizationID:   mcp.OrganizationID,
		MCPServerID:      &mcpID,
		MCPServerName:    &mcp.Name,
		Protocol:         domain.VerificationProtocolMCP,
		VerificationType: domain.VerificationTypeCapability,
		Status:           eventStatus,
		Result:           eventResult,
		Confidence:       1.0,
		TrustScore:       mcp.TrustScore,
		InitiatorType:    domain.InitiatorTypeSystem,
		Action:           &capability,
		ResourceType:     &targetService,
		ResourceID:       &resource,
		StartedAt:        now,
		Details:          &reason,
		Metadata:         metadata,
		CreatedAt:        now,
	}

	// Non-blocking - don't fail the action if audit fails
	go func() {
		if err := s.verificationEventRepo.Create(verificationEvent); err != nil {
			// Log error but don't fail the action
			fmt.Printf("Warning: Failed to create MCP action audit log: %v\n", err)
		}
	}()

	return allowed, reason, auditID, nil
}

// ========================================
// Agent Connection Tracking
// ========================================

// ConnectedAgent represents an agent that uses an MCP server
type ConnectedAgent struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"displayName"`
	Status      string    `json:"status"`
	TrustScore  float64   `json:"trustScore"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// GetConnectedAgents returns all agents that use a specific MCP server
// This enables the global MCP registry to show which agents are connected
func (s *MCPService) GetConnectedAgents(ctx context.Context, mcpServerID uuid.UUID) ([]ConnectedAgent, error) {
	// 1. Get the MCP server to find its name
	mcpServer, err := s.mcpRepo.GetByID(mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("MCP server not found: %w", err)
	}

	// 2. Get all agents in the same organization
	allAgents, err := s.agentRepo.GetByOrganization(mcpServer.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agents: %w", err)
	}

	// 3. Filter agents that have this MCP server in their talks_to list
	connectedAgents := []ConnectedAgent{}
	for _, agent := range allAgents {
		// Skip if agent has no talks_to list
		if agent.TalksTo == nil {
			continue
		}

		// Check if this MCP server is in the talks_to list
		// Match by both ID and name for flexibility
		isConnected := false
		for _, mcpIdentifier := range agent.TalksTo {
			if mcpIdentifier == mcpServer.ID.String() || mcpIdentifier == mcpServer.Name {
				isConnected = true
				break
			}
		}

		if isConnected {
			connectedAgents = append(connectedAgents, ConnectedAgent{
				ID:          agent.ID,
				Name:        agent.Name,
				DisplayName: agent.DisplayName,
				Status:      string(agent.Status),
				TrustScore:  agent.TrustScore,
				UpdatedAt:   agent.UpdatedAt,
			})
		}
	}

	return connectedAgents, nil
}

// GetConnectedAgentsCount returns the count of agents using an MCP server
func (s *MCPService) GetConnectedAgentsCount(ctx context.Context, mcpServerID uuid.UUID) (int, error) {
	agents, err := s.GetConnectedAgents(ctx, mcpServerID)
	if err != nil {
		return 0, err
	}
	return len(agents), nil
}

// updateAgentTalksTo adds an MCP server name to an agent's talks_to field
// This ensures GetMCPServerAgents can find the connection
func (s *MCPService) updateAgentTalksTo(agentID uuid.UUID, mcpServerName string) {
	if s.agentRepo == nil {
		return
	}

	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		fmt.Printf("⚠️  Warning: Failed to get agent %s for talks_to update: %v\n", agentID, err)
		return
	}

	// Initialize talks_to if nil
	if agent.TalksTo == nil {
		agent.TalksTo = []string{}
	}

	// Sanitize existing talks_to entries (fix any malformed entries from legacy registrations)
	agent.TalksTo = sanitizeTalksToEntries(agent.TalksTo)

	// Check if MCP server is already in talks_to
	for _, existing := range agent.TalksTo {
		if existing == mcpServerName {
			// Already present, no update needed (but still save sanitized version)
			if err := s.agentRepo.Update(agent); err != nil {
				fmt.Printf("⚠️  Warning: Failed to sanitize agent %s talks_to: %v\n", agentID, err)
			}
			return
		}
	}

	// Add the MCP server name to talks_to
	agent.TalksTo = append(agent.TalksTo, mcpServerName)

	if err := s.agentRepo.Update(agent); err != nil {
		fmt.Printf("⚠️  Warning: Failed to update agent %s talks_to: %v\n", agentID, err)
	} else {
		fmt.Printf("✅ Updated agent %s talks_to with MCP server %s\n", agentID, mcpServerName)
	}
}

// sanitizeTalksToEntries cleans up malformed talks_to entries
// - Splits comma-separated values into individual entries
// - Trims whitespace from each entry
// - Removes empty entries
func sanitizeTalksToEntries(talksTo []string) []string {
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
