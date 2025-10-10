package aimsdk

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
)

// GenerateEd25519Keypair generates a new Ed25519 keypair
// Returns private key (64 bytes) and public key (32 bytes)
func GenerateEd25519Keypair() (privateKey ed25519.PrivateKey, publicKey ed25519.PublicKey, err error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate Ed25519 keypair: %w", err)
	}

	return priv, pub, nil
}

// SignRequest signs request data using Ed25519 private key
// The data is marshaled to JSON with sorted keys for consistency
// Returns base64-encoded signature
func SignRequest(privateKey ed25519.PrivateKey, data interface{}) (string, error) {
	// Marshal data to JSON with sorted keys for consistency
	jsonData, err := marshalJSONSorted(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal data: %w", err)
	}

	// Sign the JSON payload
	signature := ed25519.Sign(privateKey, jsonData)

	// Return base64-encoded signature
	return base64.StdEncoding.EncodeToString(signature), nil
}

// VerifySignature verifies Ed25519 signature
// Used for testing and validation
func VerifySignature(publicKey ed25519.PublicKey, data interface{}, signatureB64 string) bool {
	// Marshal data to JSON with sorted keys
	jsonData, err := marshalJSONSorted(data)
	if err != nil {
		return false
	}

	// Decode base64 signature
	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return false
	}

	// Verify signature
	return ed25519.Verify(publicKey, jsonData, signature)
}

// marshalJSONSorted marshals data to JSON with sorted keys for deterministic output
func marshalJSONSorted(data interface{}) ([]byte, error) {
	// First marshal to get a map
	tempJSON, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	// Unmarshal to generic map
	var m map[string]interface{}
	if err := json.Unmarshal(tempJSON, &m); err != nil {
		// If it's not a map, just return the original marshaled data
		return tempJSON, nil
	}

	// Marshal again with sorted keys
	return json.Marshal(sortedMap(m))
}

// sortedMap recursively sorts map keys
func sortedMap(m map[string]interface{}) map[string]interface{} {
	// Get sorted keys
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Create new map with sorted keys
	result := make(map[string]interface{})
	for _, k := range keys {
		v := m[k]
		// Recursively sort nested maps
		if nestedMap, ok := v.(map[string]interface{}); ok {
			result[k] = sortedMap(nestedMap)
		} else {
			result[k] = v
		}
	}

	return result
}

// EncodePublicKey encodes a public key to base64
func EncodePublicKey(publicKey ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString(publicKey)
}

// DecodePublicKey decodes a base64-encoded public key
func DecodePublicKey(publicKeyB64 string) (ed25519.PublicKey, error) {
	publicKey, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}

	if len(publicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: expected %d, got %d", ed25519.PublicKeySize, len(publicKey))
	}

	return ed25519.PublicKey(publicKey), nil
}

// EncodePrivateKey encodes a private key to base64
func EncodePrivateKey(privateKey ed25519.PrivateKey) string {
	return base64.StdEncoding.EncodeToString(privateKey)
}

// DecodePrivateKey decodes a base64-encoded private key
func DecodePrivateKey(privateKeyB64 string) (ed25519.PrivateKey, error) {
	privateKey, err := base64.StdEncoding.DecodeString(privateKeyB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: expected %d, got %d", ed25519.PrivateKeySize, len(privateKey))
	}

	return ed25519.PrivateKey(privateKey), nil
}
