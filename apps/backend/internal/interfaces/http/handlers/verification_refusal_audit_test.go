package handlers

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// A suspended agent that signs a verification request with its registered key is
// refused 403 whatever its organisation's enforcement mode, and the refusal leaves one
// audit entry. The entry is written only after the signature verifies, so a caller
// without the agent's key cannot create one, and it records the refusal without the
// request's signature, key, message or context.
//
// Before, the refusal wrote nothing: an organisation that suspended an agent could not
// see that the agent kept trying.

// Written out as literals so this file states the contract independently of the
// declarations it checks.
const (
	suspendedVerificationBody = `{"error":"Agent status is suspended, cannot perform actions"}`
	verificationRefusedAction = domain.AuditAction("verification_refused")
)

// refusalAuditApp mounts CreateVerification bare, as cmd/server does on the SDK API,
// with every audit entry captured. reached is set when a request gets past
// authentication to the capability check.
type refusalAuditApp struct {
	app     *fiber.App
	logged  []*domain.AuditLog
	reached bool
}

// newRefusalAuditApp answers the organisation lookup with mode, or with an error when
// orgErr is set, and the audit write with logErr.
func newRefusalAuditApp(mode domain.EnforcementMode, orgErr, logErr error, agents ...*domain.Agent) *refusalAuditApp {
	ra := &refusalAuditApp{}
	lookup := agentsByID(agents...)
	svc := &MockAgentServiceForVerificationImpl{}
	svc.GetAgentFunc = func(_ context.Context, id uuid.UUID) (*domain.Agent, error) { return lookup(id) }
	svc.VerifyCapabilityFunc = func(context.Context, uuid.UUID, string, string, map[string]interface{}, string) (bool, string, uuid.UUID, error) {
		ra.reached = true
		return false, "stopped by the test", uuid.Nil, errors.New("stopped by the test")
	}
	audit := &MockAuditServiceForVerificationImpl{
		LogFunc: func(_ context.Context, entry *domain.AuditLog) error {
			ra.logged = append(ra.logged, entry)
			return logErr
		},
	}
	orgs := &MockOrganizationRepositoryerImpl{
		GetByIDFunc: func(id uuid.UUID) (*domain.Organization, error) {
			if orgErr != nil {
				return nil, orgErr
			}
			return &domain.Organization{ID: id, EnforcementMode: mode}, nil
		},
	}
	h := NewVerificationHandlerWithInterfaces(svc, audit, &MockAlertServiceForVerificationImpl{},
		&MockVerificationEventServiceForVerificationImpl{}, orgs)
	ra.app = fiber.New()
	ra.app.Post("/api/v1/sdk-api/verifications", h.CreateVerification)
	return ra
}

// refusals returns the captured entries whose action is the verification refusal.
func (ra *refusalAuditApp) refusals() []*domain.AuditLog {
	var out []*domain.AuditLog
	for _, entry := range ra.logged {
		if entry.Action == verificationRefusedAction {
			out = append(out, entry)
		}
	}
	return out
}

func suspendedAgentWithCreator(pub ed25519.PublicKey) *domain.Agent {
	agent := refusalAgent(domain.AgentStatusSuspended, pub)
	agent.CreatedBy = uuid.New()
	return agent
}

