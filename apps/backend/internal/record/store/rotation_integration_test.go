//go:build integration

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/trace"
	"github.com/stretchr/testify/require"
)

// tracedDraft is a draft with no type and no member besides its trace
// members: the first record of a trace minted for it alone.
func tracedDraft(t *testing.T) record.Draft {
	t.Helper()
	d := record.Draft{EventID: uuid.NewString()}
	ctx, err := trace.Begin(context.Background())
	require.NoError(t, err)
	require.NoError(t, trace.Stamp(ctx, &d, nil))
	return d
}

// chainKeyID is the key id the organization's chain row names: the key its
// next record is signed with.
func chainKeyID(t *testing.T, db *sql.DB, organizationID string) string {
	t.Helper()
	var keyID string
	require.NoError(t, db.QueryRow(`SELECT key_id FROM record_chains WHERE organization_id = $1`,
		organizationID).Scan(&keyID))
	return keyID
}

// observedApply is markOrg that also sets *ran when it is called, so a test
// can tell a state change that never ran from one that rolled back.
func observedApply(ran *bool, organizationID string) func(context.Context, *sql.Tx) error {
	mark := markOrg(organizationID)
	return func(ctx context.Context, tx *sql.Tx) error {
		*ran = true
		return mark(ctx, tx)
	}
}

// signerOf is the key id stored with the record at seq of the chain.
func signerOf(t *testing.T, db *sql.DB, chainID string, seq int64) string {
	t.Helper()
	var keyID string
	require.NoError(t, db.QueryRow(`SELECT key_id FROM audit_records WHERE chain_id = $1 AND seq = $2`,
		chainID, seq).Scan(&keyID))
	return keyID
}

// A key rotation appended through the writer is signed by the chain's key and
// names the next key, and the statement that appends it moves the chain to
// that key: the writer that holds the retired key extends the chain no
// further, a writer that holds the next key extends it, and the chain
// verifies from its first key across two rotations.
func TestRecordKeyRotationMovesTheChainToItsNextKey(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	org := seedOrg(t, plain)
	first := newHarness(t, db, nil)
	ctx := context.Background()

	genesis, err := first.w.start(ctx, org)
	require.NoError(t, err)
	_, err = first.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft()})
	require.NoError(t, err)

	secondKeys := newTestKeys(t)
	rotated, err := first.w.Write(ctx, Write{Class: ClassReduction, NoDebt: true, OrganizationID: org,
		Draft:    tracedDraft(t),
		Rotation: &Rotation{NextKey: secondKeys.publicKey(), Reason: record.RotationScheduled}})
	require.NoError(t, err)
	require.Equal(t, int64(2), rotated.Seq)
	require.Equal(t, secondKeys.publicKey().KeyID(), chainKeyID(t, plain, org))
	require.Equal(t, first.keys.publicKey().KeyID(), signerOf(t, plain, genesis.ChainID, rotated.Seq),
		"the key in force signs the rotation that retires it")
	stored := readStored(t, plain, genesis.ChainID)[rotated.Seq]
	require.Equal(t, record.TypeKeyRotation, stored.Type, "a draft with no type is written as a key rotation")
	require.Equal(t, record.RotationScheduled, stored.ext(stored.Retained, "rotation_reason"))
	nextKey, _ := stored.ext(stored.Retained, "next_key").(map[string]any)
	require.Equal(t, secondKeys.publicKey().KeyID(), nextKey["keyid"])

	status, err := ReadChainState(ctx, plain, org)
	require.NoError(t, err)
	require.Equal(t, record.ChainExtendable, status.State)
	require.Equal(t, HeadView{Seq: rotated.Seq, Hash: rotated.RecordHash}, *status.Head)

	_, err = first.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft()})
	wantWriteError(t, err, ClassExpansion, ReasonSigner)

	second := newHarness(t, db, secondKeys)
	extended, err := second.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft()})
	require.NoError(t, err)
	require.Equal(t, rotated.Seq+1, extended.Seq)
	require.Equal(t, secondKeys.publicKey().KeyID(), signerOf(t, plain, genesis.ChainID, extended.Seq))

	// A second rotation, on exposure, states when the exposure began.
	thirdKeys := newTestKeys(t)
	began := time.Date(2026, time.October, 20, 9, 30, 0, 0, time.UTC)
	exposed, err := second.w.Write(ctx, Write{Class: ClassReduction, NoDebt: true, OrganizationID: org,
		Draft: tracedDraft(t),
		Rotation: &Rotation{NextKey: thirdKeys.publicKey(), Reason: record.RotationExposure,
			ExposureBegan: began}})
	require.NoError(t, err)
	require.Equal(t, extended.Seq+1, exposed.Seq)
	require.Equal(t, thirdKeys.publicKey().KeyID(), chainKeyID(t, plain, org))
	stored = readStored(t, plain, genesis.ChainID)[exposed.Seq]
	require.Equal(t, record.RotationExposure, stored.ext(stored.Retained, "rotation_reason"))
	require.Equal(t, "2026-10-20T09:30:00.000000Z", stored.ext(stored.Retained, "exposure_began_at"))

	records, err := ReadChain(ctx, plain, genesis.ChainID)
	require.NoError(t, err)
	require.Len(t, records, 5)
	res, err := record.Verify(records, first.keys.publicKey())
	require.NoError(t, err)
	require.True(t, res.OK(), "verification failed: %v", res.Failure)
	require.Equal(t, 5, res.Verified)
	require.Equal(t, exposed.RecordHash, res.Head.Hash)
	require.Equal(t, thirdKeys.publicKey().KeyID(), res.Key.KeyID(),
		"the record after the head is signed with the key the last rotation names")
}

