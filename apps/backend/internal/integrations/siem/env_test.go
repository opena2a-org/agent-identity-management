package siem

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func envOf(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

func TestSettingsFromEnv_NothingSet(t *testing.T) {
	settings, err := SettingsFromEnv(envOf(nil))
	require.NoError(t, err)
	assert.Empty(t, settings.Adapters)
}

func TestSettingsFromEnv_Splunk(t *testing.T) {
	settings, err := SettingsFromEnv(envOf(map[string]string{
		EnvSplunkHECURL:   " https://splunk.example.com:8088/services/collector/event ",
		EnvSplunkHECToken: "fake-hec-token",
		EnvSplunkIndex:    "aim",
		EnvMinSeverity:    "MEDIUM",
	}))
	require.NoError(t, err)
	require.Len(t, settings.Adapters, 1)
	splunk, ok := settings.Adapters[0].(*SplunkAdapter)
	require.True(t, ok)
	assert.Equal(t, "https://splunk.example.com:8088/services/collector/event", splunk.config.HECURL)
	assert.Equal(t, "fake-hec-token", splunk.config.HECToken)
	assert.Equal(t, "aim", splunk.config.Index)
	assert.Equal(t, SeverityMedium, settings.Forwarder.MinSeverity)
}

func TestSettingsFromEnv_HTTPToALoopbackHost(t *testing.T) {
	for _, u := range []string{"http://127.0.0.1:8088/services/collector/event", "http://localhost:8088/x", "http://[::1]:8088/x"} {
		settings, err := SettingsFromEnv(envOf(map[string]string{EnvSplunkHECURL: u, EnvSplunkHECToken: "fake-hec-token"}))
		require.NoError(t, err, u)
		assert.Len(t, settings.Adapters, 1, u)
	}
}

func TestSettingsFromEnv_Refusals(t *testing.T) {
	cases := []struct {
		name string
		vars map[string]string
		want string
	}{
		{"url without token", map[string]string{EnvSplunkHECURL: "https://splunk.example.com/x"}, "SIEM_SPLUNK_HEC_URL is set but SIEM_SPLUNK_HEC_TOKEN is not"},
		{"token without url", map[string]string{EnvSplunkHECToken: "fake-hec-token"}, "SIEM_SPLUNK_HEC_TOKEN is set but SIEM_SPLUNK_HEC_URL is not"},
		{"http to a remote host", map[string]string{EnvSplunkHECURL: "http://splunk.example.com:8088/x", EnvSplunkHECToken: "fake-hec-token"}, "SIEM_SPLUNK_HEC_URL must use https"},
		{"not a url", map[string]string{EnvSplunkHECURL: "splunk.example.com", EnvSplunkHECToken: "fake-hec-token"}, "SIEM_SPLUNK_HEC_URL must be an absolute URL"},
		{"other scheme", map[string]string{EnvSplunkHECURL: "ftp://splunk.example.com/x", EnvSplunkHECToken: "fake-hec-token"}, "SIEM_SPLUNK_HEC_URL must use https"},
		{"unknown severity", map[string]string{EnvMinSeverity: "warning"}, "SIEM_MIN_SEVERITY must be one of info, low, medium, high, critical"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := SettingsFromEnv(envOf(tc.vars))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assert.NotContains(t, err.Error(), "fake-hec-token", "an error never carries the token")
		})
	}
}
