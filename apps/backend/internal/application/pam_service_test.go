package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newPAMServiceMock returns a PAM service on a sqlmock database whose clock
// reads now. The test fails if an expected statement is not run.
func newPAMServiceMock(t *testing.T, now time.Time) (*PAMService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	})
	svc := NewPAMService(db, nil)
	svc.now = func() time.Time { return now }
	return svc, mock
}

var pamTierRowColumns = []string{"capability", "tier", "tier_reason", "max_ttl_seconds", "requires_mfa", "requires_ticket", "renewal_allowed"}

func expectPAMTier(mock sqlmock.Sqlmock, capability, tier string, requiresTicket bool) {
	rows := sqlmock.NewRows(pamTierRowColumns)
	if tier != "" {
		rows.AddRow(capability, tier, "test tier", 900, true, requiresTicket, false)
	}
	mock.ExpectQuery(`FROM privileged_capability_registry WHERE capability = \$1`).
		WithArgs(capability).WillReturnRows(rows)
}

var pamApprovalRowColumns = []string{"id", "organization_id", "agent_id", "capability", "resource_path", "justification",
	"intent_class", "intent_confidence", "requested_by", "approver_id", "approver_decision", "approver_note",
	"ticket_id", "ticket_system", "requested_at", "expires_at", "decided_at"}

// pamApprovalRow is a pending approval of org, requested by requester at
// requestedAt.
func pamApprovalRow(id, org, agent, requester uuid.UUID, requestedAt time.Time, decision string) *sqlmock.Rows {
	var decidedAt interface{}
	if decision != "" {
		decidedAt = requestedAt.Add(time.Minute)
	}
	return sqlmock.NewRows(pamApprovalRowColumns).AddRow(id.String(), org.String(), agent.String(), "infra.modify", "", "hotfix",
		"", 0.0, requester.String(), nil, decision, "", "", "", requestedAt, requestedAt.Add(ApprovalDecisionWindow), decidedAt)
}

func expectPAMApproval(mock sqlmock.Sqlmock, id, org uuid.UUID, rows *sqlmock.Rows) {
	mock.ExpectQuery(`FROM pending_approvals p JOIN agents a ON a.id = p.agent_id\s+WHERE p.id = \$1 AND a.organization_id = \$2`).
		WithArgs(id, org).WillReturnRows(rows)
}

var pamEmergencyRowColumns = []string{"id", "organization_id", "incident_id", "declared_by", "declared_at", "expires_at",
	"extended_until", "extended_by", "affected_agents", "granted_caps", "justification", "status", "review_completed"}

func pamEmergencyRow(id, org uuid.UUID, declaredBy string, declaredAt, expiresAt time.Time, extendedUntil interface{}, status string) *sqlmock.Rows {
	return sqlmock.NewRows(pamEmergencyRowColumns).AddRow(id.String(), org.String(), "INC-1", declaredBy, declaredAt, expiresAt,
		extendedUntil, "", "{"+uuid.NewString()+"}", "{infra.modify}", "outage", status, false)
}

func expectPAMEmergency(mock sqlmock.Sqlmock, id, org uuid.UUID, rows *sqlmock.Rows) {
	mock.ExpectQuery(`FROM emergency_declarations WHERE id = \$1 AND organization_id = \$2`).
		WithArgs(id, org).WillReturnRows(rows)
}

var pamCampaignRowColumns = []string{"id", "organization_id", "name", "scope_type", "scope_value", "starts_at", "closes_at", "created_by"}

func expectPAMCampaign(mock sqlmock.Sqlmock, id, org uuid.UUID, scopeType, scopeValue string, startsAt, closesAt time.Time) {
	mock.ExpectQuery(`FROM certification_campaigns WHERE id = \$1 AND organization_id = \$2`).
		WithArgs(id, org).
		WillReturnRows(sqlmock.NewRows(pamCampaignRowColumns).
			AddRow(id.String(), org.String(), "Q4 review", scopeType, scopeValue, startsAt, closesAt, uuid.NewString()))
}

func TestGetCapabilityTier_DefaultStandard(t *testing.T) {
	svc, mock := newPAMServiceMock(t, time.Now())
	expectPAMTier(mock, "unknown.capability", "", false)
	tier, err := svc.GetCapabilityTier(context.Background(), "unknown.capability")
	require.NoError(t, err)
	assert.Equal(t, "STANDARD", tier.Tier)
	assert.Equal(t, 14400, tier.MaxTTLSeconds)
	assert.False(t, tier.RequiresMFA)
}

