package application

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

const (
	// registryATCRevocationHold is how long a claimed request stays not due
	// while its sender sends it: past the request's timeout, so no other
	// sender starts it meanwhile, and a request whose server stopped during
	// it is due again once the hold has passed.
	registryATCRevocationHold = 2 * registryATCRevocationAttemptTimeout

	// registryATCRevocationPollInterval is how often the sender looks for
	// requests that have fallen due, besides when Request wakes it. It equals
	// the shortest retry delay, so a retry that falls due while the sender
	// waits is sent within one interval.
	registryATCRevocationPollInterval = 5 * time.Second

	// registryATCRevocationBatch bounds how many requests the sender claims,
	// and sends at once, in one round.
	registryATCRevocationBatch = 16

	// registryATCRevocationStoreTimeout bounds each statement on the store.
	registryATCRevocationStoreTimeout = 10 * time.Second
)

// RegistryATCRevocationStore keeps the credential revocation requests the
// Registry has not answered yet, where they outlive the server.
// repository.RegistryATCRevocationRepository implements it on the
// registry_atc_revocations table.
type RegistryATCRevocationStore interface {
	// Store stores a request for agentID that is due at once, replacing a
	// request already stored for the agent.
	Store(ctx context.Context, agentID uuid.UUID) error
	// Claim takes up to limit due requests, counts an attempt for each and
	// makes each due again only after hold.
	Claim(ctx context.Context, limit int, hold time.Duration) ([]domain.RegistryATCRevocation, error)
	// Delete deletes a claimed request unless it was stored again since.
	Delete(ctx context.Context, r domain.RegistryATCRevocation) error
	// Retry makes a claimed request due once after has passed, unless it was
	// stored again since.
	Retry(ctx context.Context, r domain.RegistryATCRevocation, after time.Duration) error
}

// RegistryATCRevocations sends the Registry the credential revocation requests
// AIM stores. Request stores a request before it is sent, and the sender
// deletes it once the Registry has revoked the credentials or the attempts
// have ended, so a request still waiting when a server stops is sent by the
// next server to start, or by another replica. The attempts follow the rules
// of the requests sent from memory: a retry after each of
// defaultRegistryATCRevocationRetryDelays while the answer is one a later
// request can change.
type RegistryATCRevocations struct {
	store   RegistryATCRevocationStore
	revoker RegistryATCRevoker
	delays  []time.Duration
	poll    time.Duration
	wake    chan struct{}

	// claimFailing is set while the store cannot be read, so that only the
	// first failure is logged. Only the sender's goroutine reads or writes it.
	claimFailing bool
}

// NewRegistryATCRevocations returns a sender for the requests in store, which
// it sends to revoker once Start has been called.
func NewRegistryATCRevocations(store RegistryATCRevocationStore, revoker RegistryATCRevoker) *RegistryATCRevocations {
	return &RegistryATCRevocations{
		store:   store,
		revoker: revoker,
		delays:  defaultRegistryATCRevocationRetryDelays,
		poll:    registryATCRevocationPollInterval,
		wake:    make(chan struct{}, 1),
	}
}

// Request stores a request asking the Registry to revoke the credentials it
// issued for agentID, due at once, and wakes the sender. The caller's
// cancellation does not end the statement that stores it. When the request
// cannot be stored it is sent from memory, as startRegistryATCRevocation sends
// it, and a server stop before the Registry has answered ends it.
func (q *RegistryATCRevocations) Request(ctx context.Context, agentID uuid.UUID) {
	storeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), registryATCRevocationStoreTimeout)
	defer cancel()
	if err := q.store.Store(storeCtx, agentID); err != nil {
		log.Printf("agent %s: the Registry credential revocation request was not stored and is sent from memory, where a server stop ends it: %v", agentID, err)
		startRegistryATCRevocation(q.revoker, agentID, q.delays)
		return
	}
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// Start sends the stored requests that are due on a goroutine of its own, and
// then each request as it falls due, until stop is called. stop ends the
// requests in flight and the wait between rounds, and returns once the
// goroutine has exited; a request it ends stays stored, due again once its
// hold has passed. Calling stop again does nothing.
func (q *RegistryATCRevocations) Start() (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		q.run(ctx)
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			<-done
		})
	}
}

// run sends the due requests, then waits for Request to wake it or for the
// poll interval to pass, until ctx ends.
func (q *RegistryATCRevocations) run(ctx context.Context) {
	ticker := time.NewTicker(q.poll)
	defer ticker.Stop()
	for {
		q.sendDue(ctx)
		select {
		case <-ctx.Done():
			return
		case <-q.wake:
		case <-ticker.C:
		}
	}
}

// sendDue claims the due requests, registryATCRevocationBatch at a time, and
// sends the requests of a batch at once.
func (q *RegistryATCRevocations) sendDue(ctx context.Context) {
	for ctx.Err() == nil {
		claimCtx, cancel := context.WithTimeout(ctx, registryATCRevocationStoreTimeout)
		due, err := q.store.Claim(claimCtx, registryATCRevocationBatch, registryATCRevocationHold)
		cancel()
		if err != nil {
			if ctx.Err() == nil && !q.claimFailing {
				log.Printf("Registry credential revocation requests could not be read, and are read again every %s: %v", q.poll, err)
				q.claimFailing = true
			}
			return
		}
		if q.claimFailing {
			log.Printf("Registry credential revocation requests can be read again")
			q.claimFailing = false
		}
		var wg sync.WaitGroup
		for _, r := range due {
			wg.Add(1)
			go func() {
				defer wg.Done()
				q.send(ctx, r)
			}()
		}
		wg.Wait()
		if len(due) < registryATCRevocationBatch {
			return
		}
	}
}

// send makes one request for r, then deletes r once the Registry has revoked
// the credentials or the attempts have ended, and otherwise makes it due after
// the next retry delay. A request that ctx ended, because the server is
// stopping, is left as claimed.
func (q *RegistryATCRevocations) send(ctx context.Context, r domain.RegistryATCRevocation) {
	agentID := r.AgentID.String()
	attemptCtx, cancel := context.WithTimeout(ctx, registryATCRevocationAttemptTimeout)
	err := q.revoker.RevokeATC(attemptCtx, agentID, registryATCRevocationReason)
	cancel()
	if err != nil && ctx.Err() != nil {
		return
	}

	delay, again := registryATCRevocationNext(agentID, max(r.Attempts, 1), err, q.delays)
	storeCtx, cancelStore := context.WithTimeout(context.Background(), registryATCRevocationStoreTimeout)
	defer cancelStore()
	if again {
		err = q.store.Retry(storeCtx, r, delay)
	} else {
		err = q.store.Delete(storeCtx, r)
	}
	if err != nil {
		log.Printf("agent %s: the Registry credential revocation request was not updated, and is due again %s after it was claimed: %v", agentID, registryATCRevocationHold, err)
	}
}
