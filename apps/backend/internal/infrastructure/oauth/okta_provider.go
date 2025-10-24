package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/opena2a/identity/backend/internal/domain"
)

type OktaProvider struct {
	domain       string
	clientID     string
	clientSecret string
	redirectURI  string
	httpClient   *http.Client
}

func NewOktaProvider(domain, clientID, clientSecret, redirectURI string) *OktaProvider {
	return &OktaProvider{
		domain:       domain,
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		httpClient:   &http.Client{},
	}
}

func (p *OktaProvider) GetAuthURL(state string) string {
	authURL := fmt.Sprintf("https://%s/oauth2/default/authorize", p.domain)

	params := url.Values{}
	params.Add("client_id", p.clientID)
	params.Add("redirect_uri", p.redirectURI)
	params.Add("response_type", "code")
	params.Add("scope", "openid email profile")
	params.Add("state", state)

	return fmt.Sprintf("%s?%s", authURL, params.Encode())
}

func (p *OktaProvider) ExchangeCode(ctx context.Context, code string) (accessToken, refreshToken string, expiresIn int, err error) {
	tokenURL := fmt.Sprintf("https://%s/oauth2/default/token", p.domain)

	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", p.clientID)
	data.Set("client_secret", p.clientSecret)
	data.Set("redirect_uri", p.redirectURI)
	data.Set("grant_type", "authorization_code")

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", 0, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
		IDToken      string `json:"id_token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", "", 0, fmt.Errorf("failed to decode token response: %w", err)
	}

	return tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.ExpiresIn, nil
}

func (p *OktaProvider) GetUserProfile(ctx context.Context, accessToken string) (*domain.OAuthProfile, error) {
	userURL := fmt.Sprintf("https://%s/oauth2/default/userinfo", p.domain)

	req, err := http.NewRequestWithContext(ctx, "GET", userURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get user profile: %s", string(body))
	}

	var oktaUser struct {
		Sub               string `json:"sub"`
		Email             string `json:"email"`
		EmailVerified     bool   `json:"email_verified"`
		Name              string `json:"name"`
		GivenName         string `json:"given_name"`
		FamilyName        string `json:"family_name"`
		PreferredUsername string `json:"preferred_username"`
		Locale            string `json:"locale"`
		Picture           string `json:"picture"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&oktaUser); err != nil {
		return nil, fmt.Errorf("failed to decode user profile: %w", err)
	}

	// Convert to raw profile map
	rawProfile := map[string]interface{}{
		"sub":                oktaUser.Sub,
		"email":              oktaUser.Email,
		"email_verified":     oktaUser.EmailVerified,
		"name":               oktaUser.Name,
		"given_name":         oktaUser.GivenName,
		"family_name":        oktaUser.FamilyName,
		"preferred_username": oktaUser.PreferredUsername,
		"locale":             oktaUser.Locale,
		"picture":            oktaUser.Picture,
	}

	return &domain.OAuthProfile{
		ProviderUserID: oktaUser.Sub,
		Email:          oktaUser.Email,
		EmailVerified:  oktaUser.EmailVerified,
		FirstName:      oktaUser.GivenName,
		LastName:       oktaUser.FamilyName,
		FullName:       oktaUser.Name,
		PictureURL:     oktaUser.Picture,
		Locale:         oktaUser.Locale,
		RawProfile:     rawProfile,
	}, nil
}

func (p *OktaProvider) GetProviderName() domain.OAuthProvider {
	return domain.OAuthProviderOkta
}

// OAuth provider interface compliance check (currently disabled in production)
// var _ application.OAuthProvider = (*OktaProvider)(nil)
