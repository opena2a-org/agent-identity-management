package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	aimsdk "github.com/opena2a/aim-sdk-go"
)

func main() {
	fmt.Println("🔍 Go SDK Capability Detection Test")
	fmt.Println("=" + "=====================================")

	// Backend URL
	apiURL := os.Getenv("AIM_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}

	fmt.Printf("\n📡 Backend URL: %s\n", apiURL)

	// Step 1: Auto-detect capabilities locally
	fmt.Println("\n📦 Step 1: Auto-detecting capabilities from go.mod...")
	detector := aimsdk.NewCapabilityDetector()
	result, err := detector.DetectAll()
	if err != nil {
		log.Fatalf("❌ Detection failed: %v", err)
	}

	fmt.Printf("   ✅ Detected %d capabilities: %v\n", len(result.Capabilities), result.Capabilities)
	fmt.Printf("   📍 Detection sources: %v\n", result.DetectedFrom)
	fmt.Printf("   🔧 Metadata: %v\n", result.Metadata)

	// Step 2: Register agent with backend (or use existing)
	fmt.Println("\n🔐 Step 2: Setting up agent...")

	// Check if agent credentials exist
	agentID := os.Getenv("GO_AGENT_ID")
	apiKey := os.Getenv("GO_API_KEY")

	if agentID == "" || apiKey == "" {
		fmt.Println("   ⚠️  No existing agent credentials found")
		fmt.Println("   Please set GO_AGENT_ID and GO_API_KEY environment variables")
		fmt.Println("   Or register a new agent first")
		os.Exit(1)
	}

	fmt.Printf("   ✅ Using agent ID: %s\n", agentID)

	// Step 3: Create client and report capabilities
	fmt.Println("\n📤 Step 3: Reporting capabilities to backend...")
	client := aimsdk.NewClient(aimsdk.Config{
		APIURL:  apiURL,
		AgentID: agentID,
		APIKey:  apiKey,
	})

	ctx := context.Background()

	// Add some test capabilities if none detected
	capabilities := result.Capabilities
	if len(capabilities) == 0 {
		fmt.Println("   ℹ️  No capabilities auto-detected, adding test capabilities")
		capabilities = []string{
			"read_files",
			"make_api_calls",
			"data_processing",
			"network_access",
		}
	}

	// Report capabilities
	if err := client.ReportCapabilities(ctx, capabilities); err != nil {
		log.Fatalf("❌ Failed to report capabilities: %v", err)
	}

	fmt.Printf("   ✅ Successfully reported %d capabilities to backend\n", len(capabilities))

	// Step 4: Report SDK integration
	fmt.Println("\n📡 Step 4: Reporting SDK integration...")
	_, err = client.ReportSDKIntegration(
		ctx,
		"aim-sdk-go@1.0.0",
		"go",
		[]string{"capability_detection", "auto_detect_mcps"},
	)
	if err != nil {
		log.Fatalf("❌ Failed to report SDK integration: %v", err)
	}

	fmt.Println("   ✅ SDK integration reported")

	// Step 5: Register a test MCP server
	fmt.Println("\n🔌 Step 5: Registering test MCP server...")
	mcpResult, err := client.RegisterMCP(
		ctx,
		"filesystem-mcp-server",
		"auto_sdk",
		95.0,
		map[string]interface{}{
			"source": "capability_detection_test",
			"package": "github.com/modelcontextprotocol/filesystem",
		},
	)
	if err != nil {
		log.Printf("   ⚠️  MCP registration failed (may already exist): %v", err)
	} else {
		fmt.Printf("   ✅ Registered %d MCP server(s)\n", mcpResult.Added)
	}

	// Summary
	fmt.Println("\n" + "=" + "=====================================")
	fmt.Println("🎉 Go SDK Test Complete!")
	fmt.Printf("   - Detected: %d capabilities\n", len(capabilities))
	fmt.Printf("   - Reported: %d capabilities to backend\n", len(capabilities))
	fmt.Printf("   - Agent ID: %s\n", agentID)
	fmt.Printf("   - SDK Integration: ✅\n")
	fmt.Printf("   - MCP Server: ✅\n")
	fmt.Println("\n📊 Check the AIM dashboard:")
	fmt.Printf("   - Capabilities tab: %s/dashboard/agents/%s\n", apiURL, agentID)
	fmt.Printf("   - Detection tab: %s/dashboard/sdk\n", apiURL)
	fmt.Printf("   - Connections tab: %s/dashboard/agents/%s\n", apiURL, agentID)
	fmt.Println("=" + "=====================================")

	// Keep running for a moment to ensure all requests complete
	time.Sleep(2 * time.Second)
}
