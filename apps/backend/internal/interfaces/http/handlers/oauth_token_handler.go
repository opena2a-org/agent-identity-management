package handlers

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"os"
	"time"
)

// OAuthTokenHandler handles the OAuth 2.0 token endpoint (RFC 6749)
type OAuthTokenHandler struct {
	jwtService *auth.JWTService
	agentRepo  domain.AgentRepository
}

// NewOAuthTokenHandler creates a new OAuth token handler
func NewOAuthTokenHandler(jwtService *auth.JWTService, agentRepo domain.AgentRepository) *OAuthTokenHandler {
	return &OAuthTokenHandler{
		jwtService: jwtService,
		agentRepo:  agentRepo,
	}
}

// HandleTokenRequest implements POST /oauth/token per RFC 6749
// Supports grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer (RFC 7523)
func (h *OAuthTokenHandler) HandleTokenRequest(c fiber.Ctx) error {
	contentType := c.Get("Content-Type")
	if contentType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "invalid_request",
			"error_description": "Content-Type header is required",
		})
	}

	// Parse request body - support both form-urlencoded and JSON
	grantType := c.FormValue("grant_type")
	if grantType == "" {
		// Try JSON body as fallback
		var body struct {
			GrantType           string `json:"grant_type"`
			ClientID            string `json:"client_id"`
			ClientAssertion     string `json:"client_assertion"`
			ClientAssertionType string `json:"client_assertion_type"`
		}
		if err := c.Bind().JSON(&body); err == nil {
			grantType = body.GrantType
			if grantType == "" {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error":             "invalid_request",
					"error_description": "grant_type is required",
				})
			}
			return h.processTokenRequest(c, body.GrantType, body.ClientID, body.ClientAssertion, body.ClientAssertionType)
		}

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "invalid_request",
			"error_description": "grant_type is required",
		})
	}

	clientID := c.FormValue("client_id")
	clientAssertion := c.FormValue("client_assertion")
	clientAssertionType := c.FormValue("client_assertion_type")

	return h.processTokenRequest(c, grantType, clientID, clientAssertion, clientAssertionType)
}

func (h *OAuthTokenHandler) processTokenRequest(c fiber.Ctx, grantType, clientID, clientAssertion, clientAssertionType string) error {
	// Validate grant_type
	if grantType != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "unsupported_grant_type",
			"error_description": "Only urn:ietf:params:oauth:grant-type:jwt-bearer is supported",
		})
	}

	// Validate required fields for jwt-bearer grant
	if clientID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "invalid_request",
			"error_description": "client_id is required",
		})
	}

	if clientAssertion == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "invalid_request",
			"error_description": "client_assertion is required",
		})
	}

	// Validate assertion type
	if clientAssertionType != "" && clientAssertionType != "urn:ietf:params:oauth:client-assertion-type:jwt-bearer" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "invalid_request",
			"error_description": "Unsupported client_assertion_type",
		})
	}

	// Validate the JWT assertion structure (must be a valid JWT with 3 parts)
	parts := strings.Split(clientAssertion, ".")
	if len(parts) != 3 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "invalid_request",
			"error_description": "Malformed client_assertion: not a valid JWT",
		})
	}

	// Decode and validate the JWT payload
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "invalid_request",
			"error_description": "Malformed client_assertion: invalid base64 encoding",
		})
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "invalid_request",
			"error_description": "Malformed client_assertion: invalid JSON payload",
		})
	}

	// Verify the assertion's subject matches the client_id
	sub, _ := claims["sub"].(string)
	if sub != clientID {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":             "invalid_client",
			"error_description": "Assertion subject does not match client_id",
		})
	}

	// SECURITY: Verify the JWT signature against the agent's registered public key.
	// The client_id must be a registered agent UUID with an Ed25519 public key.
	agentID, err := uuid.Parse(clientID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":             "invalid_client",
			"error_description": "client_id must be a valid agent UUID",
		})
	}

	agent, err := h.agentRepo.GetByID(agentID)
	if err != nil || agent == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":             "invalid_client",
			"error_description": "Agent not found",
		})
	}

	// SECURITY: a revoked or suspended agent must not obtain a token.
	//
	// This endpoint had no status check at all, and AgentService.RevokeAgent does not
	// clear agents.public_key — revocation is expressed purely as a write to
	// agents.status. So a revoked agent that still held its private key could keep
	// minting service tokens here indefinitely, while every signature and API-key path
	// denied it. Issuance is the moment that has to enforce it; ServicePrincipalMiddleware
	// enforces the same rule at use, for tokens minted before the revocation.
	if !domain.AgentStatusPermitsAuth(agent.Status) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":             "invalid_client",
			"error_description": domain.AgentStatusDeniedMessage(agent.Status),
		})
	}

	if agent.PublicKey == nil || *agent.PublicKey == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":             "invalid_client",
			"error_description": "Agent has no registered public key",
		})
	}

	// Verify Ed25519 signature: sign(header.payload) must match the signature part
	signedContent := parts[0] + "." + parts[1]
	signatureBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":             "invalid_request",
			"error_description": "Malformed client_assertion: invalid signature encoding",
		})
	}

	publicKeyBytes, err := base64.StdEncoding.DecodeString(*agent.PublicKey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":             "server_error",
			"error_description": "Failed to decode agent public key",
		})
	}

	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":             "server_error",
			"error_description": "Agent public key has invalid size",
		})
	}

	if !ed25519.Verify(ed25519.PublicKey(publicKeyBytes), []byte(signedContent), signatureBytes) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":             "invalid_client",
			"error_description": "JWT signature verification failed",
		})
	}

	// SECURITY: validate the assertion's temporal and audience claims.
	//
	// Deliberately AFTER signature verification — before it, every claim is
	// attacker-supplied text, and acting on an unverified `aud` or `exp` decides on a
	// value nobody has vouched for.
	//
	// This endpoint previously read only `sub`. RFC 7523 §3 requires `aud` and `exp`, and
	// without them the assertion never expired: anyone who obtained one once could replay
	// it forever, against this or any other service the agent talks to. Our own SDK has
	// been signing `iss`, `sub`, `aud`, `iat` and `exp` all along
	// (sdk/typescript/src/auth/oauth.ts) — the server simply never read them, so enforcing
	// them is not a new requirement on clients that were already correct.
	if errResp := validateAssertionClaims(c, claims); errResp != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(errResp)
	}

	// Generate a service token for the authenticated agent. This is a machine
	// principal: GenerateServiceToken stamps IssuerService and leaves the role
	// empty, so the token cannot traverse a human role gate. It previously used
	// GenerateAccessToken with role="service", which passed the member gate and
	// exposed sibling agents' private keys — see GenerateServiceToken's comment.
	accessToken, err := h.jwtService.GenerateServiceToken(clientID, agent.OrganizationID.String())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":             "server_error",
			"error_description": "Failed to generate access token",
		})
	}

	return c.JSON(fiber.Map{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   3600,
	})
}

