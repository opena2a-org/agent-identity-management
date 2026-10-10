package application

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// securityAlertsFactor scores the security-alerts factor for an agent whose
// unacknowledged alerts and capability violations are the given ones.
func securityAlertsFactor(agentID uuid.UUID, alerts []*domain.Alert, violations []*domain.CapabilityViolation) float64 {
	alertRepo := new(TrustCalcMockAlertRepository)
	alertRepo.On("GetUnacknowledgedByResourceID", agentID).Return(alerts, nil)
	capabilityRepo := new(MockCapabilityRepository)
	capabilityRepo.On("GetViolationsByAgentID", agentID, 500, 0).Return(violations, len(violations), nil)
	c := &TrustCalculator{alertRepo: alertRepo, capabilityRepo: capabilityRepo}
	return c.calculateSecurityAlerts(&domain.Agent{ID: agentID})
}

// An alert that counts repeats deducts what the same repeats deducted as
// separate alerts, each raised once, so counting repeats on one alert leaves
// the trust score for the same requests unchanged. The alert raised for a
// denial because the agent has no granted capabilities deducts nothing: the
// denial is deducted through its violation record, as it was before that
// alert was raised.
func TestTrustCalculator_SecurityAlerts_CountedRepeatsDeductAsSeparateAlerts(t *testing.T) {
	agentID := uuid.New()
	alert := func(severity domain.AlertSeverity, occurrences int, dedupeKey string) *domain.Alert {
		return &domain.Alert{
			ID:              uuid.New(),
			Severity:        severity,
			ResourceID:      agentID,
			CreatedAt:       time.Now(),
			OccurrenceCount: occurrences,
			DedupeKey:       dedupeKey,
		}
	}
	separate := func(severity domain.AlertSeverity, n int, dedupeKey string) []*domain.Alert {
		alerts := make([]*domain.Alert, 0, n)
		for i := 0; i < n; i++ {
			alerts = append(alerts, alert(severity, 1, dedupeKey))
		}
		return alerts
	}
	blockedViolations := func(n int) []*domain.CapabilityViolation {
		violations := make([]*domain.CapabilityViolation, 0, n)
		for i := 0; i < n; i++ {
			violations = append(violations, &domain.CapabilityViolation{
				AgentID:   agentID,
				Severity:  domain.ViolationSeverityHigh,
				IsBlocked: true,
				CreatedAt: time.Now(),
			})
		}
		return violations
	}
	violationKey := CapabilityViolationDedupeKey(agentID, "db:write", "orders")
	unregisteredKey := UnregisteredCapabilityDedupeKey(agentID, "file:write", "/etc/passwd")
	noGrantsKey := NoGrantedCapabilitiesDedupeKey(agentID, "file:write", "/etc/passwd")

	cases := []struct {
		name       string
		separate   []*domain.Alert // one alert per occurrence
		counted    []*domain.Alert // the same occurrences counted on one alert
		violations []*domain.CapabilityViolation
		want       float64
	}{
		{name: "critical, 1 occurrence", separate: separate(domain.AlertSeverityCritical, 1, ""), counted: []*domain.Alert{alert(domain.AlertSeverityCritical, 1, "")}, want: 0.75},
		{name: "critical, 3 occurrences", separate: separate(domain.AlertSeverityCritical, 3, ""), counted: []*domain.Alert{alert(domain.AlertSeverityCritical, 3, "")}, want: 0.25},
		{name: "high, 1 occurrence", separate: separate(domain.AlertSeverityHigh, 1, violationKey), counted: []*domain.Alert{alert(domain.AlertSeverityHigh, 1, violationKey)}, want: 0.90},
		{name: "high, 3 occurrences", separate: separate(domain.AlertSeverityHigh, 3, violationKey), counted: []*domain.Alert{alert(domain.AlertSeverityHigh, 3, violationKey)}, want: 0.70},
		{name: "warning, 1 occurrence", separate: separate(domain.AlertSeverityWarning, 1, unregisteredKey), counted: []*domain.Alert{alert(domain.AlertSeverityWarning, 1, unregisteredKey)}, want: 0.95},
		{name: "warning, 3 occurrences", separate: separate(domain.AlertSeverityWarning, 3, unregisteredKey), counted: []*domain.Alert{alert(domain.AlertSeverityWarning, 3, unregisteredKey)}, want: 0.85},
		{name: "info, 3 occurrences", separate: separate(domain.AlertSeverityInfo, 3, ""), counted: []*domain.Alert{alert(domain.AlertSeverityInfo, 3, "")}, want: 1.0},
		{name: "no count recorded is 1 occurrence", separate: separate(domain.AlertSeverityHigh, 1, ""), counted: []*domain.Alert{alert(domain.AlertSeverityHigh, 0, "")}, want: 0.90},
		{name: "no-grants denial, 1 occurrence", separate: nil, counted: []*domain.Alert{alert(domain.AlertSeverityWarning, 1, noGrantsKey)}, violations: blockedViolations(1), want: 0.95},
		{name: "no-grants denial, 3 occurrences", separate: nil, counted: []*domain.Alert{alert(domain.AlertSeverityCritical, 3, noGrantsKey)}, violations: blockedViolations(3), want: 0.85},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := securityAlertsFactor(agentID, tc.separate, tc.violations)
			after := securityAlertsFactor(agentID, tc.counted, tc.violations)
			assert.InDelta(t, tc.want, before, 1e-9)
			assert.Equal(t, before, after, "counting the occurrences on one alert changed the factor")
		})
	}
}

// The alert raised for a denial because the agent has no granted capabilities
// does not enter the score, so it does not raise the score's confidence
// either: an agent whose only alert is that one has the confidence of an
// agent with no alerts.
func TestTrustCalculator_Confidence_NoGrantsAlertAddsNoConfidence(t *testing.T) {
	agent := &domain.Agent{ID: uuid.New(), Status: domain.AgentStatusVerified, CreatedAt: time.Now()}
	confidence := func(alerts []*domain.Alert) float64 {
		alertRepo := new(TrustCalcMockAlertRepository)
		alertRepo.On("GetByResourceID", agent.ID, 100, 0).Return(alerts, nil)
		c := &TrustCalculator{alertRepo: alertRepo}
		return c.calculateConfidence(agent, &domain.TrustScoreFactors{}, map[string]string{})
	}

	none := confidence([]*domain.Alert{})
	noGrantsOnly := confidence([]*domain.Alert{{
		ID:              uuid.New(),
		Severity:        domain.AlertSeverityWarning,
		OccurrenceCount: 3,
		DedupeKey:       NoGrantedCapabilitiesDedupeKey(agent.ID, "file:write", "/etc/passwd"),
	}})
	violation := confidence([]*domain.Alert{{
		ID:              uuid.New(),
		Severity:        domain.AlertSeverityHigh,
		OccurrenceCount: 1,
		DedupeKey:       CapabilityViolationDedupeKey(agent.ID, "file:write", "/etc/passwd"),
	}})

	assert.Equal(t, none, noGrantsOnly)
	assert.Greater(t, violation, none)
}
