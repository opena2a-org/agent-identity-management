package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// audit_logs.ip_address and audit_logs.user_agent are nullable. GET
// /agents/:id/activity reads an agent's rows through the real audit service
// and repository here, over sqlmock. The first row holds NULL in one or both
// of the columns, or an empty string in both; the second holds a value in
// both. The route answers 200 with both records; a NULL column leaves its
// member out of the first record, as an empty string does.
func TestAgentActivity_RowWithNullAddressOrUserAgentIsListed(t *testing.T) {
	const (
		address   = "203.0.113.9"
		userAgent = "example-sdk/1.0"
	)
	cases := []struct {
		name      string
		ipAddress interface{}
		userAgent interface{}
	}{
		{name: "ip_address and user_agent NULL", ipAddress: nil, userAgent: nil},
		{name: "ip_address NULL", ipAddress: nil, userAgent: userAgent},
		{name: "user_agent NULL", ipAddress: address, userAgent: nil},
		{name: "ip_address and user_agent empty", ipAddress: "", userAgent: ""},
	}
	for _, tc := range cases {
		for _, role := range []string{string(domain.RoleAdmin), string(domain.RoleViewer)} {
			t.Run(tc.name+"/role="+role, func(t *testing.T) {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				t.Cleanup(func() { _ = db.Close() })

				orgID, agentID := uuid.New(), uuid.New()
				firstID, secondID := uuid.New(), uuid.New()
				newest := time.Date(2026, 10, 20, 9, 30, 0, 0, time.UTC)
				rows := sqlmock.NewRows([]string{
					"id", "organization_id", "user_id", "agent_id", "action",
					"resource_type", "resource_id", "ip_address", "user_agent",
					"metadata", "timestamp", "agent_name", "user_name",
				}).
					AddRow(firstID.String(), orgID.String(), nil, agentID.String(), "file:read",
						"agent_action", agentID.String(), tc.ipAddress, tc.userAgent, []byte(`{}`),
						newest, "billing-agent", "").
					AddRow(secondID.String(), orgID.String(), nil, agentID.String(), "file:read",
						"agent_action", agentID.String(), address, userAgent, []byte(`{}`),
						newest.Add(-time.Minute), "billing-agent", "")
				mock.ExpectQuery(`FROM audit_logs al`).WithArgs(agentID, orgID, 50, 0).WillReturnRows(rows)

				agentSvc := &MockAgentServiceImpl{
					GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
						return &domain.Agent{ID: agentID, OrganizationID: orgID, Name: "billing-agent"}, nil
					},
				}
				auditSvc := application.NewAuditService(repository.NewAuditLogRepository(db))
				h := NewAgentHandlerWithInterfaces(agentSvc, nil, auditSvc, nil, nil, nil, nil, nil, nil, nil, nil)

				raw := serveAuditRoute(t, "/agents/"+agentID.String()+"/activity", "/agents/:id/activity", role, orgID, h.GetAgentActivity)
				require.NoError(t, mock.ExpectationsWereMet())

				activities := decodeActivities(t, raw)
				require.Len(t, activities, 2)
				assert.JSONEq(t, `"`+firstID.String()+`"`, string(activities[0]["id"]))
				assert.JSONEq(t, `"`+secondID.String()+`"`, string(activities[1]["id"]))

				if role != string(domain.RoleAdmin) {
					for _, record := range activities {
						assert.NotContains(t, record, "ipAddress")
						assert.NotContains(t, record, "userAgent")
					}
					return
				}
				if wantAddress, _ := tc.ipAddress.(string); wantAddress == "" {
					assert.NotContains(t, activities[0], "ipAddress", "a NULL or empty ip_address must leave the member out")
				} else {
					assert.JSONEq(t, `"`+wantAddress+`"`, string(activities[0]["ipAddress"]))
				}
				if wantUserAgent, _ := tc.userAgent.(string); wantUserAgent == "" {
					assert.NotContains(t, activities[0], "userAgent", "a NULL or empty user_agent must leave the member out")
				} else {
					assert.JSONEq(t, `"`+wantUserAgent+`"`, string(activities[0]["userAgent"]))
				}
				assert.JSONEq(t, `"`+address+`"`, string(activities[1]["ipAddress"]))
				assert.JSONEq(t, `"`+userAgent+`"`, string(activities[1]["userAgent"]))
			})
		}
	}
}
