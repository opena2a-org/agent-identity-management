package siem

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventFromSignInFailure_ForAnEmailThatMatchedAUser(t *testing.T) {
	userID, orgID := uuid.New(), uuid.New()
	at := time.Date(2026, 10, 16, 18, 4, 5, 6_000_000, time.FixedZone("MDT", -6*3600))

	event := EventFromSignInFailure(SignInFailure{
		Time:           at,
		Method:         "password",
		Reason:         "password_mismatch",
		Route:          "/api/v1/auth/login/local",
		IPAddress:      "192.0.2.10",
		UserAgent:      "client/1.0",
		UserID:         userID,
		OrganizationID: orgID,
	})

	assert.Equal(t, EventSignInFailed, event.EventType)
	assert.Equal(t, EventType("aim.auth.signin_failed"), event.EventType)
	assert.Equal(t, at.UTC(), event.Timestamp)
	assert.Equal(t, time.UTC, event.Timestamp.Location())
	assert.Equal(t, SeverityLow, event.Severity)
	assert.Equal(t, "opena2a-aim", event.Source)
	assert.Empty(t, event.AgentID)
	assert.Empty(t, event.Capability)
	assert.Empty(t, event.Outcome)
	assert.Equal(t, map[string]interface{}{
		"method":         "password",
		"reason":         "password_mismatch",
		"route":          "/api/v1/auth/login/local",
		"ipAddress":      "192.0.2.10",
		"userAgent":      "client/1.0",
		"userId":         userID.String(),
		"organizationId": orgID.String(),
	}, event.Details)
}

func TestEventFromSignInFailure_ForAnEmailThatMatchedNoUser(t *testing.T) {
	event := EventFromSignInFailure(SignInFailure{
		Time:   time.Now(),
		Method: "password",
		Reason: "unknown_email",
		Route:  "/api/v1/auth/login/local",
	})

	assert.Equal(t, map[string]interface{}{
		"method": "password",
		"reason": "unknown_email",
		"route":  "/api/v1/auth/login/local",
	}, event.Details, "no user, organization, address or client is named when none was given")

	raw, err := json.Marshal(event)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "userId")
	assert.NotContains(t, string(raw), "organizationId")
}

func TestEventFromFGADenial(t *testing.T) {
	agentID, orgID := uuid.New(), uuid.New()
	at := time.Date(2026, 10, 16, 18, 4, 5, 0, time.FixedZone("MDT", -6*3600))

	event := EventFromFGADenial(FGADenial{
		Time:           at,
		AgentID:        agentID,
		OrganizationID: orgID,
		Capability:     "files:delete",
		Outcome:        "DENY",
		DeniedBy:       "capability_check",
		DeniedReason:   "Agent does not have the requested capability",
	})

	assert.Equal(t, EventFGADenied, event.EventType)
	assert.Equal(t, EventType("aim.fga.denied"), event.EventType)
	assert.Equal(t, at.UTC(), event.Timestamp)
	assert.Equal(t, agentID.String(), event.AgentID)
	assert.Equal(t, "files:delete", event.Capability)
	assert.Equal(t, "DENY", event.Outcome)
	assert.Equal(t, SeverityMedium, event.Severity)
	assert.Equal(t, "opena2a-aim", event.Source)
	assert.Equal(t, map[string]interface{}{
		"organizationId": orgID.String(),
		"deniedBy":       "capability_check",
		"deniedReason":   "Agent does not have the requested capability",
	}, event.Details)
}

func TestEventFromFGADenial_WithoutTheAgentsOrganization(t *testing.T) {
	event := EventFromFGADenial(FGADenial{
		Time:         time.Now(),
		AgentID:      uuid.New(),
		Capability:   "files:read",
		Outcome:      "ERROR",
		DeniedReason: "failed to load FGA policy: connection refused",
	})

	assert.Equal(t, "ERROR", event.Outcome)
	assert.Equal(t, map[string]interface{}{
		"deniedBy":     "",
		"deniedReason": "failed to load FGA policy: connection refused",
	}, event.Details, "the organization is left out when the agent was not read")
}

// A sign-in failure reaches the SIEM layer with the address and user agent
// as the handler received them, which may be views of the request buffer
// (see requestView). The forwarder serializes the event after the request
// has ended, so the event copies both when it is built, inside the request.
func TestEventFromSignInFailure_CopiesTheAddressAndUserAgentOutOfTheRequestBuffer(t *testing.T) {
	view := newRequestView(t, "203.0.113.70", "client/1.0 (first request)")
	event := EventFromSignInFailure(SignInFailure{
		Time:      time.Now(),
		Method:    "password",
		Reason:    "password_mismatch",
		Route:     "/api/v1/auth/login/local",
		IPAddress: view.address,
		UserAgent: view.userAgent,
	})

	view.overwrite()

	assert.Equal(t, "203.0.113.70", event.Details["ipAddress"])
	assert.Equal(t, "client/1.0 (first request)", event.Details["userAgent"])
}
