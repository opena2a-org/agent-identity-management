import type {
  ARPEvent,
  ARPConfig,
  CapabilityManifest,
  ComplyOnViolation,
  IntelligenceConfig,
  L2AdapterKind,
  L2NotRunningReason,
  L2Status,
  LLMAdapter,
  LLMAssessment,
  LLMResponse,
  EventCategory,
  EventSeverity,
} from '../types';
import { BudgetController } from './budget';
import { createAdapter } from './adapters';
import { AnomalyDetector } from './anomaly';
import {
  DEFAULT_BEHAVIORAL_RISK_TIMEOUT_MS,
  type BehavioralRiskResult,
  type BehavioralRiskSource,
  type BehavioralRiskUnavailableCode,
} from './behavioral-risk';
import type { GuardAnomalySource, GuardAnomalyStatus } from './guard-anomaly';
import { sanitizeTerminalLine } from '../telemetry/signature/sanitize';

const SEVERITY_ORDER: EventSeverity[] = ['info', 'low', 'medium', 'high', 'critical'];

/**
 * Input tokens assumed for an L2 call's budget reservation (~200 for a micro-prompt).
 * It sizes only the estimate held while the call is in flight: the actual cost
 * replaces the estimate when the call returns.
 */
const ESTIMATED_PROMPT_TOKENS = 200;

/**
 * How long `checkL2Health()` waits for the adapter's health check before it reports the
 * destination as not answering. `AgentRuntimeProtection.start()` waits no longer than this.
 */
const L2_HEALTH_CHECK_TIMEOUT_MS = 2000;

/** Longest error message, in characters, that the line for a failed L2 call carries. */
const L2_FAILURE_MESSAGE_MAX_CHARS = 200;

/**
 * A URL that a shell hands to curl as written: a scheme, a host name or IPv4 address, and
 * a port. The host of a parsed URL can still hold `$(`, a backtick, `;` or `&`, and a
 * command printed with one of those would run more than curl, so none is printed for it.
 */
const PRINTABLE_URL = /^https?:\/\/[A-Za-z0-9.-]+(?::\d{1,5})?$/;

/**
 * Reason the comply gate denied a classified event. `prohibited` means the
 * class is on the explicit deny list; `unknown` means the class is not on
 * the allow list and parse-to-deny applies (per CR-001).
 */
export type ComplyDenyReason = 'prohibited' | 'unknown';

/**
 * Record written to `event.data.behavioralRisk` by the coordinator after
 * it queries the behavioral twin. The ok branch carries the fused score
 * and the band the coordinator used; the unavailable branch carries a
 * code so downstream audit can tell a healthy 0.0 from an IPC outage.
 *
 * The coordinator's unavailable policy is record-and-ignore: an IPC
 * failure is not evidence of threat, so severity is not mutated. A
 * deployment that wants a stricter policy can layer it on top by
 * reading `event.data.behavioralRisk` after analyze() returns.
 */
export type BehavioralRiskRecord =
  | {
      status: 'ok';
      score: number;
      action: 'allow' | 'alert' | 'throttle' | 'suspend' | 'kill';
      source: string;
      band: 'low' | 'elevated' | 'high' | 'critical';
    }
  | {
      status: 'unavailable';
      code: BehavioralRiskUnavailableCode;
    };

/**
 * Structured record of a comply gate decision. Written to `event.data.comply`
 * so downstream enforcement can route on the reason without re-checking the
 * manifest, and so tests can assert the outcome without scraping logs.
 */
export interface ComplyDecision {
  /** Classification label the gate evaluated. */
  classification: string;
  /** Reason for the deny: `prohibited` or `unknown` (parse-to-deny). */
  reason: ComplyDenyReason;
  /** Action routed from the manifest's `comply.on_violation` field. */
  action: ComplyOnViolation;
}

/**
 * The 3-Layer Intelligence Coordinator.
 *
 * L0: Rules (free)      — Pattern matching, allowlists, thresholds. Every event.
 * L1: Statistical (free) — Z-score anomaly detection, baseline deviation. L0 flags.
 * L2: LLM-Assisted ($)   — Micro-prompt to agent's LLM. Only L1 flags + budget reservation.
 *
 * 99% of events never reach L2. Cost is ~$0.01/day for most agents.
 */
/**
 * Whether L2 would actually run for a given intelligence config, and if not, why.
 *
 * This exists so that anything REPORTING L2's state settles it the same way the
 * coordinator does — by executing the same steps rather than restating them. The
 * CLI previously re-implemented the predicate as `enabled !== false`, and when the
 * default changed it went on printing "3-Layer (L0+L1+L2)" for an install where L2
 * was off. Replacing that with `enabled === true && adapter` fixed the common case
 * and still lied for `adapter: 'agent-proxy'`, which passes both checks and then
 * throws on construction.
 *
 * A status line that re-derives a predicate drifts from it. This one does not
 * re-derive: it tries to build the adapter, which is the only thing that settles it.
 */
export function describeL2Status(
  intelligence: IntelligenceConfig | undefined,
): { running: boolean; adapter?: string; reason?: string } {
  const resolved = resolveL2Adapter(intelligence);
  if (resolved.adapter !== null) return { running: true, adapter: resolved.name };
  return resolved.name === undefined
    ? { running: false, reason: resolved.reason }
    : { running: false, adapter: resolved.name, reason: resolved.reason };
}

/**
 * Either the adapter L2 will actually use, or the reason there is not one.
 *
 * `asked` carries whether the operator switched L2 on, so a caller that needs to
 * distinguish "withheld from someone who wanted it" from "never requested" does
 * not have to re-test `enabled` and become a second copy of the gate.
 *
 * `code` is the reason as one of a fixed set, for `getL2Status`; `reason` is the
 * sentence the startup warning and the CLI status line print, and can quote the
 * configured adapter name.
 */
type L2Resolution =
  | { adapter: LLMAdapter; name: string; asked: true; code?: undefined; reason?: undefined }
  | { adapter: null; name?: string; asked: boolean; code: L2NotRunningReason; reason: string };

/**
 * Settle L2's state ONCE, for every caller that needs to know it.
 *
 * The review of #457 found the gate written twice — here and in the coordinator's
 * constructor — down to the same reason string in both copies. Sharing the CALL to
 * `createAdapter` was not enough: what has to be shared is the DECISION, or the next
 * precondition added to one copy drifts the other, which is the defect the status line
 * was reported for in the first place.
 *
 * It returns the constructed adapter rather than a boolean so the constructor can keep
 * the object this already built. Asking `describeL2Status` and then constructing again
 * would build twice, which is a side effect on any adapter that opens something.
 */
