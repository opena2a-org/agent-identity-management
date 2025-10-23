package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

func main() {
	// Read test data
	data, err := os.ReadFile("signature_test_data.txt")
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	lines := strings.Split(string(data), "\n")
	var publicKeyB64, challenge, signatureB64 string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "PUBLIC_KEY":
			publicKeyB64 = value
		case "CHALLENGE":
			challenge = value
		case "SIGNATURE":
			signatureB64 = value
		}
	}

	fmt.Printf("Parsed values:\n")
	fmt.Printf("  PUBLIC_KEY=%s\n", publicKeyB64)
	fmt.Printf("  CHALLENGE=%s\n", challenge)
	fmt.Printf("  SIGNATURE=%s\n\n", signatureB64)

	fmt.Println("Testing Ed25519 Verification (Go)")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()

	// Decode public key
	publicKey, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		fmt.Printf("❌ Error decoding public key: %v\n", err)
		return
	}
	fmt.Printf("Public Key: %s (%d bytes)\n", publicKeyB64, len(publicKey))

	// Decode signature
	signature, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		fmt.Printf("❌ Error decoding signature: %v\n", err)
		return
	}
	fmt.Printf("Signature:  %s (%d bytes)\n", signatureB64, len(signature))

	// Message is the UTF-8 bytes of the challenge string
	message := []byte(challenge)
	fmt.Printf("Challenge:  %s (%d bytes)\n", challenge, len(message))
	fmt.Println()

	// Verify
	valid := ed25519.Verify(ed25519.PublicKey(publicKey), message, signature)

	if valid {
		fmt.Println("✅ VERIFICATION SUCCESS!")
		fmt.Println()
		fmt.Println("This confirms:")
		fmt.Println("  • PyNaCl signature format is correct")
		fmt.Println("  • Go ed25519.Verify can verify it")
		fmt.Println("  • The signature extraction (first 64 bytes) works")
	} else {
		fmt.Println("❌ VERIFICATION FAILED!")
		fmt.Println()
		fmt.Println("Debugging info:")
		fmt.Printf("  Public key length: %d (expected 32)\n", len(publicKey))
		fmt.Printf("  Signature length:  %d (expected 64)\n", len(signature))
		fmt.Printf("  Message length:    %d\n", len(message))
	}
}

