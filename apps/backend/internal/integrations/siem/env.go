package siem

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// The environment variables SettingsFromEnv reads.
const (
	EnvSplunkHECURL   = "SIEM_SPLUNK_HEC_URL"
	EnvSplunkHECToken = "SIEM_SPLUNK_HEC_TOKEN"
	EnvSplunkIndex    = "SIEM_SPLUNK_INDEX"
	EnvMinSeverity    = "SIEM_MIN_SEVERITY"

	EnvSentinelEndpoint       = "SIEM_SENTINEL_ENDPOINT"
	EnvSentinelDCRImmutableID = "SIEM_SENTINEL_DCR_IMMUTABLE_ID"
	EnvSentinelStreamName     = "SIEM_SENTINEL_STREAM_NAME"
	EnvSentinelTenantID       = "SIEM_SENTINEL_TENANT_ID"
	EnvSentinelClientID       = "SIEM_SENTINEL_CLIENT_ID"
	EnvSentinelClientSecret   = "SIEM_SENTINEL_CLIENT_SECRET"
	EnvSentinelAuthorityHost  = "SIEM_SENTINEL_AUTHORITY_HOST"
)

// sentinelRequired are the variables that configure Microsoft Sentinel
// together.
var sentinelRequired = []string{
	EnvSentinelEndpoint,
	EnvSentinelDCRImmutableID,
	EnvSentinelStreamName,
	EnvSentinelTenantID,
	EnvSentinelClientID,
	EnvSentinelClientSecret,
}

// Settings is the SIEM configuration the environment gives: the adapters to
// deliver to (none when no SIEM is configured) and the Forwarder's bounds.
type Settings struct {
	Adapters  []BatchAdapter
	Forwarder ForwarderConfig
}

// SettingsFromEnv reads the SIEM_* variables through getenv. Splunk HEC is
// configured by SIEM_SPLUNK_HEC_URL and SIEM_SPLUNK_HEC_TOKEN together.
// Microsoft Sentinel is configured by the six SIEM_SENTINEL_* variables in
// sentinelRequired together, with SIEM_SENTINEL_AUTHORITY_HOST optional. When
// both are configured, each event goes to both. Every URL must be https, or
// http to a loopback host, so no token or secret is sent in clear text across
// a network. An error names the variable at fault and never carries a
// variable's value.
func SettingsFromEnv(getenv func(string) string) (Settings, error) {
	read := func(name string) string { return strings.TrimSpace(getenv(name)) }

	var settings Settings

	minSeverity := strings.ToLower(read(EnvMinSeverity))
	if minSeverity != "" && !ValidSeverity(minSeverity) {
		return Settings{}, fmt.Errorf("%s must be one of info, low, medium, high, critical", EnvMinSeverity)
	}
	settings.Forwarder.MinSeverity = minSeverity

	hecURL, hecToken := read(EnvSplunkHECURL), read(EnvSplunkHECToken)
	switch {
	case hecURL == "" && hecToken == "":
	case hecURL == "":
		return Settings{}, fmt.Errorf("%s is set but %s is not; set both to forward to Splunk HEC", EnvSplunkHECToken, EnvSplunkHECURL)
	case hecToken == "":
		return Settings{}, fmt.Errorf("%s is set but %s is not; set both to forward to Splunk HEC", EnvSplunkHECURL, EnvSplunkHECToken)
	default:
		if err := checkCollectorURL(hecURL, "https://splunk.example.com:8088/services/collector/event"); err != nil {
			return Settings{}, fmt.Errorf("%s %w", EnvSplunkHECURL, err)
		}
		settings.Adapters = append(settings.Adapters, NewSplunkAdapter(SplunkConfig{
			HECURL:   hecURL,
			HECToken: hecToken,
			Index:    read(EnvSplunkIndex),
		}))
	}

	sentinel, err := sentinelFromEnv(read)
	if err != nil {
		return Settings{}, err
	}
	if sentinel != nil {
		settings.Adapters = append(settings.Adapters, sentinel)
	}

	return settings, nil
}

// sentinelFromEnv returns the Sentinel adapter the SIEM_SENTINEL_* variables
// configure, or nil when none of the six required ones is set. It reads all
// seven whether or not Sentinel is configured: internal/config checks that the
// SIEM settings ask for exactly the names config.SIEMConfig holds.
func sentinelFromEnv(read func(string) string) (*SentinelAdapter, error) {
	config := SentinelConfig{
		Endpoint:       read(EnvSentinelEndpoint),
		DCRImmutableID: read(EnvSentinelDCRImmutableID),
		StreamName:     read(EnvSentinelStreamName),
		TenantID:       read(EnvSentinelTenantID),
		ClientID:       read(EnvSentinelClientID),
		ClientSecret:   read(EnvSentinelClientSecret),
		AuthorityHost:  read(EnvSentinelAuthorityHost),
	}

	var missing []string
	for _, name := range sentinelRequired {
		if read(name) == "" {
			missing = append(missing, name)
		}
	}
	switch {
	case len(missing) == len(sentinelRequired):
		return nil, nil
	case len(missing) == 1:
		return nil, fmt.Errorf("%s is not set; set all of %s to forward to Microsoft Sentinel",
			missing[0], strings.Join(sentinelRequired, ", "))
	case len(missing) > 1:
		return nil, fmt.Errorf("%s are not set; set all of %s to forward to Microsoft Sentinel",
			strings.Join(missing, ", "), strings.Join(sentinelRequired, ", "))
	}

	if err := checkCollectorURL(config.Endpoint, ""); err != nil {
		return nil, fmt.Errorf("%s %w", EnvSentinelEndpoint, err)
	}
	if config.AuthorityHost != "" {
		if err := checkCollectorURL(config.AuthorityHost, DefaultSentinelAuthorityHost); err != nil {
			return nil, fmt.Errorf("%s %w", EnvSentinelAuthorityHost, err)
		}
	}
	return NewSentinelAdapter(config), nil
}

// checkCollectorURL refuses a URL that is not absolute, or that is not https
// unless its host is loopback. example, when set, is quoted in the refusal of
// a URL that is not absolute.
func checkCollectorURL(raw, example string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		if example == "" {
			return fmt.Errorf("must be an absolute https URL")
		}
		return fmt.Errorf("must be an absolute URL such as %s", example)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		host := u.Hostname()
		if ip := net.ParseIP(host); host == "localhost" || (ip != nil && ip.IsLoopback()) {
			return nil
		}
		return fmt.Errorf("must use https; http is accepted only for a loopback host")
	default:
		return fmt.Errorf("must use https")
	}
}