function resolveL2Adapter(intelligence: IntelligenceConfig | undefined): L2Resolution {
  if (intelligence?.enabled !== true) {
    return { adapter: null, asked: false, code: 'not-enabled', reason: 'intelligence.enabled is not true' };
  }
  if (!intelligence.adapter) {
    return { adapter: null, asked: true, code: 'no-adapter', reason: "no 'intelligence.adapter' is configured" };
  }
  try {
    return {
      adapter: createAdapter(
        intelligence.adapter,
        intelligence.adapterConfig as Record<string, unknown> | undefined,
      ),
      name: intelligence.adapter,
      asked: true,
    };
  } catch (error) {
    return {
      adapter: null,
      name: intelligence.adapter,
      asked: true,
      code: 'adapter-failed',
      reason: error instanceof Error ? error.message : String(error),
    };
  }
}

/**
 * The kind of an adapter name, from a fixed set. A name outside the three adapters
 * the SDK builds is `other`, so a status built from it does not repeat the name.
 */
function adapterKindOf(name: unknown): L2AdapterKind {
  return name === 'anthropic' || name === 'openai' || name === 'ollama' ? name : 'other';
}

/**
 * An L2 decision as codes. When L2 runs, the kind is read from the adapter object
 * the decision built. The reason sentence, the configured name and `adapterConfig`
 * are left out.
 */
function l2StatusOf(resolved: L2Resolution): L2Status {
  if (resolved.adapter !== null) {
    return { running: true, adapter: adapterKindOf(resolved.adapter.name) };
  }
  return resolved.name === undefined
    ? { running: false, reason: resolved.code }
    : { running: false, adapter: adapterKindOf(resolved.name), reason: resolved.code };
}

export class IntelligenceCoordinator {
  private readonly config: IntelligenceConfig;
  private readonly agentContext: string;
  private readonly budget: BudgetController;
  private readonly anomaly: AnomalyDetector;
  private adapter: LLMAdapter | null = null;
  /** The L2 decision the constructor made, as codes. Read with getL2Status. */
  private readonly l2Status: L2Status;
  private batchQueue: ARPEvent[] = [];
  private batchTimer?: ReturnType<typeof setTimeout>;
  /** Handlers registered with onBatchAssessed. */
  private readonly batchAssessedHandlers: Array<(event: ARPEvent) => void | Promise<void>> = [];
  /**
   * Capability manifest used to enforce the `comply` envelope on classified
   * events. Optional: when null the comply gate is inert and the coordinator
   * runs the L0/L1/L2 stack unchanged.
   */
  private manifest: CapabilityManifest | null;
  /**
   * Optional behavioral risk source. When set, `analyze()` fuses the
   * signal returned by the source with the classification decision after
   * the comply gate and before L1 statistical. When null, the coordinator
   * runs the L0/L1/L2 stack unchanged. See `behavioral-risk.ts` for the
   * IPC contract and the two shipping implementations.
   */
  private behavioralRiskSource: BehavioralRiskSource | null;
  /**
   * Optional guard anomaly (classification drift) detector. When set,
   * `analyze()` records every classified event's label before the
   * comply gate runs, so drift observations include denied attempts.
   * Drift state is written to `event.data.guardAnomaly` in every
   * branch (baseline-pending, normal, drift); only the drift branch
   * raises severity. See `guard-anomaly.ts` for the bounded memory
   * contract and the chi-square drift model.
   */
  private guardAnomaly: GuardAnomalySource | null;
  /**
   * Whether the line for a failed L2 call was printed since the last call that answered.
   * A call that answers clears it, so each run of failed calls is reported once.
   */
  private l2FailureReported = false;
  /** The run of checkL2Health(), which every later call returns. */
  private l2HealthCheck: Promise<void> | null = null;

  constructor(
    arpConfig: ARPConfig,
    dataDir: string,
    manifest: CapabilityManifest | null = null,
    behavioralRiskSource: BehavioralRiskSource | null = null,
    guardAnomaly: GuardAnomalySource | null = null,
  ) {
    this.config = arpConfig.intelligence ?? {};
    this.budget = new BudgetController(dataDir, this.config);
    this.anomaly = new AnomalyDetector();
    this.manifest = manifest;
    this.behavioralRiskSource = behavioralRiskSource;
    this.guardAnomaly = guardAnomaly;

    // Build agent context for LLM prompts
    this.agentContext = buildAgentContext(arpConfig);

    // Initialize the LLM adapter only when L2 is switched on explicitly.
    //
    // Both halves of this condition are deliberate and fail closed:
    //
    //   enabled === true   an ABSENT `enabled` must mean off, not on. `loadConfig`
    //                      merges a parsed config shallowly over the defaults, so an
    //                      operator who sets only `intelligence.budgetUsd` replaces the
    //                      whole intelligence object and arrives here with `enabled`
    //                      undefined. Under the previous `!== false` test that path
    //                      built an adapter — narrowing the budget, a hardening action,
    //                      switched L2 on.
    //
    //   this.config.adapter  an ABSENT adapter must mean NO adapter. This used to fall
    //                      back to autoDetectAdapter(), which chose an outbound
    //                      destination from whichever model key happened to be exported
    //                      in the environment. That made an unrelated environment
    //                      variable decide where a security tool sends its observations,
    //                      which is not a choice the operator ever made.
    //
    // When L2 is not configured it does not run at all: not remotely, and not locally.
    // L0 and L1 are unaffected and still gate.
    const resolved = resolveL2Adapter(this.config);
    this.adapter = resolved.adapter;
    this.l2Status = l2StatusOf(resolved);
    if (resolved.adapter === null && resolved.asked) {
      // L2 withheld, L0+L1 still work. Say so once, naming the precondition and the
      // fix, rather than degrading silently — but only to an operator who ASKED for
      // L2. `enabled` absent or false requested nothing, so nothing is unavailable.
      // `asked` comes from the resolution rather than a second read of `enabled`:
      // re-testing it here would put a copy of the gate outside the census guard.
      this.reportL2Unavailable(resolved.reason);
    }
  }

