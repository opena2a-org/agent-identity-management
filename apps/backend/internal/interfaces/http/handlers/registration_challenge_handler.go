package handlers

import (
	"github.com/gofiber/fiber/v3"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
)

// RegistrationChallengeHandler serves POST /api/v1/agents/registration-challenges.
type RegistrationChallengeHandler struct {
	challenges *application.RegistrationChallengeService
}

// NewRegistrationChallengeHandler creates the handler.
func NewRegistrationChallengeHandler(challenges *application.RegistrationChallengeService) *RegistrationChallengeHandler {
	return &RegistrationChallengeHandler{challenges: challenges}
}

// Issue answers a challenge id and a 32-byte challenge (base64), valid for five
// minutes, bound to the caller's organization and to the caller. The route is
// open to the callers that may register agents.
func (h *RegistrationChallengeHandler) Issue(c fiber.Ctx) error {
	orgID, userID, err := RequireOrgAndUserID(c)
	if err != nil {
		return err
	}
	issued, err := h.challenges.Issue(c.Context(), orgID, userID)
	if err != nil {
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(fiber.StatusCreated).JSON(issued)
}
