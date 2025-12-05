package application

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	infracrypto "github.com/opena2a/identity/backend/internal/infrastructure/crypto"
	"github.com/opena2a/identity/backend/internal/infrastructure/repository"
)

// Multi-Agent Consensus Configuration
// These thresholds determine when an MCP server is considered "verified" by the community
const (
	// DefaultMinAgentsForConsensus is the minimum number of unique agents required
	// to verify an MCP server. Can be overridden via MCP_MIN_AGENTS_FOR_CONSENSUS env var.
	DefaultMinAgentsForConsensus = 3

	// DefaultMinOwnersForConsensus is the minimum number of unique agent owners required.
	// This ensures attestations come from truly independent sources.
	// Can be overridden via MCP_MIN_OWNERS_FOR_CONSENSUS env var.
	DefaultMinOwnersForConsensus = 2

	// MinConfidenceScoreForVerification is the minimum confidence score required
	// for automatic verification via consensus.
	MinConfidenceScoreForVerification = 60.0
)

// getMinAgentsForConsensus returns the configured minimum agents for consensus
func getMinAgentsForConsensus() int {
	if val := os.Getenv("MCP_MIN_AGENTS_FOR_CONSENSUS"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			return n
		}
	}
	return DefaultMinAgentsForConsensus
}

// getMinOwnersForConsensus returns the configured minimum owners for consensus
func getMinOwnersForConsensus() int {
	if val := os.Getenv("MCP_MIN_OWNERS_FOR_CONSENSUS"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			return n
		}
	}
	return DefaultMinOwnersForConsensus
}

// MCPAttestationService handles Agent Attestation operations
type MCPAttestationService struct {
	attestationRepo *repository.MCPAttestationRepository
	agentRepo       *repository.AgentRepository
	mcpRepo         *repository.MCPServerRepository
	userRepo        *repository.UserRepository
	connectionRepo  *repository.AgentMCPConnectionRepository
	cryptoService   *infracrypto.ED25519Service
}

func NewMCPAttestationService(
	attestationRepo *repository.MCPAttestationRepository,
	agentRepo *repository.AgentRepository,
	mcpRepo *repository.MCPServerRepository,
	userRepo *repository.UserRepository,
	connectionRepo *repository.AgentMCPConnectionRepository,
) *MCPAttestationService {
	return &MCPAttestationService{
		attestationRepo: attestationRepo,
		agentRepo:       agentRepo,
		mcpRepo:         mcpRepo,
		userRepo:        userRepo,
		connectionRepo:  connectionRepo,
		cryptoService:   infracrypto.NewED25519Service(),
	}
}

// AttestMCPRequest represents the request to attest an MCP server
type AttestMCPRequest struct {
	Attestation domain.AttestationPayload `json:"attestation"`
	Signature   string                     `json:"signature"`
}

// AttestMCPResponse represents the response after attestation
type AttestMCPResponse struct {
	Success            bool    `json:"success"`
	AttestationID      string  `json:"attestationId"`
	MCPConfidenceScore float64 `json:"mcpConfidenceScore"`
	AttestationCount   int     `json:"attestationCount"`
	Message            string  `json:"message"`
}

// ChallengeResponse represents a server-generated challenge for proof of key possession
type ChallengeResponse struct {
	Challenge   string    `json:"challenge"`    // Base64-encoded random nonce
	ExpiresAt   time.Time `json:"expiresAt"`    // When the challenge expires (5 minutes)
	MCPServerID string    `json:"mcpServerId"`  // MCP server this challenge is for
}

// GenerateChallenge creates a server-side nonce for proof of private key possession
// The agent must include this challenge in their signed attestation payload
func (s *MCPAttestationService) GenerateChallenge(
	ctx context.Context,
	agentID uuid.UUID,
	mcpServerID uuid.UUID,
) (*ChallengeResponse, error) {
	// Verify agent exists and is verified
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	if agent.Status != domain.AgentStatusVerified {
		return nil, fmt.Errorf("only verified agents can request attestation challenges")
	}

	// Verify MCP server exists
	if _, err := s.mcpRepo.GetByID(mcpServerID); err != nil {
		return nil, fmt.Errorf("mcp server not found: %w", err)
	}

	// Generate cryptographically secure random nonce (32 bytes)
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate challenge nonce: %w", err)
	}

	challenge := base64.StdEncoding.EncodeToString(nonce)
	now := time.Now().UTC()
	expiresAt := now.Add(5 * time.Minute)

	// Store challenge in database
	challengeRecord := &domain.AttestationChallenge{
		ID:          uuid.New(),
		Challenge:   challenge,
		AgentID:     agentID,
		MCPServerID: mcpServerID,
		ExpiresAt:   expiresAt,
		CreatedAt:   now,
	}

	if err := s.attestationRepo.CreateChallenge(challengeRecord); err != nil {
		return nil, fmt.Errorf("failed to store challenge: %w", err)
	}

	fmt.Printf("🔐 Generated attestation challenge for agent %s -> MCP %s (expires: %s)\n",
		agentID, mcpServerID, expiresAt.Format(time.RFC3339))

	return &ChallengeResponse{
		Challenge:   challenge,
		ExpiresAt:   expiresAt,
		MCPServerID: mcpServerID.String(),
	}, nil
}

