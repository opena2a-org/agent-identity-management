package crypto

import (
	"encoding/base64"
	"testing"
)

func TestED25519Service_GenerateKeyPair(t *testing.T) {
	service := NewED25519Service()

	t.Run("generates valid keypair", func(t *testing.T) {
		keypair, err := service.GenerateKeyPair()
		if err != nil {
			t.Fatalf("GenerateKeyPair() error = %v", err)
		}

		if keypair == nil {
			t.Fatal("GenerateKeyPair() returned nil keypair")
		}

		if keypair.PublicKey == "" {
			t.Error("PublicKey is empty")
		}

		if keypair.PrivateKey == "" {
			t.Error("PrivateKey is empty")
		}

		// Verify keys are valid base64
		_, err = base64.StdEncoding.DecodeString(keypair.PublicKey)
		if err != nil {
			t.Errorf("PublicKey is not valid base64: %v", err)
		}

		_, err = base64.StdEncoding.DecodeString(keypair.PrivateKey)
		if err != nil {
			t.Errorf("PrivateKey is not valid base64: %v", err)
		}
	})

	t.Run("generates unique keypairs", func(t *testing.T) {
		keypair1, err := service.GenerateKeyPair()
		if err != nil {
			t.Fatalf("GenerateKeyPair() error = %v", err)
		}

		keypair2, err := service.GenerateKeyPair()
		if err != nil {
			t.Fatalf("GenerateKeyPair() error = %v", err)
		}

		if keypair1.PublicKey == keypair2.PublicKey {
			t.Error("Generated keypairs have identical public keys")
		}

		if keypair1.PrivateKey == keypair2.PrivateKey {
			t.Error("Generated keypairs have identical private keys")
		}
	})
}

func TestED25519Service_Sign(t *testing.T) {
	service := NewED25519Service()
	keypair, _ := service.GenerateKeyPair()
	message := []byte("test message")

	t.Run("signs message successfully", func(t *testing.T) {
		signature, err := service.Sign(keypair.PrivateKey, message)
		if err != nil {
			t.Fatalf("Sign() error = %v", err)
		}

		if signature == "" {
			t.Error("Sign() returned empty signature")
		}

		// Verify signature is valid base64
		_, err = base64.StdEncoding.DecodeString(signature)
		if err != nil {
			t.Errorf("Signature is not valid base64: %v", err)
		}
	})

	t.Run("returns error for invalid private key", func(t *testing.T) {
		_, err := service.Sign("invalid-key", message)
		if err == nil {
			t.Error("Sign() should return error for invalid private key")
		}
	})

	t.Run("returns error for wrong size private key", func(t *testing.T) {
		wrongSizeKey := base64.StdEncoding.EncodeToString([]byte("short"))
		_, err := service.Sign(wrongSizeKey, message)
		if err == nil {
			t.Error("Sign() should return error for wrong size private key")
		}
	})
}

func TestED25519Service_Verify(t *testing.T) {
	service := NewED25519Service()
	keypair, _ := service.GenerateKeyPair()
	message := []byte("test message")
	signature, _ := service.Sign(keypair.PrivateKey, message)

	t.Run("verifies valid signature", func(t *testing.T) {
		valid, err := service.Verify(keypair.PublicKey, message, signature)
		if err != nil {
			t.Fatalf("Verify() error = %v", err)
		}

		if !valid {
			t.Error("Verify() returned false for valid signature")
		}
	})

	t.Run("rejects invalid signature", func(t *testing.T) {
		invalidSignature := base64.StdEncoding.EncodeToString(make([]byte, 64))
		valid, err := service.Verify(keypair.PublicKey, message, invalidSignature)
		if err != nil {
			t.Fatalf("Verify() error = %v", err)
		}

		if valid {
			t.Error("Verify() returned true for invalid signature")
		}
	})

	t.Run("rejects tampered message", func(t *testing.T) {
		tamperedMessage := []byte("tampered message")
		valid, err := service.Verify(keypair.PublicKey, tamperedMessage, signature)
		if err != nil {
			t.Fatalf("Verify() error = %v", err)
		}

		if valid {
			t.Error("Verify() returned true for tampered message")
		}
	})

	t.Run("returns error for invalid public key", func(t *testing.T) {
		_, err := service.Verify("invalid-key", message, signature)
		if err == nil {
			t.Error("Verify() should return error for invalid public key")
		}
	})

	t.Run("returns error for wrong size public key", func(t *testing.T) {
		wrongSizeKey := base64.StdEncoding.EncodeToString([]byte("short"))
		_, err := service.Verify(wrongSizeKey, message, signature)
		if err == nil {
			t.Error("Verify() should return error for wrong size public key")
		}
	})

	t.Run("returns error for invalid signature format", func(t *testing.T) {
		_, err := service.Verify(keypair.PublicKey, message, "invalid-signature")
		if err == nil {
			t.Error("Verify() should return error for invalid signature format")
		}
	})
}

func TestED25519Service_GenerateChallenge(t *testing.T) {
	service := NewED25519Service()

	t.Run("generates valid challenge", func(t *testing.T) {
		challenge, err := service.GenerateChallenge()
		if err != nil {
			t.Fatalf("GenerateChallenge() error = %v", err)
		}

		if challenge == "" {
			t.Error("GenerateChallenge() returned empty challenge")
		}

		// Verify challenge is valid base64
		decoded, err := base64.StdEncoding.DecodeString(challenge)
		if err != nil {
			t.Errorf("Challenge is not valid base64: %v", err)
		}

		// Verify challenge is 32 bytes
		if len(decoded) != 32 {
			t.Errorf("Challenge should be 32 bytes, got %d", len(decoded))
		}
	})

	t.Run("generates unique challenges", func(t *testing.T) {
		challenge1, err := service.GenerateChallenge()
		if err != nil {
			t.Fatalf("GenerateChallenge() error = %v", err)
		}

		challenge2, err := service.GenerateChallenge()
		if err != nil {
			t.Fatalf("GenerateChallenge() error = %v", err)
		}

		if challenge1 == challenge2 {
			t.Error("Generated challenges are identical")
		}
	})
}

func TestED25519Service_SignAndVerifyIntegration(t *testing.T) {
	service := NewED25519Service()

	t.Run("complete sign and verify workflow", func(t *testing.T) {
		// Generate keypair
		keypair, err := service.GenerateKeyPair()
		if err != nil {
			t.Fatalf("GenerateKeyPair() error = %v", err)
		}

		// Create message
		message := []byte("Integration test message")

		// Sign message
		signature, err := service.Sign(keypair.PrivateKey, message)
		if err != nil {
			t.Fatalf("Sign() error = %v", err)
		}

		// Verify signature
		valid, err := service.Verify(keypair.PublicKey, message, signature)
		if err != nil {
			t.Fatalf("Verify() error = %v", err)
		}

		if !valid {
			t.Error("Signature verification failed for valid signature")
		}
	})

	t.Run("cross-keypair verification fails", func(t *testing.T) {
		// Generate two keypairs
		keypair1, _ := service.GenerateKeyPair()
		keypair2, _ := service.GenerateKeyPair()

		message := []byte("test message")

		// Sign with keypair1
		signature, _ := service.Sign(keypair1.PrivateKey, message)

		// Try to verify with keypair2's public key (should fail)
		valid, err := service.Verify(keypair2.PublicKey, message, signature)
		if err != nil {
			t.Fatalf("Verify() error = %v", err)
		}

		if valid {
			t.Error("Signature should not verify with different public key")
		}
	})
}