func TestRequestApproval_RequiresJustification(t *testing.T) {
	svc := NewPAMService(nil, nil)
	req := &ApprovalRequest{
		AgentID:    uuid.New(),
		Capability: "infra.modify",
	}
	_, err := svc.RequestApproval(context.Background(), uuid.New(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "justification")
}

func TestRequestApproval_SetsDefaults(t *testing.T) {
	now := time.Now().UTC()
	org, agent, requester := uuid.New(), uuid.New(), uuid.New()
	svc, mock := newPAMServiceMock(t, now)
	expectPAMTier(mock, "infra.modify", "SUPER_PRIVILEGED", false)
	mock.ExpectExec(`INSERT INTO pending_approvals .+ SELECT .+ FROM agents a WHERE a.id = \$2 AND a.organization_id = \$13`).
		WithArgs(sqlmock.AnyArg(), agent, "infra.modify", "", "Production hotfix for incident INC-12345",
			nil, nil, nil, nil, requester, now, now.Add(5*time.Minute), org).
		WillReturnResult(sqlmock.NewResult(0, 1))

	req := &ApprovalRequest{
		AgentID:       agent,
		Capability:    "infra.modify",
		Justification: "Production hotfix for incident INC-12345",
		RequestedBy:   &requester,
	}
	result, err := svc.RequestApproval(context.Background(), org, req)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, result.ID)
	assert.False(t, result.RequestedAt.IsZero())
	assert.False(t, result.ExpiresAt.IsZero())
	// 5 minute timeout
	assert.WithinDuration(t, result.RequestedAt.Add(5*time.Minute), result.ExpiresAt, time.Second)
	assert.Equal(t, ApprovalStatusPending, result.Status)
	assert.Equal(t, org, result.OrganizationID)
}

func TestDecideApproval_InvalidDecision(t *testing.T) {
	svc := NewPAMService(nil, nil)
	_, err := svc.DecideApproval(context.Background(), uuid.New(), uuid.New(), uuid.New(), "MAYBE", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "APPROVED or DENIED")
}

func TestDeclareEmergency_RequiresIncidentID(t *testing.T) {
	svc := NewPAMService(nil, nil)
	decl := &EmergencyDeclaration{
		DeclaredBy:    "admin@opena2a.org",
		Justification: "Production outage",
	}
	_, err := svc.DeclareEmergency(context.Background(), uuid.New(), decl)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "incident ID")
}

