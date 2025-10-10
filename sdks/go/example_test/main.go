package main

import (
	"context"
	"fmt"
	"os"

	aimsdk "github.com/opena2a-org/agent-identity-management/sdks/go"
)

// Test Go SDK like a real developer would use it
// This demonstrates the current Go SDK functionality (MCP reporting only)
func main() {
	fmt.Println("============================================================")
	fmt.Println("Testing Go SDK - Real Developer Usage")
	fmt.Println("============================================================")

	// Real API key created from AIM dashboard for test-agent-3
	apiKey := "aim_live_UoMhd6D9lGUbQhVrznTs5JltxeljfFx33jkfiPhCm5E="
	// Agent ID that matches the API key (test-agent-3)
	agentID := "a934b38f-aa1c-46ef-99b9-775da9e551dd"

	// Initialize SDK
	client := aimsdk.NewClient(aimsdk.Config{
		APIURL:  "http://localhost:8080",
		APIKey:  apiKey,
		AgentID: agentID,
	})
	defer client.Close()

	fmt.Println("\n✅ Go SDK initialized successfully!")
	fmt.Printf("   Agent ID: %s\n", agentID)
	fmt.Printf("   API URL: http://localhost:8080\n")

	// Show runtime info
	fmt.Println("\n📊 Runtime Information:")
	info := client.GetRuntimeInfo()
	for k, v := range info {
		fmt.Printf("   %s: %v\n", k, v)
	}

	// Report MCP usage (filesystem)
	fmt.Println("\n📡 Reporting MCP usage...")
	if err := client.ReportMCP(context.Background(), "filesystem"); err != nil {
		fmt.Printf("❌ Failed to report filesystem MCP: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("   ✅ Reported: filesystem")

	// Report another MCP (sqlite)
	if err := client.ReportMCP(context.Background(), "sqlite"); err != nil {
		fmt.Printf("❌ Failed to report sqlite MCP: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("   ✅ Reported: sqlite")

	// Report browser MCP
	if err := client.ReportMCP(context.Background(), "puppeteer"); err != nil {
		fmt.Printf("❌ Failed to report puppeteer MCP: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("   ✅ Reported: puppeteer")

	fmt.Println("\n✅ Test complete! Go SDK is working correctly.")
	fmt.Println("\n📌 Current Go SDK Features:")
	fmt.Println("   ✅ MCP reporting (manual)")
	fmt.Println("   ✅ Runtime information")
	fmt.Println("   ✅ Type-safe API")
	fmt.Println("   ✅ Context support")

	fmt.Println("\n📌 Feature Parity Gaps (vs Python SDK):")
	fmt.Println("   ❌ No agent registration")
	fmt.Println("   ❌ No OAuth support")
	fmt.Println("   ❌ No auto-detection")
	fmt.Println("   ❌ No Ed25519 signing")
	fmt.Println("   ❌ No capability detection")

	fmt.Println("\n============================================================")
}
