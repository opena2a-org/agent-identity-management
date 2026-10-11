package application

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// Several pages said a call for a capability the agent was not granted is
// refused, with no enforcement mode named. The capability check refuses it
// only in strict mode; a new organization starts in monitoring mode, which lets
// the call run (#681). The check is run here in both modes, and each page that
// shows the refusal must say that it needs strict mode, that a new organization
// starts in monitoring mode and, where the page walks a reader through it,
// where an administrator turns strict mode on.

// docsWords returns text with every run of whitespace replaced by one space, so
// a phrase wrapped over two lines still matches.
func docsWords(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func TestEnforcementModeDocs_PagesSayTheRefusalNeedsStrictMode(t *testing.T) {
	requirePublicRepositoryFiles(t)

	root := repoRoot(t)

	t.Run("only strict mode refuses a capability the agent was not granted", func(t *testing.T) {
		for _, tc := range []struct {
			mode    domain.EnforcementMode
			allowed bool
		}{
			{domain.EnforcementModeMonitoring, true},
			{domain.EnforcementModeStrict, false},
		} {
			f := newCapabilityCheckFixture(t, tc.mode, "db:read")
			allowed, reason, _, err := f.service.VerifyCapability(context.Background(), f.agent.ID, "network:fetch", "external-api", nil, "")
			require.NoError(t, err)
			assert.Equal(t, tc.allowed, allowed, "%s mode: %s", tc.mode, reason)
		}
	})

	t.Run("strict mode refuses every action of an agent that is not verified", func(t *testing.T) {
		for _, tc := range []struct {
			mode    domain.EnforcementMode
			allowed bool
		}{
			{domain.EnforcementModeMonitoring, true},
			{domain.EnforcementModeStrict, false},
		} {
			f := newCapabilityCheckFixture(t, tc.mode, "api:call")
			f.agent.Status = domain.AgentStatusPending
			allowed, reason, _, err := f.service.VerifyCapability(context.Background(), f.agent.ID, "api:call", "weather.api/New York", nil, "")
			require.NoError(t, err)
			assert.Equal(t, tc.allowed, allowed, "%s mode: %s", tc.mode, reason)
		}
	})

	t.Run("a new organization starts in monitoring mode", func(t *testing.T) {
		migration := aim02ReadFile(t, root, "apps/backend/migrations/054_add_enforcement_mode_to_organizations.sql")
		assert.Contains(t, migration, "enforcement_mode VARCHAR(20) NOT NULL DEFAULT 'monitoring'")
		repo := aim02ReadFile(t, root, "apps/backend/internal/infrastructure/repository/organization_repository.go")
		assert.Contains(t, docsWords(repo), `if org.EnforcementMode == "" { org.EnforcementMode = domain.EnforcementModeMonitoring }`)
	})

	// The setting the walkthrough pages name: the switch on the dashboard's
	// security policies page and the admin route behind it.
	settingsPage := aim02ReadFile(t, root, "apps/web/app/dashboard/admin/security-policies/page.tsx")
	require.Contains(t, settingsPage, "Global Enforcement Mode")
	require.Contains(t, settingsPage, "Enable Strict Mode")
	routes := aim02ReadFile(t, root, "apps/backend/cmd/server/main.go")
	require.Contains(t, routes, `admin.Get("/enforcement-settings"`)
	require.Contains(t, routes, `admin.Put("/enforcement-settings"`)

	pages := []struct {
		rel         string
		walkthrough bool
		// unqualified is the sentence the page carried that said the call is
		// refused without naming a mode.
		unqualified string
	}{
		{"docs/quick-start.md", true, "Runs ONLY if verification passes"},
		{"docs/use-cases/enforce-capabilities.md", true, "Actions not in the capabilities list are denied automatically"},
		{"docs/DEMO_SCRIPT_ECHOLEAK.md", true, "This demo shows how **AIM prevents EchoLeak-style prompt-injection attacks**"},
		{"ROADMAP.md", false, "does not run, in every enforcement mode"},
	}
	for _, page := range pages {
		t.Run(page.rel, func(t *testing.T) {
			text := docsWords(aim02ReadFile(t, root, page.rel))
			// Boolean checks: a failing Contains would print the whole page.
			assert.False(t, strings.Contains(text, page.unqualified), "%s says the call is refused without naming strict mode: %q", page.rel, page.unqualified)
			assert.True(t, strings.Contains(text, "starts in monitoring mode"), "%s must say a new organization starts in monitoring mode", page.rel)
			assert.True(t, strings.Contains(strings.ToLower(text), "only in strict mode"), "%s must say the refusal needs strict mode", page.rel)
			if page.walkthrough {
				for _, want := range []string{
					"Global Enforcement Mode",
					"/dashboard/admin/security-policies",
					"PUT /api/v1/admin/enforcement-settings",
					`{"enforcementMode": "strict"}`,
				} {
					assert.True(t, strings.Contains(text, want), "%s must name where strict mode is set: %q", page.rel, want)
				}
			}
		})
	}

	t.Run("the demo script has the agent verified before its legitimate request", func(t *testing.T) {
		agentPage := aim02ReadFile(t, root, "apps/web/app/dashboard/agents/[id]/page.tsx")
		require.Contains(t, agentPage, "Verify agent")

		const rel = "docs/DEMO_SCRIPT_ECHOLEAK.md"
		text := docsWords(aim02ReadFile(t, root, rel))
		verify := strings.Index(text, "**Verify agent**")
		part2 := strings.Index(text, "# **Part 2: Legitimate Weather Request (Allowed)**")
		require.True(t, verify != -1, "%s must tell the reader to press Verify agent", rel)
		require.True(t, part2 != -1, "%s must keep its Part 2 heading", rel)
		assert.True(t, verify < part2, "%s must have the agent verified before Part 2", rel)
	})
}
