package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// OAuthProvider represents an OAuth2 provider
type OAuthProvider string

const (
	ProviderGoogle    OAuthProvider = "google"
	ProviderMicrosoft OAuthProvider = "microsoft"
	ProviderOkta      OAuthProvider = "okta"
)

// OAuthConfig holds OAuth2 configuration
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
}

// OAuthUser represents user information from OAuth provider
type OAuthUser struct {
	ID        string
	Email     string
	Name      string
	AvatarURL string
	Provider  string
}

// OAuthService handles OAuth2 authentication
type OAuthService struct {
	configs map[OAuthProvider]*OAuthConfig
}

// NewOAuthService creates a new OAuth service
func NewOAuthService() *OAuthService {
	return &OAuthService{
		configs: map[OAuthProvider]*OAuthConfig{
			ProviderGoogle: {
				ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
				ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
				RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URI"),
				AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
				TokenURL:     "https://oauth2.googleapis.com/token",
				UserInfoURL:  "https://www.googleapis.com/oauth2/v2/userinfo",
			},
			ProviderMicrosoft: {
				ClientID:     os.Getenv("MICROSOFT_CLIENT_ID"),
				ClientSecret: os.Getenv("MICROSOFT_CLIENT_SECRET"),
				RedirectURL:  os.Getenv("MICROSOFT_REDIRECT_URI"),
				AuthURL:      "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
				TokenURL:     "https://login.microsoftonline.com/common/oauth2/v2.0/token",
				UserInfoURL:  "https://graph.microsoft.com/v1.0/me",
			},
			ProviderOkta: {
				ClientID:     os.Getenv("OKTA_CLIENT_ID"),
				ClientSecret: os.Getenv("OKTA_CLIENT_SECRET"),
				RedirectURL:  os.Getenv("OKTA_REDIRECT_URI"),
				AuthURL:      fmt.Sprintf("https://%s/oauth2/v1/authorize", os.Getenv("OKTA_DOMAIN")),
				TokenURL:     fmt.Sprintf("https://%s/oauth2/v1/token", os.Getenv("OKTA_DOMAIN")),
				UserInfoURL:  fmt.Sprintf("https://%s/oauth2/v1/userinfo", os.Getenv("OKTA_DOMAIN")),
			},
		},
	}
}

// GetAuthURL generates the OAuth2 authorization URL
func (s *OAuthService) GetAuthURL(provider OAuthProvider, state string) (string, error) {
	config, ok := s.configs[provider]
	if !ok {
		return "", fmt.Errorf("unsupported provider: %s", provider)
	}

	var scope string
	switch provider {
	case ProviderGoogle:
		scope = "openid email profile"
	case ProviderMicrosoft:
		scope = "openid email profile User.Read"
	case ProviderOkta:
		scope = "openid email profile"
	}

	url := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s",
		config.AuthURL,
		config.ClientID,
		config.RedirectURL,
		scope,
		state,
	)

	return url, nil
}

// ExchangeCode exchanges authorization code for access token
func (s *OAuthService) ExchangeCode(ctx context.Context, provider OAuthProvider, code string) (string, error) {
	config, ok := s.configs[provider]
	if !ok {
		return "", fmt.Errorf("unsupported provider: %s", provider)
	}

	data := fmt.Sprintf("client_id=%s&client_secret=%s&code=%s&redirect_uri=%s&grant_type=authorization_code",
		config.ClientID,
		config.ClientSecret,
		code,
		config.RedirectURL,
	)

	req, err := http.NewRequestWithContext(ctx, "POST", config.TokenURL, strings.NewReader(data))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token exchange failed with status: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	accessToken, ok := result["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("no access token in response")
	}

	return accessToken, nil
}

// GetUserInfo fetches user information using access token
func (s *OAuthService) GetUserInfo(ctx context.Context, provider OAuthProvider, accessToken string) (*OAuthUser, error) {
	config, ok := s.configs[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", config.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get user info failed: %d - %s", resp.StatusCode, string(body))
	}

	var userInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return s.parseUserInfo(provider, userInfo)
}

// parseUserInfo parses provider-specific user info
func (s *OAuthService) parseUserInfo(provider OAuthProvider, info map[string]interface{}) (*OAuthUser, error) {
	user := &OAuthUser{
		Provider: string(provider),
	}

	switch provider {
	case ProviderGoogle:
		user.ID = getStringField(info, "id")
		user.Email = getStringField(info, "email")
		user.Name = getStringField(info, "name")
		user.AvatarURL = getStringField(info, "picture")

	case ProviderMicrosoft:
		user.ID = getStringField(info, "id")
		user.Email = getStringField(info, "userPrincipalName")
		if user.Email == "" {
			user.Email = getStringField(info, "mail")
		}
		user.Name = getStringField(info, "displayName")

	case ProviderOkta:
		user.ID = getStringField(info, "sub")
		user.Email = getStringField(info, "email")
		user.Name = getStringField(info, "name")
	}

	if user.ID == "" || user.Email == "" {
		return nil, fmt.Errorf("missing required user fields")
	}

	return user, nil
}

// getStringField safely extracts a string field from map
func getStringField(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

// GenerateState generates a random state for CSRF protection
func GenerateState() string {
	return uuid.New().String()
}

// HashAPIKey hashes an API key using bcrypt
func HashAPIKey(key string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CompareAPIKey compares a plain API key with a hash
func CompareAPIKey(hash, key string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(key))
}
