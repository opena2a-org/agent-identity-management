//go:build integration

package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A lead's record is appended before the write's record, in the same
// transaction and with its timestamp, and the two verify as one chain. A lead
// that returns nothing appends nothing of its own.
func TestRecordLeadIsAppendedBeforeTheRecord(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	org := seedOrg(t, plain)
	genesis, err := h.w.start(ctx, org)
	require.NoError(t, err)

	lead, main := testDraft(), testDraft()
	var probed bool
	out, err := h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: main,
		Lead: func(ctx context.Context, p *Probe) (*record.Draft, error) {
			newest, err := p.Newest(ctx, 1)
			probed = err == nil && len(newest) == 1 && newest[0].Seq == 0
			return &lead, err
		}})
	require.NoError(t, err)
	require.True(t, probed, "the lead did not read the chain under the lock")
	assert.Equal(t, int64(2), out.Seq)
	assert.Equal(t, main.EventID, out.EventID)
	assert.Equal(t, lead.EventID, out.LeadEventID)

	none, err := h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft(),
		Lead: func(context.Context, *Probe) (*record.Draft, error) { return nil, nil }})
	require.NoError(t, err)
	assert.Equal(t, int64(3), none.Seq)
	assert.Empty(t, none.LeadEventID)

	records, err := ReadChain(ctx, plain, genesis.ChainID)
	require.NoError(t, err)
	require.Len(t, records, 4)
	res, err := record.Verify(records, h.keys.publicKey())
	require.NoError(t, err)
	require.True(t, res.OK(), "verification failed: %v", res.Failure)

	rows, err := plain.Query(`SELECT seq, event_id, recorded_at FROM audit_records WHERE chain_id = $1 AND seq IN (1, 2) ORDER BY seq`, genesis.ChainID)
	require.NoError(t, err)
	defer rows.Close()
	var events []string
	var times []time.Time
	for rows.Next() {
		var (
			seq     int64
			eventID string
			at      time.Time
		)
		require.NoError(t, rows.Scan(&seq, &eventID, &at))
		events, times = append(events, eventID), append(times, at)
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, []string{lead.EventID, main.EventID}, events)
	require.Len(t, times, 2)
	assert.True(t, times[0].Equal(times[1]), "the lead's record does not carry the record's timestamp")
}

