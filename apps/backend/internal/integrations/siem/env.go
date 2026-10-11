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
)

// Settings is the SIEM configuration the environment gives: the adapters to
// deliver to (none when no SIEM is configured) and the Forwarder's bounds.
type Settings struct {
	Adapters  []BatchAdapter
	Forwarder ForwarderConfig
}

// SettingsFromEnv reads the SIEM_* variables through getenv. Splunk HEC is
// configured by SIEM_SPLUNK_HEC_URL and SIEM_SPLUNK_HEC_TOKEN together. The
// URL must be https, or http to a loopback host, so the token is never sent
// in clear text across a network. An error names the variable at fault and
// never carries a variable's value.
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
		if err := checkCollectorURL(hecURL); err != nil {
			return Settings{}, fmt.Errorf("%s %w", EnvSplunkHECURL, err)
		}
		settings.Adapters = append(settings.Adapters, NewSplunkAdapter(SplunkConfig{
			HECURL:   hecURL,
			HECToken: hecToken,
			Index:    read(EnvSplunkIndex),
		}))
	}

	return settings, nil
}

func checkCollectorURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("must be an absolute URL such as https://splunk.example.com:8088/services/collector/event")
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
