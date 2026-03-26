package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCapabilityTier_DefaultStandard(t *testing.T) {
	svc := NewPAMService(nil, nil)
	tier, err := svc.GetCapabilityTier(context.Background(), "unknown.capability")
	require.NoError(t, err)
	assert.Equal(t, "STANDARD", tier.Tier)
	assert.Equal(t, 14400, tier.MaxTTLSeconds)
	assert.False(t, tier.RequiresMFA)
}

func TestRequestApproval_RequiresJustification(t *testing.T) {
	svc := NewPAMService(nil, nil)
	req := &ApprovalRequest{
		AgentID:    uuid.New(),
		Capability: "infra.modify",
	}
	_, err := svc.RequestApproval(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "justification")
}

func TestRequestApproval_SetsDefaults(t *testing.T) {
	svc := NewPAMService(nil, nil)
	req := &ApprovalRequest{
		AgentID:       uuid.New(),
		Capability:    "infra.modify",
		Justification: "Production hotfix for incident INC-12345",
	}
	result, err := svc.RequestApproval(context.Background(), req)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, result.ID)
	assert.False(t, result.RequestedAt.IsZero())
	assert.False(t, result.ExpiresAt.IsZero())
	// 5 minute timeout
	assert.WithinDuration(t, result.RequestedAt.Add(5*time.Minute), result.ExpiresAt, time.Second)
}

func TestDecideApproval_InvalidDecision(t *testing.T) {
	svc := NewPAMService(nil, nil)
	err := svc.DecideApproval(context.Background(), uuid.New(), uuid.New(), "MAYBE", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "APPROVED or DENIED")
}

func TestDeclareEmergency_RequiresIncidentID(t *testing.T) {
	svc := NewPAMService(nil, nil)
	decl := &EmergencyDeclaration{
		DeclaredBy:    "admin@opena2a.org",
		Justification: "Production outage",
	}
	_, err := svc.DeclareEmergency(context.Background(), decl)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "incident ID")
}

func TestDeclareEmergency_HardCap60Min(t *testing.T) {
	svc := NewPAMService(nil, nil)
	now := time.Now().UTC()
	decl := &EmergencyDeclaration{
		IncidentID:     "INC-99999",
		DeclaredBy:     "admin@opena2a.org",
		Justification:  "Critical production incident",
		ExpiresAt:      now.Add(24 * time.Hour), // Try 24 hours
		AffectedAgents: []uuid.UUID{uuid.New()},
		GrantedCaps:    []string{"infra.modify", "db.delete"},
	}
	result, err := svc.DeclareEmergency(context.Background(), decl)
	require.NoError(t, err)
	assert.Equal(t, "ACTIVE", result.Status)
	// Should be capped at 60 minutes, not 24 hours
	maxExpiry := result.DeclaredAt.Add(60 * time.Minute)
	assert.WithinDuration(t, maxExpiry, result.ExpiresAt, time.Second)
}

func TestCreateCampaign_Default30Days(t *testing.T) {
	svc := NewPAMService(nil, nil)
	start := time.Now().UTC()
	campaign := &CertificationCampaign{
		Name:      "Q2 2026 Access Review",
		ScopeType: "ALL",
		StartsAt:  start,
		CreatedBy: "admin",
	}
	result, err := svc.CreateCampaign(context.Background(), campaign)
	require.NoError(t, err)
	assert.Equal(t, "PENDING", result.Status)
	assert.WithinDuration(t, start.Add(30*24*time.Hour), result.ClosesAt, time.Second)
}

func TestRecordCertificationDecision_RequiresDecision(t *testing.T) {
	svc := NewPAMService(nil, nil)
	decision := &CertificationDecision{
		CampaignID: uuid.New(),
		AgentID:    uuid.New(),
		Capability: "db.read",
	}
	err := svc.RecordCertificationDecision(context.Background(), decision)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decision is required")
}

func TestPqFromUUIDs(t *testing.T) {
	assert.Equal(t, "{}", pq_from_uuids(nil))
	id := uuid.New()
	result := pq_from_uuids([]uuid.UUID{id})
	assert.Contains(t, result, id.String())
}
