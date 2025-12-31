package application

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// A2A Configuration Constants
const (
	// DefaultMinTrustScoreForA2A is the minimum trust score for A2A communication
	DefaultMinTrustScoreForA2A = 0.7

	// DefaultAttestationValidityHours is how long an attestation is valid
	DefaultAttestationValidityHours = 24

	// DefaultNonceExpiryMinutes is how long a nonce is valid
	DefaultNonceExpiryMinutes = 5

	// DefaultSignatureTimestampToleranceSeconds is the allowed clock skew
	DefaultSignatureTimestampToleranceSeconds = 300 // 5 minutes
)

// Configuration helpers
func getMinTrustScoreForA2A() float64 {
	if val := os.Getenv("A2A_MIN_TRUST_SCORE"); val != "" {
		if n, err := strconv.ParseFloat(val, 64); err == nil && n >= 0 && n <= 1 {
			return n
		}
	}
	return DefaultMinTrustScoreForA2A
}

func getAttestationValidityHours() int {
	if val := os.Getenv("A2A_ATTESTATION_VALIDITY_HOURS"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			return n
		}
	}
	return DefaultAttestationValidityHours
}

func getNonceExpiryMinutes() int {
	if val := os.Getenv("A2A_NONCE_EXPIRY_MINUTES"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			return n
		}
	}
	return DefaultNonceExpiryMinutes
}

// A2AService handles A2A (Agent-to-Agent) protocol operations
type A2AService struct {
	cardRepo       *repository.A2AAgentCardRepository
	skillRepo      *repository.A2ASkillRepository
	taskRepo       *repository.A2ATaskRepository
	peerTrustRepo  *repository.A2APeerTrustRepository
	consentRepo    *repository.A2AConsentRepository
	trustScoreRepo *repository.A2ATrustScoreRepository
	nonceRepo      *repository.A2ARequestNonceRepository
	policyRepo     *repository.A2APolicyRepository
	agentRepo      *repository.AgentRepository
	keyVault       *crypto.KeyVault
	httpClient     *http.Client
}