func TestDeclareEmergency_HardCap60Min(t *testing.T) {
	now := time.Now().UTC()
	org, agent := uuid.New(), uuid.New()
	svc, mock := newPAMServiceMock(t, now)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM agents WHERE organization_id = \$1 AND id = ANY\(\$2::uuid\[\]\)`).
		WithArgs(org, "{\""+agent.String()+"\"}").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec(`INSERT INTO emergency_declarations`).
		WithArgs(sqlmock.AnyArg(), org, "INC-99999", "admin@opena2a.org", now, now.Add(60*time.Minute),
			"{"+agent.String()+"}", `{"infra.modify","db.delete"}`, "Critical production incident", "ACTIVE").
		WillReturnResult(sqlmock.NewResult(0, 1))

	decl := &EmergencyDeclaration{
		IncidentID:     "INC-99999",
		DeclaredBy:     "admin@opena2a.org",
		Justification:  "Critical production incident",
		ExpiresAt:      now.Add(24 * time.Hour), // Try 24 hours
		AffectedAgents: []uuid.UUID{agent},
		GrantedCaps:    []string{"infra.modify", "db.delete"},
	}
	result, err := svc.DeclareEmergency(context.Background(), org, decl)
	require.NoError(t, err)
	assert.Equal(t, "ACTIVE", result.Status)
	// Should be capped at 60 minutes, not 24 hours
	maxExpiry := result.DeclaredAt.Add(60 * time.Minute)
	assert.WithinDuration(t, maxExpiry, result.ExpiresAt, time.Second)
}

func TestCreateCampaign_Default30Days(t *testing.T) {
	now := time.Now().UTC()
	org := uuid.New()
	svc, mock := newPAMServiceMock(t, now)
	// The campaign starts in an hour, so it reads PENDING until then.
	start := now.Add(time.Hour)
	mock.ExpectExec(`INSERT INTO certification_campaigns`).
		WithArgs(sqlmock.AnyArg(), org, "Q2 2026 Access Review", "ALL", nil, start, start.Add(30*24*time.Hour), "admin").
		WillReturnResult(sqlmock.NewResult(0, 1))

	campaign := &CertificationCampaign{
		Name:      "Q2 2026 Access Review",
		ScopeType: "ALL",
		StartsAt:  start,
		CreatedBy: "admin",
	}
	result, err := svc.CreateCampaign(context.Background(), org, campaign)
	require.NoError(t, err)
	assert.Equal(t, "PENDING", result.Status)
	assert.WithinDuration(t, start.Add(30*24*time.Hour), result.ClosesAt, time.Second)
}

func TestRecordCertificationDecision_RequiresDecision(t *testing.T) {
	svc := NewPAMService(nil, nil)
	decision := &CertificationDecision{
		CampaignID: uuid.New(),
		AgentID:    uuid.New(),
		Capability: "db.read",
	}
	err := svc.RecordCertificationDecision(context.Background(), uuid.New(), decision)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decision is required")
}

func TestPqFromUUIDs(t *testing.T) {
	assert.Equal(t, "{}", pq_from_uuids(nil))
	id := uuid.New()
	result := pq_from_uuids([]uuid.UUID{id})
	assert.Contains(t, result, id.String())
}

// An approval is requested for a SUPER_PRIVILEGED capability only, with a
// ticket when the tier requires one; a refused request writes nothing.
func TestRequestApproval_RefusesACapabilityOutsideTheSuperPrivilegedTier(t *testing.T) {
	cases := []struct {
		name, capability, tier string
		requiresTicket         bool
		want                   string
	}{
		{"a STANDARD capability", "db.read", "STANDARD", false,
			"capability db.read is in tier STANDARD; approvals are requested for SUPER_PRIVILEGED capabilities"},
		{"a PRIVILEGED capability", "db.write", "PRIVILEGED", false,
			"capability db.write is in tier PRIVILEGED; approvals are requested for SUPER_PRIVILEGED capabilities"},
		{"a capability the registry does not hold", "files:read", "", false,
			"capability files:read is in tier STANDARD; approvals are requested for SUPER_PRIVILEGED capabilities"},
		{"a ticketed capability without a ticket", "credential.rotate", "SUPER_PRIVILEGED", true,
			"capability credential.rotate requires a ticket: set ticketId"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, mock := newPAMServiceMock(t, time.Now())
			expectPAMTier(mock, tc.capability, tc.tier, tc.requiresTicket)
			_, err := svc.RequestApproval(context.Background(), uuid.New(), &ApprovalRequest{
				AgentID: uuid.New(), Capability: tc.capability, Justification: "needed",
			})
			var refused *PAMRequestError
			require.True(t, errors.As(err, &refused), "got %v", err)
			assert.Equal(t, tc.want, refused.Reason)
		})
	}
}

// The approval row is written only for an agent of the caller's organization:
// an agent of another organization, or none, gets ErrPAMNotFound.
func TestRequestApproval_AgentOfAnotherOrganizationIsNotFound(t *testing.T) {
	org, agent := uuid.New(), uuid.New()
	svc, mock := newPAMServiceMock(t, time.Now())
	expectPAMTier(mock, "db.delete", "SUPER_PRIVILEGED", false)
	mock.ExpectExec(`INSERT INTO pending_approvals .+ FROM agents a WHERE a.id = \$2 AND a.organization_id = \$13`).
		WithArgs(sqlmock.AnyArg(), agent, "db.delete", "", "cleanup", nil, nil, nil, nil, nil,
			sqlmock.AnyArg(), sqlmock.AnyArg(), org).
		WillReturnResult(sqlmock.NewResult(0, 0))

	_, err := svc.RequestApproval(context.Background(), org, &ApprovalRequest{
		AgentID: agent, Capability: "db.delete", Justification: "cleanup",
	})
	assert.ErrorIs(t, err, ErrPAMNotFound)
}

func TestGetApproval_OfAnotherOrganizationIsNotFound(t *testing.T) {
	org, id := uuid.New(), uuid.New()
	svc, mock := newPAMServiceMock(t, time.Now())
	expectPAMApproval(mock, id, org, sqlmock.NewRows(pamApprovalRowColumns))

	_, err := svc.GetApproval(context.Background(), org, id)
	assert.ErrorIs(t, err, ErrPAMNotFound)
}

// The user who requested an approval cannot decide it, and nothing is written.
func TestDecideApproval_RefusesTheRequester(t *testing.T) {
	now := time.Now().UTC()
	org, id, requester := uuid.New(), uuid.New(), uuid.New()
	svc, mock := newPAMServiceMock(t, now)
	expectPAMApproval(mock, id, org, pamApprovalRow(id, org, uuid.New(), requester, now.Add(-time.Minute), ""))

	_, err := svc.DecideApproval(context.Background(), org, id, requester, "APPROVED", "")
	assert.ErrorIs(t, err, ErrPAMSameUser)
}

// A decided approval, and one whose window closed, are not pending.
func TestDecideApproval_RefusesAnApprovalThatIsNotPending(t *testing.T) {
	now := time.Now().UTC()
	cases := map[string]struct {
		requestedAt time.Time
		decision    string
	}{
		"decided":              {now.Add(-time.Minute), "DENIED"},
		"past its window":      {now.Add(-ApprovalDecisionWindow), ""},
		"well past its window": {now.Add(-time.Hour), ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			org, id := uuid.New(), uuid.New()
			svc, mock := newPAMServiceMock(t, now)
			expectPAMApproval(mock, id, org, pamApprovalRow(id, org, uuid.New(), uuid.New(), tc.requestedAt, tc.decision))

			_, err := svc.DecideApproval(context.Background(), org, id, uuid.New(), "APPROVED", "")
			assert.ErrorIs(t, err, ErrPAMApprovalNotPending)
		})
	}
}

func TestDecideApproval_RecordsTheDecisionOfASecondUser(t *testing.T) {
	now := time.Now().UTC()
	org, id, approver := uuid.New(), uuid.New(), uuid.New()
	svc, mock := newPAMServiceMock(t, now)
	expectPAMApproval(mock, id, org, pamApprovalRow(id, org, uuid.New(), uuid.New(), now.Add(-time.Minute), ""))
	mock.ExpectExec(`UPDATE pending_approvals p\s+SET approver_id = \$1, approver_decision = \$2, approver_note = \$3, decided_at = \$4\s+FROM agents a\s+WHERE p.id = \$5 AND a.id = p.agent_id AND a.organization_id = \$6\s+AND p.approver_decision IS NULL AND p.expires_at > \$4\s+AND \(p.requested_by IS NULL OR p.requested_by <> \$1\)`).
		WithArgs(approver, "APPROVED", "ok for the incident", now, id, org).
		WillReturnResult(sqlmock.NewResult(0, 1))

	decided, err := svc.DecideApproval(context.Background(), org, id, approver, "APPROVED", "ok for the incident")
	require.NoError(t, err)
	assert.Equal(t, ApprovalStatusApproved, decided.Status)
	assert.Equal(t, "APPROVED", decided.ApproverDecision)
	require.NotNil(t, decided.ApproverID)
	assert.Equal(t, approver, *decided.ApproverID)
}

// An approval decided by another request between the read and the write is
// not pending.
func TestDecideApproval_LosesARaceAsNotPending(t *testing.T) {
	now := time.Now().UTC()
	org, id := uuid.New(), uuid.New()
	svc, mock := newPAMServiceMock(t, now)
	expectPAMApproval(mock, id, org, pamApprovalRow(id, org, uuid.New(), uuid.New(), now.Add(-time.Minute), ""))
	mock.ExpectExec(`UPDATE pending_approvals`).WillReturnResult(sqlmock.NewResult(0, 0))

	_, err := svc.DecideApproval(context.Background(), org, id, uuid.New(), "DENIED", "")
	assert.ErrorIs(t, err, ErrPAMApprovalNotPending)
}

// A declaration naming an agent the organization does not have is refused
// before anything is written.
func TestDeclareEmergency_AgentOfAnotherOrganizationIsNotFound(t *testing.T) {
	org, mine, theirs := uuid.New(), uuid.New(), uuid.New()
	svc, mock := newPAMServiceMock(t, time.Now())
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM agents WHERE organization_id = \$1`).
		WithArgs(org, `{"`+mine.String()+`","`+theirs.String()+`"}`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	_, err := svc.DeclareEmergency(context.Background(), org, &EmergencyDeclaration{
		IncidentID: "INC-7", Justification: "outage", DeclaredBy: uuid.NewString(),
		AffectedAgents: []uuid.UUID{mine, theirs, mine}, GrantedCaps: []string{"infra.modify"},
	})
	assert.ErrorIs(t, err, ErrPAMNotFound)
}

