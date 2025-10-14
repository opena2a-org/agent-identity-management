package aimsdk

import (
	"crypto/ed25519"
	"fmt"

	"github.com/zalando/go-keyring"
)

const serviceName = "aim_sdk"

// Credentials holds all stored credentials
type Credentials struct {
	AgentID    string
	APIKey     string
	PrivateKey ed25519.PrivateKey
}

// StoreCredentials saves credentials to system keyring
// On macOS: Keychain, Linux: Secret Service, Windows: Credential Locker
func StoreCredentials(creds *Credentials) error {
	// Store agent ID
	if err := keyring.Set(serviceName, "agent_id", creds.AgentID); err != nil {
		return fmt.Errorf("failed to store agent_id: %w", err)
	}

	// Store API key
	if err := keyring.Set(serviceName, "api_key", creds.APIKey); err != nil {
		return fmt.Errorf("failed to store api_key: %w", err)
	}

	// Store private key (base64 encoded)
	if len(creds.PrivateKey) > 0 {
		kp := &KeyPair{PrivateKey: creds.PrivateKey}
		encodedKey := kp.PrivateKeyBase64()
		if err := keyring.Set(serviceName, "private_key", encodedKey); err != nil {
			return fmt.Errorf("failed to store private_key: %w", err)
		}
	}

	return nil
}

// LoadCredentials retrieves credentials from system keyring
// Returns error if agent is not registered (no credentials found)
func LoadCredentials() (*Credentials, error) {
	// Load agent ID
	agentID, err := keyring.Get(serviceName, "agent_id")
	if err != nil {
		return nil, fmt.Errorf("agent not registered (no agent_id found): %w", err)
	}

	// Load API key
	apiKey, err := keyring.Get(serviceName, "api_key")
	if err != nil {
		return nil, fmt.Errorf("no api_key found: %w", err)
	}

	// Load private key (optional)
	var privateKey ed25519.PrivateKey
	privateKeyB64, err := keyring.Get(serviceName, "private_key")
	if err == nil && privateKeyB64 != "" {
		kp, err := NewKeyPairFromBase64(privateKeyB64)
		if err != nil {
			// Log warning but don't fail if private key is invalid
			privateKey = nil
		} else {
			privateKey = kp.PrivateKey
		}
	}

	return &Credentials{
		AgentID:    agentID,
		APIKey:     apiKey,
		PrivateKey: privateKey,
	}, nil
}

// ClearCredentials removes all stored credentials from system keyring
// This is useful for logout or re-registration
func ClearCredentials() error {
	keys := []string{"agent_id", "api_key", "private_key", "oauth_token"}

	var lastErr error
	for _, key := range keys {
		if err := keyring.Delete(serviceName, key); err != nil {
			// Continue deleting other keys even if one fails
			// Store last error to return
			lastErr = err
		}
	}

	return lastErr
}

// HasCredentials checks if credentials are stored in the keyring
func HasCredentials() bool {
	_, err := keyring.Get(serviceName, "agent_id")
	return err == nil
}

// GetAgentID retrieves just the agent ID from keyring
func GetAgentID() (string, error) {
	return keyring.Get(serviceName, "agent_id")
}

// GetAPIKey retrieves just the API key from keyring
func GetAPIKey() (string, error) {
	return keyring.Get(serviceName, "api_key")
}

// StoreOAuthToken stores OAuth token separately
func StoreOAuthToken(token string) error {
	return keyring.Set(serviceName, "oauth_token", token)
}

// GetOAuthToken retrieves stored OAuth token
func GetOAuthToken() (string, error) {
	return keyring.Get(serviceName, "oauth_token")
}

// ClearOAuthToken removes OAuth token from keyring
func ClearOAuthToken() error {
	return keyring.Delete(serviceName, "oauth_token")
}