// maxAssertionLifetime bounds how far ahead an assertion's `exp` may sit.
//
// An `exp` check with no ceiling is close to no check: a client that stamps exp ten years
// out is back to an assertion that is replayable for practical purposes. Five minutes
// matches what our own SDK already issues (iat + 300).
const maxAssertionLifetime = 5 * time.Minute

// assertionClockSkew is the tolerance applied to every temporal claim, so an agent whose
// clock is a little fast or slow is not locked out.
const assertionClockSkew = 60 * time.Second

// validateAssertionClaims enforces RFC 7523 §3's temporal and audience requirements on a
// client assertion whose signature has ALREADY been verified. Returns nil when the
// assertion is acceptable, or the error body to send.
//
// Not enforced here, deliberately: `jti` single-use. RFC 7523 §3 makes replay prevention a
// MAY, and doing it properly needs a persisted, expiring set of seen ids (the shape
// A2ARequestNonceRepository already has). Bounding the lifetime shrinks the replay window
// from unlimited to at most maxAssertionLifetime; closing it entirely is tracked
// separately. State what is enforced, not what sounds complete.
func validateAssertionClaims(c fiber.Ctx, claims map[string]interface{}) fiber.Map {
	now := time.Now()

	// numericClaim reads a JWT NumericDate. encoding/json gives float64 for every number.
	numericClaim := func(name string) (time.Time, bool) {
		raw, present := claims[name]
		if !present {
			return time.Time{}, false
		}
		secs, ok := raw.(float64)
		if !ok {
			return time.Time{}, false
		}
		return time.Unix(int64(secs), 0), true
	}

	// exp is REQUIRED (RFC 7523 §3(4)). A missing exp is the defect itself, so its absence
	// must reject rather than default to "no expiry".
	exp, ok := numericClaim("exp")
	if !ok {
		return fiber.Map{
			"error":             "invalid_client",
			"error_description": "Assertion is missing a valid exp claim",
		}
	}
	if now.After(exp.Add(assertionClockSkew)) {
		return fiber.Map{
			"error":             "invalid_client",
			"error_description": "Assertion has expired",
		}
	}
	if exp.After(now.Add(maxAssertionLifetime + assertionClockSkew)) {
		return fiber.Map{
			"error":             "invalid_client",
			"error_description": "Assertion exp is too far in the future",
		}
	}

	// nbf and iat are optional; when present they must be sane. An iat far in the future is
	// the same trick as an unbounded exp wearing a different hat.
	if nbf, present := numericClaim("nbf"); present && now.Add(assertionClockSkew).Before(nbf) {
		return fiber.Map{
			"error":             "invalid_client",
			"error_description": "Assertion is not yet valid",
		}
	}
	if iat, present := numericClaim("iat"); present && now.Add(assertionClockSkew).Before(iat) {
		return fiber.Map{
			"error":             "invalid_client",
			"error_description": "Assertion iat is in the future",
		}
	}

	// aud is REQUIRED (RFC 7523 §3(3)) and must identify THIS authorization server.
	// Without it, an assertion the agent minted for some other service is accepted here.
	aud, _ := claims["aud"].(string)
	if aud == "" {
		return fiber.Map{
			"error":             "invalid_client",
			"error_description": "Assertion is missing an aud claim",
		}
	}
	if !audienceIsThisServer(c, aud) {
		return fiber.Map{
			"error":             "invalid_client",
			"error_description": "Assertion aud does not identify this server",
		}
	}

	return nil
}

// audienceIsThisServer reports whether `aud` names this authorization server.
//
// Accepts the configured AIM_BASE_URL when one is set, and otherwise the origin the
// request actually arrived on. A self-hoster who has configured nothing still gets the
// property that matters — an assertion minted for a DIFFERENT service is refused — while
// not being locked out of their own deployment for want of an env var.
func audienceIsThisServer(c fiber.Ctx, aud string) bool {
	trim := func(s string) string { return strings.TrimRight(strings.TrimSpace(s), "/") }
	got := trim(aud)
	if got == "" {
		return false
	}

	accepted := make([]string, 0, 4)
	if base := trim(os.Getenv("AIM_BASE_URL")); base != "" {
		accepted = append(accepted, base, base+"/api/v1")
	}
	if host := c.Hostname(); host != "" {
		origin := trim(c.Scheme() + "://" + host)
		accepted = append(accepted, origin, origin+"/api/v1")
	}

	for _, want := range accepted {
		if got == trim(want) {
			return true
		}
	}
	return false
}
