package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFeedbackSentiment_BucketsEachRating(t *testing.T) {
	for rating, want := range map[int]string{
		1: "negative",
		2: "negative",
		3: "neutral",
		4: "positive",
		5: "positive",
	} {
		assert.Equal(t, want, FeedbackSentiment(rating), "rating %d", rating)
	}
}
