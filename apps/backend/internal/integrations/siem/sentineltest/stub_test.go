package sentineltest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTenant = "00000000-0000-0000-0000-00000000aaaa"
	testClient = "00000000-0000-0000-0000-00000000bbbb"
	testDCR    = "dcr-00000000000000000000000000000000"
	testStream = "Custom-AIMSecurityEvents"
)

// testSecret is assembled at run time; it is a placeholder, not a credential.
var testSecret = strings.Join([]string{"stub", "client", "secret"}, "-")

func requestToken(t *testing.T, s *TokenServer, path string, form url.Values) (int, map[string]interface{}) {
	t.Helper()
	resp, err := http.Post(s.AuthorityHost()+path, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	defer resp.Body.Close()
	var answer map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&answer))
	return resp.StatusCode, answer
}

func goodForm() url.Values {
	return url.Values{
		"client_id":     {testClient},
		"client_secret": {testSecret},
		"scope":         {"https://monitor.azure.com/.default"},
		"grant_type":    {"client_credentials"},
	}
}

func TestTokenServer_IssuesABearerToTheApplication(t *testing.T) {
	s := NewTokenServer(testTenant, testClient, testSecret)
	t.Cleanup(s.Close)

	status, answer := requestToken(t, s, "/"+testTenant+"/oauth2/v2.0/token", goodForm())
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "Bearer", answer["token_type"])
	assert.Equal(t, float64(ExpiresIn), answer["expires_in"])
	token, _ := answer["access_token"].(string)
	require.NotEmpty(t, token)
	assert.True(t, s.Valid(token))

	s.RevokeAll()
	assert.False(t, s.Valid(token))
	assert.Equal(t, 1, s.Requests())
}

func TestTokenServer_Refusals(t *testing.T) {
	s := NewTokenServer(testTenant, testClient, testSecret)
	t.Cleanup(s.Close)

	with := func(field, value string) url.Values {
		form := goodForm()
		if value == "" {
			form.Del(field)
		} else {
			form.Set(field, value)
		}
		return form
	}
	cases := []struct {
		name   string
		path   string
		form   url.Values
		status int
		code   string
	}{
		{"wrong secret", "/" + testTenant + "/oauth2/v2.0/token", with("client_secret", "not-the-secret"), 401, "invalid_client"},
		{"no secret", "/" + testTenant + "/oauth2/v2.0/token", with("client_secret", ""), 401, "invalid_client"},
		{"wrong client", "/" + testTenant + "/oauth2/v2.0/token", with("client_id", "someone-else"), 401, "invalid_client"},
		{"wrong grant", "/" + testTenant + "/oauth2/v2.0/token", with("grant_type", "password"), 400, "unsupported_grant_type"},
		{"scope without .default", "/" + testTenant + "/oauth2/v2.0/token", with("scope", "https://monitor.azure.com/"), 400, "invalid_scope"},
		{"scope of another resource", "/" + testTenant + "/oauth2/v2.0/token", with("scope", "https://graph.microsoft.com/.default"), 400, "invalid_scope"},
		{"wrong tenant", "/another-tenant/oauth2/v2.0/token", goodForm(), 404, "not_found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, answer := requestToken(t, s, tc.path, tc.form)
			assert.Equal(t, tc.status, status)
			assert.Equal(t, tc.code, answer["error"])
			assert.NotEmpty(t, answer["error_description"])
			assert.NotContains(t, answer, "access_token")
		})
	}

	resp, err := http.Post(s.AuthorityHost()+"/"+testTenant+"/oauth2/v2.0/token", "application/json", strings.NewReader(`{}`))
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "a JSON body is not a form")
}

func upload(t *testing.T, s *IngestionServer, path, query, authorization, contentType, body string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, s.Endpoint()+path+query, strings.NewReader(body))
	require.NoError(t, err)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func TestIngestionServer(t *testing.T) {
	tokens := NewTokenServer(testTenant, testClient, testSecret)
	t.Cleanup(tokens.Close)
	s := NewIngestionServer(testDCR, testStream, tokens)
	t.Cleanup(s.Close)

	_, answer := requestToken(t, tokens, "/"+testTenant+"/oauth2/v2.0/token", goodForm())
	bearer := "Bearer " + answer["access_token"].(string)
	path := "/dataCollectionRules/" + testDCR + "/streams/" + testStream
	query := "?api-version=" + APIVersion
	good := `[{"TimeGenerated":"2026-10-16T18:00:00Z","EventType":"aim.auth.signin_failed","Details":{"id":"nested keys are not columns"}},{"Severity":"low"}]`

	cases := []struct {
		name                                 string
		path, query, auth, contentType, body string
		status                               int
	}{
		{"accepted", path, query, bearer, "application/json", good, 204},
		{"charset parameter", path, query, bearer, "application/json; charset=utf-8", good, 204},
		{"no bearer", path, query, "", "application/json", good, 401},
		{"bearer the token server did not issue", path, query, "Bearer not-issued", "application/json", good, 401},
		{"wrong scheme", path, query, strings.Replace(bearer, "Bearer", "Basic", 1), "application/json", good, 401},
		{"no api-version", path, "", bearer, "application/json", good, 400},
		{"another api-version", path, "?api-version=2021-01-01", bearer, "application/json", good, 400},
		{"not json", path, query, bearer, "text/plain", good, 400},
		{"an object, not an array", path, query, bearer, "application/json", `{"EventType":"x"}`, 400},
		{"an empty array", path, query, bearer, "application/json", `[]`, 400},
		{"a record that is not an object", path, query, bearer, "application/json", `["x"]`, 400},
		{"trailing data", path, query, bearer, "application/json", `[{"A":1}] [{"B":2}]`, 400},
		{"a column starting with a digit", path, query, bearer, "application/json", `[{"1Time":"x"}]`, 400},
		{"a column with a dash", path, query, bearer, "application/json", `[{"Event-Type":"x"}]`, 400},
		{"a column of 46 characters", path, query, bearer, "application/json", `[{"` + "A" + strings.Repeat("b", 45) + `":"x"}]`, 400},
		{"reserved id", path, query, bearer, "application/json", `[{"id":"x"}]`, 400},
		{"reserved TenantId", path, query, bearer, "application/json", `[{"TenantId":"x"}]`, 400},
		{"reserved Type", path, query, bearer, "application/json", `[{"Type":"x"}]`, 400},
		{"another rule", "/dataCollectionRules/dcr-other/streams/" + testStream, query, bearer, "application/json", good, 404},
		{"another stream", "/dataCollectionRules/" + testDCR + "/streams/Custom-Other", query, bearer, "application/json", good, 404},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.status, upload(t, s, tc.path, tc.query, tc.auth, tc.contentType, tc.body))
		})
	}
	assert.True(t, AllowedColumn("A"+strings.Repeat("b", 44)), "45 characters is the limit")

	accepted := s.Accepted()
	require.Len(t, accepted, 4, "two accepted uploads of two records each")
	eventType, ok := accepted[0].String("EventType")
	assert.True(t, ok)
	assert.Equal(t, "aim.auth.signin_failed", eventType)
	assert.Equal(t, len(cases), s.Requests())
	assert.Len(t, s.Refusals(), len(cases)-2)

	tokens.RevokeAll()
	assert.Equal(t, http.StatusUnauthorized, upload(t, s, path, query, bearer, "application/json", good), "a revoked token")
}
