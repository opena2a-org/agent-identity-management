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
 * Parse API error response
 */
export function parseAPIError(statusCode: number, body: unknown): AIMError {
  const errorBody = body as Record<string, unknown>;
  const message = (errorBody?.message ?? errorBody?.error ?? 'Unknown error') as string;

  switch (statusCode) {
    case 401:
      return new AuthenticationError(message, errorBody);
    case 403:
      return new AuthorizationError(message, errorBody);
    case 404:
      return new NotFoundError('resource', message);
    case 429:
      return new RateLimitError(
        (errorBody?.retryAfter as number) ?? 60,
        (errorBody?.limit as number) ?? 0,
        (errorBody?.remaining as number) ?? 0
      );
    default:
      return new AIMError(message, 'API_ERROR', statusCode, errorBody);
  }
}
