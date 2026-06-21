// Package registry contains clients for AIM's outbound calls to the OpenA2A
// Registry. The Registry is the binding-decision Certificate Authority for Agent
// Trust Credentials (ATCs): AIM does NOT issue ATCs itself. Instead it computes
// an agent's 9-factor behavioral trust score and delegates issuance to the
// Registry's POST /internal/atc/issue endpoint, which signs (threshold Ed25519 +
// ML-DSA-65), records the issuance to the RFC 6962 transparency log, and returns
// the portable credential. See todo/roadmap/aim-atx-issuer.md (CA decision).
package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	// defaultRegistryURL is used when REGISTRY_BRIDGE_URL is unset. It matches the
	// default already used by RegistryBridgeService so both outbound integrations
	// target the same Registry.
	defaultRegistryURL = "https://api.oa2a.org"

	// issueATCPath is the Registry's internal issuance endpoint (scope
	// internal:threats:write on the service-account bearer token).
	issueATCPath = "/internal/atc/issue"

	atcClientTimeout = 30 * time.Second
	atcClientUA      = "aim-server/atc-issuer"
)

// ErrTokenNotConfigured is returned when REGISTRY_ATC_TOKEN is unset. Issuance is
// a privileged Registry call; without the service-account bearer there is no
// credential to attempt, so we fail closed rather than send an unauthenticated
// request.
var ErrTokenNotConfigured = errors.New("registry: REGISTRY_ATC_TOKEN is not set")

// ATCBehavioralProfile mirrors the Registry's behavioral profile shape. It
// summarizes observed runtime behavior; the checksum binds the credential to a
// specific 9-factor score breakdown without carrying the raw factors on the wire.
type ATCBehavioralProfile struct {
	Checksum        string    `json:"checksum"`
	GeneratedAt     time.Time `json:"generatedAt"`
	ObservationDays int       `json:"observationDays"`
}

// ATCIssuanceRequest mirrors the Registry's ATCIssuanceRequest. TrustScore,
// TrustLevel, and BehavioralProfile are the optional behavioral overrides the
// Registry applies when the subject is a did:aip agent (rather than a package).
// For an AIM agent they are always set: they carry AIM's behavioral score, not
// the Registry's supply-chain package score.
type ATCIssuanceRequest struct {
	AgentID           string                `json:"agentId"`
	AgentDID          string                `json:"agentDid"`
	Publisher         string                `json:"publisher"`
	Version           string                `json:"version"`
	ContentHash       string                `json:"contentHash"`
	Capabilities      []string              `json:"capabilities,omitempty"`
	TrustScore        *float64              `json:"trustScore,omitempty"`
	TrustLevel        *int                  `json:"trustLevel,omitempty"`
	BehavioralProfile *ATCBehavioralProfile `json:"behavioralProfile,omitempty"`
}

// ATCSignature mirrors a single signature on the returned credential.
type ATCSignature struct {
	KeyID     string `json:"keyId"`
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

// AgentTrustCredential mirrors the subset of the Registry's response that AIM
// surfaces to callers. The full response is also retained in Raw so the handler
// can return the Registry's exact signed bytes without lossy re-marshalling.
type AgentTrustCredential struct {
	ID                   string                `json:"id"`
	ATCVersion           string                `json:"atcVersion"`
	AgentID              string                `json:"agentId"`
	AgentDID             string                `json:"agentDid"`
	Publisher            string                `json:"publisher"`
	Version              string                `json:"version"`
	ContentHash          string                `json:"contentHash"`
	TransparencyLogIndex int64                 `json:"transparencyLogIndex"`
	Capabilities         []string              `json:"capabilities"`
	BehavioralProfile    *ATCBehavioralProfile `json:"behavioralProfile,omitempty"`
	TrustScore           float64               `json:"trustScore"`
	TrustLevel           int                   `json:"trustLevel"`
	IssuedAt             time.Time             `json:"issuedAt"`
	ExpiresAt            time.Time             `json:"expiresAt"`
	IssuerDID            string                `json:"issuerDid"`
	Signatures           []ATCSignature        `json:"signatures"`

	// Raw is the verbatim JSON the Registry returned, populated by the client so
	// callers can pass the signed credential through unchanged.
	Raw json.RawMessage `json:"-"`
}

// Client calls the Registry's ATC issuance endpoint.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClientFromEnv builds a Client from REGISTRY_BRIDGE_URL (shared with the
// bridge service, default https://api.oa2a.org) and REGISTRY_ATC_TOKEN (the
// new service-account bearer provisioned in Phase C).
func NewClientFromEnv() *Client {
	base := strings.TrimSpace(os.Getenv("REGISTRY_BRIDGE_URL"))
	if base == "" {
		base = defaultRegistryURL
	}
	return &Client{
		baseURL:    strings.TrimRight(base, "/"),
		token:      strings.TrimSpace(os.Getenv("REGISTRY_ATC_TOKEN")),
		httpClient: &http.Client{Timeout: atcClientTimeout},
	}
}

// NewClient builds a Client with explicit configuration (used by tests).
func NewClient(baseURL, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: atcClientTimeout}
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      strings.TrimSpace(token),
		httpClient: httpClient,
	}
}

// IssueATC POSTs the issuance request to the Registry and returns the signed
// credential. Errors fail closed: a missing token, a transport failure, or a
// non-2xx status all return an error rather than a partial credential.
func (c *Client) IssueATC(ctx context.Context, req ATCIssuanceRequest) (*AgentTrustCredential, error) {
	if c.token == "" {
		return nil, ErrTokenNotConfigured
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("registry: marshal issuance request: %w", err)
	}

	url := c.baseURL + issueATCPath
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("registry: build issuance request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	httpReq.Header.Set("User-Agent", atcClientUA)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("registry: issuance request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("registry: read issuance response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("registry: issuance returned status %d: %s", resp.StatusCode, snippet(respBody))
	}

	var cred AgentTrustCredential
	if err := json.Unmarshal(respBody, &cred); err != nil {
		return nil, fmt.Errorf("registry: decode credential: %w", err)
	}
	cred.Raw = json.RawMessage(respBody)
	return &cred, nil
}

// snippet trims an error body so a large or HTML error page does not flood logs.
func snippet(b []byte) string {
	const max = 256
	s := strings.TrimSpace(string(b))
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
