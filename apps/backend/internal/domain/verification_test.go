package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestVerificationStatusConstants(t *testing.T) {
	assert.Equal(t, VerificationStatus("pending"), VerificationStatusPending)
	assert.Equal(t, VerificationStatus("approved"), VerificationStatusApproved)
	assert.Equal(t, VerificationStatus("denied"), VerificationStatusDenied)
}

func TestVerificationStruct(t *testing.T) {
	now := time.Now()
	verificationID := uuid.New()
	orgID := uuid.New()
	agentID := uuid.New()

	verification := Verification{
		ID:             verificationID,
		OrganizationID: orgID,
		AgentID:        agentID,
		AgentName:      "test-agent",
		Action:         "file:read",
		Status:         VerificationStatusApproved,
		DurationMs:     50,
		Metadata: map[string]interface{}{
			"resource": "/data/file.txt",
			"ip":       "192.168.1.1",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	assert.Equal(t, verificationID, verification.ID)
	assert.Equal(t, orgID, verification.OrganizationID)
	assert.Equal(t, agentID, verification.AgentID)
	assert.Equal(t, "test-agent", verification.AgentName)
	assert.Equal(t, "file:read", verification.Action)
	assert.Equal(t, VerificationStatusApproved, verification.Status)
	assert.Equal(t, 50, verification.DurationMs)
	assert.NotNil(t, verification.Metadata)
	assert.Equal(t, "/data/file.txt", verification.Metadata["resource"])
}

func TestVerificationStatusStringConversion(t *testing.T) {
	status := VerificationStatusApproved
	str := string(status)
	assert.Equal(t, "approved", str)

	// Convert back
	converted := VerificationStatus(str)
	assert.Equal(t, VerificationStatusApproved, converted)
}

func TestVerificationWithMinimalFields(t *testing.T) {
	verification := Verification{
		ID:      uuid.New(),
		AgentID: uuid.New(),
		Action:  "api:call",
		Status:  VerificationStatusPending,
	}

	assert.NotEqual(t, uuid.Nil, verification.ID)
	assert.NotEqual(t, uuid.Nil, verification.AgentID)
	assert.Equal(t, VerificationStatusPending, verification.Status)
	assert.Nil(t, verification.Metadata)
}

func TestVerificationWithDeniedStatus(t *testing.T) {
	verification := Verification{
		ID:         uuid.New(),
		AgentID:    uuid.New(),
		Action:     "system:admin",
		Status:     VerificationStatusDenied,
		DurationMs: 5,
		Metadata: map[string]interface{}{
			"reason": "insufficient_trust_score",
		},
	}

	assert.Equal(t, VerificationStatusDenied, verification.Status)
	assert.Equal(t, "insufficient_trust_score", verification.Metadata["reason"])
}
