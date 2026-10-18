package main

import (
	"errors"
	"log"
	"net/http"
	"net/url"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
	infraatc "github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/atc"
)

// revocationListStartupLine returns the line the server logs at startup for
// the revocation list REGISTRY_URL and ATC_CRL_ENDPOINT configure. Without
// REGISTRY_URL no list is read, and ATC_CRL_ENDPOINT is not read either. With
// it, an error, which stops the server, is returned when REGISTRY_URL or a set
// ATC_CRL_ENDPOINT is not an https URL with a host. The line names the scheme
// and host of REGISTRY_URL, and the scheme, host and path of
// ATC_CRL_ENDPOINT: no user information and no query.
func revocationListStartupLine(registry config.RegistryConfig) (string, error) {
	if registry.URL == "" {
		if registry.ATCCRLEndpoint != "" {
			return "Revocation list: not read (REGISTRY_URL is not set; ATC_CRL_ENDPOINT is read only with it)", nil
		}
		return "Revocation list: not read (REGISTRY_URL is not set)", nil
	}
	base, ok := httpsURL(registry.URL)
	if !ok {
		return "", errors.New("REGISTRY_URL must be an https URL with a host")
	}
	authority := (&url.URL{Scheme: base.Scheme, Host: base.Host}).String()
	if registry.ATCCRLEndpoint == "" {
		return "Revocation list: read in the background from the authority at " + authority, nil
	}
	override, ok := httpsURL(registry.ATCCRLEndpoint)
	if !ok {
		return "", errors.New("ATC_CRL_ENDPOINT must be an https URL with a host")
	}
	list := (&url.URL{Scheme: override.Scheme, Host: override.Host, Path: override.Path, RawPath: override.RawPath}).String()
	return "Revocation list: read in the background from " + list + ", with the keys of the authority at " + authority, nil
}

// httpsURL parses raw as an https URL with a host, the form the revocation
// list client takes.
func httpsURL(raw string) (*url.URL, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, false
	}
	return u, true
}

// startRevocationList logs the revocation list's startup line and, when
// REGISTRY_URL is set, starts the refresher that holds the signed revocation
// list of the authority whose discovery document is served at REGISTRY_URL,
// read from the address ATC_CRL_ENDPOINT names when it is set. It returns a
// nil refresher when REGISTRY_URL is not set, and the error of
// revocationListStartupLine. No request the server answers fetches the list:
// GET /health/ready reads the refresher's State. The server passes a nil
// httpClient, and the list client then reads with an HTTP client of its own.
func startRevocationList(registry config.RegistryConfig, httpClient *http.Client) (*infraatc.CRLRefresher, error) {
	line, err := revocationListStartupLine(registry)
	if err != nil {
		return nil, err
	}
	log.Print(line)
	if registry.URL == "" {
		return nil, nil
	}
	client, err := infraatc.NewCachedCRLClient(registry.URL, registry.ATCCRLEndpoint, httpClient)
	if err != nil {
		return nil, err
	}
	refresher := infraatc.NewCRLRefresher(client)
	refresher.Start()
	return refresher, nil
}
