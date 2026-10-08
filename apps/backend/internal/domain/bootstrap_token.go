package domain

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// A bootstrap token is a short-lived, single-use credential a signed-in
// dashboard user mints so that a terminal can register one agent in the
// user's organization without a browser round trip. The plaintext is
// returned once by the mint call; only its SHA-256 hash and an 8-character
// display prefix are stored.
const (
	// BootstrapTokenPrefix marks a string as a bootstrap token, so a scanner
	// or a log redactor can recognise one without a database lookup.
	BootstrapTokenPrefix = "aim_ob_"

	// BootstrapTokenTTL is how long a minted token stays exchangeable.
	BootstrapTokenTTL = 15 * time.Minute

	// BootstrapTokenScopeAgentsRegister is the only scope a bootstrap token
	// carries: register one agent in the minting user's organization.
	BootstrapTokenScopeAgentsRegister = "agents:register"

	// BootstrapTokenDisplayPrefixLen is the number of secret characters kept
	// in clear for display.
	BootstrapTokenDisplayPrefixLen = 8

	// bootstrapTokenSecretBytes is the entropy behind a token: 32 bytes,
	// 43 characters of unpadded base64url.
	bootstrapTokenSecretBytes = 32
)

var (
	ErrBootstrapTokenNotFound  = errors.New("bootstrap token not found")
	ErrBootstrapTokenExpired   = errors.New("bootstrap token has expired")
	ErrBootstrapTokenUsed      = errors.New("bootstrap token has already been used")
	ErrBootstrapTokenRevoked   = errors.New("bootstrap token has been revoked")
	ErrBootstrapTokenMalformed = errors.New("bootstrap token is malformed")
)

// BootstrapToken is the stored record of a minted token. It never holds the
// plaintext.
type BootstrapToken struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organizationId"`
	CreatedBy      uuid.UUID  `json:"createdBy"`
	TokenHash      string     `json:"-"`
	DisplayPrefix  string     `json:"displayPrefix"`
	Scope          string     `json:"scope"`
	CreatedAt      time.Time  `json:"createdAt"`
	ExpiresAt      time.Time  `json:"expiresAt"`
	UsedAt         *time.Time `json:"usedAt,omitempty"`
	RevokedAt      *time.Time `json:"revokedAt,omitempty"`
	AgentID        *uuid.UUID `json:"agentId,omitempty"`
}

// CheckUsable reports why the token cannot be exchanged at now, or nil.
// Revocation is reported ahead of use, and use ahead of expiry, so the
// caller sees the most specific reason.
func (t *BootstrapToken) CheckUsable(now time.Time) error {
	switch {
	case t.RevokedAt != nil:
		return ErrBootstrapTokenRevoked
	case t.UsedAt != nil:
		return ErrBootstrapTokenUsed
	case !now.Before(t.ExpiresAt):
		return ErrBootstrapTokenExpired
	}
	return nil
}

// GenerateBootstrapToken returns a new plaintext token, the hash to store and
// the display prefix to store.
func GenerateBootstrapToken() (plaintext, hash, displayPrefix string, err error) {
	buf := make([]byte, bootstrapTokenSecretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", "", fmt.Errorf("generate bootstrap token: %w", err)
	}
	secret := base64.RawURLEncoding.EncodeToString(buf)
	plaintext = BootstrapTokenPrefix + secret
	return plaintext, HashBootstrapToken(plaintext), secret[:BootstrapTokenDisplayPrefixLen], nil
}

// HashBootstrapToken is the lookup key for a token: hex SHA-256 of the full
// plaintext. A salted slow hash is unnecessary because the token carries 256
// bits of entropy.
func HashBootstrapToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// ValidateBootstrapTokenFormat rejects a string that cannot be a token before
// any database lookup.
func ValidateBootstrapTokenFormat(plaintext string) error {
	secret, ok := strings.CutPrefix(plaintext, BootstrapTokenPrefix)
	if !ok {
		return ErrBootstrapTokenMalformed
	}
	decoded, err := base64.RawURLEncoding.DecodeString(secret)
	if err != nil || len(decoded) != bootstrapTokenSecretBytes {
		return ErrBootstrapTokenMalformed
	}
	return nil
}

// BootstrapTokenRepository persists bootstrap tokens.
type BootstrapTokenRepository interface {
	// CreateReplacingOpen revokes every unused, unrevoked token of
	// (token.OrganizationID, token.CreatedBy) at now and inserts token, as one
	// unit of work.
	CreateReplacingOpen(ctx context.Context, token *BootstrapToken, now time.Time) error

	// GetByHash returns the token stored under hash, or ErrBootstrapTokenNotFound.
	GetByHash(ctx context.Context, hash string) (*BootstrapToken, error)

	// Claim marks the token used at now if, and only if, it is unused,
	// unrevoked and unexpired at now. It returns false when another caller
	// claimed it first or it was no longer usable, which makes single use
	// hold under concurrent exchanges.
	Claim(ctx context.Context, id uuid.UUID, now time.Time) (bool, error)

	// ReleaseClaim undoes a Claim made at usedAt, for an exchange that failed
	// before an agent was created. It does nothing if the token was revoked
	// meanwhile or carries an agent.
	ReleaseClaim(ctx context.Context, id uuid.UUID, usedAt time.Time) error

	// SetAgent records the agent a claimed token registered.
	SetAgent(ctx context.Context, id, agentID uuid.UUID) error

	// RevokeOpenForUser revokes every unused, unrevoked token minted by userID
	// in orgID and returns how many it revoked. It never touches another
	// user's or another organization's tokens.
	RevokeOpenForUser(ctx context.Context, orgID, userID uuid.UUID, now time.Time) (int64, error)

	// RevokeByHash revokes the token stored under hash if it is still open.
	RevokeByHash(ctx context.Context, hash string, now time.Time) error
}
