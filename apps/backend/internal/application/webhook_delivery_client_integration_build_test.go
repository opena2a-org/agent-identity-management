package application

import (
	"bufio"
	"os"
	"testing"
)

// The setter that replaces the webhook delivery client compiles only with the
// integration tag: without its build constraint, every server build could send
// deliveries around the egress client's address checks.
func TestWebhookDeliveryClientSetterIsIntegrationOnly(t *testing.T) {
	f, err := os.Open("webhook_delivery_client_integration.go")
	if err != nil {
		t.Fatalf("open the setter's file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		t.Fatalf("webhook_delivery_client_integration.go is empty: %v", scanner.Err())
	}
	if first := scanner.Text(); first != "//go:build integration" {
		t.Fatalf("first line of webhook_delivery_client_integration.go is %q, want %q", first, "//go:build integration")
	}
}
