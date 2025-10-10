package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	aimsdk "github.com/opena2a/aim-sdk-go"
)

func main() {
	// Initialize AIM SDK
	client := aimsdk.NewClient(aimsdk.Config{
		APIURL:         getEnvOrDefault("AIM_API_URL", "http://localhost:8080"),
		APIKey:         getEnvOrDefault("AIM_API_KEY", "aim_test_key_12345"),
		AgentID:        getEnvOrDefault("AIM_AGENT_ID", "test-agent-id"),
		AutoDetect:     false, // Manual mode for Go
		ReportInterval: 10 * time.Second,
	})
	defer client.Close()

	fmt.Println("🚀 AIM SDK initialized")
	fmt.Printf("📊 Runtime info: %+v\n", client.GetRuntimeInfo())

	// Manually report MCP usage
	// In Go, detection is typically manual since we can't hook imports
	fmt.Println("\n📡 Reporting MCP usage...")

	if err := client.ReportMCP(context.Background(), "filesystem"); err != nil {
		fmt.Printf("❌ Failed to report filesystem MCP: %v\n", err)
	} else {
		fmt.Println("✅ Successfully reported filesystem MCP usage")
	}

	if err := client.ReportMCP(context.Background(), "sqlite"); err != nil {
		fmt.Printf("❌ Failed to report sqlite MCP: %v\n", err)
	} else {
		fmt.Println("✅ Successfully reported sqlite MCP usage")
	}

	if err := client.ReportMCP(context.Background(), "github"); err != nil {
		fmt.Printf("❌ Failed to report github MCP: %v\n", err)
	} else {
		fmt.Println("✅ Successfully reported github MCP usage")
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	fmt.Println("\n⏳ AIM SDK is running. Press Ctrl+C to exit...")
	<-sigChan

	fmt.Println("\n👋 Shutting down gracefully...")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
