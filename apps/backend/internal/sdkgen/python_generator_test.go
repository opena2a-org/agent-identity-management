package sdkgen

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// generatedMethod returns the source of one method of the generated client.
func generatedMethod(t *testing.T, name string) string {
	t.Helper()
	source := generatePythonClientFile("1.23.0")
	start := strings.Index(source, "\n    def "+name+"(")
	if start < 0 {
		t.Fatalf("the generated client has no method %s", name)
	}
	rest := source[start+1:]
	if end := strings.Index(rest, "\n    def "); end >= 0 {
		rest = rest[:end]
	}
	return rest
}

// boundMembers reads the JSON members of a Go request type from the backend
// source, so the test follows the type rather than a copy of it.
func boundMembers(t *testing.T, file, typeName string) []string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "application", file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	declared := regexp.MustCompile(`(?s)type ` + typeName + ` struct \{(.*?)\n\}`).FindSubmatch(source)
	if declared == nil {
		t.Fatalf("%s is not declared in %s", typeName, file)
	}
	var members []string
	for _, match := range regexp.MustCompile("`json:\"([^\",`]+)").FindAllSubmatch(declared[1], -1) {
		members = append(members, string(match[1]))
	}
	return members
}

// The generated client's register_mcp adds a server to the agent's list. The
// route that does that is PUT /api/v1/agents/{id}/mcp-servers, which binds
// AddMCPServersRequest. It posted snake-case members to
// POST /api/v1/sdk-api/agents/{id}/mcp-servers, the route that creates an MCP
// server and reads none of them.
func TestGeneratedRegisterMCPSendsTheAttachRequest(t *testing.T) {
	method := generatedMethod(t, "register_mcp")

	if !strings.Contains(method, `method="PUT"`) {
		t.Errorf("register_mcp does not send PUT:\n%s", method)
	}
	if !strings.Contains(method, `endpoint=f"/api/v1/agents/{self.agent_id}/mcp-servers"`) {
		t.Errorf("register_mcp does not send to /api/v1/agents/{id}/mcp-servers:\n%s", method)
	}
	if strings.Contains(method, "/sdk-api/") {
		t.Errorf("register_mcp still sends to an SDK-API route:\n%s", method)
	}

	sent := regexp.MustCompile(`"([A-Za-z_]+)":`).FindAllStringSubmatch(method, -1)
	var sentMembers []string
	for _, m := range sent {
		sentMembers = append(sentMembers, m[1])
	}
	want := boundMembers(t, "agent_service.go", "AddMCPServersRequest")
	if strings.Join(sentMembers, ",") != strings.Join(want, ",") {
		t.Errorf("register_mcp sends %v; AddMCPServersRequest binds %v", sentMembers, want)
	}
}

// AIM adds to an agent's list only for a signed-in user with the member role
// or higher, and the generated client holds the agent's signing keys only, so
// its refusal says so instead of reading as a credential failure.
func TestGeneratedRegisterMCPExplainsTheRefusal(t *testing.T) {
	method := generatedMethod(t, "register_mcp")

	handler := strings.Index(method, "except AuthenticationError")
	if handler < 0 {
		t.Fatalf("register_mcp does not handle a refusal:\n%s", method)
	}
	refusal := method[handler:]
	for _, phrase := range []string{
		"user with the member role or higher",
		"signing keys only",
		"agent's page in the AIM dashboard",
	} {
		if !strings.Contains(refusal, phrase) {
			t.Errorf("register_mcp's refusal does not say %q:\n%s", phrase, method)
		}
	}
}
