package middleware

import (
	"log"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// Labels of the credential routes: the routes that hand out a credential, set
// one the caller then holds, or choose where webhook or alert deliveries go.
// A refusal at one of them is recorded as credential_mint_refused under the
// route's label, by the route's own check (refuseIfFamilyRevoked and
// refuseUnlessLiveSignIn in the handlers package) and by AuthMiddleware.
const (
	CredentialRouteSDKDownload            = "sdk_download"
	CredentialRouteDeviceApprove          = "device_approve"
	CredentialRouteSDKRecover             = "sdk_recover"
	CredentialRouteAPIKeyCreate           = "api_key_create"
	CredentialRouteAgentSDKDownload       = "agent_sdk_download"
	CredentialRouteAgentCredentials       = "agent_credentials"
	CredentialRouteAgentRotateCredentials = "agent_rotate_credentials"
	CredentialRouteAgentKeysUpdate        = "agent_keys_update"
	CredentialRouteSecretRotate           = "secret_rotate"
	CredentialRouteWebhookCreate          = "webhook_create"
	CredentialRouteWebhookUpdate          = "webhook_update"
	CredentialRouteWebhookRotateSecret    = "webhook_rotate_secret"
	CredentialRouteAlertChannelCreate     = "alert_channel_create"
	CredentialRouteAlertChannelUpdate     = "alert_channel_update"
)

// CredentialRefusedSessionRevoked is the cause recorded when a credential
// route refuses an access token whose session (token family) was revoked.
const CredentialRefusedSessionRevoked = "sessionRevoked"

// CredentialRoute is a credential route as the server mounts it: the method,
// the full path pattern, and the label a refusal there is recorded under.
type CredentialRoute struct {
	Method string
	Path   string
	Label  string
}

// credentialRoutes is every credential route the server mounts. The server's
// census of its mounted routes (cmd/server) fails when a route here does not
// record a revoked session's request, or a route not here does.
var credentialRoutes = []CredentialRoute{
	{fiber.MethodPost, "/api/v1/oauth/device/approve", CredentialRouteDeviceApprove},
	{fiber.MethodPost, "/api/v1/auth/sdk/recover", CredentialRouteSDKRecover},
	{fiber.MethodGet, "/api/v1/sdk/download", CredentialRouteSDKDownload},
	{fiber.MethodPost, "/api/v1/agents/:id/rotate-credentials", CredentialRouteAgentRotateCredentials},
	{fiber.MethodPut, "/api/v1/agents/:id/keys", CredentialRouteAgentKeysUpdate},
	{fiber.MethodGet, "/api/v1/agents/:id/sdk", CredentialRouteAgentSDKDownload},
	{fiber.MethodGet, "/api/v1/agents/:id/credentials", CredentialRouteAgentCredentials},
	{fiber.MethodPost, "/api/v1/api-keys", CredentialRouteAPIKeyCreate},
	{fiber.MethodPost, "/api/v1/secrets/namespaces/:id/rotate", CredentialRouteSecretRotate},
	{fiber.MethodPost, "/api/v1/webhooks", CredentialRouteWebhookCreate},
	{fiber.MethodPut, "/api/v1/webhooks/:id", CredentialRouteWebhookUpdate},
	{fiber.MethodPost, "/api/v1/webhooks/:id/rotate-secret", CredentialRouteWebhookRotateSecret},
	{fiber.MethodPost, "/api/v1/alert-channels", CredentialRouteAlertChannelCreate},
	{fiber.MethodPut, "/api/v1/alert-channels/:id", CredentialRouteAlertChannelUpdate},
}

// CredentialRoutes returns a copy of the credential route table.
func CredentialRoutes() []CredentialRoute {
	return append([]CredentialRoute(nil), credentialRoutes...)
}

// credentialRouteLabel returns the label of the credential route the request
// was made at, or "" for any other route. The path is compared as the router
// matches it, under the app's case sensitivity and strict routing; a HEAD
// request is compared as the GET whose handlers the router runs for it.
func credentialRouteLabel(c fiber.Ctx) string {
	cfg := c.App().Config()
	method := c.Method()
	if method == fiber.MethodHead && !cfg.DisableHeadAutoRegister {
		method = fiber.MethodGet
	}
	path := c.Path()
	if !cfg.StrictRouting && len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	for _, route := range credentialRoutes {
		if route.Method == method && fiber.RoutePatternMatch(path, route.Path, cfg) {
			return route.Label
		}
	}
	return ""
}

// CredentialRefusal is one refused request at a credential route: the
// route's label, why it was refused, and the acting access token's user,
// organization, session (token family) and id.
type CredentialRefusal struct {
	Route  string
	Cause  string
	UserID uuid.UUID
	OrgID  uuid.UUID
	Family string
	JTI    string
}

// RecordCredentialRefusal records a refused request at a credential route:
// one SECURITY line and, when audit is set and the user and organization are
// known, one credential_mint_refused audit row. Both carry identifiers, the
// route and the cause, and never a token. nil-safe on audit, including a nil
// *application.AuditService.
func RecordCredentialRefusal(c fiber.Ctx, audit auth.AuditLogger, r CredentialRefusal) {
	ip, ua := c.IP(), c.Get("User-Agent")
	log.Printf("SECURITY %s route=%s cause=%s user=%s org=%s family=%s jti=%s ip=%s ua=%q", domain.AuditActionCredentialMintRefused, r.Route, r.Cause, r.UserID, r.OrgID, r.Family, r.JTI, ip, ua)
	// A handler built without an audit service passes a nil *AuditService.
	if s, ok := audit.(*application.AuditService); ok && s == nil {
		audit = nil
	}
	if audit == nil || r.UserID == uuid.Nil || r.OrgID == uuid.Nil {
		return
	}
	meta := map[string]interface{}{"familyId": r.Family, "jti": r.JTI, "route": r.Route, "cause": r.Cause}
	if err := audit.LogAction(c.Context(), r.OrgID, r.UserID, domain.AuditActionCredentialMintRefused, "user", r.UserID, ip, ua, meta); err != nil {
		log.Printf("%s: audit row for family %s not written: %v", r.Route, r.Family, err)
	}
}

// recordRevokedSessionAtCredentialRoute records a request AuthMiddleware
// refuses as revoked when it was made at a credential route with an access
// token whose session (token family) is revoked: one SECURITY line and one
// credential_mint_refused audit row in the JWTService's audit log, cause
// sessionRevoked, as the route's own check records the refusals it makes.
// The family is read again without the cache, as the route reads it; when the
// store does not answer nothing is recorded. A request at any other route, a
// token with no family and a token revoked by its own id alone record
// nothing.
func recordRevokedSessionAtCredentialRoute(c fiber.Ctx, jwtService *auth.JWTService, claims *auth.JWTClaims) {
	family := claims.AccessFamilyID()
	if family == "" {
		return
	}
	route := credentialRouteLabel(c)
	if route == "" {
		return
	}
	if revoked, known := jwtService.CheckFamilyRevoked(c.Context(), family); !revoked || !known {
		return
	}
	userID, _ := uuid.Parse(claims.UserID)
	orgID, _ := uuid.Parse(claims.OrganizationID)
	RecordCredentialRefusal(c, jwtService.AuditLogger(), CredentialRefusal{
		Route:  route,
		Cause:  CredentialRefusedSessionRevoked,
		UserID: userID,
		OrgID:  orgID,
		Family: family,
		JTI:    claims.ID,
	})
}
