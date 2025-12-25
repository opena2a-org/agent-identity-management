package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestEnforcementModeConstants(t *testing.T) {
	assert.Equal(t, EnforcementMode("strict"), EnforcementModeStrict)
	assert.Equal(t, EnforcementMode("monitoring"), EnforcementModeMonitoring)
}

func TestOrganizationStruct(t *testing.T) {
	now := time.Now()
	orgID := uuid.New()

	org := Organization{
		ID:              orgID,
		Name:            "Test Organization",
		Domain:          "test.example.com",
		PlanType:        "enterprise",
		MaxAgents:       100,
		MaxUsers:        50,
		IsActive:        true,
		EnforcementMode: EnforcementModeMonitoring,
		Settings: map[string]interface{}{
			"feature_flags": []string{"new_ui", "beta_features"},
			"custom_setting": "value",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	assert.Equal(t, orgID, org.ID)
	assert.Equal(t, "Test Organization", org.Name)
	assert.Equal(t, "test.example.com", org.Domain)
	assert.Equal(t, "enterprise", org.PlanType)
	assert.Equal(t, 100, org.MaxAgents)
	assert.Equal(t, 50, org.MaxUsers)
	assert.True(t, org.IsActive)
	assert.Equal(t, EnforcementModeMonitoring, org.EnforcementMode)
	assert.NotNil(t, org.Settings)
}

func TestOrganizationWithStrictEnforcement(t *testing.T) {
	org := Organization{
		ID:              uuid.New(),
		Name:            "Strict Org",
		EnforcementMode: EnforcementModeStrict,
	}

	assert.Equal(t, EnforcementModeStrict, org.EnforcementMode)
}

func TestOrganizationSettings(t *testing.T) {
	org := Organization{
		ID:   uuid.New(),
		Name: "Settings Org",
		Settings: map[string]interface{}{
			"string_setting":  "value",
			"int_setting":     42,
			"bool_setting":    true,
			"float_setting":   3.14,
			"array_setting":   []string{"a", "b", "c"},
			"nested_setting": map[string]interface{}{
				"key": "nested_value",
			},
		},
	}

	assert.NotNil(t, org.Settings)
	assert.Equal(t, "value", org.Settings["string_setting"])
	assert.Equal(t, 42, org.Settings["int_setting"])
	assert.Equal(t, true, org.Settings["bool_setting"])
	assert.Equal(t, 3.14, org.Settings["float_setting"])
}

func TestOrganizationWithNilSettings(t *testing.T) {
	org := Organization{
		ID:       uuid.New(),
		Name:     "No Settings Org",
		Settings: nil,
	}

	assert.Nil(t, org.Settings)
}

func TestOrganizationPlanTypes(t *testing.T) {
	planTypes := []string{"free", "starter", "professional", "enterprise"}

	for _, planType := range planTypes {
		t.Run(planType, func(t *testing.T) {
			org := Organization{
				ID:       uuid.New(),
				Name:     planType + " org",
				PlanType: planType,
			}
			assert.Equal(t, planType, org.PlanType)
		})
	}
}

func TestOrganizationActiveStatus(t *testing.T) {
	activeOrg := Organization{
		ID:       uuid.New(),
		Name:     "Active Org",
		IsActive: true,
	}

	inactiveOrg := Organization{
		ID:       uuid.New(),
		Name:     "Inactive Org",
		IsActive: false,
	}

	assert.True(t, activeOrg.IsActive)
	assert.False(t, inactiveOrg.IsActive)
}

func TestEnforcementModeStringConversion(t *testing.T) {
	mode := EnforcementModeStrict
	str := string(mode)
	assert.Equal(t, "strict", str)

	// Convert back
	converted := EnforcementMode(str)
	assert.Equal(t, EnforcementModeStrict, converted)
}

func TestOrganizationLimits(t *testing.T) {
	tests := []struct {
		name      string
		maxAgents int
		maxUsers  int
	}{
		{"free tier", 5, 3},
		{"starter tier", 25, 10},
		{"professional tier", 100, 50},
		{"enterprise tier", 1000, 500},
		{"unlimited", -1, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org := Organization{
				ID:        uuid.New(),
				Name:      tt.name,
				MaxAgents: tt.maxAgents,
				MaxUsers:  tt.maxUsers,
			}
			assert.Equal(t, tt.maxAgents, org.MaxAgents)
			assert.Equal(t, tt.maxUsers, org.MaxUsers)
		})
	}
}
