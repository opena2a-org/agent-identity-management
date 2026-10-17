package awssmtest

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func rawRequest(t *testing.T, s *Server, header map[string]string, body string) (int, map[string]string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, s.URL+"/", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := s.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("answer is not JSON: %v", err)
	}
	return resp.StatusCode, out
}

func validHeader(operation string) map[string]string {
	return map[string]string{
		"X-Amz-Target":  "secretsmanager." + operation,
		"Content-Type":  "application/x-amz-json-1.1",
		"Authorization": "AWS4-HMAC-SHA256 Credential=stub/20261016/us-east-1/secretsmanager/aws4_request, SignedHeaders=host, Signature=00",
	}
}

func TestServer_RefusesRequestsOutsideTheJSON11Protocol(t *testing.T) {
	s := NewServer(t)
	cases := []struct {
		name   string
		mutate func(map[string]string)
	}{
		{"no X-Amz-Target", func(h map[string]string) { delete(h, "X-Amz-Target") }},
		{"JSON 1.0 content type", func(h map[string]string) { h["Content-Type"] = "application/x-amz-json-1.0" }},
		{"plain JSON content type", func(h map[string]string) { h["Content-Type"] = "application/json" }},
		{"no Authorization", func(h map[string]string) { delete(h, "Authorization") }},
		{"Authorization of another scheme", func(h map[string]string) { h["Authorization"] = "Bearer stub" }},
		{"target of another service", func(h map[string]string) { h["X-Amz-Target"] = "kms.GetSecretValue" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := validHeader("GetSecretValue")
			tc.mutate(h)
			status, out := rawRequest(t, s, h, `{"SecretId":"aim-secrets/x"}`)
			if status != http.StatusBadRequest || out["__type"] != RefusedType {
				t.Fatalf("answer = %d %v, want 400 %s", status, out, RefusedType)
			}
		})
	}
	if n := len(s.Requests()); n != 0 {
		t.Fatalf("%d refused requests were recorded as accepted", n)
	}
}

func TestServer_UnknownSecretIDAnswersResourceNotFound(t *testing.T) {
	s := NewServer(t)
	for _, op := range []string{"GetSecretValue", "PutSecretValue", "DeleteSecret"} {
		t.Run(op, func(t *testing.T) {
			status, out := rawRequest(t, s, validHeader(op), `{"SecretId":"aim-secrets/absent","SecretString":"x"}`)
			if status != http.StatusBadRequest || out["__type"] != "ResourceNotFoundException" {
				t.Fatalf("answer = %d %v, want 400 ResourceNotFoundException", status, out)
			}
		})
	}
}

func TestServer_SecretIDLengthIsChecked(t *testing.T) {
	s := NewServer(t)
	for _, id := range []string{"", strings.Repeat("a", maxSecretIDLength+1)} {
		status, out := rawRequest(t, s, validHeader("GetSecretValue"), `{"SecretId":"`+id+`"}`)
		if status != http.StatusBadRequest || out["__type"] != "InvalidParameterException" {
			t.Fatalf("SecretId of %d characters: answer = %d %v, want 400 InvalidParameterException", len(id), status, out)
		}
	}
}
