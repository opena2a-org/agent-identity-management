package utils

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateExternalURL validates that a URL is safe for server-side requests.
// Blocks private/internal IPs, loopback, link-local, and non-HTTPS schemes
// to prevent SSRF attacks.
func ValidateExternalURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Only allow HTTP and HTTPS schemes
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s (only http and https allowed)", parsed.Scheme)
	}

	// Extract hostname (without port)
	hostname := parsed.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL has no hostname")
	}

	// Block common internal hostnames
	lowerHost := strings.ToLower(hostname)
	blockedHosts := []string{
		"localhost",
		"metadata.google.internal",       // GCP metadata
		"169.254.169.254",                 // AWS/Azure/GCP metadata endpoint
		"metadata.google.internal.",
	}
	for _, blocked := range blockedHosts {
		if lowerHost == blocked {
			return fmt.Errorf("URL hostname %q is not allowed (internal/metadata endpoint)", hostname)
		}
	}

	// Block cloud metadata IP variations
	if lowerHost == "[::1]" || lowerHost == "0.0.0.0" || lowerHost == "[::]" {
		return fmt.Errorf("URL hostname %q is not allowed (loopback/any address)", hostname)
	}

	// Resolve hostname to IP addresses
	ips, err := net.LookupHost(hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname %q: %w", hostname, err)
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}

		if !isPublicIP(ip) {
			return fmt.Errorf("URL resolves to non-public IP %s (possible SSRF)", ipStr)
		}
	}

	return nil
}

// isPublicIP returns true if the IP is a public (routable) address.
// Returns false for loopback, private, link-local, multicast, and other reserved ranges.
func isPublicIP(ip net.IP) bool {
	// Check for loopback (127.0.0.0/8, ::1)
	if ip.IsLoopback() {
		return false
	}

	// Check for private networks (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, fc00::/7)
	if ip.IsPrivate() {
		return false
	}

	// Check for link-local (169.254.0.0/16, fe80::/10)
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}

	// Check for multicast
	if ip.IsMulticast() {
		return false
	}

	// Check for unspecified (0.0.0.0, ::)
	if ip.IsUnspecified() {
		return false
	}

	// Additional reserved ranges
	reservedRanges := []struct {
		network string
		name    string
	}{
		{"100.64.0.0/10", "shared address space (CGN)"},
		{"192.0.0.0/24", "IETF protocol assignments"},
		{"192.0.2.0/24", "TEST-NET-1"},
		{"198.18.0.0/15", "benchmarking"},
		{"198.51.100.0/24", "TEST-NET-2"},
		{"203.0.113.0/24", "TEST-NET-3"},
	}

	for _, reserved := range reservedRanges {
		_, cidr, err := net.ParseCIDR(reserved.network)
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
			return false
		}
	}

	return true
}
