// Package ccptest is a test double of the CyberArk Central Credential Provider
// (CCP) REST web service, for tests only.
//
// It is written from CyberArk's published description of the service
// ("Calling the Web Service using REST" and "Authentication methods for CCP"):
// it answers GET /AIMWebService/api/Accounts only, over TLS, and refuses a
// handshake from a client that does not present a certificate signed by the
// stub's own CA. Its CA, server certificate and client certificate are
// generated when the stub starts, and the client key pair and the CA bundle are
// written to the test's temporary directory.
//
// Statuses and message codes are the ones CyberArk publishes. The shape of
// CCP's error body is not published on the pages the stub is written from, so
// the stub answers a plain-text line that starts with the code, and the
// wording after the code is the stub's own.
package ccptest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// AccountsPath is the only path the stub answers.
const AccountsPath = "/AIMWebService/api/Accounts"

// ClientCommonName is the subject common name of the client certificate the
// stub issues and trusts.
const ClientCommonName = "aim-ccp-client"

// Account is a password object the stub serves for one (AppID, Safe, Object).
// The stub finds it by the AppID, Safe and Object parameters of a request and
// does not evaluate a Query.
type Account struct {
	AppID    string
	Safe     string
	Object   string
	Content  string
	UserName string
	Address  string
	Database string
	// PasswordChanging marks the object's password as being changed by the
	// CPM. A request that sends FailRequestOnPasswordChange=true is then
	// answered 500 APPAP282E; any other request is answered 200 with
	// PasswordChangeInProcess true.
	PasswordChanging bool
}

// Request is one request that reached the stub's handler. A connection whose
// TLS handshake failed never produces one.
type Request struct {
	Method string
	// RequestURI is the request target exactly as the client sent it.
	RequestURI string
	// Params holds each query parameter, percent-decoded without turning
	// '+' into a space, in the order the client sent them.
	Params url.Values
	// ClientCommonName is the subject common name of the certificate the
	// client presented.
	ClientCommonName string
}

// Server is a running CCP stub.
type Server struct {
	// URL is the https base URL of the stub, such as https://127.0.0.1:port.
	URL string
	// CACertPath is a PEM bundle holding the CA that signed the stub's server
	// certificate and the client certificate.
	CACertPath string
	// ClientCertPath and ClientKeyPath hold a PEM client certificate the stub
	// trusts and its private key.
	ClientCertPath string
	ClientKeyPath  string

	mu       sync.Mutex
	requests []Request
	accounts map[accountKey]Account
	override *override
}

type accountKey struct{ appID, safe, object string }

type override struct {
	status int
	body   string
}

// New starts a CCP stub that requires a client certificate signed by its own CA
// and stops it when the test ends.
func New(t testing.TB) *Server {
	t.Helper()
	dir := t.TempDir()

	ca := newCA(t, "aim-ccp-stub-ca")
	serverCert := ca.issue(t, "aim-ccp-stub-server", x509.ExtKeyUsageServerAuth, []net.IP{net.ParseIP("127.0.0.1")})
	clientCert := ca.issue(t, ClientCommonName, x509.ExtKeyUsageClientAuth, nil)

	s := &Server{
		CACertPath:     filepath.Join(dir, "ccp-ca-bundle.crt"),
		ClientCertPath: filepath.Join(dir, "ccp-client.crt"),
		ClientKeyPath:  filepath.Join(dir, "ccp-client-private-key"),
		accounts:       map[accountKey]Account{},
	}
	writeFile(t, s.CACertPath, ca.certPEM)
	writeFile(t, s.ClientCertPath, clientCert.certPEM)
	writeFile(t, s.ClientKeyPath, clientCert.keyPEM)

	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(ca.cert)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(s.serveHTTP))
	srv.TLS = &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{serverCert.pair(t)},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
	}
	// A refused handshake is an expected outcome in these tests; keep the
	// server's handshake log lines out of the test output.
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	srv.StartTLS()
	t.Cleanup(srv.Close)
	s.URL = srv.URL
	return s
}

// UnrelatedCABundle writes a PEM bundle holding a CA that signed none of the
// stub's certificates and returns its path. A client that verifies the stub
// with it fails the handshake.
func UnrelatedCABundle(t testing.TB) string {
	t.Helper()
	ca := newCA(t, "aim-ccp-unrelated-ca")
	path := filepath.Join(t.TempDir(), "unrelated-ca-bundle.crt")
	writeFile(t, path, ca.certPEM)
	return path
}

