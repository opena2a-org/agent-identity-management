package application

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================
// Domain Type Tests - A2AAgentCard
// ========================================

func TestA2AAgentCard_Fields(t *testing.T) {
	agentID := uuid.New()
	now := time.Now().UTC()
	expiresAt := now.Add(24 * time.Hour)

	cardData := []byte(`{"name":"Test Agent","version":"1.0.0"}`)
	cardHash := sha256.Sum256(cardData)

	card := domain.A2AAgentCard{
		ID:                   uuid.New(),
		AgentID:              agentID,
		CardURL:              "https://agent.example.com/.well-known/agent.json",
		CardData:             cardData,
		CardHash:             hex.EncodeToString(cardHash[:]),
		ProtocolVersion:      "1.0.0",
		AttestationSignature: "base64-attestation",
		AttestationIssuedAt:  &now,
		AttestationExpiresAt: &expiresAt,
		IsValid:              true,
		LastFetchedAt:        &now,
		FetchCount:           1,
	}

	assert.Equal(t, agentID, card.AgentID)
	assert.Equal(t, "https://agent.example.com/.well-known/agent.json", card.CardURL)
	assert.NotEmpty(t, card.CardData)
	assert.NotEmpty(t, card.CardHash)
	assert.Equal(t, "1.0.0", card.ProtocolVersion)
	assert.True(t, card.IsValid)
	assert.NotNil(t, card.AttestationExpiresAt)
	assert.True(t, card.AttestationExpiresAt.After(now))
}

func TestA2AAgentCardParsed_WithAIMExtension(t *testing.T) {
	agentID := uuid.New()
	now := time.Now().UTC()
	expiresAt := now.Add(24 * time.Hour)

	parsed := domain.A2AAgentCardParsed{
		Name:        "Test Agent",
		Description: "A test agent for A2A protocol",
		URL:         "https://agent.example.com/.well-known/agent.json",
		Version:     "1.0.0",
		Skills: []domain.A2ASkillDefinition{
			{
				ID:          "skill-1",
				Name:        "Data Processing",
				Description: "Process data from various sources",
				Tags:        []string{"data", "processing"},
				InputModes:  []string{"text"},
				OutputModes: []string{"text", "json"},
			},
		},
		AIM: &domain.A2AAIMExtension{
			AgentID:      agentID,
			PublicKey:    "base64-public-key",
			TrustScore:   0.85,
			Capabilities: []string{"read", "write"},
			Attestation: &domain.A2AAttestation{
				Signature: "base64-signature",
				IssuedAt:  now,
				ExpiresAt: expiresAt,
			},
			Behavior: &domain.A2ABehavior{
				TotalTasksCompleted: 100,
				SuccessRate:         0.95,
				AvgResponseTimeMs:   500,
				LastActive:          now,
			},
		},
	}

	assert.Equal(t, "Test Agent", parsed.Name)
	assert.Len(t, parsed.Skills, 1)
	assert.Equal(t, "Data Processing", parsed.Skills[0].Name)
	assert.NotNil(t, parsed.AIM)
	assert.Equal(t, agentID, parsed.AIM.AgentID)
	assert.Equal(t, 0.85, parsed.AIM.TrustScore)
	assert.NotNil(t, parsed.AIM.Attestation)
	assert.NotNil(t, parsed.AIM.Behavior)
	assert.Equal(t, int64(100), parsed.AIM.Behavior.TotalTasksCompleted)
}

// ========================================
// Domain Type Tests - A2ARequestSignature
// ========================================

func TestA2ARequestSignature_Fields(t *testing.T) {
	agentID := uuid.New()
	now := time.Now().Unix()

	sig := domain.A2ARequestSignature{
		AgentID:   agentID,
		Timestamp: now,
		Nonce:     "unique-nonce-12345",
		Signature: "base64-signature",
	}

	assert.Equal(t, agentID, sig.AgentID)
	assert.Equal(t, now, sig.Timestamp)
	assert.Equal(t, "unique-nonce-12345", sig.Nonce)
	assert.Equal(t, "base64-signature", sig.Signature)
}