  /**
   * Report, once, that L2 is not running and what would turn it on.
   *
   * A precondition a control needs and cannot find is reported with the missing
   * precondition named and a runnable fix — never a softer verdict and never silence.
   * L2 is an optional enrichment rather than a security check being skipped, so
   * withholding it is not a failure of the run: L0 and L1 still gate.
   */
  private reportL2Unavailable(reason: string): void {
    // eslint-disable-next-line no-console
    console.warn(
      `[ARP] L2 intelligence is not running (${reason}). L0 and L1 are unaffected and still gate. ` +
        "To enable it, set intelligence.enabled to true and choose intelligence.adapter explicitly: " +
        "'ollama' keeps inference local; 'anthropic' or 'openai' send event content to that vendor.",
    );
  }

  /**
   * Count a failed L2 call in the budget status and report it. The line is printed even
   * when the count cannot be saved.
   */
  private noteL2CallFailed(adapter: LLMAdapter, error: unknown): void {
    const message = describeL2Failure(error);
    try {
      this.budget.recordFailure(adapter.name, message);
    } finally {
      this.reportL2CallFailed(adapter, message);
    }
  }

  /**
   * Report that an L2 call failed, where it was sent and what to check, on the channel
   * reportL2Unavailable uses, once for each run of failed calls: a call that answers
   * clears `l2FailureReported`, and the next failure is reported again.
   *
   * The line is built from the adapter's name and endpoint and the error's code or
   * message, never from the prompt or the event.
   */
  private reportL2CallFailed(adapter: LLMAdapter, message: string): void {
    if (this.l2FailureReported) return;
    this.l2FailureReported = true;
    // eslint-disable-next-line no-console
    console.warn(
      `[ARP] L2 call failed (${adapter.name}, ${adapter.endpoint ?? 'endpoint not reported by the adapter'}): ` +
        `${message}. L0 and L1 are unaffected and still gate. ${l2FailureNextStep(adapter)}`,
    );
  }

  /**
   * Replace or clear the capability manifest used for the comply gate.
   * Exposed so the hosting runtime (e.g. the ARP proxy) can reload the
   * manifest without tearing down the coordinator.
   */
  setCapabilityManifest(manifest: CapabilityManifest | null): void {
    this.manifest = manifest;
  }

  /** The currently installed manifest, or null if none is set. */
  getCapabilityManifest(): CapabilityManifest | null {
    return this.manifest;
  }

  /**
   * Replace or clear the behavioral risk source. Exposed so a hosting
   * runtime can swap from an in-process source to a unix socket source
   * (or vice versa) without tearing down the coordinator.
   */
  setBehavioralRiskSource(source: BehavioralRiskSource | null): void {
    this.behavioralRiskSource = source;
  }

  /** The currently installed behavioral risk source, or null. */
  getBehavioralRiskSource(): BehavioralRiskSource | null {
    return this.behavioralRiskSource;
  }

  /**
   * Replace or clear the guard anomaly detector. Exposed so a
   * hosting runtime can hot-swap the drift model (for example to
   * install a refreshed baseline from the Registry) without tearing
   * down the coordinator.
   */
  setGuardAnomaly(guardAnomaly: GuardAnomalySource | null): void {
    this.guardAnomaly = guardAnomaly;
  }

  /** The currently installed guard anomaly detector, or null. */
  getGuardAnomaly(): GuardAnomalySource | null {
    return this.guardAnomaly;
  }

  /**
   * Analyze an event through the 3-layer stack.
   * Mutates the event's category, severity, and classifiedBy fields.
   * Returns the LLM assessment if L2 was invoked. An event queued for a batch returns
   * null here; when its batch is sent and answered, its assessment is written to the event
   * and the event is passed to each onBatchAssessed handler.
   */
  async analyze(event: ARPEvent): Promise<LLMAssessment | null> {
    // Guard anomaly (classification distribution drift) detection. Runs
    // FIRST so the drift window observes every classified event,
    // including those the comply gate will reject below. A burst of
    // attempted prohibited-class calls IS the drift signal we want to
    // catch, and short-circuiting before recording would blind the
    // detector to attacks. Pure local computation, no IO, bounded
    // memory; safe to run before any short-circuit. Record-and-lift
    // policy: the status is always written, severity is only raised
    // on the `drift` branch.
    if (this.guardAnomaly) {
      applyGuardAnomaly(event, this.guardAnomaly);
    }

    // L0-comply: capability manifest gate. Runs before L1 so a classified
    // event that violates the comply envelope short-circuits the stack and
    // does not pollute the anomaly baseline or burn L2 budget. Inert when no
    // manifest is installed, or when the event carries no classification.
    //
    // ENFORCEMENT IS OPT-IN. The gate only acts when the signed manifest sets
    // `comply.enforce === true`. With the default (absent/false) the gate is a
    // no-op even when `event.data.classification` is populated: classification
    // is written for DETECTION only (the sequence projector + telemetry tee)
    // and never enters the deny path. This is what keeps populating a
    // classification from silently becoming a hot-path deny control.
    if (this.manifest && this.manifest.comply.enforce === true) {
      if (applyComplyGate(event, this.manifest)) {
        return null;
      }
    }

    // Behavioral risk fusion. Runs AFTER the comply gate (a denied event
    // should not burn an IPC round-trip) and BEFORE L1 statistical (so
    // the behavioral signal can raise severity before L1 records its
    // baseline). Bounded by a timeout enforced inside the source; every
    // error path resolves to an `unavailable` result. Ignored when no
    // source is attached.
    if (this.behavioralRiskSource) {
      const timeoutMs = this.config.behavioralRiskTimeoutMs ?? DEFAULT_BEHAVIORAL_RISK_TIMEOUT_MS;
      const riskResult = await this.behavioralRiskSource.getBehavioralRiskSignal(
        event,
        timeoutMs,
      );
      applyBehavioralRisk(event, riskResult);
    }

    // L0: Already classified by the monitor that emitted it

    // L1: Statistical anomaly detection (free)
    const anomalyScore = this.anomaly.score(event);
    if (anomalyScore > 2.0) {
      // Z-score > 2 standard deviations — flag as anomaly
      if (event.category === 'normal') {
        event.category = 'anomaly';
      }
      if (severityIndex(event.severity) < severityIndex('medium')) {
        event.severity = 'medium';
      }
      event.classifiedBy = 'L1-statistical';
    }

    // L1 records this event for future baseline
    this.anomaly.record(event);

    // L2: LLM assessment (only if L1 flagged; the budget is reserved when the call is made)
    if (this.shouldEscalateToL2(event)) {
      if (this.config.enableBatching && event.severity !== 'critical') {
        return this.queueForBatch(event);
      }
      return this.assessWithLlm(event);
    }

    return null;
  }

