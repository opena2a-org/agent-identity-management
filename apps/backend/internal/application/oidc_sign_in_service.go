package application

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/oauth"
)

const (
	// oidcSignInFlowLifetime is how long after its start a sign-in may be
	// completed.
	oidcSignInFlowLifetime = 10 * time.Minute
	// oidcSignInCallbackPath is the dashboard page the provider redirects the
	// browser to, under the dashboard origin (FRONTEND_URL).
	oidcSignInCallbackPath = "/auth/oidc/callback"

	maxOIDCIssuerLength       = 2048
	maxOIDCClientIDLength     = 512
	maxOIDCClientSecretLength = 2048
)

// The reasons an OIDCSignInRefusal names, one per check that refuses a sign-in.
const (
	OIDCSignInRefusedFlowInvalid          = "flow_invalid"          // the flow is not one this server sealed, or is malformed
	OIDCSignInRefusedFlowExpired          = "flow_expired"          // the sign-in started more than oidcSignInFlowLifetime ago
	OIDCSignInRefusedStateMismatch        = "state_mismatch"        // the state is not the one the sign-in started with
	OIDCSignInRefusedProviderChanged      = "provider_changed"      // the organization's provider was removed or changed since the start
	OIDCSignInRefusedOrganizationInactive = "organization_inactive" // the organization cannot be read or is not active
	OIDCSignInRefusedCodeRefused          = "code_refused"          // the token endpoint refused the authorization code
	OIDCSignInRefusedIDTokenInvalid       = "id_token_invalid"      // the ID token failed verification
	OIDCSignInRefusedEmailNotVerified     = "email_not_verified"    // the ID token asserts no email, or not as verified
	OIDCSignInRefusedNoUser               = "no_user"               // no user of the organization has the email
	OIDCSignInRefusedAmbiguousUser        = "ambiguous_user"        // more than one user of the organization has the email
	OIDCSignInRefusedAccountInactive      = "account_inactive"      // the user may not hold a session
)

// OIDCSignInRefusal is the error CompleteSignIn returns when it refuses a
// sign-in. Reason names the check that refused it. OrganizationID is set once
// the flow named an organization, UserID once a user of it matched.
type OIDCSignInRefusal struct {
	Reason         string
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	cause          error
}

func (e *OIDCSignInRefusal) Error() string {
	if e.cause != nil {
		return "OIDC sign-in refused (" + e.Reason + "): " + e.cause.Error()
	}
	return "OIDC sign-in refused (" + e.Reason + ")"
}

func (e *OIDCSignInRefusal) Unwrap() error { return e.cause }

// ErrOIDCSignInConfigInvalid is wrapped by the error of a configuration
// Configure refuses to store, beside the error of the value it refuses.
var ErrOIDCSignInConfigInvalid = errors.New("invalid OIDC sign-in configuration")

// The values Configure refuses, one error each. Each wraps
// ErrOIDCSignInConfigInvalid.
var (
	// ErrOIDCIssuerInvalid: the issuer is not an https URL with a host and no
	// user information, query or fragment, of at most 2048 characters.
	ErrOIDCIssuerInvalid = fmt.Errorf("%w: issuer", ErrOIDCSignInConfigInvalid)
	// ErrOIDCClientIDInvalid: the client id is empty or longer than 512
	// characters.
	ErrOIDCClientIDInvalid = fmt.Errorf("%w: clientId", ErrOIDCSignInConfigInvalid)
	// ErrOIDCClientSecretInvalid: the client secret is empty or longer than
	// 2048 characters.
	ErrOIDCClientSecretInvalid = fmt.Errorf("%w: clientSecret", ErrOIDCSignInConfigInvalid)
)

// OIDCSignInSealer seals what OIDC sign-in keeps out of reach: an
// organization's client secret at rest, and the state of a sign-in while the
// client holds it. *crypto.KeyVault implements it.
type OIDCSignInSealer interface {
	SealOIDCClientSecret(organizationID uuid.UUID, secret string) (string, error)
	OpenOIDCClientSecret(organizationID uuid.UUID, sealed string) (string, error)
	SealOIDCSignInFlow(organizationID uuid.UUID, flow []byte) (string, error)
	OpenOIDCSignInFlow(organizationID uuid.UUID, sealed string) ([]byte, error)
}

// OIDCSignInUsers is the part of the user repository OIDC sign-in uses: it
// reads the users of one organization and records a sign-in. It has no method
// that creates a user or changes the organization of one.
type OIDCSignInUsers interface {
	GetByOrganization(orgID uuid.UUID) ([]*domain.User, error)
	GetByID(id uuid.UUID) (*domain.User, error)
	UpdateLastLogin(id uuid.UUID, at time.Time) error
}

