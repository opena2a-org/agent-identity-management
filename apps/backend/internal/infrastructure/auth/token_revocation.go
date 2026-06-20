package auth

import (
	"context"
	"sync"
	"time"
)

const (
	revokedKeyPrefix = "revoked:jti:"
	// negCacheTTL bounds how long a "not revoked" result is cached in-process.
	// It also bounds cross-instance revocation lag (a token revoked on another
	// instance is honored within this window) and the blast radius of a store
	// outage (cached tokens are served without touching the store).
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

// Revoke denylists a jti until ttl elapses. Best-effort: a nil receiver/store
// or non-positive ttl is a no-op. Evicts any local "not revoked" cache entry so
// the revocation is enforced immediately on this instance.
func (r *TokenRevoker) Revoke(ctx context.Context, jti string, ttl time.Duration) error {
	if r == nil || r.store == nil || jti == "" || ttl <= 0 {
		return nil
	}
	r.mu.Lock()
	delete(r.negCache, jti)
	r.mu.Unlock()
	return r.store.Set(ctx, revokedKeyPrefix+jti, "1", ttl)
}
