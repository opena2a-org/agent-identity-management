package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// DeviceCodeStatus represents the state of a device authorization request.
type DeviceCodeStatus string

const (
	DeviceCodeStatusPending  DeviceCodeStatus = "pending"
	DeviceCodeStatusApproved DeviceCodeStatus = "approved"
	DeviceCodeStatusDenied   DeviceCodeStatus = "denied"
	DeviceCodeStatusExpired  DeviceCodeStatus = "expired"
)

// DeviceCode represents an RFC 8628 device authorization grant request.
type DeviceCode struct {
	ID              uuid.UUID
	DeviceCode      string
	UserCode        string
	ClientID        string
	Scope           string
	VerificationURI string
	ExpiresAt       time.Time
	IntervalSeconds int
	Status          DeviceCodeStatus
	UserID          *uuid.UUID
	OrganizationID  *uuid.UUID
	IPAddress       string
	UserAgent       string
	ApprovedAt      *time.Time
	CreatedAt       time.Time
}

// IsExpired returns true if the device code has passed its expiration time.
func (dc *DeviceCode) IsExpired() bool {
	return time.Now().After(dc.ExpiresAt)
}

// DeviceCodeRepository defines persistence operations for device authorization codes.
type DeviceCodeRepository interface {
	Create(ctx context.Context, dc *DeviceCode) error
	GetByDeviceCode(ctx context.Context, deviceCode string) (*DeviceCode, error)
	GetByUserCode(ctx context.Context, userCode string) (*DeviceCode, error)
	Approve(ctx context.Context, userCode string, userID uuid.UUID, orgID uuid.UUID) error
	Deny(ctx context.Context, userCode string) error
	CleanupExpired(ctx context.Context) (int64, error)
}
