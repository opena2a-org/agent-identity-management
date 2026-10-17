package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// fakeAlertChannelService answers as the service would for the channels it
// holds, and records the requests it is given.
type fakeAlertChannelService struct {
	channels  map[uuid.UUID]*domain.AlertChannel
	createErr error
	requests  []*application.AlertChannelRequest
	test      *application.AlertChannelTestResult
	deleted   []uuid.UUID
}

func (f *fakeAlertChannelService) CreateAlertChannel(_ context.Context, orgID, _ uuid.UUID, req *application.AlertChannelRequest) (*domain.AlertChannel, error) {
	f.requests = append(f.requests, req)
	if f.createErr != nil {
		return nil, f.createErr
	}
	channel := &domain.AlertChannel{ID: uuid.New(), OrganizationID: orgID, Name: *req.Name, Kind: domain.AlertChannelKind(*req.Kind),
		MinSeverity: domain.AlertSeverity(*req.MinSeverity), IsActive: true, Config: map[string]string{}}
	var config map[string]string
	_ = json.Unmarshal(req.Config, &config)
	for k, v := range config {
		channel.Config[k] = v
	}
	f.channels[channel.ID] = channel
	return channel, nil
}

func (f *fakeAlertChannelService) GetAlertChannel(_ context.Context, orgID, id uuid.UUID) (*domain.AlertChannel, error) {
	c, ok := f.channels[id]
	if !ok || c.OrganizationID != orgID {
		return nil, domain.ErrAlertChannelNotFound
	}
	return c, nil
}

func (f *fakeAlertChannelService) ListAlertChannels(_ context.Context, orgID uuid.UUID) ([]*domain.AlertChannel, error) {
	var out []*domain.AlertChannel
	for _, c := range f.channels {
		if c.OrganizationID == orgID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeAlertChannelService) UpdateAlertChannel(ctx context.Context, orgID, id uuid.UUID, req *application.AlertChannelRequest) (*domain.AlertChannel, error) {
	f.requests = append(f.requests, req)
	c, err := f.GetAlertChannel(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if req.MinSeverity != nil {
		c.MinSeverity = domain.AlertSeverity(*req.MinSeverity)
	}
	return c, nil
}

func (f *fakeAlertChannelService) DeleteAlertChannel(_ context.Context, _ uuid.UUID, id uuid.UUID) error {
	f.deleted = append(f.deleted, id)
	delete(f.channels, id)
	return nil
}

func (f *fakeAlertChannelService) SendTestMessage(context.Context, uuid.UUID, uuid.UUID) (*application.AlertChannelTestResult, error) {
	return f.test, nil
}

// alertChannelAuditRow is one audit row the handler wrote.
type alertChannelAuditRow struct {
	action       domain.AuditAction
	resourceType string
	resourceID   uuid.UUID
	metadata     map[string]interface{}
}

type alertChannelAudit struct {
	AuditServicer
	rows *[]alertChannelAuditRow
}

func (a alertChannelAudit) LogAction(_ context.Context, _, _ uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, _, _ string, metadata map[string]interface{}) error {
	*a.rows = append(*a.rows, alertChannelAuditRow{action, resourceType, resourceID, metadata})
	return nil
}

type alertChannelRig struct {
	app     *fiber.App
	service *fakeAlertChannelService
	audit   *[]alertChannelAuditRow
	org     uuid.UUID
}

func newAlertChannelRig(t *testing.T) *alertChannelRig {
	t.Helper()
	rig := &alertChannelRig{
		service: &fakeAlertChannelService{channels: map[uuid.UUID]*domain.AlertChannel{}},
		audit:   &[]alertChannelAuditRow{},
		org:     uuid.New(),
	}
	h := NewAlertChannelHandler(rig.service, alertChannelAudit{rows: rig.audit}, nil)
	rig.app = fiber.New()
	rig.app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", rig.org)
		c.Locals("user_id", uuid.New())
		return c.Next()
	})
	rig.app.Post("/alert-channels", h.CreateAlertChannel)
	rig.app.Get("/alert-channels", h.ListAlertChannels)
	rig.app.Get("/alert-channels/:id", h.GetAlertChannel)
	rig.app.Put("/alert-channels/:id", h.UpdateAlertChannel)
	rig.app.Delete("/alert-channels/:id", h.DeleteAlertChannel)
	rig.app.Post("/alert-channels/:id/test", h.TestAlertChannel)
	return rig
}

func (r *alertChannelRig) do(t *testing.T, method, path, body string) (int, string) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(raw)
}

