package repository

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AIM-02 — the persistence half of "no write path can produce a verified
// isolation attestation".
//
// The scoring gates in calculateExecutionIsolation only hold because every row
// in isolation_attestations is unverified: an unverified report is clipped to
// the commodity-container ceiling, so a single writable `verified = TRUE` would
// hand back the full 1.0 the ceiling exists to deny. These tests hold the write
// side to that, at the SQL statement rather than at a caller's good intentions.

func TestAIM02_CreateNeverWritesVerified(t *testing.T) {
	t.Run("AIM-02.AC3 the INSERT hard-codes FALSE and ignores a verified struct field", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		repo := NewIsolationAttestationRepository(db)

		// The adversarial input: an attestation handed to Create with
		// verification already set. Whether it got there through a future
		// caller's mistake or an attacker-influenced field does not matter —
		// the statement must write FALSE either way.
		verifier := "attacker-supplied"
		verifiedAt := time.Now()
		att := &domain.IsolationAttestation{
			ID:         uuid.New(),
			AgentID:    uuid.New(),
			Sandbox:    domain.SandboxFirecracker,
			Network:    domain.NetworkAirgap,
			Filesystem: domain.FilesystemReadOnly,
			Process:    domain.ProcessFull,
			Score:      1.0,
			ReportedAt: time.Now(),
			CreatedAt:  time.Now(),
			Verified:   true,
			VerifiedBy: &verifier,
			VerifiedAt: &verifiedAt,
		}

		// FALSE, NULL, NULL are literals in the statement, so they are not
		// bindable: the nine args are posture and timestamps only, and there is
		// no placeholder through which a caller could reach `verified`.
		mock.ExpectExec(regexp.QuoteMeta("VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, FALSE, NULL, NULL)")).
			WithArgs(
				att.ID, att.AgentID,
				string(att.Sandbox), string(att.Network),
				string(att.Filesystem), string(att.Process),
				att.Score, att.ReportedAt, att.CreatedAt,
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		require.NoError(t, repo.Create(att))
		require.NoError(t, mock.ExpectationsWereMet(),
			"Create must write the literal FALSE for verified; binding the struct field here "+
				"would make the unverified ceiling bypassable by whoever constructs the struct")
	})
}

func TestAIM02_ScanCarriesVerificationColumns(t *testing.T) {
	// The columns the migration adds have to survive the round trip, or the
	// read-side gate would treat a genuinely verified row as unverified once
	// Phase 2 exists — and, more immediately, a scan that omitted them would
	// leave Verified at its zero value for reasons unrelated to the data.
	cols := []string{
		"id", "agent_id", "sandbox", "network", "filesystem", "process",
		"score", "reported_at", "created_at", "verified", "verified_by", "verified_at",
	}

	t.Run("AIM-02.AC3 GetLatest scans verified, verified_by and verified_at", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		repo := NewIsolationAttestationRepository(db)
		agentID := uuid.New()
		reportedAt := time.Now().Add(-time.Hour)
		verifiedAt := time.Now().Add(-30 * time.Minute)

		mock.ExpectQuery(regexp.QuoteMeta("verified, verified_by, verified_at")).
			WithArgs(agentID).
			WillReturnRows(sqlmock.NewRows(cols).AddRow(
				uuid.New(), agentID, "docker", "namespace", "readonly", "seccomp",
				0.65, reportedAt, reportedAt, true, "orchestrator-metadata", verifiedAt,
			))

		att, err := repo.GetLatest(agentID)
		require.NoError(t, err)
		require.NotNil(t, att)

		assert.True(t, att.Verified)
		require.NotNil(t, att.VerifiedBy)
		assert.Equal(t, "orchestrator-metadata", *att.VerifiedBy)
		require.NotNil(t, att.VerifiedAt)
		assert.WithinDuration(t, verifiedAt, *att.VerifiedAt, time.Second)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("AIM-02.AC3 a NULL verifier scans to nil rather than failing", func(t *testing.T) {
		// This is every row in the table today, so it is the path that must not
		// break: verified_by and verified_at are NULL wherever verified is FALSE.
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		repo := NewIsolationAttestationRepository(db)
		agentID := uuid.New()
		reportedAt := time.Now()

		mock.ExpectQuery(regexp.QuoteMeta("verified, verified_by, verified_at")).
			WithArgs(agentID).
			WillReturnRows(sqlmock.NewRows(cols).AddRow(
				uuid.New(), agentID, "docker", "namespace", "readonly", "seccomp",
				0.65, reportedAt, reportedAt, false, nil, nil,
			))

		att, err := repo.GetLatest(agentID)
		require.NoError(t, err)
		require.NotNil(t, att)

		assert.False(t, att.Verified)
		assert.Nil(t, att.VerifiedBy, "NULL must scan to nil, not to an empty-string verifier")
		assert.Nil(t, att.VerifiedAt)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("AIM-02.AC3 GetLatest orders by reported_at so a re-attestation supersedes", func(t *testing.T) {
		// The no-carry-forward rule is enforced by the ordering: the newest
		// report is the only row the scorer ever sees, so a superseded verified
		// row cannot contribute.
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		repo := NewIsolationAttestationRepository(db)
		agentID := uuid.New()

		mock.ExpectQuery(regexp.QuoteMeta("ORDER BY reported_at DESC")).
			WithArgs(agentID).
			WillReturnRows(sqlmock.NewRows(cols))

		_, err = repo.GetLatest(agentID)
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
