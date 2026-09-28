package auth

import (
	"context"
	"sync"
	"time"
)

const (
	revokedKeyPrefix = "revoked:jti:"
	// revokedFamilyKeyPrefix denylists a whole token family (one sign-in on one
	// user agent or device, named by its login refresh token's jti): written
	// when a rotated-out member is presented again, or on logout.
	revokedFamilyKeyPrefix = "revoked:fam:"
	// negCacheTTL bounds how long a "not revoked" result is cached in-process.
	// It also bounds cross-instance revocation lag (a token revoked on another
	// instance is honored within this window) and the blast radius of a store
	// outage (cached tokens are served without touching the store). The cache
	// serves the request middleware only; the refresh route reads uncached
	// (CheckJTI, CheckFamily), because every refresh token is presented once
	// and a stale "not revoked" entry there would serve a replay.
	negCacheTTL    = 30 * time.Second
	negCacheMaxLen = 50000
)

// RevocationStore is the subset of the cache used for the jti denylist.
// *cache.RedisCache satisfies it structurally (no import of cache here, so no
// dependency cycle).
type RevocationStore interface {
	Exists(ctx context.Context, key string) (bool, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
}

// RevocationStoreNX is the optional set-if-absent form of the store. A store
// that has it makes retirement atomic: two presentations of one refresh token
// within a request's duration cannot both retire it, so the loser is a reuse
// instead of a second live chain. *cache.RedisCache satisfies it structurally.
type RevocationStoreNX interface {
	SetWithNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
}

// TokenRevoker maintains a jti denylist in a shared store (Redis) with a small
// in-process negative cache. On a store error it fails CLOSED by default
// (treats the token as revoked) unless failOpen is set.
type TokenRevoker struct {
	store    RevocationStore
	failOpen bool
	mu       sync.Mutex
	negCache map[string]time.Time // jti -> "known not revoked until"
}

// NewTokenRevoker builds a revoker over the given store. failOpen=false means a
// store error rejects the request (secure default); failOpen=true allows it.
func NewTokenRevoker(store RevocationStore, failOpen bool) *TokenRevoker {
	return &TokenRevoker{
		store:    store,
		failOpen: failOpen,
		negCache: make(map[string]time.Time),
	}
}

// IsRevoked reports whether the given jti has been revoked. nil receiver or no
// store means revocation is disabled (returns false) — absence of a store must
// not lock everyone out. On a store error the result is !failOpen.
func (r *TokenRevoker) IsRevoked(ctx context.Context, jti string) bool {
	if r == nil || r.store == nil || jti == "" {
		return false
	}

	now := time.Now()
	r.mu.Lock()
	if exp, ok := r.negCache[jti]; ok && now.Before(exp) {
		r.mu.Unlock()
		return false
	}
	r.mu.Unlock()

	exists, err := r.store.Exists(ctx, revokedKeyPrefix+jti)
	if err != nil {
		// Store unreachable: fail closed (revoked) unless explicitly fail-open.
		return !r.failOpen
	}
	if exists {
		return true
	}

	r.mu.Lock()
	if len(r.negCache) >= negCacheMaxLen {
		for k, e := range r.negCache {
			if now.After(e) {
				delete(r.negCache, k)
			}
		}
	}
	// Hard cap: if still full after evicting expired entries (pathological load),
	// skip caching this jti rather than growing unbounded. Correctness is
	// preserved — uncached jtis just consult the store each time.
	if len(r.negCache) < negCacheMaxLen {
		r.negCache[jti] = now.Add(negCacheTTL)
	}
	r.mu.Unlock()
	return false
}

// lookup reads one denylist key with no cache. revoked says whether the key
// exists, or, when the store did not answer, whether the fail-closed setting
// treats the token as revoked; known says whether the store answered. A
// caller that records evidence (a reuse event, a family revocation) acts only
// on a known result, so a store fault is never recorded as a reuse.
func (r *TokenRevoker) lookup(ctx context.Context, key string) (revoked, known bool) {
	if r == nil || r.store == nil || key == "" {
		return false, false
	}
	exists, err := r.store.Exists(ctx, key)
	if err != nil {
		return !r.failOpen, false
	}
	return exists, true
}

// CheckJTI is the uncached form of IsRevoked, with the store's answer status.
func (r *TokenRevoker) CheckJTI(ctx context.Context, jti string) (revoked, known bool) {
	if jti == "" {
		return false, false
	}
	return r.lookup(ctx, revokedKeyPrefix+jti)
}

// CheckFamily reports whether a token family is revoked, uncached, with the
// store's answer status.
func (r *TokenRevoker) CheckFamily(ctx context.Context, family string) (revoked, known bool) {
	if family == "" {
		return false, false
	}
	return r.lookup(ctx, revokedFamilyKeyPrefix+family)
}

// RevokeFamily denylists a token family until ttl elapses. A nil
// receiver/store, an empty id or a non-positive ttl is a no-op.
func (r *TokenRevoker) RevokeFamily(ctx context.Context, family string, ttl time.Duration) error {
	if r == nil || r.store == nil || family == "" || ttl <= 0 {
		return nil
	}
	return r.store.Set(ctx, revokedFamilyKeyPrefix+family, legacyRevokedValue, ttl)
}

// Retire denylists a jti as Revoke does and says whether this call was the
// one that retired it. lost is true when the key was already present at the
// moment of the write, which on a set-if-absent store means another
// presentation retired the token first. On a store without set-if-absent the
// write cannot tell, and lost is always false.
func (r *TokenRevoker) Retire(ctx context.Context, jti string, ttl time.Duration) (retired, lost bool, err error) {
	return r.retireWith(ctx, jti, ttl, legacyRevokedValue)
}

// retireWith is Retire writing value (a client mark, or "1") as the key's
// value, in the same write that retires the token.
func (r *TokenRevoker) retireWith(ctx context.Context, jti string, ttl time.Duration, value string) (retired, lost bool, err error) {
	if r == nil || r.store == nil || jti == "" || ttl <= 0 {
		return false, false, nil
	}
	nx, ok := r.store.(RevocationStoreNX)
	if !ok {
		if err := r.revokeWith(ctx, jti, ttl, value); err != nil {
			return false, false, err
		}
		return true, false, nil
	}
	r.mu.Lock()
	delete(r.negCache, jti)
	r.mu.Unlock()
	written, err := nx.SetWithNX(ctx, revokedKeyPrefix+jti, value, ttl)
	if err != nil {
		return false, false, err
	}
	if !written {
		return false, true, nil
	}
	return true, false, nil
}

// Revoke denylists a jti until ttl elapses. Best-effort: a nil receiver/store
// or non-positive ttl is a no-op. Evicts any local "not revoked" cache entry so
// the revocation is enforced immediately on this instance.
func (r *TokenRevoker) Revoke(ctx context.Context, jti string, ttl time.Duration) error {
	return r.revokeWith(ctx, jti, ttl, legacyRevokedValue)
}

// revokeWith is Revoke writing value as the key's value.
func (r *TokenRevoker) revokeWith(ctx context.Context, jti string, ttl time.Duration, value string) error {
	if r == nil || r.store == nil || jti == "" || ttl <= 0 {
		return nil
	}
	r.mu.Lock()
	delete(r.negCache, jti)
	r.mu.Unlock()
	return r.store.Set(ctx, revokedKeyPrefix+jti, value, ttl)
}

// revokeKeepFirst denylists a jti like revokeWith but never replaces a value
// already stored: the first writer's client mark stands. Logout uses it, so
// presenting an already-rotated token at logout cannot overwrite the mark of
// the client that rotated it (which would turn a later replay into a false
// sameClient). On a set-if-absent store the write is atomic; on a plain store
// an existing key is left as it is. Either way the jti is denylisted.
func (r *TokenRevoker) revokeKeepFirst(ctx context.Context, jti string, ttl time.Duration, value string) error {
	if r == nil || r.store == nil || jti == "" || ttl <= 0 {
		return nil
	}
	r.mu.Lock()
	delete(r.negCache, jti)
	r.mu.Unlock()
	if nx, ok := r.store.(RevocationStoreNX); ok {
		_, err := nx.SetWithNX(ctx, revokedKeyPrefix+jti, value, ttl)
		return err
	}
	exists, err := r.store.Exists(ctx, revokedKeyPrefix+jti)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return r.store.Set(ctx, revokedKeyPrefix+jti, value, ttl)
}

// readJTI returns the value stored for a denylisted jti, uncached. ok is false
// when the store has no read, the read fails or misses, or the value is not a
// string; the caller then classifies nothing.
func (r *TokenRevoker) readJTI(ctx context.Context, jti string) (value string, ok bool) {
	if r == nil || r.store == nil || jti == "" {
		return "", false
	}
	reader, can := r.store.(RevocationStoreReader)
	if !can {
		return "", false
	}
	if err := reader.Get(ctx, revokedKeyPrefix+jti, &value); err != nil {
		return "", false
	}
	return value, true
}