  /**
   * Register a handler for each event that receives its L2 assessment after analyze()
   * returned: an event queued for a batch.
   *
   * analyze() returns null for such an event, before its batch is sent. When the batch is
   * answered and its assessments are applied, each handler is called with each event of
   * the batch, in the order the events were queued. A batch whose call the budget refuses,
   * or whose call throws, calls no handler. A handler that throws or rejects does not stop
   * the other handlers or events.
   */
  onBatchAssessed(handler: (event: ARPEvent) => void | Promise<void>): void {
    this.batchAssessedHandlers.push(handler);
  }

  /** Get budget status */
  getBudgetStatus() {
    return this.budget.getStatus();
  }

  /**
   * Whether L2 runs in this coordinator, as decided when it was built: the kind
   * of the adapter when it runs, the reason code when it does not. Holds codes
   * from fixed sets only. Returns a copy, so changing it changes nothing here.
   */
  getL2Status(): L2Status {
    return { ...this.l2Status };
  }

  /**
   * Ask the L2 adapter whether it can answer, so that a destination that cannot be reached
   * is reported before the first event needs it.
   *
   * Calls the adapter's `healthCheck()` and waits for it at most `timeoutMs`. A check that
   * fails, throws or does not answer in time prints the line a failed call prints, under
   * the same once-only rule, and is not counted in `failedCalls`, which counts calls. The
   * returned promise never rejects. With L2 off there is no adapter and nothing is asked.
   * The check runs once per coordinator: a later call returns the first call's promise.
   */
  checkL2Health(timeoutMs: number = L2_HEALTH_CHECK_TIMEOUT_MS): Promise<void> {
    if (!this.l2HealthCheck) {
      this.l2HealthCheck = this.runL2HealthCheck(timeoutMs);
    }
    return this.l2HealthCheck;
  }

  private async runL2HealthCheck(timeoutMs: number): Promise<void> {
    // The two gates shouldEscalateToL2 applies: the check reaches the destination as well.
    if (this.config.enabled !== true) return;
    const adapter = this.adapter;
    if (!adapter) return;

    let timer: ReturnType<typeof setTimeout> | undefined;
    const deadline = new Promise<string>((resolve) => {
      timer = setTimeout(() => resolve(`health check did not answer within ${timeoutMs} ms`), timeoutMs);
      // The check never keeps the process alive on its own.
      timer.unref?.();
    });
    const answer = Promise.resolve()
      .then(() => adapter.healthCheck())
      .then(
        (healthy): string | null => (healthy === true ? null : 'health check failed'),
        (error: unknown): string => `health check failed: ${describeL2Failure(error)}`,
      );

    try {
      const failure = await Promise.race([answer, deadline]);
      if (failure !== null) this.reportL2CallFailed(adapter, failure);
    } finally {
      clearTimeout(timer);
    }
  }

  /** Stop the coordinator (flush batches, clean up) */
  async stop(): Promise<void> {
    if (this.batchTimer) {
      clearTimeout(this.batchTimer);
      this.batchTimer = undefined;
    }
    // Flush remaining batch
    if (this.batchQueue.length > 0) {
      await this.flushBatch();
    }
  }

  private shouldEscalateToL2(event: ARPEvent): boolean {
    // L2 runs only when it was switched on explicitly. Absent means off, for the same
    // reason as in the constructor: a shallow-merged partial config arrives here with
    // `enabled` undefined, and `=== false` would let it through.
    if (this.config.enabled !== true) return false;
    if (!this.adapter) return false;

    // Only escalate if L1 flagged it
    if (event.category === 'normal') return false;

    // Minimum severity check
    const minSev = this.config.minSeverityForLlm ?? 'medium';
    if (severityIndex(event.severity) < severityIndex(minSev)) return false;

    // The budget is not reserved here: callWithinBudget reserves it when the call is
    // made, which for a batched event is when its batch is sent. queueForBatch checks
    // that the budget could pay for the batch before it queues the event.
    return true;
  }

  /**
   * Make one L2 call within a budget reservation.
   *
   * The reservation is taken before the call is awaited, so a call in flight counts
   * against `budgetUsd` and `maxCallsPerHour` when the next event asks for a call.
   * Returns null without calling the adapter when the reservation is refused.
   * A call that throws releases its reservation, is counted in `failedCalls` and is
   * reported once for its run of failures; an answered call settles it at the
   * cost of the tokens the adapter reported, or at the estimate when that cost cannot
   * be worked out.
   */
  private async callWithinBudget(
    prompt: string,
    maxTokens: number,
  ): Promise<{ response: LLMResponse; cost: number } | null> {
    const adapter = this.adapter;
    if (!adapter) return null;

    const reservation = this.budget.reserve(adapter.estimateCost(ESTIMATED_PROMPT_TOKENS, maxTokens));
    if (!reservation) return null;

    let response: LLMResponse;
    try {
      response = await adapter.assess(prompt, maxTokens);
    } catch (error) {
      // Counted and reported even when giving back the reservation cannot be saved.
      try {
        this.budget.release(reservation);
      } finally {
        this.noteL2CallFailed(adapter, error);
      }
      throw error;
    }
    // The destination answered: a failure after this one starts a new run and is reported.
    this.l2FailureReported = false;

    // Settle even when pricing the answer throws: record() charges the estimate for a
    // cost that is not a number, so the reservation does not stay open.
    let cost = Number.NaN;
    try {
      cost = adapter.estimateCost(response.inputTokens, response.outputTokens);
    } finally {
      this.budget.record(reservation, cost, response.inputTokens + response.outputTokens);
    }
    return { response, cost };
  }

  private async assessWithLlm(event: ARPEvent): Promise<LLMAssessment | null> {
    if (!this.adapter) return null;

    const prompt = buildMicroPrompt(this.agentContext, event);
    const maxTokens = this.config.maxTokensPerCall ?? 300;

    try {
      const answered = await this.callWithinBudget(prompt, maxTokens);
      if (!answered) return null;
      const { response, cost } = answered;

      const assessment = parseAssessment(response.content, response.inputTokens + response.outputTokens, cost);
      applyAssessment(event, assessment);
      return assessment;
    } catch {
      // LLM failure, counted and reported by callWithinBudget: fall back to L1 classification
      return null;
    }
  }

