package handlers

// A refusal the caller can act on is answered with a line written for the
// caller; any other error is a server failure. Each error below carries
// plantedText the way a wrapped repository or library error carries a table
// name or a caller's input, and no answer may contain it.

import (
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/stretchr/testify/assert"
)

const plantedText = `pq: relation "planted_7f3c" does not exist`

func probeResponse(t *testing.T, respond func(c fiber.Ctx) error) (int, map[string]any) {
	t.Helper()
	app := fiber.New()
	app.Get("/probe", respond)
	return decodeErrorBody(t, app, "GET", "/probe")
}

func TestRegistrationRefusalsAnswerTheStatedLine(t *testing.T) {
	cases := []struct {
		err    error
		status int
		line   string
	}{
		{fmt.Errorf("%w: %s", application.ErrAgentNameExists, plantedText), fiber.StatusConflict, "an agent with this name already exists"},
		{fmt.Errorf("%w: %s", application.ErrInvalidOrgOrUser, plantedText), fiber.StatusBadRequest, "invalid organization or user for this registration"},
		{fmt.Errorf("failed to create agent: %s", plantedText), fiber.StatusInternalServerError, ServerErrorMessage},
	}
	for _, tc := range cases {
		status, body := probeResponse(t, func(c fiber.Ctx) error { return respondRegistrationError(c, tc.err) })
		assert.Equal(t, tc.status, status, tc.line)
		assert.Equal(t, map[string]any{"error": tc.line}, body)
	}
}

func TestCapabilityValidationAnswersTheRuleItBreaks(t *testing.T) {
	status, body := probeResponse(t, func(c fiber.Ctx) error {
		return respondCapabilityValidationError(c, &domain.CapabilityError{Capability: "files", Message: "invalid capability format"})
	})
	assert.Equal(t, fiber.StatusBadRequest, status)
	assert.Equal(t, map[string]any{"error": "invalid capability format"}, body)

	status, body = probeResponse(t, func(c fiber.Ctx) error {
		return respondCapabilityValidationError(c, fmt.Errorf("failed to register custom capability: %s", plantedText))
	})
	assert.Equal(t, fiber.StatusInternalServerError, status)
	assert.Equal(t, map[string]any{"error": ServerErrorMessage}, body)
}

func TestPasswordRefusalsNameTheMemberAndTheRule(t *testing.T) {
	cases := []struct {
		name string
		rule func() (string, bool)
		want string
	}{
		{"current password", func() (string, bool) {
			return changePasswordRule(fmt.Errorf("%w (%s)", application.ErrCurrentPasswordIncorrect, plantedText), "currentPassword")
		}, "currentPassword is incorrect"},
		{"short new password", func() (string, bool) {
			return changePasswordRule(auth.ErrPasswordTooShort, "currentPassword")
		}, "newPassword must be at least 8 characters long"},
		{"weak new password", func() (string, bool) {
			return changePasswordRule(auth.ErrPasswordTooWeak, "oldPassword")
		}, "newPassword must contain an uppercase letter, a lowercase letter, a number and a special character"},
		{"mismatched confirmation", func() (string, bool) {
			return resetPasswordRule(auth.ErrPasswordMismatch)
		}, "newPassword and confirmPassword must match"},
		{"reset token", func() (string, bool) {
			return resetPasswordRule(application.ErrResetTokenInvalid)
		}, "Invalid or expired reset token"},
	}
	for _, tc := range cases {
		rule, ok := tc.rule()
		assert.True(t, ok, tc.name)
		assert.Equal(t, tc.want, rule, tc.name)
	}

	for _, refused := range []func() (string, bool){
		func() (string, bool) {
			return changePasswordRule(fmt.Errorf("failed to update password: %s", plantedText), "currentPassword")
		},
		func() (string, bool) {
			return resetPasswordRule(fmt.Errorf("failed to update password: %s", plantedText))
		},
	} {
		rule, ok := refused()
		assert.False(t, ok, "a repository failure is not the caller's to fix")
		assert.Empty(t, rule)
	}

	status, message := registrationErrorResponse(fmt.Errorf("password validation failed: %w", auth.ErrPasswordTooShort))
	assert.Equal(t, fiber.StatusBadRequest, status)
	assert.Equal(t, "password must be at least 8 characters long", message)
}

func TestResolveRefusalsNameTheRuleOrRefuse(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{fmt.Errorf("%w of agent %s", application.ErrResolveKeyNotRegistered, plantedText), "agentPublicKey is not the registered key of this agent; omit it or send the registered key"},
		{fmt.Errorf("%w: nonce time is not RFC 3339", application.ErrResolveNonceMalformed), "nonce must be <RFC 3339 time>:<random part>"},
		{fmt.Errorf("%w: nonce expired: %s", application.ErrResolveNonceOutsideWindow, plantedText), "nonce time is outside the accepted window; sign the request again with a fresh nonce"},
		{fmt.Errorf("%w by agent %s", application.ErrResolveNonceAlreadyUsed, plantedText), "nonce has already been used; sign the request again with a fresh nonce"},
		{fmt.Errorf("failed to lookup namespace prod: %s", plantedText), "Secret resolution refused"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, resolveRefusalLine(tc.err))
	}
}

func TestIsolationPostureRuleNamesTheMember(t *testing.T) {
	assert.Equal(t, "", isolationPostureRule("none", "none", "none", "none"))
	assert.Equal(t, "sandbox is not a recognized sandbox type", isolationPostureRule(plantedText, "none", "none", "none"))
	assert.Equal(t, "process is not a recognized process isolation", isolationPostureRule("none", "none", "none", plantedText))
}
