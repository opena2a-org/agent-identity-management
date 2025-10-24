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

const (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	googleUserURL  = "https://www.googleapis.com/oauth2/v2/userinfo"
)

type GoogleProvider struct {
	clientID     string
	clientSecret string
	redirectURI  string
	httpClient   *http.Client
}

func NewGoogleProvider(clientID, clientSecret, redirectURI string) *GoogleProvider {
	return &GoogleProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		httpClient:   &http.Client{},
	}
}

func (p *GoogleProvider) GetAuthURL(state string) string {
	params := url.Values{}
	params.Add("client_id", p.clientID)
	params.Add("redirect_uri", p.redirectURI)
	params.Add("response_type", "code")
	params.Add("scope", "openid email profile")
	params.Add("state", state)
	params.Add("access_type", "offline")
	params.Add("prompt", "consent")

	return fmt.Sprintf("%s?%s", googleAuthURL, params.Encode())
}

func (p *GoogleProvider) ExchangeCode(ctx context.Context, code string) (accessToken, refreshToken string, expiresIn int, err error) {
	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", p.clientID)
	data.Set("client_secret", p.clientSecret)
	data.Set("redirect_uri", p.redirectURI)
	data.Set("grant_type", "authorization_code")

	// Debug logging (can be removed in production)
	fmt.Printf("🔍 Google OAuth Token Exchange Debug:\n")
	fmt.Printf("   URL: %s\n", googleTokenURL)
	fmt.Printf("   Client ID: %s\n", p.clientID)
	fmt.Printf("   Redirect URI: %s\n", p.redirectURI)
	codePreview := code
	if len(code) > 20 {
		codePreview = code[:20] + "..."
	}
	fmt.Printf("   Code: %s\n", codePreview)

	req, err := http.NewRequestWithContext(ctx, "POST", googleTokenURL, strings.NewReader(data.Encode()))
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
		fmt.Printf("❌ Google OAuth Error Response (Status %d):\n%s\n", resp.StatusCode, string(body))
		return "", "", 0, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", "", 0, fmt.Errorf("failed to decode token response: %w", err)
	}

	return tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.ExpiresIn, nil
}

func (p *GoogleProvider) GetUserProfile(ctx context.Context, accessToken string) (*domain.OAuthProfile, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", googleUserURL, nil)
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

	var googleUser struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		Name          string `json:"name"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Picture       string `json:"picture"`
		Locale        string `json:"locale"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return nil, fmt.Errorf("failed to decode user profile: %w", err)
	}

	// Convert to raw profile map
	rawProfile := map[string]interface{}{
		"id":             googleUser.ID,
		"email":          googleUser.Email,
		"verified_email": googleUser.VerifiedEmail,
		"name":           googleUser.Name,
		"given_name":     googleUser.GivenName,
		"family_name":    googleUser.FamilyName,
		"picture":        googleUser.Picture,
		"locale":         googleUser.Locale,
	}

	return &domain.OAuthProfile{
		ProviderUserID: googleUser.ID,
		Email:          googleUser.Email,
		EmailVerified:  googleUser.VerifiedEmail,
		FirstName:      googleUser.GivenName,
		LastName:       googleUser.FamilyName,
		FullName:       googleUser.Name,
		PictureURL:     googleUser.Picture,
		Locale:         googleUser.Locale,
		RawProfile:     rawProfile,
	}, nil
}

func (p *GoogleProvider) GetProviderName() domain.OAuthProvider {
	return domain.OAuthProviderGoogle
}

// OAuth provider interface compliance check (currently disabled in production)
// var _ application.OAuthProvider = (*GoogleProvider)(nil)
