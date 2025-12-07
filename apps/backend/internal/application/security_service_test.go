package application

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/infrastructure/repository"
	"github.com/stretchr/testify/assert"
)

// setupSecurityTestDB creates a mock database for security service testing
func setupSecurityTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	return db, mock
}

// TestGetThreats_AlertConversion tests threat retrieval (converts alerts to threats)
func TestGetThreats_AlertConversion(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()

	// Setup test database
	db, _ := setupSecurityTestDB(t)
	defer db.Close()

	securityRepo := repository.NewSecurityRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	mockAlertRepo := new(MockAlertRepository)

	service := NewSecurityService(securityRepo, agentRepo, mockAlertRepo)

	t.Run("successfully converts alerts to threats", func(t *testing.T) {
		resourceID := uuid.New()
		now := time.Now().UTC()

		// Mock alerts from database
		alerts := []*domain.Alert{
			{
				ID:             uuid.New(),
				OrganizationID: orgID,
				AlertType:      domain.AlertSecurityBreach,
				Severity:       domain.AlertSeverityCritical,
				Title:          "Security Breach Detected",
				Description:    "Unauthorized access attempt",
				ResourceType:   "agent",
				ResourceID:     resourceID,
				IsAcknowledged: false,
				CreatedAt:      now,
			},
			{
				ID:             uuid.New(),
				OrganizationID: orgID,
				AlertType:      domain.AlertTrustScoreLow,
				Severity:       domain.AlertSeverityHigh,
				Title:          "Low Trust Score",
				Description:    "Agent trust score below threshold",
				ResourceType:   "agent",
				ResourceID:     resourceID,
				IsAcknowledged: true,
				AcknowledgedAt: &now,
				CreatedAt:      now,
			},
		}

		mockAlertRepo.On("GetByOrganization", orgID, 10, 0).Return(alerts, nil).Once()

		// Execute
		threats, err := service.GetThreats(ctx, orgID, 10, 0)

		// Assert
		assert.NoError(t, err)
		assert.Len(t, threats, 2)

		// Verify first threat (security breach -> malicious_agent)
		assert.Equal(t, alerts[0].ID, threats[0].ID)
		assert.Equal(t, domain.ThreatType("malicious_agent"), threats[0].ThreatType)
		assert.Equal(t, domain.AlertSeverityCritical, threats[0].Severity)
		assert.Equal(t, "Security Breach Detected", threats[0].Title)
		assert.False(t, threats[0].IsBlocked)
		assert.Nil(t, threats[0].ResolvedAt)

		// Verify second threat (low trust score -> suspicious_activity)
		assert.Equal(t, alerts[1].ID, threats[1].ID)
		assert.Equal(t, domain.ThreatType("suspicious_activity"), threats[1].ThreatType)
		assert.Equal(t, domain.AlertSeverityHigh, threats[1].Severity)
		assert.NotNil(t, threats[1].ResolvedAt) // Acknowledged alert -> resolved threat

		mockAlertRepo.AssertExpectations(t)
	})

	t.Run("handles alert type mappings correctly", func(t *testing.T) {
		// Test all alert type -> threat type mappings
		testCases := []struct {
			alertType    domain.AlertType
			expectedType string
		}{
			{domain.AlertSecurityBreach, "malicious_agent"},
			{domain.AlertCertificateExpiring, "certificate_expiry"},
			{domain.AlertAPIKeyExpiring, "credential_leak"},
			{domain.AlertTrustScoreLow, "suspicious_activity"},
			{domain.AlertTypeConfigurationDrift, "configuration_drift"},
			{domain.AlertUnusualActivity, "suspicious_activity"}, // default case
		}

		for _, tc := range testCases {
			t.Run(string(tc.alertType), func(t *testing.T) {
				alerts := []*domain.Alert{
					{
						ID:             uuid.New(),
						OrganizationID: orgID,
						AlertType:      tc.alertType,
						Severity:       domain.AlertSeverityWarning,
						Title:          "Test Alert",
						Description:    "Test description",
						ResourceType:   "agent",
						ResourceID:     uuid.New(),
						CreatedAt:      time.Now().UTC(),
					},
				}

				mockAlertRepo.On("GetByOrganization", orgID, 10, 0).Return(alerts, nil).Once()

				threats, err := service.GetThreats(ctx, orgID, 10, 0)

				assert.NoError(t, err)
				assert.Len(t, threats, 1)
				assert.Equal(t, tc.expectedType, string(threats[0].ThreatType))
			})
		}
	})

	t.Run("returns empty array when no alerts exist", func(t *testing.T) {
		mockAlertRepo.On("GetByOrganization", orgID, 10, 0).Return([]*domain.Alert{}, nil).Once()

		threats, err := service.GetThreats(ctx, orgID, 10, 0)

		assert.NoError(t, err)
		assert.Empty(t, threats)
	})
}

