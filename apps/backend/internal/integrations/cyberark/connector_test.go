package cyberark

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/cyberark/ccptest"
)

const (
	testAppID  = "AIMApp"
	testSafe   = "AIM-Database"
	testObject = "db/orders-prod:5432"
)

// runtimeContent builds the stub account's credential value at run time.
func runtimeContent(t *testing.T) string {
	t.Helper()
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return strings.Join([]string{"ccp", "stub", "content", hex.EncodeToString(b)}, "-")
}

func connectorFor(t *testing.T, stub *ccptest.Server, mutate func(*Config)) *Connector {
	t.Helper()
	cfg := Config{
		CCPURL:     stub.URL,
		AppID:      testAppID,
		CertPath:   stub.ClientCertPath,
		KeyPath:    stub.ClientKeyPath,
		CACertPath: stub.CACertPath,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	c, err := NewConnector(cfg)
	if err != nil {
		t.Fatalf("NewConnector: %v", err)
	}
	return c
}

func TestCCPConnectorRoundTrip(t *testing.T) {
	stub := ccptest.New(t)
	content := runtimeContent(t)
	stub.AddAccount(ccptest.Account{
		AppID: testAppID, Safe: testSafe, Object: testObject,
		Content: content, UserName: "svc_aim", Address: "orders-db.internal.example", Database: "orders",
	})
	changing := runtimeContent(t)
	stub.AddAccount(ccptest.Account{
		AppID: testAppID, Safe: testSafe, Object: "db-rotating",
		Content: changing, UserName: "svc_rotating", PasswordChanging: true,
	})
	// Accounts whose AppID, Safe or Object name holds a space.
	spaced := runtimeContent(t)
	for _, a := range []ccptest.Account{
		{AppID: "AIM App", Safe: testSafe, Object: testObject},
		{AppID: testAppID, Safe: "AIM Database", Object: testObject},
		{AppID: testAppID, Safe: testSafe, Object: "db orders"},
	} {
		a.Content, a.UserName = spaced, "svc_spaced"
		stub.AddAccount(a)
	}
	ctx := context.Background()

	// Every error the connector returns in this test; none may hold a
	// credential value the stub served.
	var errs []error
	keep := func(err error) error {
		if err != nil {
			errs = append(errs, err)
		}
		return err
	}
	t.Cleanup(func() {
		for _, err := range errs {
			for _, secret := range []string{content, changing, spaced} {
				if strings.Contains(err.Error(), secret) {
					t.Errorf("error %q holds a credential value the stub served", err)
				}
			}
		}
	})

	// assertNothingSent fails when the stub recorded a request after before.
	assertNothingSent := func(t *testing.T, before int) {
		t.Helper()
		if got := stub.Requests(); len(got) != before {
			t.Errorf("the stub recorded %d request(s), want none; last parameters %v", len(got)-before, got[len(got)-1].Params)
		}
	}

	t.Run("retrieves an account over mutual TLS", func(t *testing.T) {
		c := connectorFor(t, stub, nil)
		before := len(stub.Requests())
		resp, err := c.RetrieveCredential(ctx, CCPRequest{Safe: testSafe, Object: testObject, Reason: "grant-approved"})
		if keep(err) != nil {
			t.Fatalf("RetrieveCredential: %v", err)
		}
		want := CCPResponse{Content: content, UserName: "svc_aim", Address: "orders-db.internal.example", Database: "orders"}
		if *resp != want {
			t.Errorf("account fields differ from what the stub served (Content compared without printing)")
		}
		reqs := stub.Requests()
		if len(reqs) != before+1 {
			t.Fatalf("the stub recorded %d request(s), want 1", len(reqs)-before)
		}
		got := reqs[len(reqs)-1]
		if got.Method != http.MethodGet || !strings.HasPrefix(got.RequestURI, ccptest.AccountsPath+"?") {
			t.Errorf("request = %s %s, want GET %s?...", got.Method, got.RequestURI, ccptest.AccountsPath)
		}
		if got.ClientCommonName != ccptest.ClientCommonName {
			t.Errorf("client certificate CN = %q, want %q", got.ClientCommonName, ccptest.ClientCommonName)
		}
		if got.Params.Get("AppID") != testAppID || got.Params.Get("Safe") != testSafe || got.Params.Get("Reason") != "grant-approved" {
			t.Errorf("parameters = %v", got.Params)
		}
	})

	t.Run("an Object holding / and : round-trips exactly", func(t *testing.T) {
		c := connectorFor(t, stub, nil)
		if _, err := c.RetrieveCredential(ctx, CCPRequest{Safe: testSafe, Object: testObject}); keep(err) != nil {
			t.Fatalf("RetrieveCredential: %v", err)
		}
		reqs := stub.Requests()
		got := reqs[len(reqs)-1]
		if objs := got.Params["Object"]; len(objs) != 1 || objs[0] != testObject {
			t.Errorf("Object received = %q, want exactly [%q]", objs, testObject)
		}
		if strings.ContainsAny(got.RequestURI, " +") {
			t.Errorf("request URI %q holds a raw space or '+'", got.RequestURI)
		}
	})

	t.Run("each parameter goes under CyberArk's name", func(t *testing.T) {
		c := connectorFor(t, stub, nil)
		req := CCPRequest{
			AppID: testAppID, Safe: testSafe, Folder: "Root", Object: testObject,
			UserName: "svc_aim", Address: "orders-db.internal.example", Database: "orders",
			PolicyID: "UnixSSH", Reason: "grant-approved", Query: "Safe=AIM-Database",
			QueryFormat: "Exact", ConnectionTimeout: 45, FailRequestOnPasswordChange: true,
		}
		if _, err := c.RetrieveCredential(ctx, req); keep(err) != nil {
			t.Fatalf("RetrieveCredential: %v", err)
		}
		reqs := stub.Requests()
		got := reqs[len(reqs)-1]
		want := map[string]string{
			"AppID": testAppID, "Safe": testSafe, "Folder": "Root", "Object": testObject,
			"UserName": "svc_aim", "Address": "orders-db.internal.example", "Database": "orders",
			"PolicyID": "UnixSSH", "Reason": "grant-approved", "Query": "Safe=AIM-Database",
			"QueryFormat": "Exact", "Connection Timeout": "45", "FailRequestOnPasswordChange": "true",
		}
		for name, value := range want {
			if v := got.Params[name]; len(v) != 1 || v[0] != value {
				t.Errorf("parameter %q = %q, want [%q]", name, v, value)
			}
		}
		for name := range got.Params {
			if !ccptest.IsDocumentedParam(name) {
				t.Errorf("parameter %q is not one CyberArk documents", name)
			}
		}
		if !strings.Contains(got.RequestURI, "&Connection%20Timeout=45&") {
			t.Errorf("request URI %q does not send Connection%%20Timeout=45", got.RequestURI)
		}
		if strings.ContainsAny(got.RequestURI, " +") {
			t.Errorf("request URI %q holds a raw space or '+'", got.RequestURI)
		}
	})

	t.Run("a value holding a space is sent with the space written as %20", func(t *testing.T) {
		c := connectorFor(t, stub, nil)
		cases := []struct {
			param, value string
			req          CCPRequest
			content      string
		}{
			{"AppID", "AIM App", CCPRequest{AppID: "AIM App", Safe: testSafe, Object: testObject}, spaced},
			{"Safe", "AIM Database", CCPRequest{Safe: "AIM Database", Object: testObject}, spaced},
			{"Folder", "Root Folder", CCPRequest{Safe: testSafe, Folder: "Root Folder", Object: testObject}, content},
			{"Object", "db orders", CCPRequest{Safe: testSafe, Object: "db orders"}, spaced},
			{"UserName", "svc aim", CCPRequest{Safe: testSafe, Object: testObject, UserName: "svc aim"}, content},
			{"Address", "orders db", CCPRequest{Safe: testSafe, Object: testObject, Address: "orders db"}, content},
			{"Database", "orders db", CCPRequest{Safe: testSafe, Object: testObject, Database: "orders db"}, content},
			{"PolicyID", "Unix SSH", CCPRequest{Safe: testSafe, Object: testObject, PolicyID: "Unix SSH"}, content},
			{"Reason", "grant approved", CCPRequest{Safe: testSafe, Object: testObject, Reason: "grant approved"}, content},
		}
		for _, tc := range cases {
			before := len(stub.Requests())
			resp, err := c.RetrieveCredential(ctx, tc.req)
			if keep(err) != nil {
				t.Errorf("%s %q: RetrieveCredential: %v", tc.param, tc.value, err)
				continue
			}
			if resp.Content != tc.content {
				t.Errorf("%s %q: the account returned is not the one the stub serves for the request (Content compared without printing)", tc.param, tc.value)
			}
			reqs := stub.Requests()
			if len(reqs) != before+1 {
				t.Errorf("%s %q: the stub recorded %d request(s), want 1", tc.param, tc.value, len(reqs)-before)
				continue
			}
			got := reqs[len(reqs)-1]
			if v := got.Params[tc.param]; len(v) != 1 || v[0] != tc.value {
				t.Errorf("%s received = %q, want exactly [%q]", tc.param, v, tc.value)
			}
			if want := tc.param + "=" + strings.ReplaceAll(tc.value, " ", "%20"); !strings.Contains(got.RequestURI, want) {
				t.Errorf("request URI %q does not send %s", got.RequestURI, want)
			}
			if strings.ContainsAny(got.RequestURI, " +") {
				t.Errorf("request URI %q holds a raw space or '+'", got.RequestURI)
			}
		}
	})

	t.Run("a Query that names several properties is sent query-escaped", func(t *testing.T) {
		c := connectorFor(t, stub, nil)
		for _, tc := range []struct{ query, sent string }{
			{"Safe=AIM-Database;Object=db", "&Query=Safe%3DAIM-Database%3BObject%3Ddb"},
			{"Safe=AIM Database;Object=db orders", "&Query=Safe%3DAIM%20Database%3BObject%3Ddb%20orders"},
		} {
			before := len(stub.Requests())
			_, err := c.RetrieveCredential(ctx, CCPRequest{Query: tc.query})
			keep(err)
			// The stub checks the format of a request before it looks an
			// account up, and answers 400 to a format CCP does not accept.
			// It looks an account up by the Safe and Object parameters and
			// does not evaluate a Query, so a request that sends neither is
			// answered 404.
			if !errors.Is(err, ErrCCPNotFound) {
				t.Errorf("Query %q: err = %v, want the stub's 404 for a request with no Safe parameter", tc.query, err)
			}
			reqs := stub.Requests()
			if len(reqs) != before+1 {
				t.Errorf("Query %q: the stub recorded %d request(s), want 1", tc.query, len(reqs)-before)
				continue
			}
			got := reqs[len(reqs)-1]
			if v := got.Params["Query"]; len(v) != 1 || v[0] != tc.query {
				t.Errorf("Query received = %q, want exactly [%q]", v, tc.query)
			}
			if !strings.Contains(got.RequestURI, tc.sent) {
				t.Errorf("request URI %q does not send %s", got.RequestURI, tc.sent)
			}
			if strings.ContainsAny(got.RequestURI, " +;") {
				t.Errorf("request URI %q holds a raw space, '+' or ';'", got.RequestURI)
			}
		}
	})

	t.Run("unset parameters are not sent", func(t *testing.T) {
		c := connectorFor(t, stub, nil)
		if _, err := c.RetrieveCredential(ctx, CCPRequest{Safe: testSafe, Object: testObject}); keep(err) != nil {
			t.Fatalf("RetrieveCredential: %v", err)
		}
		reqs := stub.Requests()
		got := reqs[len(reqs)-1]
		for _, name := range []string{"Folder", "Reason", "Connection Timeout", "QueryFormat", "FailRequestOnPasswordChange"} {
			if _, ok := got.Params[name]; ok {
				t.Errorf("parameter %q was sent although it is not set", name)
			}
		}
	})

	t.Run("without a client certificate the handshake fails and nothing reaches CCP", func(t *testing.T) {
		c := connectorFor(t, stub, func(cfg *Config) { cfg.CertPath, cfg.KeyPath = "", "" })
		before := len(stub.Requests())
		_, err := c.RetrieveCredential(ctx, CCPRequest{Safe: testSafe, Object: testObject})
		if keep(err) == nil {
			t.Fatal("RetrieveCredential succeeded without a client certificate")
		}
		var statusErr *CCPStatusError
		if errors.As(err, &statusErr) {
			t.Errorf("got a CCP answer (%v), want a refused handshake", err)
		}
		assertNothingSent(t, before)
	})

	t.Run("a client certificate from another CA is refused and nothing reaches CCP", func(t *testing.T) {
		certPath, keyPath := ccptest.UntrustedClientKeyPair(t)
		c := connectorFor(t, stub, func(cfg *Config) { cfg.CertPath, cfg.KeyPath = certPath, keyPath })
		before := len(stub.Requests())
		if _, err := c.RetrieveCredential(ctx, CCPRequest{Safe: testSafe, Object: testObject}); keep(err) == nil {
			t.Fatal("RetrieveCredential succeeded with an untrusted client certificate")
		}
		assertNothingSent(t, before)
	})

	t.Run("a CCP server certificate from another CA fails verification", func(t *testing.T) {
		c := connectorFor(t, stub, func(cfg *Config) { cfg.CACertPath = ccptest.UnrelatedCABundle(t) })
		before := len(stub.Requests())
		_, err := c.RetrieveCredential(ctx, CCPRequest{Safe: testSafe, Object: testObject})
		if keep(err) == nil {
			t.Fatal("RetrieveCredential trusted a server certificate from another CA")
		}
		var verifyErr *tls.CertificateVerificationError
		if !errors.As(err, &verifyErr) {
			t.Errorf("error = %v, want a certificate verification failure", err)
		}
		assertNothingSent(t, before)
	})

	t.Run("an empty CACertPath verifies against the system roots, which do not hold the stub CA", func(t *testing.T) {
		c := connectorFor(t, stub, func(cfg *Config) { cfg.CACertPath = "" })
		before := len(stub.Requests())
		_, err := c.RetrieveCredential(ctx, CCPRequest{Safe: testSafe, Object: testObject})
		if keep(err) == nil {
			t.Fatal("RetrieveCredential trusted the stub without its CA bundle")
		}
		var verifyErr *tls.CertificateVerificationError
		if !errors.As(err, &verifyErr) {
			t.Errorf("error = %v, want a certificate verification failure", err)
		}
		assertNothingSent(t, before)
	})

	t.Run("values CCP does not accept are refused before sending", func(t *testing.T) {
		c := connectorFor(t, stub, nil)
		type refusal struct {
			param string
			req   CCPRequest
			value string
		}
		cases := []refusal{
			{"Object", CCPRequest{Safe: testSafe, Object: "db-prod&Safe=Other"}, "db-prod&Safe=Other"},
			{"Object", CCPRequest{Safe: testSafe, Object: "db+prod"}, "db+prod"},
			{"Object", CCPRequest{Safe: testSafe, Object: "db%26prod"}, "db%26prod"},
			{"Object", CCPRequest{Safe: testSafe, Object: "db;prod"}, "db;prod"},
			{"Reason", CCPRequest{Safe: testSafe, Object: testObject, Reason: "grant&AppID=Other"}, "grant&AppID=Other"},
			{"AppID", CCPRequest{AppID: "AIM+App", Safe: testSafe, Object: testObject}, "AIM+App"},
			// In a Query, a ';' that does not separate two name=value
			// properties lies inside a property name or value.
			{"Query", CCPRequest{Query: "Safe=AIM-Database;"}, "Safe=AIM-Database;"},
			{"Query", CCPRequest{Query: ";Safe=AIM-Database"}, ";Safe=AIM-Database"},
			{"Query", CCPRequest{Query: "Safe=AIM-Database;;Object=db"}, "Safe=AIM-Database;;Object=db"},
			{"Query", CCPRequest{Query: "Safe=AIM-Database;db"}, "Safe=AIM-Database;db"},
			{"Query", CCPRequest{Query: "Safe=AIM-Database;=db"}, "Safe=AIM-Database;=db"},
			{"Query", CCPRequest{Query: "Safe=AIM-Database&Object=db"}, "Safe=AIM-Database&Object=db"},
			{"Query", CCPRequest{Query: "Safe=AIM+Database"}, "Safe=AIM+Database"},
			{"Query", CCPRequest{Query: "Safe=AIM-Database%3BObject=db"}, "Safe=AIM-Database%3BObject=db"},
		}
		// '+', '&', '%' and ';' in each parameter that carries text. In the
		// Query the value is that of its Safe property, so a ';' lies inside
		// a property value there too.
		for _, f := range []struct {
			param string
			set   func(*CCPRequest, string)
		}{
			{"AppID", func(r *CCPRequest, v string) { r.AppID = v }},
			{"Safe", func(r *CCPRequest, v string) { r.Safe = v }},
			{"Folder", func(r *CCPRequest, v string) { r.Folder = v }},
			{"Object", func(r *CCPRequest, v string) { r.Object = v }},
			{"UserName", func(r *CCPRequest, v string) { r.UserName = v }},
			{"Address", func(r *CCPRequest, v string) { r.Address = v }},
			{"Database", func(r *CCPRequest, v string) { r.Database = v }},
			{"PolicyID", func(r *CCPRequest, v string) { r.PolicyID = v }},
			{"Reason", func(r *CCPRequest, v string) { r.Reason = v }},
			{"Query", func(r *CCPRequest, v string) { r.Query = "Safe=" + v }},
		} {
			for _, ch := range []string{"+", "&", "%", ";"} {
				value := "AIM" + ch + "Database"
				req := CCPRequest{Safe: testSafe, Object: testObject}
				f.set(&req, value)
				cases = append(cases, refusal{f.param, req, value})
			}
		}
		for _, tc := range cases {
			before := len(stub.Requests())
			_, err := c.RetrieveCredential(ctx, tc.req)
			if keep(err) == nil {
				t.Errorf("%s %q: RetrieveCredential accepted it", tc.param, tc.value)
				continue
			}
			if !strings.Contains(err.Error(), "parameter "+tc.param+" ") {
				t.Errorf("%s %q: error %q does not name the parameter", tc.param, tc.value, err)
			}
			if strings.Contains(err.Error(), tc.value) {
				t.Errorf("%s: error %q holds the value", tc.param, err)
			}
			assertNothingSent(t, before)
		}
	})

	t.Run("a missing AppID or AppID alone is refused before sending", func(t *testing.T) {
		noDefault := connectorFor(t, stub, func(cfg *Config) { cfg.AppID = "" })
		before := len(stub.Requests())
		_, err := noDefault.RetrieveCredential(ctx, CCPRequest{Safe: testSafe, Object: testObject})
		if keep(err) == nil || !strings.Contains(err.Error(), "AppID") {
			t.Errorf("missing AppID: err = %v, want a refusal naming AppID", err)
		}
		_, err = noDefault.RetrieveCredential(ctx, CCPRequest{AppID: testAppID})
		if keep(err) == nil || !strings.Contains(err.Error(), "only AppID") {
			t.Errorf("AppID alone: err = %v, want a refusal", err)
		}
		withDefault := connectorFor(t, stub, nil)
		_, err = withDefault.RetrieveCredential(ctx, CCPRequest{})
		if keep(err) == nil || !strings.Contains(err.Error(), "only AppID") {
			t.Errorf("Config.AppID alone: err = %v, want a refusal", err)
		}
		assertNothingSent(t, before)
	})

	t.Run("a QueryFormat other than Exact or Regexp is refused before sending", func(t *testing.T) {
		c := connectorFor(t, stub, nil)
		before := len(stub.Requests())
		for _, qf := range []string{"exact", "Regex", "Glob"} {
			_, err := c.RetrieveCredential(ctx, CCPRequest{Query: "Safe=AIM-Database", QueryFormat: qf})
			if keep(err) == nil || !strings.Contains(err.Error(), "QueryFormat") {
				t.Errorf("QueryFormat %q: err = %v, want a refusal naming QueryFormat", qf, err)
			}
		}
		_, err := c.RetrieveCredential(ctx, CCPRequest{Safe: testSafe, Object: testObject, ConnectionTimeout: -1})
		if keep(err) == nil || !strings.Contains(err.Error(), "Connection Timeout") {
			t.Errorf("negative Connection Timeout: err = %v, want a refusal naming it", err)
		}
		assertNothingSent(t, before)
	})

	t.Run("403 for an unknown AppID maps to ErrCCPForbidden", func(t *testing.T) {
		c := connectorFor(t, stub, nil)
		_, err := c.RetrieveCredential(ctx, CCPRequest{AppID: "OtherApp", Safe: testSafe, Object: testObject})
		keep(err)
		var statusErr *CCPStatusError
		if !errors.Is(err, ErrCCPForbidden) || !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusForbidden {
			t.Fatalf("err = %v, want ErrCCPForbidden with status 403", err)
		}
		if !strings.Contains(statusErr.Body, "APPAP306E") {
			t.Errorf("error body %q does not carry APPAP306E", statusErr.Body)
		}
	})

	t.Run("404 for an unknown Safe maps to ErrCCPNotFound", func(t *testing.T) {
		c := connectorFor(t, stub, nil)
		_, err := c.RetrieveCredential(ctx, CCPRequest{Safe: "AIM-Unknown", Object: testObject})
		keep(err)
		var statusErr *CCPStatusError
		if !errors.Is(err, ErrCCPNotFound) || !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusNotFound {
			t.Fatalf("err = %v, want ErrCCPNotFound with status 404", err)
		}
		if !strings.Contains(statusErr.Body, "APPAP004E") {
			t.Errorf("error body %q does not carry APPAP004E", statusErr.Body)
		}
	})

	t.Run("500 while the password changes maps to ErrCCPServerError when FailRequestOnPasswordChange is sent", func(t *testing.T) {
		c := connectorFor(t, stub, nil)
		_, err := c.RetrieveCredential(ctx, CCPRequest{Safe: testSafe, Object: "db-rotating", FailRequestOnPasswordChange: true})
		keep(err)
		var statusErr *CCPStatusError
		if !errors.Is(err, ErrCCPServerError) || !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusInternalServerError {
			t.Fatalf("err = %v, want ErrCCPServerError with status 500", err)
		}
		if !strings.Contains(statusErr.Body, "APPAP282E") {
			t.Errorf("error body %q does not carry APPAP282E", statusErr.Body)
		}
		reqs := stub.Requests()
		if v := reqs[len(reqs)-1].Params["FailRequestOnPasswordChange"]; len(v) != 1 || v[0] != "true" {
			t.Errorf("FailRequestOnPasswordChange received = %q, want [\"true\"]", v)
		}
	})

	t.Run("PasswordChangeInProcess true is returned with the account", func(t *testing.T) {
		c := connectorFor(t, stub, nil)
		resp, err := c.RetrieveCredential(ctx, CCPRequest{Safe: testSafe, Object: "db-rotating"})
		if keep(err) != nil {
			t.Fatalf("RetrieveCredential: %v", err)
		}
		if !resp.PasswordChangeInProcess || resp.Content != changing || resp.UserName != "svc_rotating" {
			t.Errorf("PasswordChangeInProcess = %v, UserName = %q, Content matches = %v; want true, svc_rotating, true",
				resp.PasswordChangeInProcess, resp.UserName, resp.Content == changing)
		}
	})

	t.Run("400 maps to ErrCCPBadRequest and another status to a generic error", func(t *testing.T) {
		c := connectorFor(t, stub, nil)
		t.Cleanup(stub.ClearAnswer)

		stub.AnswerEvery(http.StatusBadRequest, "AIMWS030E Invalid query format.")
		_, err := c.RetrieveCredential(ctx, CCPRequest{Safe: testSafe, Object: testObject})
		keep(err)
		var statusErr *CCPStatusError
		if !errors.Is(err, ErrCCPBadRequest) || !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusBadRequest {
			t.Errorf("400: err = %v, want ErrCCPBadRequest with status 400", err)
		}

		stub.AnswerEvery(http.StatusBadGateway, "upstream unavailable")
		_, err = c.RetrieveCredential(ctx, CCPRequest{Safe: testSafe, Object: testObject})
		keep(err)
		statusErr = nil
		if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusBadGateway {
			t.Fatalf("502: err = %v, want a CCPStatusError with status 502", err)
		}
		for _, sentinel := range []error{ErrCCPBadRequest, ErrCCPForbidden, ErrCCPNotFound, ErrCCPServerError} {
			if errors.Is(err, sentinel) {
				t.Errorf("502 maps to %v, want no typed error", sentinel)
			}
		}
		if !strings.Contains(err.Error(), "502") {
			t.Errorf("502: error %q does not carry the status", err)
		}

		stub.AnswerEvery(http.StatusFound, "")
		before := len(stub.Requests())
		_, err = c.RetrieveCredential(ctx, CCPRequest{Safe: testSafe, Object: testObject})
		keep(err)
		statusErr = nil
		if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusFound {
			t.Errorf("302: err = %v, want a CCPStatusError with status 302", err)
		}
		if n := len(stub.Requests()) - before; n != 1 {
			t.Errorf("302: the stub recorded %d request(s), want 1 (the redirect is not followed)", n)
		}
	})

	t.Run("an error carries at most 256 bytes of the answer", func(t *testing.T) {
		c := connectorFor(t, stub, nil)
		t.Cleanup(stub.ClearAnswer)
		long := "APPAP306E " + strings.Repeat("x", 1000)
		stub.AnswerEvery(http.StatusForbidden, long)
		_, err := c.RetrieveCredential(ctx, CCPRequest{Safe: testSafe, Object: testObject})
		keep(err)
		var statusErr *CCPStatusError
		if !errors.As(err, &statusErr) {
			t.Fatalf("err = %v, want a CCPStatusError", err)
		}
		if len(statusErr.Body) != 256 || statusErr.Body != long[:256] {
			t.Errorf("error body is %d bytes, want the first 256", len(statusErr.Body))
		}
		if strings.Contains(err.Error(), long[:300]) {
			t.Errorf("error text carries more than 256 bytes of the answer")
		}
	})

	t.Run("a 200 body that does not decode never appears in the error", func(t *testing.T) {
		c := connectorFor(t, stub, nil)
		t.Cleanup(stub.ClearAnswer)
		bodies := []string{
			`{"Content":"` + content + `","PasswordChangeInProcess":"maybe"}`,
			`{"Content":` + content + `}`,
			content,
		}
		for _, body := range bodies {
			stub.AnswerEvery(http.StatusOK, body)
			if _, err := c.RetrieveCredential(ctx, CCPRequest{Safe: testSafe, Object: testObject}); keep(err) == nil {
				t.Errorf("RetrieveCredential accepted a 200 body that is not the documented account object")
			}
		}
	})
}

func TestCCPResponsePasswordChangeInProcessForms(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want bool
	}{
		{`true`, true}, {`false`, false}, {`"true"`, true}, {`"True"`, true},
		{`"TRUE"`, true}, {`"false"`, false}, {`"False"`, false}, {`null`, false},
	} {
		var r CCPResponse
		if err := json.Unmarshal([]byte(`{"Content":"c","UserName":"u","PasswordChangeInProcess":`+tc.raw+`}`), &r); err != nil {
			t.Errorf("%s: %v", tc.raw, err)
			continue
		}
		if r.PasswordChangeInProcess != tc.want || r.Content != "c" || r.UserName != "u" {
			t.Errorf("%s: got %+v, want PasswordChangeInProcess %v", tc.raw, r, tc.want)
		}
	}
	var absent CCPResponse
	if err := json.Unmarshal([]byte(`{"Content":"c"}`), &absent); err != nil || absent.PasswordChangeInProcess {
		t.Errorf("absent flag: %+v, %v", absent, err)
	}
	for _, raw := range []string{`"yes"`, `1`, `{}`} {
		var r CCPResponse
		if err := json.Unmarshal([]byte(`{"PasswordChangeInProcess":`+raw+`}`), &r); err == nil {
			t.Errorf("%s: accepted", raw)
		}
	}
}