// OIDCSignInOrganizations reads an organization.
type OIDCSignInOrganizations interface {
	GetByID(id uuid.UUID) (*domain.Organization, error)
}

// OIDCSignInService signs users in through their organization's OpenID
// Connect provider with the authorization code flow and PKCE. The server
// keeps nothing between the start of a sign-in and its callback: the state,
// the nonce and the code verifier travel sealed in the flow value the client
// holds. A sign-in ends as an existing user of the organization whose
// provider issued the ID token, matched by the token's verified email among
// that organization's users only; it creates no user.
type OIDCSignInService struct {
	configs     domain.OIDCSignInConfigRepository
	orgs        OIDCSignInOrganizations
	users       OIDCSignInUsers
	sealer      OIDCSignInSealer
	client      *oauth.OIDCClient
	redirectURI string
	now         func() time.Time
}

// NewOIDCSignInService returns the service. dashboardURL is the dashboard
// origin (FRONTEND_URL); the redirect URI of every authorization request is
// its /auth/oidc/callback page.
func NewOIDCSignInService(
	configs domain.OIDCSignInConfigRepository,
	orgs OIDCSignInOrganizations,
	users OIDCSignInUsers,
	sealer OIDCSignInSealer,
	client *oauth.OIDCClient,
	dashboardURL string,
) *OIDCSignInService {
	return &OIDCSignInService{
		configs:     configs,
		orgs:        orgs,
		users:       users,
		sealer:      sealer,
		client:      client,
		redirectURI: strings.TrimRight(dashboardURL, "/") + oidcSignInCallbackPath,
		now:         time.Now,
	}
}

// RedirectURL returns the redirect URI every authorization request names,
// the one to register with the provider.
func (s *OIDCSignInService) RedirectURL() string {
	return s.redirectURI
}

// Configuration returns the organization's configuration, or
// domain.ErrOIDCSignInNotConfigured.
func (s *OIDCSignInService) Configuration(ctx context.Context, organizationID uuid.UUID) (*domain.OIDCSignInConfig, error) {
	return s.configs.Get(ctx, organizationID)
}

// OIDCSignInConfigInput is a configuration an administrator stores.
type OIDCSignInConfigInput struct {
	Issuer       string
	ClientID     string
	ClientSecret string
}

// Configure stores in as the organization's configuration, in place of any
// earlier one, with the client secret sealed to the organization. It refuses
// an issuer, client id or client secret that breaks its rule with
// ErrOIDCIssuerInvalid, ErrOIDCClientIDInvalid or ErrOIDCClientSecretInvalid,
// and an issuer whose configuration document cannot be read or names another
// issuer with an error that wraps oauth.ErrOIDCProviderUnavailable.
func (s *OIDCSignInService) Configure(ctx context.Context, organizationID, actorID uuid.UUID, in OIDCSignInConfigInput) (*domain.OIDCSignInConfig, error) {
	issuer := strings.TrimSpace(in.Issuer)
	clientID := strings.TrimSpace(in.ClientID)
	switch {
	case issuer == "" || len(issuer) > maxOIDCIssuerLength:
		return nil, ErrOIDCIssuerInvalid
	case clientID == "" || len(clientID) > maxOIDCClientIDLength:
		return nil, ErrOIDCClientIDInvalid
	case strings.TrimSpace(in.ClientSecret) == "" || len(in.ClientSecret) > maxOIDCClientSecretLength:
		return nil, ErrOIDCClientSecretInvalid
	}
	if err := oauth.CheckOIDCIssuer(issuer); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOIDCIssuerInvalid, err)
	}
	if _, err := s.client.Discover(ctx, issuer); err != nil {
		return nil, err
	}

	sealed, err := s.sealer.SealOIDCClientSecret(organizationID, in.ClientSecret)
	if err != nil {
		return nil, fmt.Errorf("sealing the client secret: %w", err)
	}
	if err := s.configs.Put(ctx, &domain.OIDCSignInConfig{
		OrganizationID:     organizationID,
		Issuer:             issuer,
		ClientID:           clientID,
		SealedClientSecret: sealed,
		UpdatedBy:          actorID,
	}); err != nil {
		return nil, err
	}
	return s.configs.Get(ctx, organizationID)
}

// RemoveConfiguration removes the organization's configuration, after which
// its users cannot start or complete an OIDC sign-in.
func (s *OIDCSignInService) RemoveConfiguration(ctx context.Context, organizationID uuid.UUID) error {
	return s.configs.Delete(ctx, organizationID)
}

// OIDCSignInStart is a started sign-in: the browser goes to
// AuthorizationURL, and the client keeps Flow to send back with the code and
// state the provider redirects with, before ExpiresAt.
type OIDCSignInStart struct {
	AuthorizationURL string
	Flow             string
	ExpiresAt        time.Time
}

