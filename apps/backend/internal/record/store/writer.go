// Package store keeps signed audit record chains in PostgreSQL. It starts an
// organization's chain with its genesis record, appends records to the chain
// under the chain's append lock, in the same transaction as the state change
// they record, and reads a chain's state and records back.
//
// A write runs in this order, and the append lock covers only the last part:
//
//  1. The draft is completed and serialized once, so a draft that cannot be
//     canonicalized fails before anything waits.
//  2. The write takes one of its chain's connection slots in this process,
//     opens a transaction and runs the caller's state change.
//  3. It takes the append lock by locking the chain's record_chains row.
//     From here every statement runs under StatementTimeout.
//  4. It reads the chain state, runs the caller's guard, signs the record,
//     appends it, moves the head and commits.
//
// A chain has connectionsPerChain slots per process, so writes of one
// organization never hold more of the pool than that, however fast they
// arrive or however long its lock is held: the other organizations' writes
// do not wait for a connection.
//
// A write that fails is refused whole: the state change rolls back with the
// record. The one exception is a reduction, which removes or narrows what
// someone may do: once its state change has run, a failure to append its
// record rolls back to a savepoint taken after the state change, and the
// state change commits with a record_debts row from which the debt settler
// appends the record later. Every failure is counted in
// aim_record_write_failures_total by the class of the write and a reason
// from a closed set, and writes one SECURITY console line with both.
package store

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/prometheus/client_golang/prometheus"
)

// The writer's timeouts. These are provisional values. Their order is what
// the writer relies on: StatementTimeout and IdleInTransactionTimeout are
// both below LockTimeout, so a holder stuck in one statement, or stalled
// between two, loses the lock before a waiting write of the same chain gives
// up.
const (
	// LockTimeout bounds how long a write waits for its chain's append lock,
	// in process and in the database together.
	LockTimeout = 2 * time.Second
	// StatementTimeout bounds every statement a write runs while it holds
	// the append lock.
	StatementTimeout = 500 * time.Millisecond
	// IdleInTransactionTimeout ends, at the server, a write's session that
	// sends nothing for this long inside its transaction. It is above one
	// round trip to the database plus the writer's own work between two
	// statements.
	IdleInTransactionTimeout = 1 * time.Second
)

// connectionsPerChain is how many writes of one chain hold a database
// connection at once in one process: the one holding the append lock and
// the next one, which runs its state change meanwhile.
const connectionsPerChain = 2

// Class is the kind of state change a record accompanies. The set is closed.
type Class string

const (
	ClassExpansion   Class = "expansion"
	ClassDestruction Class = "destruction"
	ClassReduction   Class = "reduction"
	ClassObservation Class = "observation"
)

var classes = []Class{ClassExpansion, ClassDestruction, ClassReduction, ClassObservation}

func (c Class) valid() bool {
	for _, k := range classes {
		if c == k {
			return true
		}
	}
	return false
}

// Reason is why a record write failed. The set is closed.
type Reason string

const (
	// ReasonLockTimeout: the append lock was not taken within LockTimeout.
	ReasonLockTimeout Reason = "lock_timeout"
	// ReasonChainHead: the organization's chain has not started, or its
	// head cannot be extended.
	ReasonChainHead Reason = "chain_head"
	// ReasonSigner: the key provider failed, or signed with a key the chain
	// does not name.
	ReasonSigner Reason = "signer"
	// ReasonCanonical: the draft cannot be serialized as a record.
	ReasonCanonical Reason = "canonical"
	// ReasonConstraint: the database refused the append on an integrity
	// constraint.
	ReasonConstraint Reason = "constraint"
	// ReasonGuard: the guard refused the write or broke its contract.
	ReasonGuard Reason = "guard"
	// ReasonChainGuardProbe: a statement of the guard's probe failed,
	// including by exceeding StatementTimeout.
	ReasonChainGuardProbe Reason = "chain_guard_probe"
	// ReasonDatabase: any other database failure.
	ReasonDatabase Reason = "database"
	// ReasonOther: anything else, such as a cancelled request.
	ReasonOther Reason = "other"
)