// VerifyAndRecordAttestation verifies and records an agent's attestation of an MCP server
func (s *MCPAttestationService) VerifyAndRecordAttestation(
	ctx context.Context,
	mcpServerID uuid.UUID,
	req *AttestMCPRequest,
) (*AttestMCPResponse, error) {
	// 1. Parse agent ID from attestation
	agentID, err := uuid.Parse(req.Attestation.AgentID)
	if err != nil {
		return nil, fmt.Errorf("invalid agent_id in attestation: %w", err)
	}

	// 2. Fetch agent (MUST be Ed25519 verified)
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	fmt.Printf("🔍 Agent fetched from DB: ID=%s, Status=%s, VerifiedAt=%v\n", agent.ID, agent.Status, agent.VerifiedAt)

	if agent.Status != domain.AgentStatusVerified {
		fmt.Printf("❌ Agent status check failed: expected %s, got %s\n", domain.AgentStatusVerified, agent.Status)
		return nil, fmt.Errorf("only verified agents can attest MCPs (agent status: %s)", agent.Status)
	}

	fmt.Printf("✅ Agent status check passed: %s\n", agent.Status)

	if agent.PublicKey == nil || *agent.PublicKey == "" {
		return nil, fmt.Errorf("agent has no public key registered")
	}

	// 3. Verify signature using agent's public key
	attestationJSON, err := req.Attestation.ToCanonicalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize attestation: %w", err)
	}

	// Debug: Print what we're verifying
	fmt.Printf("🔍 Backend attestation verification:\n")
	fmt.Printf("   Canonical JSON (first 200): %s\n", string(attestationJSON)[:min(200, len(attestationJSON))])
	fmt.Printf("   Signature (first 40): %s\n", req.Signature[:min(40, len(req.Signature))])
	fmt.Printf("   Agent public key (first 20): %s\n", (*agent.PublicKey)[:min(20, len(*agent.PublicKey))])

	valid, err := s.cryptoService.Verify(*agent.PublicKey, attestationJSON, req.Signature)
	if err != nil {
		fmt.Printf("❌ Crypto verification error: %v\n", err)
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}

	if !valid {
		fmt.Printf("❌ Signature verification returned FALSE\n")
		return nil, fmt.Errorf("invalid attestation signature")
	}

	fmt.Printf("✅ Attestation signature verification PASSED\n")

	// 4. PROOF OF KEY POSSESSION: Validate server-generated challenge (prevents replay attacks)
	// If a challenge is provided, validate it. For backward compatibility, challenge is optional
	// but required for new attestations (when challenge is present in payload)
	var challengeRecord *domain.AttestationChallenge
	if req.Attestation.Challenge != "" {
		challengeRecord, err = s.attestationRepo.GetChallengeByValue(req.Attestation.Challenge)
		if err != nil {
			fmt.Printf("❌ Challenge not found: %s\n", req.Attestation.Challenge)
			return nil, fmt.Errorf("invalid or unknown challenge - please request a new challenge")
		}

		// Validate challenge hasn't expired
		if time.Now().UTC().After(challengeRecord.ExpiresAt) {
			fmt.Printf("❌ Challenge expired at %s\n", challengeRecord.ExpiresAt)
			return nil, fmt.Errorf("challenge expired - please request a new challenge")
		}

		// Validate challenge hasn't been used (replay attack prevention)
		if challengeRecord.UsedAt != nil {
			fmt.Printf("❌ Challenge already used at %s (replay attack detected)\n", challengeRecord.UsedAt)
			return nil, fmt.Errorf("challenge already used (possible replay attack)")
		}

		// Validate challenge was issued for this agent
		if challengeRecord.AgentID != agentID {
			fmt.Printf("❌ Challenge was issued for different agent: %s (expected %s)\n",
				challengeRecord.AgentID, agentID)
			return nil, fmt.Errorf("challenge was issued for a different agent")
		}

		// Validate challenge was issued for this MCP server
		if challengeRecord.MCPServerID != mcpServerID {
			fmt.Printf("❌ Challenge was issued for different MCP: %s (expected %s)\n",
				challengeRecord.MCPServerID, mcpServerID)
			return nil, fmt.Errorf("challenge was issued for a different MCP server")
		}

		fmt.Printf("✅ Challenge validated: proof of private key possession confirmed\n")
	} else {
		fmt.Printf("⚠️  No challenge provided (legacy attestation mode - consider upgrading SDK)\n")
	}

	// 5. Check attestation is recent (< 5 minutes old)
	attestationTime, err := time.Parse(time.RFC3339, req.Attestation.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp format: %w", err)
	}

	if time.Since(attestationTime) > 5*time.Minute {
		return nil, fmt.Errorf("attestation expired (older than 5 minutes)")
	}

	// 6. Verify MCP server exists and organization isolation
	mcpServer, err := s.mcpRepo.GetByID(mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("mcp server not found: %w", err)
	}

	// ORGANIZATION ISOLATION: Agent can only attest MCP servers in the same organization
	if mcpServer.OrganizationID != agent.OrganizationID {
		fmt.Printf("🔒 Organization isolation violation: agent %s (org: %s) attempted to attest MCP %s (org: %s)\n",
			agentID, agent.OrganizationID, mcpServerID, mcpServer.OrganizationID)
		return nil, fmt.Errorf("cannot attest MCP servers from other organizations")
	}

	// 7. Mark challenge as used (MUST happen before storing attestation to prevent race conditions)
	if challengeRecord != nil {
		if err := s.attestationRepo.MarkChallengeUsed(challengeRecord.ID); err != nil {
			return nil, fmt.Errorf("failed to mark challenge as used: %w", err)
		}
		fmt.Printf("✅ Challenge marked as used (replay protection active)\n")
	}

	// 8. Store attestation
	now := time.Now().UTC()
	attestation := &domain.MCPAttestation{
		ID:                uuid.New(),
		MCPServerID:       mcpServerID,
		AgentID:           &agentID, // Pointer for nullable support
		AttestationData:   req.Attestation,
		Signature:         req.Signature,
		SignatureVerified: true,
		VerifiedAt:        &now,
		ExpiresAt:         now.Add(30 * 24 * time.Hour), // 30 days
		IsValid:           true,
		CreatedAt:         now,
	}

	if err := s.attestationRepo.CreateAttestation(attestation); err != nil {
		return nil, fmt.Errorf("failed to store attestation: %w", err)
	}

	// 7. Update MCP confidence score
	confidenceScore, attestationCount, err := s.updateMCPConfidenceScore(ctx, mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("failed to update confidence score: %w", err)
	}

	// 8. Update or create agent-MCP connection
	if err := s.updateAgentMCPConnection(ctx, agentID, mcpServerID, now); err != nil {
		return nil, fmt.Errorf("failed to update agent-MCP connection: %w", err)
	}

	return &AttestMCPResponse{
		Success:            true,
		AttestationID:      attestation.ID.String(),
		MCPConfidenceScore: confidenceScore,
		AttestationCount:   attestationCount,
		Message:            "MCP attestation verified and recorded",
	}, nil
}

