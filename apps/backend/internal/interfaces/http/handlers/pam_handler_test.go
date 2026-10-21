package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// fakePAMService holds records of any organization, answers each read for the
// organization it is given, records the writes it is given and returns err
// from each write when err is set.
type fakePAMService struct {
	approvals   map[uuid.UUID]*application.ApprovalRequest
	emergencies map[uuid.UUID]*application.EmergencyDeclaration
	campaigns   map[uuid.UUID]*application.CertificationCampaign
	err         error

	requested []*application.ApprovalRequest
	decided   []string // approval id, approver id, decision
	declared  []*application.EmergencyDeclaration
	extended  []string // emergency id, extended by
	revoked   []uuid.UUID
	created   []*application.CertificationCampaign
	decisions []*application.CertificationDecision
	orgs      []uuid.UUID // the organization each write named
}

func newFakePAMService() *fakePAMService {
	return &fakePAMService{
		approvals:   map[uuid.UUID]*application.ApprovalRequest{},
		emergencies: map[uuid.UUID]*application.EmergencyDeclaration{},
		campaigns:   map[uuid.UUID]*application.CertificationCampaign{},
	}
}

func (f *fakePAMService) ListCapabilityTiers(context.Context) ([]*application.CapabilityTier, error) {
	return []*application.CapabilityTier{{Capability: "infra.modify", Tier: "SUPER_PRIVILEGED", MaxTTLSeconds: 900}}, nil
}

func (f *fakePAMService) RequestApproval(_ context.Context, orgID uuid.UUID, req *application.ApprovalRequest) (*application.ApprovalRequest, error) {
	f.orgs = append(f.orgs, orgID)
	f.requested = append(f.requested, req)
	if f.err != nil {
		return nil, f.err
	}
	req.ID, req.OrganizationID, req.Status = uuid.New(), orgID, application.ApprovalStatusPending
	f.approvals[req.ID] = req
	return req, nil
}

func (f *fakePAMService) GetApproval(_ context.Context, orgID, id uuid.UUID) (*application.ApprovalRequest, error) {
	a, ok := f.approvals[id]
	if !ok || a.OrganizationID != orgID {
		return nil, application.ErrPAMNotFound
	}
	return a, nil
}

