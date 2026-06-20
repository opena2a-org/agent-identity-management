package domain

import (
	"time"

	"github.com/google/uuid"
)

// UserFeedback represents an explicit rating a user gave an agent.
// Used by the trust score's User Feedback factor (factor 8, 2% weight).
type UserFeedback struct {
	ID             uuid.UUID  `json:"id"`
	AgentID        uuid.UUID  `json:"agentId"`
	OrganizationID uuid.UUID  `json:"organizationId"`
	UserID         *uuid.UUID `json:"userId,omitempty"` // who left the feedback (nil if anonymous/system)
	Rating         int        `json:"rating"`           // 1-5 stars
	Comment        string     `json:"comment,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// UserFeedbackStats aggregates feedback for an agent so the trust calculator
// can apply the documented count-threshold scoring formula without loading
// every row.
//
// Sentiment buckets (rating is 1-5):
//   - Positive: rating >= 4
//   - Neutral:  rating == 3
//   - Negative: rating <= 2
type UserFeedbackStats struct {
	Total         int     `json:"total"`
	PositiveCount int     `json:"positiveCount"`
	NeutralCount  int     `json:"neutralCount"`
	NegativeCount int     `json:"negativeCount"`
	AverageRating float64 `json:"averageRating"`
}

// FeedbackSentiment classifies a 1-5 rating into a sentiment bucket.
func FeedbackSentiment(rating int) string {
	switch {
	case rating >= 4:
		return "positive"
	case rating <= 2:
		return "negative"
	default:
		return "neutral"
	}
}

// UserFeedbackRepository defines persistence for user feedback.
type UserFeedbackRepository interface {
	Create(feedback *UserFeedback) error
	GetByAgentID(agentID uuid.UUID, limit, offset int) ([]*UserFeedback, error)
	GetStats(agentID uuid.UUID) (*UserFeedbackStats, error)
}
