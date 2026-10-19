package oauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/oauth/oidctest"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/utils"
)

const testRedirectURI = "https://aim.example.com/auth/oidc/callback"

func discoverStub(t *testing.T) (*oidctest.Provider, *OIDCClient, *OIDCProvider) {
	t.Helper()
	stub := oidctest.NewProvider(t)
	client := NewOIDCClient(stub.Client())
	provider, err := client.Discover(context.Background(), stub.Issuer)
	require.NoError(t, err)
	return stub, client, provider
}

// validClaims are the claims of an ID token the stub could have issued to its
// client for nonce.
func validClaims(stub *oidctest.Provider, nonce string) jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"iss":            stub.Issuer,
		"sub":            "user-1",
		"aud":            stub.ClientID,
		"exp":            now.Add(5 * time.Minute).Unix(),
		"iat":            now.Unix(),
		"nonce":          nonce,
		"email":          "alice@example.com",
		"email_verified": true,
	}
}

func TestDiscoverReadsTheProvidersConfigurationDocument(t *testing.T) {
	stub, _, provider := discoverStub(t)
	assert.Equal(t, stub.Issuer, provider.Issuer)
	assert.Equal(t, stub.Server.URL+"/authorize", provider.AuthorizationEndpoint)
	assert.Equal(t, stub.Server.URL+"/token", provider.TokenEndpoint)
	assert.Equal(t, stub.Server.URL+"/jwks", provider.JWKSURI)
	assert.True(t, strings.HasPrefix(stub.Issuer, "https://127.0.0.1:"), "the stub listens on 127.0.0.1: %s", stub.Issuer)
}

// With the egress client the server passes, an issuer at a loopback address
// is refused before a connection is made: the provider runs on 127.0.0.1.
func TestDiscoverWithTheEgressClientRefusesALoopbackIssuer(t *testing.T) {
	stub := oidctest.NewProvider(t)
	_, err := NewOIDCClient(utils.NewEgressClient(5*time.Second)).Discover(context.Background(), stub.Issuer)
	assert.ErrorIs(t, err, ErrOIDCProviderUnavailable)
	assert.Contains(t, err.Error(), "destination address is not allowed (loopback)")
}

func TestCheckOIDCIssuerAcceptsOnlyAnHTTPSURLWithAHost(t *testing.T) {
	assert.NoError(t, CheckOIDCIssuer("https://idp.example.com"))
	assert.NoError(t, CheckOIDCIssuer("https://idp.example.com/oauth2/default"))
	for _, issuer := range []string{
		"http://idp.example.com",
		"idp.example.com",
		"https://",
		"https://idp.example.com?tenant=1",
		"https://idp.example.com#frag",
		"https://user@idp.example.com",
		"",
	} {
		assert.Error(t, CheckOIDCIssuer(issuer), issuer)
	}
}

// A configuration document that names another issuer is refused, so a
// provider cannot vouch for tokens of an issuer it does not serve.
func TestDiscoverRefusesADocumentThatNamesAnotherIssuer(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 "https://other.example.com",
			"authorization_endpoint": "https://other.example.com/authorize",
			"token_endpoint":         "https://other.example.com/token",
			"jwks_uri":               "https://other.example.com/jwks",
		})
	}))
	defer srv.Close()

	_, err := NewOIDCClient(srv.Client()).Discover(context.Background(), srv.URL)
	assert.ErrorIs(t, err, ErrOIDCProviderUnavailable)
}

func TestDiscoverRefusesADocumentWithAnEndpointThatIsNotHTTPS(t *testing.T) {
	var issuer string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 issuer,
			"authorization_endpoint": issuer + "/authorize",
			"token_endpoint":         "http://idp.example.com/token",
			"jwks_uri":               issuer + "/jwks",
		})
	}))
	defer srv.Close()
	issuer = srv.URL

	_, err := NewOIDCClient(srv.Client()).Discover(context.Background(), srv.URL)
	assert.ErrorIs(t, err, ErrOIDCProviderUnavailable)
	assert.Contains(t, err.Error(), "token_endpoint")
}

