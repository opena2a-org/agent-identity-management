package atc

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestATCClaims_IsExpired(t *testing.T) {
	tests := []struct {
		name    string
		expires time.Time
		want    bool
	}{
		{"future", time.Now().Add(time.Hour), false},
		{"past", time.Now().Add(-time.Hour), true},
		{"just expired", time.Now().Add(-time.Second), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ATCClaims{ExpiresAt: tt.expires}
			if got := c.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestATCClaims_HasCapability(t *testing.T) {
	claims := &ATCClaims{
		AgentID:      uuid.New(),
		Capabilities: []string{"secrets:resolve", "agents:read"},
	}

	tests := []struct {
		name     string
		required string
		want     bool
	}{
		{"exact match", "secrets:resolve", true},
		{"exact match 2", "agents:read", true},
		{"hierarchical match", "secrets:resolve:github", true},
		{"no match", "admin:delete", false},
		{"partial prefix no match", "secrets:rotate", false},
		{"empty required", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := claims.HasCapability(tt.required); got != tt.want {
				t.Errorf("HasCapability(%q) = %v, want %v", tt.required, got, tt.want)
			}
		})
	}
}

func TestATCClaims_HasCapability_Wildcard(t *testing.T) {
	claims := &ATCClaims{
		Capabilities: []string{"*"},
	}

	if !claims.HasCapability("anything:at:all") {
		t.Error("wildcard should match any capability")
	}
}

func TestATCClaims_HasCapability_Empty(t *testing.T) {
	claims := &ATCClaims{
		Capabilities: nil,
	}

	if claims.HasCapability("secrets:resolve") {
		t.Error("empty capabilities should not match")
	}
}

func TestATCError_Error(t *testing.T) {
	err := &ATCError{Code: ErrCodeExpired, Message: "token expired"}
	want := "atc_expired: token expired"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