func (r *alertChannelRig) seed(org uuid.UUID, kind domain.AlertChannelKind, config map[string]string) *domain.AlertChannel {
	c := &domain.AlertChannel{ID: uuid.New(), OrganizationID: org, Name: "seeded", Kind: kind, Config: config,
		MinSeverity: domain.AlertSeverityWarning, IsActive: true}
	r.service.channels[c.ID] = c
	return c
}

// The secret test values are built from fragments.
func alertChannelTestURL() string {
	return "https://" + "hooks.slack.com" + "/services/" + "handler-" + uuid.NewString()
}

func TestAlertChannelHandler_CreateAnswersTheMaskedChannelAndAudits(t *testing.T) {
	rig := newAlertChannelRig(t)
	url := alertChannelTestURL()

	status, body := rig.do(t, fiber.MethodPost, "/alert-channels",
		`{"name":"ops","kind":"slack","config":{"webhookUrl":"`+url+`"},"minSeverity":"high"}`)
	require.Equal(t, fiber.StatusCreated, status, body)
	assert.NotContains(t, body, url)
	var view map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(body), &view))
	assert.Equal(t, map[string]interface{}{"webhookUrl": "****" + url[len(url)-4:]}, view["config"])
	for _, field := range []string{"id", "name", "kind", "minSeverity", "isActive", "config", "lastDeliveryStatus", "lastDeliveryCode", "lastDeliveryAt", "createdAt", "updatedAt"} {
		assert.Contains(t, view, field)
	}
	assert.Len(t, view, 11, "the channel object carries its eleven fields and nothing else: %s", body)

	require.Len(t, *rig.audit, 1)
	row := (*rig.audit)[0]
	assert.Equal(t, domain.AuditActionCreate, row.action)
	assert.Equal(t, "alert_channel", row.resourceType)
	assert.Equal(t, view["id"], row.resourceID.String())
	assert.Equal(t, map[string]interface{}{"kind": "slack", "minSeverity": "high"}, row.metadata)
}

func TestAlertChannelHandler_RefusesAnUnknownField(t *testing.T) {
	rig := newAlertChannelRig(t)

	status, body := rig.do(t, fiber.MethodPost, "/alert-channels",
		`{"name":"ops","kind":"slack","config":{},"minSeverity":"high","color":"red"}`)
	assert.Equal(t, fiber.StatusBadRequest, status)
	assert.JSONEq(t, `{"error":"unknown field: color"}`, body)

	status, body = rig.do(t, fiber.MethodPost, "/alert-channels", `not json`)
	assert.Equal(t, fiber.StatusBadRequest, status)
	assert.JSONEq(t, `{"error":"Invalid request body"}`, body)

	status, body = rig.do(t, fiber.MethodPost, "/alert-channels", `{"name":"ops"} {"name":"again"}`)
	assert.Equal(t, fiber.StatusBadRequest, status, "a second value after the body is refused")
	assert.JSONEq(t, `{"error":"Invalid request body"}`, body)

	c := rig.seed(rig.org, domain.AlertChannelSlack, map[string]string{"webhookUrl": alertChannelTestURL()})
	status, body = rig.do(t, fiber.MethodPut, "/alert-channels/"+c.ID.String(), `{"secret":"x"}`)
	assert.Equal(t, fiber.StatusBadRequest, status)
	assert.JSONEq(t, `{"error":"unknown field: secret"}`, body)

	assert.Empty(t, rig.service.requests, "a refused body reaches no service")
	assert.Empty(t, *rig.audit)
}

func TestAlertChannelHandler_ServiceRefusalsAreAnsweredByName(t *testing.T) {
	rig := newAlertChannelRig(t)
	valid := `{"name":"ops","kind":"slack","config":{},"minSeverity":"high"}`

	rig.service.createErr = &application.AlertChannelRequestError{Reason: "unsupported kind: sms"}
	status, body := rig.do(t, fiber.MethodPost, "/alert-channels", valid)
	assert.Equal(t, fiber.StatusBadRequest, status)
	assert.JSONEq(t, `{"error":"unsupported kind: sms"}`, body)

	rig.service.createErr = domain.ErrAlertChannelNameTaken
	status, body = rig.do(t, fiber.MethodPost, "/alert-channels", valid)
	assert.Equal(t, fiber.StatusConflict, status, body)

	assert.Empty(t, *rig.audit, "a refused create writes no audit row")
}