func TestNewConnectorConfig(t *testing.T) {
	stub := ccptest.New(t)
	dir := t.TempDir()
	marker := "file-content-" + runtimeContent(t)
	notPEM := filepath.Join(dir, "not-pem")
	if err := os.WriteFile(notPEM, []byte(marker), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("the TLS client presents the certificate, verifies with the CA bundle and needs TLS 1.2", func(t *testing.T) {
		c := connectorFor(t, stub, nil)
		if c.client.Timeout != 30*time.Second {
			t.Errorf("Timeout = %v, want 30s by default", c.client.Timeout)
		}
		tr, ok := c.client.Transport.(*http.Transport)
		if !ok || tr.TLSClientConfig == nil {
			t.Fatalf("transport = %T, want *http.Transport with a TLS config", c.client.Transport)
		}
		tc := tr.TLSClientConfig
		if tc.MinVersion != tls.VersionTLS12 {
			t.Errorf("MinVersion = %#x, want TLS 1.2", tc.MinVersion)
		}
		if tc.InsecureSkipVerify {
			t.Error("InsecureSkipVerify is set")
		}
		if len(tc.Certificates) != 1 {
			t.Errorf("%d client certificates, want 1", len(tc.Certificates))
		}
		caPEM, err := os.ReadFile(stub.CACertPath)
		if err != nil {
			t.Fatal(err)
		}
		want := x509.NewCertPool()
		want.AppendCertsFromPEM(caPEM)
		if tc.RootCAs == nil || !tc.RootCAs.Equal(want) {
			t.Error("RootCAs is not the CACertPath bundle")
		}
		withTimeout := connectorFor(t, stub, func(cfg *Config) { cfg.Timeout = 5 * time.Second })
		if withTimeout.client.Timeout != 5*time.Second {
			t.Errorf("Timeout = %v, want the configured 5s", withTimeout.client.Timeout)
		}
		noCA := connectorFor(t, stub, func(cfg *Config) { cfg.CACertPath = "" })
		if noCA.client.Transport.(*http.Transport).TLSClientConfig.RootCAs != nil {
			t.Error("an empty CACertPath does not leave RootCAs to the system roots")
		}
	})

	for _, tc := range []struct {
		name    string
		mutate  func(*Config)
		setting string
	}{
		{"an http CCPURL", func(c *Config) { c.CCPURL = strings.Replace(c.CCPURL, "https://", "http://", 1) }, "CCPURL"},
		{"a CCPURL without a host", func(c *Config) { c.CCPURL = "https://" }, "CCPURL"},
		{"a CCPURL with a query", func(c *Config) { c.CCPURL += "/?AppID=Other" }, "CCPURL"},
		{"a CCPURL with user information", func(c *Config) { c.CCPURL = strings.Replace(c.CCPURL, "https://", "https://user@", 1) }, "CCPURL"},
		{"CertPath without KeyPath", func(c *Config) { c.KeyPath = "" }, "CertPath and KeyPath"},
		{"KeyPath without CertPath", func(c *Config) { c.CertPath = "" }, "CertPath and KeyPath"},
		{"a CertPath that does not exist", func(c *Config) { c.CertPath = filepath.Join(dir, "missing") }, "CertPath"},
		{"a KeyPath that does not exist", func(c *Config) { c.KeyPath = filepath.Join(dir, "missing") }, "KeyPath"},
		{"a CertPath that is not PEM", func(c *Config) { c.CertPath = notPEM }, "CertPath and KeyPath"},
		{"a KeyPath that is not PEM", func(c *Config) { c.KeyPath = notPEM }, "CertPath and KeyPath"},
		{"a KeyPath that does not match the certificate", func(c *Config) {
			_, other := ccptest.UntrustedClientKeyPair(t)
			c.KeyPath = other
		}, "CertPath and KeyPath"},
		{"a CACertPath that does not exist", func(c *Config) { c.CACertPath = filepath.Join(dir, "missing") }, "CACertPath"},
		{"a CACertPath that is not PEM", func(c *Config) { c.CACertPath = notPEM }, "CACertPath"},
		{"a negative Timeout", func(c *Config) { c.Timeout = -time.Second }, "Timeout"},
	} {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			cfg := Config{CCPURL: stub.URL, AppID: testAppID, CertPath: stub.ClientCertPath, KeyPath: stub.ClientKeyPath, CACertPath: stub.CACertPath}
			tc.mutate(&cfg)
			c, err := NewConnector(cfg)
			if err == nil {
				t.Fatalf("NewConnector accepted it: %+v", c)
			}
			if !strings.Contains(err.Error(), tc.setting) {
				t.Errorf("error %q does not name %s", err, tc.setting)
			}
			if strings.Contains(err.Error(), marker) || strings.Contains(err.Error(), "BEGIN") {
				t.Errorf("error %q quotes a file's content", err)
			}
		})
	}
}
