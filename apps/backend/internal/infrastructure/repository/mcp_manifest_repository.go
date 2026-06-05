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

// GetCurrentBaseline returns the current baseline for a server and its stored manifest entries.
// Returns (nil, nil, nil) when no baseline exists yet.
func (r *MCPManifestRepository) GetCurrentBaseline(serverID uuid.UUID) (*domain.MCPManifestBaseline, []domain.ManifestEntry, error) {
	query := `
		SELECT id, mcp_server_id, manifest_hash, entries, tool_count, resource_count,
		       prompt_count, captured_from, captured_at, is_current
		FROM mcp_manifest_baselines
		WHERE mcp_server_id = $1 AND is_current = TRUE`

	var b domain.MCPManifestBaseline
	var entriesJSON []byte
	err := r.db.QueryRow(query, serverID).Scan(
		&b.ID, &b.MCPServerID, &b.ManifestHash, &entriesJSON, &b.ToolCount,
		&b.ResourceCount, &b.PromptCount, &b.CapturedFrom, &b.CapturedAt, &b.IsCurrent,
	)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load current manifest baseline: %w", err)
	}

	var entries []domain.ManifestEntry
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
	entriesJSON, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("failed to encode baseline entries: %w", err)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin baseline tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`UPDATE mcp_manifest_baselines SET is_current = FALSE
		 WHERE mcp_server_id = $1 AND is_current = TRUE`, b.MCPServerID,
	); err != nil {
		return fmt.Errorf("failed to demote previous baseline: %w", err)
	}

	if err := tx.QueryRow(
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

	return tx.Commit()
}

// InsertDrift records a detected manifest change.
func (r *MCPManifestRepository) InsertDrift(d *domain.MCPManifestDrift) error {
	added, _ := json.Marshal(nonNil(d.AddedTools))
	removed, _ := json.Marshal(nonNil(d.RemovedTools))
	changed, _ := json.Marshal(nonNil(d.ChangedTools))

	return r.db.QueryRow(
		`INSERT INTO mcp_manifest_drift
		   (mcp_server_id, prev_hash, new_hash, added_tools, removed_tools,
		    changed_tools, severity, source)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, detected_at`,
		d.MCPServerID, d.PrevHash, d.NewHash, added, removed, changed,
		string(d.Severity), string(d.Source),
	).Scan(&d.ID, &d.DetectedAt)
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

	var out []*domain.MCPManifestDrift
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
