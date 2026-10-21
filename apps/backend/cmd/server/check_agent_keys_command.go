package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
)

// checkAgentKeysCommandName is the argument that runs the agent private key check instead
// of the server: `aim-server check-agent-keys`.
const checkAgentKeysCommandName = "check-agent-keys"

// runCheckAgentKeysCommand writes to standard output one line for each agent whose stored
// private key does not open for the agent's own row (application.CheckAgentPrivateKeys),
// logs the counts and what replaces each kind of key, and exits without starting the
// server. It reads the POSTGRES_* variables, as `aim-server migrate` does, and the
// KEYVAULT_MASTER_KEY variables the server reads, and it writes nothing to the database.
// It refuses to run on a master key generated for the run, which opens no stored key. It
// returns the process exit code: 0 done, 1 failed.
func runCheckAgentKeysCommand() int {
	database, err := config.LoadDatabase()
	if err != nil {
		log.Printf("Database configuration: %v", err)
		return 1
	}
	keyVault, err := crypto.NewKeyVaultFromEnv()
	if err != nil {
		log.Printf("KeyVault: %v", err)
		return 1
	}
	if keyVault.MasterKeySource() == crypto.MasterKeySourceEphemeral {
		log.Printf("KeyVault: %s is not set, and a master key generated for this run opens no stored key; set the master key the server runs with", crypto.MasterKeyEnvVar)
		return 1
	}
	db, err := initMigrationHandle(&config.Config{Database: database})
	if err != nil {
		log.Printf("Failed to connect to database: %v", err)
		return 1
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	check, err := application.CheckAgentPrivateKeys(ctx, db, keyVault)
	if err != nil {
		log.Printf("Agent private key check: %v", err)
		return 1
	}
	writeAgentPrivateKeyCheck(os.Stdout, check)
	return 0
}

// writeAgentPrivateKeyCheck writes to out one line for each agent whose stored key does not
// open, `agent=<id> organization=<id> key=v1` or `key=unreadable`, and then logs the counts
// and what replaces each kind of key.
func writeAgentPrivateKeyCheck(out io.Writer, check application.AgentPrivateKeyCheck) {
	v1 := 0
	for _, key := range check.Unopenable {
		format := "unreadable"
		if key.V1 {
			format = "v1"
			v1++
		}
		fmt.Fprintf(out, "agent=%s organization=%s key=%s\n", key.AgentID, key.OrganizationID, format)
	}
	unreadable := len(check.Unopenable) - v1
	log.Printf("Agent private keys: read=%d unopenable=%d v1=%d unreadable=%d", check.Read, len(check.Unopenable), v1, unreadable)
	if v1 > 0 && check.ConversionComplete {
		log.Printf("The startup conversion is recorded complete and reads no key again, so a v1 key stays refused until the agent's credentials are rotated: POST /api/v1/agents/<agent>/rotate-credentials")
	}
	if v1 > 0 && !check.ConversionComplete {
		log.Printf("The startup conversion is not recorded complete, so it runs again at the next server start and converts the v1 keys it reads")
	}
	if unreadable > 0 {
		log.Printf("An unreadable key stays refused until the agent's credentials are rotated: POST /api/v1/agents/<agent>/rotate-credentials")
	}
}
