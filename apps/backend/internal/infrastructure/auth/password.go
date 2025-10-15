package auth

import (
	"errors"
	"fmt"
	"regexp"

	"golang.org/x/crypto/bcrypt"
)

const (
	// MinPasswordLength is the minimum required password length
	MinPasswordLength = 8

	// BcryptCost is the cost factor for bcrypt hashing
	BcryptCost = 12
)

var (
	// ErrPasswordTooShort indicates password doesn't meet minimum length
	ErrPasswordTooShort = errors.New("password must be at least 8 characters long")

	// ErrPasswordTooWeak indicates password doesn't meet complexity requirements
	ErrPasswordTooWeak = errors.New("password must contain uppercase, lowercase, number, and special character")

	// ErrPasswordMismatch indicates passwords don't match
	ErrPasswordMismatch = errors.New("passwords do not match")

	// ErrInvalidPassword indicates password verification failed
	ErrInvalidPassword = errors.New("invalid password")
)

// PasswordHasher provides password hashing and verification
type PasswordHasher struct{}

// NewPasswordHasher creates a new password hasher
func NewPasswordHasher() *PasswordHasher {
	return &PasswordHasher{}
}

// HashPassword hashes a password using bcrypt
func (h *PasswordHasher) HashPassword(password string) (string, error) {
	// Validate password strength first
	if err := h.ValidatePassword(password); err != nil {
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(hash), nil
}

// VerifyPassword compares a password with its hash
func (h *PasswordHasher) VerifyPassword(password, hash string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return ErrInvalidPassword
		}
		return fmt.Errorf("password verification failed: %w", err)
	}
	return nil
}

// ValidatePassword checks if password meets security requirements
func (h *PasswordHasher) ValidatePassword(password string) error {
	// Check minimum length
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}

	// Check for uppercase letter
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	// Check for lowercase letter
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	// Check for digit
	hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)
	// Check for special character
	hasSpecial := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(password)

	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return ErrPasswordTooWeak
	}

	return nil
}

// ComparePasswords checks if two passwords match (used for confirmation)
func (h *PasswordHasher) ComparePasswords(password, confirmation string) error {
	if password != confirmation {
		return ErrPasswordMismatch
	}
	return nil
}

// IsPasswordHash checks if a string looks like a bcrypt hash
func (h *PasswordHasher) IsPasswordHash(hash string) bool {
	// Bcrypt hashes start with $2a$, $2b$, or $2y$
	matched, _ := regexp.MatchString(`^\$2[aby]\$\d{2}\$.{53}$`, hash)
	return matched
}
