package handlers

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// LastActiveAdminCode is the code of the 409 that refuses the deletion request
// of an organization's only active admin while the organization has other
// users who are not deactivated.
const LastActiveAdminCode = "lastActiveAdmin"

// AccountDeletionHandler serves POST /api/v1/auth/delete-account: a signed-in
// user asks for the deletion of their own account.
type AccountDeletionHandler struct {
	users       domain.UserRepository
	authService *application.AuthService
	deletions   *application.AccountDeletionService
}

// NewAccountDeletionHandler builds the handler. The user repository reads the
// signed-in user's record, the auth service verifies the password with the
// sign-in's own checks (lockout, deactivation, an account without a password,
// failure records), and the deletion service records the request. All three
// are mandatory.
func NewAccountDeletionHandler(users domain.UserRepository, authService *application.AuthService, deletions *application.AccountDeletionService) *AccountDeletionHandler {
	if users == nil || authService == nil || deletions == nil {
		panic("NewAccountDeletionHandler: user repository, auth service and account deletion service are required")
	}
	return &AccountDeletionHandler{users: users, authService: authService, deletions: deletions}
}

// AccountDeletionRequest is the body of POST /api/v1/auth/delete-account.
type AccountDeletionRequest struct {
	Password string `json:"password"`
}

// AccountDeletionResponse answers an accepted deletion request: the account's
// status, when the request was made, and the time from which the account
// deletion purge deletes the account.
type AccountDeletionResponse struct {
	Status              domain.UserStatus `json:"status"`
	DeletionRequestedAt time.Time         `json:"deletionRequestedAt"`
	DeletionDueAt       time.Time         `json:"deletionDueAt"`
}

// RequestDeletion godoc
// @Summary Ask for the deletion of the signed-in user's account
// @Description Verifies the signed-in user's password as at sign-in, deactivates the account at once and records the request. The account deletion purge deletes the account from deletionDueAt, 29 days after the request, unless an admin activates the account before then. A deactivated account's sessions are refused. The organization's only active admin is refused with 409 while the organization has other users who are not deactivated.
// @Tags auth
// @Accept json
// @Produce json
// @Param body body AccountDeletionRequest true "The user's password"
// @Success 202 {object} AccountDeletionResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/auth/delete-account [post]
func (h *AccountDeletionHandler) RequestDeletion(c fiber.Ctx) error {
	var req AccountDeletionRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	if req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "password is required",
		})
	}

	userID, _ := c.Locals("user_id").(uuid.UUID)
	orgID, _ := c.Locals("organization_id").(uuid.UUID)
	user, err := h.users.GetByID(userID)
	if userID == uuid.Nil || err != nil || user == nil || user.OrganizationID != orgID {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired token",
		})
	}

	// The password is verified as at sign-in: a lockout, a deactivated
	// account and an account without a password are refused there, and a
	// wrong password is recorded as a failed attempt that counts toward a
	// lockout. Every refusal answers the same line.
	verified, err := h.authService.LoginWithPasswordExtended(c.Context(), user.Email, req.Password, c.IP(), c.Get("User-Agent"))
	if err != nil || verified == nil || verified.ID != user.ID {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid password",
		})
	}

	deletion, err := h.deletions.Request(c.Context(), user.ID, recordedClientIP(c), c.Get("User-Agent"))
	switch {
	case errors.Is(err, domain.ErrLastActiveAdmin):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "You are the only active admin of your organization, and it has other users: " +
				"make another user an admin first (PUT /api/v1/admin/users/:id/role)",
			"code": LastActiveAdminCode,
		})
	case errors.Is(err, domain.ErrAccountDeactivated), errors.Is(err, domain.ErrUserNotFound):
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired token",
		})
	case err != nil:
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}

	return c.Status(fiber.StatusAccepted).JSON(AccountDeletionResponse{
		Status:              domain.UserStatusDeactivated,
		DeletionRequestedAt: deletion.RequestedAt.UTC(),
		DeletionDueAt:       deletion.DueAt().UTC(),
	})
}
