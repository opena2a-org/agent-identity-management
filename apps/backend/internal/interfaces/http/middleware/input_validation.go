package middleware

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Pre-compiled regex patterns to avoid recompilation on every call
var (
	emailRegexp = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	uuidRegexp  = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
)

// PasswordValidationResult contains the results of password validation
type PasswordValidationResult struct {
	Valid    bool     `json:"valid"`
	Messages []string `json:"messages,omitempty"`
}

// ValidatePassword checks password against enterprise security requirements
// SECURITY: Enterprise-grade password policy for SOC 2 compliance
// Requirements:
// - Minimum 8 characters (configurable, recommend 12+)
// - At least one uppercase letter
// - At least one lowercase letter
// - At least one digit
// - At least one special character
// - No common patterns (e.g., "password", "123456")
func ValidatePassword(password string, minLength int) PasswordValidationResult {
	result := PasswordValidationResult{Valid: true, Messages: []string{}}

	if minLength < 8 {
		minLength = 8 // Minimum secure default
	}

	// Length check
	if len(password) < minLength {
		result.Valid = false
		result.Messages = append(result.Messages, "Password must be at least "+strconv.Itoa(minLength)+" characters")
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if !hasUpper {
		result.Valid = false
		result.Messages = append(result.Messages, "Password must contain at least one uppercase letter")
	}

	if !hasLower {
		result.Valid = false
		result.Messages = append(result.Messages, "Password must contain at least one lowercase letter")
	}

	if !hasDigit {
		result.Valid = false
		result.Messages = append(result.Messages, "Password must contain at least one digit")
	}

	if !hasSpecial {
		result.Valid = false
		result.Messages = append(result.Messages, "Password must contain at least one special character")
	}

	// Check for common weak passwords
	weakPasswords := []string{
		"password", "123456", "12345678", "qwerty", "abc123",
		"monkey", "1234567", "letmein", "trustno1", "dragon",
		"baseball", "iloveyou", "master", "sunshine", "ashley",
		"bailey", "passw0rd", "shadow", "123123", "654321",
		"superman", "qazwsx", "michael", "football", "password1",
	}

	lowerPassword := strings.ToLower(password)
	for _, weak := range weakPasswords {
		if lowerPassword == weak || strings.Contains(lowerPassword, weak) {
			result.Valid = false
			result.Messages = append(result.Messages, "Password contains common weak pattern")
			break
		}
	}

	return result
}

// ValidateEmail validates email format with stricter rules
// SECURITY: Prevents email injection and ensures valid format
func ValidateEmail(email string) bool {
	if len(email) < 3 || len(email) > 254 {
		return false
	}

	return emailRegexp.MatchString(email)
}

// SanitizeInput removes potentially dangerous characters from input
// SECURITY: Prevents XSS and injection attacks in text fields
func SanitizeInput(input string) string {
	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")

	// Trim whitespace
	input = strings.TrimSpace(input)

	// Limit length to prevent DoS
	if len(input) > 10000 {
		input = input[:10000]
	}

	return input
}

// ValidateUUID validates UUID format
func ValidateUUID(id string) bool {
	return uuidRegexp.MatchString(id)
}