func TestDeclareEmergency_RefusesAnEmptyOrPastDeclaration(t *testing.T) {
	now := time.Now().UTC()
	cases := map[string]struct {
		decl *EmergencyDeclaration
		want string
	}{
		"no agents": {&EmergencyDeclaration{IncidentID: "INC-1", Justification: "outage", GrantedCaps: []string{"infra.modify"}},
			"affectedAgents must name at least one agent"},
		"no capabilities": {&EmergencyDeclaration{IncidentID: "INC-1", Justification: "outage", AffectedAgents: []uuid.UUID{uuid.New()}},
			"grantedCaps must name at least one capability"},
		"an empty capability": {&EmergencyDeclaration{IncidentID: "INC-1", Justification: "outage", AffectedAgents: []uuid.UUID{uuid.New()}, GrantedCaps: []string{" "}},
			"grantedCaps holds an empty capability"},
		"an expiry in the past": {&EmergencyDeclaration{IncidentID: "INC-1", Justification: "outage", AffectedAgents: []uuid.UUID{uuid.New()}, GrantedCaps: []string{"infra.modify"}, ExpiresAt: now.Add(-time.Minute)},
			"expiresAt must be after the declaration"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := newPAMServiceMock(t, now)
			_, err := svc.DeclareEmergency(context.Background(), uuid.New(), tc.decl)
			var refused *PAMRequestError
			require.True(t, errors.As(err, &refused), "got %v", err)
			assert.Equal(t, tc.want, refused.Reason)
		})
	}
}