// updateMCPConfidenceScore calculates and updates the confidence score for an MCP server
// The scoring system weights SDK attestations (cryptographically verified) higher than manual attestations
// MULTI-AGENT CONSENSUS: If enough independent agents attest, the MCP is automatically verified
func (s *MCPAttestationService) updateMCPConfidenceScore(
	ctx context.Context,
	mcpServerID uuid.UUID,
) (float64, int, error) {
	// Get all valid attestations for this MCP
	attestations, err := s.attestationRepo.GetValidAttestationsByMCP(mcpServerID)
	if err != nil {
		return 0, 0, err
	}

	if len(attestations) == 0 {
		// No attestations - confidence is 0
		return 0, 0, nil
	}

	// Confidence calculation factors:
	// 1. SDK attestations (cryptographically verified): 20 points each, max 100 points
	// 2. Manual attestations (user-verified, no crypto): 5 points each, max 25 points
	// 3. Average trust score of attesting agents (0-50 points)
	// 4. Recency of attestations (0-25 points)

	uniqueAgents := make(map[uuid.UUID]bool)
	var totalTrust float64
	var sdkAttestations int
	var manualAttestations int
	var mostRecentAttestation time.Time

	for _, att := range attestations {
		// Track most recent attestation
		if att.VerifiedAt != nil && att.VerifiedAt.After(mostRecentAttestation) {
			mostRecentAttestation = *att.VerifiedAt
		}

		// Count SDK attestations (with agent_id and cryptographic signature)
		if att.AgentID != nil && att.Signature != "manual-attestation" {
			uniqueAgents[*att.AgentID] = true
			totalTrust += att.AgentTrustScore
			sdkAttestations++
		} else {
			// Manual attestation - lower trust weight
			manualAttestations++
		}
	}

	// Factor 1: SDK attestations with cryptographic proof (20 points each, max 100)
	// These are higher trust because they use challenge-response and Ed25519 signatures
	agentCount := len(uniqueAgents)
	sdkPoints := float64(agentCount) * 20.0
	if sdkPoints > 100.0 {
		sdkPoints = 100.0
	}

	// Factor 2: Manual attestations with reduced weight (5 points each, max 25)
	// These are lower trust because they lack cryptographic proof
	manualPoints := float64(manualAttestations) * 5.0
	if manualPoints > 25.0 {
		manualPoints = 25.0
	}

	// Factor 3: Average trust score of attesting agents (0-50 points)
	var trustPoints float64
	if sdkAttestations > 0 {
		avgTrust := totalTrust / float64(sdkAttestations)
		trustPoints = (avgTrust / 100.0) * 50.0 // Scale to 0-50
	}

	// Factor 4: Recency factor (% of attestations in last 7 days, 0-25 points)
	recentCount := 0
	for _, att := range attestations {
		if att.VerifiedAt != nil && time.Since(*att.VerifiedAt) < 7*24*time.Hour {
			recentCount++
		}
	}
	recencyFactor := float64(recentCount) / float64(len(attestations))
	recencyPoints := recencyFactor * 25.0

	// Calculate final confidence score (0-100)
	// Weighted sum: SDK (max 100) + Manual (max 25) + Trust (max 50) + Recency (max 25) = 200 max
	// Normalize to 0-100 scale
	rawScore := sdkPoints + manualPoints + trustPoints + recencyPoints
	confidenceScore := rawScore / 2.0 // Normalize from 0-200 to 0-100
	if confidenceScore > 100.0 {
		confidenceScore = 100.0
	}

	// Log the breakdown for debugging
	fmt.Printf("📊 MCP %s confidence score breakdown:\n", mcpServerID)
	fmt.Printf("   SDK attestations: %d (%.1f points)\n", sdkAttestations, sdkPoints)
	fmt.Printf("   Manual attestations: %d (%.1f points)\n", manualAttestations, manualPoints)
	fmt.Printf("   Trust score: %.1f points\n", trustPoints)
	fmt.Printf("   Recency: %.1f points\n", recencyPoints)
	fmt.Printf("   Final score: %.2f%%\n", confidenceScore)

	// Update MCP server
	err = s.attestationRepo.UpdateMCPConfidenceScore(
		mcpServerID,
		confidenceScore,
		len(attestations),
		mostRecentAttestation,
	)
	if err != nil {
		return 0, 0, err
	}

	// MULTI-AGENT CONSENSUS: Check if MCP should be auto-verified
	// Requirements:
	// 1. Minimum number of unique agents
	// 2. Minimum number of unique agent owners (independence)
	// 3. Confidence score above threshold
	if err := s.checkAndApplyConsensusVerification(ctx, mcpServerID, confidenceScore); err != nil {
		// Log but don't fail - attestation was still recorded successfully
		fmt.Printf("⚠️  Warning: failed to check consensus verification: %v\n", err)
	}

	return confidenceScore, len(attestations), nil
}