func TestCreateVerification_ASuspendedAgentIsRefusedInBothModesAndTheRefusalIsAudited(t *testing.T) {
	for _, cell := range []struct {
		name     string
		mode     domain.EnforcementMode
		orgErr   error
		wantMode string
	}{
		{name: "strict", mode: domain.EnforcementModeStrict, wantMode: "strict"},
		{name: "monitoring", mode: domain.EnforcementModeMonitoring, wantMode: "monitoring"},
		// The refusal does not depend on the mode: an organisation that does not load
		// still gets the refusal, recorded with the mode unknown.
		{name: "organisation lookup fails", orgErr: errors.New("organisation lookup failed"), wantMode: "unknown"},
	} {
		t.Run(cell.name, func(t *testing.T) {
			pub, priv := mustKeypair(t)
			suspended := suspendedAgentWithCreator(pub)
			ra := newRefusalAuditApp(cell.mode, cell.orgErr, nil, suspended)

			code, body := createVerificationCase{agentID: suspended.ID, signer: priv, presented: pub}.do(t, ra.app)
			require.Equal(t, fiber.StatusForbidden, code, body)
			require.Equal(t, suspendedVerificationBody, body)
			require.False(t, ra.reached, "a suspended agent must not reach the capability check")

			require.Len(t, ra.logged, 1, "the refusal must write exactly one audit entry")
			entry := ra.logged[0]
			require.Equal(t, verificationRefusedAction, entry.Action)
			require.Equal(t, suspended.OrganizationID, entry.OrganizationID)
			require.NotNil(t, entry.UserID)
			require.Equal(t, suspended.CreatedBy, *entry.UserID)
			require.NotNil(t, entry.AgentID)
			require.Equal(t, suspended.ID, *entry.AgentID)
			require.Equal(t, "agent_action", entry.ResourceType)
			require.Equal(t, suspended.ID, entry.ResourceID)
			require.NotEmpty(t, entry.IPAddress)

			keys := make([]string, 0, len(entry.Metadata))
			for k := range entry.Metadata {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			require.Equal(t, []string{"agentStatus", "capability", "enforcementMode", "httpStatus", "refusal", "resource"}, keys)
			require.Equal(t, "agent_status", entry.Metadata["refusal"])
			require.Equal(t, "suspended", entry.Metadata["agentStatus"])
			require.Equal(t, cell.wantMode, entry.Metadata["enforcementMode"])
			require.Equal(t, "read_file", entry.Metadata["capability"])
			require.Equal(t, "/tmp/x", entry.Metadata["resource"])
			require.Equal(t, fiber.StatusForbidden, entry.Metadata["httpStatus"])
		})
	}
}

func TestCreateVerification_TheRefusalEntryCarriesNoSignatureKeyMessageOrContext(t *testing.T) {
	pub, priv := mustKeypair(t)
	suspended := suspendedAgentWithCreator(pub)
	ra := newRefusalAuditApp(domain.EnforcementModeStrict, nil, nil, suspended)

	code, body := createVerificationCase{agentID: suspended.ID, signer: priv, presented: pub}.do(t, ra.app)
	require.Equal(t, fiber.StatusForbidden, code, body)
	require.Len(t, ra.logged, 1, "the refusal must write exactly one audit entry")
	metadata := ra.logged[0].Metadata

	for _, forbidden := range []string{"signature", "publicKey", "message", "context"} {
		require.NotContains(t, metadata, forbidden)
	}
	for k, v := range metadata {
		if k == "capability" || k == "resource" {
			continue
		}
		if s, ok := v.(string); ok {
			require.LessOrEqual(t, len(s), 64, "metadata %q carries a value-shaped string", k)
		}
	}
	raw, err := json.Marshal(ra.logged[0])
	require.NoError(t, err)
	require.NotContains(t, string(raw), base64.StdEncoding.EncodeToString(pub))
}

func TestCreateVerification_AFailedRefusalAuditWriteStillRefuses(t *testing.T) {
	pub, priv := mustKeypair(t)
	suspended := suspendedAgentWithCreator(pub)
	ra := newRefusalAuditApp(domain.EnforcementModeMonitoring, nil, errors.New("audit store unavailable"), suspended)

	code, body := createVerificationCase{agentID: suspended.ID, signer: priv, presented: pub}.do(t, ra.app)
	require.Equal(t, fiber.StatusForbidden, code, body)
	require.Equal(t, suspendedVerificationBody, body)
	require.False(t, ra.reached)
}

// Guard: a caller without the suspended agent's key creates no audit entry, so the
// audit log cannot be filled by anyone who knows only an agent id.
func TestCreateVerification_ACallerWithoutTheKeyWritesNoRefusalEntry(t *testing.T) {
	pub, priv := mustKeypair(t)
	otherPub, otherPriv := mustKeypair(t)
	suspended := suspendedAgentWithCreator(pub)

	for name, tc := range map[string]createVerificationCase{
		"a different key":       {agentID: suspended.ID, signer: otherPriv, presented: otherPub},
		"a flipped signature":   {agentID: suspended.ID, signer: priv, presented: pub, flip: true},
		"the key, another sign": {agentID: suspended.ID, signer: otherPriv, presented: pub},
	} {
		for _, mode := range []domain.EnforcementMode{domain.EnforcementModeStrict, domain.EnforcementModeMonitoring} {
			ra := newRefusalAuditApp(mode, nil, nil, suspended)
			code, body := tc.do(t, ra.app)
			require.Equal(t, fiber.StatusUnauthorized, code, "%s, %s: %s", name, mode, body)
			require.Empty(t, ra.logged, "%s, %s: a request that failed authentication wrote an audit entry", name, mode)
		}
	}
}

// Guard: an agent whose status permits it is not recorded as refused.
func TestCreateVerification_AnActiveAgentIsNotRecordedAsRefused(t *testing.T) {
	pub, priv := mustKeypair(t)
	verified := refusalAgent(domain.AgentStatusVerified, pub)
	ra := newRefusalAuditApp(domain.EnforcementModeStrict, nil, nil, verified)

	createVerificationCase{agentID: verified.ID, signer: priv, presented: pub}.do(t, ra.app)
	require.True(t, ra.reached, "the control must get past authentication")
	require.Empty(t, ra.refusals())
}
