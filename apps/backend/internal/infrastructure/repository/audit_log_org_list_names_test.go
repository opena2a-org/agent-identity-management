package repository

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The organization read backs GET /api/v1/admin/audit-logs, which the
// dashboard's audit page lists with an actor per record. The per-agent and
// per-resource reads already join the user and agent names; the organization
// read returned bare IDs, so a record with a user had no name to show.

var orgAuditLogsWithNames = `^` + regexp.QuoteMeta(`SELECT
			al.id, al.organization_id, al.user_id, al.agent_id, al.action,
			al.resource_type, al.resource_id, al.ip_address, al.user_agent,
			al.metadata, al.timestamp,
			COALESCE(a.name, '') as agent_name,
			COALESCE(u.name, '') as user_name
		FROM audit_logs al
		LEFT JOIN agents a ON al.agent_id = a.id
		LEFT JOIN users u ON al.user_id = u.id
		WHERE al.organization_id = $1
		ORDER BY al.timestamp DESC
		LIMIT $2 OFFSET $3`) + `$`

func TestAuditLogGetByOrganization_ReturnsActorNames(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := NewAuditLogRepository(db)

	orgID := uuid.New()
	userID := uuid.New()
	agentID := uuid.New()
	newest := time.Date(2026, 10, 1, 13, 18, 21, 0, time.UTC)

	rows := sqlmock.NewRows([]string{
		"id", "organization_id", "user_id", "agent_id", "action",
		"resource_type", "resource_id", "ip_address", "user_agent",
		"metadata", "timestamp", "agent_name", "user_name",
	}).
		AddRow(uuid.NewString(), orgID.String(), userID.String(), nil, "update",
			"agent", uuid.NewString(), "", "", []byte(`{}`), newest, "", "Example User").
		AddRow(uuid.NewString(), orgID.String(), nil, agentID.String(), "attest",
			"mcp_server", uuid.NewString(), "", "", []byte(`{}`), newest.Add(-time.Minute), "example-agent", "").
		AddRow(uuid.NewString(), orgID.String(), nil, nil, "calculate",
			"trust_score", uuid.NewString(), "", "", []byte(`{}`), newest.Add(-2*time.Minute), "", "")
	mock.ExpectQuery(orgAuditLogsWithNames).WithArgs(orgID, 50, 0).WillReturnRows(rows)

	logs, err := repo.GetByOrganization(orgID, 50, 0)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.Len(t, logs, 3)

	require.NotNil(t, logs[0].UserID)
	assert.Equal(t, userID, *logs[0].UserID)
	assert.Equal(t, "Example User", logs[0].UserName)
	assert.Nil(t, logs[0].AgentID)

	assert.Nil(t, logs[1].UserID)
	require.NotNil(t, logs[1].AgentID)
	assert.Equal(t, agentID, *logs[1].AgentID)
	assert.Equal(t, "example-agent", logs[1].AgentName)

	assert.Nil(t, logs[2].UserID)
	assert.Nil(t, logs[2].AgentID)
	assert.Empty(t, logs[2].UserName)
	assert.Empty(t, logs[2].AgentName)
}
