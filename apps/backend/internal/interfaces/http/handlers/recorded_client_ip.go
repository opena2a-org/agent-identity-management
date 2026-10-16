package handlers

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// recordedClientIP is the client address an audit row or a security log line
// records for this request: the trusted-proxy client address
// (middleware.ClientIP, identical to c.IP() when TRUSTED_PROXIES is unset).
// A trusted proxy's address header is recorded only when it parses as an
// address; otherwise the connecting address is, so a forwarded value can
// never add fields to a log line or overflow the audit column. The header
// value is copied: it is a view of the request buffer, which the server
// reuses for later requests, and an audit row hands the address on to the
// event SIEM forwarding queues, which is serialized after the request ends.
func recordedClientIP(c fiber.Ctx) string {
	ip := middleware.ClientIP(c)
	if net.ParseIP(ip) == nil {
		return c.IP()
	}
	return strings.Clone(ip)
}