// NewA2AService creates a new A2A service
func NewA2AService(
	cardRepo *repository.A2AAgentCardRepository,
	skillRepo *repository.A2ASkillRepository,
	taskRepo *repository.A2ATaskRepository,
	peerTrustRepo *repository.A2APeerTrustRepository,
	consentRepo *repository.A2AConsentRepository,
	trustScoreRepo *repository.A2ATrustScoreRepository,
	nonceRepo *repository.A2ARequestNonceRepository,
	policyRepo *repository.A2APolicyRepository,
	agentRepo *repository.AgentRepository,
	keyVault *crypto.KeyVault,
) *A2AService {
	return &A2AService{
		cardRepo:       cardRepo,
		skillRepo:      skillRepo,
		taskRepo:       taskRepo,
		peerTrustRepo:  peerTrustRepo,
		consentRepo:    consentRepo,
		trustScoreRepo: trustScoreRepo,
		nonceRepo:      nonceRepo,
		policyRepo:     policyRepo,
		agentRepo:      agentRepo,
		keyVault:       keyVault,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ============================================================================
// Agent Card Operations
// ============================================================================

// RegisterAgentCardRequest is the request to register an A2A agent card
type RegisterAgentCardRequest struct {
	AgentID  uuid.UUID `json:"agentId"`
	CardURL  string    `json:"cardUrl"`
}

// RegisterAgentCardResponse is the response after registering an agent card
type RegisterAgentCardResponse struct {
	CardID             uuid.UUID  `json:"cardId"`
	AgentID            uuid.UUID  `json:"agentId"`
	CardURL            string     `json:"cardUrl"`
	AttestationExpires time.Time  `json:"attestationExpires"`
	Skills             []string   `json:"skills"`
}

// RegisterAgentCard registers an A2A agent card and creates AIM attestation
func (s *A2AService) RegisterAgentCard(ctx context.Context, req RegisterAgentCardRequest) (*RegisterAgentCardResponse, error) {
	// 1. Fetch the agent
	agent, err := s.agentRepo.GetByID(req.AgentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found")
	}

	// 2. Fetch the agent card from URL
	cardData, err := s.fetchAgentCard(req.CardURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch agent card: %w", err)
	}

	// 3. Parse the card
	var parsedCard domain.A2AAgentCardParsed
	if err := json.Unmarshal(cardData, &parsedCard); err != nil {
		return nil, fmt.Errorf("failed to parse agent card: %w", err)
	}

	// 4. Calculate card hash
	cardHash := sha256.Sum256(cardData)
	cardHashStr := hex.EncodeToString(cardHash[:])

	// 5. Create attestation signature
	attestationExpires := time.Now().UTC().Add(time.Duration(getAttestationValidityHours()) * time.Hour)
	attestationSignature, err := s.createCardAttestation(agent, cardData, attestationExpires)
	if err != nil {
		return nil, fmt.Errorf("failed to create attestation: %w", err)
	}

	// 6. Save the agent card
	now := time.Now().UTC()
	card := &domain.A2AAgentCard{
		ID:                   uuid.New(),
		AgentID:              req.AgentID,
		CardURL:              req.CardURL,
		CardData:             cardData,
		CardHash:             cardHashStr,
		ProtocolVersion:      parsedCard.Version,
		AttestationSignature: attestationSignature,
		AttestationIssuedAt:  &now,
		AttestationExpiresAt: &attestationExpires,
		IsValid:              true,
		LastFetchedAt:        &now,
		FetchCount:           1,
	}

	if err := s.cardRepo.Create(ctx, card); err != nil {
		return nil, fmt.Errorf("failed to save agent card: %w", err)
	}

	// 7. Parse and save skills
	var skillIDs []string
	for _, skillDef := range parsedCard.Skills {
		skill := &domain.A2ASkill{
			AgentID:     req.AgentID,
			SkillID:     skillDef.ID,
			Name:        skillDef.Name,
			Description: skillDef.Description,
			Tags:        skillDef.Tags,
			InputModes:  skillDef.InputModes,
			OutputModes: skillDef.OutputModes,
			Examples:    skillDef.Examples,
		}
		if err := s.skillRepo.Create(ctx, skill); err != nil {
			// Log but don't fail - skills can be updated later
			continue
		}
		skillIDs = append(skillIDs, skillDef.ID)
	}

	return &RegisterAgentCardResponse{
		CardID:             card.ID,
		AgentID:            card.AgentID,
		CardURL:            card.CardURL,
		AttestationExpires: attestationExpires,
		Skills:             skillIDs,
	}, nil
}

// GetAgentCard retrieves the AIM-attested agent card
func (s *A2AService) GetAgentCard(ctx context.Context, agentID uuid.UUID) (*domain.A2AAgentCard, error) {
	return s.cardRepo.GetByAgentID(ctx, agentID)
}

// GetEnhancedAgentCard returns the agent card with AIM extensions
func (s *A2AService) GetEnhancedAgentCard(ctx context.Context, agentID uuid.UUID) (*domain.A2AAgentCardParsed, error) {
	// Get the card
	card, err := s.cardRepo.GetByAgentID(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, fmt.Errorf("agent card not found")
	}

	// Parse the base card
	var parsed domain.A2AAgentCardParsed
	if err := json.Unmarshal(card.CardData, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse card data: %w", err)
	}

	// Get the agent for additional info
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil || agent == nil {
		return &parsed, nil // Return without AIM extension
	}

	// Get A2A trust score
	trustScore, _ := s.trustScoreRepo.GetByAgentID(ctx, agentID)

	// Add AIM extension
	parsed.AIM = &domain.A2AAIMExtension{
		AgentID:      agentID,
		PublicKey:    stringValue(agent.PublicKey),
		TrustScore:   agent.TrustScore,
		Capabilities: agent.Capabilities,
	}

	if card.AttestationSignature != "" {
		parsed.AIM.Attestation = &domain.A2AAttestation{
			Signature: card.AttestationSignature,
			IssuedAt:  timeValue(card.AttestationIssuedAt),
			ExpiresAt: timeValue(card.AttestationExpiresAt),
		}
	}

	if trustScore != nil {
		parsed.AIM.Behavior = &domain.A2ABehavior{
			TotalTasksCompleted: int64(trustScore.TasksCompleted),
			SuccessRate:         calculateSuccessRate(trustScore),
			AvgResponseTimeMs:   intValue(trustScore.AvgResponseTimeMs),
			LastActive:          timeValue(trustScore.ComputedAt),
		}
	}

	return &parsed, nil
}

// RefreshAttestation refreshes the attestation for an agent card
func (s *A2AService) RefreshAttestation(ctx context.Context, agentID uuid.UUID) (*domain.A2AAgentCard, error) {
	// Get existing card
	card, err := s.cardRepo.GetByAgentID(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, fmt.Errorf("agent card not found")
	}

	// Get agent
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil || agent == nil {
		return nil, fmt.Errorf("agent not found")
	}

	// Create new attestation
	attestationExpires := time.Now().UTC().Add(time.Duration(getAttestationValidityHours()) * time.Hour)
	attestationSignature, err := s.createCardAttestation(agent, card.CardData, attestationExpires)
	if err != nil {
		return nil, fmt.Errorf("failed to create attestation: %w", err)
	}

	// Update card
	now := time.Now().UTC()
	card.AttestationSignature = attestationSignature
	card.AttestationIssuedAt = &now
	card.AttestationExpiresAt = &attestationExpires

	if err := s.cardRepo.Update(ctx, card); err != nil {
		return nil, fmt.Errorf("failed to update card: %w", err)
	}

	return card, nil
}

// ============================================================================
// Request Signing & Verification
// ============================================================================

// SignA2ARequest signs an A2A request for the given agent
func (s *A2AService) SignA2ARequest(
	ctx context.Context,
	agentID uuid.UUID,
	method, path string,
	body []byte,
) (*domain.A2ARequestSignature, error) {
	// Get agent with private key
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil || agent == nil {
		return nil, fmt.Errorf("agent not found")
	}

	if agent.EncryptedPrivateKey == nil || *agent.EncryptedPrivateKey == "" {
		return nil, fmt.Errorf("agent has no private key")
	}

	// Decrypt private key
	privateKeyB64, err := s.keyVault.DecryptPrivateKey(*agent.EncryptedPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt private key: %w", err)
	}
	privateKey, err := base64.StdEncoding.DecodeString(privateKeyB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	// Generate nonce
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)

	// Create timestamp
	timestamp := time.Now().UTC().Unix()

	// Create canonical request
	bodyHash := sha256.Sum256(body)
	canonical := fmt.Sprintf("%s\n%s\n%d\n%s\n%s",
		method,
		path,
		timestamp,
		nonce,
		hex.EncodeToString(bodyHash[:]),
	)
	canonicalHash := sha256.Sum256([]byte(canonical))

	// Sign
	signature := ed25519.Sign(privateKey, canonicalHash[:])
	signatureB64 := base64.StdEncoding.EncodeToString(signature)

	// Record nonce for replay protection
	nonceRecord := &domain.A2ARequestNonce{
		Nonce:       nonce,
		AgentID:     agentID,
		UsedAt:      time.Now().UTC(),
		ExpiresAt:   time.Now().UTC().Add(time.Duration(getNonceExpiryMinutes()) * time.Minute),
		RequestHash: hex.EncodeToString(canonicalHash[:]),
	}
	if err := s.nonceRepo.Create(ctx, nonceRecord); err != nil {
		// Log but don't fail - nonce recording is for replay protection
	}

	return &domain.A2ARequestSignature{
		AgentID:   agentID,
		Timestamp: timestamp,
		Nonce:     nonce,
		Signature: signatureB64,
	}, nil
}

// VerifyA2ARequestRequest is the request to verify an A2A request signature
type VerifyA2ARequestRequest struct {
	AgentID     uuid.UUID `json:"agentId"`
	Timestamp   int64     `json:"timestamp"`
	Nonce       string    `json:"nonce"`
	Signature   string    `json:"signature"`
	RequestHash string    `json:"requestHash"`
}

// VerifyA2ARequest verifies an incoming A2A request signature
func (s *A2AService) VerifyA2ARequest(
	ctx context.Context,
	req VerifyA2ARequestRequest,
) (*domain.A2ASignatureVerificationResult, error) {
	result := &domain.A2ASignatureVerificationResult{
		Valid: false,
	}

	// 1. Check timestamp is within tolerance
	now := time.Now().UTC().Unix()
	tolerance := int64(DefaultSignatureTimestampToleranceSeconds)
	if req.Timestamp < now-tolerance || req.Timestamp > now+tolerance {
		result.Error = "timestamp outside tolerance window"
		return result, nil
	}

	// 2. Check nonce hasn't been used (replay protection)
	nonceExists, err := s.nonceRepo.Exists(ctx, req.Nonce)
	if err != nil {
		result.Error = "failed to check nonce"
		return result, nil
	}
	if nonceExists {
		result.Error = "nonce already used (replay attack)"
		return result, nil
	}

	// 3. Get the agent
	agent, err := s.agentRepo.GetByID(req.AgentID)
	if err != nil || agent == nil {
		result.Error = "agent not found"
		return result, nil
	}

	if agent.PublicKey == nil || *agent.PublicKey == "" {
		result.Error = "agent has no public key"
		return result, nil
	}

	// 4. Parse and verify signature
	publicKey, err := base64.StdEncoding.DecodeString(*agent.PublicKey)
	if err != nil {
		result.Error = "invalid public key format"
		return result, nil
	}

	signature, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		result.Error = "invalid signature format"
		return result, nil
	}

	requestHashBytes, err := hex.DecodeString(req.RequestHash)
	if err != nil {
		result.Error = "invalid request hash format"
		return result, nil
	}

	if !ed25519.Verify(publicKey, requestHashBytes, signature) {
		result.Error = "signature verification failed"
		return result, nil
	}

	// 5. Record nonce to prevent replay
	nonceRecord := &domain.A2ARequestNonce{
		Nonce:       req.Nonce,
		AgentID:     req.AgentID,
		UsedAt:      time.Now().UTC(),
		ExpiresAt:   time.Now().UTC().Add(time.Duration(getNonceExpiryMinutes()) * time.Minute),
		RequestHash: req.RequestHash,
	}
	_ = s.nonceRepo.Create(ctx, nonceRecord)

	// 6. Check agent card attestation
	card, _ := s.cardRepo.GetByAgentID(ctx, req.AgentID)
	attestationValid := card != nil &&
		card.IsValid &&
		card.AttestationExpiresAt != nil &&
		card.AttestationExpiresAt.After(time.Now().UTC())

	// 7. Return success
	result.Valid = true
	result.AgentID = agent.ID
	result.AgentName = agent.Name
	result.TrustScore = agent.TrustScore
	result.Capabilities = agent.Capabilities
	result.AttestationValid = attestationValid

	return result, nil
}

// ============================================================================
// Trust Score Operations
// ============================================================================

// GetA2ATrustScore returns the A2A-specific trust score for an agent
func (s *A2AService) GetA2ATrustScore(ctx context.Context, agentID uuid.UUID) (*domain.A2ATrustScore, error) {
	return s.trustScoreRepo.GetByAgentID(ctx, agentID)
}

// GetPeerTrustScore returns the trust score between two agents
func (s *A2AService) GetPeerTrustScore(ctx context.Context, agentID, peerID uuid.UUID) (*domain.A2APeerTrust, error) {
	return s.peerTrustRepo.GetByPeer(ctx, agentID, peerID)
}

// ComputeA2ATrustScore computes and stores the A2A trust score for an agent
func (s *A2AService) ComputeA2ATrustScore(ctx context.Context, agentID uuid.UUID) (*domain.A2ATrustScore, error) {
	// Get existing score or create new
	score, err := s.trustScoreRepo.GetByAgentID(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if score == nil {
		score = &domain.A2ATrustScore{AgentID: agentID}
	}

	// Calculate peer trust average
	peerAvg, err := s.peerTrustRepo.ComputeAveragePeerTrust(ctx, agentID)
	if err != nil {
		peerAvg = 0
	}
	score.PeerTrustAverage = &peerAvg

	// Count unique peers
	peers, err := s.peerTrustRepo.GetByAgentID(ctx, agentID)
	if err == nil {
		score.UniquePeersCount = len(peers)
	}

	// Compute overall A2A trust score
	// Formula: 0.4 * success_rate + 0.3 * peer_trust + 0.2 * response_quality + 0.1 * account_age
	var a2aScore float64 = 0.5 // Default

	// Success rate component (40%)
	total := score.TasksCompleted + score.TasksFailed
	if total > 0 {
		successRate := float64(score.TasksCompleted) / float64(total)
		a2aScore = 0.4 * successRate
	}

	// Peer trust component (30%)
	a2aScore += 0.3 * peerAvg

	// Response quality component (20%) - based on response time
	if score.AvgResponseTimeMs != nil {
		// Lower response time = better score. Cap at 5000ms
		responseScore := 1.0 - float64(*score.AvgResponseTimeMs)/5000.0
		if responseScore < 0 {
			responseScore = 0
		}
		a2aScore += 0.2 * responseScore
	} else {
		a2aScore += 0.1 // Neutral if no data
	}

	// Account age component (10%) - would need agent creation time
	a2aScore += 0.1 // Assume established

	score.A2ATrustScore = &a2aScore
	now := time.Now().UTC()
	score.ComputedAt = &now

	// Save
	if err := s.trustScoreRepo.Upsert(ctx, score); err != nil {
		return nil, fmt.Errorf("failed to save trust score: %w", err)
	}

	return score, nil
}

// ============================================================================
// Task Logging
// ============================================================================

// LogA2ATaskRequest is the request to log an A2A task
type LogA2ATaskRequest struct {
	ExternalTaskID string       `json:"externalTaskId"`
	ContextID      string       `json:"contextId"`
	ClientAgentID  uuid.UUID    `json:"clientAgentId"`
	RemoteAgentID  uuid.UUID    `json:"remoteAgentId"`
	SkillID        string       `json:"skillId"`
}

// LogA2ATask logs an A2A task for audit trail
func (s *A2AService) LogA2ATask(ctx context.Context, req LogA2ATaskRequest) (*domain.A2ATask, error) {
	// Get trust scores at time of task
	var clientTrust, remoteTrust *float64
	if clientScore, _ := s.trustScoreRepo.GetByAgentID(ctx, req.ClientAgentID); clientScore != nil {
		clientTrust = clientScore.A2ATrustScore
	}
	if remoteScore, _ := s.trustScoreRepo.GetByAgentID(ctx, req.RemoteAgentID); remoteScore != nil {
		remoteTrust = remoteScore.A2ATrustScore
	}

	task := &domain.A2ATask{
		ExternalTaskID:           req.ExternalTaskID,
		ContextID:                req.ContextID,
		ClientAgentID:            req.ClientAgentID,
		RemoteAgentID:            req.RemoteAgentID,
		SkillID:                  req.SkillID,
		State:                    domain.A2ATaskStateSubmitted,
		ClientTrustScoreSnapshot: clientTrust,
		RemoteTrustScoreSnapshot: remoteTrust,
	}

	if err := s.taskRepo.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to log task: %w", err)
	}

	return task, nil
}

// UpdateA2ATaskState updates the state of an A2A task
func (s *A2AService) UpdateA2ATaskState(
	ctx context.Context,
	taskID uuid.UUID,
	state domain.A2ATaskState,
	errorCode, errorMessage string,
) error {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("task not found")
	}

	now := time.Now().UTC()
	task.State = state
	task.ErrorCode = errorCode
	task.ErrorMessage = errorMessage

	switch state {
	case domain.A2ATaskStateWorking:
		task.StartedAt = &now
	case domain.A2ATaskStateCompleted, domain.A2ATaskStateFailed, domain.A2ATaskStateCancelled:
		task.CompletedAt = &now
		if task.StartedAt != nil {
			task.DurationMs = int(now.Sub(*task.StartedAt).Milliseconds())
		}
	}

	if err := s.taskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	// Update trust scores
	isCompleted := state == domain.A2ATaskStateCompleted
	_ = s.trustScoreRepo.IncrementTaskStats(ctx, task.ClientAgentID, true, isCompleted)
	_ = s.trustScoreRepo.IncrementTaskStats(ctx, task.RemoteAgentID, false, isCompleted)

	// Update peer trust
	s.updatePeerTrust(ctx, task, isCompleted)

	return nil
}

