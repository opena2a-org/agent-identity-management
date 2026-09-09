/**
 * AIM SDK Exception Classes
 */

/**
 * Base exception for all AIM SDK errors
 */
export class AIMError extends Error {
  public readonly code: string;
  public readonly statusCode?: number;
  public readonly details?: Record<string, unknown>;

  constructor(
    message: string,
    code: string = 'AIM_ERROR',
    statusCode?: number,
    details?: Record<string, unknown>
  ) {
    super(message);
    this.name = 'AIMError';
    this.code = code;
    this.statusCode = statusCode;
    this.details = details;
    Error.captureStackTrace?.(this, this.constructor);
  }
}

/**
 * Authentication failed
 */
export class AuthenticationError extends AIMError {
  constructor(message: string = 'Authentication failed', details?: Record<string, unknown>) {
    super(message, 'AUTHENTICATION_ERROR', 401, details);
    this.name = 'AuthenticationError';
  }
}

/**
 * Authorization failed - action not permitted
 */
export class AuthorizationError extends AIMError {
  constructor(message: string = 'Authorization failed', details?: Record<string, unknown>) {
    super(message, 'AUTHORIZATION_ERROR', 403, details);
    this.name = 'AuthorizationError';
  }
}

/**
 * Action was denied by the AIM server
 */
export class ActionDeniedError extends AIMError {
  public readonly action: string;
  public readonly reason: string;
  public readonly trustScore?: number;

  constructor(
    action: string,
    reason: string,
    trustScore?: number,
    details?: Record<string, unknown>
  ) {
    super(`Action '${action}' was denied: ${reason}`, 'ACTION_DENIED', 403, details);
    this.name = 'ActionDeniedError';
    this.action = action;
    this.reason = reason;
    this.trustScore = trustScore;
  }
}

/**
 * Verification failed
 */
export class VerificationError extends AIMError {
  constructor(message: string = 'Verification failed', details?: Record<string, unknown>) {
    super(message, 'VERIFICATION_ERROR', 400, details);
    this.name = 'VerificationError';
  }
}

/**
 * Configuration error
 */
export class ConfigurationError extends AIMError {
  constructor(message: string, details?: Record<string, unknown>) {
    super(message, 'CONFIGURATION_ERROR', undefined, details);
    this.name = 'ConfigurationError';
  }
}

/**
 * Network or connection error
 */
export class NetworkError extends AIMError {
  public readonly originalError?: Error;

  constructor(message: string, originalError?: Error) {
    super(message, 'NETWORK_ERROR', undefined, { originalError: originalError?.message });
    this.name = 'NetworkError';
    this.originalError = originalError;
  }
}

/**
 * Resource not found
 */
export class NotFoundError extends AIMError {
  public readonly resourceType: string;
  public readonly resourceId: string;

  constructor(resourceType: string, resourceId: string) {
    super(`${resourceType} '${resourceId}' not found`, 'NOT_FOUND', 404);
    this.name = 'NotFoundError';
    this.resourceType = resourceType;
    this.resourceId = resourceId;
  }
}

/**
 * Rate limit exceeded
 */
export class RateLimitError extends AIMError {
  public readonly retryAfter: number;
  public readonly limit: number;
  public readonly remaining: number;

  constructor(retryAfter: number, limit: number, remaining: number) {
    super(`Rate limit exceeded. Retry after ${retryAfter} seconds.`, 'RATE_LIMIT', 429, {
      retryAfter,
      limit,
      remaining,
    });
    this.name = 'RateLimitError';
    this.retryAfter = retryAfter;
    this.limit = limit;
    this.remaining = remaining;
  }
}

/**
 * Secrets operation failed
 */
export class SecretsError extends AIMError {
  constructor(message: string, details?: Record<string, unknown>) {
    super(message, 'SECRETS_ERROR', undefined, details);
    this.name = 'SecretsError';
  }
}

/**
 * Cap on a server-supplied error message. The body is attacker/outage
 * controlled and the message lands in logs, terminals, and error trackers
 * verbatim, so it is type-checked and capped rather than passed through whole.
 */
const MAX_API_ERROR_MESSAGE = 512;

/**
 * Minimal header lookup — satisfied by the fetch Headers class and by mocks.
 */
export interface HeaderLookup {
  get(name: string): string | null;
}

/**
 * Read the response's Retry-After header as seconds. Accepts the delay-seconds
 * form and the HTTP-date form (converted to seconds from now); returns
 * undefined when the header is absent or unparseable.
 */
function retryAfterSeconds(headers?: HeaderLookup): number | undefined {
  const raw = headers?.get?.('retry-after')?.trim();
  if (!raw) return undefined;
  if (/^\d+$/.test(raw)) return Number(raw);
  const dateMs = Date.parse(raw);
  if (!Number.isNaN(dateMs)) return Math.max(0, Math.ceil((dateMs - Date.now()) / 1000));
  return undefined;
}

/**
 * Walk an error's cause chain (including AggregateError members, where Node's
 * fetch buries connection errno) for the first errno-style code.
 */
export function unwrapErrnoCode(error: unknown, seen = new Set<unknown>()): string | undefined {
  let current: unknown = error;
  while (current && typeof current === 'object' && !seen.has(current)) {
    seen.add(current);
    const err = current as NodeJS.ErrnoException & { cause?: unknown; errors?: unknown[] };
    if (typeof err.code === 'string' && err.code !== '') return err.code;
    if (Array.isArray(err.errors)) {
      for (const member of err.errors) {
        const code = unwrapErrnoCode(member, seen);
        if (code) return code;
      }
    }
    current = err.cause;
  }
  return undefined;
}

/**
 * Parse API error response. When the response headers are supplied, a 429's
 * Retry-After header takes precedence over the body's retryAfter field.
 */
export function parseAPIError(statusCode: number, body: unknown, headers?: HeaderLookup): AIMError {
  const errorBody = body as Record<string, unknown>;
  const raw = errorBody?.message ?? errorBody?.error;
  let message = typeof raw === 'string' && raw.length > 0 ? raw : 'Unknown error';
  if (message.length > MAX_API_ERROR_MESSAGE) {
    message = `${message.slice(0, MAX_API_ERROR_MESSAGE)}… [truncated]`;
  }

  switch (statusCode) {
    case 401:
      return new AuthenticationError(message, errorBody);
    case 403:
      return new AuthorizationError(message, errorBody);
    case 404:
      return new NotFoundError('resource', message);
    case 429:
      return new RateLimitError(
        retryAfterSeconds(headers) ?? (errorBody?.retryAfter as number) ?? 60,
        (errorBody?.limit as number) ?? 0,
        (errorBody?.remaining as number) ?? 0
      );
    default:
      return new AIMError(message, 'API_ERROR', statusCode, errorBody);
  }
}
