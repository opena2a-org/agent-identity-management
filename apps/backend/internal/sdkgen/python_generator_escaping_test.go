package sdkgen

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// GET /api/v1/agents/{id}/sdk?lang=python writes the agent's name, the server's
// URL and the agent's keys into the downloaded package's aim_sdk/config.py, and
// the name into example.py's docstring. Each value is written as an escaped
// Python string, so a name holding a double quote, a backslash or a line break
// stays one value: importing the SDK runs nothing the name contains.

// injectedName closes the string, puts a call on a line of its own and comments
// out the rest of the original line.
const injectedName = "demo\"\nprint(\"injected\")\n#"

var configAssignment = regexp.MustCompile(`^([A-Z_]+) = (".*")$`)

// configAssignments reads config.py after its docstring: every line is blank, a
// comment or one NAME = "..." assignment. It returns each assignment's value,
// decoded. The generator writes only escapes that Python and Go read alike, so
// strconv.Unquote decodes the literal as Python would.
func configAssignments(t *testing.T, config string) map[string]string {
	t.Helper()
	parts := strings.SplitN(config, `"""`, 3)
	if len(parts) != 3 {
		t.Fatalf("config.py does not open with a docstring:\n%s", config)
	}
	values := map[string]string{}
	for _, line := range strings.Split(parts[2], "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := configAssignment.FindStringSubmatch(line)
		if m == nil {
			t.Errorf("config.py has a line that is not an assignment of one string: %q\n%s", line, config)
			continue
		}
		value, err := strconv.Unquote(m[2])
		if err != nil {
			t.Errorf("config.py's %s is not one string literal (%v): %s", m[1], err, m[2])
			continue
		}
		values[m[1]] = value
	}
	return values
}

func TestGeneratedConfigWritesEachValueAsOneEscapedString(t *testing.T) {
	cases := map[string]string{
		"a quote, a line break and a call": injectedName,
		"a trailing backslash":             `demo\`,
		"a backslash before a quote":       `demo\"` + "\nprint(1)",
		"a carriage return":                "demo\"\rprint(1)",
		"a NUL and an escape character":    "demo\x00\x1b",
		"a line separator":                 "demo\u2028print(1)",
		"text outside ASCII":               "agent ünïcödé 代理",
	}
	for name, agentName := range cases {
		t.Run(name, func(t *testing.T) {
			config := generatePythonConfig(PythonSDKConfig{
				AgentID:    "00000000-0000-4000-8000-000000000001",
				PublicKey:  "FAKE-public-key",
				PrivateKey: "FAKE-private-key",
				AIMURL:     "https://aim.example.test\"\nprint(\"url\")",
				AgentName:  agentName,
				Version:    "1.6.0",
			})

			values := configAssignments(t, config)
			want := map[string]string{
				"AGENT_ID":    "00000000-0000-4000-8000-000000000001",
				"PUBLIC_KEY":  "FAKE-public-key",
				"PRIVATE_KEY": "FAKE-private-key",
				"AIM_URL":     "https://aim.example.test\"\nprint(\"url\")",
				"AGENT_NAME":  agentName,
				"SDK_VERSION": "1.6.0",
			}
			for key, value := range want {
				if got, ok := values[key]; !ok || got != value {
					t.Errorf("config.py's %s is %q, want %q", key, got, value)
				}
			}
			if len(values) != len(want) {
				t.Errorf("config.py assigns %d names, want %d: %v", len(values), len(want), values)
			}
		})
	}
}

// Ordinary values are written as they were: a UUID, base64 keys and a URL hold
// nothing that needs escaping.
func TestGeneratedConfigWritesOrdinaryValuesUnchanged(t *testing.T) {
	config := generatePythonConfig(PythonSDKConfig{
		AgentID:    "00000000-0000-4000-8000-000000000001",
		PublicKey:  "FAKE+public/key==",
		PrivateKey: "FAKE+private/key==",
		AIMURL:     "https://aim.example.test",
		AgentName:  "billing-bot",
		Version:    "1.6.0",
	})
	for _, line := range []string{
		`AGENT_ID = "00000000-0000-4000-8000-000000000001"`,
		`PUBLIC_KEY = "FAKE+public/key=="`,
		`PRIVATE_KEY = "FAKE+private/key=="`,
		`AIM_URL = "https://aim.example.test"`,
		`AGENT_NAME = "billing-bot"`,
		`SDK_VERSION = "1.6.0"`,
	} {
		if !strings.Contains(config, "\n"+line+"\n") {
			t.Errorf("config.py has no line %s:\n%s", line, config)
		}
	}
}

// example.py names the agent in its module docstring. A name holding three
// double quotes would close the docstring early and put what follows into the
// module's code.
func TestGeneratedExampleKeepsTheAgentNameInsideItsDocstring(t *testing.T) {
	for name, agentName := range map[string]string{
		"three double quotes":  "demo\"\"\"\nprint(\"injected\")\n\"\"\"",
		"a quote and a call":   injectedName,
		"a trailing backslash": `demo\`,
	} {
		t.Run(name, func(t *testing.T) {
			example := generatePythonExample(PythonSDKConfig{AgentName: agentName})

			if !strings.HasPrefix(example, `"""`+"\n") {
				t.Fatalf("example.py does not open with a docstring:\n%s", example)
			}
			docstring, _, closed := strings.Cut(example[len(`"""`):], `"""`)
			if !closed {
				t.Fatalf("example.py's docstring is not closed:\n%s", example)
			}
			if !strings.Contains(docstring, "This example demonstrates automatic identity verification.") {
				t.Errorf("the agent name closed example.py's docstring early:\n%s", example)
			}
			nameLine := strings.Split(strings.TrimPrefix(docstring, "\n"), "\n")[0]
			const prefix = "Example usage of AIM SDK for agent: "
			if !strings.HasPrefix(nameLine, prefix) {
				t.Fatalf("the docstring's first line is %q", nameLine)
			}
			decoded, err := strconv.Unquote(`"` + strings.TrimPrefix(nameLine, prefix) + `"`)
			if err != nil || decoded != agentName {
				t.Errorf("the docstring names the agent as %q (%v), want %q", decoded, err, agentName)
			}
		})
	}
}