// ============================================================================
// Consent Management
// ============================================================================

// RecordConsentRequest is the request to record user consent
type RecordConsentRequest struct {
	UserID           string    `json:"userId"`
	OrganizationID   *uuid.UUID `json:"organizationId"`
	GrantorAgentID   uuid.UUID `json:"grantorAgentId"`
	RecipientAgentID uuid.UUID `json:"recipientAgentId"`
	Scope            []string  `json:"scope"`
	Purpose          string    `json:"purpose"`
	DataTypes        []string  `json:"dataTypes"`
	ExpiresInHours   int       `json:"expiresInHours"`
	ConsentMethod    string    `json:"consentMethod"`
	Evidence         string    `json:"evidence"`
	IPAddress        string    `json:"ipAddress"`
	UserAgent        string    `json:"userAgent"`
}

// RecordConsent records user consent for cross-agent data sharing
func (s *A2AService) RecordConsent(ctx context.Context, req RecordConsentRequest) (*domain.A2AConsentRecord, error) {
	var expiresAt *time.Time
	if req.ExpiresInHours > 0 {
		t := time.Now().UTC().Add(time.Duration(req.ExpiresInHours) * time.Hour)
		expiresAt = &t
	}

	consent := &domain.A2AConsentRecord{
		UserID:           req.UserID,
		OrganizationID:   req.OrganizationID,
		GrantorAgentID:   req.GrantorAgentID,
		RecipientAgentID: req.RecipientAgentID,
		Scope:            req.Scope,
		Purpose:          req.Purpose,
		DataTypes:        req.DataTypes,
		ExpiresAt:        expiresAt,
		ConsentMethod:    domain.A2AConsentMethod(req.ConsentMethod),
		Evidence:         req.Evidence,
		UserAgent:        req.UserAgent,
	}

	if req.IPAddress != "" {
		consent.IPAddress = parseIPAddress(req.IPAddress)
	}

	if err := s.consentRepo.Create(ctx, consent); err != nil {
		return nil, fmt.Errorf("failed to record consent: %w", err)
	}

	return consent, nil
}

