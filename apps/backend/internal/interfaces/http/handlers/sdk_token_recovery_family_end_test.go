package handlers

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// SDK credential recovery refuses a token family that has ended. The family
// is ended when any of its sdk_tokens rows was revoked by a logout, by a reuse
// (token_family_revoked) or for the family's age (family_max_age). The rows
// are the presented token's row, the rows its rotations grew from and the rows
// that grew from it, so the decision needs no revocation store; a family key
// in a configured store ends the family too. A token retired by a rotation
// while its family is live is recovered.

// rotatedChain downloads an SDK on the fixture and refreshes it n times. It
// returns every refresh token of the chain, oldest first: each one but the
// last was retired by the rotation that issued the next.
func rotatedChain(t *testing.T, f *sdkFamilyFixture, n int) []string {
	t.Helper()
	chain := []string{f.downloadSDK(t)}
	for i := 0; i < n; i++ {
		out, status := postRefresh(t, f.refresh, chain[len(chain)-1])
		require.Equal(t, fiber.StatusOK, status)
		require.True(t, out.Rotated)
		chain = append(chain, out.RefreshToken)
	}
	return chain
}

// logOut signs the fixture's user out with the given refresh token.
func (f *sdkFamilyFixture) logOut(t *testing.T, refreshToken string) {
	t.Helper()
	logout := NewAuthHandler(nil, f.svc, nil, nil, application.NewSDKTokenService(f.repo))
	_, _, _, status := postLogout(t, logout, "", `{"refreshToken":"`+refreshToken+`"}`, "")
	require.Equal(t, fiber.StatusOK, status)
}

// assertNotRecovered presents token for recovery and asserts the refresh
// route's 401 answer, with no pair and no new row.
func assertNotRecovered(t *testing.T, f *sdkFamilyFixture, token, why string) {
	t.Helper()
	rows := len(f.repo.rows)
	status, body := f.recoverWith(t, token)
	assert.Equal(t, fiber.StatusUnauthorized, status, why)
	assert.Contains(t, body, familyRefusal, why)
	assert.NotContains(t, body, "refreshToken", why)
	assert.Len(t, f.repo.rows, rows, "%s: no recovered credential row", why)
}

// A1: a logout with the family's live token ends recovery of the family's
// earlier token with no revocation store configured: the earlier token's row
// was retired by a rotation, and the logout is read from its successor's row.
func TestRecoverRevokedToken_LogoutEndsTheFamilyForItsEarlierTokenWithoutAStore(t *testing.T) {
	f := newSDKFamilyFixture(t, nil)
	chain := rotatedChain(t, f, 1)
	f.logOut(t, chain[1])
	require.True(t, f.repo.row(chain[0]).RevokedFor(domain.SDKTokenRevokeReasonRotation))
	require.True(t, f.repo.row(chain[1]).RevokedFor(sdkTokenLogoutReason))

	assertNotRecovered(t, f, chain[0], "the logout ended the family the earlier token belongs to")
}

// A2: the same with a revocation store: the logout writes the family key and
// the family stays ended. The family key alone, with no row revoked by a
// logout, a reuse or the family's age, ends the family as well.
func TestRecoverRevokedToken_LogoutEndsTheFamilyForItsEarlierTokenWithAStore(t *testing.T) {
	f := newSDKFamilyFixture(t, &rotationStore{})
	chain := rotatedChain(t, f, 1)
	family := jtiOfToken(t, f.svc, chain[0])
	f.logOut(t, chain[1])
	require.True(t, f.store.data["revoked:fam:"+family], "the logout wrote the family key")

	assertNotRecovered(t, f, chain[0], "the logout ended the family the earlier token belongs to")

	t.Run("the family key alone", func(t *testing.T) {
		f := newSDKFamilyFixture(t, &rotationStore{})
		chain := rotatedChain(t, f, 1)
		family := jtiOfToken(t, f.svc, chain[0])
		_, err := f.svc.RevokeFamilyFor(context.Background(), family, time.Hour)
		require.NoError(t, err)
		require.True(t, f.repo.row(chain[1]).IsActive(), "no row ends the family")

		assertNotRecovered(t, f, chain[0], "the store's family key ended the family")
	})
}

// A3 (control): a token retired by a rotation is recovered while its family
// is live, with or without a revocation store: its successors are live or
// were retired by rotations themselves. Another family of the same user that
// a logout ended leaves this family live.
func TestRecoverRevokedToken_RecoversARotatedTokenOfALiveFamily(t *testing.T) {
	for name, store := range map[string]*rotationStore{"store": {}, "noStore": nil} {
		t.Run(name, func(t *testing.T) {
			f := newSDKFamilyFixture(t, store)
			chain := rotatedChain(t, f, 2)
			other := rotatedChain(t, f, 1)
			f.logOut(t, other[1])
			rows := len(f.repo.rows)

			status, body := f.recoverWith(t, chain[0])
			assert.Equal(t, fiber.StatusOK, status, body)
			assert.Contains(t, body, "refreshToken")
			assert.Len(t, f.repo.rows, rows+1, "the recovered credential's row")
		})
	}
}

