package domain

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ProviderDIDPrefix is the DID method prefix of the provider's self-identifier.
// AIP-SPEC 3.2: the provider identifies itself as did:web:<provider-host>, the host
// that publishes its DID Document at https://<provider-host>/.well-known/did.json.
const ProviderDIDPrefix = "did:web:"

// BuildProviderDID returns the provider's did:web self-identifier for the public
// origin the deployment is served from, e.g. https://aim.opena2a.org becomes
// did:web:aim.opena2a.org.
//
// The host is lowercased, and a non-default port is kept and percent-encoded as the
// did:web method requires (http://localhost:3000 becomes did:web:localhost%3A3000),
// because a bare colon would read as a path separator. A path on the origin is not
// part of the provider identifier. An origin with no DNS host has no did:web form
// and returns an error.
func BuildProviderDID(origin string) (string, error) {
	raw := strings.TrimSpace(origin)
	if raw == "" {
		return "", errors.New("provider origin is empty")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("provider origin %q is not a URL: %w", origin, err)
	}

	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "", fmt.Errorf("provider origin %q names no host", origin)
	}
	if strings.Contains(host, ":") {
		return "", fmt.Errorf("provider origin %q is an IPv6 literal; did:web needs a DNS name", origin)
	}

	port := u.Port()
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		host += "%3A" + port
	}

	return ProviderDIDPrefix + host, nil
}