var reasons = []Reason{
	ReasonLockTimeout, ReasonChainHead, ReasonSigner, ReasonCanonical, ReasonConstraint,
	ReasonGuard, ReasonChainGuardProbe, ReasonDatabase, ReasonOther,
}

// ErrInvalidWrite is wrapped by the error of a write the writer refuses
// before it starts: an unknown class or a malformed organization id.
var ErrInvalidWrite = errors.New("store: invalid write")

// ErrChainStarted is returned by Start when the organization already has a
// chain.
var ErrChainStarted = errors.New("store: the organization's chain has already started")

// WriteError is the error of a record write that failed. The write's state
// change did not commit.
type WriteError struct {
	Class  Class
	Reason Reason
	err    error
}

func (e *WriteError) Error() string {
	return fmt.Sprintf("store: record write failed (class %s, reason %s): %v", e.Class, e.Reason, e.err)
}

func (e *WriteError) Unwrap() error { return e.err }

// Guard runs under the append lock, after the chain state is read and before
// the append. timestamp is the record's own timestamp, fixed before the lock.
// The guard may set or clear members of draft's three parts, using values
// computed before the lock; it may not change the draft's event id, type or
// timestamp. It reads the chain only through probe. It ends the write without
// an append by returning a record the probe read; it refuses the write by
// returning an error.
type Guard func(ctx context.Context, timestamp time.Time, probe *Probe, draft *record.Draft) (*Stored, error)

// Write is one record write.
type Write struct {
	Class          Class
	OrganizationID string
	// Draft is the record. A zero Timestamp is set to the writer's clock
	// before the lock. A part with members and no salt gets a random salt.
	Draft record.Draft
	// Apply, when set, runs the state change the record accompanies, in the
	// write's transaction and before the append lock. It runs statements
	// only: any network call completes before Write is called. If it fails,
	// the write ends with its error and nothing commits.
	Apply func(ctx context.Context, tx *sql.Tx) error
	// Guard, when set, runs under the append lock.
	Guard Guard
}

// Appended is what a write left in its chain.
type Appended struct {
	ChainID    string
	Seq        int64
	EventID    string
	RecordHash string
	// Existing is true when the guard ended the write with a record already
	// in the chain, and nothing was appended.
	Existing bool
	// Debt is set when the write was a reduction whose record could not be
	// appended: its state change committed without the record, and nothing
	// was appended.
	Debt *Debt
}

// Debt is what a reduction whose record could not be appended committed in
// its place.
type Debt struct {
	// ID is the id of its record_debts row, which is also the event id of the
	// late record the settler appends. It is empty when the row could not be
	// written either, and the state change committed with no record.
	ID string
	// Reason is why the record could not be appended.
	Reason Reason
}

// Config configures a Writer.
type Config struct {
	DB *sql.DB
	// Keys signs records with the deployment's record key; Key is that key's
	// public half, the key a chain this writer extends must name.
	Keys record.SigningKeyProvider
	Key  record.PublicKey
	// Metrics, when nil, are registered with a registry of their own.
	Metrics *Metrics
	// Logger receives the console lines. When nil, the standard logger does.
	Logger *log.Logger
	// Now is the clock. When nil, time.Now.
	Now func() time.Time
}

// Writer starts and appends to organizations' record chains.
type Writer struct {
	db      *sql.DB
	keys    record.SigningKeyProvider
	key     record.PublicKey
	metrics *Metrics
	log     *log.Logger
	now     func() time.Time
	waits   lockWaitPeriod
	health  pathHealth

	mu    sync.Mutex
	slots map[string]chan struct{}
}

// keyVerifier is the record.SignatureVerifier of the one key a writer's
// chains name. record.Sign already checks the signature under the provider's
// own public key; opening the envelope with keyVerifier checks that the key
// is also the chain's.
type keyVerifier struct{ key record.PublicKey }

