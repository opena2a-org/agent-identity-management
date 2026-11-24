package main

import (
	"context"
	"log"
	"time"
)

// apiShutdownTimeout bounds how long the API listener waits for in-flight
// requests after SIGTERM. Without a bound, one stalled request holds shutdown
// open indefinitely. Connections still open when it expires are cut when the
// process exits. Together with fgaDrainTimeout, webhookDrainTimeout and the 5s
// telemetry flush, the worst case is about 27s, which fits the 30s grace period
// Kubernetes allows by default before SIGKILL.
const apiShutdownTimeout = 10 * time.Second

// fgaDrainTimeout bounds the wait for the FGA async intent-check worker pool,
// so a hung NanoMind daemon cannot deadlock shutdown. The per-call HTTP
// timeout (800ms) caps individual workers regardless.
const fgaDrainTimeout = 10 * time.Second

// webhookDrainTimeout bounds the wait for the webhook event deliveries that
// stopping the webhook service ends, each recorded as abandoned. An attempt
// already sent to its endpoint is not waited for.
const webhookDrainTimeout = 2 * time.Second

// apiServer is the part of *fiber.App that shutdown uses.
type apiServer interface {
	ShutdownWithTimeout(timeout time.Duration) error
}

// asyncDrainer is the part of *application.FGAEngine that shutdown uses.
type asyncDrainer interface {
	Shutdown(ctx context.Context) error
}

// shutdownGracefully stops the API listener within apiTimeout, then drains
// the FGA worker pool within fgaTimeout. A failed or timed-out step is logged
// and never exits the process: main has to return normally so the FGA drain
// and its deferred cleanup (background jobs, Redis, database, telemetry) run.
func shutdownGracefully(api apiServer, apiTimeout time.Duration, fga asyncDrainer, fgaTimeout time.Duration) {
	if err := api.ShutdownWithTimeout(apiTimeout); err != nil {
		log.Printf("API server did not shut down cleanly within %s: %v", apiTimeout, err)
	}

	if fga != nil {
		ctx, cancel := context.WithTimeout(context.Background(), fgaTimeout)
		defer cancel()
		if err := fga.Shutdown(ctx); err != nil {
			log.Printf("FGA engine shutdown timed out: %v", err)
		}
	}
}

// verificationAuditDrainTimeout bounds the writing, at shutdown, of the audit
// entries of verification decisions still queued. Batches of up to 100 rows
// write in milliseconds; the bound is for a database that has stopped
// answering, when each entry left is written to the server log instead.
const verificationAuditDrainTimeout = 2 * time.Second

// auditQueueCloser is the part of *application.AuditQueue that shutdown uses.
type auditQueueCloser interface {
	Close(ctx context.Context) error
}

// closeVerificationAudit writes the queued audit entries of verification
// decisions within timeout. A failure is logged and never exits the process.
func closeVerificationAudit(queue auditQueueCloser, timeout time.Duration) {
	if queue == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := queue.Close(ctx); err != nil {
		log.Printf("Verification audit queue: %v", err)
	}
}
