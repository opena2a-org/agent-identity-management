//go:build integration

package application

import (
	"net/http"
	"time"
)

// SetDeliveryClientForIntegrationTest replaces the client deliveries are sent
// with, so an integration test can receive them on a loopback server the
// egress client refuses. It exists only in a build with the integration tag;
// a server build has no way to replace the egress client.
func (s *WebhookService) SetDeliveryClientForIntegrationTest(newClient func(timeout time.Duration) *http.Client) {
	s.newHTTPClient = newClient
}
