package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/utils"
)

// AlertChannelStore is what AlertChannelService needs of the alert channel
// repository. Every method names the organization.
type AlertChannelStore interface {
	Create(ctx context.Context, channel *domain.AlertChannel) error
	GetByID(ctx context.Context, orgID, id uuid.UUID) (*domain.AlertChannel, error)
	List(ctx context.Context, orgID uuid.UUID) ([]*domain.AlertChannel, error)
	ListActiveForFloors(ctx context.Context, orgID uuid.UUID, floors []domain.AlertSeverity) ([]*domain.AlertChannel, error)
	Update(ctx context.Context, channel *domain.AlertChannel) error
	Delete(ctx context.Context, orgID, id uuid.UUID) error
	RecordDelivery(ctx context.Context, orgID, id uuid.UUID, status string, code *int, at time.Time) error
}

const (
	// AlertChannelQueueCapacity is how many alerts wait for their channel
	// deliveries. An alert that arrives while the queue is full is sent to no
	// channel; its row is unaffected.
	AlertChannelQueueCapacity = 1024
	// alertChannelWorkers take alerts off the queue.
	alertChannelWorkers = 4
	// alertChannelTimeout bounds one attempt.
	alertChannelTimeout = 10 * time.Second
	// maxAlertChannelNameLength is the most characters a channel name has.
	maxAlertChannelNameLength = 100
	// alertChannelResponseLimit is how much of an answer an attempt reads
	// before it closes the connection.
	alertChannelResponseLimit = 4096

	// slackWebhookPrefix starts every Slack incoming webhook URL.
	slackWebhookPrefix = "https://hooks.slack.com/services/"
	// PagerDutyEventsURL is where every PagerDuty event is sent. It is not
	// configurable: a channel names only its routing key.
	PagerDutyEventsURL = "https://events.pagerduty.com/v2/enqueue"
	// pagerDutyRoutingKeyLength is the length of a PagerDuty integration key.
	pagerDutyRoutingKeyLength = 32
	// pagerDutySummaryLimit is the most characters payload.summary takes.
	pagerDutySummaryLimit = 1024
)

// defaultAlertChannelRetryDelays are the waits before the second, third and
// fourth attempt of a delivery.
var defaultAlertChannelRetryDelays = []time.Duration{1 * time.Second, 4 * time.Second, 16 * time.Second}

// AlertChannelService stores an organization's alert channels and sends each
// alert the alert repository inserts to the channels whose floor admits its
// severity, in each service's published format.
type AlertChannelService struct {
	store       AlertChannelStore
	frontendURL string

	// newHTTPClient builds the client a delivery is sent with: the egress
	// client, which follows no redirect and checks the address of every
	// connection it opens.
	newHTTPClient func(timeout time.Duration) *http.Client
	// retryDelays are the waits before each retry; a delivery makes at most
	// len(retryDelays)+1 attempts.
	retryDelays []time.Duration
	// checkURL checks a Slack webhook URL the way a webhook URL is checked.
	checkURL func(rawURL string) error

	queue     chan *domain.Alert
	stop      chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once
	workers   sync.WaitGroup
}

// NewAlertChannelService returns the service on store. frontendURL is the
// dashboard's address (FRONTEND_URL): Slack messages link to its alerts page,
// and a PagerDuty event for an alert that names no agent gives its host as the
// source. Start begins delivery.
func NewAlertChannelService(store AlertChannelStore, frontendURL string) *AlertChannelService {
	return &AlertChannelService{
		store:         store,
		frontendURL:   frontendURL,
		newHTTPClient: utils.NewEgressClient,
		retryDelays:   defaultAlertChannelRetryDelays,
		checkURL:      checkWebhookURL,
		queue:         make(chan *domain.Alert, AlertChannelQueueCapacity),
		stop:          make(chan struct{}),
	}
}

// AlertChannelRequest is the body of an alert channel create or update. A
// create needs every field but isActive, which defaults to true. On an update
// every field is optional: an omitted config keeps the stored destination,
// and a config replaces it whole.
type AlertChannelRequest struct {
	Name        *string         `json:"name"`
	Kind        *string         `json:"kind"`
	Config      json.RawMessage `json:"config"`
	MinSeverity *string         `json:"minSeverity"`
	IsActive    *bool           `json:"isActive"`
}

