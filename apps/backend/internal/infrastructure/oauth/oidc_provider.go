package oauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrOIDCProviderUnavailable is wrapped by every error that leaves the
// provider's answer unknown or unusable: the request failed, the provider
// answered a server error, or its document could not be read.
var ErrOIDCProviderUnavailable = errors.New("the identity provider could not be used")

// ErrAuthorizationCodeRefused is wrapped by the error of a code exchange the
// token endpoint refused with a client error (RFC 6749, section 5.2).
var ErrAuthorizationCodeRefused = errors.New("the identity provider refused the authorization code")

// ErrIDTokenRefused is wrapped by every refusal of an ID token.
var ErrIDTokenRefused = errors.New("ID token refused")

// ErrOIDCIssuerMismatch is wrapped, beside ErrOIDCProviderUnavailable, by the
// error of a configuration document that names another issuer than the one
// it was read for.
var ErrOIDCIssuerMismatch = errors.New("the configuration document names another issuer")

const (
	// maxOIDCAnswerBytes bounds every answer read from a provider.
	maxOIDCAnswerBytes = 1 << 20
	// idTokenLeeway is the clock skew allowed when the exp and iat of an ID
	// token are compared with this server's clock.
	idTokenLeeway = 30 * time.Second
	// minRSAKeyBits is the smallest RSA key an ID token signature is
	// verified with.
	minRSAKeyBits = 2048
	// oidcScope is the scope of every authorization request.
	oidcScope = "openid email profile"
)

// idTokenAlgorithms are the signature algorithms an ID token may be signed
// with. HMAC algorithms and "none" are not among them.
var idTokenAlgorithms = []string{"RS256", "RS384", "RS512", "PS256", "PS384", "PS512", "ES256", "ES384", "ES512"}

