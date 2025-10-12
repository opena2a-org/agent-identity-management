package aimsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// RegisterOptions configures agent registration
type RegisterOptions struct {
	Name          string        // Agent name (required)
	Type          string        // Agent type: "ai_agent", "mcp_server", etc.
	OAuthProvider OAuthProvider // Optional: OAuth provider for authentication
	RedirectURL   string        // OAuth redirect URL (default: http://localhost:8080/callback)
}

// AgentRegistration holds the registration response
type AgentRegistration struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	APIKey    string `json:"api_key"`
	PublicKey string `json:"public_key"`
}

// Secure is an alias for RegisterAgent
// Registers an agent with one line of code
func (c *Client) Secure(ctx context.Context, opts RegisterOptions) (*AgentRegistration, error) {
	return c.RegisterAgent(ctx, opts)
}

// RegisterAgent registers a new agent with the AIM backend
// This method generates Ed25519 keypair, signs the request, and stores credentials securely
func (c *Client) RegisterAgent(ctx context.Context, opts RegisterOptions) (*AgentRegistration, error) {
	// Set defaults
	if opts.Type == "" {
		opts.Type = "ai_agent"
	}
	if opts.RedirectURL == "" {
		opts.RedirectURL = "http://localhost:8080/callback"
	}

	// Generate Ed25519 keypair for agent identity
	keyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("keypair generation failed: %w", err)
	}

	// Prepare registration payload
	payload := map[string]interface{}{
		"name":       opts.Name,
		"type":       opts.Type,
		"public_key": keyPair.PublicKeyBase64(),
	}

	// Sign the payload for cryptographic verification
	signature, err := keyPair.SignPayload(payload)
	if err != nil {
		return nil, fmt.Errorf("signing failed: %w", err)
	}
	payload["signature"] = signature

	// Send registration request
	var result AgentRegistration
	if err := c.post(ctx, "/api/v1/agents/register", payload, &result); err != nil {
		return nil, fmt.Errorf("registration request failed: %w", err)
	}

	// Store credentials securely in system keyring
	creds := &Credentials{
		AgentID:    result.ID,
		APIKey:     result.APIKey,
		PrivateKey: keyPair.PrivateKey,
	}

	if err := StoreCredentials(creds); err != nil {
		return nil, fmt.Errorf("credential storage failed: %w", err)
	}

	// Update client configuration with new credentials
	c.config.AgentID = result.ID
	c.config.APIKey = result.APIKey

	return &result, nil
}

// SecureWithOAuth is an alias for RegisterAgentWithOAuth
func (c *Client) SecureWithOAuth(ctx context.Context, opts RegisterOptions) (*AgentRegistration, error) {
	return c.RegisterAgentWithOAuth(ctx, opts)
}

// RegisterAgentWithOAuth registers an agent using OAuth authentication
// This method opens a browser for OAuth consent and completes the registration
func (c *Client) RegisterAgentWithOAuth(ctx context.Context, opts RegisterOptions) (*AgentRegistration, error) {
	// Get OAuth configuration
	oauthConfig, err := GetOAuthConfig(opts.OAuthProvider, opts.RedirectURL)
	if err != nil {
		return nil, fmt.Errorf("OAuth config error: %w", err)
	}

	// Start OAuth flow
	authURL, state := StartOAuthFlow(oauthConfig)
	fmt.Printf("🔐 Please visit this URL to authorize:\n%s\n", authURL)

	// Open browser automatically
	OpenBrowser(authURL)

	// Extract port from redirect URL
	port := 8080 // Default

	// Start callback server to receive OAuth code
	code, err := StartCallbackServer(ctx, port, state)
	if err != nil {
		return nil, fmt.Errorf("OAuth callback failed: %w", err)
	}

	// Exchange code for access token
	token, err := ExchangeCodeForToken(ctx, oauthConfig, code)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	// Generate Ed25519 keypair
	keyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("keypair generation failed: %w", err)
	}

	// Prepare registration payload with OAuth token
	payload := map[string]interface{}{
		"name":           opts.Name,
		"type":           opts.Type,
		"public_key":     keyPair.PublicKeyBase64(),
		"oauth_provider": string(opts.OAuthProvider),
		"oauth_token":    token.AccessToken,
	}

	// Sign the payload
	signature, err := keyPair.SignPayload(payload)
	if err != nil {
		return nil, fmt.Errorf("signing failed: %w", err)
	}
	payload["signature"] = signature

	// Send registration request with OAuth token
	var result AgentRegistration
	if err := c.postWithAuth(ctx, "/api/v1/agents/register", payload, token.AccessToken, &result); err != nil {
		return nil, fmt.Errorf("registration request failed: %w", err)
	}

	// Store credentials and OAuth token
	creds := &Credentials{
		AgentID:    result.ID,
		APIKey:     result.APIKey,
		PrivateKey: keyPair.PrivateKey,
	}

	if err := StoreCredentials(creds); err != nil {
		return nil, fmt.Errorf("credential storage failed: %w", err)
	}

	// Store OAuth token separately
	if err := StoreOAuthToken(token.AccessToken); err != nil {
		// Log warning but don't fail
		fmt.Printf("Warning: failed to store OAuth token: %v\n", err)
	}

	// Update client configuration
	c.config.AgentID = result.ID
	c.config.APIKey = result.APIKey

	return &result, nil
}

// post sends a POST request to the API
func (c *Client) post(ctx context.Context, endpoint string, data interface{}, result interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.APIURL+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// postWithAuth sends a POST request with OAuth bearer token
func (c *Client) postWithAuth(ctx context.Context, endpoint string, data interface{}, token string, result interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.APIURL+endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}