func (v keyVerifier) VerifySignature(keyID string, message, signature []byte) error {
	if v.key.Alg != record.AlgEd25519 || len(v.key.Key) != ed25519.PublicKeySize {
		return errors.New("store: the chain's key is not an Ed25519 public key")
	}
	if keyID != v.key.KeyID() {
		return record.ErrUnknownKey
	}
	if !ed25519.Verify(ed25519.PublicKey(v.key.Key), message, signature) {
		return errors.New("store: the signature does not verify under the chain's key")
	}
	return nil
}

// NewWriter returns a writer.
func NewWriter(cfg Config) (*Writer, error) {
	if cfg.DB == nil || cfg.Keys == nil {
		return nil, errors.New("store: a writer needs a database and a key provider")
	}
	if cfg.Key.Alg != record.AlgEd25519 || len(cfg.Key.Key) == 0 {
		return nil, errors.New("store: a writer needs the public half of its record key")
	}
	w := &Writer{
		db:      cfg.DB,
		keys:    cfg.Keys,
		key:     record.PublicKey{Alg: cfg.Key.Alg, Key: append([]byte(nil), cfg.Key.Key...)},
		metrics: cfg.Metrics,
		log:     cfg.Logger,
		now:     cfg.Now,
		slots:   map[string]chan struct{}{},
	}
	if w.metrics == nil {
		m, err := NewMetrics(prometheus.NewRegistry())
		if err != nil {
			return nil, err
		}
		w.metrics = m
	}
	if w.log == nil {
		w.log = log.Default()
	}
	if w.now == nil {
		w.now = time.Now
	}
	return w, nil
}

// The writer's statements.
const (
	setSessionTimeoutsQuery = `SELECT set_config('lock_timeout', $1, true),
       set_config('idle_in_transaction_session_timeout', $2, true)`
	setLockTimeoutQuery      = `SELECT set_config('lock_timeout', $1, true)`
	setStatementTimeoutQuery = `SELECT set_config('statement_timeout', $1, true)`
	lockChainQuery           = `SELECT id FROM record_chains WHERE organization_id = $1 FOR UPDATE`
	appendQuery              = `
WITH appended AS (
    INSERT INTO audit_records (chain_id, seq, event_id, record_type, recorded_at, record_hash,
                               payload_type, payload, key_id, signature,
                               tenant_part, tenant_salt, personal_part, personal_salt)
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
    RETURNING chain_id, seq, record_hash)
UPDATE record_chains c
   SET head_seq = a.seq, head_hash = a.record_hash, updated_at = NOW()
  FROM appended a
 WHERE c.id = a.chain_id AND c.head_seq = $15`
	// A reduction's record is tried in this savepoint, taken after its state
	// change, so a failed append rolls back to the state change alone.
	savepointQuery           = `SAVEPOINT record_append`
	rollbackToSavepointQuery = `ROLLBACK TO SAVEPOINT record_append`
)

// placeholderHead lets a draft be serialized before its chain position is
// known, so that a draft that cannot be canonicalized fails before the lock.
var placeholderHead = record.Head{
	ChainID: "00000000-0000-0000-0000-000000000000",
	Hash:    "0000000000000000000000000000000000000000000000000000000000000000",
}

