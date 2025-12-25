package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataTransfer_Structure(t *testing.T) {
	now := time.Now()
	agentID := uuid.New()
	orgID := uuid.New()

	transfer := DataTransfer{
		ID:                uuid.New(),
		AgentID:           agentID,
		OrganizationID:    orgID,
		ActionType:        "http_request",
		Resource:          "/api/data/export",
		DestinationIP:     "203.0.113.50",
		DestinationDomain: "external-api.example.com",
		RequestSize:       1024,
		ResponseSize:      1048576, // 1MB
		Metadata: map[string]interface{}{
			"method":       "POST",
			"content_type": "application/json",
		},
		CreatedAt: now,
	}

	assert.NotEqual(t, uuid.Nil, transfer.ID)
	assert.Equal(t, agentID, transfer.AgentID)
	assert.Equal(t, orgID, transfer.OrganizationID)
	assert.Equal(t, "http_request", transfer.ActionType)
	assert.Equal(t, int64(1024), transfer.RequestSize)
	assert.Equal(t, int64(1048576), transfer.ResponseSize)
	assert.Equal(t, "external-api.example.com", transfer.DestinationDomain)
}

func TestDataTransfer_MinimalFields(t *testing.T) {
	transfer := DataTransfer{
		ID:             uuid.New(),
		AgentID:        uuid.New(),
		OrganizationID: uuid.New(),
		ActionType:     "api_call",
		RequestSize:    100,
		ResponseSize:   500,
		CreatedAt:      time.Now(),
	}

	assert.Empty(t, transfer.Resource)
	assert.Empty(t, transfer.DestinationIP)
	assert.Empty(t, transfer.DestinationDomain)
	assert.Nil(t, transfer.Metadata)
}

func TestDataTransferAggregate_Structure(t *testing.T) {
	now := time.Now()
	bucketStart := now.Truncate(time.Hour)
	bucketEnd := bucketStart.Add(time.Hour)

	aggregate := DataTransferAggregate{
		ID:                 uuid.New(),
		AgentID:            uuid.New(),
		OrganizationID:     uuid.New(),
		BucketStart:        bucketStart,
		BucketEnd:          bucketEnd,
		TotalRequestBytes:  10240,
		TotalResponseBytes: 104857600, // 100MB
		TransferCount:      50,
		UniqueDestinations: 5,
		TopDestinations: []DestinationSummary{
			{Domain: "api.example.com", TotalBytes: 50000000, Count: 25},
			{Domain: "storage.example.com", TotalBytes: 30000000, Count: 15},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	assert.Equal(t, bucketStart, aggregate.BucketStart)
	assert.Equal(t, bucketEnd, aggregate.BucketEnd)
	assert.Equal(t, int64(104857600), aggregate.TotalResponseBytes)
	assert.Equal(t, 50, aggregate.TransferCount)
	assert.Equal(t, 5, aggregate.UniqueDestinations)
	assert.Len(t, aggregate.TopDestinations, 2)
}

func TestDestinationSummary_Structure(t *testing.T) {
	tests := []struct {
		name     string
		summary  DestinationSummary
		hasDomain bool
		hasIP    bool
	}{
		{
			name: "domain only",
			summary: DestinationSummary{
				Domain:     "api.example.com",
				TotalBytes: 1000000,
				Count:      10,
			},
			hasDomain: true,
			hasIP:     false,
		},
		{
			name: "IP only",
			summary: DestinationSummary{
				IP:         "192.168.1.100",
				TotalBytes: 500000,
				Count:      5,
			},
			hasDomain: false,
			hasIP:     true,
		},
		{
			name: "both domain and IP",
			summary: DestinationSummary{
				Domain:     "api.example.com",
				IP:         "192.168.1.100",
				TotalBytes: 2000000,
				Count:      20,
			},
			hasDomain: true,
			hasIP:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.hasDomain, tt.summary.Domain != "")
			assert.Equal(t, tt.hasIP, tt.summary.IP != "")
			assert.Greater(t, tt.summary.TotalBytes, int64(0))
			assert.Greater(t, tt.summary.Count, 0)
		})
	}
}

func TestAgentTransferSummary_Structure(t *testing.T) {
	summary := AgentTransferSummary{
		AgentID:        uuid.New(),
		AgentName:      "data-agent",
		TotalBytes:     1073741824, // 1GB
		TransferCount:  1000,
		TopDestination: "storage.example.com",
	}

	assert.NotEqual(t, uuid.Nil, summary.AgentID)
	assert.Equal(t, "data-agent", summary.AgentName)
	assert.Equal(t, int64(1073741824), summary.TotalBytes)
	assert.Equal(t, 1000, summary.TransferCount)
	assert.Equal(t, "storage.example.com", summary.TopDestination)
}

func TestDefaultDataExfiltrationConfig(t *testing.T) {
	config := DefaultDataExfiltrationConfig()

	assert.Equal(t, 100, config.ThresholdMB)
	assert.Equal(t, 60, config.TimeWindowMins)
	assert.False(t, config.BlockOnThreshold)
	assert.Equal(t, AlertSeverityHigh, config.AlertSeverity)
}

