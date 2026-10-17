package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AlertChannelKind names the service an alert channel sends an organization's
// alerts to, each in that service's published format.
type AlertChannelKind string

const (
	// AlertChannelSlack posts to a Slack incoming webhook.
	AlertChannelSlack AlertChannelKind = "slack"
	// AlertChannelPagerDuty sends a PagerDuty Events API v2 event.
	AlertChannelPagerDuty AlertChannelKind = "pagerduty"
	// AlertChannelTeams posts an Adaptive Card message to a Microsoft Teams
	// workflow's "When a Teams webhook request is received" trigger.
	AlertChannelTeams AlertChannelKind = "teams"
	// AlertChannelEmail sends a plain-text email to each of a list of
	// addresses through the server's email service.
	AlertChannelEmail AlertChannelKind = "email"
)

// The config keys that hold each kind's destination.
const (
	AlertChannelConfigWebhookURL = "webhookUrl" // slack, teams
	AlertChannelConfigRoutingKey = "routingKey" // pagerduty
	AlertChannelConfigRecipients = "recipients" // email
)

// The outcomes an alert channel's last delivery records.
const (
	AlertChannelDelivered = "delivered"
	AlertChannelFailed    = "failed"
)

var (
	// ErrAlertChannelNotFound is returned for a channel that does not exist in
	// the organization asked for, whether or not it exists in another.
	ErrAlertChannelNotFound = errors.New("alert channel not found")
	// ErrAlertChannelNameTaken is returned when another channel of the
	// organization has the name.
	ErrAlertChannelNameTaken = errors.New("an alert channel of the organization has this name")
)

// AlertChannel sends an organization's alerts of MinSeverity or above to one
// destination. Config holds the destination itself, a Slack or Teams webhook
// URL or a PagerDuty routing key, so it is never serialized; View is what an
// answer carries. Recipients are an email channel's addresses, which are not
// credentials; ConfigJSON stores them with Config.
type AlertChannel struct {
	ID                 uuid.UUID         `json:"id"`
	OrganizationID     uuid.UUID         `json:"-"`
	Name               string            `json:"name"`
	Kind               AlertChannelKind  `json:"kind"`
	Config             map[string]string `json:"-"`
	Recipients         []string          `json:"-"`
	MinSeverity        AlertSeverity     `json:"minSeverity"`
	IsActive           bool              `json:"isActive"`
	LastDeliveryStatus *string           `json:"lastDeliveryStatus"`
	LastDeliveryCode   *int              `json:"lastDeliveryCode"`
	LastDeliveryAt     *time.Time        `json:"lastDeliveryAt"`
	CreatedBy          *uuid.UUID        `json:"-"`
	CreatedAt          time.Time         `json:"createdAt"`
	UpdatedAt          time.Time         `json:"updatedAt"`
}

// AlertChannelView is an alert channel as an answer carries it: each config
// value is masked to its last four characters, and an email channel's
// recipients are given as they are stored.
type AlertChannelView struct {
	ID                 uuid.UUID              `json:"id"`
	Name               string                 `json:"name"`
	Kind               AlertChannelKind       `json:"kind"`
	MinSeverity        AlertSeverity          `json:"minSeverity"`
	IsActive           bool                   `json:"isActive"`
	Config             map[string]interface{} `json:"config"`
	LastDeliveryStatus *string                `json:"lastDeliveryStatus"`
	LastDeliveryCode   *int                   `json:"lastDeliveryCode"`
	LastDeliveryAt     *time.Time             `json:"lastDeliveryAt"`
	CreatedAt          time.Time              `json:"createdAt"`
	UpdatedAt          time.Time              `json:"updatedAt"`
}

// View returns the channel with its config masked and its recipients, if it
// has any, unmasked.
func (c *AlertChannel) View() AlertChannelView {
	masked := make(map[string]interface{}, len(c.Config)+1)
	for key, value := range c.Config {
		masked[key] = MaskAlertChannelSecret(value)
	}
	if c.Recipients != nil {
		masked[AlertChannelConfigRecipients] = append([]string(nil), c.Recipients...)
	}
	return AlertChannelView{
		ID:                 c.ID,
		Name:               c.Name,
		Kind:               c.Kind,
		MinSeverity:        c.MinSeverity,
		IsActive:           c.IsActive,
		Config:             masked,
		LastDeliveryStatus: c.LastDeliveryStatus,
		LastDeliveryCode:   c.LastDeliveryCode,
		LastDeliveryAt:     c.LastDeliveryAt,
		CreatedAt:          c.CreatedAt,
		UpdatedAt:          c.UpdatedAt,
	}
}

// ConfigJSON returns the config column's value: Config as a JSON object of
// strings, with Recipients added as the array "recipients" when the channel
// has them.
func (c *AlertChannel) ConfigJSON() ([]byte, error) {
	if c.Recipients == nil {
		return json.Marshal(c.Config)
	}
	fields := make(map[string]interface{}, len(c.Config)+1)
	for key, value := range c.Config {
		fields[key] = value
	}
	fields[AlertChannelConfigRecipients] = c.Recipients
	return json.Marshal(fields)
}

// SetConfigJSON reads a config column's value into Config and Recipients, as
// ConfigJSON writes it.
func (c *AlertChannel) SetConfigJSON(raw []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	c.Config = make(map[string]string, len(fields))
	c.Recipients = nil
	for key, value := range fields {
		if key == AlertChannelConfigRecipients {
			if err := json.Unmarshal(value, &c.Recipients); err != nil {
				return fmt.Errorf("config.%s: %w", key, err)
			}
			continue
		}
		var s string
		if err := json.Unmarshal(value, &s); err != nil {
			return fmt.Errorf("config.%s: %w", key, err)
		}
		c.Config[key] = s
	}
	return nil
}

// MaskAlertChannelSecret returns "****" followed by the last four characters
// of value, or "****" alone when value has four characters or fewer.
func MaskAlertChannelSecret(value string) string {
	runes := []rune(value)
	if len(runes) <= 4 {
		return "****"
	}
	return "****" + string(runes[len(runes)-4:])
}

// alertSeverityRanks orders the alert severities: info < warning < high <
// critical.
var alertSeverityRanks = map[AlertSeverity]int{
	AlertSeverityInfo:     0,
	AlertSeverityWarning:  1,
	AlertSeverityHigh:     2,
	AlertSeverityCritical: 3,
}

// AlertSeverityRank returns the rank of severity, and false for a value that
// is not info, warning, high or critical.
func AlertSeverityRank(severity AlertSeverity) (int, bool) {
	rank, ok := alertSeverityRanks[severity]
	return rank, ok
}

// AlertSeverities are the alert severities in rank order.
var AlertSeverities = []AlertSeverity{AlertSeverityInfo, AlertSeverityWarning, AlertSeverityHigh, AlertSeverityCritical}

// AlertChannelFloorsAdmitting returns the floors at which a channel is sent an
// alert of severity: every severity ranked at or below it. It returns none for
// a severity that is not one of AlertSeverities.
func AlertChannelFloorsAdmitting(severity AlertSeverity) []AlertSeverity {
	rank, ok := AlertSeverityRank(severity)
	if !ok {
		return nil
	}
	return append([]AlertSeverity(nil), AlertSeverities[:rank+1]...)
}

// AlertChannelAdmits reports whether a channel with the floor minSeverity is
// sent an alert of severity.
func AlertChannelAdmits(minSeverity, severity AlertSeverity) bool {
	floor, okFloor := AlertSeverityRank(minSeverity)
	rank, okRank := AlertSeverityRank(severity)
	return okFloor && okRank && floor <= rank
}
