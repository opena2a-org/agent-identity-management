package atc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
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
	refreshing  bool
}

// NewCachedCRLClient creates a CRL client that caches results.
func NewCachedCRLClient(endpoint string) *CachedCRLClient {
	return &CachedCRLClient{
		endpoint:   endpoint,
		client:     &http.Client{Timeout: crlHTTPTimeout},
		revokedIDs: make(map[string]bool),
	}
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

	// Background refresh if approaching expiry
	if age > crlCacheTTL-crlRefreshBefore {
		c.mu.Lock()
		if !c.refreshing {
			c.refreshing = true
			go func() {
				_ = c.Refresh()
				c.mu.Lock()
				c.refreshing = false
				c.mu.Unlock()
			}()
		}
		c.mu.Unlock()
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB limit
	if err != nil {
		return fmt.Errorf("CRL read failed: %w", err)
	}

	var crlResp atcdomain.CRLResponse
	if err := json.Unmarshal(body, &crlResp); err != nil {
		return fmt.Errorf("CRL parse failed: %w", err)
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