// AlertChannelRequestError is returned for a create or update the server
// refuses for what it asks. Reason is said to the caller and names the field.
type AlertChannelRequestError struct {
	Reason string
}

func (e *AlertChannelRequestError) Error() string { return e.Reason }

func refuseAlertChannel(format string, args ...interface{}) error {
	return &AlertChannelRequestError{Reason: fmt.Sprintf(format, args...)}
}

// UnknownJSONField returns the name in a json.Decoder's refusal of a field
// its target does not have.
func UnknownJSONField(err error) (string, bool) {
	name, found := strings.CutPrefix(err.Error(), `json: unknown field "`)
	if !found {
		return "", false
	}
	return strings.TrimSuffix(name, `"`), true
}

func checkAlertChannelName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", refuseAlertChannel("name is required")
	}
	if utf8.RuneCountInString(name) > maxAlertChannelNameLength {
		return "", refuseAlertChannel("name must be at most %d characters", maxAlertChannelNameLength)
	}
	return name, nil
}

func checkAlertChannelKind(kind string) (domain.AlertChannelKind, error) {
	switch k := domain.AlertChannelKind(kind); k {
	case domain.AlertChannelSlack, domain.AlertChannelPagerDuty:
		return k, nil
	}
	return "", refuseAlertChannel("unsupported kind: %s", kind)
}

func checkAlertChannelFloor(floor string) (domain.AlertSeverity, error) {
	severity := domain.AlertSeverity(floor)
	if _, ok := domain.AlertSeverityRank(severity); !ok {
		return "", refuseAlertChannel("minSeverity must be one of info, warning, high, critical")
	}
	return severity, nil
}

// checkConfig decodes and checks a config for kind and returns the values to
// store. A key the kind does not take is refused by name.
func (s *AlertChannelService) checkConfig(kind domain.AlertChannelKind, raw json.RawMessage) (map[string]string, error) {
	if trimmed := bytes.TrimSpace(raw); len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, refuseAlertChannel("config must be an object")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var config struct {
		WebhookURL *string `json:"webhookUrl,omitempty"`
		RoutingKey *string `json:"routingKey,omitempty"`
	}
	if err := dec.Decode(&config); err != nil {
		if name, ok := UnknownJSONField(err); ok {
			return nil, refuseAlertChannel("unknown field: config.%s", name)
		}
		return nil, refuseAlertChannel("config must be an object of string values")
	}

	switch kind {
	case domain.AlertChannelSlack:
		if config.RoutingKey != nil {
			return nil, refuseAlertChannel("unknown field: config.%s", domain.AlertChannelConfigRoutingKey)
		}
		if config.WebhookURL == nil {
			return nil, refuseAlertChannel("config.%s is required", domain.AlertChannelConfigWebhookURL)
		}
		if err := s.checkSlackWebhookURL(*config.WebhookURL); err != nil {
			return nil, err
		}
		return map[string]string{domain.AlertChannelConfigWebhookURL: *config.WebhookURL}, nil
	default: // domain.AlertChannelPagerDuty
		if config.WebhookURL != nil {
			return nil, refuseAlertChannel("unknown field: config.%s", domain.AlertChannelConfigWebhookURL)
		}
		if config.RoutingKey == nil {
			return nil, refuseAlertChannel("config.%s is required", domain.AlertChannelConfigRoutingKey)
		}
		if utf8.RuneCountInString(*config.RoutingKey) != pagerDutyRoutingKeyLength {
			return nil, refuseAlertChannel("config.%s must be the %d-character integration key", domain.AlertChannelConfigRoutingKey, pagerDutyRoutingKeyLength)
		}
		return map[string]string{domain.AlertChannelConfigRoutingKey: *config.RoutingKey}, nil
	}
}

