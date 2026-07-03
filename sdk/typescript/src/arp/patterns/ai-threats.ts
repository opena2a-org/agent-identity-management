// AI-layer threat detection patterns
// L0 regex-based detection for known attack payloads

export interface ThreatPattern {
  /** Pattern identifier (e.g., PI-001) */
  id: string;
  /** Attack category */
  category: 'prompt-injection' | 'jailbreak' | 'data-exfiltration' | 'output-leak' | 'context-manipulation' | 'mcp-exploitation' | 'a2a-attack';
  /** Human-readable description */
  description: string;
  /** Regex pattern to match */
  pattern: RegExp;
  /** Severity when matched */
  severity: 'medium' | 'high' | 'critical';
}

// --- Prompt Injection Patterns ---

const promptInjection: ThreatPattern[] = [
  {
    id: 'PI-001',
    category: 'prompt-injection',
    description: 'Instruction override attempt — direct override of system instructions',
    pattern: /(?:ignore|disregard|forget|override|bypass)\s+(?:all\s+)?(?:your\s+|the\s+|my\s+)?(?:previous\s+|prior\s+|above\s+|earlier\s+|system\s+|original\s+)?(?:instructions?|prompts?|rules?|directives?|guidelines?|constraints?)/i,
    severity: 'critical',
  },
  {
    id: 'PI-002',
    category: 'prompt-injection',
    description: 'Delimiter escape — attempts to break out of user message context',
    pattern: /(?:<\/?(?:system|assistant|tool|function|instruction|context|internal)>|={3,}|---\s*(?:system|end|begin)|```\s*(?:system|instruction)|\[(?:SYSTEM|INST|INSTRUCTION)\]|<\|(?:im_start|im_end|system|endoftext)\|>)/i,
    severity: 'high',
  },
  {
    id: 'PI-003',
    category: 'prompt-injection',
    description: 'Tag injection — embedding fake system/assistant turns',
    pattern: /(?:^|\n)\s*(?:system\s*:|assistant\s*:|AI\s*:|<\|(?:system|assistant)\|>)\s*.{10,}/im,
    severity: 'high',
  },
];

// --- Jailbreak Patterns ---

const jailbreak: ThreatPattern[] = [
  {
    id: 'JB-001',
    category: 'jailbreak',
    description: 'DAN mode — "Do Anything Now" jailbreak attempt',
    pattern: /(?:\b(?:DAN|D\.A\.N|do\s+anything\s+now)\b.*(?:mode|persona|act|pretend|enable|activate|jailbr[eo]ak)|(?:act|pretend|behave|roleplay|play)\s+(?:as|like)\s+(?:a\s+)?(?:DAN|D\.A\.N)\b|do\s+anything\s+now)/i,
    severity: 'critical',
  },
  {
    id: 'JB-002',
    category: 'jailbreak',
    description: 'Roleplay bypass — using fictional framing to bypass safety',
    pattern: /(?:(?:pretend|imagine|act|roleplay|play|behave)\s+(?:you\s+are|as\s+(?:if\s+)?(?:you\s+(?:are|were)\s+)?|like|that\s+you(?:'re|\s+are))\s*(?:a\s+)?(?:an?\s+)?(?:evil|malicious|unfiltered|uncensored|unrestricted|unaligned|hacker|criminal|without\s+(?:restrictions|rules|limits|safety|guidelines|filters)))/i,
    severity: 'high',
  },
  {
    id: 'JB-003',
    category: 'jailbreak',
    description: 'Hypothetical bypass — using hypothetical framing to extract harmful content',
    pattern: /(?:hypothetically|theoretically|in\s+theory|for\s+(?:educational|research|academic|fictional)\s+purposes?|just\s+for\s+fun|in\s+a\s+(?:fictional|hypothetical)\s+(?:world|scenario|universe)).*(?:how\s+(?:would|could|to)|explain|describe|show|give\s+me|write)\s+.*(?:hack|exploit|attack|inject|bypass|break|steal|exfiltrate)/i,
    severity: 'medium',
  },
];

// --- Data Exfiltration Patterns (in user input) ---

const dataExfiltration: ThreatPattern[] = [
  {
    id: 'DE-001',
    category: 'data-exfiltration',
    description: 'System prompt extraction — attempting to extract system instructions',
    pattern: /(?:(?:repeat|show|reveal|display|print|output|tell|what\s+(?:is|are)|give|list|dump|echo|copy)\s+(?:me\s+)?(?:your\s+|the\s+)?(?:full\s+|complete\s+|entire\s+|exact\s+|original\s+|initial\s+)?(?:system\s+)?(?:prompt|instructions?|rules?|guidelines?|initial\s+(?:prompt|message)|hidden\s+(?:prompt|instructions?)|(?:pre|pre-)?prompt))/i,
    severity: 'high',
  },
  {
    id: 'DE-002',
    category: 'data-exfiltration',
    description: 'Credential extraction — attempting to extract secrets or API keys',
    pattern: /(?:(?:what|show|reveal|give|tell|display|print|dump|list|output|extract|expose|leak)\s+(?:me\s+)?(?:are\s+)?(?:your\s+|the\s+|any\s+)?(?:api\s+keys?|secrets?|credentials?|passwords?|tokens?|auth(?:entication)?|(?:private|secret)\s+keys?|(?:access|bearer)\s+tokens?|environment\s+variables?|env\s+vars?|\.env))/i,
    severity: 'critical',
  },
  {
    id: 'DE-003',
    category: 'data-exfiltration',
    description: 'PII extraction — attempting to extract personal data',
    pattern: /(?:(?:list|show|give|reveal|dump|extract|tell)\s+(?:me\s+)?(?:all\s+)?(?:the\s+)?(?:users?|customers?|employees?|people|names?|emails?|addresses?|phone\s+numbers?|ssn|social\s+security|credit\s+cards?|payment|personal\s+(?:data|information|details)))/i,
    severity: 'high',
  },
];

// --- Output Leak Patterns (in LLM responses) ---

const outputLeak: ThreatPattern[] = [
  {
    id: 'OL-001',
    category: 'output-leak',
    description: 'API key in output — LLM response contains API key pattern',
    pattern: /(?:sk-[a-zA-Z0-9\-_]{20,}|(?:AKIA|ASIA)[A-Z0-9]{16}|ghp_[a-zA-Z0-9]{36}|glpat-[a-zA-Z0-9\-_]{20,}|xox[bpas]-[a-zA-Z0-9\-]{10,}|(?:eyJ[a-zA-Z0-9_-]{20,}\.){2}[a-zA-Z0-9_-]{20,})/,
    severity: 'critical',
  },
  {
    id: 'OL-002',
    category: 'output-leak',
    description: 'PII in output — LLM response contains PII patterns',
    pattern: /(?:\b\d{3}-\d{2}-\d{4}\b|\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b|\b[A-Z]{2}\d{6,9}\b)/,
    severity: 'high',
  },
  {
    id: 'OL-003',
    category: 'output-leak',
    description: 'System prompt leak — LLM response appears to contain system instructions',
    pattern: /(?:(?:my|the)\s+system\s+(?:prompt|instructions?)\s+(?:is|are|says?)|(?:here\s+(?:is|are)\s+)?(?:my|the)\s+(?:original|initial|full|hidden|secret)\s+(?:instructions?|prompt|guidelines?))/i,
    severity: 'high',
  },
];

// --- Context Manipulation Patterns ---

const contextManipulation: ThreatPattern[] = [
  {
    id: 'CM-001',
    category: 'context-manipulation',
    description: 'False memory injection — injecting false context or history',
    pattern: /(?:(?:remember|recall)\s+(?:that\s+)?(?:you|we)\s+(?:agreed|decided|confirmed|said|mentioned|established)|you\s+(?:said|told|mentioned|agreed|promised|confirmed)\s+(?:me\s+)?(?:that\s+)?(?:you|we|I)\s+(?:would|should|can|could|will|must|have|are|am))/i,
    severity: 'medium',
  },
  {
    id: 'CM-002',
    category: 'context-manipulation',
    description: 'Context reset — attempting to reset or clear conversation context',
    pattern: /(?:(?:start|begin)\s+(?:a\s+)?(?:new|fresh)\s+(?:conversation|session|context|chat)|(?:clear|reset|wipe|flush|forget)\s+(?:your\s+)?(?:context|memory|history|conversation|session|chat)|new\s+session\s*(?::|started|begins?))/i,
    severity: 'medium',
  },
];

// --- MCP Exploitation Patterns ---

const mcpExploitation: ThreatPattern[] = [
  {
    id: 'MCP-001',
    category: 'mcp-exploitation',
    description: 'Path traversal in MCP tool parameter',
    pattern: /(?:\.\.\/|\.\.\\|%2e%2e(?:%2f|%5c)|\.\.%252f)/i,
    severity: 'critical',
  },
  {
    id: 'MCP-002',
    category: 'mcp-exploitation',
    description: 'Command injection in MCP tool parameter',
    pattern: /(?:[;&|`$]\s*(?:cat|ls|rm|mv|cp|curl|wget|nc|bash|sh|python|perl|ruby|chmod|chown|kill|pkill|dd)\b|`[^`]+`|\$\([^)]+\)|\|\s*(?:sh|bash|zsh|python|perl|ruby|nc|curl|wget)\b)/,
    severity: 'critical',
  },
  {
    id: 'MCP-003',
    category: 'mcp-exploitation',
    description: 'SSRF in MCP tool parameter',
    pattern: /(?:(?:https?|ftp|gopher|dict|ldap):\/\/(?:localhost|127\.0\.0\.1|0\.0\.0\.0|10\.\d{1,3}\.\d{1,3}\.\d{1,3}|172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}|192\.168\.\d{1,3}\.\d{1,3}|169\.254\.\d{1,3}\.\d{1,3}|::1|\[::1\]|0x7f|2130706433|metadata\.google|169\.254\.169\.254))/i,
    severity: 'critical',
  },
  {
    id: 'MCP-004',
    category: 'mcp-exploitation',
    description: 'Runtime MCP tool registration or override attempt',
    pattern: /(?:register|add|install|override).*?(?:mcp|tool).*(?:endpoint|url\b|"override"\s*:\s*true|override\s*:\s*true)/i,
    severity: 'critical',
  },
];

// --- A2A Attack Patterns ---

const a2aAttack: ThreatPattern[] = [
  {
    id: 'A2A-001',
    category: 'a2a-attack',
    description: 'Identity spoofing — agent claiming to be another agent',
    pattern: /(?:(?:i\s+am|this\s+is|acting\s+as|speaking\s+(?:as|for)|on\s+behalf\s+of|representing)\s+(?:the\s+)?(?:admin|administrator|orchestrator|supervisor|master|root|system|trusted|authorized|primary)\s+agent)/i,
    severity: 'critical',
  },
  {
    id: 'A2A-002',
    category: 'a2a-attack',
    description: 'Delegation abuse — unauthorized privilege escalation via task delegation',
    pattern: /(?:(?:execute|run|perform|do)\s+(?:this\s+)?(?:\w+\s+)?(?:with|using)\s+(?:elevated|admin|root|system|full|unlimited|unrestricted)\s+(?:privileges?|permissions?|access|authority|rights)|(?:grant|give|elevate|escalate)\s+(?:my\s+)?(?:privileges?|permissions?|access|authority|rights|role))/i,
    severity: 'high',
  },
];

// --- All patterns combined ---

export const ALL_PATTERNS: ThreatPattern[] = [
  ...promptInjection,
  ...jailbreak,
  ...dataExfiltration,
  ...outputLeak,
  ...contextManipulation,
  ...mcpExploitation,
  ...a2aAttack,
];

/** Pattern sets by category for targeted scanning */
export const PATTERN_SETS = {
  promptInjection,
  jailbreak,
  dataExfiltration,
  outputLeak,
  contextManipulation,
  mcpExploitation,
  a2aAttack,
  /** Input scanning: patterns relevant to user/agent input */
  inputPatterns: [...promptInjection, ...jailbreak, ...dataExfiltration, ...contextManipulation],
  /** Output scanning: patterns relevant to LLM responses */
  outputPatterns: [...outputLeak],
  /** MCP scanning: patterns relevant to tool call parameters */
  mcpPatterns: [...mcpExploitation],
  /** A2A scanning: patterns relevant to inter-agent messages */
  a2aPatterns: [...a2aAttack],
} as const;

/** Maximum text length to scan (64 KB) — prevents ReDoS on large payloads */
const MAX_SCAN_LENGTH = 64 * 1024;

/** Scan result from matching */
export interface ScanResult {
  detected: boolean;
  matches: Array<{
    pattern: ThreatPattern;
    matchedText: string;
  }>;
  /** True if input was truncated before scanning */
  truncated?: boolean;
}

/**
 * Scan text against a set of threat patterns.
 * Returns all matches (not just first) for comprehensive reporting.
 * Input is truncated to MAX_SCAN_LENGTH to prevent ReDoS.
 */
export function scanText(text: string, patterns: readonly ThreatPattern[]): ScanResult {
  const truncated = text.length > MAX_SCAN_LENGTH;
  const scannable = truncated ? text.slice(0, MAX_SCAN_LENGTH) : text;
  const matches: ScanResult['matches'] = [];

  for (const pattern of patterns) {
    const match = pattern.pattern.exec(scannable);
    if (match) {
      matches.push({
        pattern,
        matchedText: match[0].slice(0, 200),
      });
    }
  }

  return {
    detected: matches.length > 0,
    matches,
    truncated,
  };
}
