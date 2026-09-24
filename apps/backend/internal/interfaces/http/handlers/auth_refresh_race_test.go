package handlers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/cache"
)

// Two presentations of one refresh token within a request's duration: both
// read "not revoked" before either writes. With atomic retirement the second
// write loses, and the loser is treated as a reuse (the family is revoked and
// the reuse is recorded) instead of minting a second live chain.

// raceStore is a rotationStore with set-if-absent whose reads can be made to
// report "not revoked" for the first N reads, which is what a concurrent
// presentation sees before the other one's write lands.
type raceStore struct {
	rotationStore
	blindReads int
	nxErr      error
}

func (s *raceStore) Exists(ctx context.Context, key string) (bool, error) {
	if s.blindReads > 0 {
		s.blindReads--
		return false, nil
	}
	return s.rotationStore.Exists(ctx, key)
}

func (s *raceStore) SetWithNX(_ context.Context, key string, _ interface{}, _ time.Duration) (bool, error) {
	if s.nxErr != nil {
		return false, s.nxErr
	}
	if s.data == nil {
		s.data = map[string]bool{}
	}
	if s.data[key] {
		return false, nil
	}
	s.data[key] = true
	s.writes++
	return true, nil
}

func raceApp(t *testing.T, store auth.RevocationStore) *familyFixture {
	t.Helper()
	f := &familyFixture{}
	userID, orgID := uuid.New(), uuid.New()
	users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
		return activeUser(userID, orgID, domain.RoleAdmin, "race@example.com"), nil
	}}
	f.audit = &familyAuditRepo{}
	app, jwtSvc := newRefreshTestAppAudit(t, users, nil, f.audit)
	jwtSvc.SetRevoker(auth.NewTokenRevoker(store, false))
	_, refresh, err := jwtSvc.GenerateTokenPair(userID.String(), orgID.String(), "race@example.com", "admin")
	require.NoError(t, err)
	f.app, f.svc, f.p1, f.userID, f.orgID = app, jwtSvc, refresh, userID, orgID
	return f
}

// N1: both presentations read "not revoked"; one rotates, the other loses the
// write and is refused as a reuse: family revoked, one reuse row, no tokens minted for it.
func TestRefreshToken_ConcurrentPresentationLoserIsAReuse(t *testing.T) {
	store := &raceStore{blindReads: 4} // both presentations' jti and family reads come back empty
	f := raceApp(t, store)
	jti1 := jtiOfToken(t, f.svc, f.p1)

	out1, status1 := postRefresh(t, f.app, f.p1)
	require.Equal(t, fiber.StatusOK, status1)
	assert.True(t, out1.Rotated)

	body, status2 := postRefreshRaw(t, f.app, f.p1)
	assert.Equal(t, fiber.StatusUnauthorized, status2, "the presentation that lost the write is refused")
	assert.Contains(t, body, familyRefusal)
	assert.NotContains(t, body, "accessToken")
	assert.True(t, store.data["revoked:fam:"+jti1], "the family is revoked")
	require.Len(t, f.audit.rows, 1)
	assert.Equal(t, domain.AuditActionRefreshTokenReuse, f.audit.rows[0].Action)
	assert.Equal(t, jti1, f.audit.rows[0].Metadata["jti"])

	_, status := postRefresh(t, f.app, out1.RefreshToken)
	assert.Equal(t, fiber.StatusUnauthorized, status, "the chain that won is over too: one family")
}

// N2 (pin): a store without set-if-absent keeps today's behaviour: both blind
// presentations rotate, and the fork is caught only by a later reuse.
func TestRefreshToken_PlainStoreStillForksUnderARace(t *testing.T) {
	store := &rotationStore{}
	f := raceApp(t, &blindPlainStore{rotationStore: store, blindReads: 4})
	_, status1 := postRefresh(t, f.app, f.p1)
	_, status2 := postRefresh(t, f.app, f.p1)
	assert.Equal(t, fiber.StatusOK, status1)
	assert.Equal(t, fiber.StatusOK, status2, "without set-if-absent the second presentation still rotates")
	assert.Empty(t, f.audit.rows)
}

type blindPlainStore struct {
	*rotationStore
	blindReads int
}

func (s *blindPlainStore) Exists(ctx context.Context, key string) (bool, error) {
	if s.blindReads > 0 {
		s.blindReads--
		return false, nil
	}
	return s.rotationStore.Exists(ctx, key)
}

// N3: a failed set-if-absent write is a store failure: the presented token
// comes back unchanged with rotated=false and nothing is recorded.
func TestRefreshToken_SetIfAbsentFailureDoesNotRotate(t *testing.T) {
	f := raceApp(t, &raceStore{nxErr: errors.New("store down")})
	out, status := postRefresh(t, f.app, f.p1)
	require.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, f.p1, out.RefreshToken)
	assert.False(t, out.Rotated)
	assert.Empty(t, f.audit.rows)
}

// N4 (pin): the Redis cache satisfies the set-if-absent store interface, so a
// stack with Redis gets atomic retirement without configuration.
func TestRedisCacheIsASetIfAbsentStore(t *testing.T) {
	var _ auth.RevocationStoreNX = (*cache.RedisCache)(nil)
	var _ auth.RevocationStore = (*cache.RedisCache)(nil)
}