// The writer refuses a key rotation the verifier would fail, and one of a
// chain whose key is not the writer's, before the write's state change runs:
// a rotation back to a key that has signed a record of the chain, a rotation
// by a writer whose key the chain retired, and a rotation of an organization
// whose chain has not started, in the pre-chain state as well. None appends a
// record or moves the chain's key.
func TestRecordKeyRotationRefusalsLeaveTheChainAsItWas(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	org := seedOrg(t, plain)
	first := newHarness(t, db, nil)
	ctx := context.Background()

	genesis, err := first.w.start(ctx, org)
	require.NoError(t, err)
	secondKeys := newTestKeys(t)
	_, err = first.w.Write(ctx, Write{Class: ClassObservation, OrganizationID: org, Draft: tracedDraft(t),
		Rotation: &Rotation{NextKey: secondKeys.publicKey(), Reason: record.RotationCustodyChange}})
	require.NoError(t, err)
	second := newHarness(t, db, secondKeys)
	before, err := ReadChainState(ctx, plain, org)
	require.NoError(t, err)
	unmark(t, plain, org)

	// Back to the first key, which signed the genesis.
	ran := false
	_, err = second.w.Write(ctx, Write{Class: ClassObservation, OrganizationID: org, Draft: tracedDraft(t),
		Apply:    observedApply(&ran, org),
		Rotation: &Rotation{NextKey: first.keys.publicKey(), Reason: record.RotationScheduled}})
	require.ErrorIs(t, err, ErrInvalidWrite)
	var we *WriteError
	require.False(t, errors.As(err, &we), "a refused request is not a failed record write: %v", err)
	_, total := second.failures(t, ClassObservation, ReasonSigner)
	require.Equal(t, 0.0, total)
	require.False(t, ran, "the state change did not run")
	require.False(t, marked(t, plain, org))

	// By the key the chain retired.
	_, err = first.w.Write(ctx, Write{Class: ClassObservation, OrganizationID: org, Draft: tracedDraft(t),
		Apply:    observedApply(&ran, org),
		Rotation: &Rotation{NextKey: newTestKeys(t).publicKey(), Reason: record.RotationScheduled}})
	wantWriteError(t, err, ClassObservation, ReasonSigner)
	require.False(t, ran, "the state change did not run")
	require.False(t, marked(t, plain, org))

	after, err := ReadChainState(ctx, plain, org)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.Equal(t, secondKeys.publicKey().KeyID(), chainKeyID(t, plain, org))
	records, err := ReadChain(ctx, plain, genesis.ChainID)
	require.NoError(t, err)
	res, err := record.Verify(records, first.keys.publicKey())
	require.NoError(t, err)
	require.True(t, res.OK(), "verification failed: %v", res.Failure)
	require.Equal(t, 2, res.Verified)

	// An organization whose chain has not started has no key to rotate, and
	// a writer in the pre-chain state does not commit a rotation without its
	// record.
	for _, h := range []harness{first, newPreChainHarness(t, db, first.keys)} {
		unstarted := seedOrg(t, plain)
		_, err = h.w.Write(ctx, Write{Class: ClassObservation, OrganizationID: unstarted, Draft: tracedDraft(t),
			Apply:    observedApply(&ran, unstarted),
			Rotation: &Rotation{NextKey: secondKeys.publicKey(), Reason: record.RotationScheduled}})
		wantWriteError(t, err, ClassObservation, ReasonChainHead)
		require.False(t, ran, "the state change did not run")
		require.False(t, marked(t, plain, unstarted))
		status, err := ReadChainState(ctx, plain, unstarted)
		require.NoError(t, err)
		require.Equal(t, record.ChainNotStarted, status.State)
	}
}

