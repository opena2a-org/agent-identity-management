package handlers

import (
	"errors"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
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
