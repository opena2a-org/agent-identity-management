package siem

import (
	"strings"
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

// sentinelVars sets the six required SIEM_SENTINEL_* variables. The secret is
// assembled at run time; it is a placeholder, not a credential.
func sentinelVars() map[string]string {
	return map[string]string{
		EnvSentinelEndpoint:       "https://dce.example.test/",
		EnvSentinelDCRImmutableID: "dcr-0123456789abcdef",
		EnvSentinelStreamName:     "Custom-AIMSecurityEvents",
		EnvSentinelTenantID:       "tenant-value",
		EnvSentinelClientID:       "client-value",
		EnvSentinelClientSecret:   fakeSentinelSecret,
	}
}

var fakeSentinelSecret = strings.Join([]string{"fake", "sentinel", "secret"}, "-")

func TestSettingsFromEnv_SentinelNothingSet(t *testing.T) {
	for _, vars := range []map[string]string{nil, {EnvSentinelAuthorityHost: "https://login.example.test"}} {
		settings, err := SettingsFromEnv(envOf(vars))
		require.NoError(t, err)
		assert.Empty(t, settings.Adapters, "without the six required variables Sentinel is not configured")
	}
}

func TestSettingsFromEnv_Sentinel(t *testing.T) {
	vars := sentinelVars()
	vars[EnvSentinelClientSecret] = " " + fakeSentinelSecret + " "
	settings, err := SettingsFromEnv(envOf(vars))
	require.NoError(t, err)
	require.Len(t, settings.Adapters, 1)
	sentinel, ok := settings.Adapters[0].(*SentinelAdapter)
	require.True(t, ok)
	assert.Equal(t, "sentinel", sentinel.Name())
	assert.Equal(t, SentinelConfig{
		Endpoint:       "https://dce.example.test/",
		DCRImmutableID: "dcr-0123456789abcdef",
		StreamName:     "Custom-AIMSecurityEvents",
		TenantID:       "tenant-value",
		ClientID:       "client-value",
		ClientSecret:   fakeSentinelSecret,
		AuthorityHost:  "https://login.microsoftonline.com",
	}, sentinel.config)

	vars[EnvSentinelAuthorityHost] = "https://login.example.test"
	settings, err = SettingsFromEnv(envOf(vars))
	require.NoError(t, err)
	assert.Equal(t, "https://login.example.test", settings.Adapters[0].(*SentinelAdapter).config.AuthorityHost)
}

func TestSettingsFromEnv_SplunkAndSentinel(t *testing.T) {
	vars := sentinelVars()
	vars[EnvSplunkHECURL] = "https://splunk.example.com:8088/services/collector/event"
	vars[EnvSplunkHECToken] = "fake-hec-token"
	settings, err := SettingsFromEnv(envOf(vars))
	require.NoError(t, err)
	require.Len(t, settings.Adapters, 2, "each event goes to both")
	assert.Equal(t, "splunk", settings.Adapters[0].Name())
	assert.Equal(t, "sentinel", settings.Adapters[1].Name())
}

func TestSettingsFromEnv_SentinelRefusals(t *testing.T) {
	without := func(names ...string) map[string]string {
		vars := sentinelVars()
		for _, name := range names {
			delete(vars, name)
		}
		return vars
	}
	with := func(name, value string) map[string]string {
		vars := sentinelVars()
		vars[name] = value
		return vars
	}
	cases := []struct {
		name string
		vars map[string]string
		want string
	}{
		{"secret missing", without(EnvSentinelClientSecret),
			"SIEM_SENTINEL_CLIENT_SECRET is not set; set all of SIEM_SENTINEL_ENDPOINT, SIEM_SENTINEL_DCR_IMMUTABLE_ID, SIEM_SENTINEL_STREAM_NAME, SIEM_SENTINEL_TENANT_ID, SIEM_SENTINEL_CLIENT_ID, SIEM_SENTINEL_CLIENT_SECRET to forward to Microsoft Sentinel"},
		{"endpoint missing", without(EnvSentinelEndpoint), "SIEM_SENTINEL_ENDPOINT is not set;"},
		{"two missing", without(EnvSentinelTenantID, EnvSentinelStreamName), "SIEM_SENTINEL_STREAM_NAME, SIEM_SENTINEL_TENANT_ID are not set;"},
		{"blank counts as missing", with(EnvSentinelDCRImmutableID, "  "), "SIEM_SENTINEL_DCR_IMMUTABLE_ID is not set;"},
		{"http endpoint to a remote host", with(EnvSentinelEndpoint, "http://dce.example.test"), "SIEM_SENTINEL_ENDPOINT must use https; http is accepted only for a loopback host"},
		{"endpoint not a url", with(EnvSentinelEndpoint, "dce.example.test"), "SIEM_SENTINEL_ENDPOINT must be an absolute https URL"},
		{"http authority to a remote host", with(EnvSentinelAuthorityHost, "http://login.example.test"), "SIEM_SENTINEL_AUTHORITY_HOST must use https; http is accepted only for a loopback host"},
		{"authority not a url", with(EnvSentinelAuthorityHost, "login.example.test"), "SIEM_SENTINEL_AUTHORITY_HOST must be an absolute URL such as https://login.microsoftonline.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := SettingsFromEnv(envOf(tc.vars))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			for name, value := range sentinelVars() {
				if name != EnvSentinelEndpoint {
					assert.NotContains(t, err.Error(), value, "an error never carries the value of %s", name)
				}
			}
		})
	}
}

func TestSettingsFromEnv_SentinelHTTPToALoopbackHost(t *testing.T) {
	vars := sentinelVars()
	vars[EnvSentinelEndpoint] = "http://127.0.0.1:9000"
	vars[EnvSentinelAuthorityHost] = "http://localhost:9001"
	settings, err := SettingsFromEnv(envOf(vars))
	require.NoError(t, err)
	assert.Len(t, settings.Adapters, 1)
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
