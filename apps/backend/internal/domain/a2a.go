package domain

import (
	"context"
	"encoding/json"
	"net"
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// A2A Types and Constants
// ============================================================================

// A2ATaskState represents the lifecycle state of an A2A task
type A2ATaskState string

const (
	A2ATaskStateSubmitted   A2ATaskState = "SUBMITTED"
	A2ATaskStateWorking     A2ATaskState = "WORKING"
	A2ATaskStateInputNeeded A2ATaskState = "INPUT_NEEDED"
	A2ATaskStateCompleted   A2ATaskState = "COMPLETED"
	A2ATaskStateFailed      A2ATaskState = "FAILED"
	A2ATaskStateCancelled   A2ATaskState = "CANCELLED"
)

// A2AMessageRole represents who sent a message in an A2A task
type A2AMessageRole string

const (
	A2AMessageRoleUser  A2AMessageRole = "user"
	A2AMessageRoleAgent A2AMessageRole = "agent"
)

// A2AConsentMethod represents how consent was obtained
type A2AConsentMethod string

const (
	A2AConsentMethodExplicitClick A2AConsentMethod = "explicit_click"
	A2AConsentMethodAPI           A2AConsentMethod = "api"
	A2AConsentMethodDelegated     A2AConsentMethod = "delegated"
	A2AConsentMethodInherited     A2AConsentMethod = "inherited"
)

// ============================================================================
// A2A Agent Card
// ============================================================================

// A2AAgentCard represents an A2A Agent Card with AIM attestation
type A2AAgentCard struct {
	ID      uuid.UUID `json:"id"`
	AgentID uuid.UUID `json:"agentId"`

	// A2A Card data
	CardURL         string          `json:"cardUrl"`
	CardData        json.RawMessage `json:"cardData"`
	CardHash        string          `json:"cardHash"`
	ProtocolVersion string          `json:"protocolVersion"`

	// AIM attestation
	AttestationSignature string     `json:"attestationSignature,omitempty"`
	AttestationIssuedAt  *time.Time `json:"attestationIssuedAt,omitempty"`
	AttestationExpiresAt *time.Time `json:"attestationExpiresAt,omitempty"`

	// Validation
	IsValid         bool   `json:"isValid"`
	ValidationError string `json:"validationError,omitempty"`

	// Metadata
	LastFetchedAt *time.Time `json:"lastFetchedAt,omitempty"`
	FetchCount    int        `json:"fetchCount"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// A2AAgentCardParsed represents the parsed content of an A2A Agent Card
type A2AAgentCardParsed struct {
	Name               string                 `json:"name"`
	Description        string                 `json:"description"`
	URL                string                 `json:"url"`
	Version            string                 `json:"version"`
	Skills             []A2ASkillDefinition   `json:"skills"`
	SecuritySchemes    map[string]interface{} `json:"securitySchemes,omitempty"`
	DefaultInputModes  []string               `json:"defaultInputModes,omitempty"`
	DefaultOutputModes []string               `json:"defaultOutputModes,omitempty"`

	// AIM extension
	AIM *A2AAIMExtension `json:"aim,omitempty"`
}

// A2ASkillDefinition represents a skill defined in an Agent Card
type A2ASkillDefinition struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	InputModes  []string `json:"inputModes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty"`
	Examples    []string `json:"examples,omitempty"`
}

// A2AAIMExtension represents AIM-specific extensions to an Agent Card
type A2AAIMExtension struct {
	AgentID      uuid.UUID `json:"agentId"`
	PublicKey    string    `json:"publicKey"`
	AIMServer    string    `json:"aimServer"`
	TrustScore   float64   `json:"trustScore"`
	Capabilities []string  `json:"capabilities"`
	Organization string    `json:"organization,omitempty"`

	Attestation *A2AAttestation `json:"attestation,omitempty"`
	Behavior    *A2ABehavior    `json:"behavior,omitempty"`
}

// A2AAttestation represents AIM attestation on an Agent Card
type A2AAttestation struct {
	Issuer    string    `json:"issuer"`
	IssuedAt  time.Time `json:"issuedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	Signature string    `json:"signature"`
}

// A2ABehavior represents behavioral metrics in an Agent Card
type A2ABehavior struct {
	TotalTasksCompleted int64     `json:"totalTasksCompleted"`
	SuccessRate         float64   `json:"successRate"`
	AvgResponseTimeMs   int       `json:"avgResponseTimeMs"`
	LastActive          time.Time `json:"lastActive"`
}

// ============================================================================
// A2A Skill
// ============================================================================

// A2ASkill represents a skill registered for an agent
type A2ASkill struct {
	ID      uuid.UUID `json:"id"`
	AgentID uuid.UUID `json:"agentId"`

	// Skill definition
	SkillID     string   `json:"skillId"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	InputModes  []string `json:"inputModes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty"`
	Examples    []string `json:"examples,omitempty"`

	// Usage tracking
	InvocationCount int `json:"invocationCount"`
	SuccessCount    int `json:"successCount"`
	FailureCount    int `json:"failureCount"`
	AvgDurationMs   int `json:"avgDurationMs,omitempty"`

	// Verification status (from multi-agent consensus)
	IsVerified       bool       `json:"isVerified"`
	VerifiedAt       *time.Time `json:"verifiedAt,omitempty"`
	AttestationCount int        `json:"attestationCount"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ============================================================================
// A2A Task
// ============================================================================

// A2ATask represents an A2A task between two agents
type A2ATask struct {
	ID             uuid.UUID `json:"id"`
	ExternalTaskID string    `json:"externalTaskId"`
	ContextID      string    `json:"contextId,omitempty"`

	// Participants
	ClientAgentID uuid.UUID `json:"clientAgentId"`
	RemoteAgentID uuid.UUID `json:"remoteAgentId"`

	// Task info
	SkillID string       `json:"skillId,omitempty"`
	State   A2ATaskState `json:"state"`

	// Timing
	CreatedAt   time.Time  `json:"createdAt"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	DurationMs  int        `json:"durationMs,omitempty"`

	// Policy decision
	PolicyDecision    json.RawMessage `json:"policyDecision,omitempty"`
	PolicyEvaluatedAt *time.Time      `json:"policyEvaluatedAt,omitempty"`

	// Trust snapshots
	ClientTrustScoreSnapshot *float64 `json:"clientTrustScoreSnapshot,omitempty"`
	RemoteTrustScoreSnapshot *float64 `json:"remoteTrustScoreSnapshot,omitempty"`

	// Message tracking
	MessageCount int `json:"messageCount"`

	// Error info
	ErrorCode    string `json:"errorCode,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// ============================================================================
// A2A Message
// ============================================================================

// A2AMessage represents a message within an A2A task
type A2AMessage struct {
	ID                uuid.UUID `json:"id"`
	TaskID            uuid.UUID `json:"taskId"`
	ExternalMessageID string    `json:"externalMessageId,omitempty"`

	// Message info
	Role          A2AMessageRole `json:"role"`
	SenderAgentID uuid.UUID      `json:"senderAgentId"`

	// Content summary (actual content not stored)
	PartsSummary json.RawMessage `json:"partsSummary,omitempty"`
	ContainsPII  bool            `json:"containsPii"`
	PIITypes     []string        `json:"piiTypes,omitempty"`

	// Signature verification
	AIMSignature        string     `json:"aimSignature,omitempty"`
	SignatureValid      *bool      `json:"signatureValid,omitempty"`
	SignatureVerifiedAt *time.Time `json:"signatureVerifiedAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}

// A2AMessagePartSummary represents a summary of a message part
type A2AMessagePartSummary struct {
	Type     string `json:"type"`
	Size     int    `json:"size"`
	MimeType string `json:"mimeType,omitempty"`
}

// ============================================================================
// A2A Peer Trust
// ============================================================================

// A2APeerTrust represents trust relationship between two agents
type A2APeerTrust struct {
	ID          uuid.UUID `json:"id"`
	AgentID     uuid.UUID `json:"agentId"`
	PeerAgentID uuid.UUID `json:"peerAgentId"`

	// Interaction stats as client
	TasksInitiated          int `json:"tasksInitiated"`
	TasksInitiatedCompleted int `json:"tasksInitiatedCompleted"`
	TasksInitiatedFailed    int `json:"tasksInitiatedFailed"`

	// Interaction stats as server
	TasksReceived          int `json:"tasksReceived"`
	TasksReceivedCompleted int `json:"tasksReceivedCompleted"`
	TasksReceivedFailed    int `json:"tasksReceivedFailed"`

	// Performance metrics
	SuccessRate       *float64 `json:"successRate,omitempty"`
	AvgResponseTimeMs *int     `json:"avgResponseTimeMs,omitempty"`
	P95ResponseTimeMs *int     `json:"p95ResponseTimeMs,omitempty"`

	// Trust metrics
	PeerTrustScore  *float64   `json:"peerTrustScore,omitempty"`
	TrustComputedAt *time.Time `json:"trustComputedAt,omitempty"`
	TrustDataPoints int        `json:"trustDataPoints"`

	// Interaction tracking
	FirstInteractionAt *time.Time `json:"firstInteractionAt,omitempty"`
	LastInteractionAt  *time.Time `json:"lastInteractionAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ============================================================================
// A2A Consent Record
// ============================================================================

// A2AConsentRecord represents user consent for cross-agent data sharing
type A2AConsentRecord struct {
	ID             uuid.UUID  `json:"id"`
	UserID         string     `json:"userId"`
	OrganizationID *uuid.UUID `json:"organizationId,omitempty"`

	// Agents involved
	GrantorAgentID   uuid.UUID `json:"grantorAgentId"`
	RecipientAgentID uuid.UUID `json:"recipientAgentId"`

	// Consent details
	Scope     []string `json:"scope"`
	Purpose   string   `json:"purpose"`
	DataTypes []string `json:"dataTypes"`

	// Validity
	GrantedAt     time.Time  `json:"grantedAt"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
	Revoked       bool       `json:"revoked"`
	RevokedAt     *time.Time `json:"revokedAt,omitempty"`
	RevokedReason string     `json:"revokedReason,omitempty"`

	// Audit
	ConsentMethod A2AConsentMethod `json:"consentMethod"`
	Evidence      string           `json:"evidence,omitempty"`
	IPAddress     net.IP           `json:"ipAddress,omitempty"`
	UserAgent     string           `json:"userAgent,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ============================================================================
// A2A Trust Score
// ============================================================================

// A2ATrustScore represents aggregated A2A trust metrics for an agent
type A2ATrustScore struct {
	AgentID uuid.UUID `json:"agentId"`

	// Task statistics
	TotalTasksAsClient int `json:"totalTasksAsClient"`
	TotalTasksAsRemote int `json:"totalTasksAsRemote"`
	TasksCompleted     int `json:"tasksCompleted"`
	TasksFailed        int `json:"tasksFailed"`
	TasksCancelled     int `json:"tasksCancelled"`

	// Performance metrics
	AvgResponseTimeMs *int `json:"avgResponseTimeMs,omitempty"`
	P95ResponseTimeMs *int `json:"p95ResponseTimeMs,omitempty"`
	AvgTaskDurationMs *int `json:"avgTaskDurationMs,omitempty"`

	// Trust components
	A2ATrustScore    *float64 `json:"a2aTrustScore,omitempty"`
	PeerTrustAverage *float64 `json:"peerTrustAverage,omitempty"`
	UniquePeersCount int      `json:"uniquePeersCount"`

	// Reputation
	PositiveFeedbackCount int `json:"positiveFeedbackCount"`
	NegativeFeedbackCount int `json:"negativeFeedbackCount"`

	// Metadata
	ComputedAt *time.Time `json:"computedAt,omitempty"`
	DataPoints int        `json:"dataPoints"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ============================================================================
// A2A Request Nonce
// ============================================================================

// A2ARequestNonce represents a nonce for anti-replay protection
type A2ARequestNonce struct {
	Nonce       string    `json:"nonce"`
	AgentID     uuid.UUID `json:"agentId"`
	UsedAt      time.Time `json:"usedAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
	RequestHash string    `json:"requestHash,omitempty"`
}

// ============================================================================
// A2A Policy
// ============================================================================

// A2APolicy represents a cross-agent policy
type A2APolicy struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID *uuid.UUID `json:"organizationId,omitempty"`

	// Policy definition
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	PolicyYAML  string `json:"policyYaml"`

	// Targeting
	AppliesToAgents []string `json:"appliesToAgents,omitempty"`
	AppliesToSkills []string `json:"appliesToSkills,omitempty"`

	// Status
	IsActive bool `json:"isActive"`
	Priority int  `json:"priority"`

	// Audit
	CreatedBy *uuid.UUID `json:"createdBy,omitempty"`
	UpdatedBy *uuid.UUID `json:"updatedBy,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// ============================================================================
// A2A Request Signature
// ============================================================================

// A2ARequestSignature represents headers for A2A request signing
type A2ARequestSignature struct {
	AgentID   uuid.UUID `json:"agentId"`
	Timestamp int64     `json:"timestamp"`
	Nonce     string    `json:"nonce"`
	Signature string    `json:"signature"`
}

// A2ASignatureVerificationResult represents the result of signature verification
type A2ASignatureVerificationResult struct {
	Valid            bool      `json:"valid"`
	AgentID          uuid.UUID `json:"agentId,omitempty"`
	AgentName        string    `json:"agentName,omitempty"`
	TrustScore       float64   `json:"trustScore,omitempty"`
	Capabilities     []string  `json:"capabilities,omitempty"`
	AttestationValid bool      `json:"attestationValid"`
	Error            string    `json:"error,omitempty"`
}

// ============================================================================
// A2A Policy Evaluation
// ============================================================================

// A2APolicyDecision represents the result of policy evaluation
type A2APolicyDecision struct {
	Decision          string    `json:"decision"` // ALLOW, DENY
	PoliciesEvaluated []string  `json:"policiesEvaluated"`
	Reason            string    `json:"reason,omitempty"`
	EvaluatedAt       time.Time `json:"evaluatedAt"`
}

// ============================================================================
// A2A Discovery / Intent Routing
// ============================================================================

// RoutedAgent represents an agent matched by intent-based discovery
type RoutedAgent struct {
	SkillUUID        uuid.UUID `json:"skillUuid"`
	AgentID          uuid.UUID `json:"agentId"`
	SkillID          string    `json:"skillId"`
	SkillName        string    `json:"skillName"`
	SkillDescription string    `json:"skillDescription,omitempty"`
	AgentName        string    `json:"agentName"`
	AgentStatus      string    `json:"agentStatus"`
	TrustScore       float64   `json:"trustScore"`
	Relevance        float64   `json:"relevance"`
}

// RouteIntentRequest represents a request to route by intent
type RouteIntentRequest struct {
	Intent        string  `json:"intent"`
	MinTrustScore float64 `json:"minTrustScore,omitempty"`
}

// RouteIntentResponse represents the routing response
type RouteIntentResponse struct {
	Agent        *RoutedAgent `json:"agent,omitempty"`
	Alternatives int          `json:"alternatives"`
	Error        string       `json:"error,omitempty"`
}

// ============================================================================
// A2A Agent Attestation (Multi-Agent Consensus)
// ============================================================================

// A2AAgentAttestation represents one agent attesting to another's capabilities
type A2AAgentAttestation struct {
	ID uuid.UUID `json:"id"`

	// Participants
	AttestingAgentID uuid.UUID `json:"attestingAgentId"`
	AttestedAgentID  uuid.UUID `json:"attestedAgentId"`

	// What was attested
	SkillID string     `json:"skillId,omitempty"`
	TaskID  *uuid.UUID `json:"taskId,omitempty"`

	// Attestation result
	TaskCompleted  bool    `json:"taskCompleted"`
	ResponseTimeMs int     `json:"responseTimeMs,omitempty"`
	QualityScore   float64 `json:"qualityScore,omitempty"`

	// Cryptographic proof
	AttestationSignature string `json:"attestationSignature"`
	Challenge            string `json:"challenge,omitempty"`

	// Trust weight
	AttesterTrustScore float64 `json:"attesterTrustScore,omitempty"`

	// Validity
	VerifiedAt    time.Time  `json:"verifiedAt"`
	ExpiresAt     time.Time  `json:"expiresAt"`
	IsValid       bool       `json:"isValid"`
	RevokedAt     *time.Time `json:"revokedAt,omitempty"`
	RevokedReason string     `json:"revokedReason,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}

// A2ARevokedAgent represents a revoked/banned agent
type A2ARevokedAgent struct {
	AgentID   uuid.UUID  `json:"agentId"`
	RevokedAt time.Time  `json:"revokedAt"`
	Reason    string     `json:"reason,omitempty"`
	RevokedBy *uuid.UUID `json:"revokedBy,omitempty"`
}

// A2AConsensusResult represents the consensus status for a skill
type A2AConsensusResult struct {
	SkillID          string    `json:"skillId"`
	AgentID          uuid.UUID `json:"agentId"`
	AttestationCount int       `json:"attestationCount"`
	UniqueAttesters  int       `json:"uniqueAttesters"`
	UniqueOwners     int       `json:"uniqueOwners"`
	ConfidenceScore  float64   `json:"confidenceScore"`
	IsVerified       bool      `json:"isVerified"`
	VerifiedAt       time.Time `json:"verifiedAt,omitempty"`
}

// ============================================================================
// Repository Interfaces
// ============================================================================

// A2AAgentCardRepository defines operations for A2A agent cards
type A2AAgentCardRepository interface {
	Create(ctx context.Context, card *A2AAgentCard) error
	GetByAgentID(ctx context.Context, agentID uuid.UUID) (*A2AAgentCard, error)
	Update(ctx context.Context, card *A2AAgentCard) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetValidCards(ctx context.Context, limit, offset int) ([]*A2AAgentCard, error)
	GetExpiredCards(ctx context.Context) ([]*A2AAgentCard, error)
}

// A2ASkillRepository defines operations for A2A skills
type A2ASkillRepository interface {
	Create(ctx context.Context, skill *A2ASkill) error
	GetByAgentID(ctx context.Context, agentID uuid.UUID) ([]*A2ASkill, error)
	GetBySkillID(ctx context.Context, agentID uuid.UUID, skillID string) (*A2ASkill, error)
	Update(ctx context.Context, skill *A2ASkill) error
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByAgentID(ctx context.Context, agentID uuid.UUID) error
	Search(ctx context.Context, query string, limit int) ([]*A2ASkill, error)
	IncrementUsage(ctx context.Context, id uuid.UUID, success bool, durationMs int) error

	// Intent-based discovery (FTS)
	SearchByIntent(ctx context.Context, intent string, minTrustScore float64, limit int) ([]*RoutedAgent, error)
	CountByIntent(ctx context.Context, intent string, minTrustScore float64) (int, error)
}

// A2ATaskRepository defines operations for A2A tasks
type A2ATaskRepository interface {
	Create(ctx context.Context, task *A2ATask) error
	GetByID(ctx context.Context, id uuid.UUID) (*A2ATask, error)
	GetByExternalID(ctx context.Context, externalID string) (*A2ATask, error)
	Update(ctx context.Context, task *A2ATask) error
	ListByClientAgent(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*A2ATask, error)
	ListByRemoteAgent(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*A2ATask, error)
	CountByState(ctx context.Context, agentID uuid.UUID, state A2ATaskState) (int, error)
}

// A2AMessageRepository defines operations for A2A messages
type A2AMessageRepository interface {
	Create(ctx context.Context, msg *A2AMessage) error
	GetByTaskID(ctx context.Context, taskID uuid.UUID) ([]*A2AMessage, error)
	CountByTaskID(ctx context.Context, taskID uuid.UUID) (int, error)
}

// A2APeerTrustRepository defines operations for A2A peer trust
type A2APeerTrustRepository interface {
	Upsert(ctx context.Context, trust *A2APeerTrust) error
	GetByPeer(ctx context.Context, agentID, peerAgentID uuid.UUID) (*A2APeerTrust, error)
	GetByAgentID(ctx context.Context, agentID uuid.UUID) ([]*A2APeerTrust, error)
	GetTopPeers(ctx context.Context, agentID uuid.UUID, limit int) ([]*A2APeerTrust, error)
	ComputeAveragePeerTrust(ctx context.Context, agentID uuid.UUID) (float64, error)
}

// A2AConsentRepository defines operations for A2A consent records
type A2AConsentRepository interface {
	Create(ctx context.Context, consent *A2AConsentRecord) error
	GetByID(ctx context.Context, id uuid.UUID) (*A2AConsentRecord, error)
	CheckConsent(ctx context.Context, userID string, grantorID, recipientID uuid.UUID, scope string) (bool, error)
	ListByUser(ctx context.Context, userID string, orgID uuid.UUID, includeRevoked bool) ([]*A2AConsentRecord, error)
	// ListAll is org-scoped despite the name. orgID is required; see the
	// implementation comment for why an unscoped variant must not exist.
	ListAll(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*A2AConsentRecord, int, error)
	Revoke(ctx context.Context, id uuid.UUID, reason string) error
}

// A2ATrustScoreRepository defines operations for A2A trust scores
type A2ATrustScoreRepository interface {
	Upsert(ctx context.Context, score *A2ATrustScore) error
	GetByAgentID(ctx context.Context, agentID uuid.UUID) (*A2ATrustScore, error)
	// ListAll is org-scoped despite the name; it reaches the tenant boundary
	// by joining agents, since a2a_trust_scores has no organization_id.
	ListAll(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*A2ATrustScore, int, error)
	IncrementTaskStats(ctx context.Context, agentID uuid.UUID, asClient bool, completed bool) error
	UpdatePerformanceMetrics(ctx context.Context, agentID uuid.UUID, responseTimeMs, durationMs int) error
}

// A2ARequestNonceRepository defines operations for A2A request nonces
type A2ARequestNonceRepository interface {
	Create(ctx context.Context, nonce *A2ARequestNonce) error
	Exists(ctx context.Context, nonce string) (bool, error)
	DeleteExpired(ctx context.Context) (int, error)
}

// A2APolicyRepository defines operations for A2A policies
type A2APolicyRepository interface {
	Create(ctx context.Context, policy *A2APolicy) error
	GetByID(ctx context.Context, id uuid.UUID) (*A2APolicy, error)
	GetByOrganization(ctx context.Context, orgID uuid.UUID) ([]*A2APolicy, error)
	GetActivePolicies(ctx context.Context, orgID uuid.UUID) ([]*A2APolicy, error)
	Update(ctx context.Context, policy *A2APolicy) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// A2AAgentAttestationRepository defines operations for agent attestations
type A2AAgentAttestationRepository interface {
	Create(ctx context.Context, attestation *A2AAgentAttestation) error
	GetByID(ctx context.Context, id uuid.UUID) (*A2AAgentAttestation, error)
	GetByAttestedAgent(ctx context.Context, agentID uuid.UUID, skillID string) ([]*A2AAgentAttestation, error)
	CountUniqueAttesters(ctx context.Context, agentID uuid.UUID, skillID string) (int, error)
	CountUniqueOwners(ctx context.Context, agentID uuid.UUID, skillID string) (int, error)
	Revoke(ctx context.Context, id uuid.UUID, reason string) error
	DeleteExpired(ctx context.Context) (int, error)
}

// A2ARevokedAgentRepository defines operations for revoked agents
type A2ARevokedAgentRepository interface {
	Revoke(ctx context.Context, agentID uuid.UUID, reason string, revokedBy *uuid.UUID) error
	IsRevoked(ctx context.Context, agentID uuid.UUID) (bool, error)
	GetByAgentID(ctx context.Context, agentID uuid.UUID) (*A2ARevokedAgent, error)
	List(ctx context.Context, limit, offset int) ([]*A2ARevokedAgent, error)
	Reinstate(ctx context.Context, agentID uuid.UUID) error
}

// ============================================================================
// A2A Security Settings (Monitor/Strict enforcement modes)
// ============================================================================

// A2AEnforcementMode defines the security enforcement behavior
type A2AEnforcementMode string

const (
	// A2AEnforcementMonitor logs violations but allows requests
	A2AEnforcementMonitor A2AEnforcementMode = "monitor"
	// A2AEnforcementStrict blocks requests that violate policy
	A2AEnforcementStrict A2AEnforcementMode = "strict"
)

// A2ASecuritySettings defines organization-level A2A security configuration
type A2ASecuritySettings struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`

	// Enforcement mode
	EnforcementMode A2AEnforcementMode `json:"enforcementMode"`

	// Trust requirements
	RequireVerifiedSkills bool    `json:"requireVerifiedSkills"`
	MinTrustScore         float64 `json:"minTrustScore"`
	MinAttestationCount   int     `json:"minAttestationCount"`

	// Request signing
	RequireRequestSigning  bool `json:"requireRequestSigning"`
	RequireNonceValidation bool `json:"requireNonceValidation"`
	NonceValiditySeconds   int  `json:"nonceValiditySeconds"`

	// Agent restrictions
	AllowedAgentIDs []uuid.UUID `json:"allowedAgentIds,omitempty"`
	BlockedAgentIDs []uuid.UUID `json:"blockedAgentIds,omitempty"`

	// Rate limiting
	MaxRequestsPerMinute int `json:"maxRequestsPerMinute"`
	MaxRequestsPerHour   int `json:"maxRequestsPerHour"`

	// Audit
	CreatedBy *uuid.UUID `json:"createdBy,omitempty"`
	UpdatedBy *uuid.UUID `json:"updatedBy,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// A2AViolationType defines the type of security violation
type A2AViolationType string

const (
	A2AViolationLowTrust        A2AViolationType = "LOW_TRUST"
	A2AViolationUnverifiedSkill A2AViolationType = "UNVERIFIED_SKILL"
	A2AViolationUnsignedRequest A2AViolationType = "UNSIGNED_REQUEST"
	A2AViolationNonceInvalid    A2AViolationType = "NONCE_INVALID"
	A2AViolationAgentBlocked    A2AViolationType = "AGENT_BLOCKED"
	A2AViolationRateLimited     A2AViolationType = "RATE_LIMITED"
	A2AViolationPolicyDenied    A2AViolationType = "POLICY_DENIED"
	A2AViolationRevokedAgent    A2AViolationType = "REVOKED_AGENT"
)

// A2ASecurityAction defines the action taken for a violation
type A2ASecurityAction string

const (
	A2ASecurityActionLogged  A2ASecurityAction = "LOGGED"  // Monitor mode
	A2ASecurityActionBlocked A2ASecurityAction = "BLOCKED" // Strict mode
)

// A2ASecurityViolation represents a recorded policy violation
type A2ASecurityViolation struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`

	// Request details
	RequestingAgentID *uuid.UUID `json:"requestingAgentId,omitempty"`
	TargetAgentID     *uuid.UUID `json:"targetAgentId,omitempty"`
	SkillID           string     `json:"skillId,omitempty"`
	TaskID            *uuid.UUID `json:"taskId,omitempty"`

	// Violation details
	ViolationType    A2AViolationType       `json:"violationType"`
	ViolationDetails map[string]interface{} `json:"violationDetails,omitempty"`
	PolicyID         *uuid.UUID             `json:"policyId,omitempty"`

	// Action taken
	ActionTaken A2ASecurityAction `json:"actionTaken"`

	// Request context
	RequestSignature string     `json:"requestSignature,omitempty"`
	RequestNonce     string     `json:"requestNonce,omitempty"`
	RequestTimestamp *time.Time `json:"requestTimestamp,omitempty"`

	// Metadata
	CreatedAt time.Time `json:"createdAt"`
}

// A2ASecurityCheckRequest represents a request to check A2A security
type A2ASecurityCheckRequest struct {
	RequestingAgentID uuid.UUID  `json:"requestingAgentId"`
	TargetAgentID     uuid.UUID  `json:"targetAgentId"`
	SkillID           string     `json:"skillId,omitempty"`
	TaskID            *uuid.UUID `json:"taskId,omitempty"`
	RequestSignature  string     `json:"requestSignature,omitempty"`
	RequestNonce      string     `json:"requestNonce,omitempty"`
	RequestTimestamp  *time.Time `json:"requestTimestamp,omitempty"`
}

// A2ASecurityCheckResult represents the result of a security check
type A2ASecurityCheckResult struct {
	Allowed    bool               `json:"allowed"`
	Violations []A2AViolationType `json:"violations,omitempty"`
	Reason     string             `json:"reason,omitempty"`
	Mode       A2AEnforcementMode `json:"mode"`
}

// A2ASecuritySettingsRepository defines operations for security settings
type A2ASecuritySettingsRepository interface {
	Create(ctx context.Context, settings *A2ASecuritySettings) error
	GetByOrganization(ctx context.Context, orgID uuid.UUID) (*A2ASecuritySettings, error)
	Update(ctx context.Context, settings *A2ASecuritySettings) error
	Delete(ctx context.Context, orgID uuid.UUID) error
}

// A2ASecurityViolationRepository defines operations for security violations
type A2ASecurityViolationRepository interface {
	Create(ctx context.Context, violation *A2ASecurityViolation) error
	GetByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*A2ASecurityViolation, error)
	GetByAgent(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*A2ASecurityViolation, error)
	GetByType(ctx context.Context, orgID uuid.UUID, violationType A2AViolationType, limit, offset int) ([]*A2ASecurityViolation, error)
	CountByOrganization(ctx context.Context, orgID uuid.UUID, since time.Time) (int, error)
	CountByType(ctx context.Context, orgID uuid.UUID, violationType A2AViolationType, since time.Time) (int, error)
}

// ============================================================================
// A2A Supply Chain Tracking
// ============================================================================

// A2ACallChainStatus represents the status of a call in the chain
type A2ACallChainStatus string

const (
	A2ACallChainActive    A2ACallChainStatus = "ACTIVE"
	A2ACallChainCompleted A2ACallChainStatus = "COMPLETED"
	A2ACallChainFailed    A2ACallChainStatus = "FAILED"
	A2ACallChainTimeout   A2ACallChainStatus = "TIMEOUT"
)

// A2ACallChain represents a single call in an agent-to-agent chain
type A2ACallChain struct {
	ID             uuid.UUID  `json:"id"`
	ChainID        uuid.UUID  `json:"chainId"`
	SequenceNumber int        `json:"sequenceNumber"`
	ParentCallID   *uuid.UUID `json:"parentCallId,omitempty"`

	// Participants
	CallerAgentID *uuid.UUID `json:"callerAgentId,omitempty"`
	CalleeAgentID *uuid.UUID `json:"calleeAgentId,omitempty"`
	SkillID       string     `json:"skillId,omitempty"`
	TaskID        *uuid.UUID `json:"taskId,omitempty"`

	// Chain origin
	IsHumanInitiated     bool       `json:"isHumanInitiated"`
	OriginUserID         *uuid.UUID `json:"originUserId,omitempty"`
	OriginOrganizationID *uuid.UUID `json:"originOrganizationId,omitempty"`

	// Timing
	CallStartedAt time.Time  `json:"callStartedAt"`
	CallEndedAt   *time.Time `json:"callEndedAt,omitempty"`
	DurationMs    int        `json:"durationMs,omitempty"`

	// Status
	Status       A2ACallChainStatus `json:"status"`
	ErrorMessage string             `json:"errorMessage,omitempty"`

	// Data flow
	DataInputSizeBytes  int  `json:"dataInputSizeBytes,omitempty"`
	DataOutputSizeBytes int  `json:"dataOutputSizeBytes,omitempty"`
	PIIDetected         bool `json:"piiDetected"`

	CreatedAt time.Time `json:"createdAt"`
}

// A2ADelegation represents a delegation authorization between agents
type A2ADelegation struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`

	// Delegation parties
	DelegatorAgentID uuid.UUID `json:"delegatorAgentId"`
	DelegateAgentID  uuid.UUID `json:"delegateAgentId"`

	// Scope
	SkillIDs      []string `json:"skillIds,omitempty"`
	MaxChainDepth int      `json:"maxChainDepth"`

	// Trust attenuation
	TrustAttenuationFactor float64 `json:"trustAttenuationFactor"` // Multiplier per hop (0.0-1.0, default 0.8)
	MinDelegatedTrustScore float64 `json:"minDelegatedTrustScore"` // Floor below which chain is invalid (0.0-1.0, default 0.3)

	// Permissions
	CanDelegateFurther bool `json:"canDelegateFurther"`
	RequiresConsent    bool `json:"requiresConsent"`

	// Validity
	IsActive   bool       `json:"isActive"`
	ValidFrom  time.Time  `json:"validFrom"`
	ValidUntil *time.Time `json:"validUntil,omitempty"`

	// Audit
	CreatedBy *uuid.UUID `json:"createdBy,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// A2ADependencyType represents the type of dependency between agents
type A2ADependencyType string

const (
	A2ADependencySkill         A2ADependencyType = "SKILL"
	A2ADependencyData          A2ADependencyType = "DATA"
	A2ADependencyOrchestration A2ADependencyType = "ORCHESTRATION"
)

// A2AAgentDependency represents a dependency between agents
type A2AAgentDependency struct {
	ID               uuid.UUID `json:"id"`
	AgentID          uuid.UUID `json:"agentId"`
	DependsOnAgentID uuid.UUID `json:"dependsOnAgentId"`

	// Dependency details
	DependencyType A2ADependencyType `json:"dependencyType"`
	SkillIDs       []string          `json:"skillIds,omitempty"`
	IsCritical     bool              `json:"isCritical"`

	// Statistics
	CallCount    int `json:"callCount"`
	SuccessCount int `json:"successCount"`
	FailureCount int `json:"failureCount"`
	AvgLatencyMs int `json:"avgLatencyMs,omitempty"`

	FirstSeenAt time.Time `json:"firstSeenAt"`
	LastSeenAt  time.Time `json:"lastSeenAt"`
}

// A2AChainSummary provides an overview of an agent call chain
type A2AChainSummary struct {
	ChainID          uuid.UUID          `json:"chainId"`
	TotalCalls       int                `json:"totalCalls"`
	MaxDepth         int                `json:"maxDepth"`
	UniqueAgents     int                `json:"uniqueAgents"`
	AgentIDs         []uuid.UUID        `json:"agentIds"`
	IsHumanInitiated bool               `json:"isHumanInitiated"`
	OriginUserID     *uuid.UUID         `json:"originUserId,omitempty"`
	StartedAt        time.Time          `json:"startedAt"`
	CompletedAt      *time.Time         `json:"completedAt,omitempty"`
	Status           A2ACallChainStatus `json:"status"`
	HasPII           bool               `json:"hasPii"`
}

// A2ACallChainRepository defines operations for call chain tracking
type A2ACallChainRepository interface {
	Create(ctx context.Context, call *A2ACallChain) error
	GetByID(ctx context.Context, id uuid.UUID) (*A2ACallChain, error)
	GetByChainID(ctx context.Context, chainID uuid.UUID) ([]*A2ACallChain, error)
	GetByCallerAgent(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*A2ACallChain, error)
	GetByCalleeAgent(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*A2ACallChain, error)
	GetByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*A2ACallChain, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status A2ACallChainStatus, errorMessage string) error
	CompleteCall(ctx context.Context, id uuid.UUID, outputSizeBytes int) error
	GetChainSummary(ctx context.Context, chainID uuid.UUID) (*A2AChainSummary, error)
}

// A2ADelegationRepository defines operations for delegation management
type A2ADelegationRepository interface {
	Create(ctx context.Context, delegation *A2ADelegation) error
	GetByID(ctx context.Context, id uuid.UUID) (*A2ADelegation, error)
	GetByDelegator(ctx context.Context, delegatorID uuid.UUID) ([]*A2ADelegation, error)
	GetByDelegate(ctx context.Context, delegateID uuid.UUID) ([]*A2ADelegation, error)
	GetByOrganization(ctx context.Context, orgID uuid.UUID) ([]*A2ADelegation, error)
	IsAuthorizedDelegate(ctx context.Context, delegatorID, delegateID uuid.UUID, skillID string) (bool, error)
	Update(ctx context.Context, delegation *A2ADelegation) error
	Revoke(ctx context.Context, id uuid.UUID) error
}

// A2AAgentDependencyRepository defines operations for dependency tracking
type A2AAgentDependencyRepository interface {
	Upsert(ctx context.Context, dep *A2AAgentDependency) error
	GetByAgentID(ctx context.Context, agentID uuid.UUID) ([]*A2AAgentDependency, error)
	GetDependents(ctx context.Context, dependsOnAgentID uuid.UUID) ([]*A2AAgentDependency, error)
	GetCriticalDependencies(ctx context.Context, agentID uuid.UUID) ([]*A2AAgentDependency, error)
	IncrementStats(ctx context.Context, agentID, dependsOnID uuid.UUID, success bool, latencyMs int) error
}
