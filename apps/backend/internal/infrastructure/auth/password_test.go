package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPasswordHasher(t *testing.T) {
	hasher := NewPasswordHasher()
	assert.NotNil(t, hasher)
}

func TestPasswordHasher_ValidatePassword_TooShort(t *testing.T) {
	hasher := NewPasswordHasher()

	tests := []string{
		"",
		"a",
		"Ab1!",
		"Abc123!",
	}

	for _, password := range tests {
		err := hasher.ValidatePassword(password)
		assert.ErrorIs(t, err, ErrPasswordTooShort)
	}
}

func TestPasswordHasher_ValidatePassword_TooWeak(t *testing.T) {
	hasher := NewPasswordHasher()

	tests := []struct {
		name     string
		password string
	}{
		{"no uppercase", "password123!"},
		{"no lowercase", "PASSWORD123!"},
		{"no digit", "Password!!!"},
		{"no special", "Password123"},
		{"only letters", "PasswordOnly"},
		{"only digits", "12345678901234"},
		{"only special", "!@#$%^&*()_+"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hasher.ValidatePassword(tt.password)
			assert.ErrorIs(t, err, ErrPasswordTooWeak)
		})
	}
}

func TestPasswordHasher_ValidatePassword_Valid(t *testing.T) {
	hasher := NewPasswordHasher()

	validPasswords := []string{
		"Password1!",
		"MySecure@Pass123",
		"Test#Password99",
		"Str0ng!Pass",
		"Aa1!aaaa",
		"Complex$Password123",
	}

	for _, password := range validPasswords {
		t.Run(password, func(t *testing.T) {
			err := hasher.ValidatePassword(password)
			assert.NoError(t, err)
		})
	}
}

func TestPasswordHasher_HashPassword_Valid(t *testing.T) {
	hasher := NewPasswordHasher()

	password := "SecurePass123!"
	hash, err := hasher.HashPassword(password)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, password, hash)

	// Hash should be valid bcrypt format
	assert.True(t, hasher.IsPasswordHash(hash))
}

func TestPasswordHasher_HashPassword_InvalidPassword(t *testing.T) {
	hasher := NewPasswordHasher()

	// Too short
	_, err := hasher.HashPassword("short")
	assert.Error(t, err)

	// Too weak
	_, err = hasher.HashPassword("password1234")
	assert.Error(t, err)
}

func TestPasswordHasher_VerifyPassword_Correct(t *testing.T) {
	hasher := NewPasswordHasher()

	password := "SecurePass123!"
	hash, err := hasher.HashPassword(password)
	require.NoError(t, err)

	err = hasher.VerifyPassword(password, hash)
	assert.NoError(t, err)
}

func TestPasswordHasher_VerifyPassword_Wrong(t *testing.T) {
	hasher := NewPasswordHasher()

	password := "SecurePass123!"
	hash, err := hasher.HashPassword(password)
	require.NoError(t, err)

	err = hasher.VerifyPassword("WrongPassword123!", hash)
	assert.ErrorIs(t, err, ErrInvalidPassword)
}

func TestPasswordHasher_VerifyPassword_InvalidHash(t *testing.T) {
	hasher := NewPasswordHasher()

	err := hasher.VerifyPassword("password", "not-a-valid-hash")
	assert.Error(t, err)
}

func TestPasswordHasher_ComparePasswords_Match(t *testing.T) {
	hasher := NewPasswordHasher()

	password := "SecurePass123!"
	err := hasher.ComparePasswords(password, password)
	assert.NoError(t, err)
}

func TestPasswordHasher_ComparePasswords_NoMatch(t *testing.T) {
	hasher := NewPasswordHasher()

	err := hasher.ComparePasswords("password1", "password2")
	assert.ErrorIs(t, err, ErrPasswordMismatch)
}

func TestPasswordHasher_IsPasswordHash_Valid(t *testing.T) {
	hasher := NewPasswordHasher()

	// Generate a real hash
	hash, err := hasher.HashPassword("SecurePass123!")
	require.NoError(t, err)

	assert.True(t, hasher.IsPasswordHash(hash))
}

func TestPasswordHasher_IsPasswordHash_Invalid(t *testing.T) {
	hasher := NewPasswordHasher()

	invalidHashes := []string{
		"",
		"not-a-hash",
		"$1$saltsalt$hash", // MD5 format
		"$5$rounds=5000$saltsalt$hash", // SHA256 format
		"plaintext",
		"$2a$10$short", // Too short
	}

	for _, hash := range invalidHashes {
		t.Run(hash, func(t *testing.T) {
			assert.False(t, hasher.IsPasswordHash(hash))
		})
	}
}

func TestPasswordHasher_DifferentHashesForSamePassword(t *testing.T) {
	hasher := NewPasswordHasher()

	password := "SecurePass123!"

	hash1, err := hasher.HashPassword(password)
	require.NoError(t, err)

	hash2, err := hasher.HashPassword(password)
	require.NoError(t, err)

	// Due to random salt, hashes should be different
	assert.NotEqual(t, hash1, hash2)

	// But both should verify correctly
	assert.NoError(t, hasher.VerifyPassword(password, hash1))
	assert.NoError(t, hasher.VerifyPassword(password, hash2))
}

func TestPasswordHasher_SpecialCharacters(t *testing.T) {
	hasher := NewPasswordHasher()

	specialPasswords := []string{
		"Pass@word1",
		"Pass#word1",
		"Pass$word1",
		"Pass%word1",
		"Pass^word1",
		"Pass&word1",
		"Pass*word1",
		"Pass(word1)",
		"Pass_word1",
		"Pass+word1",
		"Pass=word1",
		"Pass[word1]",
		"Pass{word1}",
		"Pass|word1",
		"Pass\\word1",
		"Pass:word1",
		"Pass;word1",
		"Pass'word1",
		"Pass\"word1",
		"Pass<word1>",
		"Pass,word1",
		"Pass.word1",
		"Pass?word1",
		"Pass/word1",
	}

	for _, password := range specialPasswords {
		t.Run(password, func(t *testing.T) {
			err := hasher.ValidatePassword(password)
			assert.NoError(t, err)

			hash, err := hasher.HashPassword(password)
			require.NoError(t, err)

			err = hasher.VerifyPassword(password, hash)
			assert.NoError(t, err)
		})
	}
}

func TestPasswordHasher_UnicodePasswords(t *testing.T) {
	hasher := NewPasswordHasher()

	// Password with unicode characters
	password := "Pässwörd1!"
	err := hasher.ValidatePassword(password)
	assert.NoError(t, err)

	hash, err := hasher.HashPassword(password)
	require.NoError(t, err)

	err = hasher.VerifyPassword(password, hash)
	assert.NoError(t, err)
}

func TestPasswordHasher_LongPassword(t *testing.T) {
	hasher := NewPasswordHasher()

	// Bcrypt has a max length of 72 bytes, but should still work
	longPassword := "Aa1!" + string(make([]byte, 60))
	err := hasher.ValidatePassword(longPassword)
	assert.NoError(t, err)

	hash, err := hasher.HashPassword(longPassword)
	require.NoError(t, err)

	err = hasher.VerifyPassword(longPassword, hash)
	assert.NoError(t, err)
}
