package handlers

import (
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// An MCP server's timeline names an audit row's actor by the row's own rule:
// an agent's act stored with its owner's user id is the agent's.
func TestAuditTimelineActor(t *testing.T) {
	user, agent := uuid.New(), uuid.New()
	nilID := uuid.Nil
	cases := []struct {
		name      string
		row       domain.AuditLog
		actorType string
		actorID   string
		actorName string
	}{
		{"user id only", domain.AuditLog{UserID: &user, UserName: "Owner"}, "user", user.String(), "Owner"},
		{"user id without a name", domain.AuditLog{UserID: &user}, "user", user.String(), "User"},
		{"agent id only", domain.AuditLog{AgentID: &agent, AgentName: "billing-agent"}, "agent", agent.String(), "billing-agent"},
		{"agent id and its owner's user id", domain.AuditLog{UserID: &user, UserName: "Owner", AgentID: &agent, AgentName: "billing-agent"}, "agent", agent.String(), "billing-agent"},
		{"agent id without a name", domain.AuditLog{UserID: &user, AgentID: &agent}, "agent", agent.String(), "Agent"},
		{"neither id", domain.AuditLog{}, "system", "", "System"},
		{"nil uuids count as absent", domain.AuditLog{UserID: &nilID, AgentID: &nilID}, "system", "", "System"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actorType, actorID, actorName := auditTimelineActor(&tc.row)
			if actorType != tc.actorType {
				t.Fatalf("actorType = %q, want %q", actorType, tc.actorType)
			}
			if actorType != string(tc.row.ActorType()) {
				t.Fatalf("actorType = %q, the row's own actorType is %q", actorType, tc.row.ActorType())
			}
			gotID := ""
			if actorID != nil {
				gotID = *actorID
			}
			if gotID != tc.actorID {
				t.Fatalf("actorId = %q, want %q", gotID, tc.actorID)
			}
			if actorName != tc.actorName {
				t.Fatalf("actorName = %q, want %q", actorName, tc.actorName)
			}
		})
	}
}
