package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

type sdkTokenRepository struct {
	db *sql.DB
}

// NewSDKTokenRepository creates a new SDK token repository
func NewSDKTokenRepository(db *sql.DB) domain.SDKTokenRepository {
	return &sdkTokenRepository{db: db}
}

// insertSDKTokenQuery stores one token row; insertSDKTokenArgs supplies its
// parameters. Create and Rotate share both.
const insertSDKTokenQuery = `
	INSERT INTO sdk_tokens (
		id, user_id, organization_id, token_hash, token_id,
		device_name, device_fingerprint, ip_address, user_agent,
		expires_at, metadata, created_at, usage_count, last_used_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	RETURNING id, created_at
`

func insertSDKTokenArgs(token *domain.SDKToken) ([]interface{}, error) {
	metadataJSON, err := json.Marshal(token.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}
	return []interface{}{
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
		token.UsageCount,
		token.LastUsedAt,
	}, nil
}

func (r *sdkTokenRepository) Create(token *domain.SDKToken) error {
	args, err := insertSDKTokenArgs(token)
	if err != nil {
		return err
	}

	err = r.db.QueryRow(insertSDKTokenQuery, args...).Scan(&token.ID, &token.CreatedAt)
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
	metadataJSON := make([]byte, 0)

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
	metadataJSON := make([]byte, 0)

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
	metadataJSON := make([]byte, 0)

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

	tokens := make([]*domain.SDKToken, 0)
	for rows.Next() {
		token := &domain.SDKToken{}
		metadataJSON := make([]byte, 0)

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

	tokens := make([]*domain.SDKToken, 0)
	for rows.Next() {
		token := &domain.SDKToken{}
		metadataJSON := make([]byte, 0)

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

// Rotate retires the token whose hash is oldTokenHash and stores its
// successor in one transaction. The old row is revoked only if it is still
// active; when it is not, or the insert fails, the transaction is rolled back
// and neither write takes effect, so a rotation never leaves the old token
// live beside the new one, nor the new token without a row.
func (r *sdkTokenRepository) Rotate(oldTokenHash string, reason string, next *domain.SDKToken) error {
	args, err := insertSDKTokenArgs(next)
	if err != nil {
		return err
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin SDK token rotation: %w", err)
	}
	defer tx.Rollback() // no-op after Commit

	result, err := tx.Exec(`
		UPDATE sdk_tokens
		SET revoked_at = $1, revoke_reason = $2
		WHERE token_hash = $3 AND revoked_at IS NULL
	`, time.Now(), reason, oldTokenHash)
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

	if err := tx.QueryRow(insertSDKTokenQuery, args...).Scan(&next.ID, &next.CreatedAt); err != nil {
		return fmt.Errorf("failed to create SDK token: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit SDK token rotation: %w", err)
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

// RevokeFamily reads the user's rows to find the family: the download's row,
// whose token_id is the family's id, and every row that descends from it, each
// naming the row it replaced in metadata parent_token. The family's active
// rows are then revoked in one statement.
func (r *sdkTokenRepository) RevokeFamily(userID uuid.UUID, familyID string, reason string) error {
	rows, err := r.db.Query(`
		SELECT id, token_id, COALESCE(metadata->>'parent_token', ''), revoked_at IS NULL
		FROM sdk_tokens
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to read SDK token family: %w", err)
	}
	defer rows.Close()

	pending := make([]string, 0)
	children := map[string][]string{}
	active := map[string]bool{}
	for rows.Next() {
		var id uuid.UUID
		var tokenID, parent string
		var isActive bool
		if err := rows.Scan(&id, &tokenID, &parent, &isActive); err != nil {
			return fmt.Errorf("failed to scan SDK token family: %w", err)
		}
		if tokenID == familyID {
			pending = append(pending, id.String())
		}
		if parent != "" {
			children[parent] = append(children[parent], id.String())
		}
		active[id.String()] = isActive
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to read SDK token family: %w", err)
	}

	revoke := make([]string, 0)
	seen := map[string]bool{}
	for len(pending) > 0 {
		id := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[id] {
			continue
		}
		seen[id] = true
		if active[id] {
			revoke = append(revoke, id)
		}
		pending = append(pending, children[id]...)
	}
	if len(revoke) == 0 {
		return nil
	}

	query := `
		UPDATE sdk_tokens
		SET revoked_at = $1, revoke_reason = $2
		WHERE user_id = $3 AND revoked_at IS NULL AND id = ANY($4::uuid[])
	`
	if _, err := r.db.Exec(query, time.Now(), reason, userID, pq.Array(revoke)); err != nil {
		return fmt.Errorf("failed to revoke SDK token family: %w", err)
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
