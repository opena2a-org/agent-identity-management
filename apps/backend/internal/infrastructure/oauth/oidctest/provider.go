// Package oidctest is an OpenID Connect provider for tests, written from
// OpenID Connect Core 1.0 (authorization code flow), OpenID Connect Discovery
// 1.0 and RFC 7636 (PKCE). It is an httptest TLS server that listens on
// 127.0.0.1 and serves:
//
//   - GET /.well-known/openid-configuration, its configuration document;
//   - GET /jwks, its key set: one RSA signing key;
//   - GET /authorize, which signs in the identity SignInAs set without asking
//     and redirects with a code and the state to the redirect URI the client
//     registered (Authorize registers it), refusing a request whose
//     redirect_uri is another;
//   - POST /token, which redeems a code once, for the client it registered,
//     with client_secret_basic, the code verifier of the request's S256
//     challenge and the request's redirect_uri, and answers an ID token.
//
// The ID token names the server's URL as issuer, the client as audience, the
// signed-in identity as subject, email and email_verified, and the nonce of
// the authorization request. EditIDToken and SigningKey change the token it
// issues, so a test can make it refuse-worthy.
package oidctest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Identity is the user the authorization endpoint signs in.
type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

// Provider is a running provider. Its fields other than the server are read
// when a request arrives, so a test sets them before it drives a sign-in.
type Provider struct {
	Server       *httptest.Server
	Issuer       string
	ClientID     string
	ClientSecret string
	// KeyID is the kid of the signing key in the key set and of each token.
	KeyID string

	// EditIDToken, when set, changes the claims of each ID token the token
	// endpoint issues before it is signed.
	EditIDToken func(claims jwt.MapClaims)
	// SigningKey, when set, signs each ID token in place of the key the key
	// set holds; the token still names KeyID.
	SigningKey *rsa.PrivateKey

	key *rsa.PrivateKey

	mu       sync.Mutex
	identity Identity
	// redirectURI is the redirect URI the client registered; Authorize
	// registers it.
	redirectURI string
	grants      map[string]grant
	// Redeemed counts the codes the token endpoint redeemed.
	redeemed int
}

// grant is one authorization the authorization endpoint gave.
type grant struct {
	identity      Identity
	clientID      string
	redirectURI   string
	nonce         string
	codeChallenge string
}

// NewProvider starts a provider for t, registers one client with a client id
// and secret made up for the run, and stops the provider when t ends.
func NewProvider(t testing.TB) *Provider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating the provider's signing key: %v", err)
	}
	p := &Provider{
		ClientID:     "aim-test-client",
		ClientSecret: "not-a-real-client-secret",
		KeyID:        "test-key-1",
		key:          key,
		grants:       map[string]grant{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", p.serveConfiguration)
	mux.HandleFunc("GET /jwks", p.serveKeySet)
	mux.HandleFunc("GET /authorize", p.serveAuthorize)
	mux.HandleFunc("POST /token", p.serveToken)
	p.Server = httptest.NewTLSServer(mux)
	p.Issuer = p.Server.URL
	t.Cleanup(p.Server.Close)
	return p
}

// Client returns an HTTP client that trusts the provider's certificate.
func (p *Provider) Client() *http.Client {
	return p.Server.Client()
}

// SignInAs sets the identity the authorization endpoint signs in.
func (p *Provider) SignInAs(id Identity) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.identity = id
}

// Redeemed returns how many codes the token endpoint has redeemed.
func (p *Provider) Redeemed() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.redeemed
}

// Authorize sends the browser's request for authorizationURL to the
// authorization endpoint and returns the code and state of the redirect it
// answers, without following it. redirectURI is the URI the redirect must go
// to: the client registers it first, so the endpoint answers 400 to a request
// whose redirect_uri is another.
func (p *Provider) Authorize(t testing.TB, authorizationURL, redirectURI string) (code, state string) {
	t.Helper()
	p.mu.Lock()
	p.redirectURI = redirectURI
	p.mu.Unlock()
	client := p.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Get(authorizationURL)
	if err != nil {
		t.Fatalf("authorization request: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("authorization endpoint answered %d, not 302", resp.StatusCode)
	}
	location, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("authorization redirect: %v", err)
	}
	if got := location.Scheme + "://" + location.Host + location.Path; got != redirectURI {
		t.Fatalf("authorization redirect goes to %s, not %s", got, redirectURI)
	}
	return location.Query().Get("code"), location.Query().Get("state")
}