// A channel of another organization answers 404 on every route that names
// it, the same as a channel that does not exist.
func TestAlertChannelHandler_AnotherOrganizationsChannelIsNotFound(t *testing.T) {
	rig := newAlertChannelRig(t)
	other := rig.seed(uuid.New(), domain.AlertChannelSlack, map[string]string{"webhookUrl": alertChannelTestURL()})

	for _, req := range [][3]string{
		{fiber.MethodGet, "/alert-channels/" + other.ID.String(), ""},
		{fiber.MethodPut, "/alert-channels/" + other.ID.String(), `{"minSeverity":"info"}`},
		{fiber.MethodDelete, "/alert-channels/" + other.ID.String(), ""},
		{fiber.MethodPost, "/alert-channels/" + other.ID.String() + "/test", ""},
		{fiber.MethodGet, "/alert-channels/" + uuid.NewString(), ""},
	} {
		status, body := rig.do(t, req[0], req[1], req[2])
		assert.Equal(t, fiber.StatusNotFound, status, "%s %s: %s", req[0], req[1], body)
	}
	assert.Empty(t, rig.service.deleted)
	assert.Empty(t, rig.service.requests)

	status, body := rig.do(t, fiber.MethodGet, "/alert-channels/not-a-uuid", "")
	assert.Equal(t, fiber.StatusBadRequest, status, body)

	status, body = rig.do(t, fiber.MethodGet, "/alert-channels", "")
	require.Equal(t, fiber.StatusOK, status)
	assert.JSONEq(t, `{"alertChannels":[],"total":0}`, body)
}

func TestAlertChannelHandler_ListAndGetMaskTheDestination(t *testing.T) {
	rig := newAlertChannelRig(t)
	key := strings.ReplaceAll(uuid.NewString(), "-", "")
	c := rig.seed(rig.org, domain.AlertChannelPagerDuty, map[string]string{"routingKey": key})

	for _, path := range []string{"/alert-channels", "/alert-channels/" + c.ID.String()} {
		status, body := rig.do(t, fiber.MethodGet, path, "")
		require.Equal(t, fiber.StatusOK, status, body)
		assert.NotContains(t, body, key, path)
		assert.Contains(t, body, `"routingKey":"****`+key[28:]+`"`, path)
	}
}

func TestAlertChannelHandler_UpdateAndDeleteAudit(t *testing.T) {
	rig := newAlertChannelRig(t)
	c := rig.seed(rig.org, domain.AlertChannelSlack, map[string]string{"webhookUrl": alertChannelTestURL()})

	status, body := rig.do(t, fiber.MethodPut, "/alert-channels/"+c.ID.String(), `{"minSeverity":"critical"}`)
	require.Equal(t, fiber.StatusOK, status, body)
	assert.Contains(t, body, `"minSeverity":"critical"`)

	status, body = rig.do(t, fiber.MethodDelete, "/alert-channels/"+c.ID.String(), "")
	require.Equal(t, fiber.StatusNoContent, status, body)
	assert.Equal(t, []uuid.UUID{c.ID}, rig.service.deleted)

	require.Len(t, *rig.audit, 2)
	assert.Equal(t, domain.AuditActionUpdate, (*rig.audit)[0].action)
	assert.Equal(t, map[string]interface{}{"kind": "slack", "minSeverity": "critical"}, (*rig.audit)[0].metadata)
	assert.Equal(t, domain.AuditActionDelete, (*rig.audit)[1].action)
	assert.Equal(t, c.ID, (*rig.audit)[1].resourceID)
	assert.Equal(t, map[string]interface{}{"kind": "slack", "minSeverity": "critical"}, (*rig.audit)[1].metadata)
}

func TestAlertChannelHandler_TestAnswersTheDelivery(t *testing.T) {
	rig := newAlertChannelRig(t)
	c := rig.seed(rig.org, domain.AlertChannelSlack, map[string]string{"webhookUrl": alertChannelTestURL()})
	code := 200

	rig.service.test = &application.AlertChannelTestResult{Delivered: true, StatusCode: &code}
	status, body := rig.do(t, fiber.MethodPost, "/alert-channels/"+c.ID.String()+"/test", "")
	assert.Equal(t, fiber.StatusOK, status)
	assert.JSONEq(t, `{"delivered":true,"statusCode":200}`, body)

	rig.service.test = &application.AlertChannelTestResult{Delivered: false, Error: "timeout"}
	status, body = rig.do(t, fiber.MethodPost, "/alert-channels/"+c.ID.String()+"/test", "")
	assert.Equal(t, fiber.StatusBadGateway, status)
	assert.JSONEq(t, `{"delivered":false,"statusCode":null,"error":"timeout"}`, body)

	code = 404
	rig.service.test = &application.AlertChannelTestResult{Delivered: false, StatusCode: &code, Error: "status 404"}
	status, body = rig.do(t, fiber.MethodPost, "/alert-channels/"+c.ID.String()+"/test", "")
	assert.Equal(t, fiber.StatusBadGateway, status)
	assert.JSONEq(t, `{"delivered":false,"statusCode":404,"error":"status 404"}`, body)
	assert.NotContains(t, body, c.Config["webhookUrl"])
}

