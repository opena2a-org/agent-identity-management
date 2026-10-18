package auth

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// SessionStateReader reads the part of a user record that decides, on every
// authenticated request, whether the request's session may act: when the
// password last changed and whether a change is required. It answers
// domain.ErrUserNotFound when no user has the ID.
type SessionStateReader interface {
	GetSessionState(ctx context.Context, userID uuid.UUID) (*domain.UserSessionState, error)
}

// SetSessionStateReader attaches the reader AuthMiddleware reads the user
// record through on every request. The server attaches the user repository;
// a JWTService without a reader (a test that builds the middleware alone)
// reads no record.
func (s *JWTService) SetSessionStateReader(r SessionStateReader) {
	s.sessionState = r
}

// SessionState reads the user record behind a session through the attached
// reader. attached is false, with no state and no error, when no reader is
// attached.
func (s *JWTService) SessionState(ctx context.Context, userID uuid.UUID) (state *domain.UserSessionState, attached bool, err error) {
	if s == nil || s.sessionState == nil {
		return nil, false, nil
	}
	state, err = s.sessionState.GetSessionState(ctx, userID)
	return state, true, err
}

// PasswordChangeBoundary returns the earliest sign-in time a token may carry
// and still be accepted after a password change recorded at changedAt: the
// first whole second after it. A token's times are whole seconds, so a token
// minted in the second of the change cannot be told apart from one minted
// just before it, and every token of that second is refused. The pair a
// change issues to the session that made it carries this time as its
// auth_time (GenerateTokenPairSince).
func PasswordChangeBoundary(changedAt time.Time) time.Time {
	return changedAt.Truncate(time.Second).Add(time.Second)
}

// SignedInBeforePasswordChange reports whether a token belongs to a session
// from before the password change recorded at changedAt. A nil changedAt (the
// password has not changed since the instant was first recorded) refuses
// nothing. The token's sign-in time is its auth_time, or its own iat when it
// carries none (an SDK-download token, an access token minted from one, a
// standalone access token), and the token is refused when that time is
// before PasswordChangeBoundary(changedAt). A token that carries neither
// claim is refused.
func SignedInBeforePasswordChange(c *JWTClaims, changedAt *time.Time) bool {
	if changedAt == nil {
		return false
	}
	return c.SignedInAt().Before(PasswordChangeBoundary(*changedAt))
}

// GenerateTokenPairSince mints a login pair, a new family as GenerateTokenPair
// mints, for an account whose password last changed at passwordChangedAt
// (nil: it has not changed). The pair's sign-in time (auth_time) is now, or
// PasswordChangeBoundary(*passwordChangedAt) when that is later, so a pair
// minted in the second of the change (the pair a change issues to the
// session that made it) is not refused by that change.
func (s *JWTService) GenerateTokenPairSince(userID, orgID, email, role string, passwordChangedAt *time.Time) (accessToken, refreshToken string, err error) {
	if passwordChangedAt == nil {
		return s.GenerateTokenPair(userID, orgID, email, role)
	}
	// The zero time signs in now (generateRefreshToken).
	signedIn := time.Time{}
	if boundary := PasswordChangeBoundary(*passwordChangedAt); boundary.After(time.Now()) {
		signedIn = boundary
	}
	refreshToken, err = s.generateRefreshToken(userID, orgID, "", signedIn)
	if err != nil {
		return "", "", err
	}
	claims, err := s.ValidateToken(refreshToken)
	if err != nil {
		return "", "", err
	}
	accessToken, err = s.generateAccessToken(userID, orgID, email, role, claims.FamilyID(), claims.SignedInAt())
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}
