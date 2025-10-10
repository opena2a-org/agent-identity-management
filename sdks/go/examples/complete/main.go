package main

import (
	"context"
	"fmt"
	"log"
	"time"

	aimsdk "github.com/opena2a/aim-sdk-go"
)

func main() {
	ctx := context.Background()

	fmt.Println("=== AIM SDK Complete Example ===")

	// Example 1: Register a new agent
	fmt.Println("--- Example 1: Register New Agent ---")
	if err := registerNewAgent(ctx); err != nil {
		log.Printf("Error: %v\n", err)
	}

	// Example 2: Use existing agent (credentials from keyring)
	fmt.Println("\n--- Example 2: Use Existing Agent ---")
	if err := useExistingAgent(ctx); err != nil {
		log.Printf("Error: %v\n", err)
	}

	// Example 3: Auto-detect and report MCPs
	fmt.Println("\n--- Example 3: Auto-Detect MCPs ---")
	if err := autoDetectMCPs(ctx); err != nil {
		log.Printf("Error: %v\n", err)
	}

	// Example 4: Manual MCP reporting
	fmt.Println("\n--- Example 4: Manual MCP Reporting ---")
	if err := manualReporting(ctx); err != nil {
		log.Printf("Error: %v\n", err)
	}

	fmt.Println("\n✅ All examples completed!")
}

// Example 1: Register a new agent with Ed25519 signing
func registerNewAgent(ctx context.Context) error {
	// Create client without credentials (for registration)
	client := aimsdk.NewClient(aimsdk.Config{
		APIURL: "http://localhost:8080",
	})

	// Register agent with automatic Ed25519 keypair generation
	registration, err := client.RegisterAgent(ctx, aimsdk.RegisterOptions{
		Name: "my-go-agent",
		Type: "ai_agent",
	})
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	fmt.Printf("✅ Agent registered successfully!\n")
	fmt.Printf("   ID: %s\n", registration.ID)
	fmt.Printf("   Name: %s\n", registration.Name)
	fmt.Printf("   API Key: %s...\n", registration.APIKey[:20])
	fmt.Printf("   Public Key: %s...\n", registration.PublicKey[:32])

	return nil
}

// Example 2: Use an existing agent (credentials loaded from keyring)
func useExistingAgent(ctx context.Context) error {
	// Check if credentials exist
	if !aimsdk.HasCredentials() {
		fmt.Println("⚠️  No credentials found. Please register an agent first.")
		return nil
	}

	// Load credentials from keyring
	creds, err := aimsdk.LoadCredentials()
	if err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}

	// Create client with loaded credentials
	client := aimsdk.NewClient(aimsdk.Config{
		APIURL:  "http://localhost:8080",
		AgentID: creds.AgentID,
		APIKey:  creds.APIKey,
	})

	fmt.Printf("✅ Using existing agent: %s\n", creds.AgentID)

	// Report MCP usage
	if err := client.ReportMCP(ctx, "filesystem"); err != nil {
		return fmt.Errorf("failed to report MCP: %w", err)
	}

	fmt.Println("✅ Reported MCP usage")
	return nil
}

// Example 3: Auto-detect MCPs from configuration files
func autoDetectMCPs(ctx context.Context) error {
	// Auto-detect MCPs
	detection, err := aimsdk.AutoDetectMCPs()
	if err != nil {
		return fmt.Errorf("detection failed: %w", err)
	}

	fmt.Printf("📡 Detected %d MCP server(s)\n", len(detection.MCPs))

	for i, mcp := range detection.MCPs {
		fmt.Printf("\n%d. %s\n", i+1, mcp.Name)
		fmt.Printf("   Type: %s\n", mcp.Type)
		fmt.Printf("   Command: %s\n", mcp.Command)
		fmt.Printf("   Capabilities: %v\n", mcp.Capabilities)
		fmt.Printf("   Detected from: %s\n", mcp.DetectedFrom)
	}

	// Report all detected MCPs if agent is registered
	if aimsdk.HasCredentials() {
		creds, _ := aimsdk.LoadCredentials()
		client := aimsdk.NewClient(aimsdk.Config{
			APIURL:  "http://localhost:8080",
			AgentID: creds.AgentID,
			APIKey:  creds.APIKey,
		})

		for _, mcp := range detection.MCPs {
			if err := client.ReportMCP(ctx, mcp.Name); err != nil {
				log.Printf("Warning: failed to report %s: %v", mcp.Name, err)
			} else {
				fmt.Printf("   ✅ Reported to AIM backend\n")
			}
		}
	}

	return nil
}

// Example 4: Manual MCP reporting with custom client
func manualReporting(ctx context.Context) error {
	// Create client with explicit credentials
	client := aimsdk.NewClient(aimsdk.Config{
		APIURL:         "http://localhost:8080",
		AutoDetect:     true,
		ReportInterval: 30 * time.Second,
	})

	// Report specific MCPs
	mcps := []string{
		"filesystem",
		"database",
		"memory",
		"github",
	}

	for _, mcp := range mcps {
		if err := client.ReportMCP(ctx, mcp); err != nil {
			log.Printf("Warning: failed to report %s: %v", mcp, err)
		} else {
			fmt.Printf("✅ Reported: %s\n", mcp)
		}
	}

	return nil
}

// Example 5: Register with OAuth (commented out - requires OAuth setup)
/*
func registerWithOAuth(ctx context.Context) error {
	client := aimsdk.NewClient(aimsdk.Config{
		APIURL: "http://localhost:8080",
	})

	// Register with Google OAuth
	registration, err := client.RegisterAgentWithOAuth(ctx, aimsdk.RegisterOptions{
		Name:          "my-oauth-agent",
		Type:          "ai_agent",
		OAuthProvider: aimsdk.OAuthProviderGoogle,
		RedirectURL:   "http://localhost:8080/callback",
	})
	if err != nil {
		return fmt.Errorf("OAuth registration failed: %w", err)
	}

	fmt.Printf("✅ Agent registered with OAuth!\n")
	fmt.Printf("   ID: %s\n", registration.ID)
	fmt.Printf("   Name: %s\n", registration.Name)

	return nil
}
*/
