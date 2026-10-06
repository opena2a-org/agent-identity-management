package application

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// complianceActorRows holds one audit row per actor shape: a user's act, an
// agent's act, an agent's act stored with its owner's user id (as the
// verification of an agent's action stores it), the system's act, and a row
// whose ids are nil UUIDs.
type complianceActorRows struct {
	orgID, userID, agentID uuid.UUID
	userEmail              string
	users                  []*domain.User
	logs                   []*domain.AuditLog
}

func newComplianceActorRows(now time.Time) complianceActorRows {
	r := complianceActorRows{
		orgID:     uuid.New(),
		userID:    uuid.New(),
		agentID:   uuid.New(),
		userEmail: "owner@example.test",
	}
	nilID := uuid.Nil
	r.users = []*domain.User{{ID: r.userID, OrganizationID: r.orgID, Email: r.userEmail, Role: domain.RoleAdmin}}
	row := func(action domain.AuditAction, user, agent *uuid.UUID, ip string) *domain.AuditLog {
		return &domain.AuditLog{
			ID:             uuid.New(),
			OrganizationID: r.orgID,
			UserID:         user,
			AgentID:        agent,
			Action:         action,
			ResourceType:   "agent",
			ResourceID:     uuid.New(),
			IPAddress:      ip,
			Timestamp:      now.Add(-time.Hour),
		}
	}
	r.logs = []*domain.AuditLog{
		row(domain.AuditActionView, &r.userID, nil, "192.0.2.10"),
		row(domain.AuditActionVerify, nil, &r.agentID, ""),
		row(domain.AuditActionExport, &r.userID, &r.agentID, ""),
		row(domain.AuditActionCalculate, nil, nil, ""),
		row(domain.AuditActionCheck, &nilID, &nilID, ""),
	}
	return r
}

// The report's recent actions name each row's actor by the audit row's own
// rule, and its top actors count an agent's act for the agent.
func TestComplianceAuditActivityNamesTheActor(t *testing.T) {
	now := time.Now()
	r := newComplianceActorRows(now)
	s := &ComplianceService{}

	report := s.buildAuditActivityReport(r.logs, r.users, now, now.Add(-24*time.Hour), now.AddDate(0, 0, -7), now.AddDate(0, 0, -30))

	agentLabel := "agent:" + r.agentID.String()[:8]
	want := []struct{ userID, email, actorType string }{
		{r.userID.String(), r.userEmail, "user"},
		{r.agentID.String(), agentLabel, "agent"},
		{r.agentID.String(), agentLabel, "agent"},
		{"", "", "system"},
		{"", "", "system"},
	}
	require.Len(t, report.RecentActions, len(want))
	for i, w := range want {
		got := report.RecentActions[i]
		assert.Equal(t, w.userID, got.UserID, "row %d userId", i)
		assert.Equal(t, w.email, got.UserEmail, "row %d userEmail", i)
		assert.Equal(t, w.actorType, got.ActorType, "row %d actorType", i)
	}

	raw, err := json.Marshal(report.RecentActions)
	require.NoError(t, err)
	var entries []map[string]any
	require.NoError(t, json.Unmarshal(raw, &entries))
	assert.Equal(t, "user", entries[0]["actorType"])
	assert.Equal(t, "192.0.2.10", entries[0]["ipAddress"])
	for i := 1; i < len(entries); i++ {
		assert.NotContains(t, entries[i], "ipAddress", "row %d recorded no address", i)
	}
	for _, i := range []int{3, 4} {
		assert.NotContains(t, entries[i], "userId", "row %d is the system's act", i)
		assert.Equal(t, "system", entries[i]["actorType"])
	}

	assert.Equal(t, 2, report.UniqueUsers)
	counts := map[string]domain.UserActivitySummary{}
	for _, u := range report.TopUsers {
		counts[u.UserID] = u
	}
	require.Len(t, counts, 2)
	assert.Equal(t, 1, counts[r.userID.String()].ActionCount)
	assert.Equal(t, "user", counts[r.userID.String()].ActorType)
	assert.Equal(t, 2, counts[r.agentID.String()].ActionCount)
	assert.Equal(t, "agent", counts[r.agentID.String()].ActorType)
	assert.Equal(t, agentLabel, counts[r.agentID.String()].UserEmail)
}

// csvSection returns the rows under a titled section of the compliance CSV,
// header first, up to the blank line that ends the section.
func csvSection(t *testing.T, doc, title string) [][]string {
	t.Helper()
	lines := strings.Split(doc, "\n")
	for i, line := range lines {
		if line != title {
			continue
		}
		var rows [][]string
		for _, l := range lines[i+1:] {
			if l == "" {
				return rows
			}
			rows = append(rows, strings.Split(l, ","))
		}
		return rows
	}
	t.Fatalf("section %q not found", title)
	return nil
}

