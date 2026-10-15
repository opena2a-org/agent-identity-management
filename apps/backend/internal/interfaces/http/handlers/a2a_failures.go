package handlers

import (
	"errors"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
)

// a2aFailure names one way an A2A request fails. An A2A handler answers a
// failure by naming it; a2aAnswers holds the status and reason each one is
// answered with, and writeA2AFailure is the only place an A2A failure
// response is built.
//
// A request RequireOrganizationID or RequireOrgAndUserID refuses is not here:
// the helper's 401 is returned unchanged and the server's error handler
// writes it.
type a2aFailure int

const (
	// A path, query or body value the handler cannot read: 400.
	a2aInvalidAgentID a2aFailure = iota + 1
	a2aInvalidPeerID
	a2aInvalidTaskID
	a2aInvalidConsentID
	a2aInvalidTargetAgentID
	a2aInvalidClientAgentID
	a2aInvalidGrantorAgentID
	a2aInvalidRecipientAgentID
	a2aInvalidAttestedAgentID
	a2aInvalidAttestingAgentID
	a2aInvalidRequestingAgentID
	a2aInvalidOrMissingAgentID
	a2aInvalidBody
	a2aInvalidTaskState
	a2aCardSourceRequired
	a2aCardURLRefused
	a2aClientAgentRequired
	a2aClientAgentRequiredInBody
	a2aSearchQueryRequired
	a2aSearchQueryNotText
	a2aIntentQueryRequired
	a2aIntentRequired
	a2aConsentCheckParamsRequired
	a2aConsentScopeNotText
	a2aUserIDRequired
	a2aPublicCardAgentIDRequired
	a2aConsensusSkillIDRequired
	a2aSkillIDNotText
	a2aDeleteSkillIDRequired
	a2aDeleteSkillIDNotText

	// POST /api/v1/a2a/verify refused the request: 400 for a body or nonce
	// it cannot read, 401 for a request that does not verify.
	a2aVerifyInvalidBody
	a2aVerifyInvalidNonce
	a2aVerifyRefused

	// The caller cannot be told apart: 401.
	a2aAttestingAgentUnknown
	a2aOrganizationUnknown
	a2aOrganizationRequired
	a2aAgentIDRequired

	// 403.
	a2aAccessDenied
	a2aPolicyOtherOrganization

	// 404.
	a2aAgentNotFound
	a2aAgentCardNotFound
	a2aNotFound
	a2aGrantorNotFound
	a2aNoAgentForIntent

	// 405.
	a2aTrustScoreNotWritable

	// The server failed: 500. Each body is a fixed line; the cause goes to
	// the server log.
	a2aServerError
	a2aTaskNotLogged
	a2aCardNotSerialized
	a2aTrustScoreNotComputed
	a2aCapableAgentsNotFound
	a2aViolationsNotRead

	a2aFailureEnd // not a failure: one past the last
)

// a2aAnswer is how one failure is answered.
type a2aAnswer struct {
	status int
	reason string
	// verdict marks a refusal of POST /api/v1/a2a/verify: its body also
	// carries valid: false. That route's 500 carries the fixed line only,
	// since it says nothing about the request.
	verdict bool
	// allow and members complete the 405 for a write the route does not take.
	allow   string
	members fiber.Map
}

// agentCardURLRefusedMessage answers a card URL the server refuses to fetch.
// It names no host or address: the card service's reason says what the URL
// resolved to, and that stays in the server log.
const agentCardURLRefusedMessage = "cardUrl must be a public http or https URL"

// evaluatePolicyOtherOrganizationMessage answers a policy evaluation whose body
// names an organization other than the caller's. It names neither organization.
const evaluatePolicyOtherOrganizationMessage = "organizationId must be your own organization; leave it out to evaluate your organization's policies"

