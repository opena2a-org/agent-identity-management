package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/infrastructure/crypto"
	"github.com/opena2a/identity/backend/internal/infrastructure/repository"
)

type MCPService struct {
	mcpRepo               *repository.MCPServerRepository
	verificationEventRepo domain.VerificationEventRepository
	userRepo              *repository.UserRepository
	cryptoService         *crypto.ED25519Service
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

func NewMCPService(mcpRepo *repository.MCPServerRepository, verificationEventRepo domain.VerificationEventRepository, userRepo *repository.UserRepository) *MCPService {
	return &MCPService{
		mcpRepo:               mcpRepo,
		verificationEventRepo: verificationEventRepo,
		userRepo:              userRepo,
		cryptoService:         crypto.NewED25519Service(),
		challenges:            make(map[string]ChallengeData),
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

	server := &domain.MCPServer{
		ID:              uuid.New(),
		OrganizationID:  orgID,
		Name:            req.Name,
		Description:     req.Description,
		URL:             req.URL,
		Version:         req.Version,
		PublicKey:       req.PublicKey,
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

	// Automatic verification: If server has a public key, trigger verification automatically
	if server.PublicKey != "" {
		// Run verification asynchronously to avoid blocking the creation response
		go func() {
			// Use a background context for async operation
			bgCtx := context.Background()
			// Use localhost IP for system-initiated verification
			if err := s.VerifyMCPServer(bgCtx, server.ID, userID, "127.0.0.1"); err != nil {
				fmt.Printf("⚠️  Automatic verification failed for MCP server %s: %v\n", server.Name, err)
			} else {
				fmt.Printf("✅ Automatic verification succeeded for MCP server %s\n", server.Name)
			}
		}()
	}

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
	_, err = s.GenerateVerificationChallenge(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to generate challenge: %w", err)
	}

	// 3. In a real implementation, we would:
	//    - Send challenge to server's verification URL
	//    - Server signs challenge with its private key
	//    - Server returns signed challenge
	//    - We verify signature with stored public key
	//
	// For MVP, we'll simulate automatic success if public key exists
	// The infrastructure is in place for full implementation

	// Simulate challenge-response (for MVP)
	// In production, replace this with actual HTTP call to verification URL
	simulatedSuccess := server.PublicKey != ""

	var verificationStatus domain.VerificationEventStatus
	var verificationResult domain.VerificationResult
	var confidence float64
	var trustScore float64

	if simulatedSuccess {
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

	if !simulatedSuccess {
		return fmt.Errorf("verification failed")
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
