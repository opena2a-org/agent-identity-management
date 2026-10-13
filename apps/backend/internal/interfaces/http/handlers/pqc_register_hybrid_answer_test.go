package handlers

import (
	"context"
	"encoding/base64"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/pqc"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// POST /agents/:id/pqc-key answers the hybrid mode it stored. The agent the
// handler loads before the store holds the opposite value in every case here,
// so an answer read from that agent disagrees with what was stored.
func TestRegisterPQCKeyAnswersTheHybridModeItStored(t *testing.T) {
	for _, tc := range []struct {
		role         domain.UserRole
		enableHybrid bool
	}{
		{domain.RoleManager, true},
		{domain.RoleAdmin, true},
		{domain.RoleManager, false},
		{domain.RoleMember, false},
	} {
		t.Run(string(tc.role)+" enableHybrid "+strconv.FormatBool(tc.enableHybrid), func(t *testing.T) {
			kp, err := pqc.GenerateMLDSAKeyPair(pqc.AlgorithmMLDSA65)
			require.NoError(t, err)

			orgID := uuid.New()
			var stored *bool
			h := &AgentHandler{
				agentService: &MockAgentServiceImpl{
					GetAgentFunc: func(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
						return &domain.Agent{
							ID: id, OrganizationID: orgID, Name: "pqc-agent",
							HybridModeEnabled: !tc.enableHybrid,
						}, nil
					},
					UpdateAgentPQCKeyFunc: func(_ context.Context, _ uuid.UUID, _, _ string, hybrid bool) error {
						stored = &hybrid
						return nil
					},
				},
				auditService: &MockAuditServiceImpl{},
			}

			status, body := postAsRole(t, h.RegisterPQCKey, orgID, string(tc.role), map[string]interface{}{
				"publicKey":    base64.StdEncoding.EncodeToString(kp.PublicKey),
				"algorithm":    "ML-DSA-65",
				"enableHybrid": tc.enableHybrid,
			})

			require.Equal(t, fiber.StatusOK, status, "response: %v", body)
			require.NotNil(t, stored, "the key must be stored")
			assert.Equal(t, tc.enableHybrid, *stored, "the request's enableHybrid is stored")
			assert.Equal(t, *stored, body["hybridModeEnabled"], "the answer reports the stored hybrid mode")
		})
	}
}
