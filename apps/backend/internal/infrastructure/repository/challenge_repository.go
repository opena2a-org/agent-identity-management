package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ChallengeData stores challenge information for agent verification
type ChallengeData struct {
	AgentID   uuid.UUID `json:"agent_id"`
	Nonce     string    `json:"nonce"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
}

// ChallengeRepository handles challenge storage and retrieval
type ChallengeRepository struct {
	redis *redis.Client
}

// NewChallengeRepository creates a new challenge repository
func NewChallengeRepository(redis *redis.Client) *ChallengeRepository {
	return &ChallengeRepository{
		redis: redis,
	}
}

// StoreChallenge stores a challenge in Redis with TTL
func (r *ChallengeRepository) StoreChallenge(ctx context.Context, challengeID string, data ChallengeData) error {
	// Serialize challenge data
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal challenge data: %w", err)
	}

	// Calculate TTL from expiration time
	ttl := time.Until(data.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("challenge already expired")
	}

	// Store in Redis with TTL
	key := fmt.Sprintf("challenge:%s", challengeID)
	if err := r.redis.Set(ctx, key, jsonData, ttl).Err(); err != nil {
		return fmt.Errorf("failed to store challenge in Redis: %w", err)
	}

	return nil
}

// GetChallenge retrieves a challenge from Redis
func (r *ChallengeRepository) GetChallenge(ctx context.Context, challengeID string) (*ChallengeData, error) {
	key := fmt.Sprintf("challenge:%s", challengeID)

	// Get from Redis
	jsonData, err := r.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("challenge not found or expired")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve challenge from Redis: %w", err)
	}

	// Deserialize
	var data ChallengeData
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal challenge data: %w", err)
	}

	// Check if expired (double-check even though Redis TTL should handle this)
	if time.Now().After(data.ExpiresAt) {
		// Clean up expired challenge
		r.DeleteChallenge(ctx, challengeID)
		return nil, fmt.Errorf("challenge expired")
	}

	return &data, nil
}

// MarkChallengeUsed marks a challenge as used (replay attack prevention)
func (r *ChallengeRepository) MarkChallengeUsed(ctx context.Context, challengeID string) error {
	data, err := r.GetChallenge(ctx, challengeID)
	if err != nil {
		return err
	}

	if data.Used {
		return fmt.Errorf("challenge already used")
	}

	// Update used flag
	data.Used = true
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal challenge data: %w", err)
	}

	// Update in Redis (keep original TTL)
	key := fmt.Sprintf("challenge:%s", challengeID)
	ttl := time.Until(data.ExpiresAt)
	if ttl > 0 {
		if err := r.redis.Set(ctx, key, jsonData, ttl).Err(); err != nil {
			return fmt.Errorf("failed to update challenge in Redis: %w", err)
		}
	}

	return nil
}

// DeleteChallenge removes a challenge from Redis
func (r *ChallengeRepository) DeleteChallenge(ctx context.Context, challengeID string) error {
	key := fmt.Sprintf("challenge:%s", challengeID)
	if err := r.redis.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete challenge from Redis: %w", err)
	}
	return nil
}

// CleanupExpiredChallenges removes expired challenges (called by background job)
// Note: Redis TTL handles automatic cleanup, but this is for manual cleanup if needed
func (r *ChallengeRepository) CleanupExpiredChallenges(ctx context.Context) (int, error) {
	// Redis automatically removes expired keys with TTL
	// This function is here for compatibility but not strictly necessary
	return 0, nil
}
