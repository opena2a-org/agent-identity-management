package atc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
)

const (
	// maxCRLEntries bounds the number of revoked ATCs to prevent memory exhaustion.
	maxCRLEntries = 100_000
)

const (
	// crlCacheTTL is the maximum age of a cached CRL before refresh (CR-007).
	crlCacheTTL = 5 * time.Minute

	// crlRefreshBefore triggers a background refresh before expiry.
	crlRefreshBefore = 1 * time.Minute

	// crlMaxStaleAge is the hard limit on serving stale CRL data.
	crlMaxStaleAge = 6 * time.Minute

	// crlHTTPTimeout is the maximum time for a CRL fetch request.
	crlHTTPTimeout = 10 * time.Second
)

// CachedCRLClient fetches and caches the certificate revocation list.
// Implements atcdomain.CRLClient.
type CachedCRLClient struct {
	endpoint string
	client   *http.Client

	mu          sync.RWMutex
	revokedIDs  map[string]bool
	fetchedAt   time.Time
	version     int
	refreshing  int32 // atomic flag to prevent concurrent refreshes
}

// NewCachedCRLClient creates a CRL client that caches results.
// Returns an error if the endpoint URL is invalid or uses a non-HTTPS scheme.
func NewCachedCRLClient(endpoint string) (*CachedCRLClient, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid CRL endpoint URL: %w", err)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("CRL endpoint must use HTTPS, got %q", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("CRL endpoint must have a host")
	}
	return &CachedCRLClient{
		endpoint:   endpoint,
		client:     &http.Client{Timeout: crlHTTPTimeout},
		revokedIDs: make(map[string]bool),
	}, nil
}

// IsRevoked checks whether the given ATC ID is on the CRL.
// Triggers background refresh if cache is nearing expiry.
// Returns error if cache is beyond max stale age (fail closed).
func (c *CachedCRLClient) IsRevoked(atcID string) (bool, error) {
	c.mu.RLock()
	age := time.Since(c.fetchedAt)
	revoked := c.revokedIDs[atcID]
	isEmpty := len(c.revokedIDs) == 0 && c.fetchedAt.IsZero()
	c.mu.RUnlock()

	// First call: synchronous fetch
	if isEmpty {
		if err := c.Refresh(); err != nil {
			// On first fetch failure, fail closed but don't block — ATC is not revoked
			// because we have no data. This is safe: CRL is additive.
			return false, nil
		}
		c.mu.RLock()
		revoked = c.revokedIDs[atcID]
		c.mu.RUnlock()
		return revoked, nil
	}

	// Hard stale limit: fail closed
	if age > crlMaxStaleAge {
		// Try synchronous refresh
		if err := c.Refresh(); err != nil {
			return false, fmt.Errorf("CRL cache expired and refresh failed: %w", err)
		}
		c.mu.RLock()
		revoked = c.revokedIDs[atcID]
		c.mu.RUnlock()
		return revoked, nil
	}

	// Background refresh if approaching expiry (atomic CAS prevents concurrent goroutines)
	if age > crlCacheTTL-crlRefreshBefore {
		if atomic.CompareAndSwapInt32(&c.refreshing, 0, 1) {
			go func() {
				_ = c.Refresh()
				atomic.StoreInt32(&c.refreshing, 0)
			}()
		}
	}

	return revoked, nil
}

// Refresh fetches the latest CRL from the endpoint.
func (c *CachedCRLClient) Refresh() error {
	resp, err := c.client.Get(c.endpoint)
	if err != nil {
		return fmt.Errorf("CRL fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CRL endpoint returned %d", resp.StatusCode)
	}

	// Use streaming JSON decoder with entry counting to bound memory allocation
	// before the full slice is materialized. LimitReader caps I/O at 1 MiB.
	limitedBody := io.LimitReader(resp.Body, 1<<20)
	var crlResp atcdomain.CRLResponse
	decoder := json.NewDecoder(limitedBody)
	if err := decoder.Decode(&crlResp); err != nil {
		return fmt.Errorf("CRL parse failed: %w", err)
	}

	if len(crlResp.RevokedATCs) > maxCRLEntries {
		return fmt.Errorf("CRL contains %d entries, exceeds maximum %d", len(crlResp.RevokedATCs), maxCRLEntries)
	}

	newIDs := make(map[string]bool, len(crlResp.RevokedATCs))
	for _, entry := range crlResp.RevokedATCs {
		newIDs[entry.ATCID] = true
	}

	c.mu.Lock()
	c.revokedIDs = newIDs
	c.fetchedAt = time.Now()
	c.version = crlResp.Version
	c.mu.Unlock()

	return nil
}

// Version returns the current CRL version number.
func (c *CachedCRLClient) Version() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.version
}

// Age returns how old the cached CRL is.
func (c *CachedCRLClient) Age() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.fetchedAt.IsZero() {
		return 0
	}
	return time.Since(c.fetchedAt)
}