// checkSlackWebhookURL accepts https://hooks.slack.com/services/ followed by
// a path, that checkWebhookURL also accepts. A refusal names the field and
// never the URL. A host lookup that failed for another reason than the name
// not existing is returned as it is, not as a refusal.
func (s *AlertChannelService) checkSlackWebhookURL(raw string) error {
	refused := refuseAlertChannel("config.%s must be a Slack incoming webhook URL: %s followed by the webhook's path",
		domain.AlertChannelConfigWebhookURL, slackWebhookPrefix)
	if !strings.HasPrefix(raw, slackWebhookPrefix) || len(raw) == len(slackWebhookPrefix) {
		return refused
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "hooks.slack.com" || u.User != nil ||
		!strings.HasPrefix(u.Path, "/services/") || len(u.Path) == len("/services/") {
		return refused
	}
	if err := s.checkURL(raw); err != nil {
		if errors.Is(err, ErrWebhookURLRefused) {
			return refused
		}
		return err
	}
	return nil
}

// CreateAlertChannel checks req and stores the organization's new channel. A
// name another channel of the organization has is
// domain.ErrAlertChannelNameTaken.
func (s *AlertChannelService) CreateAlertChannel(ctx context.Context, orgID, userID uuid.UUID, req *AlertChannelRequest) (*domain.AlertChannel, error) {
	if req.Name == nil {
		return nil, refuseAlertChannel("name is required")
	}
	name, err := checkAlertChannelName(*req.Name)
	if err != nil {
		return nil, err
	}
	if req.Kind == nil {
		return nil, refuseAlertChannel("kind is required: slack or pagerduty")
	}
	kind, err := checkAlertChannelKind(*req.Kind)
	if err != nil {
		return nil, err
	}
	if req.MinSeverity == nil {
		return nil, refuseAlertChannel("minSeverity must be one of info, warning, high, critical")
	}
	floor, err := checkAlertChannelFloor(*req.MinSeverity)
	if err != nil {
		return nil, err
	}
	if len(req.Config) == 0 {
		return nil, refuseAlertChannel("config is required")
	}
	config, err := s.checkConfig(kind, req.Config)
	if err != nil {
		return nil, err
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	channel := &domain.AlertChannel{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           name,
		Kind:           kind,
		Config:         config,
		MinSeverity:    floor,
		IsActive:       active,
		CreatedBy:      &userID,
	}
	if err := s.store.Create(ctx, channel); err != nil {
		return nil, err
	}
	return channel, nil
}

// GetAlertChannel returns the organization's channel id, or
// domain.ErrAlertChannelNotFound.
func (s *AlertChannelService) GetAlertChannel(ctx context.Context, orgID, id uuid.UUID) (*domain.AlertChannel, error) {
	return s.store.GetByID(ctx, orgID, id)
}

// ListAlertChannels returns the organization's channels.
func (s *AlertChannelService) ListAlertChannels(ctx context.Context, orgID uuid.UUID) ([]*domain.AlertChannel, error) {
	return s.store.List(ctx, orgID)
}

// UpdateAlertChannel applies the fields req carries to the organization's
// channel id. A config replaces the stored one whole and is checked as on
// create; a kind change needs one.
func (s *AlertChannelService) UpdateAlertChannel(ctx context.Context, orgID, id uuid.UUID, req *AlertChannelRequest) (*domain.AlertChannel, error) {
	channel, err := s.store.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		if channel.Name, err = checkAlertChannelName(*req.Name); err != nil {
			return nil, err
		}
	}
	kindChanged := false
	if req.Kind != nil {
		kind, err := checkAlertChannelKind(*req.Kind)
		if err != nil {
			return nil, err
		}
		kindChanged = kind != channel.Kind
		channel.Kind = kind
	}
	if req.MinSeverity != nil {
		if channel.MinSeverity, err = checkAlertChannelFloor(*req.MinSeverity); err != nil {
			return nil, err
		}
	}
	switch {
	case len(req.Config) > 0:
		if channel.Config, err = s.checkConfig(channel.Kind, req.Config); err != nil {
			return nil, err
		}
	case kindChanged:
		return nil, refuseAlertChannel("config is required when kind changes")
	}
	if req.IsActive != nil {
		channel.IsActive = *req.IsActive
	}
	if err := s.store.Update(ctx, channel); err != nil {
		return nil, err
	}
	return channel, nil
}

// DeleteAlertChannel removes the organization's channel id.
func (s *AlertChannelService) DeleteAlertChannel(ctx context.Context, orgID, id uuid.UUID) error {
	return s.store.Delete(ctx, orgID, id)
}

// AlertChannelTestResult is the outcome of a test message. StatusCode is nil
// when no answer came; Error is the failure's class and never the
// destination: "status <code>", "timeout", "refused address" or
// "unreachable".
type AlertChannelTestResult struct {
	Delivered  bool
	StatusCode *int
	Error      string
}

// SendTestMessage sends one test message through the organization's channel
// id at once, with no retry, and records the delivery. A PagerDuty test
// triggers an incident on the PagerDuty service.
func (s *AlertChannelService) SendTestMessage(ctx context.Context, orgID, id uuid.UUID) (*AlertChannelTestResult, error) {
	channel, err := s.store.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	destination, body, err := s.testMessage(channel)
	if err != nil {
		return nil, err
	}
	outcome := s.send(destination, body, 1)
	s.record(channel, outcome)
	return &AlertChannelTestResult{Delivered: outcome.delivered, StatusCode: outcome.code, Error: outcome.class}, nil
}

// Start begins delivery: alertChannelWorkers workers take alerts off the
// queue. It may be called more than once.
func (s *AlertChannelService) Start() {
	s.startOnce.Do(func() {
		for i := 0; i < alertChannelWorkers; i++ {
			s.workers.Add(1)
			go s.work()
		}
	})
}

// Stop ends delivery: the workers take no further alert, a wait before a
// retry ends, and Stop returns once each attempt in flight has its answer or
// its timeout. Alerts still queued are not sent. It may be called more than
// once.
func (s *AlertChannelService) Stop() {
	s.stopOnce.Do(func() { close(s.stop) })
	s.workers.Wait()
}

// Enqueue is the alert repository's observer. It never blocks: it puts a copy
// of alert on the queue, or, when the queue is full, logs that the alert's
// channel deliveries are dropped.
func (s *AlertChannelService) Enqueue(alert *domain.Alert) {
	if alert == nil {
		return
	}
	queued := *alert
	select {
	case s.queue <- &queued:
	default:
		log.Printf("alert %s: alert channel queue full (%d alerts waiting); the alert is sent to no alert channel", alert.ID, cap(s.queue))
	}
}

func (s *AlertChannelService) work() {
	defer s.workers.Done()
	for {
		select {
		case <-s.stop:
			return
		case alert := <-s.queue:
			s.deliverAlert(alert)
		}
	}
}

// deliverAlert sends alert to each of its organization's active channels
// whose floor admits its severity, all at once, and returns when every
// delivery has ended.
func (s *AlertChannelService) deliverAlert(alert *domain.Alert) {
	floors := domain.AlertChannelFloorsAdmitting(alert.Severity)
	if len(floors) == 0 {
		log.Printf("alert %s: severity %q is not info, warning, high or critical; the alert is sent to no alert channel", alert.ID, alert.Severity)
		return
	}
	channels, err := s.store.ListActiveForFloors(context.Background(), alert.OrganizationID, floors)
	if err != nil {
		log.Printf("alert %s: reading its organization's alert channels failed: %v", alert.ID, err)
		return
	}
	var deliveries sync.WaitGroup
	for _, channel := range channels {
		deliveries.Add(1)
		go func(channel *domain.AlertChannel) {
			defer deliveries.Done()
			s.deliverToChannel(channel, alert)
		}(channel)
	}
	deliveries.Wait()
}

func (s *AlertChannelService) deliverToChannel(channel *domain.AlertChannel, alert *domain.Alert) {
	destination, body, err := s.alertMessage(channel, alert)
	if err != nil {
		log.Printf("alert channel %s: alert %s not sent: %v", channel.ID, alert.ID, err)
		return
	}
	outcome := s.send(destination, body, len(s.retryDelays)+1)
	if !outcome.delivered {
		log.Printf("alert channel %s (%s): alert %s not delivered: %s", channel.ID, channel.Kind, alert.ID, outcome.class)
	}
	s.record(channel, outcome)
}

// alertChannelOutcome is how a delivery ended. code is the HTTP status of its
// final attempt, nil after a network error; class names a failure.
type alertChannelOutcome struct {
	delivered bool
	code      *int
	class     string
}

// alertChannelAttemptVerdict is what one attempt's answer means.
type alertChannelAttemptVerdict int

const (
	attemptDelivered alertChannelAttemptVerdict = iota
	attemptRetry
	attemptFailed
)

// judgeAlertChannelAttempt says whether an attempt delivered, is worth a
// retry, or failed for good. A 2xx is delivered. A 429, a 5xx and a network
// error are retried. Any other answer, and an address the egress policy
// refuses, are not: the same request would be answered the same way.
func judgeAlertChannelAttempt(code int, err error) alertChannelAttemptVerdict {
	if err != nil {
		var refused *utils.EgressRefusedError
		if errors.As(err, &refused) {
			return attemptFailed
		}
		return attemptRetry
	}
	switch {
	case code >= 200 && code < 300:
		return attemptDelivered
	case code == http.StatusTooManyRequests || code >= 500:
		return attemptRetry
	}
	return attemptFailed
}

// alertChannelFailureClass names a failed attempt without its destination.
func alertChannelFailureClass(code int, err error) string {
	if err == nil {
		return fmt.Sprintf("status %d", code)
	}
	var refused *utils.EgressRefusedError
	if errors.As(err, &refused) {
		return "refused address"
	}
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return "timeout"
	}
	return "unreachable"
}

