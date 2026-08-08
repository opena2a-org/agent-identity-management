package application

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/utils"
)

// ErrConsentGrantorNotOwned is returned when a consent record names a grantor
// the caller's organization does not own. It deliberately covers both "no such
// agent" and "that agent belongs to someone else": telling those apart would
// answer whether an arbitrary agent id exists, for agents the caller cannot
// otherwise see.
var ErrConsentGrantorNotOwned = errors.New("grantor agent not found in this organization")

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
	cardRepo        *repository.A2AAgentCardRepository
	skillRepo       *repository.A2ASkillRepository
	taskRepo        *repository.A2ATaskRepository
	peerTrustRepo   *repository.A2APeerTrustRepository
	consentRepo     *repository.A2AConsentRepository
	trustScoreRepo  *repository.A2ATrustScoreRepository
	nonceRepo       *repository.A2ARequestNonceRepository
	policyRepo      *repository.A2APolicyRepository
	attestationRepo *repository.A2AAgentAttestationRepository
	revokedRepo     *repository.A2ARevokedAgentRepository
	securityRepo    *repository.A2ASecuritySettingsRepository
	violationRepo   *repository.A2ASecurityViolationRepository
	agentRepo       *repository.AgentRepository
	keyVault        *crypto.KeyVault
	httpClient      *http.Client
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
	attestationRepo *repository.A2AAgentAttestationRepository,
	revokedRepo *repository.A2ARevokedAgentRepository,
	securityRepo *repository.A2ASecuritySettingsRepository,
	violationRepo *repository.A2ASecurityViolationRepository,
	agentRepo *repository.AgentRepository,
	keyVault *crypto.KeyVault,
) *A2AService {
	return &A2AService{
		cardRepo:        cardRepo,
		skillRepo:       skillRepo,
		taskRepo:        taskRepo,
		peerTrustRepo:   peerTrustRepo,
		consentRepo:     consentRepo,
		trustScoreRepo:  trustScoreRepo,
		nonceRepo:       nonceRepo,
		policyRepo:      policyRepo,
		attestationRepo: attestationRepo,
		revokedRepo:     revokedRepo,
		securityRepo:    securityRepo,
		violationRepo:   violationRepo,
		agentRepo:       agentRepo,
		keyVault:        keyVault,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			// SECURITY: Disable redirect following to prevent SSRF bypass.
			// An attacker could pass URL validation with a safe URL that redirects
			// to an internal address (e.g., cloud metadata endpoint).
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// ============================================================================
// Agent Card Operations
// ============================================================================

// RegisterAgentCardRequest is the request to register an A2A agent card
type RegisterAgentCardRequest struct {
	AgentID  uuid.UUID       `json:"agentId"`
	CardURL  string          `json:"cardUrl"`
	CardData json.RawMessage `json:"cardData"`
}

// RegisterAgentCardResponse is the response after registering an agent card
type RegisterAgentCardResponse struct {
	CardID             uuid.UUID `json:"cardId"`
	AgentID            uuid.UUID `json:"agentId"`
	CardURL            string    `json:"cardUrl"`
	AttestationExpires time.Time `json:"attestationExpires"`
	Skills             []string  `json:"skills"`
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

	// 2. Get card data: use provided cardData directly, or fetch from URL
	var cardData []byte
	if len(req.CardData) > 0 {
		cardData = []byte(req.CardData)
	} else {
		var fetchErr error
		cardData, fetchErr = s.fetchAgentCard(req.CardURL)
		if fetchErr != nil {
			return nil, fmt.Errorf("failed to fetch agent card: %w", fetchErr)
		}
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
	cardURL := req.CardURL
	if cardURL == "" {
		cardURL = fmt.Sprintf("inline://%s", req.AgentID.String())
	}
	now := time.Now().UTC()
	card := &domain.A2AAgentCard{
		ID:                   uuid.New(),
		AgentID:              req.AgentID,
		CardURL:              cardURL,
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
	skillIDs := make([]string, 0)
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

// ListAgentCards returns all valid agent cards with pagination
func (s *A2AService) ListAgentCards(ctx context.Context, limit, offset int) ([]*domain.A2AAgentCard, error) {
	return s.cardRepo.GetValidCards(ctx, limit, offset)
}

// GetEnhancedAgentCard returns the agent card with AIM extensions
// If no stored card exists, it dynamically builds one from the agent and its registered skills
func (s *A2AService) GetEnhancedAgentCard(ctx context.Context, agentID uuid.UUID) (*domain.A2AAgentCardParsed, error) {
	// Get the agent first - required for building the card
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil || agent == nil {
		return nil, fmt.Errorf("agent not found")
	}

	// Try to get stored card
	card, _ := s.cardRepo.GetByAgentID(ctx, agentID)

	var parsed domain.A2AAgentCardParsed

	if card != nil && len(card.CardData) > 0 {
		// Parse the stored card
		if err := json.Unmarshal(card.CardData, &parsed); err != nil {
			return nil, fmt.Errorf("failed to parse card data: %w", err)
		}
	} else {
		// Build card dynamically from agent and registered skills
		parsed = domain.A2AAgentCardParsed{
			Name:               agent.Name,
			Description:        agent.Description,
			URL:                fmt.Sprintf("/api/v1/a2a/agents/%s", agentID),
			Version:            "1.0.0",
			DefaultInputModes:  []string{"text"},
			DefaultOutputModes: []string{"text"},
		}

		// Get registered skills for this agent
		skills, _ := s.skillRepo.GetByAgentID(ctx, agentID)
		if len(skills) > 0 {
			parsed.Skills = make([]domain.A2ASkillDefinition, 0, len(skills))
			for _, skill := range skills {
				parsed.Skills = append(parsed.Skills, domain.A2ASkillDefinition{
					ID:          skill.SkillID,
					Name:        skill.Name,
					Description: skill.Description,
					Tags:        skill.Tags,
					InputModes:  skill.InputModes,
					OutputModes: skill.OutputModes,
				})
			}
		}
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

	if card != nil && card.AttestationSignature != "" {
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

	// Auto-generate keys if agent doesn't have a server-side private key
	// This happens when agents were registered with SDK-provided public keys
	if agent.EncryptedPrivateKey == nil || *agent.EncryptedPrivateKey == "" {
		keyPair, err := crypto.GenerateEd25519KeyPair()
		if err != nil {
			return nil, fmt.Errorf("failed to generate keys: %w", err)
		}
		encodedKeys := crypto.EncodeKeyPair(keyPair)

		encPrivKey, err := s.keyVault.EncryptPrivateKey(encodedKeys.PrivateKeyBase64)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt private key: %w", err)
		}
		agent.EncryptedPrivateKey = &encPrivKey
		agent.PublicKey = &encodedKeys.PublicKeyBase64
		if err := s.agentRepo.Update(agent); err != nil {
			return nil, fmt.Errorf("failed to update agent keys: %w", err)
		}
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

// ListPeerTrusts returns all peer trust relationships for an agent
func (s *A2AService) ListPeerTrusts(ctx context.Context, agentID uuid.UUID) ([]*domain.A2APeerTrust, error) {
	return s.peerTrustRepo.GetByAgentID(ctx, agentID)
}

// ComputeA2ATrustScore computes and stores the A2A trust score for an agent
func (s *A2AService) ComputeA2ATrustScore(ctx context.Context, agentID uuid.UUID) (*domain.A2ATrustScore, error) {
	// Verify agent exists before attempting to upsert (FK constraint on a2a_trust_scores)
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil || agent == nil {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}

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

// ListA2ATasks returns paginated A2A tasks with optional filters
func (s *A2AService) ListA2ATasks(ctx context.Context, callerOrgID uuid.UUID, agentID *uuid.UUID, state string, limit, offset int) ([]*domain.A2ATask, int, error) {
	return s.taskRepo.ListTasks(ctx, callerOrgID, agentID, state, limit, offset)
}

// LogA2ATaskRequest is the request to log an A2A task
type LogA2ATaskRequest struct {
	ExternalTaskID string    `json:"externalTaskId"`
	ContextID      string    `json:"contextId"`
	ClientAgentID  uuid.UUID `json:"clientAgentId"`
	RemoteAgentID  uuid.UUID `json:"remoteAgentId"`
	SkillID        string    `json:"skillId"`
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

// GetA2ATask loads an A2A task by ID. Returns nil + nil when not found.
// This accessor exists so the HTTP layer can run a LoadOwnedViaAgent
// tenant-ownership check (via the task's ClientAgentID) before
// UpdateA2ATaskState (audit doc A3d-vii.b).
func (s *A2AService) GetA2ATask(ctx context.Context, taskID uuid.UUID) (*domain.A2ATask, error) {
	return s.taskRepo.GetByID(ctx, taskID)
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
	UserID           string     `json:"userId"`
	OrganizationID   *uuid.UUID `json:"organizationId"`
	GrantorAgentID   uuid.UUID  `json:"grantorAgentId"`
	RecipientAgentID uuid.UUID  `json:"recipientAgentId"`
	Scope            []string   `json:"scope"`
	Purpose          string     `json:"purpose"`
	DataTypes        []string   `json:"dataTypes"`
	ExpiresInHours   int        `json:"expiresInHours"`
	ConsentMethod    string     `json:"consentMethod"`
	Evidence         string     `json:"evidence"`
	IPAddress        string     `json:"ipAddress"`
	UserAgent        string     `json:"userAgent"`
}

// RecordConsent records user consent for cross-agent data sharing
func (s *A2AService) RecordConsent(ctx context.Context, req RecordConsentRequest) (*domain.A2AConsentRecord, error) {
	// SECURITY: the caller must own the grantor. This is the check that makes
	// organization_id mean anything.
	//
	// The handler stamps organization_id from the caller's authenticated org,
	// but both agent IDs come straight off the request body and the only
	// constraint on them is a foreign key to agents(id) — which requires a real
	// agent, not one of yours. So without this check a caller could name any
	// organization's agent as grantor and have the row stamped with their own
	// org, and organization_id would diverge from the grantor's org on every
	// such write. Migration 097 asserts that ownership model when it backfills
	// organization_id from the grantor's agent, but that is a one-time backfill
	// of legacy NULL rows, not an enforced invariant. This is the enforcement.
	//
	// It lives at the service layer rather than the handler so the next caller
	// of RecordConsent inherits it. A handler-only check protects one route.
	if req.OrganizationID == nil || *req.OrganizationID == uuid.Nil {
		return nil, ErrConsentGrantorNotOwned
	}
	// GetByIDs rather than GetByID, because the org predicate belongs in the
	// query. GetByID would need the caller to compare organization_id after the
	// fact, and it reports a missing agent as an untyped "agent not found"
	// error — so a nonexistent grantor and a foreign one would come back
	// distinguishable, which is the enumeration oracle this check exists to
	// close. Here both are simply an empty result.
	owned, err := s.agentRepo.GetByIDs(ctx, *req.OrganizationID, []uuid.UUID{req.GrantorAgentID})
	if err != nil {
		return nil, fmt.Errorf("failed to verify consent grantor: %w", err)
	}
	if len(owned) == 0 {
		return nil, ErrConsentGrantorNotOwned
	}

	// The RECIPIENT is deliberately not checked. Naming another organization's
	// agent as recipient is what cross-agent consent is for.

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

// CheckConsent reports whether callerOrgID holds consent for a specific scope.
// SECURITY: callerOrgID is required; see the repository method for why the
// unscoped form was a cross-tenant oracle over user_id.
func (s *A2AService) CheckConsent(
	ctx context.Context,
	callerOrgID uuid.UUID,
	userID string,
	grantorAgentID, recipientAgentID uuid.UUID,
	scope string,
) (bool, error) {
	return s.consentRepo.CheckConsent(ctx, callerOrgID, userID, grantorAgentID, recipientAgentID, scope)
}

// RevokeConsent revokes a consent record
func (s *A2AService) RevokeConsent(ctx context.Context, consentID uuid.UUID, reason string) error {
	return s.consentRepo.Revoke(ctx, consentID, reason)
}

// GetConsent loads a consent record by ID. Returns nil + nil when not
// found. This accessor exists so the HTTP layer can run a LoadOwned
// tenant-ownership check before RevokeConsent (audit doc A3d-vii.b).
func (s *A2AService) GetConsent(ctx context.Context, consentID uuid.UUID) (*domain.A2AConsentRecord, error) {
	return s.consentRepo.GetByID(ctx, consentID)
}

// ListUserConsents lists all consent records for a user
// ListUserConsents lists consent records for a userID that belong to
// the caller's organization. SECURITY (A3c #42): orgID is required;
// without it, any authenticated caller could enumerate consents across
// tenants by walking userIDs.
func (s *A2AService) ListUserConsents(ctx context.Context, userID string, orgID uuid.UUID, includeRevoked bool) ([]*domain.A2AConsentRecord, error) {
	return s.consentRepo.ListByUser(ctx, userID, orgID, includeRevoked)
}

// ListAllConsents lists consent records belonging to orgID. SECURITY: orgID is
// required; without it any authenticated caller could page every tenant's
// consent records off GET /api/v1/a2a/consents.
func (s *A2AService) ListAllConsents(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.A2AConsentRecord, int, error) {
	return s.consentRepo.ListAll(ctx, orgID, limit, offset)
}

// ListAllTrustScores lists A2A trust scores for agents in orgID. SECURITY:
// orgID is required; see ListAllConsents.
func (s *A2AService) ListAllTrustScores(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.A2ATrustScore, int, error) {
	return s.trustScoreRepo.ListAll(ctx, orgID, limit, offset)
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
	evaluatedPolicies := make([]string, 0)
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

// RegisterSkill registers a skill directly for an agent
func (s *A2AService) RegisterSkill(ctx context.Context, skill *domain.A2ASkill) error {
	// Check if agent exists
	agent, err := s.agentRepo.GetByID(skill.AgentID)
	if err != nil || agent == nil {
		return fmt.Errorf("agent not found")
	}

	// Check if skill already exists
	existing, _ := s.skillRepo.GetBySkillID(ctx, skill.AgentID, skill.SkillID)
	if existing != nil {
		// Update existing skill
		existing.Name = skill.Name
		existing.Description = skill.Description
		existing.Tags = skill.Tags
		existing.InputModes = skill.InputModes
		existing.OutputModes = skill.OutputModes
		existing.Examples = skill.Examples
		return s.skillRepo.Update(ctx, existing)
	}

	// Create new skill
	return s.skillRepo.Create(ctx, skill)
}

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
// Agent-to-Agent Attestation (Multi-Agent Consensus)
// ============================================================================

// Consensus thresholds
const (
	DefaultMinAgentsForA2AConsensus  = 3
	DefaultMinOwnersForA2AConsensus  = 2
	MinA2AConfidenceForVerification  = 60.0
	DefaultMinTrustForA2AAttestation = 0.30
	DefaultAttestationValidityDays   = 30
)

// AutoAttestAfterTask creates attestation after successful A2A task
func (s *A2AService) AutoAttestAfterTask(ctx context.Context, task *domain.A2ATask) error {
	if task.State != domain.A2ATaskStateCompleted {
		return nil // Only attest successful tasks
	}

	// Check if client agent can attest (trust score threshold)
	clientAgent, err := s.agentRepo.GetByID(task.ClientAgentID)
	if err != nil || clientAgent == nil {
		return nil
	}
	if clientAgent.TrustScore < DefaultMinTrustForA2AAttestation {
		return nil // Low trust agents can't attest
	}

	// Check if remote agent is revoked
	revoked, _ := s.revokedRepo.IsRevoked(ctx, task.RemoteAgentID)
	if revoked {
		return nil // Don't attest revoked agents
	}

	// Create attestation
	attestation := &domain.A2AAgentAttestation{
		AttestingAgentID:   task.ClientAgentID,
		AttestedAgentID:    task.RemoteAgentID,
		SkillID:            task.SkillID,
		TaskID:             &task.ID,
		TaskCompleted:      true,
		ResponseTimeMs:     task.DurationMs,
		AttesterTrustScore: clientAgent.TrustScore,
		ExpiresAt:          time.Now().Add(time.Duration(DefaultAttestationValidityDays) * 24 * time.Hour),
	}

	// Sign attestation
	signature, err := s.signAttestation(ctx, task.ClientAgentID, attestation)
	if err != nil {
		return fmt.Errorf("failed to sign attestation: %w", err)
	}
	attestation.AttestationSignature = signature

	// Save attestation
	if err := s.attestationRepo.Create(ctx, attestation); err != nil {
		return fmt.Errorf("failed to save attestation: %w", err)
	}

	// Check consensus and possibly verify skill
	if task.SkillID != "" {
		s.checkAndApplyA2AConsensus(ctx, task.RemoteAgentID, task.SkillID)
	}

	return nil
}

// signAttestation signs an attestation with the attesting agent's key
func (s *A2AService) signAttestation(ctx context.Context, agentID uuid.UUID, att *domain.A2AAgentAttestation) (string, error) {
	// Get agent with private key
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil || agent == nil {
		return "", fmt.Errorf("agent not found")
	}

	if agent.EncryptedPrivateKey == nil || *agent.EncryptedPrivateKey == "" {
		return "", fmt.Errorf("agent has no private key")
	}

	// Decrypt private key
	privateKeyB64, err := s.keyVault.DecryptPrivateKey(*agent.EncryptedPrivateKey)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt private key: %w", err)
	}
	privateKey, err := base64.StdEncoding.DecodeString(privateKeyB64)
	if err != nil {
		return "", fmt.Errorf("failed to decode private key: %w", err)
	}

	// Create attestation payload
	payload := fmt.Sprintf("%s:%s:%s:%t:%d",
		att.AttestingAgentID.String(),
		att.AttestedAgentID.String(),
		att.SkillID,
		att.TaskCompleted,
		time.Now().UnixNano(),
	)

	hash := sha256.Sum256([]byte(payload))
	signature := ed25519.Sign(privateKey, hash[:])
	return base64.StdEncoding.EncodeToString(signature), nil
}

// checkAndApplyA2AConsensus checks if a skill meets consensus thresholds
func (s *A2AService) checkAndApplyA2AConsensus(ctx context.Context, agentID uuid.UUID, skillID string) {
	// Count unique attesters
	uniqueAttesters, _ := s.attestationRepo.CountUniqueAttesters(ctx, agentID, skillID)
	if uniqueAttesters < DefaultMinAgentsForA2AConsensus {
		return // Not enough attesters yet
	}

	// Count unique owners (organizations)
	uniqueOwners, _ := s.attestationRepo.CountUniqueOwners(ctx, agentID, skillID)
	if uniqueOwners < DefaultMinOwnersForA2AConsensus {
		return // Not enough independent owners
	}

	// Calculate confidence score
	confidence := float64(uniqueAttesters*10 + uniqueOwners*20)
	if confidence < MinA2AConfidenceForVerification {
		return
	}

	// Mark skill as verified (update a2a_skills table)
	s.skillRepo.MarkVerified(ctx, agentID, skillID)
}

// IsAgentRevoked checks if an agent has been revoked
func (s *A2AService) IsAgentRevoked(ctx context.Context, agentID uuid.UUID) (bool, error) {
	return s.revokedRepo.IsRevoked(ctx, agentID)
}

// RevokeAgent marks an agent as revoked
func (s *A2AService) RevokeAgent(ctx context.Context, agentID uuid.UUID, reason string, revokedBy *uuid.UUID) error {
	return s.revokedRepo.Revoke(ctx, agentID, reason, revokedBy)
}

// ReinstateAgent removes an agent from the revoked list
func (s *A2AService) ReinstateAgent(ctx context.Context, agentID uuid.UUID) error {
	return s.revokedRepo.Reinstate(ctx, agentID)
}

// GetAgentAttestations returns attestations for an agent
func (s *A2AService) GetAgentAttestations(ctx context.Context, agentID uuid.UUID, skillID string) ([]*domain.A2AAgentAttestation, error) {
	return s.attestationRepo.GetByAttestedAgent(ctx, agentID, skillID)
}

// AttestSkillRequest is the request to manually attest a skill
type AttestSkillRequest struct {
	AttestingAgentID uuid.UUID              `json:"attestingAgentId"`
	AttestedAgentID  uuid.UUID              `json:"attestedAgentId"`
	SkillID          string                 `json:"skillId"`
	AttestationType  string                 `json:"attestationType"`
	Confidence       float64                `json:"confidence"`
	Evidence         map[string]interface{} `json:"evidence"`
}

// AttestSkill creates a manual attestation for another agent's skill
func (s *A2AService) AttestSkill(ctx context.Context, req AttestSkillRequest) (*domain.A2AAgentAttestation, error) {
	// Validate attesting agent exists and has sufficient trust
	attestingAgent, err := s.agentRepo.GetByID(req.AttestingAgentID)
	if err != nil || attestingAgent == nil {
		return nil, fmt.Errorf("attesting agent not found")
	}
	if attestingAgent.TrustScore < DefaultMinTrustForA2AAttestation {
		return nil, fmt.Errorf("attesting agent trust score too low: %.2f < %.2f", attestingAgent.TrustScore, DefaultMinTrustForA2AAttestation)
	}

	// Validate attested agent exists
	attestedAgent, err := s.agentRepo.GetByID(req.AttestedAgentID)
	if err != nil || attestedAgent == nil {
		return nil, fmt.Errorf("attested agent not found")
	}

	// Check if attested agent is revoked
	revoked, _ := s.revokedRepo.IsRevoked(ctx, req.AttestedAgentID)
	if revoked {
		return nil, fmt.Errorf("cannot attest revoked agent")
	}

	// Serialize evidence
	var evidenceJSON []byte
	if req.Evidence != nil {
		evidenceJSON, _ = json.Marshal(req.Evidence)
	}

	// Create attestation
	attestation := &domain.A2AAgentAttestation{
		AttestingAgentID:   req.AttestingAgentID,
		AttestedAgentID:    req.AttestedAgentID,
		SkillID:            req.SkillID,
		TaskCompleted:      true, // Manual attestation assumes verified
		AttesterTrustScore: attestingAgent.TrustScore,
		ExpiresAt:          time.Now().Add(time.Duration(DefaultAttestationValidityDays) * 24 * time.Hour),
	}

	// Store attestation type and evidence in notes (if field exists) or ignore
	_ = evidenceJSON // Would be used if schema supports it

	// Sign attestation
	signature, err := s.signAttestation(ctx, req.AttestingAgentID, attestation)
	if err != nil {
		return nil, fmt.Errorf("failed to sign attestation: %w", err)
	}
	attestation.AttestationSignature = signature

	// Save attestation
	if err := s.attestationRepo.Create(ctx, attestation); err != nil {
		return nil, fmt.Errorf("failed to save attestation: %w", err)
	}

	// Check consensus and possibly verify skill
	if req.SkillID != "" {
		s.checkAndApplyA2AConsensus(ctx, req.AttestedAgentID, req.SkillID)
	}

	return attestation, nil
}

// GetConsensusStatus returns the consensus verification status for a skill
func (s *A2AService) GetConsensusStatus(ctx context.Context, agentID uuid.UUID, skillID string) (*domain.A2AConsensusResult, error) {
	attesters, _ := s.attestationRepo.CountUniqueAttesters(ctx, agentID, skillID)
	owners, _ := s.attestationRepo.CountUniqueOwners(ctx, agentID, skillID)
	attestations, _ := s.attestationRepo.GetByAttestedAgent(ctx, agentID, skillID)

	confidence := float64(attesters*10 + owners*20)
	isVerified := attesters >= DefaultMinAgentsForA2AConsensus &&
		owners >= DefaultMinOwnersForA2AConsensus &&
		confidence >= MinA2AConfidenceForVerification

	return &domain.A2AConsensusResult{
		SkillID:          skillID,
		AgentID:          agentID,
		AttestationCount: len(attestations),
		UniqueAttesters:  attesters,
		UniqueOwners:     owners,
		ConfidenceScore:  confidence,
		IsVerified:       isVerified,
	}, nil
}

// ============================================================================
// A2A Security Policy Enforcement
// ============================================================================

// GetSecuritySettings returns the A2A security settings for an organization
func (s *A2AService) GetSecuritySettings(ctx context.Context, orgID uuid.UUID) (*domain.A2ASecuritySettings, error) {
	settings, err := s.securityRepo.GetByOrganization(ctx, orgID)
	if err != nil {
		// Return default settings if none exist
		return &domain.A2ASecuritySettings{
			OrganizationID:         orgID,
			EnforcementMode:        domain.A2AEnforcementMonitor,
			RequireVerifiedSkills:  false,
			MinTrustScore:          0.0,
			MinAttestationCount:    0,
			RequireRequestSigning:  true,
			RequireNonceValidation: true,
			NonceValiditySeconds:   300,
			MaxRequestsPerMinute:   60,
			MaxRequestsPerHour:     1000,
		}, nil
	}
	return settings, nil
}

// UpdateSecuritySettings updates the A2A security settings for an organization
func (s *A2AService) UpdateSecuritySettings(ctx context.Context, settings *domain.A2ASecuritySettings) error {
	existing, err := s.securityRepo.GetByOrganization(ctx, settings.OrganizationID)
	if err != nil {
		// Create new settings
		settings.ID = uuid.New()
		settings.CreatedAt = time.Now()
		settings.UpdatedAt = time.Now()
		return s.securityRepo.Create(ctx, settings)
	}
	// Update existing
	settings.ID = existing.ID
	settings.UpdatedAt = time.Now()
	return s.securityRepo.Update(ctx, settings)
}

// CheckA2ASecurity evaluates all security policies for an A2A request
func (s *A2AService) CheckA2ASecurity(ctx context.Context, req *domain.A2ASecurityCheckRequest) (*domain.A2ASecurityCheckResult, error) {
	// Get target agent to find organization
	targetAgent, err := s.agentRepo.GetByID(req.TargetAgentID)
	if err != nil || targetAgent == nil {
		return &domain.A2ASecurityCheckResult{
			Allowed:    false,
			Violations: []domain.A2AViolationType{domain.A2AViolationAgentBlocked},
			Reason:     "target agent not found",
			Mode:       domain.A2AEnforcementStrict,
		}, nil
	}

	// Get security settings for the target agent's organization
	settings, _ := s.GetSecuritySettings(ctx, targetAgent.OrganizationID)
	violations := make([]domain.A2AViolationType, 0)

	// Check 1: Is requesting agent revoked?
	revoked, _ := s.revokedRepo.IsRevoked(ctx, req.RequestingAgentID)
	if revoked {
		violations = append(violations, domain.A2AViolationRevokedAgent)
	}

	// Check 2: Is requesting agent blocked?
	for _, blockedID := range settings.BlockedAgentIDs {
		if blockedID == req.RequestingAgentID {
			violations = append(violations, domain.A2AViolationAgentBlocked)
			break
		}
	}

	// Check 3: Is requesting agent in allowed list (if list is not empty)?
	if len(settings.AllowedAgentIDs) > 0 {
		allowed := false
		for _, allowedID := range settings.AllowedAgentIDs {
			if allowedID == req.RequestingAgentID {
				allowed = true
				break
			}
		}
		if !allowed {
			violations = append(violations, domain.A2AViolationAgentBlocked)
		}
	}

	// Check 4: Trust score requirement
	if settings.MinTrustScore > 0 {
		trustScore, _ := s.trustScoreRepo.GetByAgentID(ctx, req.RequestingAgentID)
		if trustScore == nil || trustScore.A2ATrustScore == nil || *trustScore.A2ATrustScore < settings.MinTrustScore {
			violations = append(violations, domain.A2AViolationLowTrust)
		}
	}

	// Check 5: Verified skill requirement
	if settings.RequireVerifiedSkills && req.SkillID != "" {
		skill, _ := s.skillRepo.GetBySkillID(ctx, req.TargetAgentID, req.SkillID)
		if skill == nil || !skill.IsVerified {
			violations = append(violations, domain.A2AViolationUnverifiedSkill)
		}
	}

	// Check 6: Request signing requirement
	if settings.RequireRequestSigning && req.RequestSignature == "" {
		violations = append(violations, domain.A2AViolationUnsignedRequest)
	}

	// Check 7: Nonce validation requirement
	if settings.RequireNonceValidation && req.RequestNonce != "" {
		// Check if nonce exists (unused nonces are valid)
		exists, _ := s.nonceRepo.Exists(ctx, req.RequestNonce)
		if exists {
			// Nonce already used - invalid
			violations = append(violations, domain.A2AViolationNonceInvalid)
		}
	}

	// Determine if request is allowed based on mode
	allowed := len(violations) == 0
	action := domain.A2ASecurityActionLogged
	if settings.EnforcementMode == domain.A2AEnforcementStrict && !allowed {
		action = domain.A2ASecurityActionBlocked
	} else if settings.EnforcementMode == domain.A2AEnforcementMonitor {
		// Monitor mode: allow request but log violations
		allowed = true
	}

	// Log violations
	if len(violations) > 0 {
		for _, v := range violations {
			violation := &domain.A2ASecurityViolation{
				ID:                uuid.New(),
				OrganizationID:    targetAgent.OrganizationID,
				RequestingAgentID: &req.RequestingAgentID,
				TargetAgentID:     &req.TargetAgentID,
				SkillID:           req.SkillID,
				TaskID:            req.TaskID,
				ViolationType:     v,
				ActionTaken:       action,
				RequestSignature:  req.RequestSignature,
				RequestNonce:      req.RequestNonce,
				RequestTimestamp:  req.RequestTimestamp,
				CreatedAt:         time.Now(),
			}
			s.violationRepo.Create(ctx, violation)
		}
	}

	reason := ""
	if !allowed {
		reason = fmt.Sprintf("blocked due to %d security violations", len(violations))
	}

	return &domain.A2ASecurityCheckResult{
		Allowed:    allowed,
		Violations: violations,
		Reason:     reason,
		Mode:       settings.EnforcementMode,
	}, nil
}

// GetSecurityViolations returns security violations for an organization
func (s *A2AService) GetSecurityViolations(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*domain.A2ASecurityViolation, error) {
	return s.violationRepo.GetByOrganization(ctx, orgID, limit, offset)
}

// GetSecurityViolationsByType returns security violations filtered by type
func (s *A2AService) GetSecurityViolationsByType(ctx context.Context, orgID uuid.UUID, violationType domain.A2AViolationType, limit, offset int) ([]*domain.A2ASecurityViolation, error) {
	return s.violationRepo.GetByType(ctx, orgID, violationType, limit, offset)
}

// GetViolationStats returns violation counts for an organization
func (s *A2AService) GetViolationStats(ctx context.Context, orgID uuid.UUID, since time.Time) (map[domain.A2AViolationType]int, int, error) {
	total, err := s.violationRepo.CountByOrganization(ctx, orgID, since)
	if err != nil {
		return nil, 0, err
	}

	stats := make(map[domain.A2AViolationType]int)
	violationTypes := []domain.A2AViolationType{
		domain.A2AViolationLowTrust,
		domain.A2AViolationUnverifiedSkill,
		domain.A2AViolationUnsignedRequest,
		domain.A2AViolationNonceInvalid,
		domain.A2AViolationAgentBlocked,
		domain.A2AViolationRateLimited,
		domain.A2AViolationPolicyDenied,
		domain.A2AViolationRevokedAgent,
	}

	for _, vt := range violationTypes {
		count, _ := s.violationRepo.CountByType(ctx, orgID, vt, since)
		if count > 0 {
			stats[vt] = count
		}
	}

	return stats, total, nil
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
	// SECURITY: Validate URL to prevent SSRF attacks
	if err := utils.ValidateExternalURL(url); err != nil {
		return nil, fmt.Errorf("invalid agent card URL: %w", err)
	}

	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Limit response size to 1MB to prevent memory exhaustion
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return body, nil
}

func (s *A2AService) createCardAttestation(agent *domain.Agent, cardData json.RawMessage, expiresAt time.Time) (string, error) {
	// SECURITY: Use the AIM server's signing key for attestation, NOT the agent's own key.
	// Self-attestation provides no trust guarantee — an attacker controlling an agent
	// could sign any card data. Server-signed attestation proves the AIM platform
	// verified and approved this agent card.
	serverPrivateKey := s.keyVault.GetServerSigningKey()
	if serverPrivateKey == nil {
		return "", fmt.Errorf("server signing key not configured — cannot issue attestation")
	}

	// Create attestation payload
	attestPayload := struct {
		CardHash  string    `json:"cardHash"`
		AgentID   string    `json:"agentId"`
		Issuer    string    `json:"issuer"`
		IssuedAt  time.Time `json:"issuedAt"`
		ExpiresAt time.Time `json:"expiresAt"`
	}{
		CardHash:  hex.EncodeToString(sha256Hash(cardData)),
		AgentID:   agent.ID.String(),
		Issuer:    "aim-server",
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: expiresAt,
	}

	payloadJSON, _ := json.Marshal(attestPayload)
	payloadHash := sha256.Sum256(payloadJSON)

	signature := ed25519.Sign(serverPrivateKey, payloadHash[:])
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
