package application

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/stretchr/testify/require"
)

// agreementAttestation is one row of the valid-attestations query used by these tests.
type agreementAttestation struct {
	agentID   uuid.UUID
	tools     []string
	connected bool
	at        time.Time
}

func newAgreementTestService(t *testing.T) (*MCPAttestationService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return &MCPAttestationService{
		attestationRepo: repository.NewMCPAttestationRepository(db),
		mcpRepo:         repository.NewMCPServerRepository(db),
	}, mock
}

// expectServerLookup answers MCPServerRepository.GetByID with a server in the given status.
func expectServerLookup(mock sqlmock.Sqlmock, serverID uuid.UUID, status domain.MCPServerStatus, confidence float64) {
	now := time.Now().UTC()
	mock.ExpectQuery(`FROM mcp_servers\s+WHERE id = \$1`).
		WithArgs(serverID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "organization_id", "name", "description", "url", "version",
			"public_key", "status", "is_verified", "last_verified_at", "verification_url",
			"capabilities", "trust_score", "registered_by_agent", "created_by", "created_at", "updated_at",
			"verification_method", "attestation_count", "confidence_score", "last_attested_at",
			"created_by_name", "created_by_email", "created_by_sdk_token_id", "created_by_api_key_id",
			"updated_by", "updated_by_name", "updated_by_email",
		}).AddRow(
			serverID, uuid.New(), "files", nil, "https://files.example.com", nil,
			nil, string(status), status == domain.MCPServerStatusVerified, nil, nil,
			[]byte("[]"), 0.5, nil, uuid.New(), now, now,
			"agent_attestation", 3, confidence, now,
			"", "", nil, nil,
			nil, "", "",
		))
}

// expectCounts answers the unique-agent and unique-owner counts.
func expectCounts(mock sqlmock.Sqlmock, serverID uuid.UUID, agents, owners int) {
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT agent_id\)`).
		WithArgs(serverID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(agents))
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT a\.created_by\)`).
		WithArgs(serverID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(owners))
}

// expectValidAttestations answers MCPAttestationRepository.GetValidAttestationsByMCP.
func expectValidAttestations(t *testing.T, mock sqlmock.Sqlmock, serverID uuid.UUID, atts []agreementAttestation) {
	t.Helper()
	rows := sqlmock.NewRows([]string{
		"id", "mcp_server_id", "agent_id", "attestation_data", "signature",
		"signature_verified", "verified_at", "expires_at", "is_valid", "created_at",
		"agent_name", "agent_trust_score",
	})
	for _, a := range atts {
		payload, err := json.Marshal(domain.AttestationPayload{
			AgentID:              a.agentID.String(),
			CapabilitiesFound:    a.tools,
			ConnectionSuccessful: a.connected,
			HealthCheckPassed:    a.connected,
			MCPName:              "files",
			MCPURL:               "https://files.example.com",
			SDKVersion:           "1.0.0",
			Timestamp:            a.at.Format(time.RFC3339),
		})
		require.NoError(t, err)
		rows.AddRow(uuid.New(), serverID, a.agentID, payload, "sig",
			true, a.at, a.at.Add(30*24*time.Hour), true, a.at,
			"agent", 0.9)
	}
	mock.ExpectQuery(`FROM mcp_attestations a\s+LEFT JOIN agents ag`).
		WithArgs(serverID).
		WillReturnRows(rows)
}

