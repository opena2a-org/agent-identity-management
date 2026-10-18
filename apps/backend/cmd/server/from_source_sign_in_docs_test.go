package main

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// fromSourceComposeService is the part of a compose service the From source
// section check reads.
type fromSourceComposeService struct {
	Profiles    []string  `yaml:"profiles"`
	Entrypoint  yaml.Node `yaml:"entrypoint"`
	Environment yaml.Node `yaml:"environment"`
	DependsOn   yaml.Node `yaml:"depends_on"`
	Ports       yaml.Node `yaml:"ports"`
}

// fromSourceHeading opens the From source section of docs/DEPLOYMENT.md; the
// section runs to the next second-level heading.
const fromSourceHeading = "### From source\n"

var (
	// fromSourceComposeUp captures the services a `docker compose up` line of
	// the section starts.
	fromSourceComposeUp = regexp.MustCompile(`(?m)^docker compose up(?: -d)?((?: [a-z][a-z0-9_-]*)+)$`)
	// fromSourceComposeRun captures the service a `docker compose run --rm`
	// line of the section runs.
	fromSourceComposeRun = regexp.MustCompile(`(?m)^docker compose run --rm ([a-z][a-z0-9_-]*)$`)
	// passwordVariableName matches the name of an environment variable that
	// holds a password.
	passwordVariableName = regexp.MustCompile(`\b[A-Z][A-Z0-9_]*PASSWORD\b`)
	// devSecretsVariable captures a variable scripts/gen-dev-secrets.sh writes
	// to .env.
	devSecretsVariable = regexp.MustCompile(`(?m)^\$\{prefix\}([A-Z][A-Z0-9_]*)=`)
	// bootstrapDefaultEmail captures the administrator email
	// `aim-bootstrap --default` creates.
	bootstrapDefaultEmail = regexp.MustCompile(`defaultAdminEmail\s*=\s*"([^"]+)"`)
	// composeHostPortDefault captures the default host port of a compose ports
	// entry such as ${AIM_FRONTEND_PORT:-3000}:3000.
	composeHostPortDefault = regexp.MustCompile(`^\$\{[A-Z_]+:-(\d+)\}:\d+$`)
)

// yamlKeys returns the keys of a mapping node, or for a sequence of NAME=value
// scalars the names: the two forms compose accepts for environment and
// depends_on.
func yamlKeys(n yaml.Node) []string {
	var keys []string
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			keys = append(keys, n.Content[i].Value)
		}
	case yaml.SequenceNode:
		for _, item := range n.Content {
			name, _, _ := strings.Cut(item.Value, "=")
			keys = append(keys, name)
		}
	}
	return keys
}

// yamlScalars returns the scalars of a sequence node, or the words of a scalar
// node: the two forms compose accepts for entrypoint and ports.
func yamlScalars(n yaml.Node) []string {
	switch n.Kind {
	case yaml.SequenceNode:
		var values []string
		for _, item := range n.Content {
			values = append(values, item.Value)
		}
		return values
	case yaml.ScalarNode:
		return strings.Fields(n.Value)
	}
	return nil
}

// fromSourceSection returns the From source section of the deployment guide,
// from its heading to the next second-level heading.
func fromSourceSection(t *testing.T, guide string) string {
	t.Helper()
	start := strings.Index(guide, fromSourceHeading)
	require.GreaterOrEqual(t, start, 0, "docs/DEPLOYMENT.md has no From source section")
	section := guide[start:]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	return section
}

