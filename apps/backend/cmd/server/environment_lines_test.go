package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/cache"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/email"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// The environment table of internal/config: each row sets one variable and
// records the setting config.Load resolves and the startup line the server
// logs for it. internal/config checks the settings; this file checks the
// lines, through the functions main logs them with.
const environmentTablePath = "../../internal/config/testdata/environment_table.json"

type environmentTable struct {
	Baseline map[string]string `json:"baseline"`
	Rows     []environmentRow  `json:"rows"`
}

type environmentRow struct {
	Variable string            `json:"variable"`
	Input    string            `json:"input"`
	Value    *string           `json:"value"`
	With     map[string]string `json:"with,omitempty"`
	Setting  string            `json:"setting"`
	Line     string            `json:"line"`
}

func readEnvironmentTable(t *testing.T) environmentTable {
	t.Helper()
	raw, err := os.ReadFile(environmentTablePath)
	if err != nil {
		t.Fatalf("reading the environment table: %v", err)
	}
	var table environmentTable
	if err := json.Unmarshal(raw, &table); err != nil {
		t.Fatalf("parsing the environment table: %v", err)
	}
	return table
}

// apply leaves every variable of the table unset, then sets the baseline,
// the row's other variables and the row's own value.
func (table environmentTable) apply(t *testing.T, row environmentRow) {
	t.Helper()
	unset := func(name string) {
		t.Setenv(name, "")
		os.Unsetenv(name)
	}
	for _, r := range table.Rows {
		unset(r.Variable)
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

func loadConfigForRow() (cfg *config.Config) {
	defer func() {
		if recover() != nil {
			cfg = nil
		}
	}()
	loaded, err := config.Load()
	if err != nil {
		return nil
	}
	return loaded
}

// logLines runs fn and returns the lines it logs that contain marker.
func logLines(marker string, fn func()) string {
	var buf bytes.Buffer
	flags, out := log.Flags(), log.Writer()
	log.SetFlags(0)
	log.SetOutput(&buf)
	defer func() {
		log.SetFlags(flags)
		log.SetOutput(out)
	}()
	fn()
	var lines []string
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.Contains(line, marker) {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func signingKeyLinesForRow(cfg *config.Config) string {
	keyVault, err := crypto.NewKeyVaultFromEnv()
	if err != nil {
		return "error: " + err.Error()
	}
	signingKeys, err := loadServerSigningKeys(keyVault, cfg.SigningKeys.Value)
	if err != nil {
		return "error: " + err.Error()
	}
	return strings.Join(signingKeyStartupLines(signingKeys, cfg.Server.Environment), "\n")
}

// startupLine returns the startup line the server logs for name with cfg, ""
// where it logs none.
func startupLine(t *testing.T, name string, cfg *config.Config) string {
	t.Helper()
	switch name {
	case "APP_PORT":
		return startupSummary(cfg, nil)[0]
	case "POSTGRES_HOST", "POSTGRES_USER", "POSTGRES_PORT":
		return startupSummary(cfg, nil)[1]
	case "REDIS_PORT":
		return startupSummary(cfg, nil)[2]
	case "REDIS_HOST":
		cacheLine := logLines("Cache service", func() { initCache(cfg, nil) })
		return redisStartupSummary(cfg, nil) + "\n" + cacheLine + "\n" + tokenRevocationStartupLine(cfg, nil, nil, false)
	case "AUTH_REVOCATION_FAIL_OPEN":
		return tokenRevocationStartupLine(cfg, nil, &cache.RedisCache{}, cfg.Auth.RevocationFailOpen)
	case "ENVIRONMENT", "AIM_SIGNING_KEY_CARD_ATTESTATION", "AIM_SIGNING_KEY_CARD_ATTESTATION_RETIRED",
		"AIM_SIGNING_KEY_ATC_ISSUER", "AIM_SIGNING_KEY_ATC_ISSUER_RETIRED":
		return signingKeyLinesForRow(cfg)
	case "EMAIL_PROVIDER", "EMAIL_FROM_ADDRESS":
		// The email package reads the same two variables to choose the
		// provider and the sender: the line names what it uses.
		emailCfg, _ := email.LoadEmailConfigFromEnv()
		if emailCfg.Provider != cfg.Email.Provider || emailCfg.FromAddress != cfg.Email.FromAddress {
			t.Errorf("the startup line names provider %q from %q; the email package uses %q from %q",
				cfg.Email.Provider, cfg.Email.FromAddress, emailCfg.Provider, emailCfg.FromAddress)
		}
		return emailStartupLine(cfg.Email)
	case "ADMIN_PASSWORD", "ADMIN_EMAIL":
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		if cfg.Admin.Password != "" {
			mock.ExpectQuery(`SELECT EXISTS`).WithArgs(sqlmock.AnyArg()).
				WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		}
		var seedErr error
		line := logLines("ADMIN_PASSWORD", func() { seedErr = seedAdmin(db, cfg.Admin) })
		if seedErr != nil {
			t.Fatalf("seeding the administrator: %v", seedErr)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("seeding the administrator: %v", err)
		}
		return line
	case "SIEM_SPLUNK_HEC_URL", "SIEM_SPLUNK_HEC_TOKEN", "SIEM_SPLUNK_INDEX", "SIEM_MIN_SEVERITY",
		"SIEM_SENTINEL_ENDPOINT", "SIEM_SENTINEL_DCR_IMMUTABLE_ID", "SIEM_SENTINEL_STREAM_NAME",
		"SIEM_SENTINEL_TENANT_ID", "SIEM_SENTINEL_CLIENT_ID", "SIEM_SENTINEL_CLIENT_SECRET",
		"SIEM_SENTINEL_AUTHORITY_HOST":
		db, _, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		var forwardErr error
		line := logLines("siem:", func() {
			forwarder, err := startSIEMForwarding(repository.NewAuditLogRepository(db), cfg.SIEM.Value)
			forwardErr = err
			if forwarder != nil {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_ = forwarder.Close(ctx)
			}
		})
		if forwardErr != nil {
			return "error: " + forwardErr.Error()
		}
		return line
	case "CYBERARK_CCP_URL", "CYBERARK_CCP_APP_ID", "CYBERARK_CCP_SAFE",
		"CYBERARK_CCP_CERT_PATH", "CYBERARK_CCP_KEY_PATH", "CYBERARK_CCP_CA_CERT_PATH":
		return logLines("CyberArk CCP", func() { cyberArkCCPBackend(cfg.Secrets.CyberArkCCP) })
	case "REGISTRY_URL", "ATC_CRL_ENDPOINT":
		line, err := revocationListStartupLine(cfg.Registry)
		if err != nil {
			return "error: " + err.Error()
		}
		return line
	case "DRIFT_MONITOR_INTERVAL":
		return logLines(name, func() {
			_, warning := driftMonitorInterval(cfg.Jobs.Value)
			logIntervalWarning(warning)
		})
	case "TRUST_REEVALUATION_INTERVAL":
		return logLines(name, func() {
			_, warning := trustReevaluationInterval(cfg.Jobs.Value)
			logIntervalWarning(warning)
		})
	case "AIM_PUBLIC_ORIGIN":
		return agentRequestAudienceStartupLine(cfg.Server)
	}
	return ""
}

// TestEnvironmentTable_StartupLines checks each row's startup line against
// the one recorded in the table.
func TestEnvironmentTable_StartupLines(t *testing.T) {
	table := readEnvironmentTable(t)
	if len(table.Rows) == 0 {
		t.Fatal("the environment table has no rows")
	}
	withLine := 0
	for _, row := range table.Rows {
		if row.Line != "" {
			withLine++
		}
		t.Run(fmt.Sprintf("%s/%s", row.Variable, row.Input), func(t *testing.T) {
			table.apply(t, row)
			got := ""
			if cfg := loadConfigForRow(); cfg != nil {
				got = startupLine(t, row.Variable, cfg)
			}
			if got != row.Line {
				t.Errorf("%s %s: startup line = %q, want %q", row.Variable, row.Input, got, row.Line)
			}
		})
	}
	t.Logf("%d rows, %d with a startup line", len(table.Rows), withLine)
}
