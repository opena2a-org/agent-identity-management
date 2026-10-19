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
	"unicode"

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

	// maxOIDCProvisioningDomains is the most email domains a configuration
	// provisions users at.
	maxOIDCProvisioningDomains = 20
	// maxProvisionedEmailLength and maxProvisionedNameLength are the lengths
	// of users.email and users.name (VARCHAR(255)).
	maxProvisionedEmailLength = 255
	maxProvisionedNameLength  = 255
	// provisionedUserProvider is the users.provider of a provisioned user.
	provisionedUserProvider = "oidc"
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
	OIDCSignInRefusedNoUser               = "no_user"               // no user of the organization has the email, and the configuration provisions none
	OIDCSignInRefusedAmbiguousUser        = "ambiguous_user"        // more than one user of the organization has the email
	OIDCSignInRefusedAccountInactive      = "account_inactive"      // the user may not hold a session

	// The two reasons of a configuration that provisions users, for an email
	// that no user of the organization has.
	OIDCSignInRefusedDomainNotProvisioned     = "domain_not_provisioned"      // the email is not an address at one of the configuration's domains
	OIDCSignInRefusedEmailInOtherOrganization = "email_in_other_organization" // a user of another organization has the email
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
	// ErrOIDCProvisioningRoleInvalid: provisioning is enabled with a role
	// other than viewer, member or manager.
	ErrOIDCProvisioningRoleInvalid = fmt.Errorf("%w: provisioning.role", ErrOIDCSignInConfigInvalid)
	// ErrOIDCProvisioningDomainsInvalid: provisioning is enabled with no
	// email domain, with more than 20, or with one that is not a domain name.
	ErrOIDCProvisioningDomainsInvalid = fmt.Errorf("%w: provisioning.emailDomains", ErrOIDCSignInConfigInvalid)
)

// oidcProvisionedRoles are the roles a provisioned user may be given. A
// provider's assertion of an email never makes its holder an administrator.
var oidcProvisionedRoles = map[domain.UserRole]bool{
	domain.RoleViewer:  true,
	domain.RoleMember:  true,
	domain.RoleManager: true,
}

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
// reads the users of one organization, records a sign-in, and, for a
// configuration that provisions users, looks an email up in the other
// organizations and creates a user. The service creates a user only of the
// organization a sign-in is for; the interface has no method that changes the
// organization of a user.
type OIDCSignInUsers interface {
	GetByOrganization(orgID uuid.UUID) ([]*domain.User, error)
	GetByID(id uuid.UUID) (*domain.User, error)
	UpdateLastLogin(id uuid.UUID, at time.Time) error
	EmailInUseOutsideOrganization(email string, organizationID uuid.UUID) (bool, error)
	Create(user *domain.User) error
}

// OIDCSignInOrganizations reads an organization.
type OIDCSignInOrganizations interface {
	GetByID(id uuid.UUID) (*domain.Organization, error)
}

// OIDCSignInService signs users in through their organization's OpenID
// Connect provider with the authorization code flow and PKCE. The server
// keeps nothing between the start of a sign-in and its callback: the state,
// the nonce and the code verifier travel sealed in the flow value the client
// holds. A sign-in ends as a user of the organization whose provider issued
// the ID token, matched by the token's verified email among that
// organization's users only. It creates a user only when that organization's
// configuration provisions users and none of them has the email, and then only
// a user of that organization.
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
	// Provisioning is stored only when it is enabled.
	Provisioning OIDCProvisioningInput
	// RequireProviderSignIn refuses a password sign-in to the organization's
	// users who are not administrators.
	RequireProviderSignIn bool
}

// OIDCProvisioningInput is the just-in-time provisioning of a configuration.
type OIDCProvisioningInput struct {
	Enabled bool
	// Role is viewer, member or manager; empty is viewer.
	Role string
	// EmailDomains are 1 to 20 domain names, such as example.com.
	EmailDomains []string
}

