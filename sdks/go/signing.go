package aimsdk

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// KeyPair represents an Ed25519 public/private key pair
type KeyPair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

// GenerateKeyPair generates a new Ed25519 keypair
func GenerateKeyPair() (*KeyPair, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate keypair: %w", err)
	}

	return &KeyPair{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}, nil
}

// NewKeyPairFromPrivateKey creates a KeyPair from an existing private key.
// Supports both 32-byte seed format and 64-byte (seed+public) format for compatibility.
func NewKeyPairFromPrivateKey(privateKeyBytes []byte) (*KeyPair, error) {
	var privateKey ed25519.PrivateKey
	var publicKey ed25519.PublicKey

	switch len(privateKeyBytes) {
	case ed25519.SeedSize: // 32 bytes - seed only
		privateKey = ed25519.NewKeyFromSeed(privateKeyBytes)
		publicKey = privateKey.Public().(ed25519.PublicKey)

	case ed25519.PrivateKeySize: // 64 bytes - seed + public key
		privateKey = ed25519.PrivateKey(privateKeyBytes)
		publicKey = privateKey.Public().(ed25519.PublicKey)

	default:
		return nil, fmt.Errorf("invalid private key length: expected 32 or 64 bytes, got %d", len(privateKeyBytes))
	}

	return &KeyPair{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}, nil
}

// NewKeyPairFromBase64 creates a KeyPair from base64-encoded private key string.
// Supports both 32-byte and 64-byte private key formats.
func NewKeyPairFromBase64(privateKeyBase64 string) (*KeyPair, error) {
	privateKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 private key: %w", err)
	}

	return NewKeyPairFromPrivateKey(privateKeyBytes)
}

// Sign signs a message with the private key and returns base64-encoded signature
func (kp *KeyPair) Sign(message string) (string, error) {
	if kp.PrivateKey == nil {
		return "", fmt.Errorf("private key is nil")
	}

	messageBytes := []byte(message)
	signature := ed25519.Sign(kp.PrivateKey, messageBytes)

	return base64.StdEncoding.EncodeToString(signature), nil
}

// Verify verifies a signature against a message using the public key
func (kp *KeyPair) Verify(message string, signatureBase64 string) (bool, error) {
	if kp.PublicKey == nil {
		return false, fmt.Errorf("public key is nil")
	}

	signature, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %w", err)
	}

	messageBytes := []byte(message)
	return ed25519.Verify(kp.PublicKey, messageBytes, signature), nil
}

// PublicKeyBase64 returns the public key as base64-encoded string
func (kp *KeyPair) PublicKeyBase64() string {
	return base64.StdEncoding.EncodeToString(kp.PublicKey)
}

// PrivateKeyBase64 returns the private key as base64-encoded string
func (kp *KeyPair) PrivateKeyBase64() string {
	return base64.StdEncoding.EncodeToString(kp.PrivateKey)
}

// SeedBase64 returns only the 32-byte seed portion of the private key as base64
func (kp *KeyPair) SeedBase64() string {
	seed := kp.PrivateKey.Seed()
	return base64.StdEncoding.EncodeToString(seed)
}

// SignPayload signs a JSON payload and returns base64-encoded signature
// This is used for signing registration and verification requests
func (kp *KeyPair) SignPayload(payload map[string]interface{}) (string, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	return kp.Sign(string(payloadJSON))
}

// VerificationResult represents the result of an action verification request
type VerificationResult struct {
	Success        bool                   `json:"success"`
	Verified       bool                   `json:"verified"`
	Message        string                 `json:"message"`
	TrustScore     float64                `json:"trust_score"`
	VerificationID string                 `json:"verification_id"`
	Details        map[string]interface{} `json:"details,omitempty"`
}

// SignMessage is a convenience method that signs a message using the client's keypair
func (c *Client) SignMessage(message string) (string, error) {
	if c.keyPair == nil {
		return "", fmt.Errorf("client has no keypair configured")
	}

	return c.keyPair.Sign(message)
}

// VerifyAction sends a verification request to the AIM backend
// This allows the agent to verify an action before executing it
func (c *Client) VerifyAction(ctx context.Context, actionType string, resource string, context map[string]interface{}) (*VerificationResult, error) {
	if c.keyPair == nil {
		return nil, fmt.Errorf("client has no keypair configured")
	}

	// Create verification payload
	payload := map[string]interface{}{
		"action_type": actionType,
		"resource":    resource,
		"context":     context,
		"timestamp":   fmt.Sprintf("%d", nowUnixMillis()),
	}

	// Sign the payload
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	signature, err := c.keyPair.Sign(string(payloadJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to sign payload: %w", err)
	}

	// Prepare request body
	reqBody := map[string]interface{}{
		"action_type": actionType,
		"resource":    resource,
		"context":     context,
		"signature":   signature,
		"public_key":  c.keyPair.PublicKeyBase64(),
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send verification request
	url := fmt.Sprintf("%s/api/v1/agents/%s/verify", c.config.APIURL, c.config.AgentID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.APIKey))

	resp, err := c.reporter.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("verification request failed with status: %d", resp.StatusCode)
	}

	var result VerificationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// nowUnixMillis returns current Unix timestamp in milliseconds
func nowUnixMillis() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
