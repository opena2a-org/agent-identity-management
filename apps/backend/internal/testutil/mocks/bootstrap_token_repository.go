package mocks

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// MemoryBootstrapTokenRepository is an in-memory domain.BootstrapTokenRepository
// whose methods apply the same conditions as the SQL ones, each under one
// lock, so a test sees the same single-use and scoping behaviour.
type MemoryBootstrapTokenRepository struct {
	mu     sync.Mutex
	tokens map[uuid.UUID]*domain.BootstrapToken
}

// NewMemoryBootstrapTokenRepository creates an empty repository.
func NewMemoryBootstrapTokenRepository() *MemoryBootstrapTokenRepository {
	return &MemoryBootstrapTokenRepository{tokens: map[uuid.UUID]*domain.BootstrapToken{}}
}

var _ domain.BootstrapTokenRepository = (*MemoryBootstrapTokenRepository)(nil)

func copyBootstrapToken(t *domain.BootstrapToken) *domain.BootstrapToken {
	c := *t
	return &c
}

func isOpenBootstrapToken(t *domain.BootstrapToken) bool {
	return t.UsedAt == nil && t.RevokedAt == nil
}

// CreateReplacingOpen revokes the user's open tokens in the organization and stores t.
func (r *MemoryBootstrapTokenRepository) CreateReplacingOpen(_ context.Context, t *domain.BootstrapToken, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.tokens {
		if existing.OrganizationID == t.OrganizationID && existing.CreatedBy == t.CreatedBy && isOpenBootstrapToken(existing) {
			at := now
			existing.RevokedAt = &at
		}
	}
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	r.tokens[t.ID] = copyBootstrapToken(t)
	return nil
}

// GetByHash returns a copy of the token stored under hash.
func (r *MemoryBootstrapTokenRepository) GetByHash(_ context.Context, hash string) (*domain.BootstrapToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.tokens {
		if t.TokenHash == hash {
			return copyBootstrapToken(t), nil
		}
	}
	return nil, domain.ErrBootstrapTokenNotFound
}

// Claim marks the token used if it is open and unexpired at now.
func (r *MemoryBootstrapTokenRepository) Claim(_ context.Context, id uuid.UUID, now time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tokens[id]
	if !ok || !isOpenBootstrapToken(t) || !t.ExpiresAt.After(now) {
		return false, nil
	}
	at := now
	t.UsedAt = &at
	return true, nil
}

// ReleaseClaim reopens a token claimed at usedAt that carries no agent.
func (r *MemoryBootstrapTokenRepository) ReleaseClaim(_ context.Context, id uuid.UUID, usedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tokens[id]
	if ok && t.UsedAt != nil && t.UsedAt.Equal(usedAt) && t.RevokedAt == nil && t.AgentID == nil {
		t.UsedAt = nil
	}
	return nil
}

// SetAgent records the agent a claimed token registered.
func (r *MemoryBootstrapTokenRepository) SetAgent(_ context.Context, id, agentID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.tokens[id]; ok && t.UsedAt != nil {
		a := agentID
		t.AgentID = &a
	}
	return nil
}

// RevokeOpenForUser revokes the open tokens userID minted in orgID.
func (r *MemoryBootstrapTokenRepository) RevokeOpenForUser(_ context.Context, orgID, userID uuid.UUID, now time.Time) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for _, t := range r.tokens {
		if t.OrganizationID == orgID && t.CreatedBy == userID && isOpenBootstrapToken(t) {
			at := now
			t.RevokedAt = &at
			n++
		}
	}
	return n, nil
}

// RevokeByHash revokes the open token stored under hash.
func (r *MemoryBootstrapTokenRepository) RevokeByHash(_ context.Context, hash string, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.tokens {
		if t.TokenHash == hash && isOpenBootstrapToken(t) {
			at := now
			t.RevokedAt = &at
		}
	}
	return nil
}

// All returns copies of every stored token.
func (r *MemoryBootstrapTokenRepository) All() []*domain.BootstrapToken {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*domain.BootstrapToken, 0, len(r.tokens))
	for _, t := range r.tokens {
		out = append(out, copyBootstrapToken(t))
	}
	return out
}
