package main

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/siem/hectest"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
)

// An alert written through the alert repository the alert service holds,
// by any service, reaches the SIEM as aim.alert.created once
// observeSecurityDecisions has run.
func TestObserveSecurityDecisions_ForwardsInsertedAlerts(t *testing.T) {
	hecToken := uuid.NewString()
	receiver := hectest.NewReceiver(hecToken)
	t.Cleanup(receiver.Close)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	forwarder, err := startSIEMForwarding(repository.NewAuditLogRepository(db), siemEnv(map[string]string{
		"SIEM_SPLUNK_HEC_URL":   receiver.URL(),
		"SIEM_SPLUNK_HEC_TOKEN": hecToken,
	}))
	require.NoError(t, err)

	alerts := repository.NewAlertRepository(db)
	authService := application.NewAuthService(siemNoUsers{}, nil, nil, nil, nil, nil)
	services := &Services{
		Alert: application.NewAlertService(alerts, nil, nil),
		FGA:   application.NewFGAEngine(nil, application.NewAgentService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil), nil),
	}
	h := &Handlers{
		Auth: handlers.NewAuthHandler(authService, nil, nil, nil, nil),
		PublicRegistration: handlers.NewPublicRegistrationHandler(
			application.NewRegistrationService(siemNoRegistrations{}, siemNoUsers{}, nil, nil, nil), authService, nil, nil),
	}
	observeSecurityDecisions(forwarder, services, h)

	mock.ExpectExec(`INSERT INTO alerts`).WillReturnResult(sqlmock.NewResult(0, 1))
	alert := &domain.Alert{
		OrganizationID: uuid.New(),
		AlertType:      domain.AlertSecurityBreach,
		Severity:       domain.AlertSeverityWarning,
		Title:          "Unauthorized Action Detected: agent",
		ResourceType:   "agent",
		ResourceID:     uuid.New(),
		CreatedAt:      time.Date(2026, 10, 16, 18, 4, 5, 0, time.UTC),
	}
	require.NoError(t, alerts.Create(alert))

	flushSIEMForwarding(forwarder)

	assert.Empty(t, receiver.Refusals())
	accepted := receiver.Accepted()
	require.Len(t, accepted, 1)
	var event map[string]interface{}
	require.NoError(t, accepted[0].Decode(&event))
	assert.Equal(t, "aim.alert.created", event["eventType"])
	assert.Equal(t, "medium", event["severity"])
	assert.Equal(t, alert.ResourceID.String(), event["agentId"])
	assert.Equal(t, "2026-10-16T18:04:05Z", event["timestamp"])
	assert.Equal(t, map[string]interface{}{
		"alertId":        alert.ID.String(),
		"organizationId": alert.OrganizationID.String(),
		"alertType":      "security_breach",
		"resourceType":   "agent",
		"resourceId":     alert.ResourceID.String(),
	}, event["details"])
	assert.NotContains(t, string(accepted[0].Event), alert.Title)
	require.NoError(t, mock.ExpectationsWereMet())
}
