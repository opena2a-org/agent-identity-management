package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

// An audit row names its actor by which id it carries. The verification of an
// agent's action stores the agent's owner as user_id beside the agent's id, so
// an agent id decides before a user id does.
func TestAuditLogActorType(t *testing.T) {
	user, agent := uuid.New(), uuid.New()
	nilID := uuid.Nil
	cases := []struct {
		name string
		row  AuditLog
		want AuditActorType
	}{
		{"user id only", AuditLog{UserID: &user}, AuditActorUser},
		{"agent id only", AuditLog{AgentID: &agent}, AuditActorAgent},
		{"agent id and its owner's user id", AuditLog{UserID: &user, AgentID: &agent}, AuditActorAgent},
		{"neither id", AuditLog{}, AuditActorSystem},
		{"nil uuids count as absent", AuditLog{UserID: &nilID, AgentID: &nilID}, AuditActorSystem},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.row.ActorType(); got != tc.want {
				t.Fatalf("ActorType() = %q, want %q", got, tc.want)
			}
		})
	}
}

func marshalAuditLog(t *testing.T, v any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	return out
}

// Every route that returns domain.AuditLog carries actorType, and leaves out
// ipAddress and userAgent when the row recorded none.
func TestAuditLogJSON(t *testing.T) {
	user, agent := uuid.New(), uuid.New()
	row := AuditLog{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AgentID:        &agent,
		UserID:         &user,
		Action:         AuditActionVerify,
		ResourceType:   "agent_action",
		ResourceID:     agent,
		Metadata:       map[string]interface{}{"k": "v"},
		Timestamp:      time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC),
	}

	t.Run("actorType is carried by value and by pointer", func(t *testing.T) {
		for name, v := range map[string]any{"value": row, "pointer": &row, "slice": []*AuditLog{&row}} {
			got := marshalAuditLog(t, map[string]any{"v": v})["v"]
			if list, ok := got.([]any); ok {
				got = list[0]
			}
			m, _ := got.(map[string]any)
			if m["actorType"] != "agent" {
				t.Fatalf("%s: actorType = %v, want agent", name, m["actorType"])
			}
		}
	})

	t.Run("ipAddress and userAgent are absent when not recorded", func(t *testing.T) {
		m := marshalAuditLog(t, row)
		for _, member := range []string{"ipAddress", "userAgent"} {
			if v, present := m[member]; present {
				t.Fatalf("%s is present as %v on a row that recorded none", member, v)
			}
		}
	})

	t.Run("ipAddress and userAgent are present when recorded", func(t *testing.T) {
		recorded := row
		recorded.IPAddress, recorded.UserAgent = "192.0.2.10", "aim-sdk/2.0"
		m := marshalAuditLog(t, recorded)
		if m["ipAddress"] != "192.0.2.10" || m["userAgent"] != "aim-sdk/2.0" {
			t.Fatalf("ipAddress = %v, userAgent = %v", m["ipAddress"], m["userAgent"])
		}
	})

	t.Run("the other members are unchanged", func(t *testing.T) {
		m := marshalAuditLog(t, row)
		want := map[string]any{
			"id":             row.ID.String(),
			"organizationId": row.OrganizationID.String(),
			"userId":         user.String(),
			"agentId":        agent.String(),
			"action":         string(AuditActionVerify),
			"resourceType":   "agent_action",
			"resourceId":     agent.String(),
			"timestamp":      "2026-10-06T12:00:00Z",
			"actorType":      "agent",
		}
		for k, v := range want {
			if m[k] != v {
				t.Fatalf("%s = %v, want %v", k, m[k], v)
			}
		}
		if md, _ := m["metadata"].(map[string]any); md["k"] != "v" {
			t.Fatalf("metadata = %v", m["metadata"])
		}
		if len(m) != len(want)+1 {
			t.Fatalf("members = %v, want exactly %d", m, len(want)+1)
		}
	})
}
