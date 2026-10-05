package handlers

import (
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// A poll on a code that was already exchanged for its pair receives no tokens
// and the OAuth answer for a grant that is no longer valid: invalid_grant
// (RFC 6749 Section 5.2, inherited by RFC 8628 Section 3.5).
func TestDeviceTokenPoll_ConsumedCode_AnswersInvalidGrant(t *testing.T) {
	f := newMintFixture(t, nil, false)
	repo := &mintDeviceRepo{code: &domain.DeviceCode{ID: uuid.New(), DeviceCode: "dev", UserCode: "ABCDEFGH", ClientID: "aim-sdk", ExpiresAt: time.Now().Add(10 * time.Minute), Status: domain.DeviceCodeStatusConsumed, CreatedAt: time.Now()}}
	svc := application.NewDeviceAuthService(repo, f.svc, nil, "http://localhost:3000")
	h := NewDeviceAuthHandler(svc, f.svc, application.NewAuditService(f.audit))
	app := fiber.New()
	app.Post("/oauth/device/token", h.PollDeviceToken)

	status, body := doRequest(t, app, "POST", "/oauth/device/token",
		`{"deviceCode":"dev","grantType":"urn:ietf:params:oauth:grant-type:device_code"}`)
	assert.Equal(t, fiber.StatusBadRequest, status)
	assert.Contains(t, body, `"error":"invalid_grant"`)
	assert.NotContains(t, body, "accessToken")
}
