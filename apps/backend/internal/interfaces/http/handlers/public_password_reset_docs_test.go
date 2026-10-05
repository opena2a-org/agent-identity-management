package handlers

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The dashboard's developer page renders apps/web/lib/api-documentation.ts. A
// field name there that the handler does not bind is silently dropped by
// c.Bind().Body, so a client written from the docs fails with a "required"
// error that names a field the docs never mentioned. These tests hold each
// public password endpoint's documented request (schema properties and example
// body) to the json tags of the struct its handler binds.

var docPropertyKeyRe = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)\s*:\s*\{`)

type documentedRequest struct {
	properties  []string
	required    []string
	exampleKeys []string
}

// documentedRequestFor extracts the request schema and example of the endpoint
// whose `path:` entry is apiPath.
func documentedRequestFor(t *testing.T, doc, apiPath string) documentedRequest {
	t.Helper()

	pathEntry := fmt.Sprintf("path: %q,", apiPath)
	require.Equal(t, 1, strings.Count(doc, pathEntry),
		"api-documentation.ts must document %s exactly once", apiPath)

	entry := doc[strings.Index(doc, pathEntry):]
	if next := strings.Index(entry[len(pathEntry):], "method: \""); next >= 0 {
		entry = entry[:len(pathEntry)+next]
	}

	propsAt := strings.Index(entry, "properties: {")
	require.GreaterOrEqual(t, propsAt, 0, "%s has no requestSchema properties", apiPath)

	var out documentedRequest
	depth := 0
	current := ""
	for _, line := range strings.Split(entry[propsAt+len("properties: {"):], "\n") {
		if depth == 0 {
			if m := docPropertyKeyRe.FindStringSubmatch(line); m != nil {
				current = m[1]
				out.properties = append(out.properties, current)
			}
		}
		if depth == 1 && strings.Contains(line, "required: true") {
			out.required = append(out.required, current)
		}
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if depth < 0 {
			break
		}
	}

	const exampleOpen = "example: `"
	exAt := strings.Index(entry, exampleOpen)
	require.GreaterOrEqual(t, exAt, 0, "%s has no example body", apiPath)
	example := entry[exAt+len(exampleOpen):]
	example = example[:strings.Index(example, "`")]

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(example), &body),
		"the %s example must be valid JSON", apiPath)
	for k := range body {
		out.exampleKeys = append(out.exampleKeys, k)
	}

	sort.Strings(out.properties)
	sort.Strings(out.required)
	sort.Strings(out.exampleKeys)
	return out
}

// boundFields returns the json names the handler binds from v's struct type,
// and the subset its validate tag marks required.
func boundFields(v any) (fields, required []string) {
	typ := reflect.TypeOf(v)
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		fields = append(fields, name)
		for _, rule := range strings.Split(f.Tag.Get("validate"), ",") {
			if rule == "required" {
				required = append(required, name)
			}
		}
	}
	sort.Strings(fields)
	sort.Strings(required)
	return fields, required
}

func TestPublicPasswordEndpointDocsMatchTheBoundRequest(t *testing.T) {
	doc := readBackendFile(t, "../web/lib/api-documentation.ts")

	cases := []struct {
		path    string
		request any
	}{
		{"/api/v1/public/forgot-password", ForgotPasswordRequest{}},
		{"/api/v1/public/reset-password", ResetPasswordRequest{}},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			documented := documentedRequestFor(t, doc, tc.path)
			fields, required := boundFields(tc.request)

			assert.Equal(t, fields, documented.properties,
				"the documented request fields must be the json names the handler binds")
			assert.Equal(t, required, documented.required,
				"every field the handler requires must be documented `required: true`, and no other")
			assert.Equal(t, fields, documented.exampleKeys,
				"the documented example body must use the json names the handler binds")
		})
	}
}