  private queueForBatch(event: ARPEvent): null {
    // Queue the event only while the budget could pay for its batch. The batch still
    // takes its reservation when it is sent. Without this check, an event analyzed while
    // the budget or the hour's calls are used up is held in the queue until the window
    // closes or stop() is called, and dropped if the batch's reservation is refused then.
    if (!this.canPayForBatch()) return null;

    this.batchQueue.push(event);

    if (!this.batchTimer) {
      const windowMs = this.config.batchWindowMs ?? 300000;
      this.batchTimer = setTimeout(() => {
        this.flushBatch().catch(() => {});
        this.batchTimer = undefined;
      }, windowMs);

      if (this.batchTimer.unref) {
        this.batchTimer.unref();
      }
    }

    return null; // Assessment will come later
  }

  /**
   * Whether the budget could pay now for a batch call, without reserving it. An
   * `estimateCost` that throws counts as no, as it does when the batch is sent.
   */
  private canPayForBatch(): boolean {
    const adapter = this.adapter;
    if (!adapter) return false;
    try {
      return this.budget.canReserve(adapter.estimateCost(ESTIMATED_PROMPT_TOKENS, this.batchMaxTokens()));
    } catch {
      return false;
    }
  }

  /** Output tokens for a batch call: twice `maxTokensPerCall`, at most 1000. */
  private batchMaxTokens(): number {
    return Math.min((this.config.maxTokensPerCall ?? 300) * 2, 1000);
  }

  private async flushBatch(): Promise<void> {
    if (this.batchQueue.length === 0 || !this.adapter) return;

    const events = this.batchQueue.splice(0);
    const prompt = buildBatchPrompt(this.agentContext, events);
    const maxTokens = this.batchMaxTokens();

    try {
      // One call for the whole batch, so one reservation, taken when the batch is sent.
      const answered = await this.callWithinBudget(prompt, maxTokens);
      if (!answered) return;
      const { response, cost } = answered;

      const assessments = parseBatchAssessments(
        response.content,
        events.length,
        response.inputTokens + response.outputTokens,
        cost,
      );
      events.forEach((event, i) => applyAssessment(event, assessments[i]));
    } catch {
      // Batch failure, counted and reported by callWithinBudget: events remain at L1 classification
      return;
    }

    // analyze() returned before these events had their assessments: report them now.
    for (const event of events) {
      for (const handler of this.batchAssessedHandlers) {
        try {
          await handler(event);
        } catch {
          // A handler's error does not stop the other handlers or events
        }
      }
    }
  }
}

// --- L2 failure reporting ---

/**
 * What a failed L2 call is reported with: the error's code, or its message reduced to text
 * that cannot carry the prompt or the answer.
 *
 * A system error is named by its code (ECONNREFUSED, ENOTFOUND, a TLS code). An HTTP error
 * is named by its status alone: the adapters put up to 200 characters of the response body
 * in the message, and a server can answer with what it was sent. An answer that is not JSON
 * is named as such, because the parser's message quotes the start of the answer. Any other
 * message is kept to one line of at most L2_FAILURE_MESSAGE_MAX_CHARS characters.
 */
function describeL2Failure(error: unknown): string {
  const code = (error as { code?: unknown } | null | undefined)?.code;
  if (typeof code === 'string' && /^[A-Z][A-Z0-9_]{1,63}$/.test(code)) return code;
  if (error instanceof SyntaxError) return 'the answer was not valid JSON';

  const message = error instanceof Error ? error.message : String(error);
  const status = /^HTTP \d{3}\b/.exec(message);
  if (status) return status[0];

  const line = sanitizeTerminalLine(message.trim(), L2_FAILURE_MESSAGE_MAX_CHARS).replace(/\.+$/, '');
  return line.length > 0 ? line : 'the call threw without a message';
}

/**
 * The step the line for a failed L2 call ends with: what to check for this adapter, and a
 * command that checks it when the endpoint can be printed as one.
 */
function l2FailureNextStep(adapter: LLMAdapter): string {
  const endpoint = adapter.endpoint !== undefined && PRINTABLE_URL.test(adapter.endpoint) ? adapter.endpoint : undefined;
  switch (adapter.name) {
    case 'ollama':
      return endpoint
        ? `Check that an Ollama server is listening there: curl -sS ${endpoint}/api/tags`
        : 'Check that intelligence.adapterConfig.host is the URL of the Ollama server.';
    case 'anthropic':
    case 'openai': {
      const keyVariable = adapter.name === 'anthropic' ? 'ANTHROPIC_API_KEY' : 'OPENAI_API_KEY';
      const check = `Check that ${keyVariable} or intelligence.adapterConfig.apiKey holds a valid key, and that this host reaches the endpoint`;
      return endpoint ? `${check}: curl -sSI ${endpoint}` : `${check}.`;
    }
    default:
      return 'Check the adapter settings in intelligence.adapterConfig.';
  }
}

// --- Prompt Engineering (the craft) ---

function buildAgentContext(config: ARPConfig): string {
  const parts = [`Agent: ${config.agentName}`];
  if (config.agentDescription) {
    parts.push(`Purpose: ${config.agentDescription}`);
  }
  if (config.declaredCapabilities?.length) {
    parts.push(`Capabilities: ${config.declaredCapabilities.join(', ')}`);
  }
  return parts.join('\n');
}

/** AI-layer monitor sources that use specialized prompts */
const AI_LAYER_SOURCES = new Set(['prompt', 'mcp-protocol', 'a2a-protocol']);

/**
 * Micro-prompt: ~200 tokens in, ~100 tokens out.
 * Designed for speed and cost efficiency. No chain-of-thought.
 * Uses specialized templates for AI-layer threats.
 */
/**
 * Fields of `event.data` that may travel to an L2 adapter verbatim.
 *
 * This is an ALLOWLIST, and that direction is the point: a denylist protects only
 * the fields someone remembered to name, so the next monitor that adds a field
 * carrying raw material leaks it by default. Everything not named here is replaced
 * with a non-reversible shape descriptor.
 *
 * What belongs here: identifiers and structural facts that let a model judge whether
 * a detection is a true positive. What does not: the matched material itself
 * (`matchedText`), the command line (`command`), file contents, or arguments.
 */
const L2_ALLOWED_DATA_FIELDS = new Set([
  'patternId',
  'patternCategory',
  'direction',
  'toolName',
  'from',
  'to',
  'protocol',
  'method',
  'eventType',
  'port',
  'pid',
]);