// The one extension runs 15 minutes from the declaration's expiry, and never
// past 75 minutes from the declaration.
func TestExtendEmergency_AddsFifteenMinutesToTheExpiry(t *testing.T) {
	now := time.Now().UTC()
	declaredAt := now.Add(-10 * time.Minute)
	cases := map[string]struct {
		expiresAt time.Time
		want      time.Time
	}{
		"a 30 minute declaration": {declaredAt.Add(30 * time.Minute), declaredAt.Add(45 * time.Minute)},
		"a 60 minute declaration": {declaredAt.Add(60 * time.Minute), declaredAt.Add(75 * time.Minute)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			org, id, declarer, extender := uuid.New(), uuid.New(), uuid.NewString(), uuid.NewString()
			svc, mock := newPAMServiceMock(t, now)
			expectPAMEmergency(mock, id, org, pamEmergencyRow(id, org, declarer, declaredAt, tc.expiresAt, nil, "ACTIVE"))
			mock.ExpectExec(`UPDATE emergency_declarations\s+SET extended_until = \$1, extended_by = \$2\s+WHERE id = \$3 AND organization_id = \$4 AND status = 'ACTIVE' AND extended_until IS NULL\s+AND expires_at > \$5 AND declared_by <> \$2`).
				WithArgs(tc.want, extender, id, org, now).
				WillReturnResult(sqlmock.NewResult(0, 1))

			extended, err := svc.ExtendEmergency(context.Background(), org, id, extender)
			require.NoError(t, err)
			require.NotNil(t, extended.ExtendedUntil)
			assert.Equal(t, tc.want, *extended.ExtendedUntil)
			assert.Equal(t, extender, extended.ExtendedBy)
		})
	}
}

