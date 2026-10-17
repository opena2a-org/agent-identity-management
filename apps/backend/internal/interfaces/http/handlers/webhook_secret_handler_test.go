package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// webhookSecretApp mounts the webhook routes, rotation included, for the test
// organization's member.
func webhookSecretApp(service WebhookServicer, audit AuditServicer) *fiber.App {
	handler := NewWebhookHandlerWithInterfaces(service, audit)
	app := setupWebhookTestApp(handler)
	app.Post("/webhooks/:id/rotate-secret", handler.RotateWebhookSecret)
	return app
}

func sendWebhookRequest(t *testing.T, app *fiber.App, method, path, body string) (int, string, string) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(raw), resp.Header.Get(fiber.HeaderCacheControl)
}

func storedWebhook(id uuid.UUID) *domain.Webhook {
	return &domain.Webhook{
		ID: id, OrganizationID: webhookTestOrgID, Name: "hook", URL: "https://hooks.example.com/aim",
		Events: []domain.WebhookEvent{domain.WebhookEventAgentCreated}, Secret: strings.Repeat("f", 64), IsActive: true,
	}
}

// The create answer carries the webhook and, this once, its secret, and is
// not stored by a cache. The caller's secret reaches the service.
func TestWebhookHandler_CreateWebhook_AnswersTheSecretOnce(t *testing.T) {
	chosen := strings.Repeat("c", 40)
	var received *application.CreateWebhookRequest
	service := &MockWebhookServicerImpl{
		CreateWebhookFunc: func(_ context.Context, req *application.CreateWebhookRequest, _, _ uuid.UUID) (*domain.Webhook, error) {
			received = req
			webhook := storedWebhook(uuid.New())
			webhook.Secret = req.Secret
			return webhook, nil
		},
	}
	app := webhookSecretApp(service, &MockAuditServiceImpl{})

	status, body, cacheControl := sendWebhookRequest(t, app, fiber.MethodPost, "/webhooks",
		`{"name":"hook","url":"https://hooks.example.com/aim","events":["agent.created"],"secret":"`+chosen+`"}`)
	require.Equal(t, fiber.StatusCreated, status, body)
	require.NotNil(t, received)
	assert.Equal(t, chosen, received.Secret)
	assert.Equal(t, "no-store", cacheControl)

	var answer map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(body), &answer))
	assert.Equal(t, chosen, answer["secret"])
	for _, field := range []string{"id", "organizationId", "name", "url", "events", "isActive", "createdAt"} {
		assert.Contains(t, answer, field)
	}
}

// GET, list and update never carry the secret the stored webhook holds.
func TestWebhookHandler_OnlyCreateCarriesTheSecret(t *testing.T) {
	id := uuid.New()
	service := &MockWebhookServicerImpl{
		GetWebhookFunc: func(context.Context, uuid.UUID) (*domain.Webhook, error) { return storedWebhook(id), nil },
		ListWebhooksFunc: func(context.Context, uuid.UUID) ([]*domain.Webhook, error) {
			return []*domain.Webhook{storedWebhook(id)}, nil
		},
		UpdateWebhookFunc: func(context.Context, uuid.UUID, *application.CreateWebhookRequest) (*domain.Webhook, error) {
			return storedWebhook(id), nil
		},
	}
	app := webhookSecretApp(service, &MockAuditServiceImpl{})
	secret := storedWebhook(id).Secret

	for _, call := range []struct{ method, path, body string }{
		{fiber.MethodGet, "/webhooks/" + id.String(), ""},
		{fiber.MethodGet, "/webhooks", ""},
		{fiber.MethodPut, "/webhooks/" + id.String(), `{"name":"hook","url":"https://hooks.example.com/aim","events":["agent.created"]}`},
	} {
		status, body, _ := sendWebhookRequest(t, app, call.method, call.path, call.body)
		require.Equal(t, fiber.StatusOK, status, body)
		assert.NotContains(t, body, `"secret"`, "%s %s", call.method, call.path)
		assert.NotContains(t, body, secret, "%s %s", call.method, call.path)
	}
}

