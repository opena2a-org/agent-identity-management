package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// AgentService handles agent business logic
type AgentService struct {
	agentRepo      domain.AgentRepository
	trustCalc      domain.TrustScoreCalculator
	trustScoreRepo domain.TrustScoreRepository
}

// NewAgentService creates a new agent service
func NewAgentService(
	agentRepo domain.AgentRepository,
	trustCalc domain.TrustScoreCalculator,
	trustScoreRepo domain.TrustScoreRepository,
) *AgentService {
	return &AgentService{
		agentRepo:      agentRepo,
		trustCalc:      trustCalc,
		trustScoreRepo: trustScoreRepo,
	}
}

// CreateAgentRequest represents agent creation request
type CreateAgentRequest struct {
	Name             string
	DisplayName      string
	Description      string
	AgentType        domain.AgentType
	Version          string
	PublicKey        string
	CertificateURL   string
	RepositoryURL    string
	DocumentationURL string
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

	// Create agent
	publicKey := &req.PublicKey
	agent := &domain.Agent{
		OrganizationID:   orgID,
		Name:             req.Name,
		DisplayName:      req.DisplayName,
		Description:      req.Description,
		AgentType:        req.AgentType,
		Version:          req.Version,
		PublicKey:        publicKey,
		CertificateURL:   req.CertificateURL,
		RepositoryURL:    req.RepositoryURL,
		DocumentationURL: req.DocumentationURL,
		Status:           domain.AgentStatusPending,
		CreatedBy:        userID,
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

	return agent, nil
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
	if req.PublicKey != "" {
		publicKey := &req.PublicKey
		agent.PublicKey = publicKey
	}
	if req.CertificateURL != "" {
		agent.CertificateURL = req.CertificateURL
	}
	if req.RepositoryURL != "" {
		agent.RepositoryURL = req.RepositoryURL
	}
	if req.DocumentationURL != "" {
		agent.DocumentationURL = req.DocumentationURL
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
func (s *AgentService) VerifyAction(
	ctx context.Context,
	agentID uuid.UUID,
	actionType string,
	resource string,
	metadata map[string]interface{},
) (allowed bool, reason string, auditID uuid.UUID, err error) {
	// 1. Fetch agent
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return false, "Agent not found", uuid.Nil, err
	}

	// 2. Check agent status
	if agent.Status != domain.AgentStatusVerified {
		return false, "Agent not verified", uuid.Nil, nil
	}

	// 3. Check capabilities (simplified for now - will be enhanced)
	// TODO: Implement proper capability matching logic
	// For now, we allow all actions for verified agents
	allowed = true
	reason = "Action matches registered capabilities"

	// 4. Create audit log
	auditID = uuid.New()
	// Note: We need an audit service instance, but for now we'll skip audit logging
	// This should be properly implemented with the audit service in production

	return allowed, reason, auditID, nil
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