// OIDCProvider is what discovery read from an OpenID Connect provider's
// configuration document (OpenID Connect Discovery 1.0, section 3).
type OIDCProvider struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	JWKSURI                           string   `json:"jwks_uri"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
}

// OIDCClient is the relying party side of OpenID Connect sign-in with the
// authorization code flow: it reads a provider's configuration document,
// redeems an authorization code at the provider's token endpoint, and
// verifies the ID token the provider answers with. Every request it sends goes
// through the HTTP client it is given.
type OIDCClient struct {
	httpClient *http.Client
	now        func() time.Time
}

// NewOIDCClient returns a client that sends its requests through httpClient.
// A server passes the egress client, which refuses internal addresses and
// follows no redirect.
func NewOIDCClient(httpClient *http.Client) *OIDCClient {
	return &OIDCClient{httpClient: httpClient, now: time.Now}
}

// CheckOIDCIssuer reports whether issuer can name an OpenID Connect provider:
// an absolute https URL with a host, and with no user information, query or
// fragment (OpenID Connect Discovery 1.0, section 2).
func CheckOIDCIssuer(issuer string) error {
	u, err := url.Parse(issuer)
	if err != nil {
		return fmt.Errorf("the issuer is not a URL")
	}
	if u.Scheme != "https" || u.Host == "" || u.Hostname() == "" {
		return fmt.Errorf("the issuer must be an https URL with a host")
	}
	if u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return fmt.Errorf("the issuer must not carry user information, a query or a fragment")
	}
	return nil
}

// checkProviderEndpoint reports whether an endpoint a configuration document
// names is an absolute https URL with a host and no user information.
func checkProviderEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("is not an https URL with a host")
	}
	return nil
}

// Discover reads the configuration document of the provider issuer names.
// The document must name issuer as its issuer, exactly, and name https
// authorization, token and key set endpoints.
func (c *OIDCClient) Discover(ctx context.Context, issuer string) (*OIDCProvider, error) {
	if err := CheckOIDCIssuer(issuer); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOIDCProviderUnavailable, err)
	}
	var p OIDCProvider
	if err := c.getJSON(ctx, strings.TrimSuffix(issuer, "/")+"/.well-known/openid-configuration", &p); err != nil {
		return nil, fmt.Errorf("%w: reading the configuration document: %v", ErrOIDCProviderUnavailable, err)
	}
	if p.Issuer != issuer {
		return nil, fmt.Errorf("%w: %w: %q", ErrOIDCProviderUnavailable, ErrOIDCIssuerMismatch, p.Issuer)
	}
	for _, endpoint := range []struct{ name, value string }{
		{"authorization_endpoint", p.AuthorizationEndpoint},
		{"token_endpoint", p.TokenEndpoint},
		{"jwks_uri", p.JWKSURI},
	} {
		if err := checkProviderEndpoint(endpoint.value); err != nil {
			return nil, fmt.Errorf("%w: the configuration document's %s %v", ErrOIDCProviderUnavailable, endpoint.name, err)
		}
	}
	return &p, nil
}

// AuthorizationRequest is one authorization code request with a PKCE code
// challenge (RFC 7636, method S256).
type AuthorizationRequest struct {
	ClientID      string
	RedirectURI   string
	State         string
	Nonce         string
	CodeChallenge string
}

// AuthorizationURL returns the URL of the provider's authorization endpoint
// that asks for req, with the scope openid email profile. Parameters the
// endpoint URL already carries are kept.
func (p *OIDCProvider) AuthorizationURL(req AuthorizationRequest) (string, error) {
	u, err := url.Parse(p.AuthorizationEndpoint)
	if err != nil {
		return "", fmt.Errorf("%w: the authorization endpoint is not a URL", ErrOIDCProviderUnavailable)
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", req.ClientID)
	q.Set("redirect_uri", req.RedirectURI)
	q.Set("scope", oidcScope)
	q.Set("state", req.State)
	q.Set("nonce", req.Nonce)
	q.Set("code_challenge", req.CodeChallenge)
	q.Set("code_challenge_method", "S256")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// CodeChallengeS256 returns the S256 code challenge of a PKCE code verifier
// (RFC 7636, section 4.2).
func CodeChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// usesClientSecretPost reports whether the client authenticates at the token
// endpoint with client_secret_post: only when the provider lists the methods
// it supports and lists client_secret_post but not client_secret_basic, the
// default (OpenID Connect Discovery 1.0, section 3).
func (p *OIDCProvider) usesClientSecretPost() bool {
	basic, post := false, false
	for _, m := range p.TokenEndpointAuthMethodsSupported {
		switch m {
		case "client_secret_basic":
			basic = true
		case "client_secret_post":
			post = true
		}
	}
	return post && !basic
}

// ExchangeCode redeems an authorization code at the provider's token endpoint
// with the PKCE code verifier and the redirect URI the authorization request
// named, and returns the ID token the answer carries.
func (c *OIDCClient) ExchangeCode(ctx context.Context, p *OIDCProvider, clientID, clientSecret, code, codeVerifier, redirectURI string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", codeVerifier)
	post := p.usesClientSecretPost()
	if post {
		form.Set("client_id", clientID)
		form.Set("client_secret", clientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("%w: building the token request: %v", ErrOIDCProviderUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if !post {
		// RFC 6749, section 2.3.1: each part form-encoded before Basic encoding.
		req.SetBasicAuth(url.QueryEscape(clientID), url.QueryEscape(clientSecret))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: the token request failed: %v", ErrOIDCProviderUnavailable, err)
	}
	defer resp.Body.Close()
	body, err := readOIDCAnswer(resp.Body)
	if err != nil {
		return "", fmt.Errorf("%w: reading the token answer: %v", ErrOIDCProviderUnavailable, err)
	}

	switch {
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		// The error code only (RFC 6749, section 5.2); its description is the
		// provider's text and is not repeated.
		var refusal struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &refusal)
		return "", fmt.Errorf("%w: the token endpoint answered %d %q", ErrAuthorizationCodeRefused, resp.StatusCode, refusal.Error)
	case resp.StatusCode != http.StatusOK:
		return "", fmt.Errorf("%w: the token endpoint answered %d", ErrOIDCProviderUnavailable, resp.StatusCode)
	}

	var answer struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return "", fmt.Errorf("%w: the token answer is not JSON", ErrOIDCProviderUnavailable)
	}
	if answer.IDToken == "" {
		return "", fmt.Errorf("%w: the token answer carries no id_token", ErrIDTokenRefused)
	}
	return answer.IDToken, nil
}

// IDTokenIdentity is what a verified ID token asserts about the user who
// signed in.
type IDTokenIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

// idTokenClaims are the claims of an ID token this client reads.
type idTokenClaims struct {
	jwt.RegisteredClaims
	Nonce           string          `json:"nonce"`
	AuthorizedParty string          `json:"azp"`
	Email           string          `json:"email"`
	EmailVerified   json.RawMessage `json:"email_verified"`
	Name            string          `json:"name"`
}

// VerifyIDToken verifies an ID token the provider issued to clientID in
// answer to the authorization request that carried nonce (OpenID Connect Core
// 1.0, section 3.1.3.7). The token must be signed with one of
// idTokenAlgorithms by a key of the provider's key set; name the provider's
// issuer exactly; have clientID among its audiences, and as its authorized
// party when it names one or has more than one audience; carry exp and iat,
// with exp not passed and iat not ahead of this server's clock beyond a 30
// second leeway; carry a subject; and carry nonce. Any other token is refused
// with an error that wraps ErrIDTokenRefused. An error reading the key set
// wraps ErrOIDCProviderUnavailable.
func (c *OIDCClient) VerifyIDToken(ctx context.Context, p *OIDCProvider, clientID, rawIDToken, nonce string) (*IDTokenIdentity, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods(idTokenAlgorithms),
		jwt.WithIssuer(p.Issuer),
		jwt.WithAudience(clientID),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(idTokenLeeway),
		jwt.WithTimeFunc(c.now),
	)
	claims := &idTokenClaims{}
	_, err := parser.ParseWithClaims(rawIDToken, claims, func(token *jwt.Token) (interface{}, error) {
		var keys jsonWebKeySet
		if err := c.getJSON(ctx, p.JWKSURI, &keys); err != nil {
			return nil, fmt.Errorf("%w: reading the key set: %v", ErrOIDCProviderUnavailable, err)
		}
		return keys.verificationKey(token)
	})
	if err != nil {
		if errors.Is(err, ErrOIDCProviderUnavailable) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", ErrIDTokenRefused, err)
	}

	if claims.IssuedAt == nil {
		return nil, fmt.Errorf("%w: the token carries no iat", ErrIDTokenRefused)
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("%w: the token carries no subject", ErrIDTokenRefused)
	}
	if nonce == "" || subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(nonce)) != 1 {
		return nil, fmt.Errorf("%w: the token's nonce is not the authorization request's", ErrIDTokenRefused)
	}
	if (len(claims.Audience) > 1 || claims.AuthorizedParty != "") && claims.AuthorizedParty != clientID {
		return nil, fmt.Errorf("%w: the token's authorized party is not this client", ErrIDTokenRefused)
	}

	return &IDTokenIdentity{
		Subject:       claims.Subject,
		Email:         claims.Email,
		EmailVerified: emailVerified(claims.EmailVerified),
		Name:          claims.Name,
	}, nil
}

// emailVerified reads the email_verified claim: the JSON true, or the string
// "true" some providers send. Anything else, absent included, is false.
func emailVerified(raw json.RawMessage) bool {
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		return b
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s == "true"
	}
	return false
}

// getJSON reads the JSON document at target with a GET request.
func (c *OIDCClient) getJSON(ctx context.Context, target string, v interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return fmt.Errorf("building the request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the provider answered %d", resp.StatusCode)
	}
	body, err := readOIDCAnswer(resp.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("the answer is not the JSON document expected")
	}
	return nil
}

// readOIDCAnswer reads an answer body of at most maxOIDCAnswerBytes.
func readOIDCAnswer(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxOIDCAnswerBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxOIDCAnswerBytes {
		return nil, fmt.Errorf("the answer is larger than %d bytes", maxOIDCAnswerBytes)
	}
	return body, nil
}

// jsonWebKey is one key of a JSON Web Key Set (RFC 7517) as this client reads
// it: an RSA or an elliptic curve public key.
type jsonWebKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

type jsonWebKeySet struct {
	Keys []jsonWebKey `json:"keys"`
}

// keyTypeOf returns the key type a signature algorithm verifies with.
func keyTypeOf(alg string) string {
	switch {
	case strings.HasPrefix(alg, "RS"), strings.HasPrefix(alg, "PS"):
		return "RSA"
	case strings.HasPrefix(alg, "ES"):
		return "EC"
	}
	return ""
}

// verificationKey returns the public key of the set that verifies token: the
// one signing key of the token's algorithm's type whose kid is the token's
// kid, or, when the token names no kid, the one such key in the set. A key
// whose use is not sig, or whose alg is not the token's, is passed over.
func (s jsonWebKeySet) verificationKey(token *jwt.Token) (interface{}, error) {
	alg := token.Method.Alg()
	kid, _ := token.Header["kid"].(string)
	var match []jsonWebKey
	for _, k := range s.Keys {
		if k.Kty != keyTypeOf(alg) || (k.Use != "" && k.Use != "sig") || (k.Alg != "" && k.Alg != alg) {
			continue
		}
		if kid != "" && k.Kid != kid {
			continue
		}
		match = append(match, k)
	}
	switch len(match) {
	case 0:
		return nil, fmt.Errorf("the key set has no %s key for the token", alg)
	case 1:
		return match[0].publicKey(alg)
	default:
		return nil, fmt.Errorf("the key set has more than one %s key for the token", alg)
	}
}

// publicKey decodes the key for a signature with alg.
func (k jsonWebKey) publicKey(alg string) (interface{}, error) {
	switch k.Kty {
	case "RSA":
		return k.rsaPublicKey()
	case "EC":
		return k.ecPublicKey(alg)
	}
	return nil, fmt.Errorf("unsupported key type %q", k.Kty)
}

// decodeKeyMember decodes a base64url key member, without padding as RFC 7518
// writes it, or with the padding some key sets carry.
func decodeKeyMember(v string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(v, "="))
}

func (k jsonWebKey) rsaPublicKey() (*rsa.PublicKey, error) {
	n, err := decodeKeyMember(k.N)
	if err != nil || len(n) == 0 {
		return nil, fmt.Errorf("the RSA key's modulus is not base64url")
	}
	e, err := decodeKeyMember(k.E)
	if err != nil || len(e) == 0 || len(e) > 4 {
		return nil, fmt.Errorf("the RSA key's exponent is not a base64url integer of at most 4 bytes")
	}
	exponent := new(big.Int).SetBytes(e).Int64()
	if exponent < 3 || exponent%2 == 0 {
		return nil, fmt.Errorf("the RSA key's exponent is not an odd integer of at least 3")
	}
	key := &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(exponent)}
	if key.N.BitLen() < minRSAKeyBits {
		return nil, fmt.Errorf("the RSA key is shorter than %d bits", minRSAKeyBits)
	}
	return key, nil
}

func (k jsonWebKey) ecPublicKey(alg string) (*ecdsa.PublicKey, error) {
	var curve elliptic.Curve
	var want string
	switch alg {
	case "ES256":
		curve, want = elliptic.P256(), "P-256"
	case "ES384":
		curve, want = elliptic.P384(), "P-384"
	case "ES512":
		curve, want = elliptic.P521(), "P-521"
	default:
		return nil, fmt.Errorf("no curve for %s", alg)
	}
	if k.Crv != want {
		return nil, fmt.Errorf("the key's curve is not %s, the curve of %s", want, alg)
	}
	size := (curve.Params().BitSize + 7) / 8
	x, errX := decodeKeyMember(k.X)
	y, errY := decodeKeyMember(k.Y)
	if errX != nil || errY != nil || len(x) != size || len(y) != size {
		return nil, fmt.Errorf("the EC key's coordinates are not %d-byte base64url values", size)
	}
	point := append(append([]byte{0x04}, x...), y...)
	key, err := ecdsa.ParseUncompressedPublicKey(curve, point)
	if err != nil {
		return nil, fmt.Errorf("the EC key is not a point of %s", want)
	}
	return key, nil
}