// checkAndApplyConsensusVerification checks if an MCP server has reached multi-agent consensus
// and automatically updates its status to "verified" if all criteria are met
func (s *MCPAttestationService) checkAndApplyConsensusVerification(
	ctx context.Context,
	mcpServerID uuid.UUID,
	confidenceScore float64,
) error {
	// Check current MCP status - skip if already verified
	mcpServer, err := s.mcpRepo.GetByID(mcpServerID)
	if err != nil {
		return fmt.Errorf("failed to get MCP server: %w", err)
	}

	if mcpServer.Status == domain.MCPServerStatusVerified {
		// Already verified - nothing to do
		return nil
	}

	// Get consensus thresholds
	minAgents := getMinAgentsForConsensus()
	minOwners := getMinOwnersForConsensus()

	// Check 1: Minimum unique agents
	uniqueAgentCount, err := s.attestationRepo.GetUniqueAttestingAgentCount(mcpServerID)
	if err != nil {
		return fmt.Errorf("failed to get unique agent count: %w", err)
	}

	if uniqueAgentCount < minAgents {
		fmt.Printf("🔐 MCP %s: Consensus not reached - need %d agents, have %d\n",
			mcpServerID, minAgents, uniqueAgentCount)
		return nil
	}

	// Check 2: Minimum unique agent owners (for true independence)
	uniqueOwnerCount, err := s.attestationRepo.GetUniqueAgentOwnerCount(mcpServerID)
	if err != nil {
		return fmt.Errorf("failed to get unique owner count: %w", err)
	}

	if uniqueOwnerCount < minOwners {
		fmt.Printf("🔐 MCP %s: Consensus not reached - need %d owners, have %d\n",
			mcpServerID, minOwners, uniqueOwnerCount)
		return nil
	}

	// Check 3: Confidence score threshold
	if confidenceScore < MinConfidenceScoreForVerification {
		fmt.Printf("🔐 MCP %s: Consensus not reached - need %.1f%% confidence, have %.1f%%\n",
			mcpServerID, MinConfidenceScoreForVerification, confidenceScore)
		return nil
	}

	// 🎉 All consensus criteria met! Auto-verify the MCP server
	fmt.Printf("🎉 MULTI-AGENT CONSENSUS REACHED for MCP %s:\n", mcpServerID)
	fmt.Printf("   ✓ Unique agents: %d (required: %d)\n", uniqueAgentCount, minAgents)
	fmt.Printf("   ✓ Unique owners: %d (required: %d)\n", uniqueOwnerCount, minOwners)
	fmt.Printf("   ✓ Confidence: %.1f%% (required: %.1f%%)\n", confidenceScore, MinConfidenceScoreForVerification)
	fmt.Printf("   → Auto-verifying MCP server\n")

	// Update MCP status to verified
	err = s.attestationRepo.UpdateMCPVerificationStatus(
		mcpServerID,
		string(domain.MCPServerStatusVerified),
		true,
	)
	if err != nil {
		return fmt.Errorf("failed to update MCP verification status: %w", err)
	}

	fmt.Printf("✅ MCP server %s automatically verified via multi-agent consensus\n", mcpServerID)
	return nil
}

