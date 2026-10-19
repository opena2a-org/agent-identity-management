package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// A signed-in user can ask for the deletion of their own account
// (POST /api/v1/auth/delete-account). The request deactivates the account at
// once and records when it was made; the account deletion purge deletes the
// account AccountDeletionDelay later, unless an admin has activated it again
// in between, which withdraws the request.

const (
	// AccountDeletionDelay is how long an account waits, from its deletion
	// request, before the purge deletes it. With a purge every
	// AccountDeletionPurgeInterval, a server that runs throughout deletes the
	// account in the hour after this delay, before 30 days have passed since
	// the request.
	AccountDeletionDelay = 29 * 24 * time.Hour

	// AccountDeletionPurgeInterval is how often the server runs the account
	// deletion purge, after a first run at start.
	AccountDeletionPurgeInterval = time.Hour

	// AccountDeletionPurgeBatch is the most accounts one purge statement
	// deletes. A purge repeats the statement until it deletes fewer.
	AccountDeletionPurgeBatch = 100
)

// ErrLastActiveAdmin refuses the deletion request of an organization's only
// active admin while the organization has another user who is not
// deactivated: that user would be left with nobody to manage the organization.
var ErrLastActiveAdmin = errors.New("the organization's only active admin cannot ask for the deletion of their account while it has other users")

// ErrAccountDeactivated refuses the deletion request of an account that is
// already deactivated.
var ErrAccountDeactivated = errors.New("the account is deactivated")

// AccountDeletion is the deletion request of one account: the user, the
// user's organization, and when the request was made.
type AccountDeletion struct {
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	RequestedAt    time.Time
}

// DueAt is the time from which the purge deletes the account.
func (d AccountDeletion) DueAt() time.Time {
	return d.RequestedAt.Add(AccountDeletionDelay)
}