var a2aAnswers = map[a2aFailure]a2aAnswer{
	a2aInvalidAgentID:             {status: fiber.StatusBadRequest, reason: "Invalid agent ID"},
	a2aInvalidPeerID:              {status: fiber.StatusBadRequest, reason: "Invalid peer ID"},
	a2aInvalidTaskID:              {status: fiber.StatusBadRequest, reason: "Invalid task ID"},
	a2aInvalidConsentID:           {status: fiber.StatusBadRequest, reason: "Invalid consent ID"},
	a2aInvalidTargetAgentID:       {status: fiber.StatusBadRequest, reason: "Invalid target agent ID"},
	a2aInvalidClientAgentID:       {status: fiber.StatusBadRequest, reason: "Invalid client agent ID"},
	a2aInvalidGrantorAgentID:      {status: fiber.StatusBadRequest, reason: "Invalid grantor agent ID"},
	a2aInvalidRecipientAgentID:    {status: fiber.StatusBadRequest, reason: "Invalid recipient agent ID"},
	a2aInvalidAttestedAgentID:     {status: fiber.StatusBadRequest, reason: "Invalid attested agent ID"},
	a2aInvalidAttestingAgentID:    {status: fiber.StatusBadRequest, reason: "Invalid attesting agent ID"},
	a2aInvalidRequestingAgentID:   {status: fiber.StatusBadRequest, reason: "Invalid requesting agent ID"},
	a2aInvalidOrMissingAgentID:    {status: fiber.StatusBadRequest, reason: "Invalid or missing agent ID"},
	a2aInvalidBody:                {status: fiber.StatusBadRequest, reason: "Invalid request body"},
	a2aInvalidTaskState:           {status: fiber.StatusBadRequest, reason: "Invalid task state"},
	a2aCardSourceRequired:         {status: fiber.StatusBadRequest, reason: "cardUrl or cardData is required"},
	a2aCardURLRefused:             {status: fiber.StatusBadRequest, reason: agentCardURLRefusedMessage},
	a2aClientAgentRequired:        {status: fiber.StatusBadRequest, reason: "Client agent ID required - provide in request body or context"},
	a2aClientAgentRequiredInBody:  {status: fiber.StatusBadRequest, reason: "Client agent ID required"},
	a2aSearchQueryRequired:        {status: fiber.StatusBadRequest, reason: "Query parameter 'q' is required"},
	a2aSearchQueryNotText:         {status: fiber.StatusBadRequest, reason: queryTextMessage("q")},
	a2aIntentQueryRequired:        {status: fiber.StatusBadRequest, reason: "Query parameter 'intent' is required"},
	a2aIntentRequired:             {status: fiber.StatusBadRequest, reason: "Intent is required"},
	a2aConsentCheckParamsRequired: {status: fiber.StatusBadRequest, reason: "userId, grantorAgentId, recipientAgentId, and scope are required"},
	a2aConsentScopeNotText:        {status: fiber.StatusBadRequest, reason: queryTextMessage("scope")},
	a2aUserIDRequired:             {status: fiber.StatusBadRequest, reason: "User ID is required"},
	a2aPublicCardAgentIDRequired:  {status: fiber.StatusBadRequest, reason: "Agent ID is required (query param 'agentId' or header 'X-Agent-ID')"},
	a2aConsensusSkillIDRequired:   {status: fiber.StatusBadRequest, reason: "Skill ID is required"},
	a2aSkillIDNotText:             {status: fiber.StatusBadRequest, reason: queryTextMessage("skillId")},
	a2aDeleteSkillIDRequired:      {status: fiber.StatusBadRequest, reason: "Skill ID required"},
	a2aDeleteSkillIDNotText:       {status: fiber.StatusBadRequest, reason: queryTextMessage("id")},

	a2aVerifyInvalidBody:  {status: fiber.StatusBadRequest, reason: "Invalid request body", verdict: true},
	a2aVerifyInvalidNonce: {status: fiber.StatusBadRequest, reason: application.InvalidA2ANonceMessage, verdict: true},
	a2aVerifyRefused:      {status: fiber.StatusUnauthorized, verdict: true},

	a2aAttestingAgentUnknown: {status: fiber.StatusUnauthorized, reason: "Could not determine attesting agent - provide attestingAgentId in request"},
	a2aOrganizationUnknown:   {status: fiber.StatusUnauthorized, reason: "Could not determine organization"},
	a2aOrganizationRequired:  {status: fiber.StatusUnauthorized, reason: "Organization ID required"},
	a2aAgentIDRequired:       {status: fiber.StatusUnauthorized, reason: "Agent ID required"},

	a2aAccessDenied:            {status: fiber.StatusForbidden, reason: "Access denied"},
	a2aPolicyOtherOrganization: {status: fiber.StatusForbidden, reason: evaluatePolicyOtherOrganizationMessage},

	a2aAgentNotFound:     {status: fiber.StatusNotFound, reason: "Agent not found"},
	a2aAgentCardNotFound: {status: fiber.StatusNotFound, reason: "Agent card not found"},
	a2aNotFound:          {status: fiber.StatusNotFound, reason: "not found"},
	a2aGrantorNotFound:   {status: fiber.StatusNotFound, reason: "Grantor agent not found"},
	a2aNoAgentForIntent:  {status: fiber.StatusNotFound, reason: "No agent found matching intent"},

	a2aTrustScoreNotWritable: {
		status: fiber.StatusMethodNotAllowed,
		reason: A2ATrustScoreNotAsserted,
		allow:  fiber.MethodGet,
		members: fiber.Map{
			"read":   "GET /api/v1/a2a/trust/:id",
			"record": "POST /api/v1/a2a/trust/:id/interaction",
		},
	},

	a2aServerError:           {status: fiber.StatusInternalServerError, reason: ServerErrorMessage},
	a2aTaskNotLogged:         {status: fiber.StatusInternalServerError, reason: "Failed to log task"},
	a2aCardNotSerialized:     {status: fiber.StatusInternalServerError, reason: "Failed to serialize agent card"},
	a2aTrustScoreNotComputed: {status: fiber.StatusInternalServerError, reason: "Failed to compute trust score"},
	a2aCapableAgentsNotFound: {status: fiber.StatusInternalServerError, reason: "Failed to find capable agents"},
	a2aViolationsNotRead:     {status: fiber.StatusInternalServerError, reason: "Failed to get security violations"},
}