// ConsensusStatus represents the current multi-agent consensus status for an MCP server
type ConsensusStatus struct {
	MCPServerID         string  `json:"mcpServerId"`
	Status              string  `json:"status"`            // "pending", "verified", etc.
	IsVerified          bool    `json:"isVerified"`
	UniqueAgents        int     `json:"uniqueAgents"`      // Current count
	RequiredAgents      int     `json:"requiredAgents"`    // Threshold
	UniqueOwners        int     `json:"uniqueOwners"`      // Current count
	RequiredOwners      int     `json:"requiredOwners"`    // Threshold
	ConfidenceScore     float64 `json:"confidenceScore"`   // Current score
	RequiredConfidence  float64 `json:"requiredConfidence"`// Threshold
	ConsensusReached    bool    `json:"consensusReached"`  // All criteria met
	ProgressPercent     float64 `json:"progressPercent"`   // Overall progress 0-100
	MissingCriteria     []string `json:"missingCriteria"`  // What's still needed
}

// GetConsensusStatus returns the current multi-agent consensus status for an MCP server
// Useful for dashboard to show verification progress
func (s *MCPAttestationService) GetConsensusStatus(
	ctx context.Context,
	mcpServerID uuid.UUID,
) (*ConsensusStatus, error) {
	// Get MCP server details
	mcpServer, err := s.mcpRepo.GetByID(mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("mcp server not found: %w", err)
	}

	// Get thresholds
	minAgents := getMinAgentsForConsensus()
	minOwners := getMinOwnersForConsensus()

	// Get current counts
	uniqueAgentCount, err := s.attestationRepo.GetUniqueAttestingAgentCount(mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get unique agent count: %w", err)
	}

	uniqueOwnerCount, err := s.attestationRepo.GetUniqueAgentOwnerCount(mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get unique owner count: %w", err)
	}

	// Calculate progress and missing criteria
	var missingCriteria []string
	agentsProgress := float64(uniqueAgentCount) / float64(minAgents) * 100
	if agentsProgress > 100 {
		agentsProgress = 100
	}
	if uniqueAgentCount < minAgents {
		missingCriteria = append(missingCriteria,
			fmt.Sprintf("Need %d more agent attestations", minAgents-uniqueAgentCount))
	}

	ownersProgress := float64(uniqueOwnerCount) / float64(minOwners) * 100
	if ownersProgress > 100 {
		ownersProgress = 100
	}
	if uniqueOwnerCount < minOwners {
		missingCriteria = append(missingCriteria,
			fmt.Sprintf("Need attestations from %d more independent owners", minOwners-uniqueOwnerCount))
	}

	confidenceProgress := mcpServer.ConfidenceScore / MinConfidenceScoreForVerification * 100
	if confidenceProgress > 100 {
		confidenceProgress = 100
	}
	if mcpServer.ConfidenceScore < MinConfidenceScoreForVerification {
		missingCriteria = append(missingCriteria,
			fmt.Sprintf("Need %.1f%% more confidence score", MinConfidenceScoreForVerification-mcpServer.ConfidenceScore))
	}

	// Overall progress is the minimum of all criteria (all must be met)
	overallProgress := agentsProgress
	if ownersProgress < overallProgress {
		overallProgress = ownersProgress
	}
	if confidenceProgress < overallProgress {
		overallProgress = confidenceProgress
	}

	consensusReached := len(missingCriteria) == 0

	return &ConsensusStatus{
		MCPServerID:        mcpServerID.String(),
		Status:             string(mcpServer.Status),
		IsVerified:         mcpServer.IsVerified,
		UniqueAgents:       uniqueAgentCount,
		RequiredAgents:     minAgents,
		UniqueOwners:       uniqueOwnerCount,
		RequiredOwners:     minOwners,
		ConfidenceScore:    mcpServer.ConfidenceScore,
		RequiredConfidence: MinConfidenceScoreForVerification,
		ConsensusReached:   consensusReached,
		ProgressPercent:    overallProgress,
		MissingCriteria:    missingCriteria,
	}, nil
}

// updateAgentMCPConnection updates or creates the connection between agent and MCP
func (s *MCPAttestationService) updateAgentMCPConnection(
	ctx context.Context,
	agentID uuid.UUID,
	mcpServerID uuid.UUID,
	attestedAt time.Time,
) error {
	// Check if connection already exists
	connection, err := s.attestationRepo.GetConnectionByAgentAndMCP(agentID, mcpServerID)
	if err != nil {
		return err
	}

	if connection == nil {
		// Create new connection
		connection = &domain.AgentMCPConnection{
			ID:               uuid.New(),
			AgentID:          agentID,
			MCPServerID:      mcpServerID,
			ConnectionType:   domain.ConnectionTypeAttested,
			FirstConnectedAt: attestedAt,
			LastAttestedAt:   &attestedAt,
			AttestationCount: 1,
			IsActive:         true,
			CreatedAt:        attestedAt,
			UpdatedAt:        attestedAt,
		}

		return s.attestationRepo.CreateConnection(connection)
	}

	// Update existing connection
	connection.LastAttestedAt = &attestedAt
	connection.AttestationCount++
	connection.IsActive = true
	connection.ConnectionType = domain.ConnectionTypeAttested

	return s.attestationRepo.UpdateConnection(connection)
}

