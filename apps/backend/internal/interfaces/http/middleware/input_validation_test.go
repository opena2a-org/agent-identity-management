package middleware

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ===========================
// ValidatePassword Tests
// ===========================

func TestValidatePassword_ValidPassword(t *testing.T) {
	result := ValidatePassword("SecureP@ss123!", 8)

	assert.True(t, result.Valid)
	assert.Empty(t, result.Messages)
}

func TestValidatePassword_TooShort(t *testing.T) {
	result := ValidatePassword("Ab1@", 8)

	assert.False(t, result.Valid)
	assert.Contains(t, strings.Join(result.Messages, " "), "at least")
}

func TestValidatePassword_MinLengthEnforced(t *testing.T) {
	// Even if minLength is set to 4, it should enforce minimum 8
	result := ValidatePassword("Ab1@567", 4)

	assert.False(t, result.Valid)
}

func TestValidatePassword_NoUppercase(t *testing.T) {
	result := ValidatePassword("securepass123!", 8)

	assert.False(t, result.Valid)
	assert.Contains(t, strings.Join(result.Messages, " "), "uppercase")
}

func TestValidatePassword_NoLowercase(t *testing.T) {
	result := ValidatePassword("SECUREPASS123!", 8)

	assert.False(t, result.Valid)
	assert.Contains(t, strings.Join(result.Messages, " "), "lowercase")
}

func TestValidatePassword_NoDigit(t *testing.T) {
	result := ValidatePassword("SecurePass!@#", 8)

	assert.False(t, result.Valid)
	assert.Contains(t, strings.Join(result.Messages, " "), "digit")
}

func TestValidatePassword_NoSpecialChar(t *testing.T) {
	result := ValidatePassword("SecurePass123", 8)

	assert.False(t, result.Valid)
	assert.Contains(t, strings.Join(result.Messages, " "), "special character")
}

func TestValidatePassword_CommonWeakPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{"password", "Password1!"},
		{"123456", "A1b@123456"},
		{"qwerty", "Qwerty123!"},
		{"letmein", "Letmein1!"},
		{"football", "Football1!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidatePassword(tt.password, 8)
			assert.False(t, result.Valid, "Password containing '%s' should be rejected", tt.name)
			found := false
			for _, msg := range result.Messages {
				if strings.Contains(msg, "weak pattern") {
					found = true
					break
				}
			}
			assert.True(t, found, "Should contain weak pattern message")
		})
	}
}

func TestValidatePassword_AllRequirementsMet(t *testing.T) {
	passwords := []string{
		"MyP@ssw0rd!",
		"Complex#Pass1",
		"Str0ng$ecret",
		"Valid123!Pass",
		"N0tW3ak@Key",
	}

	for _, pwd := range passwords {
		t.Run(pwd, func(t *testing.T) {
			result := ValidatePassword(pwd, 8)
			assert.True(t, result.Valid, "Password %s should be valid", pwd)
		})
	}
}

func TestValidatePassword_MultipleFailures(t *testing.T) {
	// Password with no upper, no special, too short
	result := ValidatePassword("abc1", 8)

	assert.False(t, result.Valid)
	// Should have multiple error messages
	assert.GreaterOrEqual(t, len(result.Messages), 2)
}

// ===========================
// ValidateEmail Tests
// ===========================

func TestValidateEmail_ValidEmails(t *testing.T) {
	validEmails := []string{
		"test@example.com",
		"user.name@domain.org",
		"user+tag@example.co.uk",
		"admin@subdomain.example.com",
		"first.last@company.io",
		"user123@test.net",
		"a@b.co",
	}

	for _, email := range validEmails {
		t.Run(email, func(t *testing.T) {
			assert.True(t, ValidateEmail(email), "Email %s should be valid", email)
		})
	}
}

func TestValidateEmail_InvalidEmails(t *testing.T) {
	invalidEmails := []string{
		"",
		"@",
		"test",
		"test@",
		"@example.com",
		"test@.com",
		"test@com",
		"test space@example.com",
		"test@example",
		"ab",                      // too short
		strings.Repeat("a", 250) + "@b.co", // too long
	}

	for _, email := range invalidEmails {
		t.Run(email, func(t *testing.T) {
			assert.False(t, ValidateEmail(email), "Email %s should be invalid", email)
		})
	}
}

