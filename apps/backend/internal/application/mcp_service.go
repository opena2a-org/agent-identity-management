package application

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/crypto"
	"github.com/opena2a/identity/backend/internal/domain"
	infracrypto "github.com/opena2a/identity/backend/internal/infrastructure/crypto"
	"github.com/opena2a/identity/backend/internal/infrastructure/repository"
)

type MCPService struct {
	mcpRepo               *repository.MCPServerRepository
	verificationEventRepo domain.VerificationEventRepository
	userRepo              *repository.UserRepository
	cryptoService         *infracrypto.ED25519Service
	keyVault              *crypto.KeyVault       // ✅ For secure private key storage
	capabilityService     *MCPCapabilityService  // ✅ For automatic capability detection
	httpClient            *http.Client           // ✅ For real MCP server communication
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

func NewMCPService(mcpRepo *repository.MCPServerRepository, verificationEventRepo domain.VerificationEventRepository, userRepo *repository.UserRepository, keyVault *crypto.KeyVault, capabilityService *MCPCapabilityService) *MCPService {
	return &MCPService{
		mcpRepo:               mcpRepo,
		verificationEventRepo: verificationEventRepo,
		userRepo:              userRepo,
		cryptoService:         infracrypto.NewED25519Service(),
		keyVault:              keyVault,
		capabilityService:     capabilityService,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // 30 second timeout for MCP server communication
		},
		challenges: make(map[string]ChallengeData),
	}
}

// CreateMCPServerRequest represents the request to create an MCP server
type CreateMCPServerRequest struct {
	Name            string   `json:"name" validate:"required"`
	Description     string   `json:"description"`
	URL             string   `json:"url" validate:"required,url"`
	Version         string   `json:"version"`
	PublicKey       string   `json:"public_key"`
	VerificationURL string   `json:"verification_url"`
	Capabilities    []string `json:"capabilities"`
}

// UpdateMCPServerRequest represents the request to update an MCP server
type UpdateMCPServerRequest struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	URL             string   `json:"url"`
	Version         string   `json:"version"`
	PublicKey       string   `json:"public_key"`
	VerificationURL string   `json:"verification_url"`
	Capabilities    []string `json:"capabilities"`
}

// AddPublicKeyRequest represents the request to add a public key
type AddPublicKeyRequest struct {
	PublicKey string `json:"public_key" validate:"required"`
	KeyType   string `json:"key_type" validate:"required"` // e.g., "rsa", "ed25519"
}

// CreateMCPServer creates a new MCP server
func (s *MCPService) CreateMCPServer(ctx context.Context, req *CreateMCPServerRequest, orgID, userID uuid.UUID) (*domain.MCPServer, error) {
	// Check if MCP server with this URL already exists
	existing, _ := s.mcpRepo.GetByURL(req.URL)
	if existing != nil {
		return nil, fmt.Errorf("mcp server with this URL already exists")
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

	server := &domain.MCPServer{
		ID:              uuid.New(),
		OrganizationID:  orgID,
		Name:            req.Name,
		Description:     req.Description,
		URL:             req.URL,
		Version:         req.Version,
		PublicKey:       publicKey, // ✅ Auto-generated if not provided
		Status:          domain.MCPServerStatusPending,
		IsVerified:      false,
		VerificationURL: req.VerificationURL,
		Capabilities:    req.Capabilities,
		TrustScore:      0.0,
		CreatedBy:       userID,
	}

	if err := s.mcpRepo.Create(server); err != nil {
		return nil, err
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

	return server, nil
}

// GetMCPServer retrieves an MCP server by ID
func (s *MCPService) GetMCPServer(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error) {
	return s.mcpRepo.GetByID(id)
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
		server.Name = req.Name
	}
	if req.Description != "" {
		server.Description = req.Description
	}
	if req.URL != "" {
		server.URL = req.URL
	}
	if req.Version != "" {
		server.Version = req.Version
	}
	if req.PublicKey != "" {
		server.PublicKey = req.PublicKey
	}
	if req.VerificationURL != "" {
		server.VerificationURL = req.VerificationURL
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

	// Step 3a: Send challenge to MCP server
	challengeReq := map[string]string{
		"challenge": challenge,
		"server_id": id.String(),
	}
	reqBody, err := json.Marshal(challengeReq)
	if err != nil {
		return fmt.Errorf("failed to marshal challenge request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", server.VerificationURL, bytes.NewBuffer(reqBody))
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

	// Step 3b: Parse signed challenge response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read verification response: %w", err)
	}

	var verifyResp struct {
		SignedChallenge string `json:"signed_challenge"`
	}
	if err := json.Unmarshal(respBody, &verifyResp); err != nil {
		return fmt.Errorf("failed to parse verification response: %w", err)
	}

	// Step 3c: Verify the signed challenge using Ed25519
	if err := s.VerifyChallengeResponse(ctx, id, verifyResp.SignedChallenge); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	// ✅ Verification successful - cryptographic proof established
	verificationSuccess := true

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
		server.TrustScore = 75.0 // Initial trust score for verified servers

		if err := s.mcpRepo.Update(server); err != nil {
			return err
		}

		// ✅ AUTOMATIC CAPABILITY DETECTION
		// After successful verification, automatically detect MCP server capabilities
		if s.capabilityService != nil {
			go func() {
				// Run asynchronously to avoid blocking verification
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

// VerifyMCPAction verifies if an MCP server can perform an action
func (s *MCPService) VerifyMCPAction(
	ctx context.Context,
	mcpID uuid.UUID,
	actionType string,
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

	// 4. Audit log
	auditID = uuid.New()
	// TODO: Implement proper audit logging
	// This should create an audit log entry tracking:
	// - Which MCP server performed what action
	// - The target resource/service
	// - The decision (allowed/denied)
	// - Timestamp and metadata

	return allowed, reason, auditID, nil
}
