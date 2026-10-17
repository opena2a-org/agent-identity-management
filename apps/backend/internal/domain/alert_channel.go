package domain

import (
	"errors"
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
)

// The config keys that hold each kind's destination.
const (
	AlertChannelConfigWebhookURL = "webhookUrl" // slack
	AlertChannelConfigRoutingKey = "routingKey" // pagerduty
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
// destination. Config holds the destination itself, a Slack webhook URL or a
// PagerDuty routing key, so it is never serialized; View is what an answer
// carries.
type AlertChannel struct {
	ID                 uuid.UUID         `json:"id"`
	OrganizationID     uuid.UUID         `json:"-"`
	Name               string            `json:"name"`
	Kind               AlertChannelKind  `json:"kind"`
	Config             map[string]string `json:"-"`
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
// value is masked to its last four characters.
type AlertChannelView struct {
	ID                 uuid.UUID         `json:"id"`
	Name               string            `json:"name"`
	Kind               AlertChannelKind  `json:"kind"`
	MinSeverity        AlertSeverity     `json:"minSeverity"`
	IsActive           bool              `json:"isActive"`
	Config             map[string]string `json:"config"`
	LastDeliveryStatus *string           `json:"lastDeliveryStatus"`
	LastDeliveryCode   *int              `json:"lastDeliveryCode"`
	LastDeliveryAt     *time.Time        `json:"lastDeliveryAt"`
	CreatedAt          time.Time         `json:"createdAt"`
	UpdatedAt          time.Time         `json:"updatedAt"`
}

// View returns the channel with its config masked.
func (c *AlertChannel) View() AlertChannelView {
	masked := make(map[string]string, len(c.Config))
	for key, value := range c.Config {
		masked[key] = MaskAlertChannelSecret(value)
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