// send posts body to destination, making up to attempts attempts with the
// retry delays between them.
func (s *AlertChannelService) send(destination string, body []byte, attempts int) alertChannelOutcome {
	client := s.newHTTPClient(alertChannelTimeout)
	for attempt := 1; ; attempt++ {
		code, err := postAlertChannel(client, destination, body)
		verdict := judgeAlertChannelAttempt(code, err)
		if verdict == attemptDelivered {
			return alertChannelOutcome{delivered: true, code: &code}
		}
		outcome := alertChannelOutcome{class: alertChannelFailureClass(code, err)}
		if err == nil {
			outcome.code = &code
		}
		if verdict == attemptFailed || attempt >= attempts || attempt > len(s.retryDelays) {
			return outcome
		}
		timer := time.NewTimer(s.retryDelays[attempt-1])
		select {
		case <-timer.C:
		case <-s.stop:
			timer.Stop()
			return outcome
		}
	}
}

func postAlertChannel(client *http.Client, destination string, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, destination, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "AIM-Alert-Channel/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, alertChannelResponseLimit))
	return resp.StatusCode, nil
}

func (s *AlertChannelService) record(channel *domain.AlertChannel, outcome alertChannelOutcome) {
	status := domain.AlertChannelFailed
	if outcome.delivered {
		status = domain.AlertChannelDelivered
	}
	if err := s.store.RecordDelivery(context.Background(), channel.OrganizationID, channel.ID, status, outcome.code, time.Now().UTC()); err != nil {
		log.Printf("alert channel %s: recording its last delivery failed: %v", channel.ID, err)
	}
}

