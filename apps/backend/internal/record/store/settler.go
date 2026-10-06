package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// settleBatch bounds how many of one organization's debts a settler run
// settles, oldest first.
const settleBatch = 64

// settlerOrganizationsQuery is the settler's one statement that is not about
// one organization: it lists organization ids, and reads nothing else.
const settlerOrganizationsQuery = `SELECT id FROM organizations ORDER BY id`

// Settler appends the late records of open debts to their chains. Each
// settle is one transaction that deletes the debt row and appends the record
// built from it, under the chain's append lock; a settle that fails leaves
// the row as it was.
//
// It reads record_debts one organization at a time, every statement naming
// that organization, and computes the totals over organizations in memory.
type Settler struct {
	w *Writer
}

// NewSettler returns a settler that appends through w, sharing its
// connection slots, metrics and console.
func NewSettler(w *Writer) *Settler {
	return &Settler{w: w}
}

// SettleRun is what one settler run did and saw.
type SettleRun struct {
	// Open is the number of open debts over every organization after the
	// run, and OldestOpenSeconds the age of the oldest, 0 when none is open.
	Open              int64
	OldestOpenSeconds float64
	// Settled is the number of debts whose late record the run appended.
	Settled int
	// Success is true when the run listed the organizations and read and
	// counted every one's debts. A debt that does not settle leaves the run
	// a success: its write failure is counted, and it stays open.
	Success bool
}

// Run settles, for each organization, up to settleBatch of its open debts in
// the order they occurred, stopping at that organization's first debt that
// does not settle. It then sets the open-debt gauges and writes one console
// line, neither of which names an organization.
func (s *Settler) Run(ctx context.Context) SettleRun {
	run := SettleRun{Success: true}
	var oldest time.Time
	orgs, err := s.organizations(ctx)
	if err != nil {
		run.Success = false
	}
	for _, org := range orgs {
		settled, err := s.settleOrganization(ctx, org)
		run.Settled += settled
		if err != nil {
			run.Success = false
		}
		open, first, err := s.openDebts(ctx, org)
		if err != nil {
			run.Success = false
			continue
		}
		run.Open += open
		if open > 0 && (oldest.IsZero() || first.Before(oldest)) {
			oldest = first
		}
	}
	now := s.w.now()
	if !oldest.IsZero() {
		run.OldestOpenSeconds = max(now.Sub(oldest).Seconds(), 0)
	}

	m := s.w.metrics
	m.debtsSettled.Add(float64(run.Settled))
	m.debtsOpen.Set(float64(run.Open))
	m.debtOldestOpen.Set(run.OldestOpenSeconds)
	if run.Success {
		m.settlerLastSuccess.Set(float64(now.UnixNano()) / 1e9)
	}
	s.w.log.Printf("%s open=%d oldest_open_seconds=%s settled=%d success=%t", EventDebtSettlerRun,
		run.Open, strconv.FormatFloat(run.OldestOpenSeconds, 'f', 3, 64), run.Settled, run.Success)
	return run
}

// Loop runs the settler once every period until ctx ends.
func (s *Settler) Loop(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Run(ctx)
		}
	}
}

func (s *Settler) organizations(ctx context.Context) ([]string, error) {
	rows, err := s.w.db.QueryContext(ctx, settlerOrganizationsQuery)
	if err != nil {
		return nil, fmt.Errorf("store: list organizations: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("store: list organizations: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// settleOrganization settles the organization's oldest open debts. Its
// error reports a failed read of the debts, not a debt that did not settle.
func (s *Settler) settleOrganization(ctx context.Context, organizationID string) (int, error) {
	rows, err := s.w.db.QueryContext(ctx, openDebtIDsQuery, organizationID, settleBatch)
	if err != nil {
		return 0, fmt.Errorf("store: read open debts: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, fmt.Errorf("store: read open debts: %w", err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("store: read open debts: %w", err)
	}
	settled := 0
	for _, id := range ids {
		done, err := s.settle(ctx, organizationID, id)
		if err != nil {
			break
		}
		if done {
			settled++
		}
	}
	return settled, nil
}

// openDebts counts the organization's open debts and returns when the oldest
// occurred.
func (s *Settler) openDebts(ctx context.Context, organizationID string) (int64, time.Time, error) {
	var (
		open  int64
		first sql.NullTime
	)
	if err := s.w.db.QueryRowContext(ctx, openDebtsQuery, organizationID).Scan(&open, &first); err != nil {
		return 0, time.Time{}, fmt.Errorf("store: count open debts: %w", err)
	}
	return open, first.Time, nil
}

// settle appends one debt's late record and deletes the debt, in one
// transaction. It returns false and no error when the debt is gone or
// another settler is taking it. A failed append is a failed write of a
// reduction's record: it is counted, writes its SECURITY line, and leaves
// the debt open.
func (s *Settler) settle(ctx context.Context, organizationID, debtID string) (bool, error) {
	w := s.w
	waitStart := time.Now()
	deadline := waitStart.Add(LockTimeout)
	fail := func(reason Reason, err error) (bool, error) {
		if ctx.Err() != nil {
			reason = ReasonOther
		}
		w.noteFailure(ClassReduction, reason)
		w.log.Printf("SECURITY %s class=%s reason=%s settling=%s", EventRecordWriteFailed, ClassReduction, reason, debtID)
		return false, &WriteError{Class: ClassReduction, Reason: reason, err: err}
	}

	release, err := w.takeSlot(ctx, organizationID, deadline)
	if err != nil {
		w.observeWait(time.Since(waitStart))
		return fail(ReasonLockTimeout, err)
	}
	defer release()
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fail(ReasonDatabase, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.ExecContext(ctx, setSessionTimeoutsQuery,
		millis(time.Until(deadline)), millis(IdleInTransactionTimeout)); err != nil {
		return fail(ReasonDatabase, err)
	}
	owed, err := scanDebt(tx.QueryRowContext(ctx, takeDebtQuery, organizationID, debtID))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return false, nil
	case err != nil:
		return fail(ReasonDatabase, err)
	}
	d, err := owed.lateDraft(w.now())
	if err != nil {
		return fail(ReasonCanonical, err)
	}
	if err := addSalts(&d); err != nil {
		return fail(ReasonOther, err)
	}
	if _, reason, err := w.appendLocked(ctx, tx, organizationID, &d, nil, waitStart, deadline, true); reason != "" {
		return fail(reason, err)
	}
	if err := tx.Commit(); err != nil {
		return fail(ReasonDatabase, err)
	}
	committed = true
	w.health.succeeded(w.now())
	return true, nil
}
