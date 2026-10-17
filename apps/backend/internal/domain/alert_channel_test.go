package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaskAlertChannelSecret(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{"empty", "", "****"},
		{"four characters", "abcd", "****"},
		{"five characters", "abcde", "****bcde"},
		{"placeholder key", "FAKE-KEY-5678", "****5678"},
		// The cut counts characters, not bytes: each of these is two bytes.
		{"four two-byte characters", "éèêë", "****"},
		{"five two-byte characters", "àéèêë", "****éèêë"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, MaskAlertChannelSecret(tt.value))
		})
	}
}

func TestAlertSeverityRank(t *testing.T) {
	tests := []struct {
		severity AlertSeverity
		rank     int
	}{
		{AlertSeverityInfo, 0},
		{AlertSeverityWarning, 1},
		{AlertSeverityHigh, 2},
		{AlertSeverityCritical, 3},
	}
	for _, tt := range tests {
		rank, ok := AlertSeverityRank(tt.severity)
		assert.True(t, ok, "%s", tt.severity)
		assert.Equal(t, tt.rank, rank, "%s", tt.severity)
	}

	for _, severity := range []AlertSeverity{"", "urgent", "Critical"} {
		_, ok := AlertSeverityRank(severity)
		assert.False(t, ok, "%q", severity)
	}
}

func TestAlertChannelFloorsAdmitting(t *testing.T) {
	assert.Equal(t, []AlertSeverity{AlertSeverityInfo},
		AlertChannelFloorsAdmitting(AlertSeverityInfo))
	assert.Equal(t, []AlertSeverity{AlertSeverityInfo, AlertSeverityWarning},
		AlertChannelFloorsAdmitting(AlertSeverityWarning))
	assert.Equal(t, []AlertSeverity{AlertSeverityInfo, AlertSeverityWarning, AlertSeverityHigh},
		AlertChannelFloorsAdmitting(AlertSeverityHigh))
	assert.Equal(t, []AlertSeverity{AlertSeverityInfo, AlertSeverityWarning, AlertSeverityHigh, AlertSeverityCritical},
		AlertChannelFloorsAdmitting(AlertSeverityCritical))
	assert.Nil(t, AlertChannelFloorsAdmitting("urgent"))

	// The answer is a copy: changing it leaves AlertSeverities as it was.
	floors := AlertChannelFloorsAdmitting(AlertSeverityCritical)
	floors[0] = AlertSeverityCritical
	assert.Equal(t, AlertSeverityInfo, AlertSeverities[0])
}

func TestAlertChannelAdmits(t *testing.T) {
	severities := []AlertSeverity{AlertSeverityInfo, AlertSeverityWarning, AlertSeverityHigh, AlertSeverityCritical}
	// admits[i][j] is whether a channel with the floor severities[i] is sent an
	// alert of severities[j]: each of the 16 pairs.
	admits := [4][4]bool{
		{true, true, true, true},
		{false, true, true, true},
		{false, false, true, true},
		{false, false, false, true},
	}
	for i, floor := range severities {
		for j, severity := range severities {
			assert.Equal(t, admits[i][j], AlertChannelAdmits(floor, severity), "floor %s, severity %s", floor, severity)
		}
	}

	assert.False(t, AlertChannelAdmits("urgent", AlertSeverityCritical))
	assert.False(t, AlertChannelAdmits(AlertSeverityInfo, "urgent"))
}

func TestAlertChannel_ViewMasksEachConfigValue(t *testing.T) {
	status := AlertChannelDelivered
	code := 202
	deliveredAt := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	createdBy := uuid.New()
	channel := &AlertChannel{
		ID:                 uuid.New(),
		OrganizationID:     uuid.New(),
		Name:               "on-call",
		Kind:               AlertChannelPagerDuty,
		Config:             map[string]string{AlertChannelConfigRoutingKey: "FAKE-KEY-5678"},
		MinSeverity:        AlertSeverityHigh,
		IsActive:           true,
		LastDeliveryStatus: &status,
		LastDeliveryCode:   &code,
		LastDeliveryAt:     &deliveredAt,
		CreatedBy:          &createdBy,
		CreatedAt:          deliveredAt.Add(-time.Hour),
		UpdatedAt:          deliveredAt,
	}

	view := channel.View()

	assert.Len(t, view.Config, 1)
	assert.Equal(t, "****5678", view.Config[AlertChannelConfigRoutingKey])
	assert.Equal(t, "FAKE-KEY-5678", channel.Config[AlertChannelConfigRoutingKey], "the channel keeps its destination")
	assert.Equal(t, channel.ID, view.ID)
	assert.Equal(t, channel.Name, view.Name)
	assert.Equal(t, channel.Kind, view.Kind)
	assert.Equal(t, channel.MinSeverity, view.MinSeverity)
	assert.Equal(t, channel.IsActive, view.IsActive)
	assert.Equal(t, channel.LastDeliveryStatus, view.LastDeliveryStatus)
	assert.Equal(t, channel.LastDeliveryCode, view.LastDeliveryCode)
	assert.Equal(t, channel.LastDeliveryAt, view.LastDeliveryAt)
	assert.Equal(t, channel.CreatedAt, view.CreatedAt)
	assert.Equal(t, channel.UpdatedAt, view.UpdatedAt)
}

