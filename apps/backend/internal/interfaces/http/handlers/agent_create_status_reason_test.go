package handlers

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A registration that leaves the agent pending answers with the reason in
// statusReason, the field the SDKs print. Here no trust score is calculated,
// so the agent stays under the threshold.
func TestCreateAgent_PendingRegistrationAnswersWithItsReason(t *testing.T) {
	repo := &registeringAgentRepo{}
	status, answer := postCreateAgent(t, repo, `{"name":"pending-agent","displayName":"Pending Agent","description":"Waits","agentType":"ai_agent"}`)

	require.Equal(t, fiber.StatusCreated, status, "answer: %v", answer)
	assert.Equal(t, "pending", answer["status"])
	assert.Equal(t, "pending: trust score 0.00 is below the threshold 0.30", answer["statusReason"])
}