// A key rotation checks the chain's key again under the append lock. When
// another rotation by the same key moves the chain after this one's check in
// step 2 and before this one takes the lock, this one fails with reason
// signer and appends nothing.
func TestRecordKeyRotationThatLosesTheLockToAnotherRotationFails(t *testing.T) {
	db, tp := openTapped(t, 0)
	plain := openPlain(t)
	org := seedOrg(t, plain)
	keys := newTestKeys(t)
	slow := newHarness(t, db, keys)
	fast := newHarness(t, plain, keys)
	ctx := context.Background()
	_, err := fast.w.start(ctx, org)
	require.NoError(t, err)

	tp.blockBefore(lockChainQuery, 750*time.Millisecond)
	slowDraft, slowNext := tracedDraft(t), newTestKeys(t).publicKey()
	done := make(chan error, 1)
	go func() {
		_, err := slow.w.Write(ctx, Write{Class: ClassObservation, OrganizationID: org, Draft: slowDraft,
			Rotation: &Rotation{NextKey: slowNext, Reason: record.RotationScheduled}})
		done <- err
	}()
	// The slow rotation sleeps before its lock statement only once its check
	// in step 2 has returned.
	require.Eventually(t, func() bool {
		var sleeping int
		err := plain.QueryRow(`SELECT count(*) FROM pg_stat_activity
 WHERE datname = current_database() AND state = 'active' AND query = 'SELECT pg_sleep(0.750)'`).Scan(&sleeping)
		return err == nil && sleeping == 1
	}, 5*time.Second, 5*time.Millisecond, "the slow rotation never reached its lock statement")
	require.Contains(t, tp.recorded(), rotationQuery)

	winner := newTestKeys(t)
	won, err := fast.w.Write(ctx, Write{Class: ClassObservation, OrganizationID: org, Draft: tracedDraft(t),
		Rotation: &Rotation{NextKey: winner.publicKey(), Reason: record.RotationScheduled}})
	require.NoError(t, err)

	wantWriteError(t, <-done, ClassObservation, ReasonSigner)
	require.True(t, slices.Contains(tp.recorded(), lockChainQuery), "the slow rotation failed under the append lock")
	require.Equal(t, winner.publicKey().KeyID(), chainKeyID(t, plain, org))
	status, err := ReadChainState(ctx, plain, org)
	require.NoError(t, err)
	require.Equal(t, HeadView{Seq: won.Seq, Hash: won.RecordHash}, *status.Head)
	require.Equal(t, 2, positions(t, plain, org))
}

