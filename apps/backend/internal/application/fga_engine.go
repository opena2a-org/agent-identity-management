package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// FGAEngine implements the 5-step Fine-Grained Authorization flow.
// Steps 1-4 must complete in < 10ms P99 (no external calls).
// Step 5 (NanoMind intent check) adds < 800ms for HIGH risk, async for MEDIUM, skip for LOW.
type FGAEngine struct {
	db              *sql.DB
	agentSvc        *AgentService
	daemonURL       string // NanoMind daemon URL
	registryASCURL  string // Registry ASC endpoint
	logger          *slog.Logger

	// OTel instruments. Captured in NewFGAEngine so they bind to the
	// real provider installed by telemetry.Init, not to the noop global
	// at package-init time.
	tracer       trace.Tracer
	decisions    metric.Int64Counter
	latency      metric.Int64Histogram
}

// FGARequest represents an authorization request to the FGA engine.
type FGARequest struct {
	AgentID    uuid.UUID `json:"agentId"`
	Capability string    `json:"capability"`
	Resource   string    `json:"resource,omitempty"`
	Attributes []string  `json:"attributes,omitempty"` // fields being accessed
	Action     string    `json:"action,omitempty"`     // read, write, delete, execute
	DataClass  string    `json:"dataClass,omitempty"`  // general, pii, financial, credential
}

// FGAResult represents the authorization decision.
type FGAResult struct {
	Allowed       bool     `json:"allowed"`
	Outcome       string   `json:"outcome"`       // ALLOW, DENY, DENY_INTENT, DENY_CONTEXT, DENY_CHAIN, DENY_ATTRIBUTE
	StepsTriggered []string `json:"stepsTriggered"` // which steps evaluated
	DeniedBy      string   `json:"deniedBy,omitempty"` // which step denied
	DeniedReason  string   `json:"deniedReason,omitempty"`
	LatencyMs     int64    `json:"latencyMs"`
	IntentCheck   *IntentCheckResult `json:"intentCheck,omitempty"`
}

// IntentCheckResult contains NanoMind daemon intent verification results.
type IntentCheckResult struct {
	IntentClass string  `json:"intentClass"`
	Confidence  float64 `json:"confidence"`
	Blocked     bool    `json:"blocked"`
	LatencyMs   int64   `json:"latencyMs"`
}

// FGAPolicy represents a stored FGA policy.
type FGAPolicy struct {
	ID                uuid.UUID       `json:"id"`
	AgentID           uuid.UUID       `json:"agentId"`
	Capability        string          `json:"capability"`
	AllowedAttributes []string        `json:"allowedAttributes"`
	DeniedAttributes  []string        `json:"deniedAttributes"`
	AllowedObjects    []string        `json:"allowedObjects"`
	AllowedActions    []string        `json:"allowedActions"`
	RowPredicate      json.RawMessage `json:"rowPredicate"`
	ContextRules      json.RawMessage `json:"contextRules"`
	ChainRules        json.RawMessage `json:"chainRules"`
	IntentCheck       json.RawMessage `json:"intentCheck"`
	RiskLevel         string          `json:"riskLevel"` // HIGH, MEDIUM, LOW
}

// ASCRiskSummary is the cached risk view from the Registry ASC.
type ASCRiskSummary struct {
	OverallRisk   string  `json:"overallRisk"`
	DriftScore    float64 `json:"driftScore"`
	ActiveAlerts  int     `json:"activeAlerts"`
	ATCTrustLevel int     `json:"atcTrustLevel"`
	ScanVerdict   string  `json:"scanVerdict"`
}