// respondA2AFailure answers kind with its status and {"error": reason}.
func respondA2AFailure(c fiber.Ctx, kind a2aFailure) error {
	return writeA2AFailure(c, kind, "", nil)
}

// respondA2AFailureCause answers a 5xx kind as respondA2AFailure does and
// writes cause to the server log. The cause is never part of the body: a
// repository error names tables, constraints and query fragments.
func respondA2AFailureCause(c fiber.Ctx, kind a2aFailure, cause error) error {
	log.Printf("server error [%d] %s %s: %v", a2aAnswers[kind].status, c.Method(), c.Path(), cause)
	return writeA2AFailure(c, kind, "", nil)
}

// respondA2AVerifyRefused answers a request POST /api/v1/a2a/verify read and
// did not verify: 401, valid: false and the verification's reason, one of the
// fixed lines the service states, with its reasonCode when the service names
// one.
func respondA2AVerifyRefused(c fiber.Ctx, reason, reasonCode string) error {
	var members fiber.Map
	if reasonCode != "" {
		members = fiber.Map{"reasonCode": reasonCode}
	}
	return writeA2AFailure(c, a2aVerifyRefused, reason, members)
}

// respondA2ANoAgentForIntent answers GET /api/v1/a2a/route when no agent
// matches intent: 404, the intent asked for and no alternatives.
func respondA2ANoAgentForIntent(c fiber.Ctx, intent string) error {
	return writeA2AFailure(c, a2aNoAgentForIntent, "", fiber.Map{"intent": intent, "alternatives": 0})
}

// respondAgentCardError answers a card registration error. A card URL the
// server refuses by policy is a problem with the caller's input, so it is
// answered with 400 and a fixed reason. Fetch, parse and storage failures are
// answered with the fixed 5xx line. Both keep the card service's reason in the
// server log only.
func respondAgentCardError(c fiber.Ctx, agentID uuid.UUID, err error) error {
	if errors.Is(err, application.ErrAgentCardURLRefused) {
		log.Printf("a2a: agent card URL refused for agent %s: %v", agentID, err)
		return respondA2AFailure(c, a2aCardURLRefused)
	}
	return respondA2AFailureCause(c, a2aServerError, fmt.Errorf("agent %s: %w", agentID, err))
}

// writeA2AFailure writes kind's answer. reason, when set, states the reason a
// kind without a fixed one carries; members join the body.
func writeA2AFailure(c fiber.Ctx, kind a2aFailure, reason string, members fiber.Map) error {
	answer, ok := a2aAnswers[kind]
	if !ok {
		return respondServerError(c, fiber.StatusInternalServerError, fmt.Errorf("a2a failure %d has no answer", kind))
	}
	if reason == "" {
		reason = answer.reason
	}
	body := fiber.Map{"error": reason}
	if answer.verdict {
		body["valid"] = false
	}
	for name, value := range answer.members {
		body[name] = value
	}
	for name, value := range members {
		body[name] = value
	}
	if answer.allow != "" {
		c.Set(fiber.HeaderAllow, answer.allow)
	}
	return c.Status(answer.status).JSON(body)
}