// Configure stores in as the organization's configuration, in place of any
// earlier one, with the client secret sealed to the organization. It refuses
// an issuer, client id or client secret that breaks its rule with
// ErrOIDCIssuerInvalid, ErrOIDCClientIDInvalid or ErrOIDCClientSecretInvalid,
// an enabled provisioning whose role or domains break their rule with
// ErrOIDCProvisioningRoleInvalid or ErrOIDCProvisioningDomainsInvalid, and an
// issuer whose configuration document cannot be read or names another issuer
// with an error that wraps oauth.ErrOIDCProviderUnavailable.
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
	provisioning, err := checkOIDCProvisioning(in.Provisioning)
	if err != nil {
		return nil, err
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
		OrganizationID:        organizationID,
		Issuer:                issuer,
		ClientID:              clientID,
		SealedClientSecret:    sealed,
		Provisioning:          provisioning,
		RequireProviderSignIn: in.RequireProviderSignIn,
		UpdatedBy:             actorID,
	}); err != nil {
		return nil, err
	}
	return s.configs.Get(ctx, organizationID)
}

// RequiresProviderSignIn reports whether the organization's configuration
// requires its users who are not administrators to sign in through the
// provider. An organization without a configuration requires none. An error
// reading the configuration is returned, with false.
func (s *OIDCSignInService) RequiresProviderSignIn(ctx context.Context, organizationID uuid.UUID) (bool, error) {
	cfg, err := s.configs.Get(ctx, organizationID)
	if errors.Is(err, domain.ErrOIDCSignInNotConfigured) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return cfg.RequireProviderSignIn, nil
}

// checkOIDCProvisioning returns the provisioning to store for in. A
// provisioning that is not enabled stores none, whatever role or domains it
// names. One that is enabled stores its role, or viewer when it names none,
// and its domains in lower case without a final dot, each once, in the order
// given.
func checkOIDCProvisioning(in OIDCProvisioningInput) (domain.OIDCProvisioning, error) {
	if !in.Enabled {
		return domain.OIDCProvisioning{}, nil
	}
	role := domain.UserRole(strings.TrimSpace(in.Role))
	if role == "" {
		role = domain.RoleViewer
	}
	if !oidcProvisionedRoles[role] {
		return domain.OIDCProvisioning{}, ErrOIDCProvisioningRoleInvalid
	}
	if len(in.EmailDomains) == 0 || len(in.EmailDomains) > maxOIDCProvisioningDomains {
		return domain.OIDCProvisioning{}, ErrOIDCProvisioningDomainsInvalid
	}
	domains := make([]string, 0, len(in.EmailDomains))
	seen := make(map[string]bool, len(in.EmailDomains))
	for _, d := range in.EmailDomains {
		name, ok := normalizeEmailDomain(d)
		if !ok {
			return domain.OIDCProvisioning{}, ErrOIDCProvisioningDomainsInvalid
		}
		if !seen[name] {
			seen[name] = true
			domains = append(domains, name)
		}
	}
	return domain.OIDCProvisioning{Enabled: true, Role: role, EmailDomains: domains}, nil
}

// normalizeEmailDomain returns d in lower case, without surrounding space or
// a final dot, and whether that is a domain name: at most 253 characters, two
// or more labels, and each label 1 to 63 ASCII letters, digits or hyphens
// that neither starts nor ends with a hyphen. A wildcard, an address and a
// name of one label are not.
func normalizeEmailDomain(d string) (string, bool) {
	name := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(d)), ".")
	if name == "" || len(name) > 253 {
		return "", false
	}
	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		return "", false
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
				return "", false
			}
		}
	}
	return name, true
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

// OIDCSignInResult is a completed sign-in.
type OIDCSignInResult struct {
	// User is the user signed in.
	User *domain.User
	// Provisioned is set when the sign-in created User.
	Provisioned bool
}

