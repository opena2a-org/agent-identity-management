package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The request middleware checks an access token's session (token family)
// beside its jti, through the negative cache: a cached "not revoked" answer
// records the family it was checked with, a miss reads the jti key and then
// the family key, and a family revocation drops the cached answers of its
// members on the revoking instance at once. Another instance honors it once
// its cached answer lapses (negCacheTTL).

// sessionCheckStore is a RevocationStore double that records every key read,
// and whose family-key reads and writes can be made to fail.
type sessionCheckStore struct {
	keys      map[string]bool
	reads     []string
	familyErr error
	setErr    error
}

func (s *sessionCheckStore) Exists(_ context.Context, key string) (bool, error) {
	s.reads = append(s.reads, key)
	if s.familyErr != nil && strings.HasPrefix(key, revokedFamilyKeyPrefix) {
		return false, s.familyErr
	}
	return s.keys[key], nil
}

func (s *sessionCheckStore) Set(_ context.Context, key string, _ interface{}, _ time.Duration) error {
	if s.setErr != nil {
		return s.setErr
	}
	if s.keys == nil {
		s.keys = map[string]bool{}
	}
	s.keys[key] = true
	return nil
}

// readsOf returns the keys the check read.
func (s *sessionCheckStore) readsOf(check func()) []string {
	before := len(s.reads)
	check()
	return append([]string{}, s.reads[before:]...)
}

// frozenAt pins the revoker's clock to the returned time, which the test may
// move.
func frozenAt(r *TokenRevoker) *time.Time {
	clock := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return clock }
	return &clock
}

// C2a: a family revoked through this revoker is refused at once, with no
// clock advance, although its member's "not revoked" answer was cached; the
// cached answer of another family is kept.
func TestTokenRevoker_FamilyRevokedHereIsRefusedAtOnce(t *testing.T) {
	store := &sessionCheckStore{}
	r := NewTokenRevoker(store, false)
	frozenAt(r)
	ctx := context.Background()

	require.False(t, r.IsSessionRevoked(ctx, "jti-1", "fam-1"), "control: not revoked")
	require.False(t, r.IsSessionRevoked(ctx, "jti-2", "fam-2"))
	require.Empty(t, store.readsOf(func() { r.IsSessionRevoked(ctx, "jti-1", "fam-1") }), "the answer is cached")

	require.NoError(t, r.RevokeFamily(ctx, "fam-1", time.Hour))

	assert.True(t, r.IsSessionRevoked(ctx, "jti-1", "fam-1"), "refused at once on the revoking instance")
	assert.Empty(t, store.readsOf(func() {
		assert.False(t, r.IsSessionRevoked(ctx, "jti-2", "fam-2"))
	}), "another family's cached answer is kept")
}

// gatedStore holds the first family-key read open after taking its answer,
// so a check can be in flight while a revocation is written.
type gatedStore struct {
	mu      sync.Mutex
	keys    map[string]bool
	held    bool
	reading chan struct{}
	release chan struct{}
}

func (s *gatedStore) Exists(_ context.Context, key string) (bool, error) {
	s.mu.Lock()
	answer := s.keys[key]
	hold := strings.HasPrefix(key, revokedFamilyKeyPrefix) && !s.held
	if hold {
		s.held = true
	}
	s.mu.Unlock()
	if hold {
		close(s.reading)
		<-s.release
	}
	return answer, nil
}

func (s *gatedStore) Set(_ context.Context, key string, _ interface{}, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.keys == nil {
		s.keys = map[string]bool{}
	}
	s.keys[key] = true
	return nil
}

// C2a (in flight): a check that read the family key before a revocation was
// written here, and finishes after it, does not cache its answer, so the
// next check reads the key and refuses.
func TestTokenRevoker_ACheckInFlightAcrossAFamilyRevocationIsNotCached(t *testing.T) {
	store := &gatedStore{reading: make(chan struct{}), release: make(chan struct{})}
	r := NewTokenRevoker(store, false)
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		r.IsSessionRevoked(ctx, "jti-1", "fam-1")
	}()
	select {
	case <-store.reading:
	case <-done:
		t.Fatal("the check returned without reading the family key")
	}
	require.NoError(t, r.RevokeFamily(ctx, "fam-1", time.Hour))
	close(store.release)
	<-done

	assert.True(t, r.IsSessionRevoked(ctx, "jti-1", "fam-1"), "the next check reads the family key")
}

// C2b: two revokers on one store stand for two instances. A family revoked
// on A is refused on B once B's cached answer has lapsed (negCacheTTL).
func TestTokenRevoker_FamilyRevokedOnAnotherInstanceIsRefusedOnceTheCacheLapses(t *testing.T) {
	store := &sessionCheckStore{}
	a := NewTokenRevoker(store, false)
	b := NewTokenRevoker(store, false)
	clock := frozenAt(b)
	ctx := context.Background()

	require.False(t, b.IsSessionRevoked(ctx, "jti-b", "fam-b"), "control: B checks, and caches, before the revocation")
	require.NoError(t, a.RevokeFamily(ctx, "fam-b", time.Hour))

	*clock = clock.Add(negCacheTTL)
	assert.True(t, b.IsSessionRevoked(ctx, "jti-b", "fam-b"), "B refuses once its cached answer has lapsed")
}