/**
 * Describe a value without disclosing it: length and character classes only.
 *
 * The model still learns that something matched and roughly what shape it had,
 * which is what the assessment needs. It does not learn the value. A control that
 * detects a leaked credential must not transmit the credential to a third party in
 * order to ask about it.
 */
function describeWithheld(value: unknown): string {
  const s = typeof value === 'string' ? value : (JSON.stringify(value) ?? String(value));

  // Bucket the length instead of reporting it exactly, and say nothing about the
  // character classes of a short value.
  //
  // An exact length plus a class set is not a summary of a small value, it IS the
  // value. `true` renders as 4 chars/lower and `false` as 5 chars/lower, which
  // reconstructs a boolean outright; a short enum from a known set is recovered the
  // same way, since the set is usually distinguishable by length alone. The
  // assessment never needed that precision — what it needs is whether the match was
  // short or long and roughly what it looked like.
  const n = s.length;
  const bucket =
    n === 0 ? 'empty'
      : n <= 8 ? 'up to 8 chars'
        : n <= 16 ? '9-16 chars'
          : n <= 32 ? '17-32 chars'
            : n <= 64 ? '33-64 chars'
              : n <= 128 ? '65-128 chars'
                : 'over 128 chars';

  // Below this, the class set is identifying rather than descriptive.
  if (n <= 8) return `<withheld: ${bucket}>`;

  const classes: string[] = [];
  if (/[a-z]/.test(s)) classes.push('lower');
  if (/[A-Z]/.test(s)) classes.push('upper');
  if (/[0-9]/.test(s)) classes.push('digit');
  if (/[^A-Za-z0-9]/.test(s)) classes.push('symbol');
  return `<withheld: ${bucket}${classes.length ? ', ' + classes.join('+') : ''}>`;
}

/**
 * The allowlisted view of `event.data`: allowlisted fields verbatim, every other
 * field replaced with its shape descriptor. Every channel that sends event data out
 * of the process builds from this view, so they all withhold the same keys.
 */
export function allowlistEventDataForL2(
  data: Record<string, unknown> | undefined,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  if (!data) return out;
  for (const [k, v] of Object.entries(data)) {
    out[k] = L2_ALLOWED_DATA_FIELDS.has(k) ? v : describeWithheld(v);
  }
  return out;
}

/** Render `event.data` for an L2 prompt, allowlisted and shape-described. */
function summarizeEventDataForL2(data: Record<string, unknown> | undefined): string {
  return JSON.stringify(allowlistEventDataForL2(data)).slice(0, 500);
}

function buildMicroPrompt(agentContext: string, event: ARPEvent): string {
  if (AI_LAYER_SOURCES.has(event.source)) {
    return buildAILayerPrompt(agentContext, event);
  }

  return `SECURITY ASSESSMENT — answer concisely.

${agentContext}

Event: ${event.source} monitor detected ${event.category} (${event.severity})
Detail: ${event.description}
Data: ${summarizeEventDataForL2(event.data as Record<string, unknown> | undefined)}

Is this behavior consistent with the agent's declared purpose and capabilities?
Respond in exactly this format:
CONSISTENT: YES or NO
CONFIDENCE: 0.0-1.0
REASONING: one sentence
ACTION: ALLOW, ALERT, PAUSE, or KILL`;
}

/**
 * Specialized prompt for AI-layer threats (prompt injection, MCP exploitation, A2A spoofing).
 * Provides the LLM with pattern match context for true/false positive assessment.
 */
function buildAILayerPrompt(agentContext: string, event: ARPEvent): string {
  const data = event.data;
  const patternId = data.patternId ?? 'unknown';
  const patternCategory = data.patternCategory ?? event.source;
  const matchedText = data.matchedText ?? '';
  const direction = data.direction ?? '';

  let contentContext = '';
  if (event.source === 'prompt') {
    contentContext = direction === 'input'
      ? `User/agent sent input that matched pattern ${patternId} (${patternCategory}).`
      : `LLM response matched output leak pattern ${patternId} (${patternCategory}).`;
  } else if (event.source === 'mcp-protocol') {
    const toolName = data.toolName ?? 'unknown';
    contentContext = `MCP tool call "${toolName}" has parameters matching exploitation pattern ${patternId}.`;
  } else if (event.source === 'a2a-protocol') {
    const from = data.from ?? 'unknown';
    const to = data.to ?? 'unknown';
    contentContext = `A2A message from "${from}" to "${to}" matched attack pattern ${patternId}.`;
  }

  return `AI-LAYER THREAT ASSESSMENT — determine if this is a true positive or false positive.

${agentContext}

Detection: ${event.description}
Context: ${contentContext}
Matched text: ${describeWithheld(matchedText)}
Pattern: ${patternId} — ${patternCategory}
Severity: ${event.severity}

Could this be legitimate usage given the agent's purpose, or is this a genuine attack attempt?
Consider: obfuscation techniques, benign edge cases, and the agent's declared capabilities.

Respond in exactly this format:
CONSISTENT: YES (benign/false positive) or NO (genuine threat)
CONFIDENCE: 0.0-1.0
REASONING: one sentence explaining why
ACTION: ALLOW, ALERT, PAUSE, or KILL`;
}

function buildBatchPrompt(agentContext: string, events: ARPEvent[]): string {
  const summary = events.map((e, i) =>
    `${i + 1}. [${e.source}] ${e.severity}: ${e.description}`
  ).join('\n');

  return `BATCH SECURITY ASSESSMENT — ${events.length} anomalies.

${agentContext}

Events:
${summary}

Is each event consistent with the agent's declared purpose and capabilities?
Answer every event, one line each, in exactly this format:
EVENT_NUM: ALLOW, ALERT, PAUSE, or KILL — one sentence reason`;
}

