package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

type MCPManifestRepository struct {
	db *sql.DB
}

func NewMCPManifestRepository(db *sql.DB) *MCPManifestRepository {
	return &MCPManifestRepository{db: db}
}

// txQuerier is the subset of *sql.DB / *sql.Tx the manifest read/write helpers use, so the same SQL
// serves both the standalone (auto-commit) public methods and the single-transaction atomic recompute
// path (AtomicRecomputeWellKnown). Both *sql.DB and *sql.Tx satisfy it.
type txQuerier interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

// GetCurrentBaseline returns the current baseline for a server and its stored manifest entries.
// Returns (nil, nil, nil) when no baseline exists yet.
func (r *MCPManifestRepository) GetCurrentBaseline(serverID uuid.UUID) (*domain.MCPManifestBaseline, []domain.ManifestEntry, error) {
	return getCurrentBaseline(r.db, serverID)
}

// getCurrentBaseline reads the current baseline using any querier (DB or Tx). When run inside the
// AtomicRecomputeWellKnown transaction it reads the row already protected by the per-server advisory
// lock, so the read + subsequent drift/baseline writes are a single serialized unit.
func getCurrentBaseline(q txQuerier, serverID uuid.UUID) (*domain.MCPManifestBaseline, []domain.ManifestEntry, error) {
	const query = `
		SELECT id, mcp_server_id, manifest_hash, entries, tool_count, resource_count,
		       prompt_count, captured_from, captured_at, is_current
		FROM mcp_manifest_baselines
		WHERE mcp_server_id = $1 AND is_current = TRUE`

	var b domain.MCPManifestBaseline
	var entriesJSON []byte
	err := q.QueryRow(query, serverID).Scan(
		&b.ID, &b.MCPServerID, &b.ManifestHash, &entriesJSON, &b.ToolCount,
		&b.ResourceCount, &b.PromptCount, &b.CapturedFrom, &b.CapturedAt, &b.IsCurrent,
	)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load current manifest baseline: %w", err)
	}

	entries := make([]domain.ManifestEntry, 0)
	if len(entriesJSON) > 0 {
		if err := json.Unmarshal(entriesJSON, &entries); err != nil {
			return nil, nil, fmt.Errorf("failed to decode baseline entries: %w", err)
		}
	}
	return &b, entries, nil
}

// ReplaceBaseline atomically demotes the current baseline (if any) and inserts the new one as
// current. The append-only history is preserved; only the is_current flag moves.
func (r *MCPManifestRepository) ReplaceBaseline(b *domain.MCPManifestBaseline, entries []domain.ManifestEntry) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin baseline tx: %w", err)
	}
	defer tx.Rollback()

	if err := replaceBaseline(tx, b, entries); err != nil {
		return err
	}
	return tx.Commit()
}

// replaceBaseline demotes the current baseline and inserts the new one using any querier. It does not
// manage a transaction itself; callers either wrap it (ReplaceBaseline) or run it inside an existing
// transaction (AtomicRecomputeWellKnown).
func replaceBaseline(q txQuerier, b *domain.MCPManifestBaseline, entries []domain.ManifestEntry) error {
	entriesJSON, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("failed to encode baseline entries: %w", err)
	}

	if _, err := q.Exec(
		`UPDATE mcp_manifest_baselines SET is_current = FALSE
		 WHERE mcp_server_id = $1 AND is_current = TRUE`, b.MCPServerID,
	); err != nil {
		return fmt.Errorf("failed to demote previous baseline: %w", err)
	}

	if err := q.QueryRow(
		`INSERT INTO mcp_manifest_baselines
		   (mcp_server_id, manifest_hash, entries, tool_count, resource_count,
		    prompt_count, captured_from, is_current)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, TRUE)
		 RETURNING id, captured_at`,
		b.MCPServerID, b.ManifestHash, entriesJSON, b.ToolCount, b.ResourceCount,
		b.PromptCount, string(b.CapturedFrom),
	).Scan(&b.ID, &b.CapturedAt); err != nil {
		return fmt.Errorf("failed to insert baseline: %w", err)
	}
	b.IsCurrent = true
	return nil
}

// InsertDrift records a detected manifest change.
func (r *MCPManifestRepository) InsertDrift(d *domain.MCPManifestDrift) error {
	return insertDrift(r.db, d)
}

// insertDrift writes a drift row using any querier (DB or Tx).
func insertDrift(q txQuerier, d *domain.MCPManifestDrift) error {
	added, _ := json.Marshal(nonNil(d.AddedTools))
	removed, _ := json.Marshal(nonNil(d.RemovedTools))
	changed, _ := json.Marshal(nonNil(d.ChangedTools))

	return q.QueryRow(
		`INSERT INTO mcp_manifest_drift
		   (mcp_server_id, prev_hash, new_hash, added_tools, removed_tools,
		    changed_tools, severity, source)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, detected_at`,
		d.MCPServerID, d.PrevHash, d.NewHash, added, removed, changed,
		string(d.Severity), string(d.Source),
	).Scan(&d.ID, &d.DetectedAt)
}

