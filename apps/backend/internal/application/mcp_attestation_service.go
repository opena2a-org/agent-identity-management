package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	infracrypto "github.com/opena2a/identity/backend/internal/infrastructure/crypto"
	"github.com/opena2a/identity/backend/internal/infrastructure/repository"
)

// MCPAttestationService handles Agent Attestation operations
type MCPAttestationService struct {
	attestationRepo *repository.MCPAttestationRepository
	agentRepo       *repository.AgentRepository
	mcpRepo         *repository.MCPServerRepository
	cryptoService   *infracrypto.ED25519Service
}

func NewMCPAttestationService(
	attestationRepo *repository.MCPAttestationRepository,
	agentRepo *repository.AgentRepository,
	mcpRepo *repository.MCPServerRepository,
) *MCPAttestationService {
	return &MCPAttestationService{
		attestationRepo: attestationRepo,
		agentRepo:       agentRepo,
		mcpRepo:         mcpRepo,
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
	AttestationID      string  `json:"attestation_id"`
	MCPConfidenceScore float64 `json:"mcp_confidence_score"`
	AttestationCount   int     `json:"attestation_count"`
	Message            string  `json:"message"`
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

	if agent.Status != domain.AgentStatusVerified {
		return nil, fmt.Errorf("only verified agents can attest MCPs (agent status: %s)", agent.Status)
	}

	if agent.PublicKey == nil || *agent.PublicKey == "" {
		return nil, fmt.Errorf("agent has no public key registered")
	}

	// 3. Verify signature using agent's public key
	attestationJSON, err := req.Attestation.ToCanonicalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize attestation: %w", err)
	}

	valid, err := s.cryptoService.Verify(*agent.PublicKey, attestationJSON, req.Signature)
	if err != nil {
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}

	if !valid {
		return nil, fmt.Errorf("invalid attestation signature")
	}

	// 4. Check attestation is recent (< 5 minutes old)
	attestationTime, err := time.Parse(time.RFC3339, req.Attestation.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp format: %w", err)
	}

	if time.Since(attestationTime) > 5*time.Minute {
		return nil, fmt.Errorf("attestation expired (older than 5 minutes)")
	}

	// 5. Verify MCP server exists
	if _, err := s.mcpRepo.GetByID(mcpServerID); err != nil {
		return nil, fmt.Errorf("mcp server not found: %w", err)
	}

	// 6. Store attestation
	now := time.Now().UTC()
	attestation := &domain.MCPAttestation{
		ID:                uuid.New(),
		MCPServerID:       mcpServerID,
		AgentID:           agentID,
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
	// 1. Number of unique agents attesting (20 points each, max 5 agents = 100)
	// 2. Average trust score of attesting agents (0-50 points)
	// 3. Recency of attestations (0-30 points)

	uniqueAgents := make(map[uuid.UUID]bool)
	var totalTrust float64
	var mostRecentAttestation time.Time

	for _, att := range attestations {
		uniqueAgents[att.AgentID] = true
		totalTrust += att.AgentTrustScore

		if att.VerifiedAt != nil && att.VerifiedAt.After(mostRecentAttestation) {
			mostRecentAttestation = *att.VerifiedAt
		}
	}

	// Factor 1: Number of unique agents (20 points each, max 100)
	agentCount := len(uniqueAgents)
	agentPoints := float64(agentCount) * 20.0
	if agentPoints > 100.0 {
		agentPoints = 100.0
	}

	// Factor 2: Average trust score of attesting agents (0-50 points)
	avgTrust := totalTrust / float64(len(attestations))
	trustPoints := (avgTrust / 100.0) * 50.0 // Scale to 0-50

	// Factor 3: Recency factor (% of attestations in last 7 days)
	recentCount := 0
	for _, att := range attestations {
		if att.VerifiedAt != nil && time.Since(*att.VerifiedAt) < 7*24*time.Hour {
			recentCount++
		}
	}
	recencyFactor := float64(recentCount) / float64(len(attestations))
	recencyPoints := recencyFactor * 30.0

	// Calculate final confidence score (0-100)
	confidenceScore := (agentPoints + trustPoints + recencyPoints) / 1.8
	if confidenceScore > 100.0 {
		confidenceScore = 100.0
	}

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

	return confidenceScore, len(attestations), nil
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

	// Convert to response format
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

		result = append(result, &domain.AttestationWithAgentDetails{
			ID:                    att.ID,
			AgentID:               att.AgentID,
			AgentName:             att.AgentName,
			AgentTrustScore:       att.AgentTrustScore,
			VerifiedAt:            verifiedAtStr,
			ExpiresAt:             expiresAtStr,
			CapabilitiesConfirmed: att.AttestationData.CapabilitiesFound,
			ConnectionLatencyMs:   att.AttestationData.ConnectionLatencyMs,
			HealthCheckPassed:     att.AttestationData.HealthCheckPassed,
			IsValid:               att.IsValid,
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
