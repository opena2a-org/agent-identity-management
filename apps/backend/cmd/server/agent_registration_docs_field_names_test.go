package main

import (
	"encoding/json"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/stretchr/testify/require"
)

// agentRegistrationDocs are the pages whose examples register an agent with
// POST /api/v1/agents.
var agentRegistrationDocs = []string{
	"docs/API.md",
	"docs/api/direct-api-usage.md",
	"docs/tutorials/api-quickstart.md",
}

// jsonMemberNames returns the JSON member names of a request struct, read
// from its json tags.
func jsonMemberNames(t *testing.T, v interface{}) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	typ := reflect.TypeOf(v)
	for i := 0; i < typ.NumField(); i++ {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			names[name] = true
		}
	}
	require.NotEmpty(t, names, "%s has no json tags", typ.Name())
	return names
}

var upperRune = regexp.MustCompile(`[A-Z]`)

func camelToSnake(name string) string {
	return upperRune.ReplaceAllStringFunc(name, func(r string) string { return "_" + strings.ToLower(r) })
}

// snakeKeyHits lists, as line:text, the places a page writes key as a body
// key: quoted before a colon (JSON, a Go or Python map, a JavaScript object),
// as a Go struct tag, or unquoted at the start of a JavaScript object line.
func snakeKeyHits(page, key string) []string {
	q := regexp.QuoteMeta(key)
	re := regexp.MustCompile(`["']` + q + `["']\s*:|json:"` + q + `[",]|^\s*` + q + `\s*:`)
	var hits []string
	for i, line := range strings.Split(page, "\n") {
		if re.MatchString(line) {
			hits = append(hits, strings.TrimSpace(line)+" (line "+strconv.Itoa(i+1)+")")
		}
	}
	return hits
}

// curlPostBodies returns the JSON bodies a page's curl commands post to a URL
// ending in path, for the commands whose -d argument is one single-quoted JSON
// object. A body the shell assembles from several quoted parts is left out.
func curlPostBodies(t *testing.T, page, path string) []map[string]interface{} {
	t.Helper()
	target := regexp.MustCompile(`^-X POST "?([^"\s]+)"?`)
	body := regexp.MustCompile(`-d '(\{[^']*\})'`)
	var bodies []map[string]interface{}
	for _, cmd := range strings.Split(page, "curl ")[1:] {
		m := target.FindStringSubmatch(cmd)
		if m == nil || !strings.HasSuffix(m[1], path) {
			continue
		}
		b := body.FindStringSubmatch(cmd)
		if b == nil {
			continue
		}
		var decoded map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(b[1]), &decoded), "the body posted to %s is not JSON:\n%s", path, b[1])
		bodies = append(bodies, decoded)
	}
	return bodies
}

func sortedBodyKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestAgentRegistrationDocsUseTheCreateRequestMemberNames pins one spelling
// for the body keys of the agent registration examples: the camelCase member
// names of application.CreateAgentRequest, which docs/API.md uses. The create
// handler normalizes snake_case keys to camelCase, so a page that sent
// display_name and agent_type still registered an agent, but taught a second
// convention beside docs/API.md.
func TestAgentRegistrationDocsUseTheCreateRequestMemberNames(t *testing.T) {
	members := jsonMemberNames(t, application.CreateAgentRequest{})
	require.True(t, members["displayName"] && members["agentType"],
		"CreateAgentRequest binds displayName and agentType")

	for _, doc := range agentRegistrationDocs {
		page := aim03ReadRepoFile(t, doc)
		for member := range members {
			snake := camelToSnake(member)
			if snake == member {
				continue
			}
			require.Empty(t, snakeKeyHits(page, snake),
				"%s writes the CreateAgentRequest member %q as %q; use the member name", doc, member, snake)
		}
		for _, member := range []string{"displayName", "agentType"} {
			require.Regexp(t, `(?m)["']`+member+`["']\s*:|^\s*`+member+`\s*:`, page,
				"%s registers an agent with a %q key", doc, member)
		}
	}
}

// TestAPIQuickstartCurlBodiesBindToTheCreateRequests reads the bodies the
// curl examples in the direct API page and the API quickstart post to the
// agent and MCP server create routes, and requires every key to be a member
// the route's request struct binds, spelled as the struct spells it, and the
// members the route requires to be present.
func TestAPIQuickstartCurlBodiesBindToTheCreateRequests(t *testing.T) {
	cases := []struct {
		doc      string
		path     string
		members  map[string]bool
		required []string
	}{
		{"docs/api/direct-api-usage.md", "/api/v1/agents", jsonMemberNames(t, application.CreateAgentRequest{}),
			[]string{"name", "displayName", "agentType"}},
		{"docs/tutorials/api-quickstart.md", "/api/v1/agents", jsonMemberNames(t, application.CreateAgentRequest{}),
			[]string{"name", "displayName", "agentType"}},
		{"docs/tutorials/api-quickstart.md", "/api/v1/mcp-servers", jsonMemberNames(t, application.CreateMCPServerRequest{}),
			[]string{"name", "url"}},
	}
	for _, tc := range cases {
		bodies := curlPostBodies(t, aim03ReadRepoFile(t, tc.doc), tc.path)
		require.NotEmpty(t, bodies, "%s has a curl example that posts a JSON body to %s", tc.doc, tc.path)
		for _, body := range bodies {
			for _, key := range sortedBodyKeys(body) {
				require.True(t, tc.members[key],
					"%s posts %q to %s; the route's request struct has no member by that name", tc.doc, key, tc.path)
			}
			for _, key := range tc.required {
				require.Contains(t, body, key, "%s posts to %s without %q, which the route requires", tc.doc, tc.path, key)
			}
		}
	}
}
