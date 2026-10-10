//go:build integration

package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A write appends a signed record in the same transaction as its state
// change, and what the writer stores verifies as one chain.
func TestRecordWriterAppendsAChainThatVerifies(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	org := seedOrg(t, plain)
	h := newHarness(t, db, nil)
	ctx := context.Background()

	genesis, err := h.w.start(ctx, org)
	require.NoError(t, err)
	require.Equal(t, int64(0), genesis.Seq)

	touched := func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE organizations SET updated_at = NOW() WHERE id = $1`, org)
		return err
	}
	var last Appended
	for i := 0; i < 3; i++ {
		last, err = h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft(), Apply: touched})
		require.NoError(t, err)
		require.Equal(t, int64(i+1), last.Seq)
	}

	// A state change that fails takes its record with it.
	refused := errors.New("state change refused")
	_, err = h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft(),
		Apply: func(context.Context, *sql.Tx) error { return refused }})
	require.ErrorIs(t, err, refused)
	var we *WriteError
	require.False(t, errors.As(err, &we), "a failed state change is not a failed record write")

	status, err := ReadChainState(ctx, plain, org)
	require.NoError(t, err)
	require.Equal(t, record.ChainExtendable, status.State)
	require.Equal(t, genesis.ChainID, *status.ChainID)
	require.Equal(t, HeadView{Seq: 3, Hash: last.RecordHash}, *status.Head)

	records, err := ReadChain(ctx, plain, genesis.ChainID)
	require.NoError(t, err)
	require.Len(t, records, 4)
	res, err := record.Verify(records, h.keys.publicKey())
	require.NoError(t, err)
	require.True(t, res.OK(), "verification failed: %v", res.Failure)
	require.Equal(t, 4, res.Verified)
	require.Equal(t, last.RecordHash, res.Head.Hash)
}

// The genesis pins the organization's existing audit rows by table, id and
// timestamp, and starting a second chain for one organization is refused.
func TestRecordGenesisPinsTheOrganizationsRows(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	org := seedOrg(t, plain)
	h := newHarness(t, db, nil)
	ctx := context.Background()

	var pre []record.PreGenesisRow
	for i := 0; i < 2; i++ {
		id := uuid.NewString()
		at := time.Date(2026, time.March, 4, 5, 6, i, 789123000, time.UTC)
		_, err := plain.Exec(`INSERT INTO audit_logs (id, organization_id, action, resource_type, resource_id, "timestamp")
			VALUES ($1, $2, 'create', 'agent', $3, $4)`, id, org, uuid.NewString(), at)
		require.NoError(t, err)
		pre = append(pre, record.PreGenesisRow{Table: record.TableAuditLogs, ID: id, Timestamp: at})
	}

	genesis, err := h.w.start(ctx, org)
	require.NoError(t, err)
	_, err = h.w.start(ctx, org)
	require.ErrorIs(t, err, ErrChainStarted)

	records, err := ReadChain(ctx, plain, genesis.ChainID)
	require.NoError(t, err)
	require.Len(t, records, 1)
	var payload struct {
		OpenA2A struct {
			PreGenesis struct {
				RowCounts map[string]int `json:"row_counts"`
				SetDigest string         `json:"set_digest"`
			} `json:"pre_genesis"`
		} `json:"opena2a"`
	}
	raw, err := record.Open(records[0].Envelope, record.ClassRecordV1, keyVerifier{h.keys.publicKey()})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &payload))
	want, err := record.SetDigest(pre)
	require.NoError(t, err)
	assert.Equal(t, 2, payload.OpenA2A.PreGenesis.RowCounts[record.TableAuditLogs])
	assert.Equal(t, 0, payload.OpenA2A.PreGenesis.RowCounts[record.TableVerificationEvents])
	assert.Equal(t, want, payload.OpenA2A.PreGenesis.SetDigest)
}

// No chain starts while a foreign key would remove audit rows by cascade:
// the cascade would take chained rows with it and leave no record.
func TestRecordChainDoesNotStartWhileCascadesRemoveAuditRows(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	org := seedOrg(t, plain)
	h := newHarness(t, db, nil)
	ctx := context.Background()

	cascading, err := CascadingForeignKeys(ctx, plain)
	require.NoError(t, err)
	// The schema as shipped: these are replaced by a later migration.
	require.Equal(t, []string{
		"audit_logs.audit_logs_organization_id_fkey",
		"audit_logs.audit_logs_user_id_fkey",
		"verification_events.verification_events_agent_id_fkey",
		"verification_events.verification_events_mcp_server_id_fkey",
		"verification_events.verification_events_organization_id_fkey",
	}, cascading)

	_, err = h.w.Start(ctx, org)
	var pre *PreChainError
	require.True(t, errors.As(err, &pre), "want *PreChainError, got %v", err)
	require.Equal(t, cascading, pre.Constraints)

	status, err := ReadChainState(ctx, plain, org)
	require.NoError(t, err)
	require.Equal(t, record.ChainNotStarted, status.State)
}

// One reading of chain state: each planted state is what ReadChainState
// reports, and what the writer acts on under the lock.
func TestRecordChainStateCell(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()

	write := func(org string) error {
		_, err := h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft()})
		return err
	}

	t.Run("notStarted", func(t *testing.T) {
		org := seedOrg(t, plain)
		status, err := ReadChainState(ctx, plain, org)
		require.NoError(t, err)
		require.Equal(t, record.ChainNotStarted, status.State)
		encoded, err := json.Marshal(status)
		require.NoError(t, err)
		require.JSONEq(t, `{"chainState":"notStarted","chainId":null,"head":null}`, string(encoded))
		wantWriteError(t, write(org), ClassExpansion, ReasonChainHead)
	})

	t.Run("extendable", func(t *testing.T) {
		org := seedOrg(t, plain)
		genesis, err := h.w.start(ctx, org)
		require.NoError(t, err)
		status, err := ReadChainState(ctx, plain, org)
		require.NoError(t, err)
		require.Equal(t, record.ChainExtendable, status.State)
		require.Equal(t, genesis.ChainID, *status.ChainID)
		require.NoError(t, write(org))
	})

	t.Run("notExtendable/head_mismatch", func(t *testing.T) {
		org := seedOrg(t, plain)
		_, err := h.w.start(ctx, org)
		require.NoError(t, err)
		_, err = plain.Exec(`UPDATE record_chains SET head_hash = repeat('0', 64) WHERE organization_id = $1`, org)
		require.NoError(t, err)
		status, err := ReadChainState(ctx, plain, org)
		require.NoError(t, err)
		require.Equal(t, record.ChainNotExtendable, status.State)
		require.Equal(t, NotExtendableHeadMismatch, status.Reason)
		wantWriteError(t, write(org), ClassExpansion, ReasonChainHead)
	})

	t.Run("notExtendable/no_records", func(t *testing.T) {
		org := seedOrg(t, plain)
		_, err := plain.Exec(`INSERT INTO record_chains (id, organization_id, key_id, head_seq, head_hash)
			VALUES ($1, $2, $3, 0, repeat('0', 64))`, uuid.NewString(), org, h.keys.publicKey().KeyID())
		require.NoError(t, err)
		status, err := ReadChainState(ctx, plain, org)
		require.NoError(t, err)
		require.Equal(t, record.ChainNotExtendable, status.State)
		require.Equal(t, NotExtendableNoRecords, status.Reason)
		wantWriteError(t, write(org), ClassExpansion, ReasonChainHead)
	})

	t.Run("notExtendable/head_mismatch/head_seq", func(t *testing.T) {
		org := seedOrg(t, plain)
		_, err := h.w.start(ctx, org)
		require.NoError(t, err)
		_, err = plain.Exec(`UPDATE record_chains SET head_seq = head_seq + 1 WHERE organization_id = $1`, org)
		require.NoError(t, err)
		status, err := ReadChainState(ctx, plain, org)
		require.NoError(t, err)
		require.Equal(t, record.ChainNotExtendable, status.State)
		require.Equal(t, NotExtendableHeadMismatch, status.Reason)
		wantWriteError(t, write(org), ClassExpansion, ReasonChainHead)
	})

	// The CHECK constraints of audit_records keep a stored record's hash and
	// payload type consistent, so each case drops the one in its way in a
	// transaction that is rolled back, and reads the chain and runs the
	// writer's append step in that transaction.
	for name, c := range map[string]struct{ constraint, change string }{
		"payload":      {"audit_records_hash_is_payload_digest", `payload = payload || '\x20'::bytea`},
		"payload_type": {"audit_records_payload_type_check", `payload_type = 'application/json'`},
	} {
		t.Run("notExtendable/record_modified/"+name, func(t *testing.T) {
			org := seedOrg(t, plain)
			_, err := h.w.start(ctx, org)
			require.NoError(t, err)
			require.NoError(t, write(org))
			tx, err := plain.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback() }()
			_, err = tx.ExecContext(ctx, `ALTER TABLE audit_records DROP CONSTRAINT `+c.constraint)
			require.NoError(t, err)
			res, err := tx.ExecContext(ctx, `UPDATE audit_records SET `+c.change+`
				WHERE (chain_id, seq) = (SELECT id, head_seq FROM record_chains WHERE organization_id = $1)`, org)
			require.NoError(t, err)
			changed, err := res.RowsAffected()
			require.NoError(t, err)
			require.Equal(t, int64(1), changed, "the head record was not changed")
			status, err := ReadChainState(ctx, tx, org)
			require.NoError(t, err)
			require.Equal(t, record.ChainNotExtendable, status.State)
			require.Equal(t, NotExtendableRecordModified, status.Reason)
			d := testDraft()
			_, reason, err := h.w.appendLocked(ctx, tx, org, &d, nil, time.Now(), time.Now().Add(time.Second), false)
			require.Error(t, err)
			require.Equal(t, ReasonChainHead, reason)
		})
	}

	t.Run("extendable/another_key", func(t *testing.T) {
		org := seedOrg(t, plain)
		_, err := h.w.start(ctx, org)
		require.NoError(t, err)
		other := newHarness(t, db, nil)
		_, err = other.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft()})
		wantWriteError(t, err, ClassExpansion, ReasonSigner)
		status, err := ReadChainState(ctx, plain, org)
		require.NoError(t, err)
		require.Equal(t, record.ChainExtendable, status.State)
		require.Equal(t, int64(0), status.Head.Seq, "a write under another key extended the chain")
		require.NoError(t, write(org), "the chain's own key no longer extends it")
	})
}

// Each class and reason is counted once per failed write, other series stay
// at zero, and each failure writes one SECURITY line with both. A reduction
// whose state change ran commits with a debt instead of failing: its
// SECURITY line names the debt, and the debt's own line comes first.
func TestRecordWriteFailuresCountOncePerClassAndReason(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	ctx := context.Background()

	healthy := newHarness(t, db, nil)
	okOrg := seedOrg(t, plain)
	_, err := healthy.w.start(ctx, okOrg)
	require.NoError(t, err)
	planted := seedOrg(t, plain)
	_, err = healthy.w.start(ctx, planted)
	require.NoError(t, err)
	_, err = plain.Exec(`UPDATE record_chains SET head_hash = repeat('0', 64) WHERE organization_id = $1`, planted)
	require.NoError(t, err)

	type fault struct {
		reason Reason
		run    func(h harness, class Class) (Appended, error)
	}
	faults := []fault{
		{ReasonChainHead, func(h harness, class Class) (Appended, error) {
			return h.w.Write(ctx, Write{Class: class, OrganizationID: planted, Draft: testDraft()})
		}},
		{ReasonCanonical, func(h harness, class Class) (Appended, error) {
			d := testDraft()
			d.Retained["score"] = 0.5
			return h.w.Write(ctx, Write{Class: class, OrganizationID: okOrg, Draft: d})
		}},
		{ReasonSigner, func(h harness, class Class) (Appended, error) {
			h.keys.fail = errors.New("injected signer fault")
			defer func() { h.keys.fail = nil }()
			return h.w.Write(ctx, Write{Class: class, OrganizationID: okOrg, Draft: testDraft()})
		}},
		{ReasonGuard, func(h harness, class Class) (Appended, error) {
			return h.w.Write(ctx, Write{Class: class, OrganizationID: okOrg, Draft: testDraft(),
				Guard: func(context.Context, time.Time, *Probe, *record.Draft) (*Stored, error) {
					return nil, errors.New("refused by the guard")
				}})
		}},
		{ReasonChainGuardProbe, func(h harness, class Class) (Appended, error) {
			return h.w.Write(ctx, Write{Class: class, OrganizationID: okOrg, Draft: testDraft(),
				Guard: func(ctx context.Context, _ time.Time, p *Probe, _ *record.Draft) (*Stored, error) {
					_, _ = p.Newest(ctx, MaxProbeRows+1)
					return nil, nil
				}})
		}},
		{ReasonConstraint, func(h harness, class Class) (Appended, error) {
			first, err := h.w.Write(ctx, Write{Class: class, OrganizationID: okOrg, Draft: testDraft()})
			if err != nil {
				return Appended{}, fmt.Errorf("setup write: %w", err)
			}
			d := testDraft()
			d.EventID = first.EventID
			return h.w.Write(ctx, Write{Class: class, OrganizationID: okOrg, Draft: d})
		}},
		{ReasonLockTimeout, func(h harness, class Class) (Appended, error) {
			holder, err := plain.Begin()
			if err != nil {
				return Appended{}, err
			}
			defer func() { _ = holder.Rollback() }()
			if _, err := holder.Exec(lockChainQuery, okOrg); err != nil {
				return Appended{}, err
			}
			return h.w.Write(ctx, Write{Class: class, OrganizationID: okOrg, Draft: testDraft()})
		}},
		{ReasonOther, func(h harness, class Class) (Appended, error) {
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			return h.w.Write(cancelled, Write{Class: class, OrganizationID: okOrg, Draft: testDraft()})
		}},
	}

	for _, class := range classes {
		for _, f := range faults {
			t.Run(string(class)+"/"+string(f.reason), func(t *testing.T) {
				h := newHarness(t, db, healthy.keys)
				out, err := f.run(h, class)

				one, total := h.failures(t, class, f.reason)
				require.Equal(t, 1.0, one)
				require.Equal(t, 1.0, total, "a failed write increments exactly one series")

				lines := h.logs.lines()
				if class == ClassReduction && f.reason != ReasonCanonical && f.reason != ReasonOther {
					// A reduction whose state change ran commits with a debt.
					require.NoError(t, err)
					require.NotNil(t, out.Debt)
					require.Equal(t, f.reason, out.Debt.Reason)
					require.NotEmpty(t, out.Debt.ID)
					require.Equal(t, 1.0, h.counter(t, "aim_record_debts_written_total"))
					require.Len(t, lines, 2)
					require.True(t, strings.HasPrefix(lines[0], EventRecordDebtWritten+" debt_id="+out.Debt.ID+" "), lines[0])
					lines = lines[1:]
					require.Equal(t, fmt.Sprintf("SECURITY %s class=%s reason=%s debt=%s",
						EventRecordWriteFailed, class, f.reason, out.Debt.ID), lines[0])
				} else {
					wantWriteError(t, err, class, f.reason)
					require.Equal(t, 0.0, h.counter(t, "aim_record_debts_written_total"))
					require.Len(t, lines, 1)
					require.Equal(t, fmt.Sprintf("SECURITY %s class=%s reason=%s debt=none", EventRecordWriteFailed, class, f.reason), lines[0])
				}
				require.NotContains(t, lines[0], okOrg)
				require.NotContains(t, lines[0], planted)
			})
		}
	}

	// The healthy organization still takes writes after every fault.
	_, err = healthy.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: okOrg, Draft: testDraft()})
	require.NoError(t, err)
}

// Blast radius. With organization A's append lock held past the lock timeout
// and A sending lock-taking writes faster than the pool size divided by the
// lock timeout, organization B's writes succeed and none waits as long as
// the lock timeout.
func TestRecordAppendLockCrossTenantBlastRadius(t *testing.T) {
	secret := make([]byte, 32)
	_, err := rand.Read(secret)
	require.NoError(t, err)
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_USER", "aim")
	t.Setenv("POSTGRES_DB", "aim_test")
	t.Setenv("JWT_SECRET", hex.EncodeToString(secret))
	cfg, err := config.Load()
	require.NoError(t, err)
	pool := cfg.Database.MaxConnections
	require.Positive(t, pool)

	db, _ := openTapped(t, pool)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	orgA, orgB := seedOrg(t, plain), seedOrg(t, plain)
	_, err = h.w.start(ctx, orgA)
	require.NoError(t, err)
	_, err = h.w.start(ctx, orgB)
	require.NoError(t, err)

	holder, err := plain.Begin()
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	_, err = holder.Exec(lockChainQuery, orgA)
	require.NoError(t, err)

	// A sends at four times the rate at which its writes would fill the
	// pool if each held a connection for the whole lock timeout.
	interval := LockTimeout / time.Duration(4*pool)
	flood, stopFlood := context.WithCancel(ctx)
	var aWrites sync.WaitGroup
	var aMu sync.Mutex
	aReasons := map[Reason]int{}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-flood.Done():
				return
			case <-ticker.C:
				aWrites.Add(1)
				go func() {
					defer aWrites.Done()
					_, err := h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: orgA, Draft: testDraft()})
					var we *WriteError
					aMu.Lock()
					if errors.As(err, &we) {
						aReasons[we.Reason]++
					} else if err == nil {
						aReasons["committed"]++
					}
					aMu.Unlock()
				}()
			}
		}
	}()

	// Let A's flood reach several times the pool size before B writes.
	time.Sleep(LockTimeout / 2)
	var slowest time.Duration
	for i := 0; i < 10; i++ {
		start := time.Now()
		_, err := h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: orgB, Draft: testDraft()})
		took := time.Since(start)
		require.NoError(t, err, "organization B's write %d", i)
		if took > slowest {
			slowest = took
		}
		time.Sleep(LockTimeout / 10)
	}
	stopFlood()
	aWrites.Wait()
	_ = holder.Rollback()

	t.Logf("pool %d, lock timeout %s, A send interval %s, A outcomes %v, B slowest %s",
		pool, LockTimeout, interval, aReasons, slowest)
	require.Less(t, slowest, LockTimeout, "an organization B write waited as long as the lock timeout")
	require.Positive(t, aReasons[ReasonLockTimeout], "A's writes never met the held lock")
}

// The append lock is taken before the guard runs: a record that another
// write appends to the chain while this one waits for the lock is seen by
// this write's guard, and this write links to it.
func TestRecordGuardProbeSeesAppendCommittedWhileWaitingForTheLock(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	org := seedOrg(t, plain)
	_, err := h.w.start(ctx, org)
	require.NoError(t, err)

	holding, release := make(chan struct{}), make(chan struct{})
	first := testDraft()
	firstDone := make(chan error, 1)
	go func() {
		_, err := h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: first,
			Guard: func(context.Context, time.Time, *Probe, *record.Draft) (*Stored, error) {
				close(holding)
				<-release
				return nil, nil
			}})
		firstDone <- err
	}()
	<-holding

	var seen []Stored
	secondDone := make(chan struct{})
	var second Appended
	var secondErr error
	go func() {
		defer close(secondDone)
		second, secondErr = h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft(),
			Guard: func(ctx context.Context, _ time.Time, p *Probe, _ *record.Draft) (*Stored, error) {
				var err error
				seen, err = p.Newest(ctx, 1)
				return nil, err
			}})
	}()
	waiting := waitForLockWaiters(t, plain, 1, 500*time.Millisecond)
	close(release)
	require.NoError(t, <-firstDone)
	<-secondDone
	require.True(t, waiting, "the second write never waited on the append lock")
	require.NoError(t, secondErr)
	require.Len(t, seen, 1)
	require.Equal(t, first.EventID, seen[0].EventID, "the guard did not see the record appended ahead of it")
	require.Equal(t, seen[0].Seq+1, second.Seq)
}

// A guard probe that blocks past the statement timeout fails that write with
// reason chain_guard_probe, and a write of the same chain waiting on the
// lock commits within the lock timeout.
func TestRecordGuardProbePastStatementTimeoutFailsOnlyThatWrite(t *testing.T) {
	db, tp := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	org := seedOrg(t, plain)
	_, err := h.w.start(ctx, org)
	require.NoError(t, err)
	tp.blockBefore(probeNewestQuery, LockTimeout+time.Second)

	probing := make(chan struct{})
	stuck := make(chan error, 1)
	go func() {
		_, err := h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft(),
			Guard: func(ctx context.Context, _ time.Time, p *Probe, _ *record.Draft) (*Stored, error) {
				close(probing)
				_, err := p.Newest(ctx, 1)
				return nil, err
			}})
		stuck <- err
	}()
	<-probing
	start := time.Now()
	_, err = h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft()})
	took := time.Since(start)
	require.NoError(t, err, "the waiting write did not commit")
	require.Less(t, took, LockTimeout)
	wantWriteError(t, <-stuck, ClassExpansion, ReasonChainGuardProbe)
	t.Logf("waiting write committed after %s", took)
}

// A holder that stalls between statements past the idle timeout is ended by
// the server, and a write of the same chain waiting on the lock commits
// within the lock timeout.
func TestRecordStalledLockHolderIsEndedByTheServer(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	org := seedOrg(t, plain)
	_, err := h.w.start(ctx, org)
	require.NoError(t, err)

	stalling := make(chan struct{})
	stalled := make(chan error, 1)
	go func() {
		_, err := h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft(),
			Guard: func(context.Context, time.Time, *Probe, *record.Draft) (*Stored, error) {
				close(stalling)
				time.Sleep(LockTimeout + time.Second)
				return nil, nil
			}})
		stalled <- err
	}()
	<-stalling
	start := time.Now()
	_, err = h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft()})
	took := time.Since(start)
	require.NoError(t, err, "the waiting write did not commit")
	require.Less(t, took, LockTimeout)
	wantWriteError(t, <-stalled, ClassExpansion, ReasonDatabase)
	t.Logf("waiting write committed after %s", took)
}

// Statements a write sends after the append lock statement returns, COMMIT
// included. Any change to these counts changes the lock hold time and fails
// here.
func TestRecordStatementsUnderTheAppendLock(t *testing.T) {
	db, tp := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	org := seedOrg(t, plain)
	_, err := h.w.start(ctx, org)
	require.NoError(t, err)

	underLock := func(t *testing.T) []string {
		t.Helper()
		got := tp.recorded()
		at := -1
		for i, s := range got {
			if s == lockChainQuery {
				at = i
			}
		}
		require.GreaterOrEqual(t, at, 0, "no lock statement was sent")
		return got[at+1:]
	}

	for _, hops := range []int{0, 1, 3} {
		t.Run(fmt.Sprintf("guard_probing_%d", hops), func(t *testing.T) {
			var guard Guard
			if hops > 0 {
				guard = func(ctx context.Context, _ time.Time, p *Probe, _ *record.Draft) (*Stored, error) {
					for i := 0; i < hops; i++ {
						if _, _, err := p.BySeq(ctx, int64(i)); err != nil {
							return nil, err
						}
					}
					return nil, nil
				}
			}
			tp.reset()
			_, err := h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft(), Guard: guard})
			require.NoError(t, err)
			statements := underLock(t)
			t.Logf("%d statements after the lock: %v", len(statements), statementLabels(statements))
			require.Len(t, statements, 4+hops)
			require.Equal(t, setStatementTimeoutQuery, statements[0])
			require.Equal(t, chainStateQuery, statements[1])
			require.Equal(t, appendQuery, statements[len(statements)-2])
			require.Equal(t, "COMMIT", statements[len(statements)-1])
		})
	}
}

// Every probe statement, and the chain-state read the writer runs under the
// lock, uses an index of audit_records and no sequential scan of it on a
// populated, analyzed chain. Without the indexes the same check fails.
func TestRecordProbeStatementsUseAnIndex(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	org := seedOrg(t, plain)
	genesis, err := h.w.start(ctx, org)
	require.NoError(t, err)

	const rows = 20000
	_, err = plain.Exec(`
INSERT INTO audit_records (chain_id, seq, event_id, record_type, recorded_at, record_hash,
                           payload_type, payload, key_id, signature)
SELECT $1, g, gen_random_uuid(), 'opena2a.administrative', NOW(),
       encode(sha256(convert_to('row ' || g, 'UTF8')), 'hex'),
       'application/vnd.opena2a.audit-record.v1+json', convert_to('row ' || g, 'UTF8'),
       repeat('a', 64), '\x01'::bytea
  FROM generate_series(1, $2) AS g`, genesis.ChainID, rows)
	require.NoError(t, err)
	_, err = plain.Exec(`ANALYZE audit_records`)
	require.NoError(t, err)

	statements := []struct {
		name  string
		query string
		args  []any
	}{
		{"newest", probeNewestQuery, []any{genesis.ChainID, MaxProbeRows}},
		{"by_seq", probeBySeqQuery, []any{genesis.ChainID, int64(rows / 2)}},
		{"by_event_id", probeByEventIDQuery, []any{genesis.ChainID, genesis.EventID}},
		{"chain_state", chainStateQuery, []any{org}},
	}
	check := func(q Querier) map[string]string {
		problems := map[string]string{}
		for _, s := range statements {
			var plan string
			require.NoError(t, q.QueryRowContext(ctx, "EXPLAIN (FORMAT JSON) "+s.query, s.args...).Scan(&plan))
			if p := planProblem(t, plan); p != "" {
				problems[s.name] = p
			}
		}
		return problems
	}

	require.Empty(t, check(plain), "with the indexes")

	// Negative control: the same check, in a transaction that drops the
	// indexes and is rolled back.
	tx, err := plain.Begin()
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.Exec(`ALTER TABLE audit_records DROP CONSTRAINT audit_records_pkey,
	                  DROP CONSTRAINT audit_records_event_unique_per_chain`)
	require.NoError(t, err)
	problems := check(tx)
	require.Len(t, problems, len(statements), "without the indexes every statement should scan: %v", problems)
}

// planProblem returns why an EXPLAIN (FORMAT JSON) plan is not bounded by an
// index of audit_records, or "".
func planProblem(t *testing.T, plan string) string {
	t.Helper()
	var doc []struct {
		Plan map[string]any `json:"Plan"`
	}
	require.NoError(t, json.Unmarshal([]byte(plan), &doc))
	var seq, indexed bool
	var walk func(node map[string]any)
	walk = func(node map[string]any) {
		if node["Relation Name"] == "audit_records" {
			switch node["Node Type"] {
			case "Seq Scan":
				seq = true
			case "Index Scan", "Index Only Scan", "Bitmap Heap Scan":
				indexed = true
			}
		}
		children, _ := node["Plans"].([]any)
		for _, c := range children {
			if m, ok := c.(map[string]any); ok {
				walk(m)
			}
		}
	}
	walk(doc[0].Plan)
	switch {
	case seq:
		return "sequential scan of audit_records"
	case !indexed:
		return "no index scan of audit_records"
	}
	return ""
}

// The periodic lock-wait line counts waits in fixed buckets and names no
// organization.
func TestRecordLockWaitLineCarriesNoOrganization(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	h := newHarness(t, db, nil)
	ctx := context.Background()
	org := seedOrg(t, plain)
	_, err := h.w.start(ctx, org)
	require.NoError(t, err)

	h.w.EmitLockWaits()
	const writes = 3
	for i := 0; i < writes; i++ {
		_, err := h.w.Write(ctx, Write{Class: ClassObservation, OrganizationID: org, Draft: testDraft()})
		require.NoError(t, err)
	}
	h.w.EmitLockWaits()

	lines := h.logs.lines()
	require.Len(t, lines, 2)
	fields := strings.Fields(lines[1])
	require.Equal(t, EventAppendLockWaits, fields[0])
	var names []string
	values := map[string]string{}
	for _, f := range fields[1:] {
		name, value, ok := strings.Cut(f, "=")
		require.True(t, ok, "field %q", f)
		names = append(names, name)
		values[name] = value
	}
	require.Equal(t, []string{"period_seconds", "le_0.001", "le_0.005", "le_0.01", "le_0.05", "le_0.1",
		"le_0.25", "le_0.5", "le_1", "le_2", "le_inf"}, names)
	require.Equal(t, fmt.Sprint(writes), values["le_inf"])
	for _, line := range lines {
		require.NotContains(t, line, org)
	}
}

// statementLabels names the writer's statements for a test log.
func statementLabels(statements []string) []string {
	names := map[string]string{
		setStatementTimeoutQuery: "set_statement_timeout",
		chainStateQuery:          "read_chain_state",
		probeNewestQuery:         "probe_newest",
		probeBySeqQuery:          "probe_by_seq",
		probeByEventIDQuery:      "probe_by_event_id",
		appendQuery:              "append_and_move_head",
	}
	out := make([]string, len(statements))
	for i, s := range statements {
		if name, ok := names[s]; ok {
			out[i] = name
		} else {
			out[i] = s
		}
	}
	return out
}