// GetMCPAttestations retrieves all attestations for an MCP server
func (s *MCPAttestationService) GetMCPAttestations(
	ctx context.Context,
	mcpServerID uuid.UUID,
) ([]*domain.AttestationWithAgentDetails, float64, time.Time, error) {
	// Get MCP server to verify it exists
	mcpServer, err := s.mcpRepo.GetByID(mcpServerID)
	if err != nil {
		return nil, 0, time.Time{}, fmt.Errorf("mcp server not found: %w", err)
	}

	// Get all attestations (both valid and expired for historical view)
	attestations, err := s.attestationRepo.GetAttestationsByMCP(mcpServerID)
	if err != nil {
		return nil, 0, time.Time{}, err
	}

	// Convert to response format with enriched metadata
	var result []*domain.AttestationWithAgentDetails
	var lastAttestedAt time.Time

	for _, att := range attestations {
		if att.VerifiedAt != nil && att.VerifiedAt.After(lastAttestedAt) {
			lastAttestedAt = *att.VerifiedAt
		}

		var verifiedAtStr, expiresAtStr string
		if att.VerifiedAt != nil {
			verifiedAtStr = att.VerifiedAt.Format(time.RFC3339)
		}
		expiresAtStr = att.ExpiresAt.Format(time.RFC3339)

		// Determine attestation type and who performed it
		var attestationType, attestedBy, attesterType string

		// Check if this is a manual attestation (signature == "manual-attestation")
		if att.Signature == "manual-attestation" {
			attestationType = "manual"
			attesterType = "user"

			// Parse user ID from AttestationData.AgentID (for manual attestations, this holds the userID)
			if userID, err := uuid.Parse(att.AttestationData.AgentID); err == nil {
				// Fetch user details
				if user, err := s.userRepo.GetByID(userID); err == nil && user != nil {
					attestedBy = user.Name
				} else {
					attestedBy = "Unknown User"
				}
			} else {
				attestedBy = "Unknown User"
			}
		} else {
			// SDK/Agent attestation
			attestationType = "sdk"
			attesterType = "agent"

			var agentOwnerName string
			var agentOwnerID uuid.UUID

			if att.AgentName != "" {
				attestedBy = att.AgentName
			} else if att.AgentID != nil {
				// Fallback: fetch agent name if not already populated
				if agent, err := s.agentRepo.GetByID(*att.AgentID); err == nil && agent != nil {
					attestedBy = agent.Name
				} else {
					attestedBy = "Unknown Agent"
				}
			} else {
				attestedBy = "Unknown Agent"
			}

			// Fetch agent owner information for SDK attestations
			if att.AgentID != nil {
				if agent, err := s.agentRepo.GetByID(*att.AgentID); err == nil && agent != nil {
					agentOwnerID = agent.CreatedBy
					// Fetch the user who created/owns this agent
					if owner, err := s.userRepo.GetByID(agentOwnerID); err == nil && owner != nil {
						agentOwnerName = owner.Name
					}
				}
			}

			// For SDK attestations, dereference the AgentID pointer
			agentIDValue := uuid.Nil
			if att.AgentID != nil {
				agentIDValue = *att.AgentID
			}

			result = append(result, &domain.AttestationWithAgentDetails{
				ID:                    att.ID,
				AgentID:               agentIDValue,
				AgentName:             att.AgentName,
				AgentTrustScore:       att.AgentTrustScore,
				VerifiedAt:            verifiedAtStr,
				ExpiresAt:             expiresAtStr,
				CapabilitiesConfirmed: att.AttestationData.CapabilitiesFound,
				ConnectionLatencyMs:   att.AttestationData.ConnectionLatencyMs,
				HealthCheckPassed:     att.AttestationData.HealthCheckPassed,
				IsValid:               att.IsValid,

				// New metadata fields
				AttestationType:      attestationType,
				AttestedBy:           attestedBy,
				AttesterType:         attesterType,
				SignatureVerified:    att.SignatureVerified,
				SDKVersion:           att.AttestationData.SDKVersion,
				ConnectionSuccessful: att.AttestationData.ConnectionSuccessful,
				AgentOwnerName:       agentOwnerName,
				AgentOwnerID:         agentOwnerID,
			})
			continue
		}

		// For manual attestations, use uuid.Nil since there's no agent
		result = append(result, &domain.AttestationWithAgentDetails{
			ID:                    att.ID,
			AgentID:               uuid.Nil,
			AgentName:             att.AgentName,
			AgentTrustScore:       att.AgentTrustScore,
			VerifiedAt:            verifiedAtStr,
			ExpiresAt:             expiresAtStr,
			CapabilitiesConfirmed: att.AttestationData.CapabilitiesFound,
			ConnectionLatencyMs:   att.AttestationData.ConnectionLatencyMs,
			HealthCheckPassed:     att.AttestationData.HealthCheckPassed,
			IsValid:               att.IsValid,

			// New metadata fields
			AttestationType:      attestationType,
			AttestedBy:           attestedBy,
			AttesterType:         attesterType,
			SignatureVerified:    att.SignatureVerified,
			SDKVersion:           att.AttestationData.SDKVersion,
			ConnectionSuccessful: att.AttestationData.ConnectionSuccessful,
		})
	}

	return result, mcpServer.ConfidenceScore, lastAttestedAt, nil
}