func (f *fakePAMService) ListApprovals(_ context.Context, orgID uuid.UUID) ([]*application.ApprovalRequest, error) {
	out := make([]*application.ApprovalRequest, 0)
	for _, a := range f.approvals {
		if a.OrganizationID == orgID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakePAMService) DecideApproval(ctx context.Context, orgID, id, approverID uuid.UUID, decision, _ string) (*application.ApprovalRequest, error) {
	f.orgs = append(f.orgs, orgID)
	f.decided = append(f.decided, id.String(), approverID.String(), decision)
	if f.err != nil {
		return nil, f.err
	}
	a, err := f.GetApproval(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	a.ApproverDecision, a.Status, a.ApproverID = decision, decision, &approverID
	return a, nil
}

func (f *fakePAMService) DeclareEmergency(_ context.Context, orgID uuid.UUID, decl *application.EmergencyDeclaration) (*application.EmergencyDeclaration, error) {
	f.orgs = append(f.orgs, orgID)
	f.declared = append(f.declared, decl)
	if f.err != nil {
		return nil, f.err
	}
	decl.ID, decl.OrganizationID, decl.Status = uuid.New(), orgID, application.EmergencyStatusActive
	f.emergencies[decl.ID] = decl
	return decl, nil
}

func (f *fakePAMService) GetEmergency(_ context.Context, orgID, id uuid.UUID) (*application.EmergencyDeclaration, error) {
	d, ok := f.emergencies[id]
	if !ok || d.OrganizationID != orgID {
		return nil, application.ErrPAMNotFound
	}
	return d, nil
}

func (f *fakePAMService) ListEmergencies(_ context.Context, orgID uuid.UUID) ([]*application.EmergencyDeclaration, error) {
	out := make([]*application.EmergencyDeclaration, 0)
	for _, d := range f.emergencies {
		if d.OrganizationID == orgID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (f *fakePAMService) ExtendEmergency(ctx context.Context, orgID, id uuid.UUID, extendedBy string) (*application.EmergencyDeclaration, error) {
	f.orgs = append(f.orgs, orgID)
	f.extended = append(f.extended, id.String(), extendedBy)
	if f.err != nil {
		return nil, f.err
	}
	return f.GetEmergency(ctx, orgID, id)
}

func (f *fakePAMService) RevokeEmergency(ctx context.Context, orgID, id uuid.UUID) (*application.EmergencyDeclaration, error) {
	f.orgs = append(f.orgs, orgID)
	f.revoked = append(f.revoked, id)
	if f.err != nil {
		return nil, f.err
	}
	d, err := f.GetEmergency(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	d.Status = application.EmergencyStatusRevoked
	return d, nil
}

func (f *fakePAMService) CreateCampaign(_ context.Context, orgID uuid.UUID, campaign *application.CertificationCampaign) (*application.CertificationCampaign, error) {
	f.orgs = append(f.orgs, orgID)
	f.created = append(f.created, campaign)
	if f.err != nil {
		return nil, f.err
	}
	campaign.ID, campaign.OrganizationID, campaign.Status = uuid.New(), orgID, application.CampaignStatusActive
	f.campaigns[campaign.ID] = campaign
	return campaign, nil
}

func (f *fakePAMService) GetCampaign(_ context.Context, orgID, id uuid.UUID) (*application.CertificationCampaign, error) {
	c, ok := f.campaigns[id]
	if !ok || c.OrganizationID != orgID {
		return nil, application.ErrPAMNotFound
	}
	return c, nil
}

func (f *fakePAMService) ListCampaigns(_ context.Context, orgID uuid.UUID) ([]*application.CertificationCampaign, error) {
	out := make([]*application.CertificationCampaign, 0)
	for _, c := range f.campaigns {
		if c.OrganizationID == orgID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakePAMService) RecordCertificationDecision(_ context.Context, orgID uuid.UUID, decision *application.CertificationDecision) error {
	f.orgs = append(f.orgs, orgID)
	f.decisions = append(f.decisions, decision)
	if f.err != nil {
		return f.err
	}
	decision.ID = uuid.New()
	return nil
}

func (f *fakePAMService) ListCertificationDecisions(_ context.Context, orgID, campaignID uuid.UUID) ([]*application.CertificationDecision, error) {
	out := make([]*application.CertificationDecision, 0)
	for _, d := range f.decisions {
		if d.CampaignID == campaignID {
			out = append(out, d)
		}
	}
	return out, nil
}

// pamAuditRow is one audit row the handler wrote.
type pamAuditRow struct {
	org, user    uuid.UUID
	action       domain.AuditAction
	resourceType string
	resourceID   uuid.UUID
	metadata     map[string]interface{}
}

type pamAudit struct {
	AuditServicer
	rows *[]pamAuditRow
}

func (a pamAudit) LogAction(_ context.Context, orgID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, _, _ string, metadata map[string]interface{}) error {
	*a.rows = append(*a.rows, pamAuditRow{orgID, userID, action, resourceType, resourceID, metadata})
	return nil
}

type pamRig struct {
	app     *fiber.App
	service *fakePAMService
	audit   *[]pamAuditRow
	org     uuid.UUID
	user    uuid.UUID
}

// newPAMRig mounts the handler on the /pam paths with no role gate: a request
// is the rig's user in the rig's organization.
func newPAMRig(t *testing.T) *pamRig {
	t.Helper()
	rig := &pamRig{service: newFakePAMService(), audit: &[]pamAuditRow{}, org: uuid.New(), user: uuid.New()}
	h := NewPAMHandler(rig.service, pamAudit{rows: rig.audit})
	rig.app = fiber.New()
	rig.app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", rig.org)
		c.Locals("user_id", rig.user)
		return c.Next()
	})
	rig.app.Get("/pam/tiers", h.ListTiers)
	rig.app.Post("/pam/approvals", h.RequestApproval)
	rig.app.Get("/pam/approvals", h.ListApprovals)
	rig.app.Get("/pam/approvals/:id", h.GetApproval)
	rig.app.Post("/pam/approvals/:id/decision", h.DecideApproval)
	rig.app.Post("/pam/emergencies", h.DeclareEmergency)
	rig.app.Get("/pam/emergencies", h.ListEmergencies)
	rig.app.Post("/pam/emergencies/:id/extend", h.ExtendEmergency)
	rig.app.Post("/pam/emergencies/:id/revoke", h.RevokeEmergency)
	rig.app.Post("/pam/campaigns", h.CreateCampaign)
	rig.app.Get("/pam/campaigns", h.ListCampaigns)
	rig.app.Post("/pam/campaigns/:id/decisions", h.RecordCertificationDecision)
	rig.app.Get("/pam/campaigns/:id/decisions", h.ListCertificationDecisions)
	return rig
}

func (r *pamRig) do(t *testing.T, method, path, body string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out map[string]interface{}
	if len(raw) > 0 {
		require.NoError(t, json.Unmarshal(raw, &out), "body: %s", raw)
	}
	return resp.StatusCode, out
}

func TestPAMHandler_RequestApprovalNamesTheCallerAsRequester(t *testing.T) {
	rig := newPAMRig(t)
	agent := uuid.New()

	status, body := rig.do(t, "POST", "/pam/approvals",
		`{"agentId":"`+agent.String()+`","capability":"infra.modify","justification":"INC-12 hotfix","ticketId":"CHG-7","ticketSystem":"servicenow"}`)
	require.Equal(t, fiber.StatusCreated, status, "body: %v", body)
	assert.Equal(t, "PENDING", body["status"])
	assert.Equal(t, rig.user.String(), body["requestedBy"])

	require.Len(t, rig.service.requested, 1)
	got := rig.service.requested[0]
	assert.Equal(t, agent, got.AgentID)
	assert.Equal(t, "CHG-7", got.TicketID)
	require.NotNil(t, got.RequestedBy)
	assert.Equal(t, rig.user, *got.RequestedBy)
	assert.Equal(t, []uuid.UUID{rig.org}, rig.service.orgs)

	require.Len(t, *rig.audit, 1)
	row := (*rig.audit)[0]
	assert.Equal(t, domain.AuditActionCreate, row.action)
	assert.Equal(t, "pam_approval", row.resourceType)
	assert.Equal(t, got.ID, row.resourceID)
	assert.Equal(t, rig.org, row.org)
	assert.Equal(t, rig.user, row.user)
	assert.Equal(t, "infra.modify", row.metadata["capability"])
}

// A body cannot set what the server sets: the expiry, the requester and the
// status are refused by name and nothing reaches the service.
func TestPAMHandler_RefusesAFieldTheBodyMayNotCarry(t *testing.T) {
	rig := newPAMRig(t)
	agent := uuid.New().String()
	for _, field := range []string{`"expiresAt":"2030-01-01T00:00:00Z"`, `"requestedBy":"` + uuid.NewString() + `"`, `"status":"APPROVED"`} {
		status, body := rig.do(t, "POST", "/pam/approvals",
			`{"agentId":"`+agent+`","capability":"infra.modify","justification":"x",`+field+`}`)
		assert.Equal(t, fiber.StatusBadRequest, status)
		assert.Equal(t, "unknown field: "+strings.Trim(strings.SplitN(field, ":", 2)[0], `"`), body["error"])
	}
	status, body := rig.do(t, "POST", "/pam/approvals", `{"agentId":"agent-1","capability":"infra.modify","justification":"x"}`)
	assert.Equal(t, fiber.StatusBadRequest, status)
	assert.Equal(t, "agentId must be an agent ID", body["error"])
	status, _ = rig.do(t, "POST", "/pam/approvals", `[]`)
	assert.Equal(t, fiber.StatusBadRequest, status)

	assert.Empty(t, rig.service.requested)
	assert.Empty(t, *rig.audit)
}

// Each service error has one answer, and a failure the service did not
// expect says nothing of its cause.
func TestPAMHandler_AnswersEachServiceError(t *testing.T) {
	cases := []struct {
		err    error
		status int
		body   string
	}{
		{&application.PAMRequestError{Reason: "capability db.read is in tier STANDARD; approvals are requested for SUPER_PRIVILEGED capabilities"},
			fiber.StatusBadRequest, "capability db.read is in tier STANDARD; approvals are requested for SUPER_PRIVILEGED capabilities"},
		{application.ErrPAMNotFound, fiber.StatusNotFound, "not found"},
		{errors.New("pq: relation \"pending_approvals\" does not exist"), fiber.StatusInternalServerError, ServerErrorMessage},
	}
	for _, tc := range cases {
		t.Run(tc.body, func(t *testing.T) {
			rig := newPAMRig(t)
			rig.service.err = tc.err
			status, body := rig.do(t, "POST", "/pam/approvals",
				`{"agentId":"`+uuid.NewString()+`","capability":"db.read","justification":"x"}`)
			assert.Equal(t, tc.status, status)
			assert.Equal(t, map[string]interface{}{"error": tc.body}, body)
			assert.Empty(t, *rig.audit, "a refused request writes no audit row")
		})
	}
}

func TestPAMHandler_DecideApproval(t *testing.T) {
	newApproval := func(rig *pamRig, org uuid.UUID) *application.ApprovalRequest {
		a := &application.ApprovalRequest{ID: uuid.New(), OrganizationID: org, AgentID: uuid.New(), Capability: "db.delete", Status: "PENDING"}
		rig.service.approvals[a.ID] = a
		return a
	}

	t.Run("an approval of another organization is not found and not decided", func(t *testing.T) {
		rig := newPAMRig(t)
		a := newApproval(rig, uuid.New())
		status, body := rig.do(t, "POST", "/pam/approvals/"+a.ID.String()+"/decision", `{"decision":"APPROVED"}`)
		assert.Equal(t, fiber.StatusNotFound, status)
		assert.Equal(t, "not found", body["error"])
		assert.Empty(t, rig.service.decided)
	})

	t.Run("an id that is not a UUID", func(t *testing.T) {
		rig := newPAMRig(t)
		status, body := rig.do(t, "POST", "/pam/approvals/approval-1/decision", `{"decision":"APPROVED"}`)
		assert.Equal(t, fiber.StatusBadRequest, status)
		assert.Equal(t, "invalid approval id", body["error"])
	})

	refusals := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{"the requester", application.ErrPAMSameUser, fiber.StatusForbidden, "the user who requested an approval cannot decide it"},
		{"an approval that is not pending", application.ErrPAMApprovalNotPending, fiber.StatusConflict, "the approval is not pending: it was decided or its decision window closed"},
		{"a decision that is neither", &application.PAMRequestError{Reason: "decision must be APPROVED or DENIED"}, fiber.StatusBadRequest, "decision must be APPROVED or DENIED"},
	}
	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			rig := newPAMRig(t)
			a := newApproval(rig, rig.org)
			rig.service.err = tc.err
			status, body := rig.do(t, "POST", "/pam/approvals/"+a.ID.String()+"/decision", `{"decision":"APPROVED"}`)
			assert.Equal(t, tc.status, status)
			assert.Equal(t, tc.body, body["error"])
			assert.Empty(t, *rig.audit)
		})
	}

	t.Run("a second user's decision", func(t *testing.T) {
		rig := newPAMRig(t)
		a := newApproval(rig, rig.org)
		status, body := rig.do(t, "POST", "/pam/approvals/"+a.ID.String()+"/decision", `{"decision":"DENIED","note":"not during the freeze"}`)
		require.Equal(t, fiber.StatusOK, status, "body: %v", body)
		assert.Equal(t, "DENIED", body["status"])
		assert.Equal(t, []string{a.ID.String(), rig.user.String(), "DENIED"}, rig.service.decided)
		require.Len(t, *rig.audit, 1)
		assert.Equal(t, domain.AuditActionUpdate, (*rig.audit)[0].action)
		assert.Equal(t, "DENIED", (*rig.audit)[0].metadata["decision"])
	})
}

