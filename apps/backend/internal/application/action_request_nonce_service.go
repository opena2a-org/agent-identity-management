package application

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// ActionRequestNonceService admits the nonces of signed action-request
// statements and purges them once no request can be admitted with them.
//
// Admission fails closed while the purge is not running: a nonce row is
// written only when this process completed a purge within two purge
// intervals, so the table never holds a row with no deletion path. The
// window itself is checked in the admission statement on the database clock;
// nothing here reads a clock for it.
type ActionRequestNonceService struct {
	repo domain.ActionRequestNonceRepository

	mu        sync.Mutex
	lastPurge time.Time // monotonic reading taken when the last purge completed
	now       func() time.Time
}

// NewActionRequestNonceService creates the service. It admits nothing until
// its first purge completes.
func NewActionRequestNonceService(repo domain.ActionRequestNonceRepository) *ActionRequestNonceService {
	return &ActionRequestNonceService{repo: repo, now: time.Now}
}

// purgeRunning reports whether a purge completed within two intervals.
func (s *ActionRequestNonceService) purgeRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastPurge.IsZero() {
		return false
	}
	return s.now().Sub(s.lastPurge) <= 2*domain.ActionRequestNoncePurgeInterval
}

// Admit checks the signed timestamp against the database clock and records
// (agentID, nonce) in one statement. organizationID comes from the server's
// lookup of the agent whose registered key verified the statement.
func (s *ActionRequestNonceService) Admit(ctx context.Context, agentID, organizationID uuid.UUID, nonce []byte, signedAt time.Time) (domain.ActionRequestAdmission, error) {
	if !s.purgeRunning() {
		return 0, domain.ErrActionRequestNoncePurgeNotRunning
	}
	return s.repo.Admit(ctx, agentID, organizationID, nonce, signedAt)
}

// ActionRequestNoncePurgeResult is one purge run. It carries counts only.
type ActionRequestNoncePurgeResult struct {
	Deleted       int64
	Organizations int
	Duration      time.Duration
}

// Purge deletes, organization by organization, every nonce past
// expires_at + K. A run that fails part-way returns what it deleted so far
// with the error, and does not count as a completed purge.
func (s *ActionRequestNonceService) Purge(ctx context.Context) (ActionRequestNoncePurgeResult, error) {
	start := s.now()
	var result ActionRequestNoncePurgeResult

	orgIDs, err := s.repo.ListOrganizationIDs(ctx)
	if err != nil {
		result.Duration = s.now().Sub(start)
		return result, err
	}
	for _, orgID := range orgIDs {
		deleted, err := s.repo.PurgeOrganization(ctx, orgID)
		if err != nil {
			result.Duration = s.now().Sub(start)
			return result, err
		}
		result.Deleted += deleted
		result.Organizations++
	}

	finished := s.now()
	result.Duration = finished.Sub(start)
	s.mu.Lock()
	s.lastPurge = finished
	s.mu.Unlock()
	return result, nil
}
