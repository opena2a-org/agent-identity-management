package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

func siemEnv(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

// The server forwards each audit log entry it writes to the Splunk HTTP Event
// Collector that SIEM_SPLUNK_HEC_URL and SIEM_SPLUNK_HEC_TOKEN name.
func TestStartSIEMForwarding_SendsWrittenAuditLogEntriesToSplunk(t *testing.T) {
	type received struct {
		authorization string
		events        []map[string]interface{}
	}
	requests := make(chan received, 8)
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		req := received{authorization: r.Header.Get("Authorization")}
		dec := json.NewDecoder(bytes.NewReader(body))
		for {
			var hecEvent map[string]interface{}
			if dec.Decode(&hecEvent) != nil {
				break
			}
			req.events = append(req.events, hecEvent)
		}
		requests <- req
		_, _ = w.Write([]byte(`{"text":"Success","code":0}`))
	}))
	t.Cleanup(collector.Close)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	auditLogs := repository.NewAuditLogRepository(db)

	forwarder, err := startSIEMForwarding(auditLogs, siemEnv(map[string]string{
		"SIEM_SPLUNK_HEC_URL":   collector.URL + "/services/collector/event",
		"SIEM_SPLUNK_HEC_TOKEN": "fake-hec-token",
		"SIEM_MIN_SEVERITY":     "medium",
	}))
	require.NoError(t, err)
	require.NotNil(t, forwarder)

	agentID := uuid.New()
	mock.ExpectExec(`INSERT INTO audit_logs`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO audit_logs`).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, auditLogs.Create(&domain.AuditLog{
		OrganizationID: uuid.New(), Action: domain.AuditActionLogin, ResourceType: "user", ResourceID: uuid.New(),
	}))
	refused := &domain.AuditLog{
		OrganizationID: uuid.New(), AgentID: &agentID, Action: domain.AuditActionVerificationRefused,
		ResourceType: "agent", ResourceID: agentID,
	}
	require.NoError(t, auditLogs.Create(refused))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, forwarder.Close(ctx))
	require.NoError(t, mock.ExpectationsWereMet())

	require.Len(t, requests, 1)
	req := <-requests
	assert.Equal(t, "Splunk fake-hec-token", req.authorization)
	require.Len(t, req.events, 1, "the login is below SIEM_MIN_SEVERITY=medium")
	event := req.events[0]["event"].(map[string]interface{})
	assert.Equal(t, "aim.audit.verification_refused", event["eventType"])
	assert.Equal(t, "medium", event["severity"])
	assert.Equal(t, agentID.String(), event["agentId"])
	assert.Equal(t, refused.ID.String(), event["details"].(map[string]interface{})["auditLogId"])
	assert.Equal(t, int64(1), forwarder.Stats().Sent)
}

func TestStartSIEMForwarding_NothingConfigured(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	auditLogs := repository.NewAuditLogRepository(db)

	forwarder, err := startSIEMForwarding(auditLogs, siemEnv(nil))
	require.NoError(t, err)
	assert.Nil(t, forwarder)

	mock.ExpectExec(`INSERT INTO audit_logs`).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, auditLogs.Create(&domain.AuditLog{OrganizationID: uuid.New(), Action: domain.AuditActionLogin}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStartSIEMForwarding_RefusesAPartConfiguration(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	forwarder, err := startSIEMForwarding(repository.NewAuditLogRepository(db), siemEnv(map[string]string{
		"SIEM_SPLUNK_HEC_URL": "https://splunk.example.com:8088/services/collector/event",
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SIEM_SPLUNK_HEC_TOKEN")
	assert.Nil(t, forwarder)
}
