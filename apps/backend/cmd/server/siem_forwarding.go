package main

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/metrics"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/siem"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
)

// siemFlushTimeout bounds the delivery of the queued SIEM events at shutdown.
// With the bounds in shutdown.go, the 2s metrics drain and the 5s telemetry
// flush, the worst case stays under the 30s Kubernetes grace period.
const siemFlushTimeout = 2 * time.Second

// startSIEMForwarding reads the SIEM_* variables and, when a SIEM is
// configured, forwards every audit log entry the repository writes to it and
// puts the forwarder's counters on /metrics (see registerSIEMSeries). It
// returns nil when none is configured, and an error naming the variable when
// one is configured wrongly.
func startSIEMForwarding(auditLogs *repository.AuditLogRepository, getenv func(string) string) (*siem.Forwarder, error) {
	settings, err := siem.SettingsFromEnv(getenv)
	if err != nil {
		return nil, err
	}
	if len(settings.Adapters) == 0 {
		_ = registerSIEMSeries(metrics.Registerer(), nil)
		return nil, nil
	}

	forwarder := siem.NewForwarder(settings.Adapters, settings.Forwarder, nil)
	if err := registerSIEMSeries(metrics.Registerer(), forwarder); err != nil {
		log.Printf("ERROR: siem: the SIEM counters are not on /metrics: %v", err)
	}
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

// siemSeriesMu serialises registerSIEMSeries.
var siemSeriesMu sync.Mutex

// registerSIEMSeries puts the counters of forwarder on reg, the registry
// /metrics serves: aim_siem_events_sent_total and
// aim_siem_events_failed_total for each adapter, and
// aim_siem_events_dropped_total. They replace the counters of a forwarder
// started before it in this process, and a nil forwarder leaves no SIEM
// series at all. main starts one forwarder at most, so only tests replace one.
func registerSIEMSeries(reg prometheus.Registerer, forwarder *siem.Forwarder) error {
	siemSeriesMu.Lock()
	defer siemSeriesMu.Unlock()
	reg.Unregister(siem.NewCollector(nil))
	if forwarder == nil {
		return nil
	}
	return reg.Register(siem.NewCollector(forwarder))
}

// observeSecurityDecisions forwards to the SIEM the security decisions that
// write no audit log entry: each password sign-in POST /api/v1/public/login
// or POST /api/v1/auth/login/local refuses, as aim.auth.signin_failed, and
// each FGA decision that does not allow the request, as aim.fga.denied. An
// allowed decision is not forwarded. It also forwards each alert the alert
// repository inserts, whichever service raised it, as aim.alert.created; a
// repeat counted on an open alert is not forwarded. main calls it once the
// handlers exist, when a SIEM is configured. Enqueue never blocks, so a slow
// or failing SIEM never delays an answer.
func observeSecurityDecisions(forwarder *siem.Forwarder, services *Services, h *Handlers) {
	alertsObserved := services.Alert != nil && services.Alert.OnAlertCreated(func(alert *domain.Alert) {
		forwarder.Enqueue(siem.EventFromAlert(alert))
	})
	if alertsObserved {
		log.Printf("siem: forwarding alerts (%s)", siem.EventAlertCreated)
	} else {
		log.Printf("ERROR: siem: alerts are not forwarded: the alert repository does not give out the alerts it inserts")
	}

	services.FGA.SetDecisionObserver(func(d application.FGADecision) {
		if d.Allowed {
			return
		}
		forwarder.Enqueue(siem.EventFromFGADenial(siem.FGADenial{
			Time:           d.Time,
			AgentID:        d.AgentID,
			OrganizationID: d.OrganizationID,
			Capability:     d.Capability,
			Outcome:        d.Outcome,
			DeniedBy:       d.DeniedBy,
			DeniedReason:   d.DeniedReason,
		}))
	})

	signInFailed := func(f handlers.SignInFailure) {
		forwarder.Enqueue(siem.EventFromSignInFailure(siem.SignInFailure{
			Time:           f.Time,
			Method:         f.Method,
			Reason:         f.Reason,
			Route:          f.Route,
			IPAddress:      f.IPAddress,
			UserAgent:      f.UserAgent,
			UserID:         f.UserID,
			OrganizationID: f.OrganizationID,
		}))
	}
	h.Auth.SetSignInFailureObserver(signInFailed)
	h.PublicRegistration.SetSignInFailureObserver(signInFailed)

	log.Printf("siem: forwarding failed sign-ins (%s) and FGA denials (%s)", siem.EventSignInFailed, siem.EventFGADenied)
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