// UntrustedClientKeyPair writes a client certificate signed by a CA the stub
// does not trust, and its private key, and returns their paths. The stub
// refuses a handshake that presents it.
func UntrustedClientKeyPair(t testing.TB) (certPath, keyPath string) {
	t.Helper()
	ca := newCA(t, "aim-ccp-untrusted-client-ca")
	cert := ca.issue(t, ClientCommonName, x509.ExtKeyUsageClientAuth, nil)
	dir := t.TempDir()
	certPath = filepath.Join(dir, "untrusted-client.crt")
	keyPath = filepath.Join(dir, "untrusted-client-private-key")
	writeFile(t, certPath, cert.certPEM)
	writeFile(t, keyPath, cert.keyPEM)
	return certPath, keyPath
}

// AddAccount registers a password object. Its AppID becomes a known
// application and its Safe a known safe.
func (s *Server) AddAccount(a Account) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts[accountKey{a.AppID, a.Safe, a.Object}] = a
}

// SetPasswordChanging marks or clears a registered object's password as being
// changed by the CPM.
func (s *Server) SetPasswordChanging(appID, safe, object string, changing bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := accountKey{appID, safe, object}
	if a, ok := s.accounts[k]; ok {
		a.PasswordChanging = changing
		s.accounts[k] = a
	}
}

// AnswerEvery makes the stub record each request and answer it with status and
// body, whatever it asks for, until ClearAnswer is called. A 3xx status carries
// a Location header that points back at the stub. It is not CCP
// behaviour: it lets a test reach an answer that a client's own checks keep it
// from provoking.
func (s *Server) AnswerEvery(status int, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.override = &override{status: status, body: body}
}

// ClearAnswer ends AnswerEvery.
func (s *Server) ClearAnswer() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.override = nil
}

// Requests returns a copy of every request that reached the stub, in order.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Request, len(s.requests))
	copy(out, s.requests)
	return out
}

// documentedParams are the request parameters CyberArk documents for
// GET /AIMWebService/api/Accounts.
var documentedParams = []string{
	"AppID", "Safe", "Folder", "Object", "UserName", "Address", "Database",
	"PolicyID", "Reason", "Connection Timeout", "Query", "QueryFormat",
	"FailRequestOnPasswordChange",
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	rec := Request{Method: r.Method, RequestURI: r.RequestURI}
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		rec.ClientCommonName = r.TLS.PeerCertificates[0].Subject.CommonName
	}
	params, parseErr := parseQuery(r.URL.RawQuery)
	rec.Params = params

	s.mu.Lock()
	s.requests = append(s.requests, rec)
	ov := s.override
	s.mu.Unlock()

	if ov != nil {
		if ov.status >= 300 && ov.status < 400 {
			// Point a redirect back at the stub, so a client that
			// follows it is recorded again.
			w.Header().Set("Location", AccountsPath)
		}
		writeText(w, ov.status, ov.body)
		return
	}
	if r.URL.Path != AccountsPath {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeText(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// "Make sure there are no spaces in the URL."
	if strings.Contains(r.RequestURI, " ") || parseErr != nil {
		writeText(w, http.StatusBadRequest, "AIMWS030E Invalid query format.")
		return
	}
	for name, values := range params {
		// "A property name or value may not contain ;"
		if strings.Contains(name, ";") {
			writeText(w, http.StatusBadRequest, "AIMWS030E Invalid query format.")
			return
		}
		for _, v := range values {
			// "The following characters are not supported in URL values: + & %"
			if strings.ContainsAny(v, "+&%") {
				writeText(w, http.StatusBadRequest, "AIMWS030E Invalid query format.")
				return
			}
			// A free Query separates the account properties it names with
			// ';', so each ';' in it must separate two of them; no other
			// value may hold one.
			if (name == "Query" && !separatesProperties(v)) || (name != "Query" && strings.Contains(v, ";")) {
				writeText(w, http.StatusBadRequest, "AIMWS030E Invalid query format.")
				return
			}
		}
	}
	appID := params.Get("AppID")
	if appID == "" {
		writeText(w, http.StatusBadRequest, "AIMWS031E Invalid query format. The AppID parameter is required.")
		return
	}
	if qf, ok := params["QueryFormat"]; ok && qf[0] != "Exact" && qf[0] != "Regexp" {
		writeText(w, http.StatusBadRequest, "AIMWS030E Invalid query format. QueryFormat must be Exact or Regexp.")
		return
	}

	safe, object := params.Get("Safe"), params.Get("Object")
	s.mu.Lock()
	knownApp, knownSafe := false, false
	for k := range s.accounts {
		if k.appID == appID {
			knownApp = true
			if k.safe == safe {
				knownSafe = true
			}
		}
	}
	acct, found := s.accounts[accountKey{appID, safe, object}]
	s.mu.Unlock()

	switch {
	case !knownApp:
		writeText(w, http.StatusForbidden, "APPAP306E App failed on authentication check.")
		return
	case !knownSafe:
		writeText(w, http.StatusNotFound, "APPAP004E Safe not found.")
		return
	case !found:
		writeText(w, http.StatusNotFound, "APPAP004E Password object matching the query was not found in the safe.")
		return
	}

	if acct.PasswordChanging && params.Get("FailRequestOnPasswordChange") == "true" {
		writeText(w, http.StatusInternalServerError, "APPAP282E The password is being changed by the CPM.")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"Content":                 acct.Content,
		"UserName":                acct.UserName,
		"Address":                 acct.Address,
		"Database":                acct.Database,
		"PasswordChangeInProcess": acct.PasswordChanging,
	})
}

