package utils

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// ValidateExternalURL validates that a URL is safe for server-side requests.
// Blocks private/internal IPs, loopback, link-local, and non-HTTPS schemes
// to prevent SSRF attacks.
//
// This check runs when a URL is accepted. It does not protect the request
// itself: a name can resolve differently when the request is sent, so every
// request to such a URL must also go through NewEgressClient, which applies the
// same address policy to the address it connects to.
//
// Errors name the class of address that was refused, never the hostname's
// resolved address or resolver detail.
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
		"metadata.google.internal", // GCP metadata
		"169.254.169.254",          // AWS/Azure/GCP metadata endpoint
		"metadata.google.internal.",
	}
	for _, blocked := range blockedHosts {
		if lowerHost == blocked {
			return fmt.Errorf("URL hostname is not allowed (internal or metadata endpoint)")
		}
	}

	// Block cloud metadata IP variations
	if lowerHost == "[::1]" || lowerHost == "0.0.0.0" || lowerHost == "[::]" {
		return fmt.Errorf("URL hostname is not allowed (loopback or unspecified address)")
	}

	// Resolve hostname to IP addresses
	ips, err := net.LookupHost(hostname)
	if err != nil {
		return fmt.Errorf("URL hostname could not be resolved")
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}

		if class, denied := DeniedAddressClass(ip); denied {
			return &EgressRefusedError{Class: class}
		}
	}

	return nil
}

// addressClass is one range of addresses a server-originated request must not
// reach, with the name a refusal reports for it.
type addressClass struct {
	prefix netip.Prefix
	name   string
}

func classes(entries ...string) []addressClass {
	out := make([]addressClass, 0, len(entries)/2)
	for i := 0; i+1 < len(entries); i += 2 {
		out = append(out, addressClass{prefix: netip.MustParsePrefix(entries[i]), name: entries[i+1]})
	}
	return out
}

// deniedIPv4 lists the IPv4 ranges that are not public destinations. The order
// only decides which name a refusal reports when ranges overlap.
var deniedIPv4 = classes(
	"0.0.0.0/8", "this network",
	"10.0.0.0/8", "private",
	"100.64.0.0/10", "shared address space",
	"127.0.0.0/8", "loopback",
	"168.63.129.16/32", "cloud platform", // Azure platform address (wireserver, DNS, health probes)
	"169.254.0.0/16", "link-local", // includes the cloud instance metadata address
	"172.16.0.0/12", "private",
	"192.0.0.0/24", "IETF protocol assignments",
	"192.0.2.0/24", "documentation",
	"192.168.0.0/16", "private",
	"198.18.0.0/15", "benchmarking",
	"198.51.100.0/24", "documentation",
	"203.0.113.0/24", "documentation",
	"224.0.0.0/4", "multicast",
	"255.255.255.255/32", "broadcast",
	"240.0.0.0/4", "reserved",
)

// deniedIPv6 lists the IPv6 ranges that are not public destinations.
var deniedIPv6 = classes(
	"::/128", "unspecified",
	"::1/128", "loopback",
	"100::/64", "discard-only",
	"2001::/32", "Teredo",
	"2001:db8::/32", "documentation",
	"fc00::/7", "private",
	"fe80::/10", "link-local",
	"fec0::/10", "site-local",
	"ff00::/8", "multicast",
)

var (
	nat64WellKnown = netip.MustParsePrefix("64:ff9b::/96")
	nat64LocalUse  = netip.MustParsePrefix("64:ff9b:1::/48")
	sixToFour      = netip.MustParsePrefix("2002::/16")
)

// DeniedAddressClass reports whether a server-originated request must not
// connect to ip, and names the class of the address when it must not.
//
// It is the single address policy behind ValidateExternalURL and the
// connection-time check in NewEgressClient, so the two cannot disagree.
// IPv6 addresses that carry an IPv4 address (IPv4-mapped, NAT64 and 6to4) are
// judged on the IPv4 address they carry.
func DeniedAddressClass(ip net.IP) (string, bool) {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return "unparseable address", true
	}
	addr = addr.Unmap() // ::ffff:0:0/96 is judged on its IPv4 address
	if addr.Is4() {
		return deniedClass(deniedIPv4, addr)
	}

	for _, embedded := range embeddedIPv4(addr) {
		if class, denied := deniedClass(deniedIPv4, embedded); denied {
			return class, true
		}
	}
	return deniedClass(deniedIPv6, addr)
}

func deniedClass(table []addressClass, addr netip.Addr) (string, bool) {
	for _, c := range table {
		if c.prefix.Contains(addr) {
			return c.name, true
		}
	}
	return "", false
}

// embeddedIPv4 returns the IPv4 addresses an IPv6 address carries. The
// local-use NAT64 prefix may be deployed at any length from /48 to /96, so each
// position that length allows is returned and every one must be public.
func embeddedIPv4(addr netip.Addr) []netip.Addr {
	b := addr.As16()
	v4 := func(i, j, k, l int) netip.Addr { return netip.AddrFrom4([4]byte{b[i], b[j], b[k], b[l]}) }
	switch {
	case nat64WellKnown.Contains(addr):
		return []netip.Addr{v4(12, 13, 14, 15)}
	case nat64LocalUse.Contains(addr):
		// RFC 6052 section 2.2 positions for /48, /56, /64 and /96 prefixes.
		return []netip.Addr{v4(6, 7, 9, 10), v4(7, 9, 10, 11), v4(9, 10, 11, 12), v4(12, 13, 14, 15)}
	case sixToFour.Contains(addr):
		return []netip.Addr{v4(2, 3, 4, 5)}
	}
	return nil
}
