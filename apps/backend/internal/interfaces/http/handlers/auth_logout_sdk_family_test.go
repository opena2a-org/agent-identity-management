package handlers

import (
	"errors"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// A logout that presents any token of an SDK-download token family ends the
// family, with or without a revocation store: the family's still-active
// sdk_tokens rows are revoked with the logout reason, so a later recovery of
// any member and a later refresh of the live member are refused. Where a store
// is configured, its family key ends the family as well. The answer's
// refreshToken is true only when the presented row is retired and the
// family's rows are revoked, and, where a store is configured, its writes
// succeeded too.

// logOutAnswer signs the fixture's user out with the given refresh token,
// through the given sdk_tokens model, and returns the logout answer.
func (f *sdkFamilyFixture) logOutAnswer(t *testing.T, repo domain.SDKTokenRepository, refreshToken string) logoutAnswer {
	t.Helper()
	logout := NewAuthHandler(nil, f.svc, nil, nil, application.NewSDKTokenService(repo))
	_, _, answer, status := postLogout(t, logout, "", `{"refreshToken":"`+refreshToken+`"}`, "")
	require.Equal(t, fiber.StatusOK, status)
	return answer
}

// assertEndedByLogout asserts that a logout ended the family of chain: the
// live token's row carries the logout reason, no token of the family is
// recovered, and the live token is refused at refresh.
func assertEndedByLogout(t *testing.T, f *sdkFamilyFixture, chain []string) {
	t.Helper()
	live := chain[len(chain)-1]
	assert.True(t, f.repo.row(live).RevokedFor(sdkTokenLogoutReason), "the family's live row is revoked by the logout")
	for _, token := range chain {
		assertNotRecovered(t, f, token, "the logout ended the family")
	}
	_, status := postRefresh(t, f.refresh, live)
	assert.Equal(t, fiber.StatusUnauthorized, status, "the family's live token is refused at refresh")
}

// familyWriteFailingRepo is the SDK family model whose family write fails.
type familyWriteFailingRepo struct{ *sdkFamilyRepo }

func (r familyWriteFailingRepo) RevokeFamily(uuid.UUID, string, string) error {
	return errors.New("database unavailable")
}

// L1: with no revocation store, a logout with a token already retired by a
// rotation ends its family: the presented row keeps its rotation reason, the
// live row is revoked by the logout, the retired token is not recovered and
// the live token is refused at refresh. A token from the middle of a longer
// chain ends the family the same way.
func TestAuthHandler_Logout_SDKTokenOfARotatedFamilyEndsTheFamilyWithoutAStore(t *testing.T) {
	f := newSDKFamilyFixture(t, nil)
	chain := rotatedChain(t, f, 1)
	f.logOutAnswer(t, f.repo, chain[0])
	assert.True(t, f.repo.row(chain[0]).RevokedFor(domain.SDKTokenRevokeReasonRotation), "the presented row keeps its rotation reason")
	assertEndedByLogout(t, f, chain)

	t.Run("a token from the middle of a longer chain", func(t *testing.T) {
		f := newSDKFamilyFixture(t, nil)
		chain := rotatedChain(t, f, 2)
		f.logOutAnswer(t, f.repo, chain[1])
		assertEndedByLogout(t, f, chain)
	})
}

// L2: the same with a revocation store: the logout writes the family key, the
// answer reports the revocation, and the family's rows are ended too.
func TestAuthHandler_Logout_SDKTokenOfARotatedFamilyEndsTheFamilyWithAStore(t *testing.T) {
	f := newSDKFamilyFixture(t, &rotationStore{})
	chain := rotatedChain(t, f, 1)
	family := jtiOfToken(t, f.svc, chain[0])
	answer := f.logOutAnswer(t, f.repo, chain[0])
	assert.True(t, answer.Revoked["refreshToken"])
	assert.True(t, f.store.data["revoked:fam:"+family], "the logout wrote the family key")
	assertEndedByLogout(t, f, chain)
}

// L3: the answer's refreshToken reports the family's end. With no store it is
// true when the presented row is retired and the family's rows are revoked.
// When the family write fails it is false and the live row stays active; when
// a configured store's family write fails it is false although the rows are
// revoked.
func TestAuthHandler_Logout_SDKTokenAnswerReportsTheFamilyEnd(t *testing.T) {
	t.Run("the family's rows are revoked", func(t *testing.T) {
		f := newSDKFamilyFixture(t, nil)
		chain := rotatedChain(t, f, 1)
		answer := f.logOutAnswer(t, f.repo, chain[0])
		assert.True(t, answer.Revoked["refreshToken"], "the rows ended the family")
		assert.False(t, answer.Revoked["accessToken"], "no bearer was presented")
	})

	for name, store := range map[string]*rotationStore{"store": {}, "noStore": nil} {
		t.Run("the family write fails/"+name, func(t *testing.T) {
			f := newSDKFamilyFixture(t, store)
			chain := rotatedChain(t, f, 1)
			answer := f.logOutAnswer(t, familyWriteFailingRepo{f.repo}, chain[0])
			assert.False(t, answer.Revoked["refreshToken"], "no claim of an end the rows do not show")
			assert.True(t, f.repo.row(chain[1]).IsActive(), "the family's live row stays active")
		})
	}

	t.Run("the store's family write fails", func(t *testing.T) {
		f := newSDKFamilyFixture(t, nil)
		chain := rotatedChain(t, f, 1)
		f.svc.SetRevoker(auth.NewTokenRevoker(&familyKeyFailingStore{}, false))
		answer := f.logOutAnswer(t, f.repo, chain[0])
		assert.False(t, answer.Revoked["refreshToken"], "no claim of a revocation the configured store did not complete")
		assert.True(t, f.repo.row(chain[1]).RevokedFor(sdkTokenLogoutReason), "the family's rows are revoked")
	})
}

// L4: the logout ends only the presented token's family: another family of
// the same user stays live and its token still refreshes.
func TestAuthHandler_Logout_SDKTokenEndsOnlyItsOwnFamily(t *testing.T) {
	for name, store := range map[string]*rotationStore{"store": {}, "noStore": nil} {
		t.Run(name, func(t *testing.T) {
			f := newSDKFamilyFixture(t, store)
			chain := rotatedChain(t, f, 1)
			other := rotatedChain(t, f, 1)
			f.logOutAnswer(t, f.repo, chain[0])
			assert.True(t, f.repo.row(chain[1]).RevokedFor(sdkTokenLogoutReason), "the logged-out family's live row is revoked")
			assert.True(t, f.repo.row(other[1]).IsActive(), "the other family's live row stays active")

			out, status := postRefresh(t, f.refresh, other[1])
			assert.Equal(t, fiber.StatusOK, status, "the other family's live token still refreshes")
			assert.True(t, out.Rotated)
		})
	}
}
