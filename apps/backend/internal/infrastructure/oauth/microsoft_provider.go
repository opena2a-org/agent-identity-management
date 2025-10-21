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
	microsoftAuthURL  = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
	microsoftTokenURL = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	microsoftUserURL  = "https://graph.microsoft.com/v1.0/me"
)

type MicrosoftProvider struct {
	clientID     string
	clientSecret string
	redirectURI  string
	tenantID     string
	httpClient   *http.Client
}

func NewMicrosoftProvider(clientID, clientSecret, redirectURI, tenantID string) *MicrosoftProvider {
	if tenantID == "" {
		tenantID = "common"
	}
	return &MicrosoftProvider{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURI:  redirectURI,
		tenantID:     tenantID,
		httpClient:   &http.Client{},
	}
}

func (p *MicrosoftProvider) GetAuthURL(state string) string {
	authURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize", p.tenantID)

	params := url.Values{}
	params.Add("client_id", p.clientID)
	params.Add("redirect_uri", p.redirectURI)
	params.Add("response_type", "code")
	params.Add("scope", "openid email profile User.Read")
	params.Add("state", state)
	params.Add("response_mode", "query")

	return fmt.Sprintf("%s?%s", authURL, params.Encode())
}

func (p *MicrosoftProvider) ExchangeCode(ctx context.Context, code string) (accessToken, refreshToken string, expiresIn int, err error) {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", p.tenantID)

	data := url.Values{}
	data.Set("code", code)
	data.Set("client_id", p.clientID)
	data.Set("client_secret", p.clientSecret)
	data.Set("redirect_uri", p.redirectURI)
	data.Set("grant_type", "authorization_code")
	data.Set("scope", "openid email profile User.Read")

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
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", "", 0, fmt.Errorf("failed to decode token response: %w", err)
	}

	return tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.ExpiresIn, nil
}

func (p *MicrosoftProvider) GetUserProfile(ctx context.Context, accessToken string) (*domain.OAuthProfile, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", microsoftUserURL, nil)
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

	var msUser struct {
		ID                string `json:"id"`
		Mail              string `json:"mail"`
		UserPrincipalName string `json:"userPrincipalName"`
		DisplayName       string `json:"displayName"`
		GivenName         string `json:"givenName"`
		Surname           string `json:"surname"`
		PreferredLanguage string `json:"preferredLanguage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&msUser); err != nil {
		return nil, fmt.Errorf("failed to decode user profile: %w", err)
	}

	// Microsoft doesn't provide email verification status in basic profile
	// We'll assume emails from Microsoft are verified
	emailVerified := true

	// Use mail if available, otherwise use userPrincipalName
	email := msUser.Mail
	if email == "" {
		email = msUser.UserPrincipalName
	}

	// Convert to raw profile map
	rawProfile := map[string]interface{}{
		"id":                  msUser.ID,
		"mail":                msUser.Mail,
		"user_principal_name": msUser.UserPrincipalName,
		"display_name":        msUser.DisplayName,
		"given_name":          msUser.GivenName,
		"surname":             msUser.Surname,
		"preferred_language":  msUser.PreferredLanguage,
	}

	return &domain.OAuthProfile{
		ProviderUserID: msUser.ID,
		Email:          email,
		EmailVerified:  emailVerified,
		FirstName:      msUser.GivenName,
		LastName:       msUser.Surname,
		FullName:       msUser.DisplayName,
		PictureURL:     "", // Microsoft Graph requires separate call for photo
		Locale:         msUser.PreferredLanguage,
		RawProfile:     rawProfile,
	}, nil
}

func (p *MicrosoftProvider) GetProviderName() domain.OAuthProvider {
	return domain.OAuthProviderMicrosoft
}

// OAuth provider interface compliance check (currently disabled in production)
// var _ application.OAuthProvider = (*MicrosoftProvider)(nil)
