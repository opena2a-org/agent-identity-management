package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/siem"
)

// siemFlushTimeout bounds the delivery of the queued SIEM events at shutdown.
// With the bounds in shutdown.go, the 2s metrics drain and the 5s telemetry
// flush, the worst case stays under the 30s Kubernetes grace period.
const siemFlushTimeout = 2 * time.Second

// startSIEMForwarding reads the SIEM_* variables and, when a SIEM is
// configured, forwards every audit log entry the repository writes to it. It
// returns nil when none is configured, and an error naming the variable when
// one is configured wrongly.
func startSIEMForwarding(auditLogs *repository.AuditLogRepository, getenv func(string) string) (*siem.Forwarder, error) {
	settings, err := siem.SettingsFromEnv(getenv)
	if err != nil {
		return nil, err
	}
	if len(settings.Adapters) == 0 {
		return nil, nil
	}

	forwarder := siem.NewForwarder(settings.Adapters, settings.Forwarder, nil)
	auditLogs.OnCreate(func(entry *domain.AuditLog) {
		forwarder.Enqueue(siem.EventFromAuditLog(entry))
	})

	names := make([]string, len(settings.Adapters))
	for i, adapter := range settings.Adapters {
		names[i] = adapter.Name()
	}
	minSeverity := settings.Forwarder.MinSeverity
	if minSeverity == "" {
		minSeverity = siem.SeverityInfo
	}
	log.Printf("siem: forwarding audit log entries at severity %s and above to %s", minSeverity, strings.Join(names, ", "))
	return forwarder, nil
}

// flushSIEMForwarding delivers the queued SIEM events within siemFlushTimeout
// and logs what was not delivered.
func flushSIEMForwarding(forwarder *siem.Forwarder) {
	ctx, cancel := context.WithTimeout(context.Background(), siemFlushTimeout)
	defer cancel()
	if err := forwarder.Close(ctx); err != nil {
		log.Printf("ERROR: siem: events still queued after %s were not delivered: %v", siemFlushTimeout, err)
	}
	stats := forwarder.Stats()
	log.Printf("siem: forwarding stopped: %d sent, %d failed, %d dropped", stats.Sent, stats.Failed, stats.Dropped)
}
