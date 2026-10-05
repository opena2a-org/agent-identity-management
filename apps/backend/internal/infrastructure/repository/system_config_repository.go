package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SystemConfigRepository reads and writes completion markers in the
// system_config key-value table (migration 007), the same table that holds
// the bootstrap_completed marker.
type SystemConfigRepository struct {
	db *sql.DB
}

func NewSystemConfigRepository(db *sql.DB) *SystemConfigRepository {
	return &SystemConfigRepository{db: db}
}

// IsMarkerSet reports whether key is present with the value 'true'. A missing
// key is not an error: it means the task has not completed.
func (r *SystemConfigRepository) IsMarkerSet(ctx context.Context, key string) (bool, error) {
	var value string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM system_config WHERE key = $1`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to read system_config %s: %w", key, err)
	}
	return value == "true", nil
}

// SetMarker records key as 'true'.
func (r *SystemConfigRepository) SetMarker(ctx context.Context, key, description string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO system_config (key, value, description, updated_at)
		VALUES ($1, 'true', $2, NOW())
		ON CONFLICT (key) DO UPDATE
		SET value = 'true', description = EXCLUDED.description, updated_at = NOW()
	`, key, description)
	if err != nil {
		return fmt.Errorf("failed to set system_config %s: %w", key, err)
	}
	return nil
}