// The compliance CSV export carries each recent action's and each top
// actor's actor type, and leaves the id cell empty for the system's acts.
func TestComplianceCSVExportNamesTheActor(t *testing.T) {
	now := time.Now()
	r := newComplianceActorRows(now)

	agentRepo := new(SharedMockAgentRepository)
	userRepo := new(SharedMockUserRepository)
	auditRepo := new(SharedMockAuditLogRepository)
	agentRepo.On("GetByOrganization", r.orgID).Return([]*domain.Agent{}, nil)
	userRepo.On("GetByOrganization", r.orgID).Return(r.users, nil)
	auditRepo.On("GetByOrganization", r.orgID, mock.Anything, mock.Anything).Return(r.logs, nil)
	s := NewComplianceService(auditRepo, agentRepo, userRepo, nil)

	doc, err := s.ExportToCSV(context.Background(), r.orgID, "soc2")
	require.NoError(t, err)

	recent := csvSection(t, doc, "Recent Actions (Last 50)")
	require.Len(t, recent, 1+len(r.logs))
	assert.Equal(t, []string{"ID", "Action", "Resource Type", "Resource ID", "User ID", "User Email", "IP Address", "Timestamp", "Actor Type"}, recent[0])
	agentLabel := "agent:" + r.agentID.String()[:8]
	want := [][3]string{
		{r.userID.String(), r.userEmail, "user"},
		{r.agentID.String(), agentLabel, "agent"},
		{r.agentID.String(), agentLabel, "agent"},
		{"", "", "system"},
		{"", "", "system"},
	}
	for i, w := range want {
		row := recent[i+1]
		require.Len(t, row, 9, "row %d", i)
		assert.Equal(t, r.logs[i].ID.String(), row[0])
		assert.Equal(t, w[0], row[4], "row %d User ID", i)
		assert.Equal(t, w[1], row[5], "row %d User Email", i)
		assert.Equal(t, w[2], row[8], "row %d Actor Type", i)
	}

	top := csvSection(t, doc, "Top Users by Activity")
	assert.Equal(t, []string{"User ID", "User Email", "Action Count", "Last Action", "Last Action Time", "Actor Type"}, top[0])
	byID := map[string][]string{}
	for _, row := range top[1:] {
		byID[row[0]] = row
	}
	require.Len(t, byID, 2)
	assert.Equal(t, "user", byID[r.userID.String()][5])
	assert.Equal(t, "2", byID[r.agentID.String()][2])
	assert.Equal(t, "agent", byID[r.agentID.String()][5])
}

type capturedEvidenceRepo struct {
	domain.ComplianceEvidenceRepository
	created []*domain.ComplianceEvidence
}

func (r *capturedEvidenceRepo) Create(e *domain.ComplianceEvidence) error {
	r.created = append(r.created, e)
	return nil
}

// PHI access evidence names who read or exported, and a row without a user
// id no longer fails the collection.
func TestCollectPHIAccessEvidenceNamesTheActor(t *testing.T) {
	now := time.Now()
	r := newComplianceActorRows(now)
	nilID := uuid.Nil
	logs := []*domain.AuditLog{
		{ID: uuid.New(), UserID: &r.userID, Action: domain.AuditActionView, ResourceType: "agent", Timestamp: now},
		{ID: uuid.New(), UserID: &r.userID, AgentID: &r.agentID, Action: domain.AuditActionExport, ResourceType: "agent", Timestamp: now},
		{ID: uuid.New(), Action: domain.AuditActionView, ResourceType: "agent", Timestamp: now},
		{ID: uuid.New(), UserID: &nilID, Action: domain.AuditActionExport, ResourceType: "agent", Timestamp: now},
	}
	auditRepo := new(SharedMockAuditLogRepository)
	auditRepo.On("GetByOrganization", r.orgID, 500, 0).Return(logs, nil)
	evidenceRepo := &capturedEvidenceRepo{}
	s := NewComplianceServiceFull(auditRepo, nil, nil, nil, evidenceRepo, nil, nil)

	evidence, err := s.CollectEvidence(context.Background(), r.orgID, domain.ComplianceFramework("hipaa"), "phi_access_logging", r.userID)
	require.NoError(t, err)
	require.Len(t, evidenceRepo.created, 1)

	accessLogs, ok := evidence.Data["accessLogs"].([]map[string]interface{})
	require.True(t, ok, "accessLogs is %T", evidence.Data["accessLogs"])
	require.Len(t, accessLogs, len(logs))
	want := []struct {
		userID    string
		actorType string
	}{
		{r.userID.String(), "user"},
		{r.agentID.String(), "agent"},
		{"", "system"},
		{"", "system"},
	}
	for i, w := range want {
		entry := accessLogs[i]
		assert.Equal(t, w.actorType, entry["actorType"], "entry %d actorType", i)
		if w.userID == "" {
			assert.NotContains(t, entry, "userId", "entry %d is the system's act", i)
		} else {
			assert.Equal(t, w.userID, entry["userId"], "entry %d userId", i)
		}
	}
}

// The audit log CSV export leaves the user_id cell empty for a row without a
// user id, rather than failing, and names each row's actor type. Like the
// admin route's CSV, the cell holds the row's own user id.
func TestExportAuditLogCSVWithoutUserID(t *testing.T) {
	now := time.Now()
	r := newComplianceActorRows(now)
	auditRepo := new(SharedMockAuditLogRepository)
	auditRepo.On("GetByOrganization", r.orgID, 10000, 0).Return(r.logs, nil)
	s := NewComplianceService(auditRepo, nil, nil, nil)

	doc, err := s.ExportAuditLog(context.Background(), r.orgID, now.Add(-2*time.Hour), now, "csv")
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSuffix(doc, "\n"), "\n")
	require.Len(t, lines, 1+len(r.logs))
	assert.Equal(t, "timestamp,user_id,action,resource_type,resource_id,ip_address,actor_type", lines[0])
	want := [][2]string{
		{r.userID.String(), "user"},
		{"", "agent"},
		{r.userID.String(), "agent"},
		{"", "system"},
		{uuid.Nil.String(), "system"},
	}
	for i, w := range want {
		cells := strings.Split(lines[i+1], ",")
		require.Len(t, cells, 7, "row %d", i)
		assert.Equal(t, w[0], cells[1], "row %d user_id", i)
		assert.Equal(t, w[1], cells[6], "row %d actor_type", i)
	}
}