// NewFGAEngine creates a new FGA engine.
func NewFGAEngine(db *sql.DB, agentSvc *AgentService, logger *slog.Logger) *FGAEngine {
	if logger == nil {
		logger = slog.Default()
	}

	tracer := otel.Tracer("aim/fga")
	meter := otel.Meter("aim/fga")
	decisions, err := meter.Int64Counter(
		"fga.decisions_total",
		metric.WithDescription("FGA authorization decisions by outcome"),
	)
	if err != nil {
		logger.Warn("fga decisions counter init failed", "error", err)
	}
	latency, err := meter.Int64Histogram(
		"fga.latency_ms",
		metric.WithDescription("FGA total latency in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		logger.Warn("fga latency histogram init failed", "error", err)
	}

	return &FGAEngine{
		db:             db,
		agentSvc:       agentSvc,
		daemonURL:      "http://127.0.0.1:47200",
		registryASCURL: "https://api.oa2a.org",
		logger:         logger,
		tracer:         tracer,
		decisions:      decisions,
		latency:        latency,
	}
}

// SetDaemonURL configures the NanoMind daemon endpoint.
func (e *FGAEngine) SetDaemonURL(url string) {
	e.daemonURL = url
}

// SetRegistryASCURL configures the Registry ASC endpoint.
func (e *FGAEngine) SetRegistryASCURL(url string) {
	e.registryASCURL = url
}

// Authorize executes the 5-step FGA authorization flow.
//
// Emits one parent span (`fga.authorize`) plus one child span per FGA
// step actually triggered. Span attribute names are pinned to the AIM
// SemConv proposal (Slide 14 of the May 22 talk); see
// internal/telemetry/init.go for the canonical reference.
func (e *FGAEngine) Authorize(ctx context.Context, req *FGARequest) (*FGAResult, error) {
	e.ensureTelemetry()
	start := time.Now()
	result := &FGAResult{
		StepsTriggered: make([]string, 0, 5),
	}

	ctx, span := e.tracer.Start(ctx, "fga.authorize",
		trace.WithAttributes(
			attribute.String("agent.id", req.AgentID.String()),
			attribute.String("agent.capability", req.Capability),
		),
	)
	defer func() {
		span.SetAttributes(
			attribute.String("fga.outcome", result.Outcome),
			attribute.Int64("fga.latency_ms", result.LatencyMs),
			attribute.StringSlice("fga.steps_triggered", result.StepsTriggered),
		)
		if result.DeniedBy != "" {
			span.SetAttributes(attribute.String("fga.denied_by", result.DeniedBy))
		}
		if !result.Allowed {
			span.SetStatus(codes.Error, result.DeniedReason)
		}
		e.emitDecisionTelemetry(ctx, req, result)
		span.End()
	}()

	// Load FGA policy for this agent+capability
	policy, err := e.loadPolicy(ctx, req.AgentID, req.Capability)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to load FGA policy: %w", err)
	}

	// No policy = fall through to existing capability system (zero behavior change)
	if policy == nil {
		// Step 1 only: basic capability check via existing AIM
		result.StepsTriggered = append(result.StepsTriggered, "capability_check")
		stepCtx, stepSpan := e.tracer.Start(ctx, "fga.capability_check",
			trace.WithAttributes(
				attribute.String("fga.step", "capability_check"),
				attribute.String("agent.id", req.AgentID.String()),
				attribute.String("agent.capability", req.Capability),
			),
		)
		hasCapability, hcErr := e.agentSvc.HasCapability(stepCtx, req.AgentID, req.Capability, req.Resource)
		stepSpan.SetAttributes(attribute.Bool("fga.allowed", hasCapability))
		if hcErr != nil {
			stepSpan.RecordError(hcErr)
			stepSpan.End()
			return nil, hcErr
		}
		stepSpan.End()
		result.Allowed = hasCapability
		result.Outcome = "ALLOW"
		if !hasCapability {
			result.Outcome = "DENY"
			result.DeniedBy = "capability_check"
			result.DeniedReason = "Agent does not have the requested capability"
		}
		result.LatencyMs = time.Since(start).Milliseconds()
		return result, nil
	}

	// Step 1: Capability Check (< 1ms)
	result.StepsTriggered = append(result.StepsTriggered, "capability_check")
	stepCtx, stepSpan := e.tracer.Start(ctx, "fga.capability_check",
		trace.WithAttributes(
			attribute.String("fga.step", "capability_check"),
			attribute.String("agent.id", req.AgentID.String()),
			attribute.String("agent.capability", req.Capability),
		),
	)
	hasCapability, hcErr := e.agentSvc.HasCapability(stepCtx, req.AgentID, req.Capability, req.Resource)
	stepSpan.SetAttributes(attribute.Bool("fga.allowed", hasCapability))
	if hcErr != nil {
		stepSpan.RecordError(hcErr)
		stepSpan.End()
		return nil, hcErr
	}
	stepSpan.End()
	if !hasCapability {
		result.Outcome = "DENY"
		result.DeniedBy = "capability_check"
		result.DeniedReason = "Agent does not have the requested capability"
		result.LatencyMs = time.Since(start).Milliseconds()
		e.recordAttestation(ctx, req, result)
		return result, nil
	}

	// Step 2: Attribute Check (< 2ms)
	result.StepsTriggered = append(result.StepsTriggered, "attribute_check")
	_, attrSpan := e.tracer.Start(ctx, "fga.attribute_check",
		trace.WithAttributes(attribute.String("fga.step", "attribute_check")),
	)
	denied, reason := e.checkAttributes(req, policy)
	attrSpan.SetAttributes(attribute.Bool("fga.allowed", !denied))
	if denied {
		attrSpan.SetAttributes(attribute.String("fga.denied_reason", reason))
		attrSpan.SetStatus(codes.Error, reason)
	}
	attrSpan.End()
	if denied {
		result.Outcome = "DENY_ATTRIBUTE"
		result.DeniedBy = "attribute_check"
		result.DeniedReason = reason
		result.LatencyMs = time.Since(start).Milliseconds()
		e.recordAttestation(ctx, req, result)
		return result, nil
	}

	// Step 3: Context Check (< 5ms) - reads ASC from cache
	result.StepsTriggered = append(result.StepsTriggered, "context_check")
	ctxCtx, ctxSpan := e.tracer.Start(ctx, "fga.context_check",
		trace.WithAttributes(attribute.String("fga.step", "context_check")),
	)
	denied, reason = e.checkContext(ctxCtx, req, policy)
	ctxSpan.SetAttributes(attribute.Bool("fga.allowed", !denied))
	if denied {
		ctxSpan.SetAttributes(attribute.String("fga.denied_reason", reason))
		ctxSpan.SetStatus(codes.Error, reason)
	}
	ctxSpan.End()
	if denied {
		result.Outcome = "DENY_CONTEXT"
		result.DeniedBy = "context_check"
		result.DeniedReason = reason
		result.LatencyMs = time.Since(start).Milliseconds()
		e.recordAttestation(ctx, req, result)
		return result, nil
	}

	// Step 4: Chain Check (< 3ms) - rolling call history
	result.StepsTriggered = append(result.StepsTriggered, "chain_check")
	chainCtx, chainSpan := e.tracer.Start(ctx, "fga.chain_check",
		trace.WithAttributes(attribute.String("fga.step", "chain_check")),
	)
	denied, reason = e.checkChain(chainCtx, req, policy)
	chainSpan.SetAttributes(attribute.Bool("fga.allowed", !denied))
	if denied {
		chainSpan.SetAttributes(attribute.String("fga.denied_reason", reason))
		chainSpan.SetStatus(codes.Error, reason)
	}
	chainSpan.End()
	if denied {
		result.Outcome = "DENY_CHAIN"
		result.DeniedBy = "chain_check"
		result.DeniedReason = reason
		result.LatencyMs = time.Since(start).Milliseconds()
		e.recordAttestation(ctx, req, result)
		return result, nil
	}

	// Step 5: Intent Check (< 800ms for HIGH, async for MEDIUM, skip for LOW)
	if policy.RiskLevel == "HIGH" {
		result.StepsTriggered = append(result.StepsTriggered, "intent_check_sync")
		intentCtx, intentSpan := e.tracer.Start(ctx, "fga.intent_check_sync",
			trace.WithAttributes(attribute.String("fga.step", "intent_check_sync")),
		)
		intentResult := e.checkIntentSync(intentCtx, req)
		result.IntentCheck = intentResult
		if intentResult != nil {
			intentSpan.SetAttributes(
				attribute.String("fga.intent_class", intentResult.IntentClass),
				attribute.Float64("fga.intent_confidence", intentResult.Confidence),
				attribute.Bool("fga.allowed", !intentResult.Blocked),
			)
		}
		if intentResult != nil && intentResult.Blocked {
			intentSpan.SetStatus(codes.Error, "intent blocked")
			intentSpan.End()
			result.Outcome = "DENY_INTENT"
			result.DeniedBy = "intent_check"
			result.DeniedReason = fmt.Sprintf("Intent classified as %s (confidence %.2f)", intentResult.IntentClass, intentResult.Confidence)
			result.LatencyMs = time.Since(start).Milliseconds()
			e.recordAttestation(ctx, req, result)
			return result, nil
		}
		intentSpan.End()
	} else if policy.RiskLevel == "MEDIUM" {
		result.StepsTriggered = append(result.StepsTriggered, "intent_check_async")
		// Span only marks the dispatch; the async check itself is detached.
		_, asyncSpan := e.tracer.Start(ctx, "fga.intent_check_async",
			trace.WithAttributes(
				attribute.String("fga.step", "intent_check_async"),
				attribute.Bool("fga.dispatched", true),
			),
		)
		asyncSpan.End()
		go e.checkIntentAsync(context.Background(), req) // detached context for fire-and-forget
	}
	// LOW: skip intent check entirely

	// All checks passed
	result.Allowed = true
	result.Outcome = "ALLOW"
	result.LatencyMs = time.Since(start).Milliseconds()

	// Record tool call history
	e.recordToolCall(ctx, req, result)
	// Record Level 9 attestation
	e.recordAttestation(ctx, req, result)

	return result, nil
}

