package crypto

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// The compose stack passes the backend only the variables its environment list
// names, so a signing key variable missing there has no effect in a compose
// deployment: a rotation that lists the old key in a _RETIRED variable would
// publish no retired key.
func TestDockerComposePassesEverySigningKeyVariable(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docker-compose.yml"))
	require.NoError(t, err)
	var compose struct {
		Services map[string]struct {
			Environment yaml.Node `yaml:"environment"`
		} `yaml:"services"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &compose))
	backend, ok := compose.Services["backend"]
	require.True(t, ok, "docker-compose.yml defines no backend service")

	// Compose accepts a list of NAME=value or a mapping.
	passed := map[string]bool{}
	env := backend.Environment
	switch env.Kind {
	case yaml.SequenceNode:
		for _, item := range env.Content {
			name, _, _ := strings.Cut(item.Value, "=")
			passed[name] = true
		}
	case yaml.MappingNode:
		for i := 0; i < len(env.Content); i += 2 {
			passed[env.Content[i].Value] = true
		}
	default:
		t.Fatalf("the backend environment at line %d is neither a list nor a mapping", env.Line)
	}
	for _, purpose := range signingPurposes {
		for _, name := range []string{SigningKeyEnvVar(purpose), SigningKeyEnvVar(purpose) + retiredKeysEnvSuffix} {
			require.True(t, passed[name], "the backend service in docker-compose.yml does not pass %s", name)
		}
	}
}
