package application

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/require"
)

// expectWithdrawal answers MCPAttestationRepository.WithdrawConsensusVerification. The statement
// only matches a row that is verified and carries the consensus marker, so a server verified through
// SDK registration or challenge-response verification is left alone; rowsAffected says which it was.
func expectWithdrawal(mock sqlmock.Sqlmock, serverID uuid.UUID, rowsAffected int64) {
	mock.ExpectExec(`UPDATE mcp_servers\s+SET\s+status = \$1,\s+is_verified = false,\s+consensus_verified_at = NULL,\s+updated_at = \$2\s+WHERE id = \$3\s+AND status = \$4\s+AND consensus_verified_at IS NOT NULL`).
		WithArgs(string(domain.MCPServerStatusPending), sqlmock.AnyArg(), serverID, string(domain.MCPServerStatusVerified)).
		WillReturnResult(sqlmock.NewResult(0, rowsAffected))
}

// A server that multi-agent consensus verified keeps that status only while its attestations agree.
// Each case is a verified server that clears the agent, owner and confidence thresholds; only what
// the attesting agents now report differs.
func TestConsensusVerification_WithdrawnWhenAttestationsLaterDisagree(t *testing.T) {
	now := time.Now().UTC()
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	tools := []string{"read_file", "write_file"}

	cases := []struct {
		name          string
		attestations  []agreementAttestation
		wantWithdrawn bool
	}{
		{
			name: "an agent re-attests and reports an extra tool",
			attestations: []agreementAttestation{
				{a, tools, true, now.Add(-time.Hour)},
				{b, tools, true, now.Add(-time.Hour)},
				{c, tools, true, now.Add(-time.Hour)},
				{c, []string{"read_file", "write_file", "exfiltrate"}, true, now},
			},
			wantWithdrawn: true,
		},
		{
			name: "an agent re-attests and reports a failed connection",
			attestations: []agreementAttestation{
				{a, tools, true, now.Add(-time.Hour)},
				{b, tools, true, now.Add(-time.Hour)},
				{c, tools, true, now.Add(-time.Hour)},
				{c, tools, false, now},
			},
			wantWithdrawn: true,
		},
		{
			name: "three agents report three different tool sets",
			attestations: []agreementAttestation{
				{a, []string{"read_file"}, true, now},
				{b, []string{"write_file"}, true, now},
				{c, []string{"delete_file"}, true, now},
			},
			wantWithdrawn: true,
		},
		{
			name: "the agents still connect and report the same tool set",
			attestations: []agreementAttestation{
				{a, tools, true, now},
				{b, []string{"write_file", "read_file"}, true, now},
				{c, tools, true, now},
			},
			wantWithdrawn: false,
		},
		{
			name:          "no valid agent attestation is left",
			attestations:  nil,
			wantWithdrawn: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, mock := newAgreementTestService(t)
			serverID := uuid.New()

			expectServerLookup(mock, serverID, domain.MCPServerStatusVerified, 65)
			expectValidAttestations(t, mock, serverID, tc.attestations)
			if tc.wantWithdrawn {
				expectWithdrawal(mock, serverID, 1)
			}

			err := svc.checkAndApplyConsensusVerification(context.Background(), serverID, 65)
			require.NoError(t, err, "a status update the case does not expect means the verification was withdrawn")
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// A server verified through SDK registration or challenge-response verification carries no
// consensus marker, so disagreeing attestations match no row and leave it verified.
func TestConsensusVerification_DisagreementKeepsAVerificationThatDidNotComeFromConsensus(t *testing.T) {
	svc, mock := newAgreementTestService(t)
	serverID := uuid.New()
	now := time.Now().UTC()

	expectValidAttestations(t, mock, serverID, []agreementAttestation{
		{uuid.New(), []string{"read_file"}, true, now},
		{uuid.New(), []string{"write_file"}, true, now},
	})
	expectWithdrawal(mock, serverID, 0)

	withdrawn, err := svc.withdrawConsensusVerificationOnDisagreement(serverID)
	require.NoError(t, err)
	require.False(t, withdrawn)
	require.NoError(t, mock.ExpectationsWereMet())
}

// Consensus verification records that it came from consensus; challenge-response verification
// clears that record, because the server's signed challenge is then what it rests on.
func TestConsensusMarker_SetByConsensusAndClearedByChallengeResponse(t *testing.T) {
	t.Run("consensus verification sets the marker", func(t *testing.T) {
		svc, mock := newAgreementTestService(t)
		serverID := uuid.New()
		tools := []string{"read_file"}
		now := time.Now().UTC()

		expectServerLookup(mock, serverID, domain.MCPServerStatusPending, 65)
		expectCounts(mock, serverID, 3, 2)
		expectValidAttestations(t, mock, serverID, []agreementAttestation{
			{uuid.New(), tools, true, now},
			{uuid.New(), tools, true, now},
			{uuid.New(), tools, true, now},
		})
		mock.ExpectExec(`UPDATE mcp_servers\s+SET\s+status = \$1,\s+is_verified = \$2,\s+last_verified_at = CASE WHEN \$2 = true THEN \$4 ELSE last_verified_at END,\s+consensus_verified_at = CASE WHEN \$2 = true THEN \$4 ELSE NULL END`).
			WithArgs(string(domain.MCPServerStatusVerified), true, serverID, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))

		require.NoError(t, svc.checkAndApplyConsensusVerification(context.Background(), serverID, 65))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("challenge-response verification clears the marker", func(t *testing.T) {
		svc, mock := newAgreementTestService(t)
		serverID := uuid.New()

		mock.ExpectExec(`UPDATE mcp_servers\s+SET\s+is_verified = true,\s+last_verified_at = \$1,\s+status = \$2,\s+consensus_verified_at = NULL,`).
			WithArgs(sqlmock.AnyArg(), domain.MCPServerStatusVerified, serverID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		require.NoError(t, svc.mcpRepo.VerifyServer(context.Background(), serverID))
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

// The recheck at start covers servers whose attestations nobody records or revokes again, such as
// servers verified before verification compared what the attestations report.
func TestRecheckConsensusVerifications(t *testing.T) {
	svc, mock := newAgreementTestService(t)
	disagreeing, agreeing, unreadable := uuid.New(), uuid.New(), uuid.New()
	now := time.Now().UTC()
	tools := []string{"read_file", "write_file"}

	mock.ExpectQuery(`SELECT id\s+FROM mcp_servers\s+WHERE status = \$1\s+AND consensus_verified_at IS NOT NULL`).
		WithArgs(string(domain.MCPServerStatusVerified)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(disagreeing).AddRow(agreeing).AddRow(unreadable))

	expectValidAttestations(t, mock, disagreeing, []agreementAttestation{
		{uuid.New(), []string{"read_file"}, true, now},
		{uuid.New(), []string{"write_file"}, true, now},
		{uuid.New(), []string{"delete_file"}, true, now},
	})
	expectWithdrawal(mock, disagreeing, 1)

	expectValidAttestations(t, mock, agreeing, []agreementAttestation{
		{uuid.New(), tools, true, now},
		{uuid.New(), tools, true, now},
		{uuid.New(), tools, true, now},
	})

	mock.ExpectQuery(`FROM mcp_attestations a\s+LEFT JOIN agents ag`).
		WithArgs(unreadable).
		WillReturnError(context.DeadlineExceeded)

	result, err := svc.RecheckConsensusVerifications(context.Background())
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.Equal(t, 2, result.Checked)
	require.Equal(t, 1, result.Withdrawn)
	require.Equal(t, []uuid.UUID{unreadable}, result.Failed)
}