// ensureTelemetry guards against zero-value FGAEngine{} construction (used
// in helper unit tests) by lazily wiring instruments. NewFGAEngine populates
// these eagerly; this is purely a safety net.
func (e *FGAEngine) ensureTelemetry() {
	if e.logger == nil {
		e.logger = slog.Default()
	}
	if e.tracer == nil {
		e.tracer = otel.Tracer("aim/fga")
	}
	if e.decisions == nil {
		e.decisions, _ = otel.Meter("aim/fga").Int64Counter(
			"fga.decisions_total",
			metric.WithDescription("FGA authorization decisions by outcome"),
		)
	}
	if e.latency == nil {
		e.latency, _ = otel.Meter("aim/fga").Int64Histogram(
			"fga.latency_ms",
			metric.WithDescription("FGA total latency in milliseconds"),
			metric.WithUnit("ms"),
		)
	}
}

// emitDecisionTelemetry emits the per-decision metric + structured log line.
// Called from the deferred span finalizer in Authorize so it runs on every
// return path (allow, deny, error). The slog logger is bridged through
// telemetry, so trace.id and span.id auto-attach.
func (e *FGAEngine) emitDecisionTelemetry(ctx context.Context, req *FGARequest, result *FGAResult) {
	if e.decisions != nil {
		attrs := []attribute.KeyValue{attribute.String("fga.outcome", result.Outcome)}
		if result.DeniedBy != "" {
			attrs = append(attrs, attribute.String("fga.denied_by", result.DeniedBy))
		}
		e.decisions.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	if e.latency != nil {
		e.latency.Record(ctx, result.LatencyMs)
	}

	// Structured log emitted on every decision. WARN-level for denies so it
	// surfaces in default LogQL queries; INFO for allows.
	level := slog.LevelInfo
	if !result.Allowed {
		level = slog.LevelWarn
	}
	e.logger.LogAttrs(ctx, level, "fga.decision",
		slog.String("agent.id", req.AgentID.String()),
		slog.String("agent.capability", req.Capability),
		slog.String("fga.outcome", result.Outcome),
		slog.String("fga.denied_by", result.DeniedBy),
		slog.String("fga.denied_reason", result.DeniedReason),
		slog.Int64("fga.latency_ms", result.LatencyMs),
	)
}

// ============================================================================
// Step 2: Attribute Check
// ============================================================================

func (e *FGAEngine) checkAttributes(req *FGARequest, policy *FGAPolicy) (denied bool, reason string) {
	// Check denied attributes (deny list takes precedence)
	if len(policy.DeniedAttributes) > 0 && len(req.Attributes) > 0 {
		for _, reqAttr := range req.Attributes {
			for _, denied := range policy.DeniedAttributes {
				if reqAttr == denied || matchPattern(reqAttr, denied) {
					return true, fmt.Sprintf("Attribute '%s' is denied by policy", reqAttr)
				}
			}
		}
	}

	// Check allowed attributes (if specified, only these are permitted)
	if len(policy.AllowedAttributes) > 0 && len(req.Attributes) > 0 {
		for _, reqAttr := range req.Attributes {
			found := false
			for _, allowed := range policy.AllowedAttributes {
				if reqAttr == allowed || matchPattern(reqAttr, allowed) {
					found = true
					break
				}
			}
			if !found {
				return true, fmt.Sprintf("Attribute '%s' not in allowed list", reqAttr)
			}
		}
	}

	// Check allowed actions
	if len(policy.AllowedActions) > 0 && req.Action != "" {
		found := false
		for _, a := range policy.AllowedActions {
			if a == req.Action {
				found = true
				break
			}
		}
		if !found {
			return true, fmt.Sprintf("Action '%s' not in allowed list", req.Action)
		}
	}

	return false, ""
}

// ============================================================================
// Step 3: Context Check (reads ASC risk summary)
// ============================================================================

func (e *FGAEngine) checkContext(ctx context.Context, req *FGARequest, policy *FGAPolicy) (denied bool, reason string) {
	if len(policy.ContextRules) == 0 || string(policy.ContextRules) == "{}" {
		return false, ""
	}

	var rules struct {
		MaxDriftScore    *float64 `json:"maxDriftScore"`
		RequireScanClean *bool    `json:"requireScanClean"`
		MaxAlerts        *int     `json:"maxAlerts"`
		MinTrustLevel    *int     `json:"minTrustLevel"`
	}
	if err := json.Unmarshal(policy.ContextRules, &rules); err != nil {
		e.logger.Warn("failed to parse context rules", "error", err)
		return false, ""
	}

	// Fetch ASC risk summary from Registry cache
	summary := e.fetchASCRiskSummary(ctx, req.AgentID)
	if summary == nil {
		// No ASC data = cannot enforce context rules, allow (fail open for now)
		return false, ""
	}

	if rules.MaxDriftScore != nil && summary.DriftScore > *rules.MaxDriftScore {
		return true, fmt.Sprintf("Drift score %.2f exceeds maximum %.2f", summary.DriftScore, *rules.MaxDriftScore)
	}

	if rules.RequireScanClean != nil && *rules.RequireScanClean && summary.ScanVerdict != "clean" {
		return true, fmt.Sprintf("Scan verdict is '%s', clean required", summary.ScanVerdict)
	}

	if rules.MaxAlerts != nil && summary.ActiveAlerts > *rules.MaxAlerts {
		return true, fmt.Sprintf("Active alerts %d exceeds maximum %d", summary.ActiveAlerts, *rules.MaxAlerts)
	}

	if rules.MinTrustLevel != nil && summary.ATCTrustLevel < *rules.MinTrustLevel {
		return true, fmt.Sprintf("Trust level %d below minimum %d", summary.ATCTrustLevel, *rules.MinTrustLevel)
	}

	return false, ""
}

// ============================================================================
// Step 4: Chain Check (rolling tool call history)
// ============================================================================

func (e *FGAEngine) checkChain(ctx context.Context, req *FGARequest, policy *FGAPolicy) (denied bool, reason string) {
	if len(policy.ChainRules) == 0 || string(policy.ChainRules) == "{}" {
		return false, ""
	}

	var rules struct {
		MaxCallsPerHour    *int    `json:"maxCallsPerHour"`
		DenyAfterCapability *string `json:"denyAfterCapability"`
	}
	if err := json.Unmarshal(policy.ChainRules, &rules); err != nil {
		return false, ""
	}

	if e.db == nil {
		return false, ""
	}

	// Rate limit check
	if rules.MaxCallsPerHour != nil {
		var count int
		err := e.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM tool_call_history
			 WHERE agent_id = $1 AND capability = $2
			 AND called_at > NOW() - INTERVAL '1 hour'`,
			req.AgentID, req.Capability,
		).Scan(&count)
		if err == nil && count >= *rules.MaxCallsPerHour {
			return true, fmt.Sprintf("Rate limit exceeded: %d calls in last hour (max %d)", count, *rules.MaxCallsPerHour)
		}
	}

	// Deny after specific capability was used
	if rules.DenyAfterCapability != nil {
		var exists bool
		err := e.db.QueryRowContext(ctx,
			`SELECT EXISTS(
				SELECT 1 FROM tool_call_history
				WHERE agent_id = $1 AND capability = $2
				AND called_at > NOW() - INTERVAL '24 hours'
			)`,
			req.AgentID, *rules.DenyAfterCapability,
		).Scan(&exists)
		if err == nil && exists {
			return true, fmt.Sprintf("Access denied: agent used '%s' within 24h", *rules.DenyAfterCapability)
		}
	}

	return false, ""
}

// ============================================================================
// Step 5: Intent Check (NanoMind daemon)
// ============================================================================

func (e *FGAEngine) checkIntentSync(ctx context.Context, req *FGARequest) *IntentCheckResult {
	start := time.Now()

	client := &http.Client{Timeout: 800 * time.Millisecond}
	inferBody := map[string]interface{}{
		"intent": "INTENT_CHECK",
		"input":  req.Capability + " on " + req.Resource,
		"context": map[string]interface{}{
			"agentId": req.AgentID.String(),
		},
	}
	bodyBytes, _ := json.Marshal(inferBody)

	resp, err := client.Post(e.daemonURL+"/v1/infer", "application/json", strings.NewReader(string(bodyBytes)))
	if err != nil {
		e.logger.Debug("NanoMind daemon unavailable for intent check", "error", err)
		return &IntentCheckResult{
			IntentClass: "unknown",
			Confidence:  0,
			Blocked:     false, // fail open if daemon unavailable
			LatencyMs:   time.Since(start).Milliseconds(),
		}
	}
	defer resp.Body.Close()

	var inferResp struct {
		Result      string  `json:"result"`
		Confidence  float64 `json:"confidence"`
		AttackClass string  `json:"attackClass"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&inferResp); err != nil {
		return nil
	}

	blocked := inferResp.AttackClass != "" && inferResp.Confidence > 0.8

	return &IntentCheckResult{
		IntentClass: inferResp.AttackClass,
		Confidence:  inferResp.Confidence,
		Blocked:     blocked,
		LatencyMs:   time.Since(start).Milliseconds(),
	}
}

func (e *FGAEngine) checkIntentAsync(ctx context.Context, req *FGARequest) {
	// Fire-and-forget intent check for MEDIUM risk
	result := e.checkIntentSync(ctx, req)
	if result != nil && result.Blocked {
		e.logger.Warn("async intent check flagged suspicious activity",
			"agentId", req.AgentID,
			"capability", req.Capability,
			"intentClass", result.IntentClass,
		)
		// TODO: Write to ASC activeAlerts, create alert in AIM
	}
}

// ============================================================================
// Data Access
// ============================================================================

func (e *FGAEngine) loadPolicy(ctx context.Context, agentID uuid.UUID, capability string) (*FGAPolicy, error) {
	if e.db == nil {
		return nil, nil
	}

	var p FGAPolicy
	err := e.db.QueryRowContext(ctx,
		`SELECT id, agent_id, capability,
			allowed_attributes, denied_attributes, allowed_objects, allowed_actions,
			row_predicate, context_rules, chain_rules, intent_check, risk_level
		 FROM fga_policies WHERE agent_id = $1 AND capability = $2`,
		agentID, capability,
	).Scan(
		&p.ID, &p.AgentID, &p.Capability,
		pq_array(&p.AllowedAttributes), pq_array(&p.DeniedAttributes),
		pq_array(&p.AllowedObjects), pq_array(&p.AllowedActions),
		&p.RowPredicate, &p.ContextRules, &p.ChainRules, &p.IntentCheck, &p.RiskLevel,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// fetchASCRiskSummary reads the ASC risk summary.
// IMPORTANT: This must NOT make network calls in the hot path.
// In production, this reads from a local Redis cache populated by the Registry.
// If no local cache is available, returns nil (fail open -- context rules skip).
func (e *FGAEngine) fetchASCRiskSummary(ctx context.Context, agentID uuid.UUID) *ASCRiskSummary {
	// Read from local DB if available (the ASC table is replicated locally)
	if e.db != nil {
		var summary ASCRiskSummary
		err := e.db.QueryRowContext(ctx,
			`SELECT overall_risk, drift_score, active_alerts, atc_trust_level, scan_verdict
			 FROM agent_security_contexts WHERE agent_id = $1`, agentID,
		).Scan(&summary.OverallRisk, &summary.DriftScore, &summary.ActiveAlerts,
			&summary.ATCTrustLevel, &summary.ScanVerdict)
		if err == nil {
			return &summary
		}
	}

	// No local ASC data -- fail open, context rules will not trigger
	return nil
}

func (e *FGAEngine) recordToolCall(ctx context.Context, req *FGARequest, result *FGAResult) {
	if e.db == nil {
		return
	}
	_, err := e.db.ExecContext(ctx,
		`INSERT INTO tool_call_history (agent_id, capability, object_path, attributes_accessed, data_class, authorized, fga_steps_triggered)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		req.AgentID, req.Capability, req.Resource,
		pq_from_strings(req.Attributes), req.DataClass,
		result.Allowed, pq_from_strings(result.StepsTriggered),
	)
	if err != nil {
		e.logger.Error("failed to record tool call", "agentId", req.AgentID, "error", err)
	}
}

func (e *FGAEngine) recordAttestation(ctx context.Context, req *FGARequest, result *FGAResult) {
	if e.db == nil {
		return
	}
	_, err := e.db.ExecContext(ctx,
		`INSERT INTO access_attestations (agent_id, capability, resource_path, attributes_accessed, fga_outcome, fga_steps_triggered)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		req.AgentID, req.Capability, req.Resource,
		pq_from_strings(req.Attributes), result.Outcome,
		pq_from_strings(result.StepsTriggered),
	)
	if err != nil {
		e.logger.Error("failed to record access attestation", "agentId", req.AgentID, "error", err)
	}
}

// ============================================================================
// Helpers
// ============================================================================

// matchPattern matches a simple wildcard pattern (supports trailing *)
func matchPattern(value, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(value, prefix)
	}
	return value == pattern
}

// pq_array is a scanner for PostgreSQL text[] columns.
type pqStringArray struct {
	arr *[]string
}

func pq_array(arr *[]string) *pqStringArray {
	return &pqStringArray{arr: arr}
}

func (s *pqStringArray) Scan(src interface{}) error {
	if src == nil {
		*s.arr = []string{}
		return nil
	}
	switch v := src.(type) {
	case []byte:
		return s.parse(string(v))
	case string:
		return s.parse(v)
	default:
		return fmt.Errorf("unsupported type for pq_array: %T", src)
	}
}

func (s *pqStringArray) parse(str string) error {
	str = strings.TrimSpace(str)
	if str == "{}" || str == "" {
		*s.arr = []string{}
		return nil
	}
	str = strings.TrimPrefix(str, "{")
	str = strings.TrimSuffix(str, "}")
	parts := strings.Split(str, ",")
	result := make([]string, len(parts))
	for i, p := range parts {
		result[i] = strings.Trim(strings.TrimSpace(p), "\"")
	}
	*s.arr = result
	return nil
}

// pq_from_strings converts a string slice to a PostgreSQL array literal.
func pq_from_strings(arr []string) string {
	if len(arr) == 0 {
		return "{}"
	}
	parts := make([]string, len(arr))
	for i, s := range arr {
		parts[i] = "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\""
	}
	return "{" + strings.Join(parts, ",") + "}"
}