// separatesProperties reports whether every ';' in query separates two
// properties written name=value with a non-empty name. A query that holds no
// ';' passes.
func separatesProperties(query string) bool {
	if !strings.Contains(query, ";") {
		return true
	}
	for _, property := range strings.Split(query, ";") {
		if name, _, ok := strings.Cut(property, "="); !ok || name == "" {
			return false
		}
	}
	return true
}

// IsDocumentedParam reports whether name is one of the request parameters
// CyberArk documents for GET /AIMWebService/api/Accounts.
func IsDocumentedParam(name string) bool {
	for _, p := range documentedParams {
		if p == name {
			return true
		}
	}
	return false
}

// parseQuery splits a raw query on '&' and '=' and percent-decodes each name
// and value. Unlike url.ParseQuery it keeps '+' as '+', so a value that holds
// a '+' is seen as one, and it accepts ';' so that the handler can refuse it.
func parseQuery(raw string) (url.Values, error) {
	params := url.Values{}
	if raw == "" {
		return params, nil
	}
	var firstErr error
	for _, pair := range strings.Split(raw, "&") {
		if pair == "" {
			continue
		}
		name, value, _ := strings.Cut(pair, "=")
		n, err := url.PathUnescape(name)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		v, err := url.PathUnescape(value)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		params[n] = append(params[n], v)
	}
	return params, firstErr
}

func writeText(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

type issued struct {
	cert    *x509.Certificate
	key     *ecdsa.PrivateKey
	certPEM []byte
	keyPEM  []byte
}

func (i issued) pair(t testing.TB) tls.Certificate {
	t.Helper()
	pair, err := tls.X509KeyPair(i.certPEM, i.keyPEM)
	if err != nil {
		t.Fatalf("ccptest: building a key pair: %v", err)
	}
	return pair
}

func newCA(t testing.TB, commonName string) issued {
	t.Helper()
	key := newKey(t)
	tmpl := &x509.Certificate{
		SerialNumber:          serial(t),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("ccptest: creating a CA certificate: %v", err)
	}
	return finish(t, der, key)
}

func (ca issued) issue(t testing.TB, commonName string, usage x509.ExtKeyUsage, ips []net.IP) issued {
	t.Helper()
	key := newKey(t)
	tmpl := &x509.Certificate{
		SerialNumber: serial(t),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
		IPAddresses:  ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatalf("ccptest: issuing a certificate: %v", err)
	}
	return finish(t, der, key)
}

func finish(t testing.TB, der []byte, key *ecdsa.PrivateKey) issued {
	t.Helper()
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ccptest: parsing a certificate: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("ccptest: encoding a private key: %v", err)
	}
	return issued{
		cert:    cert,
		key:     key,
		certPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		keyPEM:  pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
	}
}

func newKey(t testing.TB) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ccptest: generating a key: %v", err)
	}
	return key
}

func serial(t testing.TB) *big.Int {
	t.Helper()
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		t.Fatalf("ccptest: generating a serial number: %v", err)
	}
	return n
}

func writeFile(t testing.TB, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("ccptest: writing %s: %v", filepath.Base(path), err)
	}
}