// MintIDToken signs claims with the provider's signing key and kid.
func (p *Provider) MintIDToken(claims jwt.MapClaims) (string, error) {
	return p.sign(claims, p.key)
}

func (p *Provider) sign(claims jwt.MapClaims, key *rsa.PrivateKey) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = p.KeyID
	return token.SignedString(key)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (p *Provider) serveConfiguration(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"issuer":                                p.Issuer,
		"authorization_endpoint":                p.Server.URL + "/authorize",
		"token_endpoint":                        p.Server.URL + "/token",
		"jwks_uri":                              p.Server.URL + "/jwks",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic"},
		"code_challenge_methods_supported":      []string{"S256"},
	})
}

func (p *Provider) serveKeySet(w http.ResponseWriter, _ *http.Request) {
	pub := p.key.PublicKey
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"keys": []map[string]string{{
			"kty": "RSA",
			"use": "sig",
			"alg": "RS256",
			"kid": p.KeyID,
			"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
		}},
	})
}

func (p *Provider) serveAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	p.mu.Lock()
	registered := p.redirectURI
	p.mu.Unlock()
	// The redirect_uri must be exactly the registered one (OpenID Connect
	// Core 1.0, 3.1.2.1), and the redirect goes to the registered value.
	if q.Get("response_type") != "code" || q.Get("client_id") != p.ClientID || q.Get("code_challenge_method") != "S256" ||
		q.Get("code_challenge") == "" || registered == "" || q.Get("redirect_uri") != registered {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	code := randomValue()
	p.mu.Lock()
	p.grants[code] = grant{
		identity:      p.identity,
		clientID:      q.Get("client_id"),
		redirectURI:   registered,
		nonce:         q.Get("nonce"),
		codeChallenge: q.Get("code_challenge"),
	}
	p.mu.Unlock()

	separator := "?"
	if strings.Contains(registered, "?") {
		separator = "&"
	}
	location := registered + separator + "code=" + url.QueryEscape(code) + "&state=" + url.QueryEscape(q.Get("state"))
	http.Redirect(w, r, location, http.StatusFound)
}

func (p *Provider) serveToken(w http.ResponseWriter, r *http.Request) {
	user, pass, ok := r.BasicAuth()
	clientID, errID := url.QueryUnescape(user)
	secret, errSecret := url.QueryUnescape(pass)
	if !ok || errID != nil || errSecret != nil || clientID != p.ClientID ||
		subtle.ConstantTimeCompare([]byte(secret), []byte(p.ClientSecret)) != 1 {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_client"})
		return
	}
	if err := r.ParseForm(); err != nil || r.PostForm.Get("grant_type") != "authorization_code" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	code := r.PostForm.Get("code")
	p.mu.Lock()
	g, found := p.grants[code]
	delete(p.grants, code) // a code is redeemed once, whatever the outcome
	p.mu.Unlock()
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if !found || g.clientID != clientID || g.redirectURI != r.PostForm.Get("redirect_uri") ||
		base64.RawURLEncoding.EncodeToString(sum[:]) != g.codeChallenge {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
		return
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":            p.Issuer,
		"sub":            g.identity.Subject,
		"aud":            p.ClientID,
		"exp":            now.Add(5 * time.Minute).Unix(),
		"iat":            now.Unix(),
		"nonce":          g.nonce,
		"email":          g.identity.Email,
		"email_verified": g.identity.EmailVerified,
		"name":           g.identity.Name,
	}
	if p.EditIDToken != nil {
		p.EditIDToken(claims)
	}
	key := p.key
	if p.SigningKey != nil {
		key = p.SigningKey
	}
	idToken, err := p.sign(claims, key)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "server_error"})
		return
	}
	p.mu.Lock()
	p.redeemed++
	p.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"access_token": randomValue(),
		"token_type":   "Bearer",
		"expires_in":   300,
		"id_token":     idToken,
	})
}

// randomValue returns 16 random bytes, base64url.
func randomValue() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("oidctest: reading random bytes: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