// CheckConsent checks if consent exists for a specific scope
func (s *A2AService) CheckConsent(
	ctx context.Context,
	userID string,
	grantorAgentID, recipientAgentID uuid.UUID,
	scope string,
) (bool, error) {
	return s.consentRepo.CheckConsent(ctx, userID, grantorAgentID, recipientAgentID, scope)
}

// RevokeConsent revokes a consent record
func (s *A2AService) RevokeConsent(ctx context.Context, consentID uuid.UUID, reason string) error {
	return s.consentRepo.Revoke(ctx, consentID, reason)
}

// ListUserConsents lists all consent records for a user
func (s *A2AService) ListUserConsents(ctx context.Context, userID string, includeRevoked bool) ([]*domain.A2AConsentRecord, error) {
	return s.consentRepo.ListByUser(ctx, userID, includeRevoked)
}

// ============================================================================
// Policy Evaluation
// ============================================================================

// EvaluateA2APolicyRequest is the request to evaluate A2A policies
type EvaluateA2APolicyRequest struct {
	Event          string    `json:"event"` // task.create, task.received, etc.
	ClientAgentID  uuid.UUID `json:"clientAgentId"`
	RemoteAgentID  uuid.UUID `json:"remoteAgentId"`
	SkillID        string    `json:"skillId"`
	OrganizationID uuid.UUID `json:"organizationId"`
}

