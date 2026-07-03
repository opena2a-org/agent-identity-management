/**
 * AST-Aware Runtime Monitor
 *
 * Connects ARP runtime monitoring to the NanoMind Semantic Compiler.
 * Compares observed runtime events against the AST's declared behavioral
 * envelope. Flags events that fall outside what the AST predicted.
 *
 * This is real-time behavioral verification:
 *   AST declares: "this skill reads customer data"
 *   ARP observes: "this skill wrote to /etc/passwd"
 *   Monitor flags: capability exercise outside declared scope
 *
 * Architecture:
 *   AST (compiled at scan time) → ASTMonitor (loaded at runtime)
 *   ARP events → compared against AST declarations → drift detected
 */

import type { SecurityAST, Capability } from './security-ast-types';
import type { ARPEvent } from '../types.js';

// ============================================================================
// AST Monitor
// ============================================================================

export interface ASTDriftEvent {
  /** The runtime event that caused the drift */
  event: ARPEvent;
  /** Which AST claim was violated */
  violation: string;
  /** The declared capability (if any) that was exceeded */
  declaredCapability?: Capability;
  /** What was observed vs what was declared */
  declared: string;
  observed: string;
  /** Severity of the drift */
  severity: 'critical' | 'high' | 'medium' | 'low';
  /** Suggested response */
  action: 'alert' | 'throttle' | 'suspend' | 'kill';
}

/**
 * AST-Aware Runtime Monitor.
 *
 * Loads a SecurityAST at startup and compares every ARP event against it.
 * When an event falls outside the AST's declared behavioral envelope,
 * it generates an ASTDriftEvent.
 */
export class ASTMonitor {
  private ast: SecurityAST | null = null;
  private declaredCapNames: Set<string> = new Set();
  private hasOverrideResistance = false;
  private maxDeclaredRisk: string = 'low';

  /**
   * Load an AST for monitoring. Call this when the agent starts
   * or when the AST is recompiled after a skill update.
   */
  loadAST(ast: SecurityAST): void {
    this.ast = ast;
    this.declaredCapNames = new Set(ast.declaredCapabilities.map(c => c.name));

    // Pre-compute risk profile
    this.hasOverrideResistance = ast.declaredConstraints.some(c =>
      c.domain === 'behavioral_constraint' && c.enforceability > 0.5
    );

    const riskOrder = ['low', 'medium', 'high', 'critical'];
    for (const cap of ast.declaredCapabilities) {
      if (riskOrder.indexOf(cap.riskLevel) > riskOrder.indexOf(this.maxDeclaredRisk)) {
        this.maxDeclaredRisk = cap.riskLevel;
      }
    }
  }

  /**
   * Check a runtime event against the loaded AST.
   * Returns null if the event is within the declared envelope,
   * or an ASTDriftEvent if it falls outside.
   */
  checkEvent(event: ARPEvent): ASTDriftEvent | null {
    if (!this.ast) return null;

    // Check 1: Capability scope violation
    const capDrift = this.checkCapabilityScope(event);
    if (capDrift) return capDrift;

    // Check 2: Data access outside declared patterns
    const dataDrift = this.checkDataAccess(event);
    if (dataDrift) return dataDrift;

    // Check 3: External communication not declared
    const netDrift = this.checkNetworkAccess(event);
    if (netDrift) return netDrift;

    // Check 4: Override attempt detection
    const overrideDrift = this.checkOverrideAttempt(event);
    if (overrideDrift) return overrideDrift;

    return null;
  }

  // ============================================================================
  // Event Checks
  // ============================================================================