function parseAssessment(content: string, tokens: number, cost: number): LLMAssessment {
  const lines = content.trim().split('\n');
  // CR-001: Default to deny-safe values. If the LLM response can't be parsed,
  // we assume the worst (inconsistent behavior, recommend alert).
  let consistent = false;
  let confidence = 0.5;
  let reasoning = 'Assessment parse incomplete — defaulting to deny-safe';
  let recommendation: LLMAssessment['recommendation'] = 'alert';
  let hasAction = false;

  for (const line of lines) {
    const upper = line.toUpperCase();
    if (upper.startsWith('CONSISTENT:')) {
      consistent = upper.includes('YES');
    } else if (upper.startsWith('CONFIDENCE:')) {
      const val = parseFloat(line.split(':')[1]?.trim() ?? '0.5');
      confidence = isNaN(val) ? 0.5 : Math.max(0, Math.min(1, val));
    } else if (upper.startsWith('REASONING:')) {
      reasoning = line.split(':').slice(1).join(':').trim();
    } else if (upper.startsWith('ACTION:')) {
      hasAction = true;
      const action = line.split(':')[1]?.trim().toUpperCase();
      if (action === 'ALLOW') recommendation = 'allow';
      else if (action === 'ALERT') recommendation = 'alert';
      else if (action === 'PAUSE') recommendation = 'pause';
      else if (action === 'KILL') recommendation = 'kill';
      // CR-001: Unrecognized action string defaults to 'alert', not 'allow'
      else recommendation = 'alert';
    }
  }

  // CR-001: If no ACTION line was found, keep the deny-safe default ('alert')
  if (!hasAction) {
    recommendation = 'alert';
    consistent = false;
  }

  return { consistent, confidence, reasoning, recommendation, tokensUsed: tokens, estimatedCost: cost };
}

/** The actions a batch answer can give an event. */
const BATCH_ACTIONS = new Map<string, LLMAssessment['recommendation']>([
  ['ALLOW', 'allow'],
  ['ALERT', 'alert'],
  ['PAUSE', 'pause'],
  ['KILL', 'kill'],
]);

/** Recommendations from least to most strict. */
const RECOMMENDATION_ORDER: LLMAssessment['recommendation'][] = ['allow', 'alert', 'pause', 'kill'];

/**
 * One verdict line of a batch answer: `EVENT_NUM: ACTION — reason`. A leading list bullet,
 * `EVENT` or `#` before the number, and `.`, `)` or `-` in place of the colon are accepted;
 * markdown `*` and backticks are removed before a line is read. The action must be followed
 * by a space, `—`, `–`, `-`, `:`, `,`, `.`, `;` or the end of the line, so `ALLOW/KILL` is
 * not read as ALLOW.
 */
const BATCH_VERDICT_LINE = /^(?:[-•]\s*)?(?:EVENT[\s_]*)?#?(\d+)\s*[:.)-]\s*([A-Z]+)(?=$|[\s—–:,.;-])(.*)$/i;

/**
 * Read a batch answer into one assessment per event, in the order the events were numbered.
 *
 * An event the answer gives no recognized action gets the deny-safe default a single-event
 * answer without an ACTION line gets: `alert`, not consistent. An event given more than one
 * action keeps the strictest. A batch line carries no confidence, so each assessment
 * has the single-event parser's default of 0.5, and is consistent only when its action is
 * ALLOW. The call's tokens and cost are shared equally among the events, so the assessments
 * of a batch add up to what the call used and cost.
 */