func TestExtendEmergency_RefusesTheDeclarerAndASecondExtension(t *testing.T) {
	now := time.Now().UTC()
	declaredAt := now.Add(-10 * time.Minute)
	declarer := uuid.NewString()
	cases := map[string]struct {
		extender      string
		expiresAt     time.Time
		extendedUntil interface{}
		status        string
		want          error
	}{
		"the declarer":    {declarer, declaredAt.Add(60 * time.Minute), nil, "ACTIVE", ErrPAMSameUser},
		"an extended one": {uuid.NewString(), declaredAt.Add(60 * time.Minute), declaredAt.Add(75 * time.Minute), "ACTIVE", ErrPAMEmergencyExtended},
		"an expired one":  {uuid.NewString(), now, nil, "ACTIVE", ErrPAMEmergencyNotActive},
		"a revoked one":   {uuid.NewString(), declaredAt.Add(60 * time.Minute), nil, "REVOKED", ErrPAMEmergencyNotActive},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			org, id := uuid.New(), uuid.New()
			svc, mock := newPAMServiceMock(t, now)
			expectPAMEmergency(mock, id, org, pamEmergencyRow(id, org, declarer, declaredAt, tc.expiresAt, tc.extendedUntil, tc.status))

			_, err := svc.ExtendEmergency(context.Background(), org, id, tc.extender)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestRevokeEmergency_EndsAnActiveDeclarationOnly(t *testing.T) {
	now := time.Now().UTC()
	declaredAt := now.Add(-10 * time.Minute)

	t.Run("active", func(t *testing.T) {
		org, id := uuid.New(), uuid.New()
		svc, mock := newPAMServiceMock(t, now)
		expectPAMEmergency(mock, id, org, pamEmergencyRow(id, org, uuid.NewString(), declaredAt, declaredAt.Add(time.Hour), nil, "ACTIVE"))
		mock.ExpectExec(`UPDATE emergency_declarations SET status = 'REVOKED'\s+WHERE id = \$1 AND organization_id = \$2 AND status = 'ACTIVE'\s+AND COALESCE\(extended_until, expires_at\) > \$3`).
			WithArgs(id, org, now).
			WillReturnResult(sqlmock.NewResult(0, 1))

		revoked, err := svc.RevokeEmergency(context.Background(), org, id)
		require.NoError(t, err)
		assert.Equal(t, EmergencyStatusRevoked, revoked.Status)
	})

	t.Run("past its extension", func(t *testing.T) {
		org, id := uuid.New(), uuid.New()
		svc, mock := newPAMServiceMock(t, now)
		expectPAMEmergency(mock, id, org, pamEmergencyRow(id, org, uuid.NewString(), now.Add(-2*time.Hour), now.Add(-time.Hour), now.Add(-45*time.Minute), "ACTIVE"))

		_, err := svc.RevokeEmergency(context.Background(), org, id)
		assert.ErrorIs(t, err, ErrPAMEmergencyNotActive)
	})
}

func TestEmergencyStatus_ReadsTheExtension(t *testing.T) {
	now := time.Now().UTC()
	until := now.Add(time.Minute)
	d := &EmergencyDeclaration{Status: "ACTIVE", ExpiresAt: now.Add(-time.Minute), ExtendedUntil: &until}
	assert.Equal(t, EmergencyStatusActive, emergencyStatus(d, now))
	assert.Equal(t, EmergencyStatusExpired, emergencyStatus(d, until))
	d.Status = "REVOKED"
	assert.Equal(t, EmergencyStatusRevoked, emergencyStatus(d, now))
}

func TestCampaignStatus_ReadsTheClock(t *testing.T) {
	now := time.Now().UTC()
	c := &CertificationCampaign{StartsAt: now, ClosesAt: now.Add(time.Hour)}
	assert.Equal(t, CampaignStatusPending, campaignStatus(c, now.Add(-time.Second)))
	assert.Equal(t, CampaignStatusActive, campaignStatus(c, now))
	assert.Equal(t, CampaignStatusClosed, campaignStatus(c, now.Add(time.Hour)))
}

func TestCreateCampaign_RefusesAnInvalidScope(t *testing.T) {
	now := time.Now().UTC()
	cases := map[string]struct {
		campaign *CertificationCampaign
		want     string
	}{
		"TEAM":                    {&CertificationCampaign{Name: "r", ScopeType: "TEAM", ScopeValue: "sre"}, "scopeType must be ALL, TIER or AGENT"},
		"ALL with value":          {&CertificationCampaign{Name: "r", ScopeType: "ALL", ScopeValue: "x"}, "scopeValue must be empty for scopeType ALL"},
		"TIER unknown":            {&CertificationCampaign{Name: "r", ScopeType: "TIER", ScopeValue: "ROOT"}, "scopeValue must be STANDARD, PRIVILEGED or SUPER_PRIVILEGED for scopeType TIER"},
		"AGENT not a UUID":        {&CertificationCampaign{Name: "r", ScopeType: "AGENT", ScopeValue: "agent-1"}, "scopeValue must be an agent ID for scopeType AGENT"},
		"no name":                 {&CertificationCampaign{Name: " ", ScopeType: "ALL"}, "name is required"},
		"closed already":          {&CertificationCampaign{Name: "r", ScopeType: "ALL", StartsAt: now.Add(-48 * time.Hour), ClosesAt: now.Add(-time.Hour)}, "closesAt must be in the future"},
		"closes before it starts": {&CertificationCampaign{Name: "r", ScopeType: "ALL", StartsAt: now.Add(48 * time.Hour), ClosesAt: now.Add(time.Hour)}, "closesAt must be after startsAt"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := newPAMServiceMock(t, now)
			_, err := svc.CreateCampaign(context.Background(), uuid.New(), tc.campaign)
			var refused *PAMRequestError
			require.True(t, errors.As(err, &refused), "got %v", err)
			assert.Equal(t, tc.want, refused.Reason)
		})
	}
}

func TestCreateCampaign_AgentScopeOfAnotherOrganizationIsNotFound(t *testing.T) {
	org, agent := uuid.New(), uuid.New()
	svc, mock := newPAMServiceMock(t, time.Now())
	mock.ExpectQuery(`SELECT EXISTS \(SELECT 1 FROM agents WHERE id = \$1 AND organization_id = \$2\)`).
		WithArgs(agent, org).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	_, err := svc.CreateCampaign(context.Background(), org, &CertificationCampaign{
		Name: "agent review", ScopeType: "AGENT", ScopeValue: agent.String(),
	})
	assert.ErrorIs(t, err, ErrPAMNotFound)
}

func TestRecordCertificationDecision_InAnActiveCampaignOfTheOrganization(t *testing.T) {
	now := time.Now().UTC()
	org, campaign, agent := uuid.New(), uuid.New(), uuid.New()
	reviewer := uuid.NewString()
	svc, mock := newPAMServiceMock(t, now)
	expectPAMCampaign(mock, campaign, org, "ALL", "", now.Add(-time.Hour), now.Add(time.Hour))
	decisionID := uuid.New()
	mock.ExpectQuery(`INSERT INTO certification_decisions .+ FROM agents a WHERE a.id = \$3 AND a.organization_id = \$9\s+ON CONFLICT \(campaign_id, agent_id, capability\) DO UPDATE SET .+ RETURNING id`).
		WithArgs(sqlmock.AnyArg(), campaign, agent, "db.read", "REVOKED", reviewer, "unused since June", now, org).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(decisionID.String()))

	decision := &CertificationDecision{CampaignID: campaign, AgentID: agent, Capability: "db.read",
		Decision: "REVOKED", ReviewerID: reviewer, ReviewerNote: "unused since June"}
	require.NoError(t, svc.RecordCertificationDecision(context.Background(), org, decision))
	assert.Equal(t, decisionID, decision.ID)
	require.NotNil(t, decision.DecidedAt)
	assert.Equal(t, now, *decision.DecidedAt)
}