// oidcSignInFlow is the state of one sign-in, sealed into the flow value.
type oidcSignInFlow struct {
	OrganizationID uuid.UUID `json:"organizationId"`
	State          string    `json:"state"`
	Nonce          string    `json:"nonce"`
	CodeVerifier   string    `json:"codeVerifier"`
	Issuer         string    `json:"issuer"`
	ClientID       string    `json:"clientId"`
	RedirectURI    string    `json:"redirectUri"`
	IssuedAt       int64     `json:"issuedAt"`
}

// randomFlowValue returns 32 random bytes, base64url without padding: 43
// characters, a valid PKCE code verifier (RFC 7636, section 4.1).
func randomFlowValue() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// StartSignIn starts a sign-in through the organization's provider. An
// organization that has no configuration, cannot be read or is not active
// answers domain.ErrOIDCSignInNotConfigured. An error reading the provider's
// configuration document wraps oauth.ErrOIDCProviderUnavailable.
func (s *OIDCSignInService) StartSignIn(ctx context.Context, organizationID uuid.UUID) (*OIDCSignInStart, error) {
	cfg, err := s.configs.Get(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	org, err := s.orgs.GetByID(organizationID)
	if err != nil || org == nil || !org.IsActive {
		if err != nil {
			log.Printf("OIDC sign-in: organization %s not read: %v", organizationID, err)
		}
		return nil, domain.ErrOIDCSignInNotConfigured
	}
	provider, err := s.client.Discover(ctx, cfg.Issuer)
	if err != nil {
		return nil, err
	}

	now := s.now()
	flow := oidcSignInFlow{
		OrganizationID: organizationID,
		Issuer:         cfg.Issuer,
		ClientID:       cfg.ClientID,
		RedirectURI:    s.redirectURI,
		IssuedAt:       now.Unix(),
	}
	for _, v := range []*string{&flow.State, &flow.Nonce, &flow.CodeVerifier} {
		if *v, err = randomFlowValue(); err != nil {
			return nil, err
		}
	}
	payload, err := json.Marshal(flow)
	if err != nil {
		return nil, fmt.Errorf("encoding the sign-in flow: %w", err)
	}
	sealed, err := s.sealer.SealOIDCSignInFlow(organizationID, payload)
	if err != nil {
		return nil, fmt.Errorf("sealing the sign-in flow: %w", err)
	}
	authURL, err := provider.AuthorizationURL(oauth.AuthorizationRequest{
		ClientID:      cfg.ClientID,
		RedirectURI:   s.redirectURI,
		State:         flow.State,
		Nonce:         flow.Nonce,
		CodeChallenge: oauth.CodeChallengeS256(flow.CodeVerifier),
	})
	if err != nil {
		return nil, err
	}
	return &OIDCSignInStart{
		AuthorizationURL: authURL,
		Flow:             organizationID.String() + "." + sealed,
		ExpiresAt:        now.Add(oidcSignInFlowLifetime),
	}, nil
}

// openFlow reads a flow value: the organization's id, a period, and the flow
// sealed to that organization.
func (s *OIDCSignInService) openFlow(value string) (*oidcSignInFlow, error) {
	orgPart, sealed, ok := strings.Cut(value, ".")
	if !ok {
		return nil, errors.New("the flow has no organization")
	}
	organizationID, err := uuid.Parse(orgPart)
	if err != nil {
		return nil, errors.New("the flow's organization is not a UUID")
	}
	payload, err := s.sealer.OpenOIDCSignInFlow(organizationID, sealed)
	if err != nil {
		return nil, errors.New("the flow does not open")
	}
	var flow oidcSignInFlow
	if err := json.Unmarshal(payload, &flow); err != nil {
		return nil, errors.New("the flow is not readable")
	}
	if flow.OrganizationID != organizationID || flow.State == "" || flow.Nonce == "" || flow.CodeVerifier == "" {
		return nil, errors.New("the flow is incomplete")
	}
	return &flow, nil
}

// CompleteSignIn completes the sign-in flow started with the code and state
// the provider redirected with. It returns the signed-in user, after
// recording the sign-in in the user's last_login_at. A refused sign-in
// returns an *OIDCSignInRefusal. An error reaching the provider wraps
// oauth.ErrOIDCProviderUnavailable.
func (s *OIDCSignInService) CompleteSignIn(ctx context.Context, code, state, flowValue string) (*domain.User, error) {
	flow, err := s.openFlow(flowValue)
	if err != nil {
		return nil, &OIDCSignInRefusal{Reason: OIDCSignInRefusedFlowInvalid, cause: err}
	}
	refuse := func(reason string, userID uuid.UUID, cause error) error {
		return &OIDCSignInRefusal{Reason: reason, OrganizationID: flow.OrganizationID, UserID: userID, cause: cause}
	}

	now := s.now()
	issued := time.Unix(flow.IssuedAt, 0)
	switch {
	case now.Sub(issued) > oidcSignInFlowLifetime:
		return nil, refuse(OIDCSignInRefusedFlowExpired, uuid.Nil, nil)
	case issued.After(now.Add(time.Minute)):
		return nil, refuse(OIDCSignInRefusedFlowInvalid, uuid.Nil, errors.New("the flow starts in the future"))
	case state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(flow.State)) != 1:
		return nil, refuse(OIDCSignInRefusedStateMismatch, uuid.Nil, nil)
	}

	cfg, err := s.configs.Get(ctx, flow.OrganizationID)
	if errors.Is(err, domain.ErrOIDCSignInNotConfigured) {
		return nil, refuse(OIDCSignInRefusedProviderChanged, uuid.Nil, err)
	}
	if err != nil {
		return nil, err
	}
	if cfg.Issuer != flow.Issuer || cfg.ClientID != flow.ClientID {
		return nil, refuse(OIDCSignInRefusedProviderChanged, uuid.Nil, nil)
	}
	org, err := s.orgs.GetByID(flow.OrganizationID)
	if err != nil || org == nil || !org.IsActive {
		return nil, refuse(OIDCSignInRefusedOrganizationInactive, uuid.Nil, err)
	}
	secret, err := s.sealer.OpenOIDCClientSecret(flow.OrganizationID, cfg.SealedClientSecret)
	if err != nil {
		return nil, fmt.Errorf("opening the organization's client secret: %w", err)
	}

	provider, err := s.client.Discover(ctx, cfg.Issuer)
	if err != nil {
		return nil, err
	}
	rawIDToken, err := s.client.ExchangeCode(ctx, provider, cfg.ClientID, secret, code, flow.CodeVerifier, flow.RedirectURI)
	switch {
	case errors.Is(err, oauth.ErrAuthorizationCodeRefused):
		return nil, refuse(OIDCSignInRefusedCodeRefused, uuid.Nil, err)
	case errors.Is(err, oauth.ErrIDTokenRefused):
		return nil, refuse(OIDCSignInRefusedIDTokenInvalid, uuid.Nil, err)
	case err != nil:
		return nil, err
	}
	identity, err := s.client.VerifyIDToken(ctx, provider, cfg.ClientID, rawIDToken, flow.Nonce)
	switch {
	case errors.Is(err, oauth.ErrIDTokenRefused):
		return nil, refuse(OIDCSignInRefusedIDTokenInvalid, uuid.Nil, err)
	case err != nil:
		return nil, err
	}
	email := strings.TrimSpace(identity.Email)
	if email == "" || !identity.EmailVerified {
		return nil, refuse(OIDCSignInRefusedEmailNotVerified, uuid.Nil, nil)
	}

	// The user is looked up among the users of the flow's organization only:
	// a provider configured for one organization never signs in a user of
	// another, whatever email it asserts.
	members, err := s.users.GetByOrganization(flow.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("reading the organization's users: %w", err)
	}
	matched := make([]*domain.User, 0)
	for _, member := range members {
		if member.OrganizationID == flow.OrganizationID && strings.EqualFold(strings.TrimSpace(member.Email), email) {
			matched = append(matched, member)
		}
	}
	switch len(matched) {
	case 0:
		return nil, refuse(OIDCSignInRefusedNoUser, uuid.Nil, nil)
	case 1:
	default:
		return nil, refuse(OIDCSignInRefusedAmbiguousUser, uuid.Nil, nil)
	}
	// The organization's list does not carry deleted_at; the user's own row
	// does, and CanHoldSession reads it.
	user, err := s.users.GetByID(matched[0].ID)
	if err != nil {
		return nil, fmt.Errorf("reading the user: %w", err)
	}
	if user.OrganizationID != flow.OrganizationID {
		return nil, refuse(OIDCSignInRefusedNoUser, uuid.Nil, nil)
	}
	if !user.CanHoldSession() {
		return nil, refuse(OIDCSignInRefusedAccountInactive, user.ID, nil)
	}

	// Only last_login_at and updated_at are written, as a password sign-in
	// writes them.
	at := now.UTC()
	if err := s.users.UpdateLastLogin(user.ID, at); err != nil {
		log.Printf("OIDC sign-in: last_login_at of user %s not written: %v", user.ID, err)
	} else {
		user.LastLoginAt = &at
		user.UpdatedAt = at
	}
	return user, nil
}
