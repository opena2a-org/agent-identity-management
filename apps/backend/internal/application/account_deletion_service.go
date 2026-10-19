package application

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// AccountDeletionStore records account deletion requests and deletes the
// accounts whose request is due. *repository.UserRepository implements it.
type AccountDeletionStore interface {
	// RequestDeletion deactivates the user's account and records the request
	// made at at; see the repository method for its refusals.
	RequestDeletion(ctx context.Context, userID uuid.UUID, at time.Time) (domain.AccountDeletion, error)
	// PurgeDeletionRequests deletes at most limit deactivated accounts whose
	// request was made at or before requestedBy, and returns them.
	PurgeDeletionRequests(ctx context.Context, requestedBy time.Time, limit int) ([]domain.AccountDeletion, error)
}

// accountDeletionAuditor writes the audit rows of the requests and of the
// purge. *AuditService implements it.
type accountDeletionAuditor interface {
	LogAction(ctx context.Context, orgID, userID uuid.UUID, action domain.AuditAction, resourceType string,
		resourceID uuid.UUID, ipAddress, userAgent string, metadata map[string]interface{}) error
}

// AccountDeletionService lets a user ask for the deletion of their own
// account, and deletes the account domain.AccountDeletionDelay later.
type AccountDeletionService struct {
	store AccountDeletionStore
	audit accountDeletionAuditor
	now   func() time.Time
}

// NewAccountDeletionService builds the service on the store and the audit
// service; a nil audit service writes no audit rows.
func NewAccountDeletionService(store AccountDeletionStore, audit *AuditService) *AccountDeletionService {
	s := &AccountDeletionService{store: store, now: time.Now}
	if audit != nil {
		s.audit = audit
	}
	return s
}

// Request deactivates the user's account and records the request for its
// deletion, then writes an account_deletion_requested audit row in the
// user's organization, whose metadata carries the time from which the purge
// deletes the account (deletionDueAt). A failed audit write is logged and does
// not undo the request. The store's refusals are returned as they are:
// domain.ErrLastActiveAdmin, domain.ErrAccountDeactivated and
// domain.ErrUserNotFound.
func (s *AccountDeletionService) Request(ctx context.Context, userID uuid.UUID, ipAddress, userAgent string) (domain.AccountDeletion, error) {
	deletion, err := s.store.RequestDeletion(ctx, userID, s.now().UTC())
	if err != nil {
		return domain.AccountDeletion{}, err
	}
	if s.audit != nil {
		meta := map[string]interface{}{"deletionDueAt": deletion.DueAt().UTC().Format(time.RFC3339)}
		if err := s.audit.LogAction(ctx, deletion.OrganizationID, userID, domain.AuditActionAccountDeletionRequested,
			"user", userID, ipAddress, userAgent, meta); err != nil {
			log.Printf("Account deletion: the audit row of user %s's request was not written: %v", userID, err)
		}
	}
	return deletion, nil
}

// Purge deletes the accounts whose deletion was requested
// domain.AccountDeletionDelay ago or earlier and that are still deactivated.
// It repeats the store's purge, domain.AccountDeletionPurgeBatch accounts at a
// time, until a batch deletes fewer, and writes for each deleted account an
// account_deleted audit row in its organization that names no user, with the
// deleted user's id as its resource and the request's time
// (deletionRequestedAt) in its metadata. It returns how many accounts it
// deleted, also when a batch fails.
func (s *AccountDeletionService) Purge(ctx context.Context) (int, error) {
	requestedBy := s.now().UTC().Add(-domain.AccountDeletionDelay)
	total := 0
	for {
		deleted, err := s.store.PurgeDeletionRequests(ctx, requestedBy, domain.AccountDeletionPurgeBatch)
		total += len(deleted)
		for _, d := range deleted {
			s.recordDeleted(ctx, d)
		}
		if err != nil {
			return total, err
		}
		if len(deleted) < domain.AccountDeletionPurgeBatch {
			return total, nil
		}
	}
}

func (s *AccountDeletionService) recordDeleted(ctx context.Context, d domain.AccountDeletion) {
	if s.audit == nil {
		return
	}
	meta := map[string]interface{}{"deletionRequestedAt": d.RequestedAt.UTC().Format(time.RFC3339)}
	if err := s.audit.LogAction(ctx, d.OrganizationID, uuid.Nil, domain.AuditActionAccountDeleted,
		"user", d.UserID, "", "", meta); err != nil {
		log.Printf("Account deletion: the audit row of the deletion of user %s was not written: %v", d.UserID, err)
	}
}
