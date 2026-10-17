package hectest

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testToken = "11111111-2222-3333-4444-555555555555"

func post(t *testing.T, url, authorization, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	require.NoError(t, err)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, strings.TrimSpace(string(raw))
}

func TestReceiver_AcceptsABatchInEitherForm(t *testing.T) {
	r := NewReceiver(testToken)
	t.Cleanup(r.Close)

	concatenated := `{"time":1760650000.123,"host":"aim-1","source":"opena2a-aim","sourcetype":"opena2a:aim","index":"aim_audit","event":{"eventType":"a"}}` + "\n" +
		`{"time":1760650000123,"event":"plain text","fields":{"club":"glee","wins":["regionals","nationals"],"count":2}}`
	status, body := post(t, r.URL(), "Splunk "+testToken, concatenated)
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"text":"Success","code":0}`, body)

	status, body = post(t, r.URL(), "Splunk "+testToken, `[{"event":{"eventType":"b"}},{"event":{"eventType":"c"}}]`)
	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"text":"Success","code":0}`, body)

	accepted := r.Accepted()
	require.Len(t, accepted, 4)
	assert.Empty(t, r.Refusals())

	first := accepted[0]
	assert.Equal(t, json.Number("1760650000.123"), first.Time)
	assert.Equal(t, "aim-1", first.Host)
	assert.Equal(t, "opena2a-aim", first.Source)
	assert.Equal(t, "opena2a:aim", first.Sourcetype)
	assert.Equal(t, "aim_audit", first.Index)
	var decoded map[string]string
	require.NoError(t, first.Decode(&decoded))
	assert.Equal(t, "a", decoded["eventType"])

	assert.Equal(t, json.Number("1760650000123"), accepted[1].Time)
	assert.Equal(t, json.RawMessage(`"glee"`), accepted[1].Fields["club"])
	require.NoError(t, accepted[3].Decode(&decoded))
	assert.Equal(t, "c", decoded["eventType"])
}

func TestReceiver_RefusesWhatThePublishedFormatRefuses(t *testing.T) {
	valid := `{"event":{"eventType":"a"}}`
	cases := []struct {
		name          string
		authorization string
		body          string
		want          Answer
	}{
		{"no authorization header", "", valid, TokenRequired},
		{"another scheme", "Bearer " + testToken, valid, InvalidAuth},
		{"a different token", "Splunk 99999999-2222-3333-4444-555555555555", valid, InvalidToken},
		{"an empty body", "Splunk " + testToken, "", NoData},
		{"a body that does not parse", "Splunk " + testToken, `{"event":`, InvalidDataFormat},
		{"an array followed by more data", "Splunk " + testToken, `[` + valid + `] {}`, InvalidDataFormat},
		{"an element that is not an object", "Splunk " + testToken, `[` + valid + `, 5]`, InvalidDataFormat},
		{"an object without event", "Splunk " + testToken, valid + `{"time":1760650000}`, EventRequired},
		{"an empty event", "Splunk " + testToken, `{"event":""}`, EventBlank},
		{"a null event", "Splunk " + testToken, `{"event":null}`, EventBlank},
		{"a time that is a string", "Splunk " + testToken, `{"time":"1760650000","event":"x"}`, InvalidDataFormat},
		{"a negative time", "Splunk " + testToken, `{"time":-1,"event":"x"}`, InvalidDataFormat},
		{"a time that is an object", "Splunk " + testToken, `{"time":{"s":1},"event":"x"}`, InvalidDataFormat},
		{"a host that is a number", "Splunk " + testToken, `{"host":5,"event":"x"}`, InvalidDataFormat},
		{"a sourcetype that is an object", "Splunk " + testToken, `{"sourcetype":{},"event":"x"}`, InvalidDataFormat},
		{"an index that is an array", "Splunk " + testToken, `{"index":["a"],"event":"x"}`, InvalidDataFormat},
		{"a source that is a boolean", "Splunk " + testToken, `{"source":true,"event":"x"}`, InvalidDataFormat},
		{"nested fields", "Splunk " + testToken, `{"event":"x","fields":{"outer":{"inner":"v"}}}`, IndexedFields},
		{"nested fields inside an array", "Splunk " + testToken, `{"event":"x","fields":{"outer":[{"inner":"v"}]}}`, IndexedFields},
		{"fields that is not an object", "Splunk " + testToken, `{"event":"x","fields":"v"}`, IndexedFields},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewReceiver(testToken)
			t.Cleanup(r.Close)

			status, body := post(t, r.URL(), tc.authorization, tc.body)
			assert.Equal(t, tc.want.Status, status)
			want, err := json.Marshal(tc.want)
			require.NoError(t, err)
			assert.JSONEq(t, string(want), body)
			assert.Empty(t, r.Accepted(), "a refused request records no event")
			assert.Equal(t, []Refusal{{Status: tc.want.Status, Code: tc.want.Code, Text: tc.want.Text}}, r.Refusals())
		})
	}
}

func TestReceiver_RefusesTheTokenWhenTold(t *testing.T) {
	r := NewReceiver(testToken)
	t.Cleanup(r.Close)

	r.RefuseToken(true)
	status, body := post(t, r.URL(), "Splunk "+testToken, `{"event":"x"}`)
	assert.Equal(t, http.StatusForbidden, status)
	assert.JSONEq(t, `{"text":"Invalid token","code":4}`, body)
	assert.Empty(t, r.Accepted())

	r.RefuseToken(false)
	status, _ = post(t, r.URL(), "Splunk "+testToken, `{"event":"x"}`)
	assert.Equal(t, http.StatusOK, status)
	assert.Len(t, r.Accepted(), 1)
	assert.Equal(t, []Refusal{{Status: http.StatusForbidden, Code: 4, Text: "Invalid token"}}, r.Refusals())
}

func TestReceiver_ClosesConnectionsWhenTold(t *testing.T) {
	r := NewReceiver(testToken)
	t.Cleanup(r.Close)

	r.CloseConnections(true)
	req, err := http.NewRequest(http.MethodPost, r.URL(), strings.NewReader(`{"event":"x"}`))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Splunk "+testToken)
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
	require.Error(t, err, "the receiver answers nothing")
	assert.Empty(t, r.Accepted())
	assert.Equal(t, []Refusal{{Text: "connection closed"}}, r.Refusals())
}

func TestReceiver_ServesOnlyTheEventEndpoint(t *testing.T) {
	r := NewReceiver(testToken)
	t.Cleanup(r.Close)

	status, _ := post(t, strings.TrimSuffix(r.URL(), EventPath)+"/services/collector/raw", "Splunk "+testToken, `{"event":"x"}`)
	assert.Equal(t, http.StatusNotFound, status)

	resp, err := http.Get(r.URL())
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	assert.Empty(t, r.Accepted())
	assert.Len(t, r.Refusals(), 2)
}