func TestPAMHandler_GetAndListApprovalsOfTheOrganization(t *testing.T) {
	rig := newPAMRig(t)
	mine := &application.ApprovalRequest{ID: uuid.New(), OrganizationID: rig.org, Status: "PENDING"}
	theirs := &application.ApprovalRequest{ID: uuid.New(), OrganizationID: uuid.New(), Status: "PENDING"}
	rig.service.approvals[mine.ID], rig.service.approvals[theirs.ID] = mine, theirs

	status, body := rig.do(t, "GET", "/pam/approvals", "")
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, float64(1), body["total"])
	list := body["approvals"].([]interface{})
	require.Len(t, list, 1)
	assert.Equal(t, mine.ID.String(), list[0].(map[string]interface{})["id"])

	status, body = rig.do(t, "GET", "/pam/approvals/"+mine.ID.String(), "")
	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, "PENDING", body["status"])
	status, _ = rig.do(t, "GET", "/pam/approvals/"+theirs.ID.String(), "")
	assert.Equal(t, fiber.StatusNotFound, status)
}

func TestPAMHandler_DeclareEmergencyNamesTheCallerAsDeclarer(t *testing.T) {
	rig := newPAMRig(t)
	agent := uuid.New()
	status, body := rig.do(t, "POST", "/pam/emergencies",
		`{"incidentId":"INC-42","justification":"database outage","affectedAgents":["`+agent.String()+`"],"grantedCaps":["db.write:bulk"],"expiresAt":"2030-01-01T00:30:00Z"}`)
	require.Equal(t, fiber.StatusCreated, status, "body: %v", body)
	assert.Equal(t, "ACTIVE", body["status"])

	require.Len(t, rig.service.declared, 1)
	decl := rig.service.declared[0]
	assert.Equal(t, rig.user.String(), decl.DeclaredBy)
	assert.Equal(t, []uuid.UUID{agent}, decl.AffectedAgents)
	assert.Equal(t, time.Date(2030, 1, 1, 0, 30, 0, 0, time.UTC), decl.ExpiresAt.UTC())

	require.Len(t, *rig.audit, 1)
	row := (*rig.audit)[0]
	assert.Equal(t, "pam_emergency", row.resourceType)
	assert.Equal(t, "INC-42", row.metadata["incidentId"])
	assert.Equal(t, 1, row.metadata["affectedAgents"])

	status, body = rig.do(t, "POST", "/pam/emergencies",
		`{"incidentId":"INC-42","justification":"x","affectedAgents":["agent-1"],"grantedCaps":["db.write"]}`)
	assert.Equal(t, fiber.StatusBadRequest, status)
	assert.Equal(t, "affectedAgents must hold agent IDs", body["error"])
	assert.Len(t, rig.service.declared, 1)
}