// A key the create or update body does not have is refused by name, before
// the service is called; an update cannot carry a secret.
func TestWebhookHandler_CreateAndUpdate_RefuseAnUnknownField(t *testing.T) {
	called := false
	service := &MockWebhookServicerImpl{
		CreateWebhookFunc: func(context.Context, *application.CreateWebhookRequest, uuid.UUID, uuid.UUID) (*domain.Webhook, error) {
			called = true
			return storedWebhook(uuid.New()), nil
		},
		GetWebhookFunc: func(_ context.Context, id uuid.UUID) (*domain.Webhook, error) { return storedWebhook(id), nil },
		UpdateWebhookFunc: func(context.Context, uuid.UUID, *application.CreateWebhookRequest) (*domain.Webhook, error) {
			called = true
			return storedWebhook(uuid.New()), nil
		},
	}
	app := webhookSecretApp(service, &MockAuditServiceImpl{})
	base := `{"name":"hook","url":"https://hooks.example.com/aim","events":["agent.created"]`
	update := "/webhooks/" + uuid.NewString()

	cases := []struct{ method, path, body, answer string }{
		{fiber.MethodPost, "/webhooks", base + `,"headers":{"X-Tenant":"a"}}`, `{"error":"unknown field: headers"}`},
		{fiber.MethodPost, "/webhooks", base + `,"retries":5,"description":"d"}`, `{"error":"unknown fields: description, retries"}`},
		{fiber.MethodPost, "/webhooks", base + `,"Secret":"` + strings.Repeat("c", 40) + `"}`, `{"error":"unknown field: Secret"}`},
		{fiber.MethodPut, update, base + `,"headers":{"X-Tenant":"a"}}`, `{"error":"unknown field: headers"}`},
		{fiber.MethodPut, update, base + `,"secret":"` + strings.Repeat("c", 40) + `"}`, `{"error":"unknown field: secret"}`},
		{fiber.MethodPost, "/webhooks", `null`, `{"error":"Invalid request body"}`},
		{fiber.MethodPost, "/webhooks", `["agent.created"]`, `{"error":"Invalid request body"}`},
		{fiber.MethodPost, "/webhooks", `{"name":1}`, `{"error":"Invalid request body"}`},
	}
	for _, tc := range cases {
		status, body, _ := sendWebhookRequest(t, app, tc.method, tc.path, tc.body)
		assert.Equal(t, fiber.StatusBadRequest, status, "%s %s", tc.method, tc.body)
		assert.JSONEq(t, tc.answer, body, "%s %s", tc.method, tc.body)
	}
	assert.False(t, called, "a refused body reaches no create or update")
}

// The keys a create takes are the JSON names of CreateWebhookRequest; an
// update takes the same without secret.
func TestWebhookHandler_BodyFieldsAreTheRequestStructs(t *testing.T) {
	assert.Equal(t, map[string]bool{"name": true, "url": true, "events": true, "isActive": true, "secret": true}, webhookCreateFields)
	assert.Equal(t, map[string]bool{"name": true, "url": true, "events": true, "isActive": true}, webhookUpdateFields)
}

// An event or secret the service refuses is answered with 400 and the
// service's reason.
func TestWebhookHandler_CreateWebhook_AnswersARefusedRequestWithItsReason(t *testing.T) {
	for _, reason := range []string{"unsupported event: auth.failed", "secret must be at least 32 characters"} {
		service := &MockWebhookServicerImpl{
			CreateWebhookFunc: func(context.Context, *application.CreateWebhookRequest, uuid.UUID, uuid.UUID) (*domain.Webhook, error) {
				return nil, &application.WebhookRequestError{Reason: reason}
			},
		}
		app := webhookSecretApp(service, &MockAuditServiceImpl{})
		status, body, _ := sendWebhookRequest(t, app, fiber.MethodPost, "/webhooks",
			`{"name":"hook","url":"https://hooks.example.com/aim","events":["agent.created"]}`)
		assert.Equal(t, fiber.StatusBadRequest, status, body)
		assert.JSONEq(t, `{"error":"`+reason+`"}`, body)
	}
}

