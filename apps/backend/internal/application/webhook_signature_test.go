package application

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests in this file pin how a webhook delivery is signed: each attempt
// carries X-Webhook-Timestamp, the time it was sent in Unix seconds, and
// X-Webhook-Signature, v1= and the hex HMAC-SHA256 of "<timestamp>.<body>"
// under the webhook's secret.

const (
	signatureTestSecret = "fake-webhook-signing-secret"
	signatureTestBody   = `{"id":"0c6f3c1e-8f0e-4a63-9a43-2f1d8b7e5a10","event":"agent.created"}`
)

// signatureTestSentAt is 2025-10-17T00:00:00Z.
var signatureTestSentAt = time.Unix(1760659200, 0)

// The signed string is the timestamp, a dot and the body. The expected value
// was computed outside Go, with Python's hmac and hashlib:
// hmac.new(secret, b"1760659200." + body, hashlib.sha256).hexdigest().
func TestSignDelivery_SignsTheTimestampAndTheBody(t *testing.T) {
	timestamp, signature := signDelivery([]byte(signatureTestBody), signatureTestSecret, signatureTestSentAt)

	assert.Equal(t, "1760659200", timestamp)
	assert.Equal(t, "v1=874af0e00ccf570390defe0c6274ea9f0cad24936d208171529193b8353fb394", signature)
	assert.Equal(t, "v1="+createSignature([]byte("1760659200."+signatureTestBody), signatureTestSecret), signature)
}

// The signature is not the HMAC of the body alone, the form a receiver could
// not tell apart from a replay.
func TestSignDelivery_IsNotTheSignatureOfTheBodyAlone(t *testing.T) {
	_, signature := signDelivery([]byte(signatureTestBody), signatureTestSecret, signatureTestSentAt)

	bodyAlone := createSignature([]byte(signatureTestBody), signatureTestSecret)
	assert.Equal(t, "c5e3e53b9a2bb7512d46d1052f2e2c529967a7fc46e1f962b2ab120b0ce05a37", bodyAlone)
	assert.NotContains(t, signature, bodyAlone)
}

// An attempt sent a second later, the same body with another timestamp, has
// another signature; so does another body or another secret.
func TestSignDelivery_ChangesWithTheTimestampTheBodyAndTheSecret(t *testing.T) {
	timestamp, signature := signDelivery([]byte(signatureTestBody), signatureTestSecret, signatureTestSentAt)

	laterTimestamp, later := signDelivery([]byte(signatureTestBody), signatureTestSecret, signatureTestSentAt.Add(time.Second))
	assert.Equal(t, "1760659201", laterTimestamp)
	assert.NotEqual(t, signature, later)

	otherTimestamp, otherBody := signDelivery([]byte(strings.Replace(signatureTestBody, "agent.created", "agent.deleted", 1)), signatureTestSecret, signatureTestSentAt)
	assert.Equal(t, timestamp, otherTimestamp)
	assert.NotEqual(t, signature, otherBody)

	_, otherSecret := signDelivery([]byte(signatureTestBody), signatureTestSecret+"-other", signatureTestSentAt)
	assert.NotEqual(t, signature, otherSecret)
}

// The timestamp is whole Unix seconds whatever the time zone or the fraction
// of a second.
func TestSignDelivery_TimestampIsUnixSeconds(t *testing.T) {
	denver := time.FixedZone("MDT", -6*60*60)
	timestamp, _ := signDelivery([]byte(signatureTestBody), signatureTestSecret, signatureTestSentAt.Add(999*time.Millisecond).In(denver))
	assert.Equal(t, "1760659200", timestamp)
}

// setDeliveryHeaders sets both headers from the sending time it is given, next
// to the headers a delivery carried before.
func TestSetDeliveryHeaders_SignsWithTheSendingTime(t *testing.T) {
	webhook := &domain.Webhook{ID: uuid.New(), Secret: signatureTestSecret}
	req, err := http.NewRequest(http.MethodPost, "https://example.com/hook", strings.NewReader(signatureTestBody))
	require.NoError(t, err)

	setDeliveryHeaders(req, webhook, "agent.created", []byte(signatureTestBody), signatureTestSentAt)

	assert.Equal(t, "1760659200", req.Header.Get("X-Webhook-Timestamp"))
	assert.Equal(t, "v1=874af0e00ccf570390defe0c6274ea9f0cad24936d208171529193b8353fb394", req.Header.Get("X-Webhook-Signature"))
	assert.Equal(t, "agent.created", req.Header.Get("X-Webhook-Event"))
	assert.Equal(t, webhook.ID.String(), req.Header.Get("X-Webhook-ID"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, "AIM-Webhook/1.0", req.Header.Get("User-Agent"))
}

// An event delivery attempt and a test delivery are each signed with the time
// they are sent, so a receiver verifies both the same way.
func TestDeliveryAttemptAndTestDelivery_AreSignedWhenSent(t *testing.T) {
	endpoint, lastHeaders := testDeliveryEndpoint(t, http.StatusOK)
	svc, mock := newTestDeliveryService(t)
	webhook := &domain.Webhook{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		URL:            endpoint.URL + "/hook",
		Events:         []domain.WebhookEvent{domain.WebhookEventAgentCreated},
		Secret:         signatureTestSecret,
	}

	before := time.Now().Unix()
	_, _, _, err := svc.attemptDelivery(webhook, domain.WebhookEventAgentCreated, []byte(signatureTestBody), 5*time.Second)
	require.NoError(t, err)
	after := time.Now().Unix()
	assertSignedWithin(t, lastHeaders(), []byte(signatureTestBody), before, after)

	mock.ExpectExec("INSERT INTO webhook_deliveries").WillReturnResult(sqlmock.NewResult(0, 1))
	before = time.Now().Unix()
	_, err = svc.sendWebhookWithResult(webhook, "webhook.test", map[string]string{"event": "webhook.test"})
	require.NoError(t, err)
	after = time.Now().Unix()
	assertSignedWithin(t, lastHeaders(), []byte(`{"event":"webhook.test"}`), before, after)
}

// assertSignedWithin asserts that headers carry a timestamp from before to
// after and a v1 signature of that timestamp and body under the test secret.
func assertSignedWithin(t *testing.T, headers http.Header, body []byte, before, after int64) {
	t.Helper()
	timestamp := headers.Get("X-Webhook-Timestamp")
	require.Regexp(t, `^[0-9]+$`, timestamp)
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, seconds, before)
	assert.LessOrEqual(t, seconds, after)
	assert.Equal(t, receiverSignature(signatureTestSecret, timestamp, body), headers.Get("X-Webhook-Signature"))
}