func TestPAMHandler_ExtendAndRevokeEmergency(t *testing.T) {
	newDecl := func(rig *pamRig, org uuid.UUID) *application.EmergencyDeclaration {
		d := &application.EmergencyDeclaration{ID: uuid.New(), OrganizationID: org, IncidentID: "INC-9", Status: "ACTIVE"}
		rig.service.emergencies[d.ID] = d
		return d
	}

	t.Run("the declarer cannot extend it", func(t *testing.T) {
		rig := newPAMRig(t)
		d := newDecl(rig, rig.org)
		rig.service.err = application.ErrPAMSameUser
		status, body := rig.do(t, "POST", "/pam/emergencies/"+d.ID.String()+"/extend", "")
		assert.Equal(t, fiber.StatusForbidden, status)
		assert.Equal(t, "the user who declared an emergency cannot extend it", body["error"])
	})

	t.Run("a second extension", func(t *testing.T) {
		rig := newPAMRig(t)
		d := newDecl(rig, rig.org)
		rig.service.err = application.ErrPAMEmergencyExtended
		status, body := rig.do(t, "POST", "/pam/emergencies/"+d.ID.String()+"/extend", "")
		assert.Equal(t, fiber.StatusConflict, status)
		assert.Equal(t, "the emergency declaration was already extended", body["error"])
	})

	t.Run("an extension by another user", func(t *testing.T) {
		rig := newPAMRig(t)
		d := newDecl(rig, rig.org)
		status, _ := rig.do(t, "POST", "/pam/emergencies/"+d.ID.String()+"/extend", "")
		assert.Equal(t, fiber.StatusOK, status)
		assert.Equal(t, []string{d.ID.String(), rig.user.String()}, rig.service.extended)
		require.Len(t, *rig.audit, 1)
		assert.Equal(t, domain.AuditActionUpdate, (*rig.audit)[0].action)
	})

	t.Run("a declaration of another organization", func(t *testing.T) {
		rig := newPAMRig(t)
		d := newDecl(rig, uuid.New())
		for _, action := range []string{"extend", "revoke"} {
			status, _ := rig.do(t, "POST", "/pam/emergencies/"+d.ID.String()+"/"+action, "")
			assert.Equal(t, fiber.StatusNotFound, status, action)
		}
		assert.Empty(t, rig.service.extended)
		assert.Empty(t, rig.service.revoked)
	})

	t.Run("a revocation", func(t *testing.T) {
		rig := newPAMRig(t)
		d := newDecl(rig, rig.org)
		status, body := rig.do(t, "POST", "/pam/emergencies/"+d.ID.String()+"/revoke", "")
		assert.Equal(t, fiber.StatusOK, status)
		assert.Equal(t, "REVOKED", body["status"])
		require.Len(t, *rig.audit, 1)
		assert.Equal(t, domain.AuditActionRevoke, (*rig.audit)[0].action)
	})

	t.Run("a declaration that is not active", func(t *testing.T) {
		rig := newPAMRig(t)
		d := newDecl(rig, rig.org)
		rig.service.err = application.ErrPAMEmergencyNotActive
		status, body := rig.do(t, "POST", "/pam/emergencies/"+d.ID.String()+"/revoke", "")
		assert.Equal(t, fiber.StatusConflict, status)
		assert.Equal(t, "the emergency declaration is not active: it expired or was revoked", body["error"])
	})
}