// Rotation answers the webhook's id and new secret once, uncached, and writes
// an update audit row that says the secret was rotated and does not carry it.
func TestWebhookHandler_RotateWebhookSecret_AnswersTheNewSecretOnce(t *testing.T) {
	id := uuid.New()
	newSecret := strings.Repeat("n", 64)
	var rotatedFor uuid.UUID
	service := &MockWebhookServicerImpl{
		GetWebhookFunc: func(_ context.Context, got uuid.UUID) (*domain.Webhook, error) { return storedWebhook(got), nil },
		RotateWebhookSecretFunc: func(_ context.Context, got, org uuid.UUID) (string, error) {
			rotatedFor = got
			assert.Equal(t, webhookTestOrgID, org)
			return newSecret, nil
		},
	}
	var audited []map[string]interface{}
	audit := &MockAuditServiceImpl{
		LogActionFunc: func(_ context.Context, orgID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID,
			_, _ string, metadata map[string]interface{}) error {
			assert.Equal(t, webhookTestOrgID, orgID)
			assert.Equal(t, webhookTestUserID, userID)
			assert.Equal(t, domain.AuditActionUpdate, action)
			assert.Equal(t, "webhook", resourceType)
			assert.Equal(t, id, resourceID)
			audited = append(audited, metadata)
			return nil
		},
	}
	app := webhookSecretApp(service, audit)

	status, body, cacheControl := sendWebhookRequest(t, app, fiber.MethodPost, "/webhooks/"+id.String()+"/rotate-secret", "")
	require.Equal(t, fiber.StatusOK, status, body)
	assert.JSONEq(t, `{"id":"`+id.String()+`","secret":"`+newSecret+`"}`, body)
	assert.Equal(t, "no-store", cacheControl)
	assert.Equal(t, id, rotatedFor)
	require.Len(t, audited, 1)
	assert.Equal(t, true, audited[0]["rotatedSecret"])
	raw, err := json.Marshal(audited[0])
	require.NoError(t, err)
	assert.NotContains(t, string(raw), newSecret)
}

// Rotation applies the update route's checks: a bad id is 400, a webhook
// that is not found is 404, another organization's is 403, and none of them
// rotates anything or writes an audit row.
func TestWebhookHandler_RotateWebhookSecret_RefusesWhatUpdateRefuses(t *testing.T) {
	otherOrg := storedWebhook(uuid.New())
	otherOrg.OrganizationID = uuid.New()
	cases := []struct {
		name   string
		path   string
		get    func(context.Context, uuid.UUID) (*domain.Webhook, error)
		rotate error
		status int
		answer string
	}{
		{"bad id", "/webhooks/not-a-uuid/rotate-secret", nil, nil, fiber.StatusBadRequest, `{"error":"Invalid webhook ID"}`},
		{"not found", "/webhooks/" + uuid.NewString() + "/rotate-secret",
			func(context.Context, uuid.UUID) (*domain.Webhook, error) { return nil, sql.ErrNoRows }, nil,
			fiber.StatusNotFound, `{"error":"Webhook not found"}`},
		{"another organization's", "/webhooks/" + otherOrg.ID.String() + "/rotate-secret",
			func(context.Context, uuid.UUID) (*domain.Webhook, error) { return otherOrg, nil }, nil,
			fiber.StatusForbidden, `{"error":"Access denied"}`},
		{"deleted before the write", "/webhooks/" + uuid.NewString() + "/rotate-secret",
			func(_ context.Context, id uuid.UUID) (*domain.Webhook, error) { return storedWebhook(id), nil }, sql.ErrNoRows,
			fiber.StatusNotFound, `{"error":"Webhook not found"}`},
	}
	for _, tc := range cases {
		rotated, audited := false, false
		service := &MockWebhookServicerImpl{
			GetWebhookFunc: tc.get,
			RotateWebhookSecretFunc: func(context.Context, uuid.UUID, uuid.UUID) (string, error) {
				rotated = true
				return "", tc.rotate
			},
		}
		audit := &MockAuditServiceImpl{
			LogActionFunc: func(context.Context, uuid.UUID, uuid.UUID, domain.AuditAction, string, uuid.UUID, string, string, map[string]interface{}) error {
				audited = true
				return nil
			},
		}
		status, body, _ := sendWebhookRequest(t, webhookSecretApp(service, audit), fiber.MethodPost, tc.path, "")
		assert.Equal(t, tc.status, status, tc.name)
		assert.JSONEq(t, tc.answer, body, tc.name)
		assert.Equal(t, tc.rotate != nil, rotated, tc.name)
		assert.False(t, audited, tc.name)
	}
}
