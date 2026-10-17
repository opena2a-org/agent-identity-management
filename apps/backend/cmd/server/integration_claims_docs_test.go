package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// What the documentation says about integrations the server does not ship yet.
// Measured 2026-10-16 at c679aff: the cyberark package is imported by nothing,
// the Microsoft Sentinel adapter is not loaded, and Splunk forwarding runs in
// self-hosted deployments only. A Markdown line that names CyberArk, Sentinel,
// PagerDuty or Microsoft Teams states its status. When one of them is proved end
// to end in CI, the change that proves it takes its term out of
// unshippedIntegration and rewrites its lines in the same commit.
//
// PagerDuty is out of the term list: an organization's alerts reach it through
// the Events API v2, proved by TestAlertChannelsRoundTrip
// (alert_channels_integration_test.go, integration tag).

// unshippedIntegration matches the name of an integration the server does not
// ship.
var unshippedIntegration = regexp.MustCompile(`(?i)cyberark|sentinel|microsoft teams`)

// integrationStatus matches the words that state such an integration's status.
var integrationStatus = regexp.MustCompile(`(?i)\b(planned|under evaluation|not available|not loaded|not supported)\b`)

// retiredIntegrationClaims are, per file, the claims the documentation no
// longer makes.
var retiredIntegrationClaims = map[string][]string{
	"README.md": {
		"All AIM Cloud features.",
		"PAM, SIEM adapters.",
	},
	"docs/server-features.md": {
		"the CyberArk and SIEM integrations",
		"CCP for vaulted credential retrieval. PSM for privileged session recording.",
		"the Data Collector API it targets was retired on 14 September 2026",
	},
	"docs/DEPLOYMENT.md": {
		"targets was retired on 14 September 2026",
	},
	"docs/API.md": {
		"\"secret\": \"your-webhook-secret\"",
		"a receiver cannot check that header yet",
		"a `secret` field in the request is ignored",
		"An event name outside the table below is accepted and never sent.",
	},
	"apps/backend/internal/integrations/siem/adapter.go": {
		"which Microsoft retired on 14 September 2026",
	},
}

// measuredIntegrationClaims are, per file, the statements of the measured state
// the documentation makes instead.
var measuredIntegrationClaims = map[string][]string{
	"README.md": {
		"MCP attestation, PAM, audit log forwarding to Splunk. |",
	},
	"docs/server-features.md": {
		"except Splunk forwarding, which runs in self-hosted deployments only. A CyberArk connector and a Microsoft Sentinel adapter are planned.",
		"## CyberArk integration (planned)",
		"Sending session events to Privileged Session Manager (PSM) is under evaluation",
		"Microsoft ended support for the HTTP Data Collector API it targets on 14 September 2026. A Sentinel adapter for the Logs Ingestion API is planned.",
	},
	"docs/DEPLOYMENT.md": {
		"Microsoft ended support for the Azure Monitor HTTP Data Collector API, which the source tree's Sentinel adapter targets, on 14 September 2026.",
	},
	"docs/API.md": {
		"The create answer is the only one that carries the secret",
		"| `{\"error\":\"unsupported event: auth.failed\"}` | An event name that is not one of those in [Webhook Events](#webhook-events). |",
	},
	"apps/backend/internal/integrations/siem/adapter.go": {
		"// Data Collector API, for which Microsoft ended support on 14 September 2026,",
	},
}

// unshippedIntegrationFindings returns file:line for every line of text that
// names an integration the server does not ship and states no status for it.
func unshippedIntegrationFindings(name, text string) []string {
	var findings []string
	for i, line := range strings.Split(text, "\n") {
		if unshippedIntegration.MatchString(line) && !integrationStatus.MatchString(line) {
			findings = append(findings, fmt.Sprintf("%s:%d: %s", name, i+1, strings.TrimSpace(line)))
		}
	}
	return findings
}

// integrationDocFiles returns the tracked Markdown files, changelogs left out.
func integrationDocFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	for _, rel := range providerSignInDocFiles(t, root) {
		if strings.HasSuffix(rel, ".md") {
			files = append(files, rel)
		}
	}
	return files
}

func TestIntegrationDocsDropTheRetiredClaims(t *testing.T) {
	requirePublicRepositoryFiles(t)

	root := aim03RepoRoot(t)
	for file, claims := range retiredIntegrationClaims {
		body, err := os.ReadFile(filepath.Join(root, file))
		require.NoError(t, err, file)
		for _, claim := range claims {
			assert.False(t, strings.Contains(string(body), claim), "%s still says %q", file, claim)
		}
	}
}

func TestIntegrationDocsCarryTheMeasuredState(t *testing.T) {
	requirePublicRepositoryFiles(t)

	root := aim03RepoRoot(t)
	for file, claims := range measuredIntegrationClaims {
		body, err := os.ReadFile(filepath.Join(root, file))
		require.NoError(t, err, file)
		for _, claim := range claims {
			assert.True(t, strings.Contains(string(body), claim), "%s does not say %q", file, claim)
		}
	}
}

func TestUnshippedIntegrationsCarryAStatusInTheDocs(t *testing.T) {
	requirePublicRepositoryFiles(t)

	root := aim03RepoRoot(t)

	var findings []string
	for _, rel := range integrationDocFiles(t, root) {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if errors.Is(err, fs.ErrNotExist) {
			// Tracked but deleted from the working tree: there is no text to read.
			continue
		}
		require.NoError(t, err, rel)
		findings = append(findings, unshippedIntegrationFindings(rel, string(body))...)
	}
	assert.Empty(t, findings,
		"a Markdown line names an integration the server does not ship, without a status: mark it planned, under evaluation or not available, or prove it end to end first:\n%s",
		strings.Join(findings, "\n"))
}

// Slack is not in the term list: the example guides post to Slack from the
// agent's own code, not through the server. PagerDuty is not in it either: the
// server ships it.
func TestUnshippedIntegrationFindings(t *testing.T) {
	sample := strings.Join([]string{
		"CCP for vaulted credential retrieval through CyberArk.",
		"## CyberArk integration (planned)",
		"Microsoft Sentinel is not supported: the source tree's adapter is not loaded.",
		"Send alerts to PagerDuty",
		"Slack Notifications for Business Travel",
	}, "\n")
	assert.Equal(t, []string{
		"sample.md:1: CCP for vaulted credential retrieval through CyberArk.",
	}, unshippedIntegrationFindings("sample.md", sample))
}