// GetConnectedAgentsForMCP retrieves all agents connected to an MCP server
func (s *MCPAttestationService) GetConnectedAgentsForMCP(
	ctx context.Context,
	mcpServerID uuid.UUID,
) ([]*domain.Agent, error) {
	// Get all connections for this MCP
	connections, err := s.attestationRepo.GetConnectionsByMCP(mcpServerID)
	if err != nil {
		return nil, err
	}

	// Fetch agent details
	var agents []*domain.Agent
	for _, conn := range connections {
		if !conn.IsActive {
			continue // Skip inactive connections
		}

		agent, err := s.agentRepo.GetByID(conn.AgentID)
		if err != nil {
			continue // Skip if agent not found
		}

		agents = append(agents, agent)
	}

	return agents, nil
}

// GetMCPServersForAgent retrieves all MCP servers connected to an agent
func (s *MCPAttestationService) GetMCPServersForAgent(
	ctx context.Context,
	agentID uuid.UUID,
) ([]*domain.MCPServer, error) {
	// Get all connections for this agent
	connections, err := s.attestationRepo.GetConnectionsByAgent(agentID)
	if err != nil {
		return nil, err
	}

	// Fetch MCP server details
	var mcpServers []*domain.MCPServer
	for _, conn := range connections {
		if !conn.IsActive {
			continue // Skip inactive connections
		}

		mcpServer, err := s.mcpRepo.GetByID(conn.MCPServerID)
		if err != nil {
			continue // Skip if MCP not found
		}

		mcpServers = append(mcpServers, mcpServer)
	}

	return mcpServers, nil
}

// InvalidateExpiredAttestations is a background job to invalidate expired attestations
func (s *MCPAttestationService) InvalidateExpiredAttestations(ctx context.Context) error {
	return s.attestationRepo.InvalidateExpiredAttestations()
}

// RevokeAttestation revokes a specific attestation with a reason (supply chain security)
// This is critical for incident response when an agent's private key is compromised
func (s *MCPAttestationService) RevokeAttestation(
	ctx context.Context,
	attestationID uuid.UUID,
	revokerID uuid.UUID,
	reason string,
) error {
	// Get the attestation to find the MCP server ID
	attestation, err := s.attestationRepo.GetAttestationByID(attestationID)
	if err != nil {
		return fmt.Errorf("attestation not found: %w", err)
	}

	// Invalidate the attestation
	if err := s.attestationRepo.InvalidateAttestation(attestationID); err != nil {
		return fmt.Errorf("failed to revoke attestation: %w", err)
	}

	// Recalculate MCP confidence score after revocation
	_, _, err = s.updateMCPConfidenceScore(ctx, attestation.MCPServerID)
	if err != nil {
		// Log but don't fail - revocation was successful
		fmt.Printf("⚠️  Warning: failed to recalculate confidence score after revocation: %v\n", err)
	}

	fmt.Printf("🔴 Attestation %s revoked by %s: %s\n", attestationID, revokerID, reason)
	return nil
}

// RevokeAllAttestationsByAgent revokes all attestations made by a specific agent
// Use when an agent's private key is compromised
func (s *MCPAttestationService) RevokeAllAttestationsByAgent(
	ctx context.Context,
	agentID uuid.UUID,
	revokerID uuid.UUID,
	reason string,
) (int, error) {
	// Get all attestations by this agent
	attestations, err := s.attestationRepo.GetAttestationsByAgent(agentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get attestations by agent: %w", err)
	}

	revokedCount := 0
	affectedMCPs := make(map[uuid.UUID]bool)

	for _, attestation := range attestations {
		if attestation.IsValid {
			if err := s.attestationRepo.InvalidateAttestation(attestation.ID); err != nil {
				fmt.Printf("⚠️  Warning: failed to revoke attestation %s: %v\n", attestation.ID, err)
				continue
			}
			revokedCount++
			affectedMCPs[attestation.MCPServerID] = true
		}
	}

	// Recalculate confidence scores for affected MCPs
	for mcpID := range affectedMCPs {
		_, _, err = s.updateMCPConfidenceScore(ctx, mcpID)
		if err != nil {
			fmt.Printf("⚠️  Warning: failed to recalculate confidence score for MCP %s: %v\n", mcpID, err)
		}
	}

	fmt.Printf("🔴 Revoked %d attestations by agent %s (reason: %s)\n", revokedCount, agentID, reason)
	return revokedCount, nil
}

// RecalculateAllConfidenceScores recalculates confidence scores for all MCPs (background job)
func (s *MCPAttestationService) RecalculateAllConfidenceScores(ctx context.Context) error {
	// Get all MCP servers
	mcpServers, err := s.mcpRepo.List(1000, 0) // Get up to 1000 MCPs
	if err != nil {
		return err
	}

	// Recalculate each one
	for _, mcp := range mcpServers {
		_, _, err := s.updateMCPConfidenceScore(ctx, mcp.ID)
		if err != nil {
			// Log error but continue with next MCP
			fmt.Printf("Failed to update confidence score for MCP %s: %v\n", mcp.ID, err)
		}
	}

	return nil
}

