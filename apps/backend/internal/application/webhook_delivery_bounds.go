package application

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// Defaults for the delivery bounds a WebhookConfig leaves at zero.
const (
	defaultMaxRetriesCeiling     = 5
	defaultMaxPendingDeliveries  = 1000
	defaultMaxPendingPerWebhook  = 100
	defaultMaxAttemptsPerWebhook = 2
	defaultMaxAttemptsInFlight   = 5
)

// positiveOr returns v, or def when v is not positive, so a config that leaves
// a bound at zero keeps the default bound instead of none.
func positiveOr(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

// deliveryBounds holds the count of the event deliveries a WebhookService has
// in hand, so neither a burst of events nor a slow or failing endpoint starts
// an unbounded number of deliveries or connections.
//
// A delivery is pending from the moment admit takes it until done gives it
// back: while it sends an attempt, waits for an attempt slot, or waits to
// retry. An attempt in flight holds one of the attempt slots across webhooks
// and one of its webhook's slots; the wait before a retry holds neither.
type deliveryBounds struct {
	maxPending            int
	maxPendingPerWebhook  int
	maxAttemptsPerWebhook int

	// attempts has one slot per attempt that may be in flight across webhooks.
	attempts chan struct{}

	mu               sync.Mutex
	pending          int
	pendingByWebhook map[uuid.UUID]int
	webhookAttempts  map[uuid.UUID]chan struct{}
	stopping         bool
	drained          chan struct{}
}

func newDeliveryBounds(config *WebhookConfig) *deliveryBounds {
	return &deliveryBounds{
		maxPending:            positiveOr(config.MaxPendingDeliveries, defaultMaxPendingDeliveries),
		maxPendingPerWebhook:  positiveOr(config.MaxPendingPerWebhook, defaultMaxPendingPerWebhook),
		maxAttemptsPerWebhook: positiveOr(config.MaxAttemptsPerWebhook, defaultMaxAttemptsPerWebhook),
		attempts:              make(chan struct{}, positiveOr(config.DeliveryWorkerCount, defaultMaxAttemptsInFlight)),
		pendingByWebhook:      make(map[uuid.UUID]int),
		webhookAttempts:       make(map[uuid.UUID]chan struct{}),
		drained:               make(chan struct{}),
	}
}

// admit takes a pending place for a delivery to webhookID. When it refuses, it
// takes nothing and returns the reason the delivery is not sent.
func (b *deliveryBounds) admit(webhookID uuid.UUID) (bool, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch {
	case b.stopping:
		return false, "not sent: the server is stopping"
	case b.pending >= b.maxPending:
		return false, fmt.Sprintf("not sent: pending event deliveries are at their limit of %d", b.maxPending)
	case b.pendingByWebhook[webhookID] >= b.maxPendingPerWebhook:
		return false, fmt.Sprintf("not sent: this webhook's pending deliveries are at their limit of %d", b.maxPendingPerWebhook)
	}
	b.pending++
	b.pendingByWebhook[webhookID]++
	return true, ""
}

// done gives back the pending place admit took for webhookID.
func (b *deliveryBounds) done(webhookID uuid.UUID) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pending--
	if b.pendingByWebhook[webhookID]--; b.pendingByWebhook[webhookID] <= 0 {
		delete(b.pendingByWebhook, webhookID)
		delete(b.webhookAttempts, webhookID)
	}
	if b.stopping && b.pending == 0 {
		close(b.drained)
	}
}

// acquireAttempt waits for a slot to send one attempt to webhookID and returns
// the function that gives it back. It returns ok false, holding nothing, when
// stop is closed before a slot is free.
func (b *deliveryBounds) acquireAttempt(webhookID uuid.UUID, stop <-chan struct{}) (release func(), ok bool) {
	select {
	case <-stop:
		return nil, false
	default:
	}

	b.mu.Lock()
	perWebhook, found := b.webhookAttempts[webhookID]
	if !found {
		perWebhook = make(chan struct{}, b.maxAttemptsPerWebhook)
		b.webhookAttempts[webhookID] = perWebhook
	}
	b.mu.Unlock()

	select {
	case perWebhook <- struct{}{}:
	case <-stop:
		return nil, false
	}
	select {
	case b.attempts <- struct{}{}:
	case <-stop:
		<-perWebhook
		return nil, false
	}
	return func() {
		<-b.attempts
		<-perWebhook
	}, true
}

// stop refuses every later delivery and returns a channel that is closed once
// no delivery is pending.
func (b *deliveryBounds) stop() <-chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.stopping {
		b.stopping = true
		if b.pending == 0 {
			close(b.drained)
		}
	}
	return b.drained
}