// EvaluateA2APolicy evaluates A2A policies for a given action
func (s *A2AService) EvaluateA2APolicy(ctx context.Context, req EvaluateA2APolicyRequest) (*domain.A2APolicyDecision, error) {
	// Get active policies for the organization
	policies, err := s.policyRepo.GetActivePolicies(ctx, req.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get policies: %w", err)
	}

	// If no policies, default allow
	if len(policies) == 0 {
		return &domain.A2APolicyDecision{
			Decision:          "ALLOW",
			PoliciesEvaluated: []string{},
			EvaluatedAt:       time.Now().UTC(),
		}, nil
	}

	// Get agent trust scores for policy evaluation
	var clientTrust, remoteTrust float64
	if clientScore, _ := s.trustScoreRepo.GetByAgentID(ctx, req.ClientAgentID); clientScore != nil && clientScore.A2ATrustScore != nil {
		clientTrust = *clientScore.A2ATrustScore
	}
	if remoteScore, _ := s.trustScoreRepo.GetByAgentID(ctx, req.RemoteAgentID); remoteScore != nil && remoteScore.A2ATrustScore != nil {
		remoteTrust = *remoteScore.A2ATrustScore
	}

	// Evaluate policies (simplified - in production would use CEL or OPA)
	var evaluatedPolicies []string
	for _, policy := range policies {
		evaluatedPolicies = append(evaluatedPolicies, policy.Name)

		// Check if policy applies to this agent/skill
		if !policyApplies(policy, req.ClientAgentID.String(), req.SkillID) {
			continue
		}

		// Basic trust score check (would be more complex with CEL)
		minTrust := getMinTrustScoreForA2A()
		if clientTrust < minTrust || remoteTrust < minTrust {
			return &domain.A2APolicyDecision{
				Decision:          "DENY",
				PoliciesEvaluated: evaluatedPolicies,
				Reason:            fmt.Sprintf("Trust score below threshold (client: %.2f, remote: %.2f, required: %.2f)", clientTrust, remoteTrust, minTrust),
				EvaluatedAt:       time.Now().UTC(),
			}, nil
		}
	}

	return &domain.A2APolicyDecision{
		Decision:          "ALLOW",
		PoliciesEvaluated: evaluatedPolicies,
		EvaluatedAt:       time.Now().UTC(),
	}, nil
}

