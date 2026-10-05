package record

import (
	"fmt"
	"sort"
	"time"
)

// The two tables whose rows can exist before a chain starts.
const (
	TableAuditLogs          = "audit_logs"
	TableVerificationEvents = "verification_events"
)

// PreGenesisRow names one row that existed before a chain started: its table,
// its id and the timestamp written when it was inserted. Nothing of the row's
// content is named.
type PreGenesisRow struct {
	Table string
	// ID is the row's UUID in lowercase hyphenated form.
	ID        string
	Timestamp time.Time
}

// SetDigest is the lowercase hex SHA-256 over the canonical serialization of
// the array of [table, id, timestamp] for every given row, sorted by table
// name and then by id, both as byte strings. For no rows it is the SHA-256 of
// the canonical serialization of the empty array.
//
// It pins which rows existed and when, as stored at genesis. It does not pin
// their content.
//
// Open question: whether a digest over table, id and timestamp alone is what
// the genesis commits to awaits a final security ruling; this is the current
// position.
func SetDigest(rows []PreGenesisRow) (string, error) {
	set, err := preGenesisSet(rows)
	if err != nil {
		return "", err
	}
	return hashHex(set), nil
}

// preGenesisSet returns the bytes SetDigest hashes.
func preGenesisSet(rows []PreGenesisRow) ([]byte, error) {
	sorted, err := sortedPreGenesis(rows)
	if err != nil {
		return nil, err
	}
	entries := make([]any, 0, len(sorted))
	for _, r := range sorted {
		ts, err := FormatTimestamp(r.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("%w: pre-genesis row %s %s: %v", ErrInvalidRecord, r.Table, r.ID, err)
		}
		entries = append(entries, []any{r.Table, r.ID, ts})
	}
	set, err := canonicalJSON(entries)
	if err != nil {
		return nil, fmt.Errorf("%w: pre-genesis set: %v", ErrInvalidRecord, err)
	}
	return set, nil
}

// preGenesisMember builds opena2a.pre_genesis: the count of pre-genesis rows
// per table, the newest of their timestamps (null for no rows) and the set
// digest.
func preGenesisMember(rows []PreGenesisRow) (map[string]any, error) {
	digest, err := SetDigest(rows)
	if err != nil {
		return nil, err
	}
	counts := map[string]any{TableAuditLogs: 0, TableVerificationEvents: 0}
	var newest any
	var newestAt time.Time
	for _, r := range rows {
		counts[r.Table] = counts[r.Table].(int) + 1
		if at := r.Timestamp.UTC().Truncate(time.Microsecond); newest == nil || at.After(newestAt) {
			ts, err := FormatTimestamp(at)
			if err != nil {
				return nil, fmt.Errorf("%w: pre-genesis row %s %s: %v", ErrInvalidRecord, r.Table, r.ID, err)
			}
			newest, newestAt = ts, at
		}
	}
	return map[string]any{
		"row_counts":       counts,
		"newest_timestamp": newest,
		"set_digest":       digest,
	}, nil
}

// sortedPreGenesis validates the rows and returns them sorted by table and
// then by id.
func sortedPreGenesis(rows []PreGenesisRow) ([]PreGenesisRow, error) {
	sorted := append([]PreGenesisRow(nil), rows...)
	for _, r := range sorted {
		if r.Table != TableAuditLogs && r.Table != TableVerificationEvents {
			return nil, fmt.Errorf("%w: %q is not a table that holds pre-genesis rows", ErrInvalidRecord, r.Table)
		}
		if !isCanonicalUUID(r.ID) {
			return nil, fmt.Errorf("%w: a pre-genesis row id in %s is not a lowercase hyphenated UUID", ErrInvalidRecord, r.Table)
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Table != sorted[j].Table {
			return sorted[i].Table < sorted[j].Table
		}
		return sorted[i].ID < sorted[j].ID
	})
	for i := 1; i < len(sorted); i++ {
		if sorted[i].Table == sorted[i-1].Table && sorted[i].ID == sorted[i-1].ID {
			return nil, fmt.Errorf("%w: pre-genesis row %s %s is named twice", ErrInvalidRecord, sorted[i].Table, sorted[i].ID)
		}
	}
	return sorted, nil
}