// memoryAlertChannelStore holds channels for the real alert channel service.
type memoryAlertChannelStore struct {
	channels map[uuid.UUID]*domain.AlertChannel
}

func (m *memoryAlertChannelStore) Create(_ context.Context, c *domain.AlertChannel) error {
	m.channels[c.ID] = c
	return nil
}

func (m *memoryAlertChannelStore) GetByID(_ context.Context, orgID, id uuid.UUID) (*domain.AlertChannel, error) {
	c, ok := m.channels[id]
	if !ok || c.OrganizationID != orgID {
		return nil, domain.ErrAlertChannelNotFound
	}
	copied := *c
	return &copied, nil
}

func (m *memoryAlertChannelStore) List(_ context.Context, orgID uuid.UUID) ([]*domain.AlertChannel, error) {
	out := []*domain.AlertChannel{}
	for _, c := range m.channels {
		if c.OrganizationID == orgID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *memoryAlertChannelStore) ListActiveForFloors(context.Context, uuid.UUID, []domain.AlertSeverity) ([]*domain.AlertChannel, error) {
	return nil, nil
}

func (m *memoryAlertChannelStore) Update(_ context.Context, c *domain.AlertChannel) error {
	m.channels[c.ID] = c
	return nil
}

func (m *memoryAlertChannelStore) Delete(_ context.Context, _, id uuid.UUID) error {
	delete(m.channels, id)
	return nil
}

func (m *memoryAlertChannelStore) RecordDelivery(context.Context, uuid.UUID, uuid.UUID, string, *int, time.Time) error {
	return nil
}

// On a server with no email service, the real service behind the handler
// answers an email channel's create with 400 and its test message with 502
// email_unavailable; an email channel's recipients are answered unmasked.
func TestAlertChannelHandler_EmailChannelWithoutEmailService(t *testing.T) {
	org := uuid.New()
	store := &memoryAlertChannelStore{channels: map[uuid.UUID]*domain.AlertChannel{}}
	existing := &domain.AlertChannel{ID: uuid.New(), OrganizationID: org, Name: "mail", Kind: domain.AlertChannelEmail,
		Config: map[string]string{}, Recipients: []string{"ops@example.com", "sec@example.com"},
		MinSeverity: domain.AlertSeverityWarning, IsActive: true}
	store.channels[existing.ID] = existing
	audit := &[]alertChannelAuditRow{}
	h := NewAlertChannelHandler(application.NewAlertChannelService(store, "https://aim.example.test", nil), alertChannelAudit{rows: audit}, nil)
	rig := &alertChannelRig{app: fiber.New(), audit: audit, org: org}
	rig.app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", org)
		c.Locals("user_id", uuid.New())
		return c.Next()
	})
	rig.app.Post("/alert-channels", h.CreateAlertChannel)
	rig.app.Get("/alert-channels/:id", h.GetAlertChannel)
	rig.app.Post("/alert-channels/:id/test", h.TestAlertChannel)

	status, body := rig.do(t, fiber.MethodPost, "/alert-channels",
		`{"name":"new","kind":"email","config":{"recipients":["ops@example.com"]},"minSeverity":"info"}`)
	assert.Equal(t, fiber.StatusBadRequest, status)
	assert.JSONEq(t, `{"error":"email is not configured on this server"}`, body)
	assert.Empty(t, *audit, "a refused create writes no audit row")

	status, body = rig.do(t, fiber.MethodPost, "/alert-channels/"+existing.ID.String()+"/test", "")
	assert.Equal(t, fiber.StatusBadGateway, status)
	assert.JSONEq(t, `{"delivered":false,"statusCode":null,"error":"email_unavailable"}`, body)

	status, body = rig.do(t, fiber.MethodGet, "/alert-channels/"+existing.ID.String(), "")
	require.Equal(t, fiber.StatusOK, status, body)
	var view map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(body), &view))
	assert.Equal(t, map[string]interface{}{"recipients": []interface{}{"ops@example.com", "sec@example.com"}}, view["config"])
}