// Write appends one record to the organization's chain, in one transaction
// with req.Apply. A *WriteError reports a failed record write; any other
// error is req.Apply's or a refusal of the request itself.
//
// A reduction is the exception. Its draft must be one a record_debts row can
// hold (debtFromDraft), or it is refused before anything waits, with reason
// canonical. Once its state change has run, a failure to append its record
// is counted and logged as any other, but Write returns no error: the state
// change commits with a debt, reported in Appended.Debt.
func (w *Writer) Write(ctx context.Context, req Write) (Appended, error) {
	if !req.Class.valid() {
		return Appended{}, fmt.Errorf("%w: %q is not a write class", ErrInvalidWrite, req.Class)
	}
	if !canonicalUUID(req.OrganizationID) {
		return Appended{}, fmt.Errorf("%w: the organization id is not a lowercase hyphenated UUID", ErrInvalidWrite)
	}
	out, reason, err := w.write(ctx, req)
	if reason != "" {
		if ctx.Err() != nil {
			reason = ReasonOther
		}
		w.noteFailure(req.Class, reason)
		if out.Debt != nil {
			out.Debt.Reason = reason
			id := out.Debt.ID
			if id == "" {
				id = "none"
			}
			w.log.Printf("SECURITY %s class=%s reason=%s debt=%s", EventRecordWriteFailed, req.Class, reason, id)
			return out, nil
		}
		w.log.Printf("SECURITY %s class=%s reason=%s", EventRecordWriteFailed, req.Class, reason)
		return Appended{}, &WriteError{Class: req.Class, Reason: reason, err: err}
	}
	if err == nil {
		w.health.succeeded(w.now())
	}
	return out, err
}

// noteFailure counts one failed record write.
func (w *Writer) noteFailure(class Class, reason Reason) {
	w.metrics.failures.WithLabelValues(string(class), string(reason)).Inc()
	w.health.failed(w.now(), class, reason)
}

// write returns a reason when the record write failed, and an error with no
// reason when req.Apply did. A reduction that committed with a debt returns
// its reason and an Appended whose Debt is set.
func (w *Writer) write(ctx context.Context, req Write) (Appended, Reason, error) {
	d := cloneDraft(req.Draft)
	if d.Timestamp.IsZero() {
		d.Timestamp = w.now()
	}
	d.Timestamp = d.Timestamp.UTC().Truncate(time.Microsecond)
	if err := addSalts(&d); err != nil {
		return Appended{}, ReasonOther, err
	}
	if _, err := record.NewRecord(placeholderHead, d); err != nil {
		return Appended{}, ReasonCanonical, err
	}
	var owed *debt
	if req.Class == ClassReduction {
		row, err := debtFromDraft(req.OrganizationID, d)
		if err != nil {
			return Appended{}, ReasonCanonical, err
		}
		owed = &row
	}

	waitStart := time.Now()
	deadline := waitStart.Add(LockTimeout)
	release, err := w.takeSlot(ctx, req.OrganizationID, deadline)
	if err != nil {
		w.observeWait(time.Since(waitStart))
		if owed != nil {
			return w.commitDebtOnly(ctx, req, *owed, err)
		}
		return Appended{}, ReasonLockTimeout, err
	}
	defer release()

	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return Appended{}, ReasonDatabase, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.ExecContext(ctx, setSessionTimeoutsQuery,
		millis(time.Until(deadline)), millis(IdleInTransactionTimeout)); err != nil {
		return Appended{}, ReasonDatabase, err
	}
	if req.Apply != nil {
		if err := req.Apply(ctx, tx); err != nil {
			return Appended{}, "", err
		}
	}
	if owed != nil {
		if _, err := tx.ExecContext(ctx, savepointQuery); err != nil {
			return Appended{}, ReasonDatabase, err
		}
	}

	out, reason, err := w.appendLocked(ctx, tx, req.OrganizationID, &d, req.Guard, waitStart, deadline, req.Apply != nil)
	if reason == "" {
		if err := tx.Commit(); err != nil {
			return Appended{}, ReasonDatabase, err
		}
		committed = true
		return out, "", nil
	}
	if owed == nil {
		return Appended{}, reason, err
	}

	// The reduction commits without its record, with a debt.
	if _, rerr := tx.ExecContext(ctx, rollbackToSavepointQuery); rerr != nil {
		return Appended{}, reason, errors.Join(err, rerr)
	}
	debtID := w.insertDebt(ctx, tx, *owed)
	if cerr := tx.Commit(); cerr != nil {
		return Appended{}, reason, errors.Join(err, cerr)
	}
	committed = true
	return w.debtCommitted(*owed, debtID), reason, err
}

