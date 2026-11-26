package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// RegistrationChallengeLifetime is how long a registration challenge can be
// signed after it is issued.
const RegistrationChallengeLifetime = 5 * time.Minute

// RegistrationChallengeSize is the length of a challenge in bytes.
const RegistrationChallengeSize = 32

// ErrRegistrationChallengeNotFound is returned by Consume for an id that names
// no challenge.
var ErrRegistrationChallengeNotFound = errors.New("registration challenge not found")

// RegistrationChallenge is a server-issued, single-use, time-bounded value that
// an agent signs with its private key to show, at registration, that it holds
// the key it registers. It is bound to the organization and the user that asked
// for it.
type RegistrationChallenge struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	PrincipalID    uuid.UUID
	Challenge      []byte
	IssuedAt       time.Time
	ExpiresAt      time.Time
	// UsedAt is when a registration first presented the challenge; nil until then.
	UsedAt *time.Time
}

// RegistrationChallengeRepository stores registration challenges.
type RegistrationChallengeRepository interface {
	// Create stores a new challenge.
	Create(ctx context.Context, challenge *RegistrationChallenge) error
	// Consume marks the challenge used at now, once. It returns the stored
	// challenge and whether this call was the one that used it: false means an
	// earlier call had already used it. An id that names no challenge returns
	// ErrRegistrationChallengeNotFound. Two concurrent calls for one id return
	// true for exactly one of them. Consume does not look at the expiry or at who
	// the challenge was issued to: the caller decides, and the challenge is
	// used whatever it decides.
	Consume(ctx context.Context, id uuid.UUID, now time.Time) (challenge *RegistrationChallenge, firstUse bool, err error)
	// DeleteExpired deletes the challenges that expired at or before now and
	// returns how many.
	DeleteExpired(ctx context.Context, now time.Time) (int64, error)
}