func TestAlertChannel_ViewGivesRecipientsUnmasked(t *testing.T) {
	channel := &AlertChannel{
		Kind:       AlertChannelEmail,
		Config:     map[string]string{},
		Recipients: []string{"oncall@example.com", "security@example.com"},
	}

	view := channel.View()

	assert.Len(t, view.Config, 1)
	assert.Equal(t, []string{"oncall@example.com", "security@example.com"}, view.Config[AlertChannelConfigRecipients])

	// The view carries a copy: changing it leaves the channel's recipients as
	// they were.
	recipients, ok := view.Config[AlertChannelConfigRecipients].([]string)
	require.True(t, ok)
	recipients[0] = "changed@example.com"
	assert.Equal(t, "oncall@example.com", channel.Recipients[0])
}

func TestAlertChannel_ConfigJSON(t *testing.T) {
	// Without recipients the column holds Config alone.
	pagerDuty := &AlertChannel{
		Kind:   AlertChannelPagerDuty,
		Config: map[string]string{AlertChannelConfigRoutingKey: "FAKE-KEY-5678"},
	}
	raw, err := pagerDuty.ConfigJSON()
	require.NoError(t, err)
	assert.Equal(t, `{"routingKey":"FAKE-KEY-5678"}`, string(raw))

	// Recipients are added as the array "recipients" beside Config's values.
	withRecipients := &AlertChannel{
		Config:     map[string]string{AlertChannelConfigWebhookURL: "FAKE-URL-1234"},
		Recipients: []string{"oncall@example.com", "security@example.com"},
	}
	raw, err = withRecipients.ConfigJSON()
	require.NoError(t, err)
	assert.JSONEq(t, `{"webhookUrl":"FAKE-URL-1234","recipients":["oncall@example.com","security@example.com"]}`, string(raw))
}

func TestAlertChannel_SetConfigJSON(t *testing.T) {
	// SetConfigJSON reads back what ConfigJSON wrote.
	for _, written := range []*AlertChannel{
		{Kind: AlertChannelSlack, Config: map[string]string{AlertChannelConfigWebhookURL: "FAKE-URL-1234"}},
		{Kind: AlertChannelEmail, Config: map[string]string{}, Recipients: []string{"oncall@example.com", "security@example.com"}},
	} {
		raw, err := written.ConfigJSON()
		require.NoError(t, err)

		var read AlertChannel
		require.NoError(t, read.SetConfigJSON(raw))
		assert.Equal(t, written.Config, read.Config, "%s", raw)
		assert.Equal(t, written.Recipients, read.Recipients, "%s", raw)
	}

	// A read replaces what the channel held.
	channel := &AlertChannel{
		Config:     map[string]string{AlertChannelConfigWebhookURL: "FAKE-URL-1234"},
		Recipients: []string{"oncall@example.com"},
	}
	require.NoError(t, channel.SetConfigJSON([]byte(`{"routingKey":"FAKE-KEY-5678"}`)))
	assert.Equal(t, map[string]string{AlertChannelConfigRoutingKey: "FAKE-KEY-5678"}, channel.Config)
	assert.Nil(t, channel.Recipients)

	// A value of another type names its field, and a column that is not an
	// object is refused.
	var read AlertChannel
	assert.ErrorContains(t, read.SetConfigJSON([]byte(`{"routingKey":5678}`)), "config.routingKey: ")
	assert.ErrorContains(t, read.SetConfigJSON([]byte(`{"recipients":"oncall@example.com"}`)), "config.recipients: ")
	assert.Error(t, read.SetConfigJSON([]byte(`["oncall@example.com"]`)))
}