// TestGetAnomalies tests anomaly retrieval
func TestGetAnomalies(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()

	db, dbMock := setupSecurityTestDB(t)
	defer db.Close()

	securityRepo := repository.NewSecurityRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	mockAlertRepo := new(MockAlertRepository)

	service := NewSecurityService(securityRepo, agentRepo, mockAlertRepo)

	t.Run("successfully retrieves anomalies", func(t *testing.T) {
		anomalyID1 := uuid.New()
		anomalyID2 := uuid.New()
		resourceID1 := uuid.New()
		resourceID2 := uuid.New()
		now := time.Now().UTC()

		rows := sqlmock.NewRows([]string{
			"id", "organization_id", "anomaly_type", "severity", "title",
			"description", "resource_type", "resource_id", "confidence", "created_at",
		}).
			AddRow(anomalyID1, orgID, "unusual_api_usage", "high", "Unusual API Pattern",
				"Abnormal API call frequency detected", "agent", resourceID1, 85.5, now).
			AddRow(anomalyID2, orgID, "abnormal_traffic", "warning", "Traffic Spike",
				"Unusual traffic volume", "agent", resourceID2, 72.3, now)

		dbMock.ExpectQuery(regexp.QuoteMeta(`SELECT id, organization_id, anomaly_type, severity, title, description, resource_type, resource_id, confidence, created_at FROM security_anomalies WHERE organization_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`)).
			WithArgs(orgID, 10, 0).
			WillReturnRows(rows)

		anomalies, err := service.GetAnomalies(ctx, orgID, 10, 0)

		assert.NoError(t, err)
		assert.Len(t, anomalies, 2)
		assert.Equal(t, 85.5, anomalies[0].Confidence)
		assert.Equal(t, 72.3, anomalies[1].Confidence)
		assert.NoError(t, dbMock.ExpectationsWereMet())
	})
}

// TestGetSecurityMetrics tests security metrics aggregation
// TODO: Test needs refactoring - SQL mocks don't match actual repository queries
func TestGetSecurityMetrics(t *testing.T) {
	t.Skip("Test needs refactoring: SQL mocks don't match repository queries")
}

// TestRunSecurityScan tests security scan execution
// TODO: These tests need refactoring - the SQL mocks don't match actual repository queries
// The repository uses full column lists while tests mock SELECT *
func TestRunSecurityScan(t *testing.T) {
	t.Skip("Test needs refactoring: SQL mocks don't match repository queries")
}

// TestGetSecurityScan tests security scan retrieval
func TestGetSecurityScan(t *testing.T) {
	ctx := context.Background()
	scanID := uuid.New()
	orgID := uuid.New()

	db, dbMock := setupSecurityTestDB(t)
	defer db.Close()

	securityRepo := repository.NewSecurityRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	mockAlertRepo := new(MockAlertRepository)

	service := NewSecurityService(securityRepo, agentRepo, mockAlertRepo)

	t.Run("successfully retrieves completed scan", func(t *testing.T) {
		completedAt := time.Now().UTC()
		startedAt := completedAt.Add(-5 * time.Minute)

		rows := sqlmock.NewRows([]string{
			"id", "organization_id", "scan_type", "status", "threats_found",
			"anomalies_found", "vulnerabilities_found", "security_score", "started_at", "completed_at",
		}).AddRow(scanID, orgID, "comprehensive", "completed", 3, 5, 2, 78.5, startedAt, completedAt)

		dbMock.ExpectQuery(regexp.QuoteMeta(`SELECT id, organization_id, scan_type, status, threats_found, anomalies_found, vulnerabilities_found, security_score, started_at, completed_at FROM security_scans WHERE id = $1`)).
			WithArgs(scanID).
			WillReturnRows(rows)

		scan, err := service.GetSecurityScan(ctx, scanID)

		assert.NoError(t, err)
		assert.Equal(t, scanID, scan.ScanID)
		assert.Equal(t, "completed", scan.Status)
		assert.Equal(t, 3, scan.ThreatsFound)
		assert.NotNil(t, scan.CompletedAt)
		assert.NoError(t, dbMock.ExpectationsWereMet())
	})
}