// ============================================================================
// Skill Operations
// ============================================================================

// GetAgentSkills returns all skills for an agent
func (s *A2AService) GetAgentSkills(ctx context.Context, agentID uuid.UUID) ([]*domain.A2ASkill, error) {
	return s.skillRepo.GetByAgentID(ctx, agentID)
}

// SearchSkills searches for skills across all agents
func (s *A2AService) SearchSkills(ctx context.Context, query string, limit int) ([]*domain.A2ASkill, error) {
	return s.skillRepo.Search(ctx, query, limit)
}

// IncrementSkillUsage updates skill usage statistics
func (s *A2AService) IncrementSkillUsage(ctx context.Context, skillID uuid.UUID, success bool, durationMs int) error {
	return s.skillRepo.IncrementUsage(ctx, skillID, success, durationMs)
}

// ============================================================================
// Intent-Based Discovery
// ============================================================================

// RouteByIntent finds the best agent for a given intent using FTS
func (s *A2AService) RouteByIntent(ctx context.Context, intent string, minTrustScore float64) (*domain.RouteIntentResponse, error) {
	// Search for agents matching the intent
	agents, err := s.skillRepo.SearchByIntent(ctx, intent, minTrustScore, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to search by intent: %w", err)
	}

	if len(agents) == 0 {
		return &domain.RouteIntentResponse{
			Agent:        nil,
			Alternatives: 0,
		}, nil
	}

	// Best match is first (sorted by relevance * trust)
	return &domain.RouteIntentResponse{
		Agent:        agents[0],
		Alternatives: len(agents) - 1,
	}, nil
}

