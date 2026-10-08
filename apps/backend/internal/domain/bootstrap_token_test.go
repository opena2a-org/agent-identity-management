package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateBootstrapToken_ReturnsPlaintextHashAndDisplayPrefix(t *testing.T) {
	plaintext, hash, displayPrefix, err := GenerateBootstrapToken()
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(plaintext, BootstrapTokenPrefix))
	secret := strings.TrimPrefix(plaintext, BootstrapTokenPrefix)
	assert.Len(t, secret, 43, "32 random bytes in unpadded base64url")

	sum := sha256.Sum256([]byte(plaintext))
	assert.Equal(t, hex.EncodeToString(sum[:]), hash)
	assert.Equal(t, HashBootstrapToken(plaintext), hash)
	assert.NotContains(t, hash, secret)

	assert.Len(t, displayPrefix, BootstrapTokenDisplayPrefixLen)
	assert.Equal(t, secret[:BootstrapTokenDisplayPrefixLen], displayPrefix)

	require.NoError(t, ValidateBootstrapTokenFormat(plaintext))
}

func TestGenerateBootstrapToken_IsUniquePerCall(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		plaintext, _, _, err := GenerateBootstrapToken()
		require.NoError(t, err)
		require.False(t, seen[plaintext], "duplicate token")
		seen[plaintext] = true
	}
}

func TestValidateBootstrapTokenFormat_RefusesAnythingElse(t *testing.T) {
	valid, _, _, err := GenerateBootstrapToken()
	require.NoError(t, err)

	for _, bad := range []string{
		"",
		BootstrapTokenPrefix,
		strings.TrimPrefix(valid, BootstrapTokenPrefix),             // no prefix
		"aim_xx_" + strings.TrimPrefix(valid, BootstrapTokenPrefix), // wrong prefix
		valid[:len(valid)-1], // truncated
		valid + "A",          // too long
		BootstrapTokenPrefix + strings.Repeat("!", 43), // not base64url
		" " + valid,
	} {
		assert.ErrorIs(t, ValidateBootstrapTokenFormat(bad), ErrBootstrapTokenMalformed, "input %q", bad)
	}
}

func TestBootstrapTokenCheckUsable(t *testing.T) {
	created := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	fresh := func() *BootstrapToken {
		return &BootstrapToken{CreatedAt: created, ExpiresAt: created.Add(BootstrapTokenTTL)}
	}
	assert.Equal(t, 15*time.Minute, BootstrapTokenTTL)

	assert.NoError(t, fresh().CheckUsable(created))
	assert.NoError(t, fresh().CheckUsable(created.Add(BootstrapTokenTTL-time.Second)))
	assert.ErrorIs(t, fresh().CheckUsable(created.Add(BootstrapTokenTTL)), ErrBootstrapTokenExpired)

	used := fresh()
	used.UsedAt = &created
	assert.ErrorIs(t, used.CheckUsable(created), ErrBootstrapTokenUsed)

	revoked := fresh()
	revoked.UsedAt = &created
	revoked.RevokedAt = &created
	assert.ErrorIs(t, revoked.CheckUsable(created), ErrBootstrapTokenRevoked, "revocation is reported ahead of use")
}