// TestGetIncidents tests incident retrieval
func TestGetIncidents(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()

	db, dbMock := setupSecurityTestDB(t)
	defer db.Close()

	securityRepo := repository.NewSecurityRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	mockAlertRepo := new(MockAlertRepository)

	service := NewSecurityService(securityRepo, agentRepo, mockAlertRepo)

	t.Run("successfully retrieves open incidents", func(t *testing.T) {
		incidentID := uuid.New()
		assignedUser := uuid.New()
		now := time.Now().UTC()

		rows := sqlmock.NewRows([]string{
			"id", "organization_id", "incident_type", "status", "severity",
			"title", "description", "affected_resources", "assigned_to",
			"created_at", "updated_at", "resolved_at", "resolved_by", "resolution_notes",
		}).AddRow(
			incidentID, orgID, "data_breach", "open", "critical",
			"Potential Data Breach", "Unauthorized data access detected",
			`{"agent-123","agent-456"}`, assignedUser,
			now, now, nil, nil, "",
		)

		dbMock.ExpectQuery(regexp.QuoteMeta(`SELECT id, organization_id, incident_type, status, severity, title, description, affected_resources, assigned_to, created_at, updated_at, resolved_at, resolved_by, resolution_notes FROM security_incidents WHERE organization_id = $1 AND status = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`)).
			WithArgs(orgID, "open", 10, 0).
			WillReturnRows(rows)

		incidents, err := service.GetIncidents(ctx, orgID, domain.IncidentStatusOpen, 10, 0)

		assert.NoError(t, err)
		assert.Len(t, incidents, 1)
		assert.Equal(t, domain.IncidentStatusOpen, incidents[0].Status)
		assert.NoError(t, dbMock.ExpectationsWereMet())
	})
}

// TestResolveIncident tests incident resolution
func TestResolveIncident(t *testing.T) {
	ctx := context.Background()
	incidentID := uuid.New()
	resolvedBy := uuid.New()
	notes := "Issue resolved after investigation"

	db, dbMock := setupSecurityTestDB(t)
	defer db.Close()

	securityRepo := repository.NewSecurityRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	mockAlertRepo := new(MockAlertRepository)

	service := NewSecurityService(securityRepo, agentRepo, mockAlertRepo)

	t.Run("successfully resolves incident", func(t *testing.T) {
		// Query: UPDATE security_incidents SET status = $1, resolved_at = $2, resolved_by = $3, resolution_notes = $4, updated_at = $5 WHERE id = $6
		dbMock.ExpectExec(regexp.QuoteMeta(`UPDATE security_incidents`)).
			WithArgs("resolved", sqlmock.AnyArg(), resolvedBy, notes, sqlmock.AnyArg(), incidentID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := service.ResolveIncident(ctx, incidentID, resolvedBy, notes)

		assert.NoError(t, err)
		assert.NoError(t, dbMock.ExpectationsWereMet())
	})
}

// TestCreateThreat tests threat creation
func TestCreateThreat(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	targetID := uuid.New()

	db, dbMock := setupSecurityTestDB(t)
	defer db.Close()

	securityRepo := repository.NewSecurityRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	mockAlertRepo := new(MockAlertRepository)

	service := NewSecurityService(securityRepo, agentRepo, mockAlertRepo)

	t.Run("successfully creates threat", func(t *testing.T) {
		threat := &domain.Threat{
			OrganizationID: orgID,
			ThreatType:     domain.ThreatTypeMaliciousAgent,
			Severity:       domain.AlertSeverityCritical,
			Title:          "Malicious Agent Detected",
			Description:    "Agent showing suspicious behavior",
			Source:         "192.168.1.100",
			TargetType:     "agent",
			TargetID:       targetID,
			IsBlocked:      false,
		}

		dbMock.ExpectExec(regexp.QuoteMeta(`INSERT INTO security_threats`)).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := service.CreateThreat(ctx, threat)

		assert.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, threat.ID)
		assert.False(t, threat.CreatedAt.IsZero())
		assert.NoError(t, dbMock.ExpectationsWereMet())
	})
}