function parseBatchAssessments(
  content: string,
  eventCount: number,
  tokens: number,
  cost: number,
): LLMAssessment[] {
  const verdicts = new Map<number, { recommendation: LLMAssessment['recommendation']; reasoning: string }>();

  for (const line of content.split('\n')) {
    const match = BATCH_VERDICT_LINE.exec(line.replace(/[*`]/g, '').trim());
    if (!match) continue;
    const index = Number(match[1]) - 1;
    const recommendation = BATCH_ACTIONS.get(match[2].toUpperCase());
    // A number outside the batch, or an action that is not one of the four, is not a
    // verdict; an event left without one gets the deny-safe default below.
    if (recommendation === undefined || index < 0 || index >= eventCount) continue;
    const held = verdicts.get(index);
    if (held && RECOMMENDATION_ORDER.indexOf(held.recommendation) >= RECOMMENDATION_ORDER.indexOf(recommendation)) {
      continue;
    }
    const reasoning = match[3].replace(/^[\s—–:,.;-]+/, '').trim();
    verdicts.set(index, { recommendation, reasoning: reasoning || 'No reason given in the batch answer' });
  }

  return Array.from({ length: eventCount }, (_, i): LLMAssessment => {
    const verdict = verdicts.get(i);
    return {
      consistent: verdict?.recommendation === 'allow',
      confidence: 0.5,
      reasoning: verdict?.reasoning ?? 'No action for this event in the batch answer — defaulting to deny-safe',
      recommendation: verdict?.recommendation ?? 'alert',
      tokensUsed: Math.floor(tokens / eventCount) + (i < tokens % eventCount ? 1 : 0),
      estimatedCost: cost / eventCount,
    };
  });
}

/**
 * Apply an L2 assessment to its event: the same step for a single-event call and for each
 * event of a batch.
 */
function applyAssessment(event: ARPEvent, assessment: LLMAssessment): void {
  event.llmAssessment = assessment;
  event.classifiedBy = 'L2-llm';

  // LLM can upgrade severity
  if (assessment.recommendation === 'kill') {
    event.severity = 'critical';
    event.category = 'threat';
  } else if (assessment.recommendation === 'pause') {
    event.severity = 'high';
    event.category = 'violation';
  }
}

function severityIndex(severity: EventSeverity): number {
  return SEVERITY_ORDER.indexOf(severity);
}

/**
 * Extract a classification label from an event's `data` bag. The coordinator
 * recognizes `data.classification` as the canonical field (what
 * NanoMind-Guard writes). Returns null when the event is not tagged with a
 * classifier output, in which case the comply gate is inert.
 */
function getEventClassification(event: ARPEvent): string | null {
  const raw = event.data?.classification;
  return typeof raw === 'string' && raw.length > 0 ? raw : null;
}

/**
 * Evaluate a classified event against the manifest's comply envelope and
 * mutate the event in place when a policy deny fires. Returns `true` when
 * the gate denied and the caller should short-circuit the rest of the
 * analyze() stack; `false` when the event passed through (either because it
 * carries no classification, or because the class is on the permitted list).
 *
 * Parse-to-deny ordering (CR-001):
 *   1. Classification on `prohibited_classes` denies first, even if it also
 *      appears on the permitted list. The deny list takes precedence.
 *   2. Classification on `permitted_classes` passes through.
 *   3. Any other classification is treated as unknown and denied via the
 *      same `on_violation` action. The default of "unknown is denied" is
 *      the parse-to-deny invariant documented in CR-001.
 */
function applyComplyGate(
  event: ARPEvent,
  manifest: CapabilityManifest,
): boolean {
  const classification = getEventClassification(event);
  if (classification === null) {
    // No classifier output on this event; comply gate cannot interpret it.
    // Leaving it alone is intentional: the manifest governs classified
    // traffic only, and we do not invent a class to deny on.
    return false;
  }

  const { permitted_classes, prohibited_classes, on_violation } = manifest.comply;

  let reason: ComplyDenyReason | null = null;
  if (prohibited_classes.includes(classification)) {
    reason = 'prohibited';
  } else if (!permitted_classes.includes(classification)) {
    reason = 'unknown';
  }

  if (reason === null) return false;

  const decision: ComplyDecision = {
    classification,
    reason,
    action: on_violation,
  };
  event.data.comply = decision;
  event.classifiedBy = 'L0-comply';

  applyComplyAction(event, on_violation);
  return true;
}

/**
 * Map a `ComplyOnViolation` action to concrete event category/severity
 * mutations. Severity is only ever raised, never lowered, so a previously
 * escalated event keeps its higher level. `log` is intentionally a no-op
 * beyond the `comply` decision record already written by the caller.
 */
function applyComplyAction(
  event: ARPEvent,
  action: ComplyOnViolation,
): void {
  switch (action) {
    case 'log':
      // Decision is recorded on event.data.comply; no category/severity
      // mutation so downstream rule engines can still classify.
      return;
    case 'alert':
      event.category = raiseCategory(event.category, 'violation');
      event.severity = raiseSeverity(event.severity, 'medium');
      return;
    case 'pause':
      event.category = raiseCategory(event.category, 'violation');
      event.severity = raiseSeverity(event.severity, 'high');
      return;
    case 'kill':
      event.category = 'threat';
      event.severity = 'critical';
      return;
    case 'deny':
      // Proxy-level denial in coordinator terms is equivalent to a critical
      // threat: the event must not be allowed to propagate normally.
      event.category = 'threat';
      event.severity = 'critical';
      return;
  }
}

const CATEGORY_ORDER: EventCategory[] = [
  'normal',
  'anomaly',
  'violation',
  'threat',
];

function raiseCategory(
  current: EventCategory,
  floor: EventCategory,
): EventCategory {
  return CATEGORY_ORDER.indexOf(current) >= CATEGORY_ORDER.indexOf(floor)
    ? current
    : floor;
}

function raiseSeverity(
  current: EventSeverity,
  floor: EventSeverity,
): EventSeverity {
  return severityIndex(current) >= severityIndex(floor) ? current : floor;
}

/**
 * Bands the behavioral risk score is sorted into, matched to severity
 * floors raised on the event when the band fires. The score ranges come
 * from RuntimeTwin's response table; they are deliberately mirrored
 * here rather than imported, so the coordinator's policy can drift
 * independently of the twin's internal action labels if needed.
 *
 *   >= 0.8 : critical   -> event.category >= threat, severity >= critical
 *   >= 0.6 : high       -> event.category >= violation, severity >= high
 *   >= 0.4 : elevated   -> event.category >= anomaly, severity >= medium
 *   <  0.4 : low        -> no mutation, record only
 */
function bandForScore(score: number): 'low' | 'elevated' | 'high' | 'critical' {
  if (score >= 0.8) return 'critical';
  if (score >= 0.6) return 'high';
  if (score >= 0.4) return 'elevated';
  return 'low';
}

/**
 * Fuse a `BehavioralRiskResult` into an ARPEvent. Writes a structured
 * record to `event.data.behavioralRisk` in both branches so downstream
 * audit can distinguish a healthy low score from an IPC outage. On the
 * `ok` branch, raises category and severity floors according to the
 * band. On the `unavailable` branch, records the code and leaves
 * category/severity untouched (record-and-ignore policy).
 *
 * `classifiedBy` is only rewritten to `L1-behavioral-risk` when the
 * event has not already been classified by a stricter layer; the L0
 * labels stay as-is so the origin of the decision is preserved.
 */
function applyBehavioralRisk(
  event: ARPEvent,
  result: BehavioralRiskResult,
): void {
  if (result.status === 'unavailable') {
    const record: BehavioralRiskRecord = {
      status: 'unavailable',
      code: result.code,
    };
    event.data.behavioralRisk = record;
    return;
  }
  const { score, action, source } = result.signal;
  const band = bandForScore(score);
  const record: BehavioralRiskRecord = {
    status: 'ok',
    score,
    action,
    source,
    band,
  };
  event.data.behavioralRisk = record;

  if (band === 'critical') {
    event.category = 'threat';
    event.severity = 'critical';
    if (event.classifiedBy === 'L0-rules') {
      event.classifiedBy = 'L1-behavioral-risk';
    }
    return;
  }
  if (band === 'high') {
    event.category = raiseCategory(event.category, 'violation');
    event.severity = raiseSeverity(event.severity, 'high');
    if (event.classifiedBy === 'L0-rules') {
      event.classifiedBy = 'L1-behavioral-risk';
    }
    return;
  }
  if (band === 'elevated') {
    event.category = raiseCategory(event.category, 'anomaly');
    event.severity = raiseSeverity(event.severity, 'medium');
    if (event.classifiedBy === 'L0-rules') {
      event.classifiedBy = 'L1-behavioral-risk';
    }
    return;
  }
  // Low band: record only, no mutation.
}

/**
 * Record a classified event's label into the guard anomaly detector
 * and fuse the resulting drift status into the event. Always writes
 * `event.data.guardAnomaly` with the current status. Only the
 * `drift` branch raises category to `anomaly` and severity to
 * `medium`; `baseline-pending` and `normal` are record-only.
 *
 * `classifiedBy` is only rewritten to `L1-guard-anomaly` when the
 * prior layer was `L0-rules`, preserving the origin of any stricter
 * decision already on the event. The comply gate runs AFTER this
 * helper, so on a denied event the comply gate will overwrite
 * `classifiedBy` to `L0-comply`, which correctly reflects the
 * strictest decider.
 *
 * Skips silently on events with no classification label: drift is
 * defined over classified traffic, and inventing a synthetic label
 * would corrupt the baseline.
 */
function applyGuardAnomaly(
  event: ARPEvent,
  detector: GuardAnomalySource,
): void {
  const classification = getEventClassification(event);
  if (classification === null) return;
  const status: GuardAnomalyStatus = detector.record(classification);
  event.data.guardAnomaly = status;
  if (status.status === 'drift') {
    event.category = raiseCategory(event.category, 'anomaly');
    event.severity = raiseSeverity(event.severity, 'medium');
    if (event.classifiedBy === 'L0-rules') {
      event.classifiedBy = 'L1-guard-anomaly';
    }
  }
}
