package repository

import (
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// audit_logs.ip_address and audit_logs.user_agent are nullable columns (see
// migrations/001_initial_schema.sql). Each read of AuditLogRepository that
// returns them is given a row holding NULL in one or both, and must return the
// row with an empty IPAddress or UserAgent in place of the NULL.

var auditLogColumns = []string{
	"id", "organization_id", "user_id", "agent_id", "action",
	"resource_type", "resource_id", "ip_address", "user_agent",
	"metadata", "timestamp",
}

var auditLogColumnsWithNames = append(append([]string{}, auditLogColumns...), "agent_name", "user_name")

type auditLogRead struct {
	name      string
	withNames bool
	// ipFilter is the address the read selects by; its rows hold that
	// address, so only the user agent can be NULL.
	ipFilter string
	read     func(repo *AuditLogRepository, orgID, rowID uuid.UUID) ([]*domain.AuditLog, error)
}

const auditFilterAddress = "198.51.100.4"

var auditLogReads = []auditLogRead{
	{name: "GetByID", read: func(repo *AuditLogRepository, orgID, rowID uuid.UUID) ([]*domain.AuditLog, error) {
		entry, err := repo.GetByID(rowID)
		if entry == nil {
			return nil, err
		}
		return []*domain.AuditLog{entry}, err
	}},
	{name: "GetByOrganization", withNames: true, read: func(repo *AuditLogRepository, orgID, rowID uuid.UUID) ([]*domain.AuditLog, error) {
		return repo.GetByOrganization(orgID, 50, 0)
	}},
	{name: "GetByUser", read: func(repo *AuditLogRepository, orgID, rowID uuid.UUID) ([]*domain.AuditLog, error) {
		return repo.GetByUser(orgID, uuid.New(), 50, 0)
	}},
	{name: "GetByAgent", withNames: true, read: func(repo *AuditLogRepository, orgID, rowID uuid.UUID) ([]*domain.AuditLog, error) {
		return repo.GetByAgent(orgID, uuid.New(), 50, 0)
	}},
	{name: "GetByResource", withNames: true, read: func(repo *AuditLogRepository, orgID, rowID uuid.UUID) ([]*domain.AuditLog, error) {
		return repo.GetByResource(orgID, "agent", uuid.New())
	}},
	{name: "Search", read: func(repo *AuditLogRepository, orgID, rowID uuid.UUID) ([]*domain.AuditLog, error) {
		return repo.Search("update", 50, 0)
	}},
	{name: "GetRecentActionsByAgent", read: func(repo *AuditLogRepository, orgID, rowID uuid.UUID) ([]*domain.AuditLog, error) {
		return repo.GetRecentActionsByAgent(uuid.New(), 10)
	}},
	{name: "GetAgentActionsByIPAddress", ipFilter: auditFilterAddress, read: func(repo *AuditLogRepository, orgID, rowID uuid.UUID) ([]*domain.AuditLog, error) {
		return repo.GetAgentActionsByIPAddress(uuid.New(), auditFilterAddress, 10)
	}},
}

func TestAuditLogReads_ReturnARowWithNullAddressOrUserAgent(t *testing.T) {
	const userAgent = "example-sdk/1.0"
	cases := []struct {
		name      string
		ipAddress interface{}
		userAgent interface{}
	}{
		{name: "ip_address and user_agent NULL", ipAddress: nil, userAgent: nil},
		{name: "ip_address NULL", ipAddress: nil, userAgent: userAgent},
		{name: "user_agent NULL", ipAddress: "203.0.113.9", userAgent: nil},
	}
	for _, read := range auditLogReads {
		for _, tc := range cases {
			if read.ipFilter != "" && tc.ipAddress == nil {
				// A row this read selects holds the address it selects by.
				continue
			}
			t.Run(read.name+"/"+tc.name, func(t *testing.T) {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				t.Cleanup(func() { _ = db.Close() })
				repo := NewAuditLogRepository(db)

				orgID, rowID := uuid.New(), uuid.New()
				ipAddress := tc.ipAddress
				if read.ipFilter != "" {
					ipAddress = read.ipFilter
				}
				values := []driver.Value{
					rowID.String(), orgID.String(), nil, uuid.NewString(), "update",
					"agent", uuid.NewString(), ipAddress, tc.userAgent,
					[]byte(`{}`), time.Date(2026, 10, 20, 9, 30, 0, 0, time.UTC),
				}
				columns := auditLogColumns
				if read.withNames {
					columns = auditLogColumnsWithNames
					values = append(values, "billing-agent", "")
				}
				mock.ExpectQuery(`FROM audit_logs`).WillReturnRows(sqlmock.NewRows(columns).AddRow(values...))

				logs, err := read.read(repo, orgID, rowID)
				require.NoError(t, err)
				require.NoError(t, mock.ExpectationsWereMet())
				require.Len(t, logs, 1)
				assert.Equal(t, rowID, logs[0].ID)

				wantAddress, _ := ipAddress.(string)
				wantUserAgent, _ := tc.userAgent.(string)
				assert.Equal(t, wantAddress, logs[0].IPAddress)
				assert.Equal(t, wantUserAgent, logs[0].UserAgent)
			})
		}
	}
}