// A lead that refuses the write, returns the record's own event id or a draft
// that cannot be serialized, or whose probe statement fails, fails the whole
// write: nothing is appended, the state change rolls back, and the failure is
// counted under its reason. A lead does not run when the guard ends the
// write.
func TestRecordLeadFailureFailsTheWholeWrite(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	org := seedOrg(t, plain)
	genesis, err := h.w.start(ctx, org)
	require.NoError(t, err)

	marked := func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE organizations SET updated_at = '2001-02-03T04:05:06Z' WHERE id = $1`, org)
		return err
	}
	main := testDraft()
	for name, tc := range map[string]struct {
		lead   Lead
		reason Reason
	}{
		"a refusal": {func(context.Context, *Probe) (*record.Draft, error) {
			return nil, errors.New("lead refused")
		}, ReasonGuard},
		"the record's own event id": {func(context.Context, *Probe) (*record.Draft, error) {
			d := testDraft()
			d.EventID = main.EventID
			return &d, nil
		}, ReasonGuard},
		"a draft that cannot be serialized": {func(context.Context, *Probe) (*record.Draft, error) {
			d := testDraft()
			d.Retained["score"] = 0.5
			return &d, nil
		}, ReasonCanonical},
		"a failed probe": {func(ctx context.Context, p *Probe) (*record.Draft, error) {
			_, err := p.Newest(ctx, 0)
			d := testDraft()
			return &d, err
		}, ReasonChainGuardProbe},
	} {
		t.Run(name, func(t *testing.T) {
			oneBefore, totalBefore := h.failures(t, ClassExpansion, tc.reason)
			_, err := h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: main, Apply: marked, Lead: tc.lead})
			wantWriteError(t, err, ClassExpansion, tc.reason)
			one, total := h.failures(t, ClassExpansion, tc.reason)
			assert.Equal(t, oneBefore+1, one)
			assert.Equal(t, totalBefore+1, total, "the failure was counted more than once")
			status, err := ReadChainState(ctx, plain, org)
			require.NoError(t, err)
			assert.Equal(t, int64(0), status.Head.Seq, "a record was appended")
			var rolledBack bool
			require.NoError(t, plain.QueryRow(`SELECT updated_at <> '2001-02-03T04:05:06Z' FROM organizations WHERE id = $1`, org).Scan(&rolledBack))
			assert.True(t, rolledBack, "the state change committed")
		})
	}

	ran := false
	out, err := h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft(),
		Guard: func(ctx context.Context, _ time.Time, p *Probe, _ *record.Draft) (*Stored, error) {
			s, _, err := p.BySeq(ctx, 0)
			return &s, err
		},
		Lead: func(context.Context, *Probe) (*record.Draft, error) {
			ran = true
			return nil, nil
		}})
	require.NoError(t, err)
	assert.True(t, out.Existing)
	assert.Equal(t, genesis.EventID, out.EventID)
	assert.False(t, ran, "the lead ran after the guard ended the write")
}

// A lead's draft takes a place in a trace as the write's own draft does: one
// with no trace_id, or whose parent is not a record of the organization's
// chain, fails the whole write with reason trace.
func TestRecordLeadDraftIsCheckedForItsTrace(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	org := seedOrg(t, plain)
	_, err := h.w.start(ctx, org)
	require.NoError(t, err)

	for name, lead := range map[string]func() *record.Draft{
		"no trace_id": func() *record.Draft {
			d := testDraft()
			delete(d.Tenant, "trace_id")
			return &d
		},
		"a parent outside the chain": func() *record.Draft {
			d := testDraft()
			d.Tenant["parent_id"] = uuid.NewString()
			d.Retained["opena2a"].(map[string]any)["trace_origin"] = "parent"
			return &d
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft(), Apply: markOrg(org),
				Lead: func(context.Context, *Probe) (*record.Draft, error) { return lead(), nil }})
			wantWriteError(t, err, ClassExpansion, ReasonTrace)
			assert.Equal(t, 1, positions(t, plain, org), "a record was appended")
			assert.False(t, marked(t, plain, org), "the state change committed")
		})
	}
}

// A debt row holds one record, so a reduction that may commit with a debt
// cannot take a lead: it is refused before its state change runs. A reduction
// that sets NoDebt takes one, and a failure of its lead refuses it whole with
// no debt. A writer in the pre-chain state commits a write for an
// organization whose chain has not started without running its lead.
func TestRecordLeadOnAReductionAndBeforeTheChain(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	ctx := context.Background()
	keys := newTestKeys(t)
	h := newHarness(t, db, keys)
	org := seedOrg(t, plain)
	_, err := h.w.start(ctx, org)
	require.NoError(t, err)
	refuse := func(context.Context, *Probe) (*record.Draft, error) { return nil, errors.New("lead refused") }

	t.Run("a reduction that may owe a debt", func(t *testing.T) {
		_, err := h.w.Write(ctx, Write{Class: ClassReduction, OrganizationID: org, Draft: testDraft(),
			Apply: markOrg(org), Lead: refuse})
		require.ErrorIs(t, err, ErrInvalidWrite)
		assert.False(t, marked(t, plain, org), "the state change ran")
		assert.Equal(t, 1, positions(t, plain, org))
		assert.Empty(t, debtRows(t, plain, org))
	})

	t.Run("a reduction that sets NoDebt", func(t *testing.T) {
		_, err := h.w.Write(ctx, Write{Class: ClassReduction, OrganizationID: org, Draft: testDraft(),
			Apply: markOrg(org), Lead: refuse, NoDebt: true})
		wantWriteError(t, err, ClassReduction, ReasonGuard)
		assert.False(t, marked(t, plain, org), "the state change committed")
		assert.Equal(t, 1, positions(t, plain, org))
		assert.Empty(t, debtRows(t, plain, org), "a debt was written")

		lead := testDraft()
		out, err := h.w.Write(ctx, Write{Class: ClassReduction, OrganizationID: org, Draft: testDraft(), NoDebt: true,
			Lead: func(context.Context, *Probe) (*record.Draft, error) { return &lead, nil }})
		require.NoError(t, err)
		assert.Equal(t, lead.EventID, out.LeadEventID)
		assert.Equal(t, int64(2), out.Seq)
	})

	t.Run("the pre-chain state", func(t *testing.T) {
		pre := newPreChainHarness(t, db, keys)
		unstarted := seedOrg(t, plain)
		ran := false
		draft := testDraft()
		out, err := pre.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: unstarted, Draft: draft, Apply: markOrg(unstarted),
			Lead: func(context.Context, *Probe) (*record.Draft, error) {
				ran = true
				return nil, nil
			}})
		require.NoError(t, err)
		assert.Equal(t, Appended{EventID: draft.EventID, Unchained: true}, out)
		assert.False(t, ran, "the lead ran with no chain to read")
		assert.True(t, marked(t, plain, unstarted), "the state change did not commit")
		assert.Zero(t, positions(t, plain, unstarted))
	})
}