func TestAuthorizationURLAsksForACodeWithAnS256Challenge(t *testing.T) {
	_, _, provider := discoverStub(t)
	authURL, err := provider.AuthorizationURL(AuthorizationRequest{
		ClientID: "aim-client", RedirectURI: testRedirectURI, State: "st", Nonce: "no", CodeChallenge: CodeChallengeS256("verifier"),
	})
	require.NoError(t, err)

	u, err := url.Parse(authURL)
	require.NoError(t, err)
	q := u.Query()
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "aim-client", q.Get("client_id"))
	assert.Equal(t, testRedirectURI, q.Get("redirect_uri"))
	assert.Equal(t, "openid email profile", q.Get("scope"))
	assert.Equal(t, "st", q.Get("state"))
	assert.Equal(t, "no", q.Get("nonce"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	// RFC 7636, appendix B: the S256 challenge of its example verifier.
	assert.Equal(t, "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", CodeChallengeS256("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"))
}

// The authorization code flow end to end against the stub: the client asks
// for a code, redeems it with the PKCE verifier, and verifies the ID token.
func TestTheCodeFlowEndsWithTheIdentityTheProviderSignedIn(t *testing.T) {
	stub, client, provider := discoverStub(t)
	stub.SignInAs(oidctest.Identity{Subject: "user-1", Email: "alice@example.com", EmailVerified: true, Name: "Alice"})

	verifier := "a-code-verifier-of-at-least-forty-three-characters"
	authURL, err := provider.AuthorizationURL(AuthorizationRequest{
		ClientID: stub.ClientID, RedirectURI: testRedirectURI, State: "st", Nonce: "the-nonce", CodeChallenge: CodeChallengeS256(verifier),
	})
	require.NoError(t, err)
	code, state := stub.Authorize(t, authURL, testRedirectURI)
	assert.Equal(t, "st", state)

	idToken, err := client.ExchangeCode(context.Background(), provider, stub.ClientID, stub.ClientSecret, code, verifier, testRedirectURI)
	require.NoError(t, err)
	identity, err := client.VerifyIDToken(context.Background(), provider, stub.ClientID, idToken, "the-nonce")
	require.NoError(t, err)
	assert.Equal(t, &IDTokenIdentity{Subject: "user-1", Email: "alice@example.com", EmailVerified: true, Name: "Alice"}, identity)

	_, err = client.ExchangeCode(context.Background(), provider, stub.ClientID, stub.ClientSecret, code, verifier, testRedirectURI)
	assert.ErrorIs(t, err, ErrAuthorizationCodeRefused, "the provider redeems a code once")
}

func TestExchangeCodeWithAnotherVerifierIsRefused(t *testing.T) {
	stub, client, provider := discoverStub(t)
	authURL, err := provider.AuthorizationURL(AuthorizationRequest{
		ClientID: stub.ClientID, RedirectURI: testRedirectURI, State: "st", Nonce: "n", CodeChallenge: CodeChallengeS256("the-verifier"),
	})
	require.NoError(t, err)
	code, _ := stub.Authorize(t, authURL, testRedirectURI)

	_, err = client.ExchangeCode(context.Background(), provider, stub.ClientID, stub.ClientSecret, code, "another-verifier", testRedirectURI)
	assert.ErrorIs(t, err, ErrAuthorizationCodeRefused)
}

func TestExchangeCodeWithAServerErrorIsTheProviderUnavailable(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	provider := &OIDCProvider{Issuer: srv.URL, TokenEndpoint: srv.URL + "/token"}

	_, err := NewOIDCClient(srv.Client()).ExchangeCode(context.Background(), provider, "c", "s", "code", "v", testRedirectURI)
	assert.ErrorIs(t, err, ErrOIDCProviderUnavailable)
}

func TestExchangeCodeSendsTheSecretInTheBodyOnlyWhenTheProviderAsksForIt(t *testing.T) {
	var got url.Values
	var basicUser string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		got = r.PostForm
		basicUser, _, _ = r.BasicAuth()
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": "x"})
	}))
	defer srv.Close()
	client := NewOIDCClient(srv.Client())

	post := &OIDCProvider{TokenEndpoint: srv.URL, TokenEndpointAuthMethodsSupported: []string{"client_secret_post"}}
	_, err := client.ExchangeCode(context.Background(), post, "aim client", "not-a-real-secret", "code", "v", testRedirectURI)
	require.NoError(t, err)
	assert.Equal(t, "aim client", got.Get("client_id"))
	assert.Equal(t, "not-a-real-secret", got.Get("client_secret"))
	assert.Empty(t, basicUser)

	basic := &OIDCProvider{TokenEndpoint: srv.URL, TokenEndpointAuthMethodsSupported: []string{"client_secret_post", "client_secret_basic"}}
	_, err = client.ExchangeCode(context.Background(), basic, "aim client", "not-a-real-secret", "code", "v", testRedirectURI)
	require.NoError(t, err)
	assert.Empty(t, got.Get("client_secret"))
	assert.Equal(t, "aim+client", basicUser, "the Basic user is the form-encoded client id (RFC 6749, section 2.3.1)")
}