func TestDataExfiltrationConfig_Custom(t *testing.T) {
	config := DataExfiltrationConfig{
		ThresholdMB:      50,
		TimeWindowMins:   30,
		BlockOnThreshold: true,
		AlertSeverity:    AlertSeverityCritical,
	}

	assert.Equal(t, 50, config.ThresholdMB)
	assert.Equal(t, 30, config.TimeWindowMins)
	assert.True(t, config.BlockOnThreshold)
	assert.Equal(t, AlertSeverityCritical, config.AlertSeverity)
}

func TestBytesToMB(t *testing.T) {
	tests := []struct {
		bytes      int64
		expectedMB float64
	}{
		{0, 0.0},
		{1024, 0.0009765625},
		{1048576, 1.0},          // 1MB
		{10485760, 10.0},        // 10MB
		{104857600, 100.0},      // 100MB
		{1073741824, 1024.0},    // 1GB
	}

	for _, tt := range tests {
		mb := BytesToMB(tt.bytes)
		assert.InDelta(t, tt.expectedMB, mb, 0.0001)
	}
}

func TestMBToBytes(t *testing.T) {
	tests := []struct {
		mb            int
		expectedBytes int64
	}{
		{0, 0},
		{1, 1048576},
		{10, 10485760},
		{100, 104857600},
		{1024, 1073741824}, // 1GB
	}

	for _, tt := range tests {
		bytes := MBToBytes(tt.mb)
		assert.Equal(t, tt.expectedBytes, bytes)
	}
}

func TestBytesToMB_MBToBytes_RoundTrip(t *testing.T) {
	// Test that converting MB to bytes and back gives the same value
	for _, mb := range []int{1, 10, 50, 100, 500, 1000} {
		bytes := MBToBytes(mb)
		result := BytesToMB(bytes)
		assert.Equal(t, float64(mb), result)
	}
}

func TestDataTransfer_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	transfer := DataTransfer{
		ID:                uuid.New(),
		AgentID:           uuid.New(),
		OrganizationID:    uuid.New(),
		ActionType:        "api_call",
		Resource:          "/api/data",
		DestinationIP:     "10.0.0.1",
		DestinationDomain: "internal.example.com",
		RequestSize:       1000,
		ResponseSize:      5000,
		Metadata:          map[string]interface{}{"key": "value"},
		CreatedAt:         now,
	}

	// Serialize to JSON
	data, err := json.Marshal(transfer)
	require.NoError(t, err)

	// Deserialize back
	var restored DataTransfer
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, transfer.ID, restored.ID)
	assert.Equal(t, transfer.AgentID, restored.AgentID)
	assert.Equal(t, transfer.ActionType, restored.ActionType)
	assert.Equal(t, transfer.RequestSize, restored.RequestSize)
	assert.Equal(t, transfer.ResponseSize, restored.ResponseSize)
}

func TestDataTransferAggregate_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	aggregate := DataTransferAggregate{
		ID:                 uuid.New(),
		AgentID:            uuid.New(),
		OrganizationID:     uuid.New(),
		BucketStart:        now,
		BucketEnd:          now.Add(time.Hour),
		TotalRequestBytes:  1000,
		TotalResponseBytes: 5000,
		TransferCount:      10,
		UniqueDestinations: 2,
		TopDestinations: []DestinationSummary{
			{Domain: "example.com", TotalBytes: 3000, Count: 6},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Serialize to JSON
	data, err := json.Marshal(aggregate)
	require.NoError(t, err)

	// Deserialize back
	var restored DataTransferAggregate
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, aggregate.ID, restored.ID)
	assert.Equal(t, aggregate.TotalResponseBytes, restored.TotalResponseBytes)
	assert.Equal(t, aggregate.TransferCount, restored.TransferCount)
	assert.Len(t, restored.TopDestinations, 1)
}

func TestDataExfiltration_ThresholdCalculation(t *testing.T) {
	config := DefaultDataExfiltrationConfig()

	// Test threshold calculation
	thresholdBytes := MBToBytes(config.ThresholdMB)
	assert.Equal(t, int64(104857600), thresholdBytes) // 100MB

	// Simulate checking if transfer exceeds threshold
	transfers := []struct {
		bytes          int64
		exceedsThreshold bool
	}{
		{50 * 1024 * 1024, false},  // 50MB - under threshold
		{100 * 1024 * 1024, false}, // 100MB - exactly at threshold (not exceeded)
		{101 * 1024 * 1024, true},  // 101MB - over threshold
		{500 * 1024 * 1024, true},  // 500MB - way over threshold
	}

	for _, tt := range transfers {
		exceeds := tt.bytes > thresholdBytes
		assert.Equal(t, tt.exceedsThreshold, exceeds, "bytes: %d", tt.bytes)
	}
}