// Statements a key rotation and a correction send after the append lock
// statement returns, COMMIT included, with no guard and with a guard that
// probes one record: as many as an ordinary write sends, and the same append
// statement, which for the rotation also moves the chain's key. The read
// each makes of the chain to check the request is sent before the lock.
func TestRecordRotationAndCorrectionStatementsUnderTheAppendLock(t *testing.T) {
	db, tp := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	org := seedOrg(t, plain)
	_, err := h.w.start(ctx, org)
	require.NoError(t, err)
	corrected, err := h.w.Write(ctx, Write{Class: ClassObservation, OrganizationID: org, Draft: testDraft()})
	require.NoError(t, err)

	split := func(t *testing.T) (before, under []string) {
		t.Helper()
		got := tp.recorded()
		at := -1
		for i, s := range got {
			if s == lockChainQuery {
				at = i
			}
		}
		require.GreaterOrEqual(t, at, 0, "no lock statement was sent")
		return got[:at], got[at+1:]
	}
	countOf := func(statements []string, query string) int {
		n := 0
		for _, s := range statements {
			if s == query {
				n++
			}
		}
		return n
	}
	guardProbing := func(hops int) Guard {
		if hops == 0 {
			return nil
		}
		return func(ctx context.Context, _ time.Time, p *Probe, _ *record.Draft) (*Stored, error) {
			for i := 0; i < hops; i++ {
				if _, _, err := p.BySeq(ctx, int64(i)); err != nil {
					return nil, err
				}
			}
			return nil, nil
		}
	}

	w := h.w
	for _, hops := range []int{0, 1} {
		t.Run(fmt.Sprintf("correction_guard_probing_%d", hops), func(t *testing.T) {
			tp.reset()
			_, err := w.Write(ctx, Write{Class: ClassObservation, OrganizationID: org, Draft: tracedDraft(t),
				Corrects: corrected.EventID, Guard: guardProbing(hops)})
			require.NoError(t, err)
			before, under := split(t)
			t.Logf("%d statements after the lock: %v", len(under), statementLabels(under))
			require.Equal(t, 1, countOf(before, correctedQuery), "the check reads the chain once, before the lock")
			require.Len(t, under, 4+hops)
			require.Equal(t, setStatementTimeoutQuery, under[0])
			require.Equal(t, chainStateQuery, under[1])
			require.Equal(t, appendQuery, under[len(under)-2])
			require.Equal(t, "COMMIT", under[len(under)-1])
		})
	}
	for _, hops := range []int{0, 1} {
		t.Run(fmt.Sprintf("key_rotation_guard_probing_%d", hops), func(t *testing.T) {
			next := newTestKeys(t)
			tp.reset()
			_, err := w.Write(ctx, Write{Class: ClassObservation, OrganizationID: org, Draft: tracedDraft(t),
				Rotation: &Rotation{NextKey: next.publicKey(), Reason: record.RotationScheduled},
				Guard:    guardProbing(hops)})
			require.NoError(t, err)
			before, under := split(t)
			t.Logf("%d statements after the lock: %v", len(under), statementLabels(under))
			require.Equal(t, 1, countOf(before, rotationQuery), "the check reads the chain once, before the lock")
			require.Len(t, under, 4+hops)
			require.Equal(t, setStatementTimeoutQuery, under[0])
			require.Equal(t, chainStateQuery, under[1])
			require.Equal(t, appendQuery, under[len(under)-2])
			require.Equal(t, "COMMIT", under[len(under)-1])
			require.Equal(t, next.publicKey().KeyID(), chainKeyID(t, plain, org),
				"the append statement moved the chain's key")
			// The chain is now the next key's.
			w = newHarness(t, db, next).w
		})
	}
}