func expectVerified(mock sqlmock.Sqlmock, serverID uuid.UUID) {
	mock.ExpectExec(`UPDATE mcp_servers\s+SET\s+status = \$1`).
		WithArgs(string(domain.MCPServerStatusVerified), true, serverID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

// The consensus gate must compare what the attesting agents report, not only count them. Each case
// clears the agent, owner and confidence thresholds (3 agents, 2 owners, confidence 65); only the
// content of the attestations differs.
func TestConsensusVerification_RequiresAgreeingAttestations(t *testing.T) {
	now := time.Now().UTC()
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	tools := []string{"read_file", "write_file"}

	cases := []struct {
		name         string
		attestations []agreementAttestation
		wantVerified bool
	}{
		{
			name: "three agents report three different tool sets",
			attestations: []agreementAttestation{
				{a, []string{"read_file"}, true, now},
				{b, []string{"write_file"}, true, now},
				{c, []string{"delete_file"}, true, now},
			},
			wantVerified: false,
		},
		{
			name: "three agents each report a failed connection",
			attestations: []agreementAttestation{
				{a, tools, false, now},
				{b, tools, false, now},
				{c, tools, false, now},
			},
			wantVerified: false,
		},
		{
			name: "one of three agents reports a failed connection",
			attestations: []agreementAttestation{
				{a, tools, true, now},
				{b, tools, true, now},
				{c, tools, false, now},
			},
			wantVerified: false,
		},
		{
			name: "one of three agents reports an extra tool",
			attestations: []agreementAttestation{
				{a, tools, true, now},
				{b, tools, true, now},
				{c, []string{"read_file", "write_file", "exfiltrate"}, true, now},
			},
			wantVerified: false,
		},
		{
			name: "three agents connect and report the same tool set",
			attestations: []agreementAttestation{
				{a, tools, true, now},
				// Order, duplicates and capability-category tokens do not change the tool set.
				{b, []string{"write_file", "read_file", "read_file"}, true, now},
				{c, []string{"tools", "read_file", "write_file"}, true, now},
			},
			wantVerified: true,
		},
		{
			name: "an agent's latest attestation replaces its earlier one",
			attestations: []agreementAttestation{
				{a, tools, true, now},
				{b, tools, true, now},
				{c, tools, true, now.Add(-time.Hour)},
				{c, []string{"read_file"}, false, now.Add(-2 * time.Hour)},
			},
			wantVerified: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, mock := newAgreementTestService(t)
			serverID := uuid.New()

			expectServerLookup(mock, serverID, domain.MCPServerStatusPending, 65)
			expectCounts(mock, serverID, 3, 2)
			expectValidAttestations(t, mock, serverID, tc.attestations)
			if tc.wantVerified {
				expectVerified(mock, serverID)
			}

			err := svc.checkAndApplyConsensusVerification(context.Background(), serverID, 65)
			require.NoError(t, err, "a verification update the case does not expect means the server was verified")
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// The consensus status reported to the dashboard names disagreement as a missing criterion, and the
// manifest drift check keeps seeing the threshold-only signal: a report that disagrees with the
// others is the input drift detection exists to record, so disagreement must not switch it off.
func TestConsensusStatus_ReportsDisagreement(t *testing.T) {
	svc, mock := newAgreementTestService(t)
	serverID := uuid.New()
	now := time.Now().UTC()

	expectServerLookup(mock, serverID, domain.MCPServerStatusPending, 65)
	expectCounts(mock, serverID, 3, 2)
	expectValidAttestations(t, mock, serverID, []agreementAttestation{
		{uuid.New(), []string{"read_file"}, true, now},
		{uuid.New(), []string{"write_file"}, true, now},
		{uuid.New(), []string{"read_file"}, false, now},
	})

	status, err := svc.GetConsensusStatus(context.Background(), serverID)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	require.False(t, status.ConsensusReached)
	require.False(t, status.AttestationsAgree)
	require.Equal(t, 1, status.AgreeingAgents)
	require.Equal(t, 1, status.ConnectionFailures)
	require.Equal(t, 2, status.DistinctToolSets)
	require.Contains(t, status.MissingCriteria, "1 attesting agent reports a failed connection")
	require.Contains(t, status.MissingCriteria, "Attesting agents report 2 different tool sets")
	require.Less(t, status.ProgressPercent, 100.0)
	require.True(t, status.thresholdsMet(), "drift detection keys on the agent, owner and confidence thresholds only")
}

func TestConsensusStatus_AgreeingAttestationsReachConsensus(t *testing.T) {
	svc, mock := newAgreementTestService(t)
	serverID := uuid.New()
	now := time.Now().UTC()
	tools := []string{"read_file", "write_file"}

	expectServerLookup(mock, serverID, domain.MCPServerStatusPending, 65)
	expectCounts(mock, serverID, 3, 2)
	expectValidAttestations(t, mock, serverID, []agreementAttestation{
		{uuid.New(), tools, true, now},
		{uuid.New(), tools, true, now},
		{uuid.New(), tools, true, now},
	})

	status, err := svc.GetConsensusStatus(context.Background(), serverID)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	require.True(t, status.ConsensusReached)
	require.True(t, status.AttestationsAgree)
	require.Equal(t, 3, status.AgreeingAgents)
	require.Empty(t, status.MissingCriteria)
	require.Equal(t, 100.0, status.ProgressPercent)
}

func TestEvaluateAttestationAgreement(t *testing.T) {
	now := time.Now().UTC()
	agent := func() *uuid.UUID { id := uuid.New(); return &id }
	att := func(agentID *uuid.UUID, sig string, connected bool, at time.Time, tools ...string) *domain.MCPAttestation {
		return &domain.MCPAttestation{
			AgentID:    agentID,
			Signature:  sig,
			IsValid:    true,
			VerifiedAt: &at,
			CreatedAt:  at,
			AttestationData: domain.AttestationPayload{
				CapabilitiesFound:    tools,
				ConnectionSuccessful: connected,
			},
		}
	}

	t.Run("no attestations do not agree", func(t *testing.T) {
		got := evaluateAttestationAgreement(nil)
		require.False(t, got.agree())
		require.Equal(t, attestationAgreement{}, got)
	})

	t.Run("manual attestations are not agent reports", func(t *testing.T) {
		a := agent()
		got := evaluateAttestationAgreement([]*domain.MCPAttestation{
			att(a, "sig", true, now, "read_file"),
			att(nil, "manual-attestation", false, now, "other_tool"),
			att(agent(), "manual-attestation", true, now, "other_tool"),
		})
		require.Equal(t, attestationAgreement{Agents: 1, AgreeingAgents: 1, DistinctToolSets: 1}, got)
		require.True(t, got.agree())
	})

	t.Run("invalid attestations are ignored", func(t *testing.T) {
		a, b := agent(), agent()
		revoked := att(b, "sig", true, now, "other_tool")
		revoked.IsValid = false
		got := evaluateAttestationAgreement([]*domain.MCPAttestation{
			att(a, "sig", true, now, "read_file"),
			revoked,
		})
		require.Equal(t, attestationAgreement{Agents: 1, AgreeingAgents: 1, DistinctToolSets: 1}, got)
	})

	t.Run("the largest agreeing group is counted", func(t *testing.T) {
		got := evaluateAttestationAgreement([]*domain.MCPAttestation{
			att(agent(), "sig", true, now, "read_file", "write_file"),
			att(agent(), "sig", true, now, "write_file", "read_file"),
			att(agent(), "sig", true, now, "read_file"),
			att(agent(), "sig", false, now, "read_file", "write_file"),
		})
		require.Equal(t, attestationAgreement{Agents: 4, AgreeingAgents: 2, ConnectionFailures: 1, DistinctToolSets: 2}, got)
		require.False(t, got.agree())
	})

	t.Run("an empty tool set is a tool set", func(t *testing.T) {
		got := evaluateAttestationAgreement([]*domain.MCPAttestation{
			att(agent(), "sig", true, now),
			att(agent(), "sig", true, now, "tools"),
			att(agent(), "sig", true, now, "read_file"),
		})
		require.Equal(t, attestationAgreement{Agents: 3, AgreeingAgents: 2, DistinctToolSets: 2}, got)
		require.False(t, got.agree())
	})

	t.Run("a tool name holding a separator character is not two tool names", func(t *testing.T) {
		got := evaluateAttestationAgreement([]*domain.MCPAttestation{
			att(agent(), "sig", true, now, "read_file\x00write_file"),
			att(agent(), "sig", true, now, "read_file", "write_file"),
		})
		require.Equal(t, 2, got.DistinctToolSets)
		require.False(t, got.agree())
	})

	t.Run("the latest attestation per agent wins regardless of input order", func(t *testing.T) {
		a := agent()
		got := evaluateAttestationAgreement([]*domain.MCPAttestation{
			att(a, "sig", false, now.Add(-time.Hour), "read_file"),
			att(a, "sig", true, now, "write_file"),
			att(a, "sig", true, now.Add(-2*time.Hour), "delete_file"),
		})
		require.Equal(t, attestationAgreement{Agents: 1, AgreeingAgents: 1, DistinctToolSets: 1}, got)
		require.True(t, got.agree())
	})
}