func TestA2ASignatureVerificationResult_Valid(t *testing.T) {
	agentID := uuid.New()

	result := domain.A2ASignatureVerificationResult{
		Valid:            true,
		AgentID:          agentID,
		AgentName:        "Test Agent",
		TrustScore:       0.85,
		Capabilities:     []string{"read", "write"},
		AttestationValid: true,
	}

	assert.True(t, result.Valid)
	assert.Equal(t, agentID, result.AgentID)
	assert.Equal(t, "Test Agent", result.AgentName)
	assert.Equal(t, 0.85, result.TrustScore)
	assert.True(t, result.AttestationValid)
	assert.Empty(t, result.Error)
}

func TestA2ASignatureVerificationResult_Invalid(t *testing.T) {
	result := domain.A2ASignatureVerificationResult{
		Valid: false,
		Error: "timestamp outside tolerance window",
	}

	assert.False(t, result.Valid)
	assert.Equal(t, "timestamp outside tolerance window", result.Error)
}

// ========================================
// Domain Type Tests - A2ATrustScore
// ========================================

func TestA2ATrustScore_Fields(t *testing.T) {
	agentID := uuid.New()
	now := time.Now().UTC()
	score := 0.85
	peerAvg := 0.80
	responseTime := 500

	trustScore := domain.A2ATrustScore{
		AgentID:          agentID,
		TasksCompleted:   95,
		TasksFailed:      5,
		PeerTrustAverage: &peerAvg,
		UniquePeersCount: 10,
		AvgResponseTimeMs: &responseTime,
		A2ATrustScore:    &score,
		ComputedAt:       &now,
	}

	assert.Equal(t, agentID, trustScore.AgentID)
	assert.Equal(t, 95, trustScore.TasksCompleted)
	assert.Equal(t, 5, trustScore.TasksFailed)
	assert.NotNil(t, trustScore.A2ATrustScore)
	assert.Equal(t, 0.85, *trustScore.A2ATrustScore)
	assert.Equal(t, 10, trustScore.UniquePeersCount)
}

func TestA2ATrustScore_SuccessRateCalculation(t *testing.T) {
	tests := []struct {
		name      string
		completed int
		failed    int
		expected  float64
	}{
		{
			name:      "high success rate",
			completed: 95,
			failed:    5,
			expected:  0.95,
		},
		{
			name:      "medium success rate",
			completed: 50,
			failed:    50,
			expected:  0.50,
		},
		{
			name:      "low success rate",
			completed: 10,
			failed:    90,
			expected:  0.10,
		},
		{
			name:      "perfect success rate",
			completed: 100,
			failed:    0,
			expected:  1.00,
		},
		{
			name:      "no tasks",
			completed: 0,
			failed:    0,
			expected:  0.00,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := domain.A2ATrustScore{
				TasksCompleted: tt.completed,
				TasksFailed:    tt.failed,
			}

			result := calculateSuccessRate(&score)
			assert.InDelta(t, tt.expected, result, 0.001)
		})
	}
}

// ========================================
// Domain Type Tests - A2APeerTrust
// ========================================

func TestA2APeerTrust_Fields(t *testing.T) {
	agentID := uuid.New()
	peerID := uuid.New()
	now := time.Now().UTC()
	trustScore := 0.88
	successRate := 0.92

	peerTrust := domain.A2APeerTrust{
		AgentID:                 agentID,
		PeerAgentID:             peerID,
		TasksInitiated:          50,
		TasksInitiatedCompleted: 46,
		TasksInitiatedFailed:    4,
		TasksReceived:           30,
		TasksReceivedCompleted:  28,
		TasksReceivedFailed:     2,
		SuccessRate:             &successRate,
		PeerTrustScore:          &trustScore,
		TrustDataPoints:         80,
		FirstInteractionAt:      &now,
		LastInteractionAt:       &now,
	}

	assert.Equal(t, agentID, peerTrust.AgentID)
	assert.Equal(t, peerID, peerTrust.PeerAgentID)
	assert.Equal(t, 50, peerTrust.TasksInitiated)
	assert.Equal(t, 30, peerTrust.TasksReceived)
	assert.NotNil(t, peerTrust.PeerTrustScore)
	assert.Equal(t, 0.88, *peerTrust.PeerTrustScore)
}