// CapableOf returns all agents capable of a given intent
func (s *A2AService) CapableOf(ctx context.Context, intent string, minTrustScore float64, limit int) ([]*domain.RoutedAgent, error) {
	return s.skillRepo.SearchByIntent(ctx, intent, minTrustScore, limit)
}

// ============================================================================
// Maintenance Operations
// ============================================================================

// CleanupExpiredNonces removes expired nonces
func (s *A2AService) CleanupExpiredNonces(ctx context.Context) (int, error) {
	return s.nonceRepo.DeleteExpired(ctx)
}

// RefreshExpiredCards refreshes cards with expired attestations
func (s *A2AService) RefreshExpiredCards(ctx context.Context) (int, error) {
	cards, err := s.cardRepo.GetExpiredCards(ctx)
	if err != nil {
		return 0, err
	}

	refreshed := 0
	for _, card := range cards {
		if _, err := s.RefreshAttestation(ctx, card.AgentID); err == nil {
			refreshed++
		}
	}

	return refreshed, nil
}

// ============================================================================
// Helper Functions
// ============================================================================

func (s *A2AService) fetchAgentCard(url string) ([]byte, error) {
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return body, nil
}

func (s *A2AService) createCardAttestation(agent *domain.Agent, cardData json.RawMessage, expiresAt time.Time) (string, error) {
	// Get server's signing key (would be from config in production)
	// For now, use the agent's own key for self-attestation
	if agent.EncryptedPrivateKey == nil {
		return "", fmt.Errorf("agent has no private key for signing")
	}

	privateKeyB64, err := s.keyVault.DecryptPrivateKey(*agent.EncryptedPrivateKey)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt private key: %w", err)
	}
	privateKey, err := base64.StdEncoding.DecodeString(privateKeyB64)
	if err != nil {
		return "", fmt.Errorf("failed to decode private key: %w", err)
	}

	// Create attestation payload
	attestPayload := struct {
		CardHash  string    `json:"cardHash"`
		AgentID   string    `json:"agentId"`
		IssuedAt  time.Time `json:"issuedAt"`
		ExpiresAt time.Time `json:"expiresAt"`
	}{
		CardHash:  hex.EncodeToString(sha256Hash(cardData)),
		AgentID:   agent.ID.String(),
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: expiresAt,
	}

	payloadJSON, _ := json.Marshal(attestPayload)
	payloadHash := sha256.Sum256(payloadJSON)

	signature := ed25519.Sign(privateKey, payloadHash[:])
	return base64.StdEncoding.EncodeToString(signature), nil
}