// CompleteSignIn completes the sign-in flow started with the code and state
// the provider redirected with. It returns the signed-in user, after
// recording the sign-in in the user's last_login_at, and whether the sign-in
// created the user. A refused sign-in returns an *OIDCSignInRefusal. An error
// reaching the provider wraps oauth.ErrOIDCProviderUnavailable.
func (s *OIDCSignInService) CompleteSignIn(ctx context.Context, code, state, flowValue string) (*OIDCSignInResult, error) {
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
	matched, err := s.usersWithEmail(flow.OrganizationID, email)
	if err != nil {
		return nil, err
	}
	provisioned := false
	// A stored role that Configure would refuse provisions no one.
	if len(matched) == 0 && cfg.Provisioning.Enabled && oidcProvisionedRoles[cfg.Provisioning.Role] {
		created, reason, err := s.provision(flow.OrganizationID, cfg.Provisioning, identity, email, now)
		switch {
		case reason != "":
			return nil, refuse(reason, uuid.Nil, nil)
		case err != nil:
			// A concurrent first sign-in with the email may have created the
			// user first, so the organization's users are read again.
			again, readErr := s.usersWithEmail(flow.OrganizationID, email)
			if readErr != nil || len(again) == 0 {
				return nil, err
			}
			matched = again
		default:
			matched, provisioned = []*domain.User{created}, true
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
	return &OIDCSignInResult{User: user, Provisioned: provisioned}, nil
}

// usersWithEmail returns the users of the organization whose email is email,
// compared without regard to case.
func (s *OIDCSignInService) usersWithEmail(organizationID uuid.UUID, email string) ([]*domain.User, error) {
	members, err := s.users.GetByOrganization(organizationID)
	if err != nil {
		return nil, fmt.Errorf("reading the organization's users: %w", err)
	}
	matched := make([]*domain.User, 0)
	for _, member := range members {
		if member.OrganizationID == organizationID && strings.EqualFold(strings.TrimSpace(member.Email), email) {
			matched = append(matched, member)
		}
	}
	return matched, nil
}

// provision creates a user of the organization for the verified email that a
// sign-in through the organization's provider asserts and no user of the
// organization has. The user has p's role, is active, and has the email in
// lower case and the ID token's name. The reason of a refusal is returned,
// and nothing is created, when the email is not an address at one of p's
// domains or a user of another organization has it.
func (s *OIDCSignInService) provision(organizationID uuid.UUID, p domain.OIDCProvisioning, identity *oauth.IDTokenIdentity, email string, now time.Time) (*domain.User, string, error) {
	address := strings.ToLower(email)
	if !provisionsAddress(p, address) {
		return nil, OIDCSignInRefusedDomainNotProvisioned, nil
	}
	inUse, err := s.users.EmailInUseOutsideOrganization(address, organizationID)
	if err != nil {
		return nil, "", fmt.Errorf("looking the email up in other organizations: %w", err)
	}
	if inUse {
		return nil, OIDCSignInRefusedEmailInOtherOrganization, nil
	}

	at := now.UTC()
	id := uuid.New()
	user := &domain.User{
		ID:             id,
		OrganizationID: organizationID,
		Email:          address,
		Name:           provisionedName(identity.Name, address),
		Role:           p.Role,
		Provider:       provisionedUserProvider,
		// users.provider_id is unique per provider across every organization,
		// and a provider's subject is unique only per issuer, so the user's
		// own id stands in for it.
		ProviderID: id.String(),
		Status:     domain.UserStatusActive,
		ApprovedAt: &at,
		CreatedAt:  at,
		UpdatedAt:  at,
	}
	if err := s.users.Create(user); err != nil {
		return nil, "", fmt.Errorf("creating the provisioned user: %w", err)
	}
	log.Printf("OIDC sign-in: user %s provisioned in organization %s with role %s", user.ID, organizationID, user.Role)
	return user, "", nil
}

// provisionsAddress reports whether p creates a user for address, an email in
// lower case: at most 255 bytes, one @ after a nonempty local part, no space
// or control character, and a domain that is one of p's domains exactly. A
// subdomain of one of them is not.
func provisionsAddress(p domain.OIDCProvisioning, address string) bool {
	if len(address) > maxProvisionedEmailLength || strings.Count(address, "@") != 1 {
		return false
	}
	for _, r := range address {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	local, domainName, _ := strings.Cut(address, "@")
	if local == "" {
		return false
	}
	for _, d := range p.EmailDomains {
		if domainName == d {
			return true
		}
	}
	return false
}

// provisionedName is the name of a provisioned user: the ID token's name, or
// the address when the token has none, cut to 255 characters.
func provisionedName(name, address string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = address
	}
	if r := []rune(name); len(r) > maxProvisionedNameLength {
		name = string(r[:maxProvisionedNameLength])
	}
	return name
}
