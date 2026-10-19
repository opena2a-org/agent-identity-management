package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// oidcSignInConfigKeyPrefix starts the system_config key of an organization's
// OIDC sign-in configuration; the organization's id follows it.
const oidcSignInConfigKeyPrefix = "oidc_sign_in:"

// OIDCSignInConfigRepository keeps each organization's OIDC sign-in
// configuration in one row of the system_config key-value table (migration
// 007), keyed by the organization's id. The row's value is a JSON document
// that holds the client secret sealed, never in the clear. The table has no
// foreign key to organizations: a reader of a row checks the organization
// itself.
type OIDCSignInConfigRepository struct {
	db *sql.DB
}

// NewOIDCSignInConfigRepository returns the repository over db.
func NewOIDCSignInConfigRepository(db *sql.DB) *OIDCSignInConfigRepository {
	return &OIDCSignInConfigRepository{db: db}
}

// oidcSignInConfigValue is the JSON document a configuration row's value holds.
type oidcSignInConfigValue struct {
	Issuer             string    `json:"issuer"`
	ClientID           string    `json:"clientId"`
	SealedClientSecret string    `json:"sealedClientSecret"`
	UpdatedBy          uuid.UUID `json:"updatedBy"`
}

func oidcSignInConfigKey(organizationID uuid.UUID) string {
	return oidcSignInConfigKeyPrefix + organizationID.String()
}

// Get returns the organization's configuration, or
// domain.ErrOIDCSignInNotConfigured when no row has its key.
func (r *OIDCSignInConfigRepository) Get(ctx context.Context, organizationID uuid.UUID) (*domain.OIDCSignInConfig, error) {
	var value string
	var updatedAt time.Time
	err := r.db.QueryRowContext(ctx,
		`SELECT value, updated_at FROM system_config WHERE key = $1`,
		oidcSignInConfigKey(organizationID),
	).Scan(&value, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrOIDCSignInNotConfigured
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read the OIDC sign-in configuration: %w", err)
	}

	var doc oidcSignInConfigValue
	if err := json.Unmarshal([]byte(value), &doc); err != nil {
		return nil, fmt.Errorf("failed to read the OIDC sign-in configuration: %w", err)
	}
	return &domain.OIDCSignInConfig{
		OrganizationID:     organizationID,
		Issuer:             doc.Issuer,
		ClientID:           doc.ClientID,
		SealedClientSecret: doc.SealedClientSecret,
		UpdatedBy:          doc.UpdatedBy,
		UpdatedAt:          updatedAt,
	}, nil
}

// Put writes cfg to its organization's row, inserting the row or replacing
// its value.
func (r *OIDCSignInConfigRepository) Put(ctx context.Context, cfg *domain.OIDCSignInConfig) error {
	if cfg.OrganizationID == uuid.Nil {
		return errors.New("an OIDC sign-in configuration needs an organization")
	}
	value, err := json.Marshal(oidcSignInConfigValue{
		Issuer:             cfg.Issuer,
		ClientID:           cfg.ClientID,
		SealedClientSecret: cfg.SealedClientSecret,
		UpdatedBy:          cfg.UpdatedBy,
	})
	if err != nil {
		return fmt.Errorf("failed to write the OIDC sign-in configuration: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO system_config (key, value, description, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (key) DO UPDATE
		SET value = EXCLUDED.value, description = EXCLUDED.description, updated_at = NOW()
	`, oidcSignInConfigKey(cfg.OrganizationID), string(value),
		"OIDC sign-in configuration of organization "+cfg.OrganizationID.String())
	if err != nil {
		return fmt.Errorf("failed to write the OIDC sign-in configuration: %w", err)
	}
	return nil
}

// Delete removes the organization's row, if it has one.
func (r *OIDCSignInConfigRepository) Delete(ctx context.Context, organizationID uuid.UUID) error {
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM system_config WHERE key = $1`,
		oidcSignInConfigKey(organizationID),
	); err != nil {
		return fmt.Errorf("failed to remove the OIDC sign-in configuration: %w", err)
	}
	return nil
}