// ========================================
// Domain Type Tests - A2AConsentRecord
// ========================================

func TestA2AConsentRecord_Fields(t *testing.T) {
	grantorID := uuid.New()
	recipientID := uuid.New()
	orgID := uuid.New()
	now := time.Now().UTC()
	expiresAt := now.Add(365 * 24 * time.Hour)

	consent := domain.A2AConsentRecord{
		ID:               uuid.New(),
		UserID:           "user-123",
		OrganizationID:   &orgID,
		GrantorAgentID:   grantorID,
		RecipientAgentID: recipientID,
		Scope:            []string{"read:profile", "write:preferences"},
		Purpose:          "data-sharing",
		DataTypes:        []string{"profile", "preferences"},
		ExpiresAt:        &expiresAt,
		ConsentMethod:    domain.A2AConsentMethodExplicitClick,
		Evidence:         "click on consent button",
		UserAgent:        "Mozilla/5.0",
		CreatedAt:        now,
	}

	assert.Equal(t, "user-123", consent.UserID)
	assert.Equal(t, grantorID, consent.GrantorAgentID)
	assert.Equal(t, recipientID, consent.RecipientAgentID)
	assert.Len(t, consent.Scope, 2)
	assert.Equal(t, "data-sharing", consent.Purpose)
	assert.Equal(t, domain.A2AConsentMethodExplicitClick, consent.ConsentMethod)
	assert.NotNil(t, consent.ExpiresAt)
	assert.True(t, consent.ExpiresAt.After(now))
}

func TestA2AConsentRecord_IsActive(t *testing.T) {
	now := time.Now().UTC()
	past := now.Add(-24 * time.Hour)
	future := now.Add(24 * time.Hour)

	tests := []struct {
		name      string
		revokedAt *time.Time
		expiresAt *time.Time
		expected  bool
	}{
		{
			name:      "active with future expiry",
			revokedAt: nil,
			expiresAt: &future,
			expected:  true,
		},
		{
			name:      "active with no expiry",
			revokedAt: nil,
			expiresAt: nil,
			expected:  true,
		},
		{
			name:      "expired",
			revokedAt: nil,
			expiresAt: &past,
			expected:  false,
		},
		{
			name:      "revoked",
			revokedAt: &now,
			expiresAt: &future,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consent := domain.A2AConsentRecord{
				RevokedAt: tt.revokedAt,
				ExpiresAt: tt.expiresAt,
			}

			isActive := consent.RevokedAt == nil
			if isActive && consent.ExpiresAt != nil {
				isActive = consent.ExpiresAt.After(time.Now().UTC())
			}
			assert.Equal(t, tt.expected, isActive)
		})
	}
}

func TestA2AConsentRecord_Revoked(t *testing.T) {
	grantorID := uuid.New()
	recipientID := uuid.New()
	grantedAt := time.Now().UTC().Add(-30 * 24 * time.Hour)
	revokedAt := time.Now().UTC().Add(-1 * time.Hour)

	consent := domain.A2AConsentRecord{
		ID:               uuid.New(),
		UserID:           "user-123",
		GrantorAgentID:   grantorID,
		RecipientAgentID: recipientID,
		Scope:            []string{"read:profile"},
		Purpose:          "data-sharing",
		Revoked:          true,
		RevokedAt:        &revokedAt,
		RevokedReason:    "User requested revocation",
		CreatedAt:        grantedAt,
	}

	assert.True(t, consent.Revoked)
	assert.NotNil(t, consent.RevokedAt)
	assert.Equal(t, "User requested revocation", consent.RevokedReason)
	assert.True(t, consent.RevokedAt.After(consent.CreatedAt))
}