func (s *A2AService) updatePeerTrust(ctx context.Context, task *domain.A2ATask, completed bool) {
	// Update client -> remote trust
	clientToRemote, _ := s.peerTrustRepo.GetByPeer(ctx, task.ClientAgentID, task.RemoteAgentID)
	if clientToRemote == nil {
		clientToRemote = &domain.A2APeerTrust{
			AgentID:     task.ClientAgentID,
			PeerAgentID: task.RemoteAgentID,
		}
		now := time.Now().UTC()
		clientToRemote.FirstInteractionAt = &now
	}

	clientToRemote.TasksInitiated++
	if completed {
		clientToRemote.TasksInitiatedCompleted++
	} else {
		clientToRemote.TasksInitiatedFailed++
	}
	now := time.Now().UTC()
	clientToRemote.LastInteractionAt = &now
	clientToRemote.TrustDataPoints++

	// Calculate success rate
	total := clientToRemote.TasksInitiatedCompleted + clientToRemote.TasksInitiatedFailed
	if total > 0 {
		rate := float64(clientToRemote.TasksInitiatedCompleted) / float64(total)
		clientToRemote.SuccessRate = &rate
		clientToRemote.PeerTrustScore = &rate // Simple: trust = success rate
		clientToRemote.TrustComputedAt = &now
	}

	_ = s.peerTrustRepo.Upsert(ctx, clientToRemote)

	// Update remote -> client trust (as receiver)
	remoteToClient, _ := s.peerTrustRepo.GetByPeer(ctx, task.RemoteAgentID, task.ClientAgentID)
	if remoteToClient == nil {
		remoteToClient = &domain.A2APeerTrust{
			AgentID:     task.RemoteAgentID,
			PeerAgentID: task.ClientAgentID,
		}
		remoteToClient.FirstInteractionAt = &now
	}

	remoteToClient.TasksReceived++
	if completed {
		remoteToClient.TasksReceivedCompleted++
	} else {
		remoteToClient.TasksReceivedFailed++
	}
	remoteToClient.LastInteractionAt = &now
	remoteToClient.TrustDataPoints++

	_ = s.peerTrustRepo.Upsert(ctx, remoteToClient)
}

func sha256Hash(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func timeValue(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

func intValue(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

func calculateSuccessRate(score *domain.A2ATrustScore) float64 {
	total := score.TasksCompleted + score.TasksFailed
	if total == 0 {
		return 0
	}
	return float64(score.TasksCompleted) / float64(total)
}

func policyApplies(policy *domain.A2APolicy, agentID, skillID string) bool {
	// Empty list = applies to all
	if len(policy.AppliesToAgents) > 0 {
		found := false
		for _, a := range policy.AppliesToAgents {
			if a == agentID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(policy.AppliesToSkills) > 0 {
		found := false
		for _, s := range policy.AppliesToSkills {
			if s == skillID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func parseIPAddress(ip string) []byte {
	// Simple IP parsing - returns nil on failure
	return nil // Would use net.ParseIP in production
}
