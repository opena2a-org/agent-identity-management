// Package cyberark implements the AIM CyberArk connector: a client for
// CyberArk's Central Credential Provider (CCP) REST web service, which
// retrieves an account's credential with GET /AIMWebService/api/Accounts over
// TLS, presenting a client certificate when CertPath and KeyPath are set.
package cyberark

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DefaultTimeout is the request timeout used when Config.Timeout is zero.
const DefaultTimeout = 30 * time.Second

// accountsPath is the CCP REST endpoint that returns an account.
const accountsPath = "/AIMWebService/api/Accounts"

// maxErrorBody is the number of bytes of a non-200 answer an error carries.
const maxErrorBody = 256

// maxAccountBody bounds the 200 answer the connector decodes.
const maxAccountBody = 1 << 20

// unsupportedValueChars are the characters CyberArk does not accept in a CCP
// URL value. CyberArk's rule against spaces is about the URL, and encode
// writes a space in a value as %20, so a space is not among them.
const unsupportedValueChars = "+&%"

// propertySeparator separates the account properties a free Query names, as in
// Safe=AIM-Database;Object=db. CyberArk does not accept it in a property name
// or value, so no other parameter's value may hold it, and in a Query each one
// must separate two properties.
const propertySeparator = ";"

// Config holds CyberArk integration configuration.
type Config struct {
	CCPURL     string        `json:"ccpUrl"`     // https base URL of the CCP web service
	AppID      string        `json:"appId"`      // Application ID registered in CyberArk, used when a request names none
	CertPath   string        `json:"certPath"`   // PEM client certificate presented to CCP (mutual TLS)
	KeyPath    string        `json:"keyPath"`    // PEM private key of the client certificate
	CACertPath string        `json:"caCertPath"` // PEM bundle that verifies the CCP server; empty means the system roots
	Timeout    time.Duration `json:"timeout"`    // whole-request timeout; zero means DefaultTimeout
}

// Connector retrieves credentials from CyberArk CCP.
type Connector struct {
	config      Config
	accountsURL string
	client      *http.Client
}

// CCPRequest is a query to GET /AIMWebService/api/Accounts. Each non-empty
// field is sent under CyberArk's own parameter name. AppID is required, with
// at least one other parameter.
type CCPRequest struct {
	AppID    string // AppID; Config.AppID when empty
	Safe     string // Safe
	Folder   string // Folder
	Object   string // Object
	UserName string // UserName
	Address  string // Address
	Database string // Database
	PolicyID string // PolicyID
	Reason   string // Reason
	// Query is a free query: account properties written name=value and
	// separated by ';', such as Safe=AIM-Database;Object=db.
	Query string
	// QueryFormat is Exact or Regexp; empty omits it.
	QueryFormat string
	// ConnectionTimeout is sent as "Connection Timeout", in seconds; zero
	// omits it.
	ConnectionTimeout int
	// FailRequestOnPasswordChange sends FailRequestOnPasswordChange=true:
	// CCP then answers 500 (APPAP282E) while the CPM is changing the
	// password. When false the parameter is not sent.
	FailRequestOnPasswordChange bool
}

// CCPResponse is the account CCP returns. Content is the credential value.
type CCPResponse struct {
	Content  string `json:"Content"`
	UserName string `json:"UserName"`
	Address  string `json:"Address"`
	Database string `json:"Database"`
	// PasswordChangeInProcess is true while the CPM is changing the
	// password. The connector returns the account as CCP sent it; the
	// caller decides whether to use it.
	PasswordChangeInProcess bool `json:"PasswordChangeInProcess"`
}

