package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// OIDCSignInConfig is the OpenID Connect provider the users of one
// organization sign in through.
type OIDCSignInConfig struct {
	OrganizationID uuid.UUID
	// Issuer is the provider's issuer identifier, an https URL. The provider's
	// configuration document is read from it, and every ID token it issues
	// names it.
	Issuer string
	// ClientID is the client the provider registered for this deployment.
	ClientID string
	// SealedClientSecret is the client secret sealed by the key vault to the
	// organization. No answer carries it, sealed or not.
	SealedClientSecret string
	// Provisioning is what a sign-in does for a verified email that no user of
	// the organization has.
	Provisioning OIDCProvisioning
	// UpdatedBy is the user who stored the configuration.
	UpdatedBy uuid.UUID
	UpdatedAt time.Time
}

// OIDCProvisioning is the just-in-time provisioning of an organization's OIDC
// sign-in. Without it, a sign-in for a verified email that no user of the
// organization has is refused.
type OIDCProvisioning struct {
	// Enabled creates a user of the organization for such an email, at its
	// first sign-in, when the email's domain is one of EmailDomains and no
	// user of another organization has the email.
	Enabled bool
	// Role is the role of a user it creates.
	Role UserRole
	// EmailDomains are the domains, in lower case, of the emails it creates
	// a user for.
	EmailDomains []string
}

// ErrOIDCSignInNotConfigured is returned when an organization has no OIDC
// sign-in configuration.
var ErrOIDCSignInNotConfigured = errors.New("the organization has no OIDC sign-in configuration")

// OIDCSignInConfigRepository stores one OIDC sign-in configuration per
// organization.
type OIDCSignInConfigRepository interface {
	// Get returns the organization's configuration, or
	// ErrOIDCSignInNotConfigured when it has none.
	Get(ctx context.Context, organizationID uuid.UUID) (*OIDCSignInConfig, error)
	// Put stores cfg as its organization's configuration, in place of any
	// earlier one.
	Put(ctx context.Context, cfg *OIDCSignInConfig) error
	// Delete removes the organization's configuration. An organization with
	// none is not an error.
	Delete(ctx context.Context, organizationID uuid.UUID) error
}
