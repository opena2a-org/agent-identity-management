package handlers

import (
	"errors"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The sign-up page shows the error string verbatim, so the message itself is the contract.
func TestRegistrationErrorResponse_NoAdministratorsNamesTheVariable(t *testing.T) {
	status, message := registrationErrorResponse(application.ErrNoAdministrators)
	if status != fiber.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", status, fiber.StatusServiceUnavailable)
	}
	if !strings.Contains(message, "AIM_PLATFORM_ADMINS") {
		t.Fatalf("message does not name AIM_PLATFORM_ADMINS: %q", message)
	}
	if message != NoAdministratorsMessage {
		t.Fatalf("message is not the verbatim copy: %q", message)
	}
}

func TestRegistrationErrorResponse_ExistingMappingsUnchanged(t *testing.T) {
	cases := []struct {
		err    error
		status int
	}{
		{application.ErrUserAlreadyExists, fiber.StatusConflict},
		{application.ErrRegistrationRequestExists, fiber.StatusConflict},
		{errors.New("password validation failed: too short"), fiber.StatusBadRequest},
		{errors.New("database exploded"), fiber.StatusInternalServerError},
	}
	for _, c := range cases {
		status, message := registrationErrorResponse(c.err)
		if status != c.status {
			t.Errorf("%v: status = %d, want %d", c.err, status, c.status)
		}
		if c.status == fiber.StatusInternalServerError && strings.Contains(message, "exploded") {
			t.Errorf("internal error text leaked to the client: %q", message)
		}
	}
}

func TestRegistrationSuccessMessage_StatesTheOutcome(t *testing.T) {
	approved := registrationSuccessMessage(domain.RegistrationStatusApproved)
	pending := registrationSuccessMessage(domain.RegistrationStatusPending)
	if !strings.Contains(approved, "approved") || strings.Contains(approved, "wait") {
		t.Fatalf("approved message must say approved and not ask to wait: %q", approved)
	}
	if !strings.Contains(pending, "wait for admin approval") {
		t.Fatalf("pending message must ask to wait for approval: %q", pending)
	}
}

// The refusal carries a machine-readable code; unrelated errors carry none.
func TestRegistrationErrorBody_CarriesTheCodeOnlyForTheOperatorRefusal(t *testing.T) {
	body := registrationErrorBody(application.ErrNoAdministrators, NoAdministratorsMessage)
	if body["success"] != false || body["error"] != NoAdministratorsMessage || body["code"] != NoAdministratorsCode {
		t.Fatalf("unexpected refusal body: %v", body)
	}
	other := registrationErrorBody(errors.New("boom"), "Registration failed")
	if _, ok := other["code"]; ok {
		t.Fatalf("code attached to an unrelated error: %v", other)
	}
}

// One message serves every refusal state (variable unset, set but nobody listed has registered,
// sole administrator deactivated), so it must not claim the variable is unset.
func TestNoAdministratorsMessage_IsTrueWhenTheAllowlistIsSet(t *testing.T) {
	lower := strings.ToLower(NoAdministratorsMessage)
	if strings.Contains(lower, "no administrators configured") || strings.Contains(lower, "not set") {
		t.Fatalf("message asserts an unset variable, which is false when it is set: %q", NoAdministratorsMessage)
	}
	if !strings.Contains(lower, "no active administrator") {
		t.Fatalf("message does not state the measured condition: %q", NoAdministratorsMessage)
	}
}