// slackEscaper escapes Slack's control characters, so text taken from an
// alert is shown as written and cannot mention a channel or a user.
var slackEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// EscapeSlackText returns s with &, < and > written as &amp;, &lt; and &gt;.
func EscapeSlackText(s string) string {
	return slackEscaper.Replace(s)
}

// alertsPageURL is the dashboard's alerts page.
func (s *AlertChannelService) alertsPageURL() string {
	return strings.TrimRight(s.frontendURL, "/") + "/dashboard/admin/alerts"
}

// frontendHost is the host of the dashboard's address, the source of a
// PagerDuty event that names no agent.
func (s *AlertChannelService) frontendHost() string {
	if u, err := url.Parse(s.frontendURL); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return "aim"
}

// SlackAlertText is the text of the Slack message for alert: its severity and
// title, its description, the agent it names when it names one, and the
// dashboard's alerts page. Every string taken from the alert is escaped.
func SlackAlertText(alert *domain.Alert, alertsPage string) string {
	lines := []string{
		"AIM alert, severity " + EscapeSlackText(string(alert.Severity)) + ": " + EscapeSlackText(alert.Title),
		EscapeSlackText(alert.Description),
	}
	if alert.AgentName != "" {
		lines = append(lines, "Agent: "+EscapeSlackText(alert.AgentName))
	}
	lines = append(lines, EscapeSlackText(alertsPage))
	return strings.Join(lines, "\n")
}

// pagerDutySeverities maps AIM's severities to PagerDuty's.
var pagerDutySeverities = map[domain.AlertSeverity]string{
	domain.AlertSeverityCritical: "critical",
	domain.AlertSeverityHigh:     "error",
	domain.AlertSeverityWarning:  "warning",
	domain.AlertSeverityInfo:     "info",
}

// PagerDutySeverity returns PagerDuty's severity for an AIM severity: critical
// is critical, high is error, warning is warning and info is info.
func PagerDutySeverity(severity domain.AlertSeverity) (string, bool) {
	s, ok := pagerDutySeverities[severity]
	return s, ok
}

// cutRunes returns s cut to at most limit characters.
func cutRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit])
}

