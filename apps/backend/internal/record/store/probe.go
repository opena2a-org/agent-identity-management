package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// MaxProbeRows bounds the rows one probe statement returns.
const MaxProbeRows = 32

// ErrProbeClosed is returned by a probe used after its guard returned.
var ErrProbeClosed = errors.New("store: the probe is used outside its guard")

// Stored is a record of a chain as a probe reads it.
type Stored struct {
	ChainID    string
	Seq        int64
	EventID    string
	Type       string
	Timestamp  time.Time
	RecordHash string
	// Canonical is the record's canonical bytes: its retained part.
	Canonical []byte
}

// Probe is a guard's read access to the chain it guards. Every method is one
// statement over that chain's rows, bounded by an index, run inside the
// write's transaction under the append lock, so it sees every record
// appended to the chain before the lock was taken. Any error a method returns
// fails the write with reason chain_guard_probe, whatever the guard does with
// it.
type Probe struct {
	q       Querier
	chainID string
	open    bool
	err     error
}

const probeColumns = `chain_id, seq, event_id, record_type, recorded_at, record_hash, payload`

const (
	probeNewestQuery = `SELECT ` + probeColumns + ` FROM audit_records
 WHERE chain_id = $1 ORDER BY seq DESC LIMIT $2`
	probeBySeqQuery = `SELECT ` + probeColumns + ` FROM audit_records
 WHERE chain_id = $1 AND seq = $2`
	probeByEventIDQuery = `SELECT ` + probeColumns + ` FROM audit_records
 WHERE chain_id = $1 AND event_id = $2`
)

// Newest returns the chain's newest n records, newest first. n is between 1
// and MaxProbeRows.
func (p *Probe) Newest(ctx context.Context, n int) ([]Stored, error) {
	if err := p.usable(); err != nil {
		return nil, err
	}
	if n < 1 || n > MaxProbeRows {
		return nil, p.fail(fmt.Errorf("store: a probe reads between 1 and %d records, not %d", MaxProbeRows, n))
	}
	rows, err := p.q.QueryContext(ctx, probeNewestQuery, p.chainID, n)
	if err != nil {
		return nil, p.fail(err)
	}
	defer rows.Close()
	var out []Stored
	for rows.Next() {
		s, err := scanStored(rows)
		if err != nil {
			return nil, p.fail(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, p.fail(err)
	}
	return out, nil
}

// BySeq returns the chain's record at seq. The second result is false when
// there is none.
func (p *Probe) BySeq(ctx context.Context, seq int64) (Stored, bool, error) {
	return p.one(ctx, probeBySeqQuery, seq)
}

// ByEventID returns the chain's record whose event_id is eventID. The second
// result is false when there is none.
func (p *Probe) ByEventID(ctx context.Context, eventID string) (Stored, bool, error) {
	if !canonicalUUID(eventID) {
		if err := p.usable(); err != nil {
			return Stored{}, false, err
		}
		return Stored{}, false, p.fail(errors.New("store: a probe's event id is not a lowercase hyphenated UUID"))
	}
	return p.one(ctx, probeByEventIDQuery, eventID)
}

func (p *Probe) one(ctx context.Context, query string, key any) (Stored, bool, error) {
	if err := p.usable(); err != nil {
		return Stored{}, false, err
	}
	rows, err := p.q.QueryContext(ctx, query, p.chainID, key)
	if err != nil {
		return Stored{}, false, p.fail(err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return Stored{}, false, p.fail(err)
		}
		return Stored{}, false, nil
	}
	s, err := scanStored(rows)
	if err != nil {
		return Stored{}, false, p.fail(err)
	}
	return s, true, nil
}

func (p *Probe) usable() error {
	if !p.open {
		return p.fail(ErrProbeClosed)
	}
	return nil
}

// fail keeps the first error the probe returned.
func (p *Probe) fail(err error) error {
	if p.err == nil {
		p.err = err
	}
	return err
}

func scanStored(rows *sql.Rows) (Stored, error) {
	var s Stored
	if err := rows.Scan(&s.ChainID, &s.Seq, &s.EventID, &s.Type, &s.Timestamp, &s.RecordHash, &s.Canonical); err != nil {
		return Stored{}, err
	}
	s.Timestamp = s.Timestamp.UTC()
	return s, nil
}