func TestPAMHandler_CampaignsAndCertificationDecisions(t *testing.T) {
	rig := newPAMRig(t)
	status, body := rig.do(t, "POST", "/pam/campaigns", `{"name":"Q4 privileged review","scopeType":"TIER","scopeValue":"SUPER_PRIVILEGED"}`)
	require.Equal(t, fiber.StatusCreated, status, "body: %v", body)
	require.Len(t, rig.service.created, 1)
	assert.Equal(t, rig.user.String(), rig.service.created[0].CreatedBy)
	campaign := rig.service.created[0].ID

	agent := uuid.New()
	status, body = rig.do(t, "POST", "/pam/campaigns/"+campaign.String()+"/decisions",
		`{"agentId":"`+agent.String()+`","capability":"db.delete","decision":"REVOKED","note":"unused"}`)
	require.Equal(t, fiber.StatusOK, status, "body: %v", body)
	require.Len(t, rig.service.decisions, 1)
	d := rig.service.decisions[0]
	assert.Equal(t, campaign, d.CampaignID)
	assert.Equal(t, agent, d.AgentID)
	assert.Equal(t, rig.user.String(), d.ReviewerID)
	assert.Equal(t, "unused", d.ReviewerNote)

	status, body = rig.do(t, "GET", "/pam/campaigns/"+campaign.String()+"/decisions", "")
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, float64(1), body["total"])

	status, body = rig.do(t, "GET", "/pam/campaigns", "")
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, float64(1), body["total"])

	rig.service.err = application.ErrPAMCampaignNotActive
	status, body = rig.do(t, "POST", "/pam/campaigns/"+campaign.String()+"/decisions",
		`{"agentId":"`+agent.String()+`","capability":"db.delete","decision":"CERTIFIED"}`)
	assert.Equal(t, fiber.StatusConflict, status)
	assert.Equal(t, "the campaign is not active: it has not started or has closed", body["error"])
}

func TestPAMHandler_CampaignOfAnotherOrganizationIsNotFound(t *testing.T) {
	rig := newPAMRig(t)
	theirs := &application.CertificationCampaign{ID: uuid.New(), OrganizationID: uuid.New()}
	rig.service.campaigns[theirs.ID] = theirs

	status, _ := rig.do(t, "POST", "/pam/campaigns/"+theirs.ID.String()+"/decisions",
		`{"agentId":"`+uuid.NewString()+`","capability":"db.delete","decision":"CERTIFIED"}`)
	assert.Equal(t, fiber.StatusNotFound, status)
	status, _ = rig.do(t, "GET", "/pam/campaigns/"+theirs.ID.String()+"/decisions", "")
	assert.Equal(t, fiber.StatusNotFound, status)
	assert.Empty(t, rig.service.decisions)
}

func TestPAMHandler_ListTiers(t *testing.T) {
	rig := newPAMRig(t)
	status, body := rig.do(t, "GET", "/pam/tiers", "")
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, float64(1), body["total"])
	tier := body["tiers"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, "SUPER_PRIVILEGED", tier["tier"])
	assert.Equal(t, float64(900), tier["maxTtlSeconds"])
}
