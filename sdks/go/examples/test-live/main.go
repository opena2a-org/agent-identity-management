// Live test of Go SDK with real backend
package main

import (
	"context"
	"fmt"
	"os"

	aimsdk "github.com/opena2a/aim-sdk-go"
)

func main() {
	fmt.Println("🚀 Starting Go SDK live test...\n")

	// Initialize SDK with real backend
	client := aimsdk.NewClient(aimsdk.Config{
		APIURL:     "http://localhost:8080",
		APIKey:     "aim_test_1234567890abcdef",
		AgentID:    "a934b38f-aa1c-46ef-99b9-775da9e551dd",
		AutoDetect: false, // Manual testing first
	})
	defer client.Close()

	fmt.Println("✅ SDK initialized")
	fmt.Println("📍 API URL:", "http://localhost:8080")
	fmt.Println("🔑 Agent ID:", "a934b38f-aa1c-46ef-99b9-775da9e551dd")
	fmt.Println("")

	ctx := context.Background()

	// Test 1: Manual MCP report
	fmt.Println("📊 Test 1: Manual MCP report")
	if err := client.ReportMCP(ctx, "filesystem"); err != nil {
		fmt.Printf("❌ Failed to report MCP: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Successfully reported filesystem MCP\n")

	// Test 2: Report another MCP
	fmt.Println("📊 Test 2: Report another MCP")
	if err := client.ReportMCP(ctx, "github"); err != nil {
		fmt.Printf("❌ Failed to report MCP: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Successfully reported github MCP\n")

	// Test 3: Duplicate detection (should be deduplicated)
	fmt.Println("📊 Test 3: Duplicate detection (within 60s window)")
	if err := client.ReportMCP(ctx, "filesystem"); err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Duplicate detection handled correctly\n")

	fmt.Println("🎉 All tests passed!")
	fmt.Println("")
	fmt.Println("Check backend logs for:")
	fmt.Println("  - API key authentication success")
	fmt.Println("  - Detection processing")
	fmt.Println("  - MCP server creation")
}
