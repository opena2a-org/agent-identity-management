//go:build integration

package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/stretchr/testify/require"
)

// storedRow is the stored row of the record at seq of the chain, every
// column, as JSON text.
func storedRow(t *testing.T, db *sql.DB, chainID string, seq int64) string {
	t.Helper()
	var row string
	require.NoError(t, db.QueryRow(`SELECT row_to_json(r)::text FROM audit_records r WHERE chain_id = $1 AND seq = $2`,
		chainID, seq).Scan(&row))
	return row
}

// A correction appended through the writer names, in its tenant part, the
// record of the organization's chain it corrects. The corrected record's row
// is left as it was, and the chain verifies.
func TestRecordCorrectionNamesARecordOfTheOrganizationsChain(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	org := seedOrg(t, plain)
	h := newHarness(t, db, nil)
	ctx := context.Background()

	genesis, err := h.w.start(ctx, org)
	require.NoError(t, err)
	corrected, err := h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft()})
	require.NoError(t, err)
	row := storedRow(t, plain, genesis.ChainID, corrected.Seq)

	unmark(t, plain, org)
	correction, err := h.w.Write(ctx, Write{Class: ClassObservation, OrganizationID: org, Draft: tracedDraft(t),
		Apply: markOrg(org), Corrects: corrected.EventID})
	require.NoError(t, err)
	require.Equal(t, corrected.Seq+1, correction.Seq)
	require.True(t, marked(t, plain, org), "the correction's state change committed with it")

	stored := readStored(t, plain, genesis.ChainID)[correction.Seq]
	require.Equal(t, record.TypeCorrection, stored.Type, "a draft with no type is written as a correction")
	require.Equal(t, corrected.EventID, stored.ext(stored.Tenant, "corrected_event_id"))
	require.Nil(t, stored.ext(stored.Retained, "corrected_event_id"), "the reference sits in the tenant part only")
	require.Equal(t, row, storedRow(t, plain, genesis.ChainID, corrected.Seq), "the corrected record is left as it was")

	// A correction of a correction names the earlier correction.
	again, err := h.w.Write(ctx, Write{Class: ClassObservation, OrganizationID: org, Draft: tracedDraft(t),
		Corrects: correction.EventID})
	require.NoError(t, err)

	records, err := ReadChain(ctx, plain, genesis.ChainID)
	require.NoError(t, err)
	require.Len(t, records, 4)
	res, err := record.Verify(records, h.keys.publicKey())
	require.NoError(t, err)
	require.True(t, res.OK(), "verification failed: %v", res.Failure)
	require.Equal(t, 4, res.Verified)
	require.Equal(t, again.RecordHash, res.Head.Hash)
}

// The writer refuses, before the write's state change runs, a correction
// that names no record of the organization's chain: an event id no record
// carries, the event id of another organization's record, and any event id
// while the organization's chain has not started, in the pre-chain state as
// well. None appends a record or is a failed record write.
func TestRecordCorrectionOfARecordOutsideTheOrganizationsChainIsRefused(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	org, other := seedOrg(t, plain), seedOrg(t, plain)
	h := newHarness(t, db, nil)
	ctx := context.Background()

	_, err := h.w.start(ctx, org)
	require.NoError(t, err)
	_, err = h.w.start(ctx, other)
	require.NoError(t, err)
	foreign, err := h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: other, Draft: testDraft()})
	require.NoError(t, err)
	before, err := ReadChainState(ctx, plain, org)
	require.NoError(t, err)
	unmark(t, plain, org)

	for _, tc := range []struct{ name, corrects string }{
		{"an_event_id_no_record_carries", uuid.NewString()},
		{"another_organizations_record", foreign.EventID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ran := false
			_, err := h.w.Write(ctx, Write{Class: ClassObservation, OrganizationID: org, Draft: tracedDraft(t),
				Apply: observedApply(&ran, org), Corrects: tc.corrects})
			require.ErrorIs(t, err, ErrInvalidWrite)
			var we *WriteError
			require.False(t, errors.As(err, &we), "a refused request is not a failed record write: %v", err)
			require.False(t, ran, "the state change did not run")
			require.False(t, marked(t, plain, org))
		})
	}
	after, err := ReadChainState(ctx, plain, org)
	require.NoError(t, err)
	require.Equal(t, before, after)
	_, total := h.failures(t, ClassObservation, ReasonTrace)
	require.Equal(t, 0.0, total)

	for _, w := range []harness{h, newPreChainHarness(t, db, h.keys)} {
		unstarted := seedOrg(t, plain)
		ran := false
		_, err := w.w.Write(ctx, Write{Class: ClassObservation, OrganizationID: unstarted, Draft: tracedDraft(t),
			Apply: observedApply(&ran, unstarted), Corrects: foreign.EventID})
		require.ErrorIs(t, err, ErrInvalidWrite)
		require.False(t, ran, "the state change did not run")
		require.False(t, marked(t, plain, unstarted))
		require.Equal(t, 0, positions(t, plain, unstarted))
	}
}