// ========================================
// Domain Type Tests - A2ATask
// ========================================

func TestA2ATask_Fields(t *testing.T) {
	clientID := uuid.New()
	remoteID := uuid.New()
	now := time.Now().UTC()
	completedAt := now.Add(5 * time.Second)
	clientTrust := 0.85
	remoteTrust := 0.90

	task := domain.A2ATask{
		ID:                       uuid.New(),
		ExternalTaskID:           "ext-task-123",
		ContextID:                "ctx-456",
		ClientAgentID:            clientID,
		RemoteAgentID:            remoteID,
		SkillID:                  "skill-data-process",
		State:                    domain.A2ATaskStateCompleted,
		ClientTrustScoreSnapshot: &clientTrust,
		RemoteTrustScoreSnapshot: &remoteTrust,
		StartedAt:                &now,
		CompletedAt:              &completedAt,
		DurationMs:               5000,
		CreatedAt:                now,
	}

	assert.Equal(t, clientID, task.ClientAgentID)
	assert.Equal(t, remoteID, task.RemoteAgentID)
	assert.Equal(t, "skill-data-process", task.SkillID)
	assert.Equal(t, domain.A2ATaskStateCompleted, task.State)
	assert.Equal(t, 0.85, *task.ClientTrustScoreSnapshot)
	assert.Equal(t, 5000, task.DurationMs)
}

func TestA2ATaskState_Values(t *testing.T) {
	states := []domain.A2ATaskState{
		domain.A2ATaskStateSubmitted,
		domain.A2ATaskStateWorking,
		domain.A2ATaskStateInputNeeded,
		domain.A2ATaskStateCompleted,
		domain.A2ATaskStateFailed,
		domain.A2ATaskStateCancelled,
	}

	for _, state := range states {
		assert.NotEmpty(t, string(state))
	}
}

// ========================================
// Domain Type Tests - A2APolicy
// ========================================