// PagerDutyEvent is a PagerDuty Events API v2 event.
type PagerDutyEvent struct {
	RoutingKey  string           `json:"routing_key"`
	EventAction string           `json:"event_action"`
	DedupKey    string           `json:"dedup_key"`
	Payload     PagerDutyPayload `json:"payload"`
}

// PagerDutyPayload is the payload of a PagerDuty event.
type PagerDutyPayload struct {
	Summary       string                 `json:"summary"`
	Source        string                 `json:"source"`
	Severity      string                 `json:"severity"`
	Timestamp     string                 `json:"timestamp,omitempty"`
	Class         string                 `json:"class,omitempty"`
	CustomDetails map[string]interface{} `json:"custom_details,omitempty"`
}

// PagerDutyAlertEvent is the event that triggers an incident for alert on the
// service routingKey names, deduplicated on the alert's id. source names the
// alert's agent, or is fallbackSource when the alert names none.
func PagerDutyAlertEvent(alert *domain.Alert, routingKey, fallbackSource string) (PagerDutyEvent, error) {
	severity, ok := PagerDutySeverity(alert.Severity)
	if !ok {
		return PagerDutyEvent{}, fmt.Errorf("severity %q has no PagerDuty severity", alert.Severity)
	}
	summary := alert.Title
	if strings.TrimSpace(summary) == "" {
		summary = "AIM alert, severity " + string(alert.Severity)
	}
	source := alert.AgentName
	if source == "" {
		source = fallbackSource
	}
	return PagerDutyEvent{
		RoutingKey:  routingKey,
		EventAction: "trigger",
		DedupKey:    alert.ID.String(),
		Payload: PagerDutyPayload{
			Summary:   cutRunes(summary, pagerDutySummaryLimit),
			Source:    source,
			Severity:  severity,
			Timestamp: alert.CreatedAt.UTC().Format(time.RFC3339),
			Class:     string(alert.AlertType),
			CustomDetails: map[string]interface{}{
				"alertId":        alert.ID.String(),
				"organizationId": alert.OrganizationID.String(),
				"alertType":      string(alert.AlertType),
				"severity":       string(alert.Severity),
				"description":    alert.Description,
				"resourceType":   alert.ResourceType,
				"resourceId":     alert.ResourceID.String(),
			},
		},
	}, nil
}

// encodeAlertChannelBody encodes v as JSON without escaping &, < and >, so a
// Slack-escaped string arrives as written.
func encodeAlertChannelBody(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// alertMessage returns where to send alert through channel and the body.
func (s *AlertChannelService) alertMessage(channel *domain.AlertChannel, alert *domain.Alert) (string, []byte, error) {
	switch channel.Kind {
	case domain.AlertChannelSlack:
		body, err := encodeAlertChannelBody(map[string]string{"text": SlackAlertText(alert, s.alertsPageURL())})
		return channel.Config[domain.AlertChannelConfigWebhookURL], body, err
	case domain.AlertChannelPagerDuty:
		event, err := PagerDutyAlertEvent(alert, channel.Config[domain.AlertChannelConfigRoutingKey], s.frontendHost())
		if err != nil {
			return "", nil, err
		}
		body, err := encodeAlertChannelBody(event)
		return PagerDutyEventsURL, body, err
	}
	return "", nil, fmt.Errorf("unsupported kind %q", channel.Kind)
}

// testMessage returns where to send channel's test message and the body.
func (s *AlertChannelService) testMessage(channel *domain.AlertChannel) (string, []byte, error) {
	switch channel.Kind {
	case domain.AlertChannelSlack:
		body, err := encodeAlertChannelBody(map[string]string{
			"text": "AIM test message for alert channel " + EscapeSlackText(channel.Name),
		})
		return channel.Config[domain.AlertChannelConfigWebhookURL], body, err
	case domain.AlertChannelPagerDuty:
		body, err := encodeAlertChannelBody(PagerDutyEvent{
			RoutingKey:  channel.Config[domain.AlertChannelConfigRoutingKey],
			EventAction: "trigger",
			DedupKey:    "aim-channel-test-" + channel.ID.String(),
			Payload: PagerDutyPayload{
				Summary:   cutRunes("AIM test event for alert channel "+channel.Name, pagerDutySummaryLimit),
				Source:    s.frontendHost(),
				Severity:  "info",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		})
		return PagerDutyEventsURL, body, err
	}
	return "", nil, fmt.Errorf("unsupported kind %q", channel.Kind)
}
