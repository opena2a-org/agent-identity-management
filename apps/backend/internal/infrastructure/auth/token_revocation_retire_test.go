package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Atomic retirement: on a store with set-if-absent, the first write retires
// the token and a second write of the same jti reports that it lost; on a
// store without it, writes cannot tell and lost is always false.

// nxStore is a familyStore with set-if-absent.
type nxStore struct {
	familyStore
	nxErr error
}

func (s *nxStore) SetWithNX(_ context.Context, key string, _ interface{}, ttl time.Duration) (bool, error) {
	if s.nxErr != nil {
		return false, s.nxErr
	}
	if s.keys == nil {
		s.keys = map[string]time.Duration{}
	}
	if _, present := s.keys[key]; present {
		return false, nil
	}
	s.keys[key] = ttl
	return true, nil
}

// T1: on a set-if-absent store the second retirement of one jti loses.
func TestRetire_SecondWriterLosesOnASetIfAbsentStore(t *testing.T) {
	store := &nxStore{}
	r := NewTokenRevoker(store, false)
	ctx := context.Background()
	retired, lost, err := r.Retire(ctx, "jti-1", time.Hour)
	require.NoError(t, err)
	assert.True(t, retired)
	assert.False(t, lost)
	retired, lost, err = r.Retire(ctx, "jti-1", time.Hour)
	require.NoError(t, err)
	assert.False(t, retired, "the second writer did not retire it")
	assert.True(t, lost, "and knows it lost")
	assert.True(t, r.IsRevoked(ctx, "jti-1"))
}

// T2 (pin): a store without set-if-absent keeps today's write: both calls report retired.
func TestRetire_PlainStoreCannotTell(t *testing.T) {
	store := &familyStore{}
	r := NewTokenRevoker(store, false)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		retired, lost, err := r.Retire(ctx, "jti-2", time.Hour)
		require.NoError(t, err)
		assert.True(t, retired)
		assert.False(t, lost)
	}
}

// T3: a set-if-absent write that fails is an error, not a loss.
func TestRetire_SetIfAbsentErrorIsAnError(t *testing.T) {
	store := &nxStore{nxErr: errors.New("store down")}
	r := NewTokenRevoker(store, false)
	retired, lost, err := r.Retire(context.Background(), "jti-3", time.Hour)
	assert.Error(t, err)
	assert.False(t, retired)
	assert.False(t, lost)
}

// T4: RetireTokenChecked carries the same three answers for a real token.
func TestRetireTokenChecked_ReportsLost(t *testing.T) {
	svc := familyTestService(t, "2h")
	store := &nxStore{}
	svc.SetRevoker(NewTokenRevoker(store, false))
	p1, err := svc.GenerateRefreshToken(uuid.New().String(), uuid.New().String())
	require.NoError(t, err)
	ctx := context.Background()
	retired, lost, err := svc.RetireTokenChecked(ctx, p1)
	require.NoError(t, err)
	assert.True(t, retired)
	assert.False(t, lost)
	retired, lost, err = svc.RetireTokenChecked(ctx, p1)
	require.NoError(t, err)
	assert.False(t, retired)
	assert.True(t, lost)
	bare := familyTestService(t, "2h")
	retired, lost, err = bare.RetireTokenChecked(ctx, p1)
	assert.NoError(t, err)
	assert.False(t, retired)
	assert.False(t, lost, "no revoker: nothing retired, nothing lost")
}