func TestRecordCertificationDecision_RefusedOutsideTheCampaign(t *testing.T) {
	now := time.Now().UTC()
	agent := uuid.New()

	t.Run("a closed campaign", func(t *testing.T) {
		org, campaign := uuid.New(), uuid.New()
		svc, mock := newPAMServiceMock(t, now)
		expectPAMCampaign(mock, campaign, org, "ALL", "", now.Add(-48*time.Hour), now)
		err := svc.RecordCertificationDecision(context.Background(), org, &CertificationDecision{
			CampaignID: campaign, AgentID: agent, Capability: "db.read", Decision: "CERTIFIED"})
		assert.ErrorIs(t, err, ErrPAMCampaignNotActive)
	})

	t.Run("another agent than the campaign's", func(t *testing.T) {
		org, campaign, scoped := uuid.New(), uuid.New(), uuid.New()
		svc, mock := newPAMServiceMock(t, now)
		expectPAMCampaign(mock, campaign, org, "AGENT", scoped.String(), now.Add(-time.Hour), now.Add(time.Hour))
		err := svc.RecordCertificationDecision(context.Background(), org, &CertificationDecision{
			CampaignID: campaign, AgentID: agent, Capability: "db.read", Decision: "CERTIFIED"})
		var refused *PAMRequestError
		require.True(t, errors.As(err, &refused), "got %v", err)
		assert.Equal(t, "the campaign covers agent "+scoped.String()+" only", refused.Reason)
	})

	t.Run("a capability of another tier than the campaign's", func(t *testing.T) {
		org, campaign := uuid.New(), uuid.New()
		svc, mock := newPAMServiceMock(t, now)
		expectPAMCampaign(mock, campaign, org, "TIER", "SUPER_PRIVILEGED", now.Add(-time.Hour), now.Add(time.Hour))
		expectPAMTier(mock, "db.read", "STANDARD", false)
		err := svc.RecordCertificationDecision(context.Background(), org, &CertificationDecision{
			CampaignID: campaign, AgentID: agent, Capability: "db.read", Decision: "CERTIFIED"})
		var refused *PAMRequestError
		require.True(t, errors.As(err, &refused), "got %v", err)
		assert.Equal(t, "capability db.read is in tier STANDARD; the campaign covers tier SUPER_PRIVILEGED", refused.Reason)
	})

	t.Run("an agent of another organization", func(t *testing.T) {
		org, campaign := uuid.New(), uuid.New()
		svc, mock := newPAMServiceMock(t, now)
		expectPAMCampaign(mock, campaign, org, "ALL", "", now.Add(-time.Hour), now.Add(time.Hour))
		mock.ExpectQuery(`INSERT INTO certification_decisions`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
		err := svc.RecordCertificationDecision(context.Background(), org, &CertificationDecision{
			CampaignID: campaign, AgentID: agent, Capability: "db.read", Decision: "CERTIFIED"})
		assert.ErrorIs(t, err, ErrPAMNotFound)
	})
}

// Each list reads the caller's organization's rows, newest first, at most
// PAMListLimit of them.
func TestPAMLists_ReadTheOrganizationOnly(t *testing.T) {
	now := time.Now().UTC()
	org := uuid.New()
	svc, mock := newPAMServiceMock(t, now)

	mock.ExpectQuery(`FROM pending_approvals p JOIN agents a ON a.id = p.agent_id\s+WHERE a.organization_id = \$1\s+ORDER BY p.requested_at DESC, p.id\s+LIMIT \$2`).
		WithArgs(org, PAMListLimit).
		WillReturnRows(pamApprovalRow(uuid.New(), org, uuid.New(), uuid.New(), now.Add(-time.Hour), ""))
	approvals, err := svc.ListApprovals(context.Background(), org)
	require.NoError(t, err)
	require.Len(t, approvals, 1)
	assert.Equal(t, ApprovalStatusExpired, approvals[0].Status)

	mock.ExpectQuery(`FROM emergency_declarations\s+WHERE organization_id = \$1\s+ORDER BY declared_at DESC, id\s+LIMIT \$2`).
		WithArgs(org, PAMListLimit).
		WillReturnRows(sqlmock.NewRows(pamEmergencyRowColumns))
	emergencies, err := svc.ListEmergencies(context.Background(), org)
	require.NoError(t, err)
	assert.NotNil(t, emergencies)
	assert.Empty(t, emergencies)

	mock.ExpectQuery(`FROM certification_campaigns\s+WHERE organization_id = \$1\s+ORDER BY starts_at DESC, id\s+LIMIT \$2`).
		WithArgs(org, PAMListLimit).
		WillReturnRows(sqlmock.NewRows(pamCampaignRowColumns))
	campaigns, err := svc.ListCampaigns(context.Background(), org)
	require.NoError(t, err)
	assert.Empty(t, campaigns)

	campaign := uuid.New()
	mock.ExpectQuery(`FROM certification_decisions d\s+JOIN certification_campaigns c ON c.id = d.campaign_id\s+WHERE d.campaign_id = \$1 AND c.organization_id = \$2`).
		WithArgs(campaign, org).
		WillReturnRows(sqlmock.NewRows([]string{"id", "campaign_id", "agent_id", "capability", "decision", "reviewer_id", "reviewer_note", "decided_at"}))
	decisions, err := svc.ListCertificationDecisions(context.Background(), org, campaign)
	require.NoError(t, err)
	assert.Empty(t, decisions)
}