  private checkCapabilityScope(event: ARPEvent): ASTDriftEvent | null {
    if (!this.ast) return null;

    // Map ARP event source to capability name
    const capName = mapEventToCapability(event);
    if (!capName) return null;

    // Check if this capability was declared
    if (!this.declaredCapNames.has(capName)) {
      // Check for wildcard matches (db.* covers db.read, db.write)
      const hasWildcard = [...this.declaredCapNames].some(d => {
        const prefix = d.replace('.*', '');
        return capName.startsWith(prefix);
      });

      if (!hasWildcard) {
        return {
          event,
          violation: 'Undeclared capability exercised at runtime',
          declared: `Declared: ${[...this.declaredCapNames].join(', ')}`,
          observed: `Observed: ${capName}`,
          severity: event.severity === 'critical' ? 'critical' : 'high',
          action: 'throttle',
        };
      }
    }

    return null;
  }

  private checkDataAccess(event: ARPEvent): ASTDriftEvent | null {
    if (!this.ast) return null;

    // Check if event involves data access outside declared patterns
    const data = event.data as Record<string, unknown> | undefined;
    if (!data) return null;

    const accessedPath = (data.path || data.url || data.query || '') as string;
    if (!accessedPath) return null;

    // Check against declared data access patterns
    const declaredDataTypes = this.ast.declaredDataAccess.map(d => d.dataType);

    // Sensitive data access not in declarations
    const sensitivePatterns = ['password', 'credential', 'secret', 'token', 'ssn', 'medical'];
    const accessesSensitive = sensitivePatterns.some(p => accessedPath.toLowerCase().includes(p));
    const declaresSensitive = declaredDataTypes.some(d => ['credentials', 'pii', 'financial'].includes(d));

    if (accessesSensitive && !declaresSensitive) {
      return {
        event,
        violation: 'Sensitive data access not declared',
        declared: `Declared data types: ${declaredDataTypes.join(', ')}`,
        observed: `Accessed: ${accessedPath}`,
        severity: 'critical',
        action: 'suspend',
      };
    }

    return null;
  }

  private checkNetworkAccess(event: ARPEvent): ASTDriftEvent | null {
    if (!this.ast) return null;

    const data = event.data as Record<string, unknown> | undefined;
    if (!data) return null;

    const url = (data.url || data.endpoint || data.host || '') as string;
    if (!url) return null;

    // Check if network access was declared
    const hasNetworkCap = this.declaredCapNames.has('api.call') ||
      this.declaredCapNames.has('http.request') ||
      [...this.declaredCapNames].some(c => c.includes('external'));

    const isExternal = !url.includes('localhost') && !url.includes('127.0.0.1') && !url.includes('internal');

    if (isExternal && !hasNetworkCap) {
      return {
        event,
        violation: 'External network access not declared',
        declared: 'No external network capability declared',
        observed: `External request to: ${url}`,
        severity: 'high',
        action: 'alert',
      };
    }

    return null;
  }

  private checkOverrideAttempt(event: ARPEvent): ASTDriftEvent | null {
    if (!this.ast) return null;

    const description = (event.description || '').toLowerCase();
    const isOverrideAttempt = /ignore.*previous|override.*instruction|new.*task|bypass.*security/i.test(description);

    if (isOverrideAttempt && !this.hasOverrideResistance) {
      return {
        event,
        violation: 'Override attempt detected and no override resistance declared',
        declared: 'No override resistance in constraints',
        observed: `Override language detected: "${event.description?.slice(0, 80)}"`,
        severity: 'critical',
        action: 'kill',
      };
    }

    return null;
  }
}

// ============================================================================
// Event → Capability Mapping
// ============================================================================

function mapEventToCapability(event: ARPEvent): string | null {
  const source = event.source?.toLowerCase() ?? '';
  const desc = (event.description || '').toLowerCase();

  if (source.includes('network') || desc.includes('http') || desc.includes('fetch')) return 'api.call';
  if (source.includes('filesystem') || desc.includes('file')) return desc.includes('write') ? 'file.write' : 'file.read';
  if (source.includes('process') || desc.includes('exec') || desc.includes('spawn')) return 'process.execute';
  if (source.includes('memory') || desc.includes('memory')) return desc.includes('write') ? 'memory.write' : 'memory.read';
  if (desc.includes('database') || desc.includes('query')) return desc.includes('write') ? 'db.write' : 'db.read';

  return null;
}
