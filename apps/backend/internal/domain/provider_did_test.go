package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildProviderDID pins the provider self-identifier AIP-SPEC 3.2 requires,
// did:web:<provider-host>, including the did:web rule that a port is
// percent-encoded so it is not read as a path separator.
func TestBuildProviderDID(t *testing.T) {
	cases := []struct {
		name   string
		origin string
		want   string
	}{
		{"hosted service", "https://aim.opena2a.org", "did:web:aim.opena2a.org"},
		{"trailing slash", "https://aim.opena2a.org/", "did:web:aim.opena2a.org"},
		{"surrounding whitespace", "  https://aim.opena2a.org \n", "did:web:aim.opena2a.org"},
		{"host is lowercased", "https://AIM.Example.COM", "did:web:aim.example.com"},
		{"path is not part of the provider identifier", "https://aim.example.com/dashboard", "did:web:aim.example.com"},
		{"port is percent-encoded", "http://localhost:3000", "did:web:localhost%3A3000"},
		{"https default port is dropped", "https://aim.example.com:443", "did:web:aim.example.com"},
		{"http default port is dropped", "http://aim.example.com:80", "did:web:aim.example.com"},
		{"non-default https port is kept", "https://aim.example.com:8443", "did:web:aim.example.com%3A8443"},
		{"origin without a scheme", "aim.example.com", "did:web:aim.example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildProviderDID(tc.origin)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// An origin that names no DNS host has no did:web form. Serving a made-up
// identifier would be worse than serving none, so the builder refuses.
func TestBuildProviderDIDRejectsOriginsWithoutAHost(t *testing.T) {
	for _, origin := range []string{
		"",
		"   ",
		"https://",
		"https://:3000",
		"https://aim.example.com:notaport",
		"http://[::1]:3000",
	} {
		t.Run(origin, func(t *testing.T) {
			got, err := BuildProviderDID(origin)
			assert.Error(t, err)
			assert.Empty(t, got)
		})
	}
}
