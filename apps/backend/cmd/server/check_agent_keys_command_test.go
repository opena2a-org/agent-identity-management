package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
)

// `aim-server check-agent-keys` with a missing database variable exits 1 with one line
// naming the variable.
func TestCheckAgentKeysCommand_MissingDatabaseVariableIsOneLine(t *testing.T) {
	logs := captureLog(t)
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_USER", "")
	t.Setenv("POSTGRES_DB", "aim")

	assert.Equal(t, 1, runCheckAgentKeysCommand())
	out := strings.TrimSpace(logs.String())
	assert.Equal(t, 0, strings.Count(out, "\n"), "one line: %s", out)
	assert.Contains(t, out, "POSTGRES_USER")
}

// A master key generated for the run opens no stored key, so every key would be listed as
// unreadable: the command refuses to run on one, before it connects to the database.
func TestCheckAgentKeysCommand_RefusesAMasterKeyGeneratedForTheRun(t *testing.T) {
	logs := captureLog(t)
	t.Setenv("POSTGRES_HOST", "localhost")
	t.Setenv("POSTGRES_USER", "aim")
	t.Setenv("POSTGRES_DB", "aim")
	t.Setenv("ENVIRONMENT", "development")
	t.Setenv("KEYVAULT_MASTER_KEY", "")
	t.Setenv("KEYVAULT_MASTER_KEY_ID", "")

	assert.Equal(t, 1, runCheckAgentKeysCommand())
	out := logs.String()
	assert.Contains(t, out, "KeyVault: KEYVAULT_MASTER_KEY is not set, and a master key generated for this run opens no stored key")
	assert.NotContains(t, out, "Failed to connect to database")
}

func TestWriteAgentPrivateKeyCheck_OneLinePerAgentThenWhatReplacesEachKey(t *testing.T) {
	legacyID, copyID := uuid.MustParse("1a000000-0000-4000-8000-000000000000"), uuid.MustParse("1b000000-0000-4000-8000-000000000000")
	orgID := uuid.MustParse("2a000000-0000-4000-8000-000000000000")
	unopenable := []application.UnopenableAgentPrivateKey{
		{AgentID: legacyID, OrganizationID: orgID, V1: true},
		{AgentID: copyID, OrganizationID: orgID},
	}
	rotate := "until the agent's credentials are rotated: POST /api/v1/agents/<agent>/rotate-credentials"

	t.Run("conversion recorded complete", func(t *testing.T) {
		logs := captureLog(t)
		var out bytes.Buffer
		writeAgentPrivateKeyCheck(&out, application.AgentPrivateKeyCheck{Read: 3, ConversionComplete: true, Unopenable: unopenable})

		assert.Equal(t,
			"agent=1a000000-0000-4000-8000-000000000000 organization=2a000000-0000-4000-8000-000000000000 key=v1\n"+
				"agent=1b000000-0000-4000-8000-000000000000 organization=2a000000-0000-4000-8000-000000000000 key=unreadable\n",
			out.String())
		assert.Equal(t, []string{
			"Agent private keys: read=3 unopenable=2 v1=1 unreadable=1",
			"The startup conversion is recorded complete and reads no key again, so a v1 key stays refused " + rotate,
			"An unreadable key stays refused " + rotate,
		}, strings.Split(strings.TrimSpace(logs.String()), "\n"))
	})

	t.Run("conversion not recorded complete", func(t *testing.T) {
		logs := captureLog(t)
		var out bytes.Buffer
		writeAgentPrivateKeyCheck(&out, application.AgentPrivateKeyCheck{Read: 1, Unopenable: unopenable[:1]})

		assert.Equal(t, "agent=1a000000-0000-4000-8000-000000000000 organization=2a000000-0000-4000-8000-000000000000 key=v1\n", out.String())
		assert.Equal(t, []string{
			"Agent private keys: read=1 unopenable=1 v1=1 unreadable=0",
			"The startup conversion is not recorded complete, so it runs again at the next server start and converts the v1 keys it reads",
		}, strings.Split(strings.TrimSpace(logs.String()), "\n"))
	})

	t.Run("every key opens", func(t *testing.T) {
		logs := captureLog(t)
		var out bytes.Buffer
		writeAgentPrivateKeyCheck(&out, application.AgentPrivateKeyCheck{Read: 5, ConversionComplete: true, Unopenable: []application.UnopenableAgentPrivateKey{}})

		assert.Empty(t, out.String())
		assert.Equal(t, "Agent private keys: read=5 unopenable=0 v1=0 unreadable=0", strings.TrimSpace(logs.String()))
	})
}