// A4: a reuse ends the family: every member is refused, the row revoked with
// the family and each row retired by a rotation before it, with or without a
// revocation store.
func TestRecoverRevokedToken_RefusesEveryMemberOfAFamilyAReuseEnded(t *testing.T) {
	for name, store := range map[string]*rotationStore{"store": {}, "noStore": nil} {
		t.Run(name, func(t *testing.T) {
			f := newSDKFamilyFixture(t, store)
			chain := rotatedChain(t, f, 2)
			_, status := postRefresh(t, f.refresh, chain[0])
			require.Equal(t, fiber.StatusUnauthorized, status, "a reuse")
			require.True(t, f.repo.row(chain[0]).RevokedFor(domain.SDKTokenRevokeReasonRotation))
			require.True(t, f.repo.row(chain[1]).RevokedFor(domain.SDKTokenRevokeReasonRotation))
			require.True(t, f.repo.row(chain[2]).RevokedFor(domain.SDKTokenRevokeReasonFamilyRevoked))

			for i, token := range chain {
				assertNotRecovered(t, f, token, fmt.Sprintf("member %d of the family a reuse ended", i))
			}
		})
	}
}

// The family's maximum age ends it: once a refresh past
// SDK_TOKEN_FAMILY_MAX_AGE revoked the family's live row for its age, no
// member is recovered, with or without a revocation store (the age writes no
// family key).
func TestRecoverRevokedToken_RefusesEveryMemberOfAFamilyPastItsMaximumAge(t *testing.T) {
	for name, store := range map[string]*rotationStore{"store": {}, "noStore": nil} {
		t.Run(name, func(t *testing.T) {
			f := newSDKFamilyFixture(t, store)
			chain := rotatedChain(t, f, 1)
			f.repo.row(chain[1]).SetFamilyCreatedAt(time.Now().Add(-366 * 24 * time.Hour))
			_, status := postRefresh(t, f.refresh, chain[1])
			require.Equal(t, fiber.StatusUnauthorized, status, "a refresh past the family's maximum age")
			require.True(t, f.repo.row(chain[1]).RevokedFor(domain.SDKTokenRevokeReasonFamilyMaxAge))

			for i, token := range chain {
				assertNotRecovered(t, f, token, fmt.Sprintf("member %d of the family past its maximum age", i))
			}
		})
	}
}

// unreadableFamilyRepo is a single-row sdk_tokens model whose read of the
// owner's rows fails.
type unreadableFamilyRepo struct{ *refreshTestSDKRepo }

func (r unreadableFamilyRepo) GetByUserID(uuid.UUID, bool) ([]*domain.SDKToken, error) {
	return nil, errors.New("connection refused")
}

// A family whose rows cannot be read is not recovered: the route answers 500
// and mints nothing. A presented row that ends the family itself is refused
// with the 401 answer before the family is read.
func TestRecoverRevokedToken_AFamilyThatCannotBeReadMintsNothing(t *testing.T) {
	for name, reason := range map[string]string{"retired by a rotation": domain.SDKTokenRevokeReasonRotation, "logged out": sdkTokenLogoutReason} {
		t.Run(name, func(t *testing.T) {
			userID, orgID := uuid.New(), uuid.New()
			users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
				return activeUser(userID, orgID, domain.RoleAdmin, "unread@example.com"), nil
			}}
			row := revokedSDKToken(userID, orgID)
			row.RevokeReason = &reason
			sdkRepo := unreadableFamilyRepo{&refreshTestSDKRepo{getByTokenHash: func(string) (*domain.SDKToken, error) {
				return row, nil
			}}}
			app, jwtSvc := newRecoveryTestAppAs(t, users, sdkRepo, userID, orgID)
			old, err := jwtSvc.GenerateSDKRefreshToken(userID.String(), orgID.String(), "unread@example.com", "admin")
			require.NoError(t, err)

			status, body := doRequest(t, app, "POST", "/auth/sdk/recover", recoverBody(old))
			if reason == sdkTokenLogoutReason {
				assert.Equal(t, fiber.StatusUnauthorized, status, body)
				assert.Contains(t, body, familyRefusal)
			} else {
				assert.Equal(t, fiber.StatusInternalServerError, status, body)
				assert.Contains(t, body, "Failed to read the token family")
			}
			assert.NotContains(t, body, "refreshToken")
			assert.Empty(t, sdkRepo.created, "no recovered credential row")
		})
	}
}

// The recovery route reads the presented row's family from its owner's rows.
// The single-row sdk_tokens models of the owner, principal and mint fixtures
// answer that read with the rows they hold.
func (r *rotationSDKRepo) GetByUserID(userID uuid.UUID, _ bool) ([]*domain.SDKToken, error) {
	out := make([]*domain.SDKToken, 0)
	if r.token != nil && r.token.UserID == userID {
		out = append(out, r.token)
	}
	for _, row := range r.created {
		if row.UserID == userID {
			out = append(out, row)
		}
	}
	return out, nil
}

func (r *refreshTestSDKRepo) GetByUserID(userID uuid.UUID, _ bool) ([]*domain.SDKToken, error) {
	out := make([]*domain.SDKToken, 0)
	for _, row := range r.created {
		if row.UserID == userID {
			out = append(out, row)
		}
	}
	return out, nil
}
