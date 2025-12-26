package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

type PasswordResetHandler struct {
	db           *sql.DB
	emailService domain.EmailService
}

func NewPasswordResetHandler(db *sql.DB, emailService domain.EmailService) *PasswordResetHandler {
	return &PasswordResetHandler{
		db:           db,
		emailService: emailService,
	}
}

// RequestPasswordReset initiates a password reset flow
func (h *PasswordResetHandler) RequestPasswordReset(c fiber.Ctx) error {
	var req struct {
		Email string `json:"email" validate:"required,email"`
	}

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Check if user exists
	var userID uuid.UUID
	var userName string
	err := h.db.QueryRow(`
		SELECT id, name
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`, req.Email).Scan(&userID, &userName)

	if err == sql.ErrNoRows {
		// For security, don't reveal if email exists
		// Return success anyway to prevent email enumeration
		return c.JSON(fiber.Map{
			"success": true,
			"message": "If an account with that email exists, we've sent a password reset link",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to process password reset request",
		})
	}

	// Generate secure random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate reset token",
		})
	}
	resetToken := base64.URLEncoding.EncodeToString(tokenBytes)

	// Hash the token before storing (security best practice)
	tokenHash, err := bcrypt.GenerateFromPassword([]byte(resetToken), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to secure reset token",
		})
	}

	// Store hashed token with expiration (1 hour)
	expiresAt := time.Now().Add(1 * time.Hour)
	_, err = h.db.Exec(`
		UPDATE users
		SET password_reset_token = $1,
		    password_reset_expires = $2,
		    updated_at = NOW()
		WHERE id = $3
	`, string(tokenHash), expiresAt, userID)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save reset token",
		})
	}

	// Send password reset email
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	resetLink := fmt.Sprintf("%s/auth/reset-password?token=%s&email=%s", frontendURL, resetToken, req.Email)

	// Send email using email service
	if h.emailService != nil {
		emailData := map[string]interface{}{
			"UserName":    userName,
			"UserEmail":   req.Email,
			"ResetToken":  resetToken,
			"ResetURL":    resetLink,
			"DashboardURL": frontendURL,
		}

		if err := h.emailService.SendTemplatedEmail(
			domain.TemplatePasswordReset,
			req.Email,
			emailData,
		); err != nil {
			// SECURITY: Email service failed - don't log sensitive data (email, reset link)
			// In development, check email service configuration
			_ = err // Silently fail for security - user already gets generic response
		}
	} else {
		// SECURITY: Production systems should ALWAYS have email service configured
		// Never log password reset tokens or user emails to console
		// In production, this case should not occur - email service is required
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "If an account with that email exists, we've sent a password reset link",
	})
}

// VerifyResetToken verifies a password reset token is valid
func (h *PasswordResetHandler) VerifyResetToken(c fiber.Ctx) error {
	email := c.Query("email")
	token := c.Query("token")

	if email == "" || token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email and token are required",
		})
	}

	// Get user and their reset token
	var userID uuid.UUID
	var tokenHash string
	var expiresAt time.Time

	err := h.db.QueryRow(`
		SELECT id, password_reset_token, password_reset_expires
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`, email).Scan(&userID, &tokenHash, &expiresAt)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Invalid or expired reset token",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to verify reset token",
		})
	}

	// Check if token has expired
	if time.Now().After(expiresAt) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Reset token has expired. Please request a new one",
		})
	}

	// Verify token matches
	err = bcrypt.CompareHashAndPassword([]byte(tokenHash), []byte(token))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid reset token",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"valid":   true,
		"message": "Token is valid. You can now reset your password",
	})
}

// ResetPassword completes the password reset with a new password
func (h *PasswordResetHandler) ResetPassword(c fiber.Ctx) error {
	var req struct {
		Email       string `json:"email" validate:"required,email"`
		Token       string `json:"token" validate:"required"`
		NewPassword string `json:"newPassword" validate:"required,min=8"`
	}

	if err := c.Bind().Body(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate password strength
	if len(req.NewPassword) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Password must be at least 8 characters long",
		})
	}

	// Get user and their reset token
	var userID uuid.UUID
	var tokenHash string
	var expiresAt time.Time

	err := h.db.QueryRow(`
		SELECT id, password_reset_token, password_reset_expires
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
	`, req.Email).Scan(&userID, &tokenHash, &expiresAt)

	if err == sql.ErrNoRows {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Invalid or expired reset token",
		})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to verify reset token",
		})
	}

	// Check if token has expired
	if time.Now().After(expiresAt) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Reset token has expired. Please request a new one",
		})
	}

	// Verify token matches
	err = bcrypt.CompareHashAndPassword([]byte(tokenHash), []byte(req.Token))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid reset token",
		})
	}

	// Hash new password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to secure new password",
		})
	}

	// Update password and clear reset token
	_, err = h.db.Exec(`
		UPDATE users
		SET password_hash = $1,
		    password_reset_token = NULL,
		    password_reset_expires = NULL,
		    must_change_password = FALSE,
		    updated_at = NOW()
		WHERE id = $2
	`, string(passwordHash), userID)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update password",
		})
	}

	// SECURITY: No logging of email addresses or password events to prevent information leakage

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Password reset successful. You can now login with your new password",
	})
}
