package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
)

// providerSignInRequiredMessage is the line a password route answers to a
// user whose organization requires sign-in through its identity provider.
const providerSignInRequiredMessage = "Your organization requires sign-in through its identity provider. Sign in from your organization's sign-in link."

// providerSignInRequiredAnswer is the answer a password route gives, with
// 403 in place of a login pair, to a user whose organization requires sign-in
// through its identity provider: the line, the code provider_sign_in_required,
// and the organization, whose sign-in link in the dashboard is
// /auth/login?organization=<organizationId>.
func providerSignInRequiredAnswer(organizationID uuid.UUID) fiber.Map {
	return fiber.Map{
		"error":          providerSignInRequiredMessage,
		"code":           application.SignInRefusedProviderSignInRequired,
		"organizationId": organizationID,
	}
}
