package handlers

import (
	"context"
	"io"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The five A2A write handlers that resolve the ACTING agent out of
// the request body (or, failing that, out of the context principal) scope
// that agent to the caller's organization before any store is touched, with
// the read side's not-found shape. Subject-side identifiers (the attested
// agent, the security-check target) stay cross-organization by protocol.

// bodyWriterDouble stands in for A2AService on the five write paths. Every
// call is recorded so a test can prove the service was, or was never, reached.
type bodyWriterDouble struct {
	registeredSkill   *domain.A2ASkill
	attestReq         *application.AttestSkillRequest
	securityReq       *domain.A2ASecurityCheckRequest
	signedAgentID     *uuid.UUID
	registeredCardReq *application.RegisterAgentCardRequest
}

func (d *bodyWriterDouble) RegisterSkill(_ context.Context, skill *domain.A2ASkill) error {
	d.registeredSkill = skill
	return nil
}

func (d *bodyWriterDouble) AttestSkill(_ context.Context, req application.AttestSkillRequest) (*domain.A2AAgentAttestation, error) {
	d.attestReq = &req
	return &domain.A2AAgentAttestation{ID: uuid.New(), AttestingAgentID: req.AttestingAgentID, AttestedAgentID: req.AttestedAgentID, SkillID: req.SkillID}, nil
}

func (d *bodyWriterDouble) CheckA2ASecurity(_ context.Context, req *domain.A2ASecurityCheckRequest) (*domain.A2ASecurityCheckResult, error) {
	d.securityReq = req
	return &domain.A2ASecurityCheckResult{Allowed: true}, nil
}

func (d *bodyWriterDouble) SignA2ARequest(_ context.Context, agentID uuid.UUID, _, _ string, _ []byte) (*domain.A2ARequestSignature, error) {
	d.signedAgentID = &agentID
	return &domain.A2ARequestSignature{AgentID: agentID, Signature: "sig"}, nil
}

func (d *bodyWriterDouble) RegisterAgentCard(_ context.Context, req application.RegisterAgentCardRequest) (*application.RegisterAgentCardResponse, error) {
	d.registeredCardReq = &req
	return &application.RegisterAgentCardResponse{}, nil
}

func (d *bodyWriterDouble) reached() bool {
	return d.registeredSkill != nil || d.attestReq != nil || d.securityReq != nil || d.signedAgentID != nil || d.registeredCardReq != nil
}

// bodyChannelFixture: the caller's organization owns ownedAgent; every other
// agent id the loader sees belongs to victimOrg.
type bodyChannelFixture struct {
	callerOrg, victimOrg uuid.UUID
	ownedAgent           uuid.UUID
	foreignAgent         uuid.UUID
	writer               *bodyWriterDouble
	handler              *A2AHandler
	loaded               []uuid.UUID
}

func newBodyChannelFixture(withWriter bool) *bodyChannelFixture {
	f := &bodyChannelFixture{callerOrg: uuid.New(), victimOrg: uuid.New(), ownedAgent: uuid.New(), foreignAgent: uuid.New()}
	agentSvc := &MockAgentServiceImpl{
		GetAgentFunc: func(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
			f.loaded = append(f.loaded, id)
			org := f.victimOrg
			if id == f.ownedAgent {
				org = f.callerOrg
			}
			return &domain.Agent{ID: id, OrganizationID: org, Name: "agent"}, nil
		},
	}
	// a2aService stays nil in every fixture: a bypass surfaces as a 500 from
	// the nil dereference (or a 200 through the double), never as the
	// asserted 404.
	f.handler = &A2AHandler{a2aService: nil, agentService: agentSvc}
	if withWriter {
		f.writer = &bodyWriterDouble{}
		f.handler.a2aWriter = f.writer
	}
	return f
}

type bodyChannelSite struct {
	name    string
	route   string
	handler func(*A2AHandler) fiber.Handler
	// body renders the request for the given ACTOR id ("" = omit the actor
	// field so the handler falls back to the context principal); the subject
	// id, where the endpoint has one, is always the foreign agent.
	body func(actor string, foreign uuid.UUID) string
	// reached reports whether the double saw the call, and with which actor.
	reached func(*bodyWriterDouble) (bool, uuid.UUID)
	success int
}

var bodyChannelSites = []bodyChannelSite{
	{
		name: "RegisterSkill", route: "/a2a/skills",
		handler: func(h *A2AHandler) fiber.Handler { return h.RegisterSkill },
		body: func(actor string, _ uuid.UUID) string {
			if actor == "" {
				return `{"id":"skill-1","name":"Skill"}`
			}
			return `{"agentId":"` + actor + `","id":"skill-1","name":"Skill"}`
		},
		reached: func(d *bodyWriterDouble) (bool, uuid.UUID) {
			if d.registeredSkill == nil {
				return false, uuid.Nil
			}
			return true, d.registeredSkill.AgentID
		},
		success: fiber.StatusCreated,
	},
	{
		name: "AttestSkill", route: "/a2a/attestations",
		handler: func(h *A2AHandler) fiber.Handler { return h.AttestSkill },
		body: func(actor string, foreign uuid.UUID) string {
			attested := `"attestedAgentId":"` + foreign.String() + `","skillId":"skill-1","attestationType":"peer","confidence":0.9`
			if actor == "" {
				return `{` + attested + `}`
			}
			return `{"attestingAgentId":"` + actor + `",` + attested + `}`
		},
		reached: func(d *bodyWriterDouble) (bool, uuid.UUID) {
			if d.attestReq == nil {
				return false, uuid.Nil
			}
			return true, d.attestReq.AttestingAgentID
		},
		success: fiber.StatusCreated,
	},
	{
		name: "CheckSecurity", route: "/a2a/security/check",
		handler: func(h *A2AHandler) fiber.Handler { return h.CheckSecurity },
		body: func(actor string, foreign uuid.UUID) string {
			target := `"targetAgentId":"` + foreign.String() + `","skillId":"skill-1"`
			if actor == "" {
				return `{` + target + `}`
			}
			return `{"requestingAgentId":"` + actor + `",` + target + `}`
		},
		reached: func(d *bodyWriterDouble) (bool, uuid.UUID) {
			if d.securityReq == nil {
				return false, uuid.Nil
			}
			return true, d.securityReq.RequestingAgentID
		},
		success: fiber.StatusOK,
	},
	{
		name: "SignRequestAlt", route: "/a2a/sign",
		handler: func(h *A2AHandler) fiber.Handler { return h.SignRequestAlt },
		body: func(actor string, _ uuid.UUID) string {
			if actor == "" {
				return `{"method":"POST","path":"/x","body":"{}"}`
			}
			return `{"agentId":"` + actor + `","method":"POST","path":"/x","body":"{}"}`
		},
		reached: func(d *bodyWriterDouble) (bool, uuid.UUID) {
			if d.signedAgentID == nil {
				return false, uuid.Nil
			}
			return true, *d.signedAgentID
		},
		success: fiber.StatusOK,
	},
	{
		name: "RegisterAgentCardAlt", route: "/a2a/cards",
		handler: func(h *A2AHandler) fiber.Handler { return h.RegisterAgentCardAlt },
		body: func(actor string, _ uuid.UUID) string {
			if actor == "" {
				return `{"cardUrl":"https://agent.example.com/.well-known/agent.json"}`
			}
			return `{"agentId":"` + actor + `","cardUrl":"https://agent.example.com/.well-known/agent.json"}`
		},
		reached: func(d *bodyWriterDouble) (bool, uuid.UUID) {
			if d.registeredCardReq == nil {
				return false, uuid.Nil
			}
			return true, d.registeredCardReq.AgentID
		},
		success: fiber.StatusCreated,
	},
}

func postBodyChannel(t *testing.T, site bodyChannelSite, h *A2AHandler, locals func(fiber.Ctx), body string) (int, string) {
	t.Helper()
	// The package's test app preserves a response the handler already wrote
	// when it returns ErrUnauthorized, which is how the 401 the guard writes
	// is observable here (the default fiber error handler would overwrite it).
	app := newTestApp()
	app.Post(site.route, func(c fiber.Ctx) error {
		locals(c)
		return site.handler(h)(c)
	})
	req := httptest.NewRequest("POST", site.route, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

func TestA2ABodyChannel_SameOrgActorReachesTheService(t *testing.T) {
	for _, site := range bodyChannelSites {
		t.Run(site.name+" same-org actor, foreign subject", func(t *testing.T) {
			f := newBodyChannelFixture(true)
			status, body := postBodyChannel(t, site, f.handler, func(c fiber.Ctx) {
				c.Locals("organization_id", f.callerOrg)
				c.Locals("agent_id", f.ownedAgent)
			}, site.body(f.ownedAgent.String(), f.foreignAgent))
			assert.Equal(t, site.success, status, "body: %s", body)
			reached, actor := site.reached(f.writer)
			assert.True(t, reached, "the service must be reached for an owned actor")
			assert.Equal(t, f.ownedAgent, actor, "the service must see the resolved actor")
		})
	}
}

func TestA2ABodyChannel_CrossOrgActorInTheBodyIsRefused(t *testing.T) {
	for _, site := range bodyChannelSites {
		t.Run(site.name+" foreign actor supplied in the body", func(t *testing.T) {
			f := newBodyChannelFixture(false)
			status, body := postBodyChannel(t, site, f.handler, func(c fiber.Ctx) {
				c.Locals("organization_id", f.callerOrg)
				c.Locals("agent_id", f.ownedAgent) // the attacker's own agent is in the principal
			}, site.body(f.foreignAgent.String(), f.foreignAgent))
			assert.Equal(t, fiber.StatusNotFound, status, "body: %s", body)
			assert.JSONEq(t, `{"error":"not found"}`, body, "the refusal is silent about existence")
			assert.NotContains(t, body, f.foreignAgent.String())
			assert.NotContains(t, body, f.victimOrg.String())
			assert.Contains(t, f.loaded, f.foreignAgent, "the guard must load the resolved actor")
		})
	}
}

func TestA2ABodyChannel_ContextFallbackIsStillChecked(t *testing.T) {
	for _, site := range bodyChannelSites {
		t.Run(site.name+" absent actor field, owned principal", func(t *testing.T) {
			f := newBodyChannelFixture(true)
			status, body := postBodyChannel(t, site, f.handler, func(c fiber.Ctx) {
				c.Locals("organization_id", f.callerOrg)
				c.Locals("agent_id", f.ownedAgent)
			}, site.body("", f.foreignAgent))
			assert.Equal(t, site.success, status, "body: %s", body)
			reached, actor := site.reached(f.writer)
			assert.True(t, reached)
			assert.Equal(t, f.ownedAgent, actor)
		})
		t.Run(site.name+" absent actor field, foreign principal is refused", func(t *testing.T) {
			f := newBodyChannelFixture(false)
			status, body := postBodyChannel(t, site, f.handler, func(c fiber.Ctx) {
				c.Locals("organization_id", f.callerOrg)
				c.Locals("agent_id", f.foreignAgent)
			}, site.body("", f.foreignAgent))
			assert.Equal(t, fiber.StatusNotFound, status, "body: %s", body)
			assert.JSONEq(t, `{"error":"not found"}`, body)
		})
	}
}

func TestA2ABodyChannel_NoOrganizationInContextIs401(t *testing.T) {
	for _, site := range bodyChannelSites {
		t.Run(site.name, func(t *testing.T) {
			f := newBodyChannelFixture(true)
			status, body := postBodyChannel(t, site, f.handler, func(c fiber.Ctx) {
				c.Locals("agent_id", f.ownedAgent)
			}, site.body(f.ownedAgent.String(), f.foreignAgent))
			assert.Equal(t, fiber.StatusUnauthorized, status, "body: %s", body)
			assert.JSONEq(t, `{"error":"Organization ID not found in context"}`, body)
			assert.False(t, f.writer.reached(), "no organization, no write")
		})
	}
}

func TestCapabilityHandler_HasNoVerifyActionMethod(t *testing.T) {
	// The unmounted request-body handler is gone from the tree; the service
	// method of the same name (CapabilityService.VerifyAction) is untouched.
	_, found := reflect.TypeOf(&CapabilityHandler{}).MethodByName("VerifyAction")
	assert.False(t, found, "CapabilityHandler.VerifyAction must be absent: it took agentId from the body with no organization predicate and no route served it")
}
