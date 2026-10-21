package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAccountDeletion_DueAtIsTheRequestTimePlusTheDelay(t *testing.T) {
	requested := time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC)
	deletion := AccountDeletion{RequestedAt: requested}

	assert.Equal(t, time.Date(2026, 10, 30, 9, 30, 0, 0, time.UTC), deletion.DueAt(), "29 days after the request")
	assert.Equal(t, requested.Add(AccountDeletionDelay), deletion.DueAt())
}