// UnmarshalJSON accepts PasswordChangeInProcess as a JSON boolean or as the
// string "true" or "false" in any letter case.
func (r *CCPResponse) UnmarshalJSON(data []byte) error {
	type plain CCPResponse
	var aux struct {
		plain
		PasswordChangeInProcess json.RawMessage `json:"PasswordChangeInProcess"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*r = CCPResponse(aux.plain)
	flag, err := parseFlag(aux.PasswordChangeInProcess)
	if err != nil {
		return err
	}
	r.PasswordChangeInProcess = flag
	return nil
}

func parseFlag(raw json.RawMessage) (bool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return false, nil
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch {
		case strings.EqualFold(s, "true"):
			return true, nil
		case strings.EqualFold(s, "false"):
			return false, nil
		}
	}
	return false, errors.New("cyberark: CCP field PasswordChangeInProcess is neither a boolean nor the string true or false")
}

// Errors a CCPStatusError unwraps to, by the status CCP answered.
var (
	// ErrCCPBadRequest is a 400: CCP refused the query (AIMWS030E,
	// AIMWS031E, APPAP227E-229E, APPAP007E, APPBC007E, APPAP081E,
	// CASVL010E).
	ErrCCPBadRequest = errors.New("cyberark: CCP refused the query")
	// ErrCCPForbidden is a 403: CCP refused the application (APPAP306E,
	// APPAP008E).
	ErrCCPForbidden = errors.New("cyberark: CCP refused the application")
	// ErrCCPNotFound is a 404: CCP found no matching safe or object
	// (APPAP004E).
	ErrCCPNotFound = errors.New("cyberark: CCP found no matching account")
	// ErrCCPServerError is a 500, the status CCP answers with APPAP282E while
	// the CPM is changing the password and FailRequestOnPasswordChange was
	// sent.
	ErrCCPServerError = errors.New("cyberark: CCP could not serve the request")
)

// CCPStatusError is a non-200 answer from CCP. It carries the status and at
// most the first 256 bytes of CCP's answer. It unwraps to ErrCCPBadRequest,
// ErrCCPForbidden, ErrCCPNotFound or ErrCCPServerError for 400, 403, 404 and
// 500, and to nothing for any other status.
type CCPStatusError struct {
	StatusCode int
	Body       string
}

// Error names the status and quotes the answer excerpt.
func (e *CCPStatusError) Error() string {
	if kind := e.Unwrap(); kind != nil {
		return fmt.Sprintf("%v (status %d): %q", kind, e.StatusCode, e.Body)
	}
	return fmt.Sprintf("cyberark: CCP answered status %d: %q", e.StatusCode, e.Body)
}

// Unwrap returns the sentinel error for the status, or nil.
func (e *CCPStatusError) Unwrap() error {
	switch e.StatusCode {
	case http.StatusBadRequest:
		return ErrCCPBadRequest
	case http.StatusForbidden:
		return ErrCCPForbidden
	case http.StatusNotFound:
		return ErrCCPNotFound
	case http.StatusInternalServerError:
		return ErrCCPServerError
	default:
		return nil
	}
}

// NewConnector checks config, loads the client key pair and the CA bundle,
// and returns a connector whose TLS client presents the client certificate,
// verifies the CCP server against CACertPath (or the system roots) and
// negotiates TLS 1.2 or later. An error names the setting at fault and never
// a file's content.
func NewConnector(config Config) (*Connector, error) {
	base, err := url.Parse(config.CCPURL)
	if err != nil || base.Scheme != "https" || base.Host == "" {
		return nil, errors.New("cyberark: CCPURL must be an https URL with a host")
	}
	if base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("cyberark: CCPURL must not carry user information, a query or a fragment")
	}
	if (config.CertPath == "") != (config.KeyPath == "") {
		return nil, errors.New("cyberark: CertPath and KeyPath must both be set or both be empty")
	}
	if config.Timeout < 0 {
		return nil, errors.New("cyberark: Timeout must not be negative")
	}
	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if config.CertPath != "" {
		certPEM, err := os.ReadFile(filepath.Clean(config.CertPath))
		if err != nil {
			return nil, fmt.Errorf("cyberark: CertPath could not be read: %w", err)
		}
		keyPEM, err := os.ReadFile(filepath.Clean(config.KeyPath))
		if err != nil {
			return nil, fmt.Errorf("cyberark: KeyPath could not be read: %w", err)
		}
		pair, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			return nil, errors.New("cyberark: CertPath and KeyPath do not hold a PEM certificate and its matching private key")
		}
		tlsConfig.Certificates = []tls.Certificate{pair}
	}
	if config.CACertPath != "" {
		caPEM, err := os.ReadFile(filepath.Clean(config.CACertPath))
		if err != nil {
			return nil, fmt.Errorf("cyberark: CACertPath could not be read: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, errors.New("cyberark: CACertPath holds no PEM certificate")
		}
		tlsConfig.RootCAs = pool
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig

	return &Connector{
		config:      config,
		accountsURL: strings.TrimRight(base.String(), "/") + accountsPath,
		client: &http.Client{
			Timeout:   config.Timeout,
			Transport: transport,
			// A redirect is answered as the status it carries; the
			// connector does not follow CCP to another location.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

// RetrieveCredential retrieves an account from CyberArk CCP. It checks req
// and encodes it as CyberArk publishes before anything is sent: a refused
// request names the parameter at fault and never its value. A 200 returns the
// account; any other status returns a *CCPStatusError.
func (c *Connector) RetrieveCredential(ctx context.Context, req CCPRequest) (*CCPResponse, error) {
	if req.AppID == "" {
		req.AppID = c.config.AppID
	}
	query, err := req.encode()
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.accountsURL+"?"+query, nil)
	if err != nil {
		return nil, fmt.Errorf("cyberark: building the CCP request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("cyberark: CCP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		excerpt, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		return nil, &CCPStatusError{StatusCode: resp.StatusCode, Body: string(excerpt)}
	}

	var account CCPResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxAccountBody)).Decode(&account); err != nil {
		// The decoder's message can quote the answer, which holds the
		// credential; report the outcome only.
		return nil, errors.New("cyberark: CCP answered 200 with a body that is not the documented account object")
	}
	return &account, nil
}

type ccpParam struct {
	name  string
	value string
}

// params lists the request's parameters in the order CyberArk documents them,
// under CyberArk's names, leaving out the ones that are not set.
func (r CCPRequest) params() []ccpParam {
	all := []ccpParam{
		{"AppID", r.AppID},
		{"Safe", r.Safe},
		{"Folder", r.Folder},
		{"Object", r.Object},
		{"UserName", r.UserName},
		{"Address", r.Address},
		{"Database", r.Database},
		{"PolicyID", r.PolicyID},
		{"Reason", r.Reason},
		{"Connection Timeout", ""},
		{"Query", r.Query},
		{"QueryFormat", r.QueryFormat},
		{"FailRequestOnPasswordChange", ""},
	}
	if r.ConnectionTimeout > 0 {
		all[9].value = strconv.Itoa(r.ConnectionTimeout)
	}
	if r.FailRequestOnPasswordChange {
		all[12].value = "true"
	}
	set := all[:0]
	for _, p := range all {
		if p.value != "" {
			set = append(set, p)
		}
	}
	return set
}

// encode checks the request and returns its query string. Every name and value
// is query-escaped with a space written as %20, so the query holds no raw
// space, no '+' and no raw ';'.
func (r CCPRequest) encode() (string, error) {
	if r.AppID == "" {
		return "", errors.New("cyberark: CCP request has no AppID; set CCPRequest.AppID or Config.AppID")
	}
	if r.ConnectionTimeout < 0 {
		return "", errors.New("cyberark: CCP parameter Connection Timeout must not be negative")
	}
	params := r.params()
	if len(params) < 2 {
		return "", errors.New("cyberark: CCP request names only AppID; CCP needs at least one other parameter, such as Safe and Object or Query")
	}
	for _, p := range params {
		if strings.ContainsAny(p.value, unsupportedValueChars) {
			return "", fmt.Errorf("cyberark: CCP parameter %s holds a character CCP does not accept in a URL value (+, & or %%)", p.name)
		}
		if p.name == "Query" {
			if !separatesProperties(p.value) {
				return "", errors.New("cyberark: CCP parameter Query holds a ';' that does not separate two name=value properties; CCP does not accept one in a property name or value")
			}
		} else if strings.Contains(p.value, propertySeparator) {
			return "", fmt.Errorf("cyberark: CCP parameter %s holds a ';', which CCP does not accept in a property name or value", p.name)
		}
	}
	if r.QueryFormat != "" && r.QueryFormat != "Exact" && r.QueryFormat != "Regexp" {
		return "", errors.New("cyberark: CCP parameter QueryFormat must be Exact or Regexp")
	}

	pairs := make([]string, 0, len(params))
	for _, p := range params {
		pairs = append(pairs, escape(p.name)+"="+escape(p.value))
	}
	return strings.Join(pairs, "&"), nil
}

// separatesProperties reports whether every ';' in query separates two account
// properties written name=value with a non-empty name. A query that holds no
// ';' passes.
func separatesProperties(query string) bool {
	if !strings.Contains(query, propertySeparator) {
		return true
	}
	for _, property := range strings.Split(query, propertySeparator) {
		if name, _, ok := strings.Cut(property, "="); !ok || name == "" {
			return false
		}
	}
	return true
}

// escape query-escapes s and writes a space as %20 rather than '+'. A '+' in s
// is escaped as %2B by url.QueryEscape, so the result holds no '+'.
func escape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// MapCapabilityToSafe maps an AIM capability to a CyberArk Safe name.
// Safe naming convention: "AIM-<capability_category>"
func MapCapabilityToSafe(capability string) string {
	switch {
	case len(capability) > 3 && capability[:3] == "db.":
		return "AIM-Database"
	case len(capability) > 4 && capability[:4] == "api.":
		return "AIM-APIKeys"
	case len(capability) > 6 && capability[:6] == "infra.":
		return "AIM-Infrastructure"
	default:
		return "AIM-General"
	}
}
