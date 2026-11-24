package database

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ReplicaProbeTimeout bounds the freshness probe of ReplicaFence. A probe that
// is not answered in this time sends the read to the primary.
const ReplicaProbeTimeout = 250 * time.Millisecond

// OpenReplicaPool opens the pool for the read replica on dsn. It sets role the
// way OpenPool does, so a statement on the replica runs with the privileges
// and under the row-level security policies of a statement on the primary.
func OpenReplicaPool(dsn, role string, census *Census) (*sql.DB, error) {
	return openPool(dsn, role, census)
}

// ReplicaFence decides, for one read, whether the replica may answer it.
//
// A replica applies the primary's writes after a delay, so a read on it can
// still show an agent as verified, or a capability as granted, after the
// primary committed the revocation. The fence closes that gap without any
// state of its own: it reads the primary's current write-ahead log position
// and lets the replica answer only when it has replayed up to that position.
// Every transaction the primary had committed before the fence ran, a
// revocation included, is then visible to the read on the replica. When the
// replica is behind, cannot be asked, or is not a standby, the read goes to
// the primary.
type ReplicaFence struct {
	primary *sql.DB
	replica *sql.DB
	timeout time.Duration
}

// NewReplicaFence returns the fence between primary and replica.
func NewReplicaFence(primary, replica *sql.DB) (*ReplicaFence, error) {
	if primary == nil || replica == nil {
		return nil, errors.New("database: a replica fence needs a primary and a replica")
	}
	return &ReplicaFence{primary: primary, replica: replica, timeout: ReplicaProbeTimeout}, nil
}

// Replica reports whether the replica holds every write the primary had
// committed when Replica was called. Any failure to find out is false.
func (f *ReplicaFence) Replica(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	var position string
	if err := f.primary.QueryRowContext(ctx, `SELECT pg_current_wal_lsn()::text`).Scan(&position); err != nil {
		return false
	}
	var caughtUp sql.NullBool
	if err := f.replica.QueryRowContext(ctx,
		`SELECT pg_last_wal_replay_lsn() >= $1::pg_lsn`, position).Scan(&caughtUp); err != nil {
		return false
	}
	// pg_last_wal_replay_lsn is NULL on a server that is not a standby, which
	// leaves caughtUp invalid: that server is not a replica of anything.
	return caughtUp.Valid && caughtUp.Bool
}
