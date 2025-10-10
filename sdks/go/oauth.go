package aimsdk

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/microsoft"
)

// OAuthProvider represents supported OAuth providers
type OAuthProvider string

const (
	OAuthProviderGoogle    OAuthProvider = "google"
	OAuthProviderMicrosoft OAuthProvider = "microsoft"
	OAuthProviderOkta      OAuthProvider = "okta"
)

// OAuthConfig holds OAuth configuration
type OAuthConfig struct {
	Provider     OAuthProvider
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// OAuthToken represents OAuth token response
type OAuthToken struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Expiry       time.Time
}

// GetOAuthConfig returns OAuth2 config for the specified provider
func GetOAuthConfig(provider OAuthProvider, redirectURL string) (*oauth2.Config, error) {
	var endpoint oauth2.Endpoint
	var scopes []string

	switch provider {
	case OAuthProviderGoogle:
		endpoint = google.Endpoint
		scopes = []string{"openid", "profile", "email"}
		return &oauth2.Config{
			ClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
			ClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
			RedirectURL:  redirectURL,
			Scopes:       scopes,
			Endpoint:     endpoint,
		}, nil

	case OAuthProviderMicrosoft:
		endpoint = microsoft.AzureADEndpoint("common")
		scopes = []string{"openid", "profile", "email"}
		return &oauth2.Config{
			ClientID:     getEnv("MICROSOFT_CLIENT_ID", ""),
			ClientSecret: getEnv("MICROSOFT_CLIENT_SECRET", ""),
			RedirectURL:  redirectURL,
			Scopes:       scopes,
			Endpoint:     endpoint,
		}, nil

	case OAuthProviderOkta:
		// For Okta, we need the domain
		oktaDomain := getEnv("OKTA_DOMAIN", "")
		if oktaDomain == "" {
			return nil, fmt.Errorf("OKTA_DOMAIN environment variable is required")
		}
		endpoint = oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("https://%s/oauth2/v1/authorize", oktaDomain),
			TokenURL: fmt.Sprintf("https://%s/oauth2/v1/token", oktaDomain),
		}
		scopes = []string{"openid", "profile", "email"}
		return &oauth2.Config{
			ClientID:     getEnv("OKTA_CLIENT_ID", ""),
			ClientSecret: getEnv("OKTA_CLIENT_SECRET", ""),
			RedirectURL:  redirectURL,
			Scopes:       scopes,
			Endpoint:     endpoint,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported OAuth provider: %s", provider)
	}
}

// StartOAuthFlow initiates OAuth flow and returns authorization URL
func StartOAuthFlow(config *oauth2.Config) (authURL string, state string) {
	// Generate random state for CSRF protection
	state = generateRandomState()
	authURL = config.AuthCodeURL(state, oauth2.AccessTypeOffline)
	return authURL, state
}

// StartCallbackServer starts HTTP server to receive OAuth callback
// Returns the authorization code or an error
func StartCallbackServer(ctx context.Context, port int, expectedState string) (string, error) {
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// Extract code and state from query parameters
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		errorParam := r.URL.Query().Get("error")

		// Check for errors
		if errorParam != "" {
			errorDesc := r.URL.Query().Get("error_description")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(fmt.Sprintf("<h1>❌ Authorization Failed</h1><p>%s: %s</p>", errorParam, errorDesc)))
			errChan <- fmt.Errorf("OAuth error: %s - %s", errorParam, errorDesc)
			return
		}

		// Verify state to prevent CSRF
		if state != expectedState {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("<h1>❌ Authorization Failed</h1><p>Invalid state parameter (CSRF protection)</p>"))
			errChan <- fmt.Errorf("invalid state parameter")
			return
		}

		// Check if code was received
		if code == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("<h1>❌ Authorization Failed</h1><p>No authorization code received</p>"))
			errChan <- fmt.Errorf("no authorization code received")
			return
		}

		// Success
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<h1>✅ Authorization Successful!</h1><p>You can close this window and return to your application.</p>"))
		codeChan <- code
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start server in goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("failed to start callback server: %w", err)
		}
	}()

	// Wait for callback or timeout
	select {
	case code := <-codeChan:
		// Shutdown server gracefully
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
		return code, nil

	case err := <-errChan:
		// Shutdown server gracefully
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
		return "", err

	case <-ctx.Done():
		// Context canceled or timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
		return "", fmt.Errorf("OAuth flow canceled or timeout")
	}
}

// ExchangeCodeForToken exchanges authorization code for access token
func ExchangeCodeForToken(ctx context.Context, config *oauth2.Config, code string) (*OAuthToken, error) {
	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code for token: %w", err)
	}

	return &OAuthToken{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		Expiry:       token.Expiry,
	}, nil
}

// OpenBrowser attempts to open the URL in the default browser
func OpenBrowser(url string) error {
	// Note: Browser opening is simplified to avoid exec dependencies
	// Users can implement their own browser opening logic if needed
	// For production use, consider using: exec.Command(cmd, args...).Start()
	fmt.Printf("🔐 Please open this URL in your browser:\n%s\n", url)
	return nil
}

// generateRandomState generates a random state string for CSRF protection
func generateRandomState() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based state
		return fmt.Sprintf("state_%d", time.Now().UnixNano())
	}
	return base64.URLEncoding.EncodeToString(b)
}

// getEnv retrieves environment variable with a fallback default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
