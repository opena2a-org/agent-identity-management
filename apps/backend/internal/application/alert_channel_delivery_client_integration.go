//go:build integration

package application

import (
	"net/http"
	"time"
)

// SetDeliveryClientForIntegrationTest replaces the client alert channel
// deliveries are sent with, so an integration test can receive them on a
// loopback server the egress client refuses. It exists only in a build with
// the integration tag; a server build has no way to replace the egress client.
func (s *AlertChannelService) SetDeliveryClientForIntegrationTest(newClient func(timeout time.Duration) *http.Client) {
	s.newHTTPClient = newClient
}

// SetRetryDelaysForIntegrationTest replaces the waits before each retry of an
// alert channel delivery, so an integration test need not wait for them. It
// exists only in a build with the integration tag.
func (s *AlertChannelService) SetRetryDelaysForIntegrationTest(delays []time.Duration) {
	s.retryDelays = append([]time.Duration(nil), delays...)
}