// C3: a store error on the family read answers !failOpen, as the jti read
// does, and caches nothing.
func TestTokenRevoker_FamilyReadErrorFollowsTheFailOpenSetting(t *testing.T) {
	ctx := context.Background()
	closed := &sessionCheckStore{familyErr: errors.New("store down")}
	rc := NewTokenRevoker(closed, false)
	assert.True(t, rc.IsSessionRevoked(ctx, "jti-1", "fam-1"), "fail closed: revoked")
	assert.Len(t, closed.readsOf(func() { rc.IsSessionRevoked(ctx, "jti-1", "fam-1") }), 2, "nothing was cached")

	open := &sessionCheckStore{familyErr: errors.New("store down")}
	ro := NewTokenRevoker(open, true)
	assert.False(t, ro.IsSessionRevoked(ctx, "jti-1", "fam-1"), "fail open: not revoked")
	assert.Len(t, open.readsOf(func() { ro.IsSessionRevoked(ctx, "jti-1", "fam-1") }), 2, "nothing was cached")
}

// C4: a hit reads no key; a miss reads the jti key and, only for a token with
// a family, the family key; a revoked jti answers without the family read.
// An answer checked with a family serves a jti-only check, not the reverse.
func TestTokenRevoker_SessionCheckReads(t *testing.T) {
	store := &sessionCheckStore{keys: map[string]bool{"revoked:jti:jti-3": true}}
	r := NewTokenRevoker(store, false)
	ctx := context.Background()

	assert.Equal(t, []string{"revoked:jti:jti-1", "revoked:fam:fam-1"},
		store.readsOf(func() { r.IsSessionRevoked(ctx, "jti-1", "fam-1") }), "a miss with a family")
	assert.Empty(t, store.readsOf(func() { r.IsSessionRevoked(ctx, "jti-1", "fam-1") }), "a hit")
	assert.Equal(t, []string{"revoked:jti:jti-2"},
		store.readsOf(func() { r.IsSessionRevoked(ctx, "jti-2", "") }), "a miss without a family")
	assert.Empty(t, store.readsOf(func() { r.IsRevoked(ctx, "jti-1") }), "a family-checked answer serves a jti-only check")
	assert.Equal(t, []string{"revoked:jti:jti-2", "revoked:fam:fam-2"},
		store.readsOf(func() { r.IsSessionRevoked(ctx, "jti-2", "fam-2") }), "a jti-only answer does not serve a family check")
	assert.Equal(t, []string{"revoked:jti:jti-3"}, store.readsOf(func() {
		assert.True(t, r.IsSessionRevoked(ctx, "jti-3", "fam-3"))
	}), "a revoked jti answers without the family read")

	t.Run("a family revocation the store refused still drops the cached answers", func(t *testing.T) {
		store.setErr = errors.New("store down")
		assert.Error(t, r.RevokeFamily(ctx, "fam-1", time.Hour))
		assert.Len(t, store.readsOf(func() { r.IsSessionRevoked(ctx, "jti-1", "fam-1") }), 2)
	})
}

// The revocation checks are disabled with no revoker or no store, as
// IsRevoked is.
func TestTokenRevoker_SessionCheckNeedsAStore(t *testing.T) {
	ctx := context.Background()
	var none *TokenRevoker
	assert.False(t, none.IsSessionRevoked(ctx, "jti-1", "fam-1"))
	assert.NoError(t, none.RevokeFamily(ctx, "fam-1", time.Hour))
	assert.False(t, NewTokenRevoker(nil, false).IsSessionRevoked(ctx, "jti-1", "fam-1"))
	assert.False(t, (&JWTService{}).IsSessionRevoked(ctx, "jti-1", "fam-1"), "a service with no revoker")
}

// fillToTheCap caches negCacheMaxLen answers, three jtis per family.
func fillToTheCap(r *TokenRevoker, until time.Time) {
	for i := 0; i < negCacheMaxLen; i++ {
		r.negCache[fmt.Sprintf("jti-%d", i)] = negEntry{family: fmt.Sprintf("fam-%d", i/3), until: until}
	}
}

// C6 (eviction pass): at the cache's cap a family revocation drops exactly
// that family's answers. The pass is one scan of the cache under its lock;
// the time of one revocation at the cap is logged, and the benchmark below
// measures it.
func TestTokenRevoker_RevokeFamilyAtTheCapDropsOnlyThatFamily(t *testing.T) {
	r := NewTokenRevoker(&sessionCheckStore{}, false)
	fillToTheCap(r, time.Now().Add(time.Hour))
	ctx := context.Background()

	start := time.Now()
	require.NoError(t, r.RevokeFamily(ctx, "fam-7", time.Hour))
	elapsed := time.Since(start)
	t.Logf("RevokeFamily with %d cached answers: %s", negCacheMaxLen, elapsed)

	assert.Len(t, r.negCache, negCacheMaxLen-3)
	for _, jti := range []string{"jti-21", "jti-22", "jti-23"} {
		_, cached := r.negCache[jti]
		assert.False(t, cached, jti)
	}
	_, kept := r.negCache["jti-24"]
	assert.True(t, kept, "the next family's answers are kept")
}

func BenchmarkTokenRevoker_RevokeFamilyAtTheCap(b *testing.B) {
	r := NewTokenRevoker(&sessionCheckStore{}, false)
	fillToTheCap(r, time.Now().Add(time.Hour))
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// no answer of this family is cached, so the cache stays at the cap
		_ = r.RevokeFamily(ctx, "fam-none", time.Hour)
	}
}
