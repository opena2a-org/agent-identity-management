package application

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validateClaudeDesktopConfigPath is the allowlist guard that prevents the
// authenticated, member-level detect-MCP endpoint from being turned into an
// arbitrary server-side file read (LFI). These tests pin that behavior so a
// future refactor can't silently re-open the hole.
func TestValidateClaudeDesktopConfigPath_RejectsArbitraryPaths(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	rejected := []struct {
		name string
		path string
	}{
		{"empty", ""},
		{"etc passwd", "/etc/passwd"},
		{"proc environ (env secret leak)", "/proc/self/environ"},
		{"ssh key", "/root/.ssh/id_rsa"},
		{"relative traversal", "../../../../etc/passwd"},
		{"tilde traversal escape", "~/../../etc/passwd"},
		{"right basename wrong dir", "/tmp/claude_desktop_config.json"},
		{"known dir wrong basename", filepath.Join(home, ".config", "Claude", "passwd")},
		{"trailing traversal to sibling", filepath.Join(home, ".config", "Claude", "..", "secrets.json")},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateClaudeDesktopConfigPath(tc.path)
			assert.Error(t, err, "expected %q to be rejected", tc.path)
			assert.Empty(t, got)
		})
	}
}

func TestValidateClaudeDesktopConfigPath_AcceptsStandardLocations(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	accepted := []string{
		filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"), // macOS
		filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"),                        // Linux
		filepath.Join(home, "AppData", "Roaming", "Claude", "claude_desktop_config.json"),             // Windows
	}
	for _, p := range accepted {
		got, err := validateClaudeDesktopConfigPath(p)
		assert.NoError(t, err, "expected %q to be accepted", p)
		assert.Equal(t, p, got)
	}
}

func TestValidateClaudeDesktopConfigPath_ExpandsTilde(t *testing.T) {
	got, err := validateClaudeDesktopConfigPath("~/.config/Claude/claude_desktop_config.json")
	require.NoError(t, err)

	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"), got)
}
