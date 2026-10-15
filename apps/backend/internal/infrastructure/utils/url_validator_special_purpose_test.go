package utils

import (
	"errors"
	"net"
	"testing"
)

// The Dummy IPv6 Prefix (100:0:0:1::/64) and the part of the IETF Protocol
// Assignments block (2001::/23) that has no globally reachable assignment are
// not public destinations. The globally reachable assignments inside 2001::/23
// stay public, and the first address after the block is public.

var specialPurposeRefused = []struct {
	addr  string
	class string
}{
	{"100:0:0:1::", "dummy prefix"},
	{"100:0:0:1::1", "dummy prefix"},
	{"100:0:0:1:ffff:ffff:ffff:ffff", "dummy prefix"},
	{"2001:1::", "IETF protocol assignments"},
	{"2001:1::4", "IETF protocol assignments"},
	{"2001:4::1", "IETF protocol assignments"},
	{"2001:5::1", "IETF protocol assignments"},
	{"2001:40::1", "IETF protocol assignments"},
	{"2001:100::1", "IETF protocol assignments"},
	{"2001:1ff:ffff:ffff:ffff:ffff:ffff:ffff", "IETF protocol assignments"},
	// The narrower ranges inside 2001::/23 keep their own class.
	{"2001::1", "Teredo"},
	{"2001:2::1", "benchmarking"},
	{"2001:10::1", "ORCHID"},
	{"2001:20::1", "ORCHIDv2"},
}

var specialPurposeAdmitted = []string{
	"100:0:0:2::1",    // after the dummy prefix
	"2001:1::1",       // Port Control Protocol anycast
	"2001:1::2",       // TURN anycast
	"2001:1::3",       // DNS-SD Service Registration Protocol anycast
	"2001:3::1",       // AMT
	"2001:4:112::1",   // AS112-v6
	"2001:30::1",      // Drone Remote ID Protocol Entity Tags
	"2001:200::1",     // first address after 2001::/23
	"2001:4860::8888", // a public resolver
}

func TestDeniedAddressClass_DummyPrefixAndIETFProtocolAssignments(t *testing.T) {
	for _, tc := range specialPurposeRefused {
		ip := net.ParseIP(tc.addr)
		if ip == nil {
			t.Fatalf("test address %s does not parse", tc.addr)
		}
		class, denied := DeniedAddressClass(ip)
		if !denied || class != tc.class {
			t.Errorf("DeniedAddressClass(%s) = (%q, %v), want (%q, true)", tc.addr, class, denied, tc.class)
		}
	}
	for _, addr := range specialPurposeAdmitted {
		ip := net.ParseIP(addr)
		if ip == nil {
			t.Fatalf("test address %s does not parse", addr)
		}
		if class, denied := DeniedAddressClass(ip); denied {
			t.Errorf("DeniedAddressClass(%s) refused a public address as %q", addr, class)
		}
	}
}

func TestValidateExternalURL_DummyPrefixAndIETFProtocolAssignments(t *testing.T) {
	for _, tc := range specialPurposeRefused {
		rawURL := "https://[" + tc.addr + "]:443/"
		err := ValidateExternalURL(rawURL)
		var refused *EgressRefusedError
		if !errors.As(err, &refused) || refused.Class != tc.class {
			t.Errorf("ValidateExternalURL(%s) = %v, want a refusal of class %q", rawURL, err, tc.class)
		}
	}
	for _, addr := range specialPurposeAdmitted {
		rawURL := "https://[" + addr + "]:443/"
		if err := ValidateExternalURL(rawURL); err != nil {
			t.Errorf("ValidateExternalURL(%s) refused a public address: %v", rawURL, err)
		}
	}
}