// ToCanonicalJSON is a helper to ensure consistent JSON serialization
func toCanonicalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// RecordManualAttestation records a manual attestation from a user (no cryptographic signature required)
// This allows users without SDK integration to manually attest MCP servers they've verified
// SECURITY: Manual attestations have lower trust weight than SDK attestations (see updateMCPConfidenceScore)
// SECURITY: Organization isolation - users can only attest MCP servers within their organization
func (s *MCPAttestationService) RecordManualAttestation(
	ctx context.Context,
	mcpServerID uuid.UUID,
	userID uuid.UUID,
	organizationID uuid.UUID,
	capabilitiesVerified []string,
	connectionTested bool,
	healthCheckPassed bool,
	notes string,
) (*AttestMCPResponse, error) {
	// 1. Verify MCP server exists
	mcpServer, err := s.mcpRepo.GetByID(mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("mcp server not found: %w", err)
	}

	// 2. ORGANIZATION ISOLATION: Verify user can only attest MCP servers in their organization
	if mcpServer.OrganizationID != organizationID {
		fmt.Printf("🔒 Organization isolation violation: user %s (org: %s) attempted to attest MCP %s (org: %s)\n",
			userID, organizationID, mcpServerID, mcpServer.OrganizationID)
		return nil, fmt.Errorf("cannot attest MCP servers from other organizations")
	}

	// 3. Create attestation record (manual type, no cryptographic signature)
	now := time.Now().UTC()
	attestation := &domain.MCPAttestation{
		ID:            uuid.New(),
		MCPServerID:   mcpServerID,
		AgentID:       nil, // No agent involved in manual attestations
		AttestationData: domain.AttestationPayload{
			AgentID:              userID.String(), // Use user ID as the "agent" for tracking
			MCPURL:               mcpServer.URL,
			MCPName:              mcpServer.Name,
			CapabilitiesFound:    capabilitiesVerified,
			ConnectionSuccessful: connectionTested,
			HealthCheckPassed:    healthCheckPassed,
			ConnectionLatencyMs:  0, // Not measured for manual attestations
			Timestamp:            now.Format(time.RFC3339),
			SDKVersion:           "manual-v1.0.0",
		},
		Signature:         "manual-attestation", // No cryptographic signature for manual attestations
		SignatureVerified: true,                 // Manual attestations are pre-verified by user login
		VerifiedAt:        &now,
		ExpiresAt:         now.Add(90 * 24 * time.Hour), // Manual attestations expire after 90 days
		IsValid:           true,
		CreatedAt:         now,
	}

	// 3. Store attestation
	if err := s.attestationRepo.CreateAttestation(attestation); err != nil {
		return nil, fmt.Errorf("failed to store manual attestation: %w", err)
	}

	fmt.Printf("✅ Manual attestation created: %s for MCP %s by user %s\n", attestation.ID, mcpServerID, userID)

	// 4. Update MCP server confidence score and attestation count
	// Use the updateMCPConfidenceScore helper which recalculates from all attestations
	confidenceScore, attestationCount, err := s.updateMCPConfidenceScore(ctx, mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("failed to update confidence score: %w", err)
	}

	fmt.Printf("✅ Updated MCP server %s: confidence_score=%.2f%%, attestation_count=%d\n",
		mcpServerID, confidenceScore, attestationCount)

	return &AttestMCPResponse{
		Success:            true,
		AttestationID:      attestation.ID.String(),
		MCPConfidenceScore: confidenceScore,
		AttestationCount:   attestationCount,
		Message:            fmt.Sprintf("Manual attestation recorded successfully. MCP confidence score: %.2f%%", confidenceScore),
	}, nil
}

// RecordAgentMCPConnection creates or updates an agent-MCP connection when agent uses MCP tools
func (s *MCPAttestationService) RecordAgentMCPConnection(
	ctx context.Context,
	agentID uuid.UUID,
	mcpServerID uuid.UUID,
	toolName string,
) (*domain.AgentMCPConnection, error) {
	// 1. Check if connection already exists
	existingConnection, err := s.connectionRepo.GetByAgentAndMCPServer(ctx, agentID, mcpServerID)
	if err == nil && existingConnection != nil {
		// Update existing connection
		if err := s.connectionRepo.UpdateAttestation(ctx, agentID, mcpServerID); err != nil {
			return nil, fmt.Errorf("failed to update connection: %w", err)
		}

		// Fetch updated connection
		updatedConnection, err := s.connectionRepo.GetByAgentAndMCPServer(ctx, agentID, mcpServerID)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch updated connection: %w", err)
		}

		return updatedConnection, nil
	}

	// 2. Create new connection
	now := time.Now().UTC()
	connection := &domain.AgentMCPConnection{
		ID:               uuid.New(),
		AgentID:          agentID,
		MCPServerID:      mcpServerID,
		DetectionID:      nil,
		ConnectionType:   domain.ConnectionTypeAttested,
		FirstConnectedAt: now,
		LastAttestedAt:   &now,
		AttestationCount: 1,
		IsActive:         true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.connectionRepo.Create(ctx, connection); err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}

	fmt.Printf("✅ Created agent-MCP connection: agent=%s, mcp=%s, tool=%s\n",
		agentID, mcpServerID, toolName)

	return connection, nil
}
