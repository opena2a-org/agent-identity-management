package utils

import (
	"net"
	"testing"
)

// A server-side request to a caller-supplied URL is judged by the egress
// address policy twice: by ValidateExternalURL when the URL is checked, and by
// the egress client's dialer when it connects. These tests hold the policy
// through ValidateExternalURL alone, with IP literals so that no DNS lookup is
// made, and pin which addresses are refused rather than how a refusal reads.

// urlPolicyRefusedAddresses are addresses no server-side request may reach:
// private, loopback, link-local and cloud platform addresses, the other
// special-purpose ranges, and IPv6 forms that carry one of them.
var urlPolicyRefusedAddresses = []string{
	"127.0.0.1",
	"169.254.169.254",
	"10.0.0.5",
	"172.16.3.4",
	"192.168.1.1",
	"100.64.0.1",
	"0.0.0.0",
	"0.1.2.3",
	"168.63.129.16",
	"192.0.0.8",
	"192.0.2.1",
	"192.88.99.1",
	"198.18.0.1",
	"198.51.100.1",
	"203.0.113.1",
	"224.0.0.1",
	"240.0.0.1",
	"255.255.255.255",
	"::",
	"::1",
	"::7f00:1",
	"::ffff:127.0.0.1",
	"::ffff:169.254.169.254",
	"::ffff:0:a9fe:a9fe",
	"64:ff9b::7f00:1",
	"64:ff9b::a9fe:a9fe",
	"64:ff9b:1::1",
	"100::1",
	"2001::1",
	"2001:2::1",
	"2001:10::1",
	"2001:20::1",
	"2001:db8::1",
	"2002:7f00:1::",
	"2002:a9fe:a9fe::",
	"3fff::1",
	"5f00::1",
	"fd00::1",
	"fd00:ec2::254",
	"fe80::1",
	"fec0::1",
	"ff02::1",
	// A zoned literal, escaped as a URL carries it. The lookup hands it back
	// with its zone, which is not an IP net.ParseIP can read.
	"fe80::1%25lo0",
}

// urlPolicyAdmittedAddresses are public addresses, including the IPv6 forms
// that carry a public IPv4 address, so the refusals above are not a policy
// that refuses everything.
var urlPolicyAdmittedAddresses = []string{
	"93.184.215.14",
	"8.8.8.8",
	"168.63.129.17",
	"2606:4700:4700::1111",
	"::ffff:8.8.8.8",
	"64:ff9b::808:808",
	"2002:808:808::1",
}

func TestURLPolicyRefusesSpecialPurposeAddressLiterals(t *testing.T) {
	for _, address := range urlPolicyRefusedAddresses {
		rawURL := "http://" + net.JoinHostPort(address, "80") + "/.well-known/agent.json"
		if err := ValidateExternalURL(rawURL); err == nil {
			t.Errorf("ValidateExternalURL admitted %s", rawURL)
		}
	}
}

func TestURLPolicyAdmitsPublicAddressLiterals(t *testing.T) {
	for _, address := range urlPolicyAdmittedAddresses {
		rawURL := "https://" + net.JoinHostPort(address, "443") + "/.well-known/agent.json"
		if err := ValidateExternalURL(rawURL); err != nil {
			t.Errorf("ValidateExternalURL refused %s: %v", rawURL, err)
		}
	}
}