func TestA2APolicy_Fields(t *testing.T) {
	orgID := uuid.New()
	now := time.Now().UTC()

	policy := domain.A2APolicy{
		ID:              uuid.New(),
		OrganizationID:  &orgID,
		Name:            "Minimum Trust Policy",
		Description:     "Require minimum trust score for A2A communication",
		PolicyYAML:      "conditions:\n  min_trust_score: 0.7\naction: allow",
		AppliesToAgents: []string{"agent-1", "agent-2"},
		AppliesToSkills: []string{},
		Priority:        100,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	assert.Equal(t, orgID, *policy.OrganizationID)
	assert.Equal(t, "Minimum Trust Policy", policy.Name)
	assert.NotEmpty(t, policy.PolicyYAML)
	assert.True(t, policy.IsActive)
	assert.Len(t, policy.AppliesToAgents, 2)
	assert.Equal(t, 100, policy.Priority)
}

func TestA2APolicyDecision_Allow(t *testing.T) {
	now := time.Now().UTC()

	decision := domain.A2APolicyDecision{
		Decision:          "ALLOW",
		PoliciesEvaluated: []string{"trust-policy", "rate-limit-policy"},
		EvaluatedAt:       now,
	}

	assert.Equal(t, "ALLOW", decision.Decision)
	assert.Len(t, decision.PoliciesEvaluated, 2)
	assert.Empty(t, decision.Reason)
}

func TestA2APolicyDecision_Deny(t *testing.T) {
	now := time.Now().UTC()

	decision := domain.A2APolicyDecision{
		Decision:          "DENY",
		PoliciesEvaluated: []string{"trust-policy"},
		Reason:            "Trust score below threshold",
		EvaluatedAt:       now,
	}

	assert.Equal(t, "DENY", decision.Decision)
	assert.Equal(t, "Trust score below threshold", decision.Reason)
}

// ========================================
// Domain Type Tests - A2ASkill
// ========================================

func TestA2ASkill_Fields(t *testing.T) {
	agentID := uuid.New()
	now := time.Now().UTC()

	skill := domain.A2ASkill{
		ID:              uuid.New(),
		AgentID:         agentID,
		SkillID:         "summarization",
		Name:            "Text Summarization",
		Description:     "Summarize long text documents",
		Tags:            []string{"nlp", "text"},
		InputModes:      []string{"text"},
		OutputModes:     []string{"text", "json"},
		Examples:        []string{"Summarize this article..."},
		InvocationCount: 500,
		SuccessCount:    480,
		FailureCount:    20,
		AvgDurationMs:   1500,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	assert.Equal(t, agentID, skill.AgentID)
	assert.Equal(t, "summarization", skill.SkillID)
	assert.Equal(t, "Text Summarization", skill.Name)
	assert.Len(t, skill.Tags, 2)
	assert.Contains(t, skill.InputModes, "text")
	assert.Contains(t, skill.OutputModes, "json")
	assert.Equal(t, 500, skill.InvocationCount)
}

// ========================================
// Ed25519 Signature Tests
// ========================================

func TestEd25519SignatureCreation(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	// Create canonical request format
	method := "POST"
	path := "/api/v1/tasks/send"
	timestamp := time.Now().Unix()
	nonce := uuid.New().String()
	body := []byte(`{"task":"test"}`)

	bodyHash := sha256.Sum256(body)
	canonical := method + "\n" + path + "\n" + string(rune(timestamp)) + "\n" + nonce + "\n" + hex.EncodeToString(bodyHash[:])
	canonicalHash := sha256.Sum256([]byte(canonical))

	signature := ed25519.Sign(privateKey, canonicalHash[:])
	isValid := ed25519.Verify(publicKey, canonicalHash[:], signature)
	assert.True(t, isValid)

	sig := domain.A2ARequestSignature{
		AgentID:   uuid.New(),
		Timestamp: timestamp,
		Nonce:     nonce,
		Signature: base64.StdEncoding.EncodeToString(signature),
	}

	assert.NotEmpty(t, sig.Signature)
	assert.Equal(t, nonce, sig.Nonce)
}

func TestEd25519SignatureVerification(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	message := []byte("test message to sign")
	signature := ed25519.Sign(privateKey, message)

	// Valid signature
	isValid := ed25519.Verify(publicKey, message, signature)
	assert.True(t, isValid, "Valid signature should verify")

	// Modified message fails
	modifiedMessage := []byte("modified message")
	isValid = ed25519.Verify(publicKey, modifiedMessage, signature)
	assert.False(t, isValid, "Modified message should not verify")

	// Wrong public key fails
	wrongPublicKey, _, _ := ed25519.GenerateKey(nil)
	isValid = ed25519.Verify(wrongPublicKey, message, signature)
	assert.False(t, isValid, "Wrong public key should not verify")
}

// ========================================
// Anti-Replay Protection Tests
// ========================================

func TestNonceUniqueness(t *testing.T) {
	nonces := make(map[string]bool)
	count := 1000

	for i := 0; i < count; i++ {
		nonce := uuid.New().String()
		assert.False(t, nonces[nonce], "Nonce should be unique")
		nonces[nonce] = true
	}

	assert.Len(t, nonces, count)
}

func TestA2ARequestNonce_Fields(t *testing.T) {
	agentID := uuid.New()
	now := time.Now().UTC()
	expiresAt := now.Add(5 * time.Minute)

	nonce := domain.A2ARequestNonce{
		Nonce:       "unique-nonce-12345",
		AgentID:     agentID,
		UsedAt:      now,
		ExpiresAt:   expiresAt,
		RequestHash: "abcdef1234567890",
	}

	assert.Equal(t, "unique-nonce-12345", nonce.Nonce)
	assert.Equal(t, agentID, nonce.AgentID)
	assert.True(t, nonce.ExpiresAt.After(nonce.UsedAt))
}

func TestTimestampValidation(t *testing.T) {
	now := time.Now().Unix()
	tolerance := int64(DefaultSignatureTimestampToleranceSeconds)

	tests := []struct {
		name      string
		timestamp int64
		valid     bool
	}{
		{
			name:      "current timestamp",
			timestamp: now,
			valid:     true,
		},
		{
			name:      "4 minutes ago",
			timestamp: now - 240,
			valid:     true,
		},
		{
			name:      "6 minutes ago - expired",
			timestamp: now - 360,
			valid:     false,
		},
		{
			name:      "1 minute in future",
			timestamp: now + 60,
			valid:     true,
		},
		{
			name:      "10 minutes in future - too far",
			timestamp: now + 600,
			valid:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.timestamp >= now-tolerance && tt.timestamp <= now+tolerance
			assert.Equal(t, tt.valid, isValid)
		})
	}
}

// ========================================
// Request/Response Tests
// ========================================

func TestRegisterAgentCardRequest_Fields(t *testing.T) {
	agentID := uuid.New()

	req := RegisterAgentCardRequest{
		AgentID: agentID,
		CardURL: "https://agent.example.com/.well-known/agent.json",
	}

	assert.Equal(t, agentID, req.AgentID)
	assert.NotEmpty(t, req.CardURL)
	assert.Contains(t, req.CardURL, ".well-known/agent.json")
}

func TestRegisterAgentCardResponse_Fields(t *testing.T) {
	cardID := uuid.New()
	agentID := uuid.New()
	expiresAt := time.Now().UTC().Add(24 * time.Hour)

	resp := RegisterAgentCardResponse{
		CardID:             cardID,
		AgentID:            agentID,
		CardURL:            "https://agent.example.com/.well-known/agent.json",
		AttestationExpires: expiresAt,
		Skills:             []string{"skill-1", "skill-2"},
	}

	assert.Equal(t, cardID, resp.CardID)
	assert.Equal(t, agentID, resp.AgentID)
	assert.Len(t, resp.Skills, 2)
	assert.True(t, resp.AttestationExpires.After(time.Now().UTC()))
}

func TestVerifyA2ARequestRequest_Fields(t *testing.T) {
	agentID := uuid.New()
	now := time.Now().Unix()

	req := VerifyA2ARequestRequest{
		AgentID:     agentID,
		Timestamp:   now,
		Nonce:       "unique-nonce",
		Signature:   "base64-signature",
		RequestHash: "abcdef1234567890",
	}

	assert.Equal(t, agentID, req.AgentID)
	assert.Equal(t, now, req.Timestamp)
	assert.NotEmpty(t, req.Nonce)
	assert.NotEmpty(t, req.Signature)
}

func TestLogA2ATaskRequest_Fields(t *testing.T) {
	clientID := uuid.New()
	remoteID := uuid.New()

	req := LogA2ATaskRequest{
		ExternalTaskID: "ext-123",
		ContextID:      "ctx-456",
		ClientAgentID:  clientID,
		RemoteAgentID:  remoteID,
		SkillID:        "skill-process",
	}

	assert.Equal(t, "ext-123", req.ExternalTaskID)
	assert.Equal(t, clientID, req.ClientAgentID)
	assert.Equal(t, remoteID, req.RemoteAgentID)
	assert.Equal(t, "skill-process", req.SkillID)
}

func TestRecordConsentRequest_Fields(t *testing.T) {
	orgID := uuid.New()
	grantorID := uuid.New()
	recipientID := uuid.New()

	req := RecordConsentRequest{
		UserID:           "user-123",
		OrganizationID:   &orgID,
		GrantorAgentID:   grantorID,
		RecipientAgentID: recipientID,
		Scope:            []string{"read:profile", "write:preferences"},
		Purpose:          "data-sharing",
		DataTypes:        []string{"profile"},
		ExpiresInHours:   8760, // 1 year
		ConsentMethod:    "explicit",
		Evidence:         "clicked consent button",
		IPAddress:        "192.168.1.1",
		UserAgent:        "Mozilla/5.0",
	}

	assert.Equal(t, "user-123", req.UserID)
	assert.Equal(t, grantorID, req.GrantorAgentID)
	assert.Equal(t, recipientID, req.RecipientAgentID)
	assert.Len(t, req.Scope, 2)
	assert.Equal(t, "data-sharing", req.Purpose)
	assert.Equal(t, 8760, req.ExpiresInHours)
}

func TestEvaluateA2APolicyRequest_Fields(t *testing.T) {
	clientID := uuid.New()
	remoteID := uuid.New()
	orgID := uuid.New()

	req := EvaluateA2APolicyRequest{
		Event:          "task.create",
		ClientAgentID:  clientID,
		RemoteAgentID:  remoteID,
		SkillID:        "skill-process",
		OrganizationID: orgID,
	}

	assert.Equal(t, "task.create", req.Event)
	assert.Equal(t, clientID, req.ClientAgentID)
	assert.Equal(t, remoteID, req.RemoteAgentID)
	assert.Equal(t, orgID, req.OrganizationID)
}

// ========================================
// Configuration Tests
// ========================================

func TestDefaultConstants(t *testing.T) {
	assert.Equal(t, 0.7, DefaultMinTrustScoreForA2A)
	assert.Equal(t, 24, DefaultAttestationValidityHours)
	assert.Equal(t, 5, DefaultNonceExpiryMinutes)
	assert.Equal(t, 300, DefaultSignatureTimestampToleranceSeconds)
}

func TestGetMinTrustScoreForA2A(t *testing.T) {
	// Test default value
	score := getMinTrustScoreForA2A()
	assert.Equal(t, DefaultMinTrustScoreForA2A, score)
}

func TestGetAttestationValidityHours(t *testing.T) {
	hours := getAttestationValidityHours()
	assert.Equal(t, DefaultAttestationValidityHours, hours)
}

func TestGetNonceExpiryMinutes(t *testing.T) {
	minutes := getNonceExpiryMinutes()
	assert.Equal(t, DefaultNonceExpiryMinutes, minutes)
}

// ========================================
// Edge Cases and Error Handling
// ========================================

func TestEmptyAgentCard(t *testing.T) {
	card := domain.A2AAgentCard{}

	assert.Equal(t, uuid.Nil, card.ID)
	assert.Equal(t, uuid.Nil, card.AgentID)
	assert.Empty(t, card.CardURL)
	assert.Nil(t, card.CardData)
	assert.False(t, card.IsValid)
}

func TestZeroTrustScore(t *testing.T) {
	zero := 0.0
	score := domain.A2ATrustScore{
		A2ATrustScore: &zero,
	}

	assert.Equal(t, 0.0, *score.A2ATrustScore)
}

func TestMaxTrustScore(t *testing.T) {
	max := 1.0
	score := domain.A2ATrustScore{
		A2ATrustScore: &max,
	}

	assert.Equal(t, 1.0, *score.A2ATrustScore)
}

func TestEmptyConsent(t *testing.T) {
	consent := domain.A2AConsentRecord{}

	assert.Equal(t, uuid.Nil, consent.ID)
	assert.Empty(t, consent.UserID)
	assert.Nil(t, consent.Scope)
	assert.Nil(t, consent.RevokedAt)
}

// ========================================
// Helper Functions
// ========================================

func stringPtr(s string) *string {
	return &s
}
