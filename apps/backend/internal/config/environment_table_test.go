package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/siem"
)

// environmentTable is testdata/environment_table.json: for every variable the
// server process reads, and for each input (unset, empty, whitespace only, a
// valid value, an invalid value, and for passwords and tokens a quote and a
// backslash), the setting the server resolves and the line it logs at
// startup. Every expected value was recorded by running the code paths that
// read each variable before Load read them all. cmd/server checks the lines.
type environmentTable struct {
	// Baseline is set for every row: the required variables.
	Baseline map[string]string `json:"baseline"`
	Rows     []environmentRow  `json:"rows"`
}

type environmentRow struct {
	Variable string `json:"variable"`
	Input    string `json:"input"`
	// Value is the row's value of Variable; nil leaves it unset.
	Value *string `json:"value"`
	// With are other variables the row sets, after Baseline.
	With map[string]string `json:"with,omitempty"`
	// Setting is the resolved setting, rendered by renderSetting, or the
	// error or panic of Load.
	Setting string `json:"setting"`
	// Line is the startup line, "" where the server logs none.
	Line string `json:"line"`
}

func readEnvironmentTable(t *testing.T) environmentTable {
	t.Helper()
	raw, err := os.ReadFile("testdata/environment_table.json")
	if err != nil {
		t.Fatalf("reading the environment table: %v", err)
	}
	var table environmentTable
	if err := json.Unmarshal(raw, &table); err != nil {
		t.Fatalf("parsing the environment table: %v", err)
	}
	return table
}

func (table environmentTable) variables() []string {
	seen := map[string]bool{}
	var names []string
	for _, row := range table.Rows {
		if !seen[row.Variable] {
			seen[row.Variable] = true
			names = append(names, row.Variable)
		}
	}
	return names
}

