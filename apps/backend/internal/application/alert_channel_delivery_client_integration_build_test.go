package application

import (
	"bufio"
	"os"
	"testing"
)

// The setters that replace the alert channel delivery client and retry
// delays compile only with the integration tag: without its build
// constraint, every server build could send alerts around the egress
// client's address checks.
func TestAlertChannelDeliveryClientSetterIsIntegrationOnly(t *testing.T) {
	const name = "alert_channel_delivery_client_integration.go"
	f, err := os.Open(name)
	if err != nil {
		t.Fatalf("open the setters' file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		t.Fatalf("%s is empty: %v", name, scanner.Err())
	}
	if first := scanner.Text(); first != "//go:build integration" {
		t.Fatalf("first line of %s is %q, want %q", name, first, "//go:build integration")
	}
}