// commitDebtOnly runs a reduction whose write found no free connection slot
// of its chain within the lock timeout: it takes one of the chain's debt
// slots instead, runs the state change and commits it with a debt, without
// waiting for the append lock.
func (w *Writer) commitDebtOnly(ctx context.Context, req Write, owed debt, cause error) (Appended, Reason, error) {
	release, err := w.takeSlot(ctx, debtSlotKey(req.OrganizationID), time.Now().Add(LockTimeout))
	if err != nil {
		return Appended{}, ReasonLockTimeout, errors.Join(cause, err)
	}
	defer release()
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return Appended{}, ReasonLockTimeout, errors.Join(cause, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.ExecContext(ctx, setSessionTimeoutsQuery,
		millis(LockTimeout), millis(IdleInTransactionTimeout)); err != nil {
		return Appended{}, ReasonLockTimeout, errors.Join(cause, err)
	}
	if req.Apply != nil {
		if err := req.Apply(ctx, tx); err != nil {
			return Appended{}, "", err
		}
	}
	if _, err := tx.ExecContext(ctx, savepointQuery); err != nil {
		return Appended{}, ReasonLockTimeout, errors.Join(cause, err)
	}
	debtID := w.insertDebt(ctx, tx, owed)
	if err := tx.Commit(); err != nil {
		return Appended{}, ReasonLockTimeout, errors.Join(cause, err)
	}
	committed = true
	return w.debtCommitted(owed, debtID), ReasonLockTimeout, cause
}

// insertDebt writes the debt row inside the savepoint. When the insert
// fails it rolls back to the savepoint, so the state change still commits,
// and returns an empty id.
func (w *Writer) insertDebt(ctx context.Context, tx *sql.Tx, owed debt) string {
	args, err := owed.args()
	if err == nil {
		_, err = tx.ExecContext(ctx, insertDebtQuery, args...)
	}
	if err != nil {
		_, _ = tx.ExecContext(ctx, rollbackToSavepointQuery)
		return ""
	}
	return owed.id
}

// debtCommitted counts and logs a debt once the transaction that wrote it
// committed, and returns what the write reports. debtID is empty when the
// row could not be written.
func (w *Writer) debtCommitted(owed debt, debtID string) Appended {
	if debtID != "" {
		w.metrics.debtsWritten.Inc()
		w.log.Print(owed.line())
	}
	return Appended{EventID: owed.id, Debt: &Debt{ID: debtID}}
}

func debtSlotKey(organizationID string) string { return "debt/" + organizationID }

// appendLocked takes the append lock in tx and appends d to the
// organization's chain. It does not commit. When applied is true, a state
// change ran in tx first, and the lock wait is bounded by what remains of
// the deadline.
func (w *Writer) appendLocked(ctx context.Context, tx *sql.Tx, organizationID string, d *record.Draft,
	guard Guard, waitStart, deadline time.Time, applied bool) (Appended, Reason, error) {
	if applied {
		remaining := time.Until(deadline)
		if remaining < time.Millisecond {
			w.observeWait(time.Since(waitStart))
			return Appended{}, ReasonLockTimeout, errors.New("store: the lock wait budget ran out before the lock")
		}
		if _, err := tx.ExecContext(ctx, setLockTimeoutQuery, millis(remaining)); err != nil {
			return Appended{}, ReasonDatabase, err
		}
	}

	// The append lock. Everything after this statement is covered by it.
	var chainID string
	err := tx.QueryRowContext(ctx, lockChainQuery, organizationID).Scan(&chainID)
	w.observeWait(time.Since(waitStart))
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Appended{}, ReasonChainHead, errors.New("store: the organization's chain has not started")
	case pqCode(err) == "55P03":
		return Appended{}, ReasonLockTimeout, err
	case err != nil:
		return Appended{}, ReasonDatabase, err
	}
	if _, err := tx.ExecContext(ctx, setStatementTimeoutQuery, millis(StatementTimeout)); err != nil {
		return Appended{}, ReasonDatabase, err
	}

	status, err := ReadChainState(ctx, tx, organizationID)
	if err != nil {
		return Appended{}, ReasonDatabase, err
	}
	if status.State != record.ChainExtendable || *status.ChainID != chainID {
		return Appended{}, ReasonChainHead, fmt.Errorf("store: the chain is %s (%s)", status.State, status.Reason)
	}
	if status.keyID != w.key.KeyID() {
		return Appended{}, ReasonSigner, errors.New("store: the writer's key is not the key the chain names")
	}
	head := record.Head{ChainID: chainID, Seq: status.Head.Seq, Hash: status.Head.Hash}

	if guard != nil {
		existing, reason, err := runGuard(ctx, guard, tx, chainID, d)
		if reason != "" {
			return Appended{}, reason, err
		}
		if existing != nil {
			return Appended{ChainID: chainID, Seq: existing.Seq, EventID: existing.EventID,
				RecordHash: existing.RecordHash, Existing: true}, "", nil
		}
	}

	body, err := record.NewRecord(head, *d)
	if err != nil {
		return Appended{}, ReasonCanonical, err
	}
	rec, err := record.Sign(ctx, body, w.keys)
	if err != nil {
		return Appended{}, ReasonSigner, err
	}
	if _, err := record.Open(rec.Envelope, record.ClassRecordV1, keyVerifier{w.key}); err != nil {
		return Appended{}, ReasonSigner, fmt.Errorf("store: the record's signature is not by the chain's key: %w", err)
	}
	signature, err := base64.StdEncoding.DecodeString(rec.Envelope.Signatures[0].Sig)
	if err != nil {
		return Appended{}, ReasonSigner, err
	}
	next := body.Head()
	res, err := tx.ExecContext(ctx, appendQuery,
		chainID, next.Seq, d.EventID, d.Type, d.Timestamp, next.Hash,
		rec.Envelope.PayloadType, body.Canonical(), rec.Envelope.Signatures[0].KeyID, signature,
		nullable(rec.TenantPart), nullable(rec.TenantSalt), nullable(rec.PersonalPart), nullable(rec.PersonalSalt),
		head.Seq)
	if err != nil {
		if code := pqCode(err); len(code) == 5 && code[:2] == "23" {
			return Appended{}, ReasonConstraint, err
		}
		return Appended{}, ReasonDatabase, err
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return Appended{}, ReasonChainHead, errors.New("store: the head moved under the append lock")
	}
	return Appended{ChainID: chainID, Seq: next.Seq, EventID: d.EventID, RecordHash: next.Hash}, "", nil
}