// TestFromSourceSectionSaysHowToSignIn pins the From source section of
// docs/DEPLOYMENT.md to the stack it starts. The section had the reader write
// .env with scripts/gen-dev-secrets.sh, start postgres, redis, backend and
// frontend and check the API's health, and then named no account and no
// password, so a reader who built the stack from source had nothing to sign
// in with. The section must run the compose service that runs aim-bootstrap,
// after the `docker compose up` that starts the services it depends on, and
// name the administrator email that command creates, the dashboard's URL and
// the password variable; every password variable it names must be one the
// script writes to .env.
func TestFromSourceSectionSaysHowToSignIn(t *testing.T) {
	requirePublicRepositoryFiles(t)

	var compose struct {
		Services map[string]fromSourceComposeService `yaml:"services"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(aim03ReadRepoFile(t, "docker-compose.yml")), &compose))

	bootstrapName := ""
	for name, svc := range compose.Services {
		for _, word := range yamlScalars(svc.Entrypoint) {
			if word == "aim-bootstrap" || strings.HasSuffix(word, "/aim-bootstrap") {
				bootstrapName = name
			}
		}
	}
	require.NotEmpty(t, bootstrapName, "docker-compose.yml has no service whose entrypoint runs aim-bootstrap; the compose scan is broken, not the guide")
	bootstrap := compose.Services[bootstrapName]
	require.NotEmpty(t, bootstrap.Profiles, "the %s service is in no profile, so `docker compose up` would start it; the compose scan is broken, not the guide", bootstrapName)
	require.Contains(t, yamlKeys(bootstrap.Environment), "DEFAULT_ADMIN_PASSWORD", "the %s service passes no DEFAULT_ADMIN_PASSWORD; the compose scan is broken, not the guide", bootstrapName)
	dependsOn := yamlKeys(bootstrap.DependsOn)
	require.NotEmpty(t, dependsOn, "the %s service depends on no service; the compose scan is broken, not the guide", bootstrapName)

	frontend, ok := compose.Services["frontend"]
	require.True(t, ok, "docker-compose.yml has no frontend service; the compose scan is broken, not the guide")
	dashboardPort := ""
	for _, port := range yamlScalars(frontend.Ports) {
		if m := composeHostPortDefault.FindStringSubmatch(port); m != nil {
			dashboardPort = m[1]
		}
	}
	require.NotEmpty(t, dashboardPort, "the frontend service publishes no host port with a default; the compose scan is broken, not the guide")

	written := map[string]bool{}
	for _, m := range devSecretsVariable.FindAllStringSubmatch(aim03ReadRepoFile(t, "scripts/gen-dev-secrets.sh"), -1) {
		written[m[1]] = true
	}
	require.True(t, written["DEFAULT_ADMIN_PASSWORD"], "scripts/gen-dev-secrets.sh writes no DEFAULT_ADMIN_PASSWORD; the script scan is broken, not the guide")

	emailMatch := bootstrapDefaultEmail.FindStringSubmatch(aim03ReadRepoFile(t, "apps/backend/cmd/bootstrap/main.go"))
	require.NotNil(t, emailMatch, "apps/backend/cmd/bootstrap/main.go sets no defaultAdminEmail; the source scan is broken, not the guide")
	email := emailMatch[1]

	section := fromSourceSection(t, aim03ReadRepoFile(t, "docs/DEPLOYMENT.md"))

	ups := fromSourceComposeUp.FindAllStringSubmatchIndex(section, -1)
	require.Len(t, ups, 1, "the From source section has one `docker compose up` that starts the dev stack")
	started := strings.Fields(section[ups[0][2]:ups[0][3]])
	for _, dep := range dependsOn {
		assert.Contains(t, started, dep, "the section's `docker compose up` starts %s, which the %s service depends on", dep, bootstrapName)
	}

	runAt := -1
	for _, run := range fromSourceComposeRun.FindAllStringSubmatchIndex(section, -1) {
		if section[run[2]:run[3]] == bootstrapName {
			runAt = run[0]
		}
	}
	require.NotEqual(t, -1, runAt, "the From source section never runs `docker compose run --rm %s`, the step that creates the first administrator", bootstrapName)
	assert.Greater(t, runAt, ups[0][0], "the section runs %s before the `docker compose up` that starts the services it depends on", bootstrapName)

	assert.Contains(t, section, "`"+email+"`", "the section names the administrator email `%s` that aim-bootstrap --default creates", email)
	assert.Contains(t, section, "http://localhost:"+dashboardPort, "the section names the dashboard's URL, host port %s", dashboardPort)

	named := map[string]bool{}
	for _, name := range passwordVariableName.FindAllString(section, -1) {
		named[name] = true
	}
	assert.True(t, named["DEFAULT_ADMIN_PASSWORD"], "the section names DEFAULT_ADMIN_PASSWORD, the variable the %s service reads the password from", bootstrapName)
	for name := range named {
		assert.True(t, written[name], "the section names %s, which scripts/gen-dev-secrets.sh does not write to .env, so the reader has no value for it", name)
	}
}
