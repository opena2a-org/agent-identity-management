package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

type sdkTokenRepository struct {
	db *sql.DB
}

// NewSDKTokenRepository creates a new SDK token repository
func NewSDKTokenRepository(db *sql.DB) domain.SDKTokenRepository {
	return &sdkTokenRepository{db: db}
}

func (r *sdkTokenRepository) Create(token *domain.SDKToken) error {
	metadataJSON, err := json.Marshal(token.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO sdk_tokens (
			id, user_id, organization_id, token_hash, token_id,
			device_name, device_fingerprint, ip_address, user_agent,
			expires_at, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at
	`

	err = r.db.QueryRow(
		query,
		token.ID,
		token.UserID,
		token.OrganizationID,
		token.TokenHash,
		token.TokenID,
		token.DeviceName,
		token.DeviceFingerprint,
		token.IPAddress,
		token.UserAgent,
		token.ExpiresAt,
		metadataJSON,
		token.CreatedAt,
	).Scan(&token.ID, &token.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create SDK token: %w", err)
	}

	return nil
}

func (r *sdkTokenRepository) GetByID(id uuid.UUID) (*domain.SDKToken, error) {
	query := `
		SELECT id, user_id, organization_id, token_hash, token_id,
		       device_name, device_fingerprint, ip_address, user_agent,
		       last_used_at, last_ip_address, usage_count,
		       created_at, expires_at, revoked_at, revoke_reason, metadata
		FROM sdk_tokens
		WHERE id = $1
	`

	token := &domain.SDKToken{}
	var metadataJSON []byte

	err := r.db.QueryRow(query, id).Scan(
		&token.ID,
		&token.UserID,
		&token.OrganizationID,
		&token.TokenHash,
		&token.TokenID,
		&token.DeviceName,
		&token.DeviceFingerprint,
		&token.IPAddress,
		&token.UserAgent,
		&token.LastUsedAt,
		&token.LastIPAddress,
		&token.UsageCount,
		&token.CreatedAt,
		&token.ExpiresAt,
		&token.RevokedAt,
		&token.RevokeReason,
		&metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("SDK token not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get SDK token: %w", err)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &token.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return token, nil
}

func (r *sdkTokenRepository) GetByTokenID(tokenID string) (*domain.SDKToken, error) {
	query := `
		SELECT id, user_id, organization_id, token_hash, token_id,
		       device_name, device_fingerprint, ip_address, user_agent,
		       last_used_at, last_ip_address, usage_count,
		       created_at, expires_at, revoked_at, revoke_reason, metadata
		FROM sdk_tokens
		WHERE token_id = $1
	`

	token := &domain.SDKToken{}
	var metadataJSON []byte

	err := r.db.QueryRow(query, tokenID).Scan(
		&token.ID,
		&token.UserID,
		&token.OrganizationID,
		&token.TokenHash,
		&token.TokenID,
		&token.DeviceName,
		&token.DeviceFingerprint,
		&token.IPAddress,
		&token.UserAgent,
		&token.LastUsedAt,
		&token.LastIPAddress,
		&token.UsageCount,
		&token.CreatedAt,
		&token.ExpiresAt,
		&token.RevokedAt,
		&token.RevokeReason,
		&metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("SDK token not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get SDK token by token ID: %w", err)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &token.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return token, nil
}

func (r *sdkTokenRepository) GetByTokenHash(tokenHash string) (*domain.SDKToken, error) {
	query := `
		SELECT id, user_id, organization_id, token_hash, token_id,
		       device_name, device_fingerprint, ip_address, user_agent,
		       last_used_at, last_ip_address, usage_count,
		       created_at, expires_at, revoked_at, revoke_reason, metadata
		FROM sdk_tokens
		WHERE token_hash = $1
	`

	token := &domain.SDKToken{}
	var metadataJSON []byte

	err := r.db.QueryRow(query, tokenHash).Scan(
		&token.ID,
		&token.UserID,
		&token.OrganizationID,
		&token.TokenHash,
		&token.TokenID,
		&token.DeviceName,
		&token.DeviceFingerprint,
		&token.IPAddress,
		&token.UserAgent,
		&token.LastUsedAt,
		&token.LastIPAddress,
		&token.UsageCount,
		&token.CreatedAt,
		&token.ExpiresAt,
		&token.RevokedAt,
		&token.RevokeReason,
		&metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("SDK token not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get SDK token by hash: %w", err)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &token.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return token, nil
}

func (r *sdkTokenRepository) GetByUserID(userID uuid.UUID, includeRevoked bool) ([]*domain.SDKToken, error) {
	query := `
		SELECT id, user_id, organization_id, token_hash, token_id,
		       device_name, device_fingerprint, ip_address, user_agent,
		       last_used_at, last_ip_address, usage_count,
		       created_at, expires_at, revoked_at, revoke_reason, metadata
		FROM sdk_tokens
		WHERE user_id = $1
	`

	if !includeRevoked {
		query += " AND revoked_at IS NULL"
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get SDK tokens by user ID: %w", err)
	}
	defer rows.Close()

	var tokens []*domain.SDKToken
	for rows.Next() {
		token := &domain.SDKToken{}
		var metadataJSON []byte

		err := rows.Scan(
			&token.ID,
			&token.UserID,
			&token.OrganizationID,
			&token.TokenHash,
			&token.TokenID,
			&token.DeviceName,
			&token.DeviceFingerprint,
			&token.IPAddress,
			&token.UserAgent,
			&token.LastUsedAt,
			&token.LastIPAddress,
			&token.UsageCount,
			&token.CreatedAt,
			&token.ExpiresAt,
			&token.RevokedAt,
			&token.RevokeReason,
			&metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan SDK token: %w", err)
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &token.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		tokens = append(tokens, token)
	}

	return tokens, nil
}

func (r *sdkTokenRepository) GetByOrganizationID(organizationID uuid.UUID, includeRevoked bool) ([]*domain.SDKToken, error) {
	query := `
		SELECT id, user_id, organization_id, token_hash, token_id,
		       device_name, device_fingerprint, ip_address, user_agent,
		       last_used_at, last_ip_address, usage_count,
		       created_at, expires_at, revoked_at, revoke_reason, metadata
		FROM sdk_tokens
		WHERE organization_id = $1
	`

	if !includeRevoked {
		query += " AND revoked_at IS NULL"
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.db.Query(query, organizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get SDK tokens by organization ID: %w", err)
	}
	defer rows.Close()

	var tokens []*domain.SDKToken
	for rows.Next() {
		token := &domain.SDKToken{}
		var metadataJSON []byte

		err := rows.Scan(
			&token.ID,
			&token.UserID,
			&token.OrganizationID,
			&token.TokenHash,
			&token.TokenID,
			&token.DeviceName,
			&token.DeviceFingerprint,
			&token.IPAddress,
			&token.UserAgent,
			&token.LastUsedAt,
			&token.LastIPAddress,
			&token.UsageCount,
			&token.CreatedAt,
			&token.ExpiresAt,
			&token.RevokedAt,
			&token.RevokeReason,
			&metadataJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan SDK token: %w", err)
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &token.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		tokens = append(tokens, token)
	}

	return tokens, nil
}

func (r *sdkTokenRepository) Update(token *domain.SDKToken) error {
	metadataJSON, err := json.Marshal(token.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE sdk_tokens
		SET device_name = $1, device_fingerprint = $2,
		    last_used_at = $3, last_ip_address = $4, usage_count = $5,
		    revoked_at = $6, revoke_reason = $7, metadata = $8
		WHERE id = $9
	`

	result, err := r.db.Exec(
		query,
		token.DeviceName,
		token.DeviceFingerprint,
		token.LastUsedAt,
		token.LastIPAddress,
		token.UsageCount,
		token.RevokedAt,
		token.RevokeReason,
		metadataJSON,
		token.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update SDK token: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("SDK token not found")
	}

	return nil
}

func (r *sdkTokenRepository) Revoke(id uuid.UUID, reason string) error {
	query := `
		UPDATE sdk_tokens
		SET revoked_at = $1, revoke_reason = $2
		WHERE id = $3 AND revoked_at IS NULL
	`

	result, err := r.db.Exec(query, time.Now(), reason, id)
	if err != nil {
		return fmt.Errorf("failed to revoke SDK token: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("SDK token not found or already revoked")
	}

	return nil
}

func (r *sdkTokenRepository) RevokeByTokenHash(tokenHash string, reason string) error {
	query := `
		UPDATE sdk_tokens
		SET revoked_at = $1, revoke_reason = $2
		WHERE token_hash = $3 AND revoked_at IS NULL
	`

	result, err := r.db.Exec(query, time.Now(), reason, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to revoke SDK token by hash: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("SDK token not found or already revoked")
	}

	return nil
}

func (r *sdkTokenRepository) RevokeAllForUser(userID uuid.UUID, reason string) error {
	query := `
		UPDATE sdk_tokens
		SET revoked_at = $1, revoke_reason = $2
		WHERE user_id = $3 AND revoked_at IS NULL
	`

	_, err := r.db.Exec(query, time.Now(), reason, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke all SDK tokens for user: %w", err)
	}

	return nil
}

func (r *sdkTokenRepository) RecordUsage(tokenID string, ipAddress string) error {
	query := `
		UPDATE sdk_tokens
		SET last_used_at = $1, last_ip_address = $2, usage_count = usage_count + 1
		WHERE token_id = $3
	`

	_, err := r.db.Exec(query, time.Now(), ipAddress, tokenID)
	if err != nil {
		return fmt.Errorf("failed to record SDK token usage: %w", err)
	}

	return nil
}

func (r *sdkTokenRepository) DeleteExpired() error {
	query := `
		DELETE FROM sdk_tokens
		WHERE expires_at < NOW()
	`

	result, err := r.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to delete expired SDK tokens: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows > 0 {
		fmt.Printf("Deleted %d expired SDK tokens\n", rows)
	}

	return nil
}

func (r *sdkTokenRepository) GetActiveCount(userID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM sdk_tokens
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > NOW()
	`

	var count int
	err := r.db.QueryRow(query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get active SDK token count: %w", err)
	}

	return count, nil
}