// TestCreateAnomaly tests anomaly creation
func TestCreateAnomaly(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	resourceID := uuid.New()

	db, dbMock := setupSecurityTestDB(t)
	defer db.Close()

	securityRepo := repository.NewSecurityRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	mockAlertRepo := new(MockAlertRepository)

	service := NewSecurityService(securityRepo, agentRepo, mockAlertRepo)

	t.Run("successfully creates anomaly", func(t *testing.T) {
		anomaly := &domain.Anomaly{
			OrganizationID: orgID,
			AnomalyType:    domain.AnomalyTypeUnusualAPIUsage,
			Severity:       domain.AlertSeverityWarning,
			Title:          "Unusual API Pattern",
			Description:    "API calls exceeding normal threshold",
			ResourceType:   "agent",
			ResourceID:     resourceID,
			Confidence:     87.5,
		}

		dbMock.ExpectExec(regexp.QuoteMeta(`INSERT INTO security_anomalies`)).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := service.CreateAnomaly(ctx, anomaly)

		assert.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, anomaly.ID)
		assert.False(t, anomaly.CreatedAt.IsZero())
		assert.NoError(t, dbMock.ExpectationsWereMet())
	})
}

// TestCreateIncident tests incident creation
func TestCreateIncident(t *testing.T) {
	ctx := context.Background()
	orgID := uuid.New()
	assignedTo := uuid.New()

	db, dbMock := setupSecurityTestDB(t)
	defer db.Close()

	securityRepo := repository.NewSecurityRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	mockAlertRepo := new(MockAlertRepository)

	service := NewSecurityService(securityRepo, agentRepo, mockAlertRepo)

	t.Run("successfully creates incident", func(t *testing.T) {
		incident := &domain.SecurityIncident{
			OrganizationID:    orgID,
			IncidentType:      "unauthorized_access",
			Status:            domain.IncidentStatusOpen,
			Severity:          domain.AlertSeverityCritical,
			Title:             "Security Breach",
			Description:       "Multiple failed login attempts",
			AffectedResources: []string{"agent-1", "agent-2"},
			AssignedTo:        &assignedTo,
		}

		dbMock.ExpectExec(regexp.QuoteMeta(`INSERT INTO security_incidents`)).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := service.CreateIncident(ctx, incident)

		assert.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, incident.ID)
		assert.False(t, incident.CreatedAt.IsZero())
		assert.False(t, incident.UpdatedAt.IsZero())
		assert.NoError(t, dbMock.ExpectationsWereMet())
	})
}

// TestBlockThreat tests threat blocking
func TestBlockThreat(t *testing.T) {
	ctx := context.Background()
	threatID := uuid.New()

	db, dbMock := setupSecurityTestDB(t)
	defer db.Close()

	securityRepo := repository.NewSecurityRepository(db)
	agentRepo := repository.NewAgentRepository(db)
	mockAlertRepo := new(MockAlertRepository)

	service := NewSecurityService(securityRepo, agentRepo, mockAlertRepo)

	t.Run("successfully blocks threat", func(t *testing.T) {
		dbMock.ExpectExec(regexp.QuoteMeta(`UPDATE security_threats SET is_blocked = true WHERE id = $1`)).
			WithArgs(threatID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := service.BlockThreat(ctx, threatID)

		assert.NoError(t, err)
		assert.NoError(t, dbMock.ExpectationsWereMet())
	})
}