// AtomicRecomputeWellKnown serializes concurrent well-known recomputes for one server and moves the
// drift row and baseline together in a single transaction.
//
// Concurrency hazard (issue #276): the well-known path previously read the baseline outside any
// transaction and ran InsertDrift and ReplaceBaseline as two separate transactions. Two concurrent
// recomputes for the same server could interleave to insert a duplicate drift row, or insert a drift
// row whose prev_hash/new_hash no longer matched the surviving baseline after the partial-unique
// current-baseline index serialized the competing inserts — and because the hook is best-effort, the
// losing recompute failed silently. A row lock (SELECT ... FOR UPDATE) cannot serialize the
// first-capture case where no baseline row exists yet, so this takes a per-server, transaction-scoped
// advisory lock instead, which serializes regardless of whether a baseline row exists.
//
// buildDrift is invoked with the locked current baseline and its entries and must return the drift to
// record, or nil when the manifest is unchanged (no drift, no baseline move). It is not consulted on
// first capture (prev == nil), which seeds the baseline and records no drift. Returns the recorded
// drift, or nil when none was recorded.
func (r *MCPManifestRepository) AtomicRecomputeWellKnown(
	serverID uuid.UUID,
	newBaseline *domain.MCPManifestBaseline,
	newEntries []domain.ManifestEntry,
	buildDrift func(prev *domain.MCPManifestBaseline, prevEntries []domain.ManifestEntry) *domain.MCPManifestDrift,
) (*domain.MCPManifestDrift, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin recompute tx: %w", err)
	}
	defer tx.Rollback()

	// Per-server, transaction-scoped advisory lock. The first key namespaces the lock so it cannot
	// collide with advisory locks taken elsewhere; the second is derived from the server UUID. Released
	// automatically on commit/rollback. Concurrent recomputes for the SAME server serialize here;
	// recomputes for different servers proceed in parallel.
	if _, err := tx.Exec(
		`SELECT pg_advisory_xact_lock(hashtext('mcp_manifest_wellknown'), hashtext($1))`,
		serverID.String(),
	); err != nil {
		return nil, fmt.Errorf("failed to acquire manifest recompute lock: %w", err)
	}

	prev, prevEntries, err := getCurrentBaseline(tx, serverID)
	if err != nil {
		return nil, err
	}

	if prev == nil {
		// First capture: seed the baseline, record no drift.
		if err := replaceBaseline(tx, newBaseline, newEntries); err != nil {
			return nil, err
		}
		return nil, tx.Commit()
	}

	drift := buildDrift(prev, prevEntries)
	if drift == nil {
		// Manifest unchanged: nothing to record, just release the lock.
		return nil, tx.Commit()
	}

	if err := insertDrift(tx, drift); err != nil {
		return nil, err
	}
	if err := replaceBaseline(tx, newBaseline, newEntries); err != nil {
		return nil, err
	}
	return drift, tx.Commit()
}

// HasUnacknowledgedDriftWithHash reports whether an unacknowledged drift row from the given source
// already records new_hash for a server. Used to deduplicate attestation-observed divergence exactly:
// the baseline never absorbs attestation drift, so without this check the same divergence would
// re-insert on every subsequent attestation. An exact SQL existence test (not a bounded scan of recent
// rows) so a repeat is suppressed no matter how many other drift rows have accumulated since.
func (r *MCPManifestRepository) HasUnacknowledgedDriftWithHash(serverID uuid.UUID, source domain.ManifestSource, newHash string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(
		`SELECT EXISTS (
			SELECT 1 FROM mcp_manifest_drift
			WHERE mcp_server_id = $1 AND source = $2 AND new_hash = $3 AND acknowledged = FALSE
		)`, serverID, string(source), newHash).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check existing drift: %w", err)
	}
	return exists, nil
}

// GetRecentDrift returns the most recent drift records for a server, newest first.
func (r *MCPManifestRepository) GetRecentDrift(serverID uuid.UUID, limit int) ([]*domain.MCPManifestDrift, error) {
	rows, err := r.db.Query(
		`SELECT id, mcp_server_id, prev_hash, new_hash, added_tools, removed_tools,
		        changed_tools, severity, source, detected_at, acknowledged
		 FROM mcp_manifest_drift
		 WHERE mcp_server_id = $1
		 ORDER BY detected_at DESC
		 LIMIT $2`, serverID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query drift: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.MCPManifestDrift, 0)
	for rows.Next() {
		var d domain.MCPManifestDrift
		var added, removed, changed []byte
		if err := rows.Scan(
			&d.ID, &d.MCPServerID, &d.PrevHash, &d.NewHash, &added, &removed,
			&changed, &d.Severity, &d.Source, &d.DetectedAt, &d.Acknowledged,
		); err != nil {
			return nil, fmt.Errorf("failed to scan drift row: %w", err)
		}
		_ = json.Unmarshal(added, &d.AddedTools)
		_ = json.Unmarshal(removed, &d.RemovedTools)
		_ = json.Unmarshal(changed, &d.ChangedTools)
		out = append(out, &d)
	}
	return out, rows.Err()
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
