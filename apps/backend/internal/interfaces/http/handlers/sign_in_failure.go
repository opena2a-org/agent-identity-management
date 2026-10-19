package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
)

// The reasons POST /api/v1/public/login gives for a refused sign-in, one per
// check that answers 401, and the one it answers 403. POST
// /api/v1/auth/login/local gives the reason of the service's
// application.SignInRefusal.
const (
	SignInFailedAccountDeactivated      = application.SignInRefusedAccountDeactivated // the user is deactivated or deleted
	SignInFailedRegistrationRejected    = "registration_rejected"                     // the email and password match a rejected registration request
	SignInFailedEmailOrPasswordMismatch = "email_or_password_mismatch"                // no user or registration request has the email and password
	signInFailedUnclassified            = "sign_in_refused"                           // the service refused without naming a check
	// the password is the user's, and the user's organization requires sign-in
	// through its identity provider (answered 403)
	SignInFailedProviderSignInRequired = application.SignInRefusedProviderSignInRequired
)

// SignInFailure is one password sign-in a route refused, as a sign-in
// failure observer receives it. It never carries the submitted email, the
// password or a token.
type SignInFailure struct {
	Time           time.Time
	Route          string // the route's path
	Method         string // "password"
	Reason         string // the check that refused the sign-in
	IPAddress      string
	UserAgent      string
	UserID         uuid.UUID // uuid.Nil unless the submitted email matched a user
	OrganizationID uuid.UUID // uuid.Nil unless the submitted email matched a user
}

// reportSignInFailure gives observe the refused sign-in. A nil observe does
// nothing.
func reportSignInFailure(c fiber.Ctx, observe func(SignInFailure), reason string, userID, orgID uuid.UUID) {
	if observe == nil {
		return
	}
	observe(SignInFailure{
		Time:   time.Now().UTC(),
		Route:  c.Route().Path,
		Method: "password",
		Reason: reason,
		// Copied out of the request buffer, which the server rewrites for the
		// next request: an observer may keep the report past this request.
		IPAddress:      strings.Clone(c.IP()),
		UserAgent:      strings.Clone(c.Get("User-Agent")),
		UserID:         userID,
		OrganizationID: orgID,
	})
}
