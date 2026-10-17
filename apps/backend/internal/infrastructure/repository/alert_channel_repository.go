package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// AlertChannelRepository stores alert channels. Every query names the
// organization, so a channel is read, changed or deleted only inside the
// organization it belongs to.
type AlertChannelRepository struct {
	db *sql.DB
}

func NewAlertChannelRepository(db *sql.DB) *AlertChannelRepository {
	return &AlertChannelRepository{db: db}
}

const alertChannelColumns = `id, organization_id, name, kind, config, min_severity, is_active,
	last_delivery_status, last_delivery_code, last_delivery_at, created_by, created_at, updated_at`

// alertChannelNameConstraint is the unique constraint on an organization's
// channel names (migration 131).
const alertChannelNameConstraint = "alert_channels_organization_id_name_key"

// alertChannelWriteError turns the refusal of a name another channel of the
// organization has (23505, unique_violation) into
// domain.ErrAlertChannelNameTaken.
func alertChannelWriteError(err error) error {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" && pqErr.Constraint == alertChannelNameConstraint {
		return domain.ErrAlertChannelNameTaken
	}
	return err
}

// Create inserts channel. A name another channel of the organization has is
// refused with domain.ErrAlertChannelNameTaken.
func (r *AlertChannelRepository) Create(ctx context.Context, channel *domain.AlertChannel) error {
	config, err := json.Marshal(channel.Config)
	if err != nil {
		return fmt.Errorf("encode alert channel config: %w", err)
	}
	now := time.Now().UTC()
	channel.CreatedAt, channel.UpdatedAt = now, now
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO alert_channels (id, organization_id, name, kind, config, min_severity, is_active, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		channel.ID, channel.OrganizationID, channel.Name, string(channel.Kind), config,
		string(channel.MinSeverity), channel.IsActive, channel.CreatedBy, now, now)
	return alertChannelWriteError(err)
}

// GetByID returns the organization's channel id, or
// domain.ErrAlertChannelNotFound.
func (r *AlertChannelRepository) GetByID(ctx context.Context, orgID, id uuid.UUID) (*domain.AlertChannel, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+alertChannelColumns+`
		FROM alert_channels WHERE id = $1 AND organization_id = $2`, id, orgID)
	channel, err := scanAlertChannel(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrAlertChannelNotFound
	}
	return channel, err
}

// List returns the organization's channels, by name.
func (r *AlertChannelRepository) List(ctx context.Context, orgID uuid.UUID) ([]*domain.AlertChannel, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+alertChannelColumns+`
		FROM alert_channels WHERE organization_id = $1 ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	return scanAlertChannels(rows)
}

// ListActiveForFloors returns the organization's active channels whose
// min_severity is one of floors.
func (r *AlertChannelRepository) ListActiveForFloors(ctx context.Context, orgID uuid.UUID, floors []domain.AlertSeverity) ([]*domain.AlertChannel, error) {
	values := make([]string, len(floors))
	for i, floor := range floors {
		values[i] = string(floor)
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+alertChannelColumns+`
		FROM alert_channels
		WHERE organization_id = $1 AND is_active AND min_severity = ANY($2)
		ORDER BY name`, orgID, pq.Array(values))
	if err != nil {
		return nil, err
	}
	return scanAlertChannels(rows)
}

// Update writes channel's name, kind, config, floor and active flag. A channel
// that is not the organization's is domain.ErrAlertChannelNotFound, and a name
// another channel of the organization has is domain.ErrAlertChannelNameTaken.
func (r *AlertChannelRepository) Update(ctx context.Context, channel *domain.AlertChannel) error {
	config, err := json.Marshal(channel.Config)
	if err != nil {
		return fmt.Errorf("encode alert channel config: %w", err)
	}
	channel.UpdatedAt = time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
		UPDATE alert_channels
		SET name = $3, kind = $4, config = $5, min_severity = $6, is_active = $7, updated_at = $8
		WHERE id = $1 AND organization_id = $2`,
		channel.ID, channel.OrganizationID, channel.Name, string(channel.Kind), config,
		string(channel.MinSeverity), channel.IsActive, channel.UpdatedAt)
	if err != nil {
		return alertChannelWriteError(err)
	}
	return alertChannelOneRow(result)
}

// Delete removes the organization's channel id, or returns
// domain.ErrAlertChannelNotFound.
func (r *AlertChannelRepository) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM alert_channels WHERE id = $1 AND organization_id = $2`, id, orgID)
	if err != nil {
		return err
	}
	return alertChannelOneRow(result)
}

// RecordDelivery stores the outcome of the channel's latest delivery: status,
// the HTTP status of its final attempt (nil after a network error) and when it
// ended.
func (r *AlertChannelRepository) RecordDelivery(ctx context.Context, orgID, id uuid.UUID, status string, code *int, at time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE alert_channels
		SET last_delivery_status = $3, last_delivery_code = $4, last_delivery_at = $5
		WHERE id = $1 AND organization_id = $2`,
		id, orgID, status, code, at)
	if err != nil {
		return err
	}
	return alertChannelOneRow(result)
}

func alertChannelOneRow(result sql.Result) error {
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrAlertChannelNotFound
	}
	return nil
}

type alertChannelScanner interface {
	Scan(dest ...interface{}) error
}

func scanAlertChannel(row alertChannelScanner) (*domain.AlertChannel, error) {
	var (
		channel             domain.AlertChannel
		kind, floor, config string
		status              sql.NullString
		code                sql.NullInt64
		at                  sql.NullTime
		createdBy           uuid.NullUUID
	)
	if err := row.Scan(&channel.ID, &channel.OrganizationID, &channel.Name, &kind, &config, &floor, &channel.IsActive,
		&status, &code, &at, &createdBy, &channel.CreatedAt, &channel.UpdatedAt); err != nil {
		return nil, err
	}
	channel.Kind = domain.AlertChannelKind(kind)
	channel.MinSeverity = domain.AlertSeverity(floor)
	if err := json.Unmarshal([]byte(config), &channel.Config); err != nil {
		return nil, fmt.Errorf("decode the config of alert channel %s: %w", channel.ID, err)
	}
	if status.Valid {
		channel.LastDeliveryStatus = &status.String
	}
	if code.Valid {
		c := int(code.Int64)
		channel.LastDeliveryCode = &c
	}
	if at.Valid {
		channel.LastDeliveryAt = &at.Time
	}
	if createdBy.Valid {
		channel.CreatedBy = &createdBy.UUID
	}
	return &channel, nil
}

func scanAlertChannels(rows *sql.Rows) ([]*domain.AlertChannel, error) {
	defer rows.Close()
	channels := []*domain.AlertChannel{}
	for rows.Next() {
		channel, err := scanAlertChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	return channels, rows.Err()
}