func TestValidateEmail_LengthBoundaries(t *testing.T) {
	// Exactly 3 chars - minimum valid
	assert.True(t, ValidateEmail("a@b.co"))

	// 2 chars - too short
	assert.False(t, ValidateEmail("a@"))

	// 254 chars - at limit
	localPart := strings.Repeat("a", 200)
	domain := "example.com"
	longEmail := localPart + "@" + domain
	if len(longEmail) <= 254 {
		assert.True(t, ValidateEmail(longEmail))
	}

	// 255+ chars - too long
	veryLongEmail := strings.Repeat("a", 250) + "@example.com"
	assert.False(t, ValidateEmail(veryLongEmail))
}

// ===========================
// SanitizeInput Tests
// ===========================

func TestSanitizeInput_RemovesNullBytes(t *testing.T) {
	input := "test\x00string\x00here"
	result := SanitizeInput(input)

	assert.Equal(t, "teststringhere", result)
	assert.NotContains(t, result, "\x00")
}

func TestSanitizeInput_TrimsWhitespace(t *testing.T) {
	input := "  test string  "
	result := SanitizeInput(input)

	assert.Equal(t, "test string", result)
}

func TestSanitizeInput_TruncatesLongInput(t *testing.T) {
	longInput := strings.Repeat("a", 15000)
	result := SanitizeInput(longInput)

	assert.Len(t, result, 10000)
}

func TestSanitizeInput_PreservesNormalInput(t *testing.T) {
	input := "Normal input with special chars: @#$%^&*()"
	result := SanitizeInput(input)

	assert.Equal(t, input, result)
}

func TestSanitizeInput_EmptyString(t *testing.T) {
	result := SanitizeInput("")
	assert.Empty(t, result)
}

func TestSanitizeInput_OnlyWhitespace(t *testing.T) {
	result := SanitizeInput("   \t\n   ")
	assert.Empty(t, result)
}

func TestSanitizeInput_MixedNullAndWhitespace(t *testing.T) {
	input := "  \x00test\x00  "
	result := SanitizeInput(input)

	assert.Equal(t, "test", result)
}

// ===========================
// ValidateUUID Tests
// ===========================

func TestValidateUUID_ValidUUIDs(t *testing.T) {
	validUUIDs := []string{
		"123e4567-e89b-12d3-a456-426614174000",
		"550e8400-e29b-41d4-a716-446655440000",
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		"f47ac10b-58cc-4372-a567-0e02b2c3d479",
		"A0EEBC99-9C0B-4EF8-BB6D-6BB9BD380A11", // uppercase
	}

	for _, uuid := range validUUIDs {
		t.Run(uuid, func(t *testing.T) {
			assert.True(t, ValidateUUID(uuid), "UUID %s should be valid", uuid)
		})
	}
}

func TestValidateUUID_InvalidUUIDs(t *testing.T) {
	invalidUUIDs := []string{
		"",
		"not-a-uuid",
		"123e4567-e89b-12d3-a456",              // too short
		"123e4567-e89b-12d3-a456-4266141740001", // too long
		"123e4567-e89b-62d3-a456-426614174000", // invalid version (6)
		"123e4567-e89b-12d3-c456-426614174000", // invalid variant (c)
		"123e4567e89b12d3a456426614174000",     // no dashes
		"123e4567-e89b-12d3-a456_426614174000", // underscore instead of dash
		"gggg4567-e89b-12d3-a456-426614174000", // invalid hex chars
	}

	for _, uuid := range invalidUUIDs {
		t.Run(uuid, func(t *testing.T) {
			assert.False(t, ValidateUUID(uuid), "UUID %s should be invalid", uuid)
		})
	}
}

func TestValidateUUID_EdgeCases(t *testing.T) {
	// All zeros (nil UUID)
	assert.True(t, ValidateUUID("00000000-0000-1000-8000-000000000000"))

	// All ones
	assert.True(t, ValidateUUID("ffffffff-ffff-1fff-bfff-ffffffffffff"))

	// Mixed case
	assert.True(t, ValidateUUID("123E4567-E89B-12D3-A456-426614174000"))
}