func TestExchangeCodeRefusesAnAnswerWithoutAnIDToken(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "x", "token_type": "Bearer"})
	}))
	defer srv.Close()
	provider := &OIDCProvider{TokenEndpoint: srv.URL}

	_, err := NewOIDCClient(srv.Client()).ExchangeCode(context.Background(), provider, "c", "s", "code", "v", testRedirectURI)
	assert.ErrorIs(t, err, ErrIDTokenRefused)
}

func TestVerifyIDTokenAcceptsATokenOfTheProviderForTheClient(t *testing.T) {
	stub, client, provider := discoverStub(t)
	token, err := stub.MintIDToken(validClaims(stub, "n-1"))
	require.NoError(t, err)

	identity, err := client.VerifyIDToken(context.Background(), provider, stub.ClientID, token, "n-1")
	require.NoError(t, err)
	assert.Equal(t, "user-1", identity.Subject)
	assert.Equal(t, "alice@example.com", identity.Email)
	assert.True(t, identity.EmailVerified)
}

// Each case changes one thing about a token the provider could have issued,
// and each is refused.
func TestVerifyIDTokenRefuses(t *testing.T) {
	stub, client, provider := discoverStub(t)
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	signed := func(edit func(jwt.MapClaims)) string {
		claims := validClaims(stub, "n-1")
		edit(claims)
		token, err := stub.MintIDToken(claims)
		require.NoError(t, err)
		return token
	}
	signedBy := func(key *rsa.PrivateKey, kid string) string {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, validClaims(stub, "n-1"))
		token.Header["kid"] = kid
		s, err := token.SignedString(key)
		require.NoError(t, err)
		return s
	}

	cases := map[string]string{
		"a wrong issuer":   signed(func(c jwt.MapClaims) { c["iss"] = "https://other.example.com" }),
		"a wrong audience": signed(func(c jwt.MapClaims) { c["aud"] = "another-client" }),
		"a wrong nonce":    signed(func(c jwt.MapClaims) { c["nonce"] = "n-2" }),
		"no nonce":         signed(func(c jwt.MapClaims) { delete(c, "nonce") }),
		"a bad signature":  signedBy(otherKey, stub.KeyID),
		"an unknown kid":   signedBy(otherKey, "unknown-key"),
		"an expired token": signed(func(c jwt.MapClaims) {
			c["exp"] = time.Now().Add(-10 * time.Minute).Unix()
			c["iat"] = time.Now().Add(-20 * time.Minute).Unix()
		}),
		"no exp":                        signed(func(c jwt.MapClaims) { delete(c, "exp") }),
		"no iat":                        signed(func(c jwt.MapClaims) { delete(c, "iat") }),
		"an iat ahead of the clock":     signed(func(c jwt.MapClaims) { c["iat"] = time.Now().Add(10 * time.Minute).Unix() }),
		"a not-before ahead":            signed(func(c jwt.MapClaims) { c["nbf"] = time.Now().Add(10 * time.Minute).Unix() }),
		"no subject":                    signed(func(c jwt.MapClaims) { delete(c, "sub") }),
		"two audiences and no azp":      signed(func(c jwt.MapClaims) { c["aud"] = []string{stub.ClientID, "another-client"} }),
		"an azp that is another client": signed(func(c jwt.MapClaims) { c["azp"] = "another-client" }),
		"a token that is not a JWT":     "not-a-jwt",
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			identity, err := client.VerifyIDToken(context.Background(), provider, stub.ClientID, token, "n-1")
			assert.ErrorIs(t, err, ErrIDTokenRefused)
			assert.Nil(t, identity)
		})
	}

	t.Run("an empty expected nonce", func(t *testing.T) {
		token := signed(func(c jwt.MapClaims) { c["nonce"] = "" })
		_, err := client.VerifyIDToken(context.Background(), provider, stub.ClientID, token, "")
		assert.ErrorIs(t, err, ErrIDTokenRefused)
	})
}

// A token whose header names an algorithm outside idTokenAlgorithms is
// refused before a key is looked up: "none", and HS256 keyed with the bytes of
// the provider's public key, the confusion an RSA verifier must not fall for.
func TestVerifyIDTokenRefusesAlgorithmsOutsideTheList(t *testing.T) {
	stub, client, provider := discoverStub(t)

	unsigned := jwt.NewWithClaims(jwt.SigningMethodNone, validClaims(stub, "n-1"))
	none, err := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	resp, err := stub.Client().Get(provider.JWKSURI)
	require.NoError(t, err)
	var set jsonWebKeySet
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&set))
	_ = resp.Body.Close()
	hmacToken := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims(stub, "n-1"))
	hmacToken.Header["kid"] = stub.KeyID
	hs256, err := hmacToken.SignedString([]byte(set.Keys[0].N))
	require.NoError(t, err)

	for name, token := range map[string]string{"none": none, "HS256": hs256} {
		_, err := client.VerifyIDToken(context.Background(), provider, stub.ClientID, token, "n-1")
		assert.ErrorIs(t, err, ErrIDTokenRefused, name)
	}
}