// runGuard calls the guard with a probe open only for the call, and checks
// what it returned.
func runGuard(ctx context.Context, guard Guard, tx *sql.Tx, chainID string, d *record.Draft) (*Stored, Reason, error) {
	probe := &Probe{q: tx, chainID: chainID, open: true}
	eventID, recordType, timestamp := d.EventID, d.Type, d.Timestamp
	existing, err := guard(ctx, timestamp, probe, d)
	probe.open = false
	switch {
	case probe.err != nil:
		return nil, ReasonChainGuardProbe, probe.err
	case err != nil:
		return nil, ReasonGuard, err
	case d.EventID != eventID || d.Type != recordType || !d.Timestamp.Equal(timestamp):
		return nil, ReasonGuard, errors.New("store: the guard changed the record's event id, type or timestamp")
	case existing != nil && existing.ChainID != chainID:
		return nil, ReasonGuard, errors.New("store: the guard returned a record of another chain")
	}
	if existing == nil {
		if err := addSalts(d); err != nil {
			return nil, ReasonOther, err
		}
		dropUnusedSalts(d)
	}
	return existing, "", nil
}

// takeSlot waits, until deadline, for one of the chain's connection slots in
// this process, and returns its release.
func (w *Writer) takeSlot(ctx context.Context, organizationID string, deadline time.Time) (func(), error) {
	w.mu.Lock()
	slot, ok := w.slots[organizationID]
	if !ok {
		slot = make(chan struct{}, connectionsPerChain)
		w.slots[organizationID] = slot
	}
	w.mu.Unlock()
	release := func() { <-slot }
	select {
	case slot <- struct{}{}:
		return release, nil
	default:
	}
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case slot <- struct{}{}:
		return release, nil
	case <-timer.C:
		return nil, errors.New("store: no connection slot of the chain came free within the lock timeout")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (w *Writer) observeWait(d time.Duration) {
	w.metrics.lockWait.Observe(d.Seconds())
	w.waits.observe(d.Seconds())
}

// PreChainError is returned by Start while a foreign key would remove rows of
// audit_logs or verification_events by cascade. Such a removal would take
// chained and pre-genesis rows with it and leave no record, so no chain
// starts until the migration that replaces those foreign keys is applied.
type PreChainError struct {
	// Constraints names each cascading foreign key as table.constraint.
	Constraints []string
}

func (e *PreChainError) Error() string {
	return fmt.Sprintf("store: no chain starts while %d foreign keys remove audit rows by cascade: %v",
		len(e.Constraints), e.Constraints)
}

// CascadingForeignKeys names every foreign key of audit_logs and
// verification_events that removes their rows by cascade.
func CascadingForeignKeys(ctx context.Context, q Querier) ([]string, error) {
	rows, err := q.QueryContext(ctx, `
SELECT c.conrelid::regclass::text || '.' || c.conname
  FROM pg_constraint c
 WHERE c.contype = 'f'
   AND c.confdeltype = 'c'
   AND c.conrelid IN ('audit_logs'::regclass, 'verification_events'::regclass)
 ORDER BY 1`)
	if err != nil {
		return nil, fmt.Errorf("store: read foreign keys: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("store: read foreign keys: %w", err)
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// Start writes the genesis of the organization's chain. The genesis names the
// writer's key as the chain's first key and pins every row of audit_logs and
// verification_events the organization has by table, id and timestamp.
//
// It refuses with *PreChainError while CascadingForeignKeys names any foreign
// key, and with ErrChainStarted when the organization already has a chain.
func (w *Writer) Start(ctx context.Context, organizationID string) (Appended, error) {
	if !canonicalUUID(organizationID) {
		return Appended{}, fmt.Errorf("%w: the organization id is not a lowercase hyphenated UUID", ErrInvalidWrite)
	}
	cascading, err := CascadingForeignKeys(ctx, w.db)
	if err != nil {
		return Appended{}, err
	}
	if len(cascading) > 0 {
		return Appended{}, &PreChainError{Constraints: cascading}
	}
	return w.start(ctx, organizationID)
}

// start writes a genesis without checking foreign keys.
func (w *Writer) start(ctx context.Context, organizationID string) (Appended, error) {
	release, err := w.takeSlot(ctx, organizationID, time.Now().Add(LockTimeout))
	if err != nil {
		return Appended{}, err
	}
	defer release()

	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return Appended{}, fmt.Errorf("store: start chain: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, setSessionTimeoutsQuery,
		millis(LockTimeout), millis(IdleInTransactionTimeout)); err != nil {
		return Appended{}, fmt.Errorf("store: start chain: %w", err)
	}

	pre, err := preGenesisRows(ctx, tx, organizationID)
	if err != nil {
		return Appended{}, err
	}
	chainID, eventID := uuid.NewString(), uuid.NewString()
	timestamp := w.now().UTC().Truncate(time.Microsecond)
	body, err := record.NewGenesis(record.Genesis{
		ChainID:    chainID,
		FirstKey:   w.key,
		PreGenesis: pre,
		Draft:      record.Draft{EventID: eventID, Timestamp: timestamp},
	})
	if err != nil {
		return Appended{}, err
	}
	rec, err := record.Sign(ctx, body, w.keys)
	if err != nil {
		return Appended{}, err
	}
	signature, err := base64.StdEncoding.DecodeString(rec.Envelope.Signatures[0].Sig)
	if err != nil {
		return Appended{}, err
	}
	head := body.Head()
	if _, err := tx.ExecContext(ctx, `
INSERT INTO record_chains (id, organization_id, key_id, head_seq, head_hash)
VALUES ($1, $2, $3, $4, $5)`, chainID, organizationID, w.key.KeyID(), head.Seq, head.Hash); err != nil {
		if pqCode(err) == "23505" {
			return Appended{}, ErrChainStarted
		}
		return Appended{}, fmt.Errorf("store: start chain: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO audit_records (chain_id, seq, event_id, record_type, recorded_at, record_hash,
                           payload_type, payload, key_id, signature)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		chainID, head.Seq, eventID, record.TypeChainGenesis, timestamp,
		head.Hash, rec.Envelope.PayloadType, body.Canonical(), rec.Envelope.Signatures[0].KeyID, signature); err != nil {
		return Appended{}, fmt.Errorf("store: start chain: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Appended{}, fmt.Errorf("store: start chain: %w", err)
	}
	return Appended{ChainID: chainID, Seq: head.Seq, EventID: eventID, RecordHash: head.Hash}, nil
}

// preGenesisRows reads, in one snapshot, every row the organization has in
// the two tables a genesis pins, with the timestamp written at insert.
func preGenesisRows(ctx context.Context, q Querier, organizationID string) ([]record.PreGenesisRow, error) {
	rows, err := q.QueryContext(ctx, `
SELECT 'audit_logs', id, "timestamp" FROM audit_logs WHERE organization_id = $1
UNION ALL
SELECT 'verification_events', id, created_at FROM verification_events WHERE organization_id = $1`,
		organizationID)
	if err != nil {
		return nil, fmt.Errorf("store: read pre-genesis rows: %w", err)
	}
	defer rows.Close()
	var out []record.PreGenesisRow
	for rows.Next() {
		var r record.PreGenesisRow
		if err := rows.Scan(&r.Table, &r.ID, &r.Timestamp); err != nil {
			return nil, fmt.Errorf("store: read pre-genesis rows: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// cloneDraft copies the draft's maps one level deep, and the "opena2a"
// object inside each, so that a guard's edits never reach the caller's
// draft.
func cloneDraft(d record.Draft) record.Draft {
	d.Retained = cloneMembers(d.Retained)
	d.Tenant = cloneMembers(d.Tenant)
	d.Personal = cloneMembers(d.Personal)
	d.TenantSalt = append([]byte(nil), d.TenantSalt...)
	d.PersonalSalt = append([]byte(nil), d.PersonalSalt...)
	return d
}

func cloneMembers(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if inner, ok := v.(map[string]any); ok && k == "opena2a" {
			v = cloneMembers(inner)
		}
		out[k] = v
	}
	return out
}

// addSalts gives each part that has members and no salt a random salt.
func addSalts(d *record.Draft) error {
	for _, p := range []struct {
		members map[string]any
		salt    *[]byte
	}{{d.Tenant, &d.TenantSalt}, {d.Personal, &d.PersonalSalt}} {
		if len(p.members) > 0 && len(*p.salt) == 0 {
			*p.salt = make([]byte, record.SaltSize)
			if _, err := rand.Read(*p.salt); err != nil {
				return fmt.Errorf("store: salt: %w", err)
			}
		}
	}
	return nil
}

// dropUnusedSalts removes the salt of a part the guard emptied.
func dropUnusedSalts(d *record.Draft) {
	if len(d.Tenant) == 0 {
		d.TenantSalt = nil
	}
	if len(d.Personal) == 0 {
		d.PersonalSalt = nil
	}
}

func millis(d time.Duration) string {
	ms := (d + time.Millisecond - 1) / time.Millisecond
	if ms < 1 {
		ms = 1
	}
	return strconv.FormatInt(int64(ms), 10)
}

func nullable(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

func pqCode(err error) string {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return string(pqErr.Code)
	}
	return ""
}
