package domain

import (
	"context"
	"errors"
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
	// DeviceCodeStatusConsumed marks an approved code that has been exchanged
	// for its token pair. A code is exchanged once.
	DeviceCodeStatusConsumed DeviceCodeStatus = "consumed"
)

// ErrDeviceCodeNotApproved is returned by Consume when the code is not, or is
// no longer, an approved and unexpired code: another poll exchanged it first,
// or it expired.
var ErrDeviceCodeNotApproved = errors.New("device code is not approved")

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
	// Consume moves an approved, unexpired code to consumed and calls issue in
	// the same transaction. The change commits only if issue returns nil; if
	// issue fails the code stays approved. When the code is not approved it
	// returns ErrDeviceCodeNotApproved without calling issue. Of any number of
	// concurrent calls on one code, at most one commits.
	Consume(ctx context.Context, deviceCode string, issue func() error) error
	CleanupExpired(ctx context.Context) (int64, error)
}