// keySetServer serves set as a key set, and a configuration document for
// itself.
func keySetServer(t *testing.T, set interface{}) (*httptest.Server, *OIDCProvider) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(set)
	}))
	t.Cleanup(srv.Close)
	return srv, &OIDCProvider{Issuer: srv.URL, JWKSURI: srv.URL + "/jwks"}
}

func ecClaims(issuer string) jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{"iss": issuer, "sub": "user-1", "aud": "aim-client", "exp": now.Add(time.Minute).Unix(),
		"iat": now.Unix(), "nonce": "n-1"}
}

func TestVerifyIDTokenWithAnECKey(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	public, err := key.PublicKey.Bytes()
	require.NoError(t, err)
	jwk := map[string]string{
		"kty": "EC", "crv": "P-256", "kid": "ec-1", "use": "sig",
		"x": base64.RawURLEncoding.EncodeToString(public[1:33]),
		"y": base64.RawURLEncoding.EncodeToString(public[33:]),
	}
	srv, provider := keySetServer(t, map[string]interface{}{"keys": []map[string]string{jwk}})
	client := NewOIDCClient(srv.Client())

	token := jwt.NewWithClaims(jwt.SigningMethodES256, ecClaims(srv.URL))
	token.Header["kid"] = "ec-1"
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	_, err = client.VerifyIDToken(context.Background(), provider, "aim-client", signed, "n-1")
	require.NoError(t, err)

	// The same key named as P-384 is not the key of an ES256 token.
	jwk["crv"] = "P-384"
	srv2, provider2 := keySetServer(t, map[string]interface{}{"keys": []map[string]string{jwk}})
	_, err = NewOIDCClient(srv2.Client()).VerifyIDToken(context.Background(), provider2, "aim-client", signed, "n-1")
	assert.ErrorIs(t, err, ErrIDTokenRefused)

	// A point that is not on the curve is not a key.
	jwk["crv"] = "P-256"
	jwk["y"] = jwk["x"]
	srv3, provider3 := keySetServer(t, map[string]interface{}{"keys": []map[string]string{jwk}})
	_, err = NewOIDCClient(srv3.Client()).VerifyIDToken(context.Background(), provider3, "aim-client", signed, "n-1")
	assert.ErrorIs(t, err, ErrIDTokenRefused)
}

func TestVerifyIDTokenRefusesAnRSAKeyShorterThan2048Bits(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	srv, provider := keySetServer(t, map[string]interface{}{"keys": []map[string]string{{
		"kty": "RSA", "kid": "short",
		"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e": "AQAB",
	}}})

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, ecClaims(srv.URL))
	token.Header["kid"] = "short"
	signed, err := token.SignedString(key)
	require.NoError(t, err)
	_, err = NewOIDCClient(srv.Client()).VerifyIDToken(context.Background(), provider, "aim-client", signed, "n-1")
	assert.ErrorIs(t, err, ErrIDTokenRefused)
}

func TestVerifyIDTokenWhenTheKeySetCannotBeReadIsTheProviderUnavailable(t *testing.T) {
	stub := oidctest.NewProvider(t)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	provider := &OIDCProvider{Issuer: stub.Issuer, JWKSURI: srv.URL + "/jwks"}
	token, err := stub.MintIDToken(validClaims(stub, "n-1"))
	require.NoError(t, err)

	_, err = NewOIDCClient(srv.Client()).VerifyIDToken(context.Background(), provider, stub.ClientID, token, "n-1")
	assert.ErrorIs(t, err, ErrOIDCProviderUnavailable)
	assert.NotErrorIs(t, err, ErrIDTokenRefused)
}

func TestEmailVerifiedReadsTrueOnly(t *testing.T) {
	assert.True(t, emailVerified(json.RawMessage(`true`)))
	assert.True(t, emailVerified(json.RawMessage(`"true"`)))
	assert.False(t, emailVerified(json.RawMessage(`false`)))
	assert.False(t, emailVerified(json.RawMessage(`"false"`)))
	assert.False(t, emailVerified(json.RawMessage(`1`)))
	assert.False(t, emailVerified(nil))
}