// apply leaves every variable of the table unset, then sets the baseline,
// the row's other variables and the row's own value. No container marker file
// exists for the row, so HOST's default is the one outside a container
// wherever the table runs.
func (table environmentTable) apply(t *testing.T, row environmentRow) {
	t.Helper()
	noContainerMarkers(t)
	unset := func(name string) {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	for _, name := range table.variables() {
		unset(name)
	}
	for name, value := range table.Baseline {
		t.Setenv(name, value)
	}
	for name, value := range row.With {
		t.Setenv(name, value)
	}
	if row.Value == nil {
		unset(row.Variable)
	} else {
		t.Setenv(row.Variable, *row.Value)
	}
}

// loadForRow runs Load and renders its error or panic the way the table
// records them.
func loadForRow() (cfg *Config, failure string) {
	defer func() {
		if r := recover(); r != nil {
			cfg, failure = nil, fmt.Sprintf("panic: %v", r)
		}
	}()
	loaded, err := Load()
	if err != nil {
		return nil, "error: " + err.Error()
	}
	return loaded, ""
}

// renderSetting renders the setting cfg holds for name: strings quoted,
// numbers, flags and durations as Go prints them.
func renderSetting(cfg *Config, name string) (string, bool) {
	q := func(s string) string { return fmt.Sprintf("%q", s) }
	switch name {
	case "HOST":
		return q(cfg.Server.Host), true
	case "KUBERNETES_SERVICE_HOST":
		// Load reads it only to tell whether the server runs in a container,
		// which sets the host HOST's default resolves to.
		return q(cfg.Server.Host), true
	case "APP_PORT":
		return q(cfg.Server.Port), true
	case "ENVIRONMENT":
		return q(cfg.Server.Environment), true
	case "LOG_LEVEL":
		return q(cfg.Server.LogLevel), true
	case "FRONTEND_URL":
		return q(cfg.Server.FrontendURL), true
	case "METRICS_AUTH_TOKEN":
		return q(cfg.Server.MetricsAuthToken), true
	case "METRICS_LISTEN_ADDR":
		return q(cfg.Server.MetricsListenAddr), true
	case "AIM_PUBLIC_ORIGIN":
		return q(cfg.Server.PublicOrigin), true
	case "ALLOWED_ORIGINS":
		return fmt.Sprintf("%q", cfg.Server.AllowedOrigins), true
	case "REDIS_HOST":
		return fmt.Sprintf("%q configured=%t disabled=%t", cfg.Redis.Host, cfg.Redis.Configured, cfg.Redis.Disabled), true
	case "REDIS_PORT":
		return fmt.Sprintf("%d", cfg.Redis.Port), true
	case "REDIS_PASSWORD":
		return q(cfg.Redis.Password), true
	case "REDIS_DB":
		return fmt.Sprintf("%d", cfg.Redis.DB), true
	case "REDIS_USE_TLS":
		return fmt.Sprintf("%t", cfg.Redis.UseTLS), true
	case "JWT_SECRET":
		return q(cfg.JWT.Secret), true
	case "GOOGLE_CLIENT_ID":
		return q(cfg.OAuth.Google.ClientID), true
	case "GOOGLE_CLIENT_SECRET":
		return q(cfg.OAuth.Google.ClientSecret), true
	case "GOOGLE_REDIRECT_URL":
		return q(cfg.OAuth.Google.RedirectURL), true
	case "MICROSOFT_CLIENT_ID":
		return q(cfg.OAuth.Microsoft.ClientID), true
	case "MICROSOFT_CLIENT_SECRET":
		return q(cfg.OAuth.Microsoft.ClientSecret), true
	case "MICROSOFT_REDIRECT_URL":
		return q(cfg.OAuth.Microsoft.RedirectURL), true
	case "OKTA_CLIENT_ID":
		return q(cfg.OAuth.Okta.ClientID), true
	case "OKTA_CLIENT_SECRET":
		return q(cfg.OAuth.Okta.ClientSecret), true
	case "OKTA_DOMAIN":
		return q(cfg.OAuth.Okta.Domain), true
	case "OKTA_REDIRECT_URL":
		return q(cfg.OAuth.Okta.RedirectURL), true
	case "POSTGRES_HOST":
		return q(cfg.Database.Host), true
	case "POSTGRES_PORT":
		return fmt.Sprintf("%d", cfg.Database.Port), true
	case "POSTGRES_USER":
		return q(cfg.Database.User), true
	case "POSTGRES_PASSWORD":
		return q(cfg.Database.Password), true
	case "POSTGRES_DB":
		return q(cfg.Database.Database), true
	case "POSTGRES_SSL_MODE":
		return q(cfg.Database.SSLMode), true
	case "POSTGRES_MAX_CONNECTIONS":
		return fmt.Sprintf("%d", cfg.Database.MaxConnections), true
	case "POSTGRES_CONN_MAX_LIFETIME":
		return cfg.Database.ConnMaxLifetime.String(), true
	case "POSTGRES_POOL_ROLE":
		return q(cfg.Database.PoolRole), true
	case "POSTGRES_SCOPE_CENSUS":
		return fmt.Sprintf("%t", cfg.Database.ScopeCensus), true
	case "POSTGRES_APP_USER":
		return q(cfg.Database.AppUser), true
	case "POSTGRES_APP_PASSWORD":
		return q(cfg.Database.AppPassword), true
	case "KEYVAULT_MASTER_KEY":
		// Load only checks it against the known development values; the key
		// vault reads it.
		return "accepted", true
	case "AUTH_REVOCATION_FAIL_OPEN":
		return fmt.Sprintf("%t", cfg.Auth.RevocationFailOpen), true
	case "ADMIN_PASSWORD":
		return q(cfg.Admin.Password), true
	case "ADMIN_EMAIL":
		return q(cfg.Admin.Email), true
	case "ADMIN_NAME":
		return q(cfg.Admin.Name), true
	case "REGISTRY_BRIDGE_URL":
		return q(cfg.Registry.BridgeURL), true
	case "REGISTRY_BRIDGE_ENABLED":
		return fmt.Sprintf("%t", cfg.Registry.BridgeEnabled), true
	case "ATC_ISSUER_URI":
		return q(cfg.Registry.ATCIssuerURI), true
	case "REGISTRY_URL":
		return q(cfg.Registry.URL), true
	case "ATC_CRL_ENDPOINT":
		return q(cfg.Registry.ATCCRLEndpoint), true
	case "VAULT_ADDR":
		return q(cfg.Secrets.Vault.Addr), true
	case "VAULT_MOUNT_PATH":
		return q(cfg.Secrets.Vault.MountPath), true
	case "VAULT_AUTH_PATH":
		return q(cfg.Secrets.Vault.AuthPath), true
	case "VAULT_ROLE":
		return q(cfg.Secrets.Vault.Role), true
	case "VAULT_TOKEN":
		return q(cfg.Secrets.Vault.Token), true
	case "VAULT_NAMESPACE":
		return q(cfg.Secrets.Vault.Namespace), true
	case "AWS_SECRETS_REGION":
		return q(cfg.Secrets.AWS.Region), true
	case "AWS_SECRETS_KMS_KEY_ID":
		return q(cfg.Secrets.AWS.KMSKeyID), true
	case "AWS_SECRETS_PREFIX":
		return q(cfg.Secrets.AWS.Prefix), true
	case "AZURE_KEYVAULT_URL":
		return q(cfg.Secrets.AzureVaultURL), true
	case "GCP_SECRETS_PROJECT":
		return q(cfg.Secrets.GCPProject), true
	case "OP_CONNECT_HOST":
		return q(cfg.Secrets.OnePassword.ConnectHost), true
	case "OP_CONNECT_TOKEN":
		return q(cfg.Secrets.OnePassword.ConnectToken), true
	case "OP_VAULT_UUID":
		return q(cfg.Secrets.OnePassword.VaultUUID), true
	case "FGA_DAEMON_URL":
		return q(cfg.FGA.DaemonURL), true
	case "FGA_REGISTRY_ASC_URL":
		return q(cfg.FGA.RegistryASCURL), true
	case "EMAIL_PROVIDER":
		return q(cfg.Email.Provider), true
	case "EMAIL_FROM_ADDRESS":
		return q(cfg.Email.FromAddress), true
	case "SIEM_SPLUNK_HEC_URL", "SIEM_SPLUNK_HEC_TOKEN", "SIEM_SPLUNK_INDEX", "SIEM_MIN_SEVERITY",
		"SIEM_SENTINEL_ENDPOINT", "SIEM_SENTINEL_DCR_IMMUTABLE_ID", "SIEM_SENTINEL_STREAM_NAME",
		"SIEM_SENTINEL_TENANT_ID", "SIEM_SENTINEL_CLIENT_ID", "SIEM_SENTINEL_CLIENT_SECRET",
		"SIEM_SENTINEL_AUTHORITY_HOST":
		return q(cfg.SIEM.Value(name)), true
	case "AIM_SIGNING_KEY_CARD_ATTESTATION", "AIM_SIGNING_KEY_CARD_ATTESTATION_RETIRED",
		"AIM_SIGNING_KEY_ATC_ISSUER", "AIM_SIGNING_KEY_ATC_ISSUER_RETIRED":
		return q(cfg.SigningKeys.Value(name)), true
	case "DRIFT_MONITOR_INTERVAL", "TRUST_REEVALUATION_INTERVAL":
		return q(cfg.Jobs.Value(name)), true
	}
	return "", false
}

// TestEnvironmentTable_ResolvesEveryVariableAsRecorded checks each row's
// resolved setting against the one recorded in the table.
func TestEnvironmentTable_ResolvesEveryVariableAsRecorded(t *testing.T) {
	table := readEnvironmentTable(t)
	if len(table.Rows) == 0 {
		t.Fatal("the environment table has no rows")
	}
	for _, row := range table.Rows {
		t.Run(row.Variable+"/"+row.Input, func(t *testing.T) {
			table.apply(t, row)
			cfg, failure := loadForRow()
			got := failure
			if cfg != nil {
				setting, ok := renderSetting(cfg, row.Variable)
				if !ok {
					t.Fatalf("Config holds no setting for %s", row.Variable)
				}
				got = setting
			}
			if got != row.Setting {
				t.Errorf("%s %s: setting = %s, want %s", row.Variable, row.Input, got, row.Setting)
			}
		})
	}
	t.Logf("%d rows, %d variables", len(table.Rows), len(table.variables()))
}

// recordLookups makes lookupEnv record each name it is asked for.
func recordLookups(t *testing.T) map[string]bool {
	t.Helper()
	asked := map[string]bool{}
	original := lookupEnv
	lookupEnv = func(key string) (string, bool) {
		asked[key] = true
		return original(key)
	}
	t.Cleanup(func() { lookupEnv = original })
	return asked
}

func sortedNames(set map[string]bool) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// TestEnvironmentTable_NamesEveryVariableLoadReads fails when Load reads a
// variable through lookupEnv that the table has no rows for, or the table has
// rows for a variable Load does not read. A read that bypasses lookupEnv is
// caught by TestConfigReadsTheEnvironmentOnlyThroughLookupEnv in cmd/server.
func TestEnvironmentTable_NamesEveryVariableLoadReads(t *testing.T) {
	table := readEnvironmentTable(t)
	table.apply(t, environmentRow{Variable: "APP_PORT"})
	asked := recordLookups(t)
	if _, failure := loadForRow(); failure != "" {
		t.Fatalf("Load with the baseline: %s", failure)
	}

	inTable := map[string]bool{}
	for _, name := range table.variables() {
		inTable[name] = true
	}
	var notInTable, notRead []string
	for _, name := range sortedNames(asked) {
		if !inTable[name] {
			notInTable = append(notInTable, name)
		}
	}
	for _, name := range sortedNames(inTable) {
		if !asked[name] {
			notRead = append(notRead, name)
		}
	}
	if len(notInTable) > 0 {
		t.Errorf("Load reads %s, which the environment table has no rows for; record them from the code that reads them", strings.Join(notInTable, ", "))
	}
	if len(notRead) > 0 {
		t.Errorf("the environment table has rows for %s, which Load does not read", strings.Join(notRead, ", "))
	}
}

// TestEnvironmentTable_DelegatedReadersAskOnlyForHeldNames checks that the
// SIEM settings and the signing key ring, which read their variables through
// SIEMConfig.Value and SigningKeysConfig.Value, ask for exactly the names
// those hold: a name they do not hold would read as unset.
func TestEnvironmentTable_DelegatedReadersAskOnlyForHeldNames(t *testing.T) {
	table := readEnvironmentTable(t)
	// SIEM_SPLUNK_INDEX is read only with a Splunk URL and token: the row
	// that sets it sets them too.
	var indexRow *environmentRow
	for i, row := range table.Rows {
		if row.Variable == "SIEM_SPLUNK_INDEX" && row.Input == "valid" {
			indexRow = &table.Rows[i]
		}
	}
	if indexRow == nil {
		t.Fatal("the environment table has no valid SIEM_SPLUNK_INDEX row")
	}
	table.apply(t, *indexRow)
	cfg, failure := loadForRow()
	if failure != "" {
		t.Fatalf("Load with the baseline: %s", failure)
	}

	held := func(prefix string) map[string]bool {
		names := map[string]bool{}
		for _, name := range table.variables() {
			if strings.HasPrefix(name, prefix) {
				names[name] = true
			}
		}
		return names
	}
	recording := func(asked map[string]bool, value func(string) string) func(string) string {
		return func(name string) string {
			asked[name] = true
			return value(name)
		}
	}

	siemAsked := map[string]bool{}
	if _, err := siem.SettingsFromEnv(recording(siemAsked, cfg.SIEM.Value)); err != nil {
		t.Fatalf("SIEM settings: %v", err)
	}
	if got, want := sortedNames(siemAsked), sortedNames(held("SIEM_")); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("siem.SettingsFromEnv asks for %v; SIEMConfig holds %v", got, want)
	}

	kv, err := crypto.NewKeyVault(table.Baseline["KEYVAULT_MASTER_KEY"])
	if err != nil {
		t.Fatalf("key vault: %v", err)
	}
	keysAsked := map[string]bool{}
	if _, err := crypto.LoadSigningKeyRing(kv, recording(keysAsked, cfg.SigningKeys.Value)); err != nil {
		t.Fatalf("signing keys: %v", err)
	}
	if got, want := sortedNames(keysAsked), sortedNames(held("AIM_SIGNING_KEY_")); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("crypto.LoadSigningKeyRing asks for %v; SigningKeysConfig holds %v", got, want)
	}
}
