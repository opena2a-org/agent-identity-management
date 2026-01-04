/**
 * Tests for AIM SDK Exception Classes
 *
 * Tests cover:
 * - Base AIMError class
 * - Specialized error classes
 * - Error properties and inheritance
 * - parseAPIError function
 */

import { describe, it, expect } from 'vitest';
import {
  AIMError,
  AuthenticationError,
  AuthorizationError,
  ActionDeniedError,
  VerificationError,
  ConfigurationError,
  NetworkError,
  NotFoundError,
  RateLimitError,
  parseAPIError,
} from './exceptions';

describe('AIMError', () => {
  describe('constructor', () => {
    it('should create error with message only', () => {
      const error = new AIMError('Test error');

      expect(error.message).toBe('Test error');
      expect(error.code).toBe('AIM_ERROR');
      expect(error.statusCode).toBeUndefined();
      expect(error.details).toBeUndefined();
      expect(error.name).toBe('AIMError');
    });

    it('should create error with all parameters', () => {
      const details = { key: 'value' };
      const error = new AIMError('Test error', 'CUSTOM_CODE', 500, details);

      expect(error.message).toBe('Test error');
      expect(error.code).toBe('CUSTOM_CODE');
      expect(error.statusCode).toBe(500);
      expect(error.details).toEqual(details);
    });

    it('should be instanceof Error', () => {
      const error = new AIMError('Test error');
      expect(error).toBeInstanceOf(Error);
    });

    it('should have stack trace', () => {
      const error = new AIMError('Test error');
      expect(error.stack).toBeDefined();
    });
  });
});

describe('AuthenticationError', () => {
  it('should create with default message', () => {
    const error = new AuthenticationError();

    expect(error.message).toBe('Authentication failed');
    expect(error.code).toBe('AUTHENTICATION_ERROR');
    expect(error.statusCode).toBe(401);
    expect(error.name).toBe('AuthenticationError');
  });

  it('should create with custom message', () => {
    const error = new AuthenticationError('Invalid token');

    expect(error.message).toBe('Invalid token');
    expect(error.code).toBe('AUTHENTICATION_ERROR');
    expect(error.statusCode).toBe(401);
  });

  it('should create with details', () => {
    const details = { reason: 'token_expired' };
    const error = new AuthenticationError('Token expired', details);

    expect(error.details).toEqual(details);
  });

  it('should be instanceof AIMError', () => {
    const error = new AuthenticationError();
    expect(error).toBeInstanceOf(AIMError);
  });
});

describe('AuthorizationError', () => {
  it('should create with default message', () => {
    const error = new AuthorizationError();

    expect(error.message).toBe('Authorization failed');
    expect(error.code).toBe('AUTHORIZATION_ERROR');
    expect(error.statusCode).toBe(403);
    expect(error.name).toBe('AuthorizationError');
  });

  it('should create with custom message', () => {
    const error = new AuthorizationError('Insufficient permissions');

    expect(error.message).toBe('Insufficient permissions');
  });

  it('should be instanceof AIMError', () => {
    const error = new AuthorizationError();
    expect(error).toBeInstanceOf(AIMError);
  });
});

describe('ActionDeniedError', () => {
  it('should create with action and reason', () => {
    const error = new ActionDeniedError('db:write', 'Trust score too low');

    expect(error.message).toBe("Action 'db:write' was denied: Trust score too low");
    expect(error.code).toBe('ACTION_DENIED');
    expect(error.statusCode).toBe(403);
    expect(error.name).toBe('ActionDeniedError');
    expect(error.action).toBe('db:write');
    expect(error.reason).toBe('Trust score too low');
    expect(error.trustScore).toBeUndefined();
  });

  it('should include trust score when provided', () => {
    const error = new ActionDeniedError('api:call', 'Low trust', 0.35);

    expect(error.trustScore).toBe(0.35);
    expect(error.action).toBe('api:call');
    expect(error.reason).toBe('Low trust');
  });

  it('should include details when provided', () => {
    const details = { requiredScore: 0.5 };
    const error = new ActionDeniedError('file:delete', 'Denied', 0.2, details);

    expect(error.details).toEqual(details);
  });

  it('should be instanceof AIMError', () => {
    const error = new ActionDeniedError('test', 'test');
    expect(error).toBeInstanceOf(AIMError);
  });
});

describe('VerificationError', () => {
  it('should create with default message', () => {
    const error = new VerificationError();

    expect(error.message).toBe('Verification failed');
    expect(error.code).toBe('VERIFICATION_ERROR');
    expect(error.statusCode).toBe(400);
    expect(error.name).toBe('VerificationError');
  });

  it('should create with custom message', () => {
    const error = new VerificationError('Signature mismatch');

    expect(error.message).toBe('Signature mismatch');
  });

  it('should be instanceof AIMError', () => {
    const error = new VerificationError();
    expect(error).toBeInstanceOf(AIMError);
  });
});

describe('ConfigurationError', () => {
  it('should create with message', () => {
    const error = new ConfigurationError('Missing API key');

    expect(error.message).toBe('Missing API key');
    expect(error.code).toBe('CONFIGURATION_ERROR');
    expect(error.statusCode).toBeUndefined();
    expect(error.name).toBe('ConfigurationError');
  });

  it('should include details', () => {
    const details = { missingField: 'apiKey' };
    const error = new ConfigurationError('Invalid config', details);

    expect(error.details).toEqual(details);
  });

  it('should be instanceof AIMError', () => {
    const error = new ConfigurationError('test');
    expect(error).toBeInstanceOf(AIMError);
  });
});

describe('NetworkError', () => {
  it('should create with message only', () => {
    const error = new NetworkError('Connection refused');

    expect(error.message).toBe('Connection refused');
    expect(error.code).toBe('NETWORK_ERROR');
    expect(error.statusCode).toBeUndefined();
    expect(error.name).toBe('NetworkError');
    expect(error.originalError).toBeUndefined();
  });

  it('should wrap original error', () => {
    const originalError = new Error('ECONNREFUSED');
    const error = new NetworkError('Connection failed', originalError);

    expect(error.originalError).toBe(originalError);
    expect(error.details).toEqual({ originalError: 'ECONNREFUSED' });
  });

  it('should be instanceof AIMError', () => {
    const error = new NetworkError('test');
    expect(error).toBeInstanceOf(AIMError);
  });
});

describe('NotFoundError', () => {
  it('should create with resource type and id', () => {
    const error = new NotFoundError('Agent', 'agent-123');

    expect(error.message).toBe("Agent 'agent-123' not found");
    expect(error.code).toBe('NOT_FOUND');
    expect(error.statusCode).toBe(404);
    expect(error.name).toBe('NotFoundError');
    expect(error.resourceType).toBe('Agent');
    expect(error.resourceId).toBe('agent-123');
  });

  it('should be instanceof AIMError', () => {
    const error = new NotFoundError('Resource', 'id');
    expect(error).toBeInstanceOf(AIMError);
  });
});

describe('RateLimitError', () => {
  it('should create with rate limit info', () => {
    const error = new RateLimitError(60, 100, 0);

    expect(error.message).toBe('Rate limit exceeded. Retry after 60 seconds.');
    expect(error.code).toBe('RATE_LIMIT');
    expect(error.statusCode).toBe(429);
    expect(error.name).toBe('RateLimitError');
    expect(error.retryAfter).toBe(60);
    expect(error.limit).toBe(100);
    expect(error.remaining).toBe(0);
  });

  it('should include details', () => {
    const error = new RateLimitError(30, 1000, 5);

    expect(error.details).toEqual({
      retryAfter: 30,
      limit: 1000,
      remaining: 5,
    });
  });

  it('should be instanceof AIMError', () => {
    const error = new RateLimitError(1, 1, 0);
    expect(error).toBeInstanceOf(AIMError);
  });
});

describe('parseAPIError', () => {
  it('should parse 401 as AuthenticationError', () => {
    const error = parseAPIError(401, { message: 'Invalid credentials' });

    expect(error).toBeInstanceOf(AuthenticationError);
    expect(error.message).toBe('Invalid credentials');
    expect(error.statusCode).toBe(401);
  });

  it('should parse 403 as AuthorizationError', () => {
    const error = parseAPIError(403, { message: 'Access denied' });

    expect(error).toBeInstanceOf(AuthorizationError);
    expect(error.message).toBe('Access denied');
    expect(error.statusCode).toBe(403);
  });

  it('should parse 404 as NotFoundError', () => {
    const error = parseAPIError(404, { message: 'agent-123' });

    expect(error).toBeInstanceOf(NotFoundError);
    expect(error.statusCode).toBe(404);
  });

  it('should parse 429 as RateLimitError', () => {
    const error = parseAPIError(429, {
      message: 'Too many requests',
      retryAfter: 30,
      limit: 100,
      remaining: 0,
    });

    expect(error).toBeInstanceOf(RateLimitError);
    expect(error.statusCode).toBe(429);
    expect((error as RateLimitError).retryAfter).toBe(30);
  });

  it('should use default retry values for 429', () => {
    const error = parseAPIError(429, { message: 'Rate limited' });

    expect(error).toBeInstanceOf(RateLimitError);
    expect((error as RateLimitError).retryAfter).toBe(60);
    expect((error as RateLimitError).limit).toBe(0);
    expect((error as RateLimitError).remaining).toBe(0);
  });

  it('should parse unknown status as generic AIMError', () => {
    const error = parseAPIError(500, { message: 'Internal error' });

    expect(error).toBeInstanceOf(AIMError);
    expect(error.message).toBe('Internal error');
    expect(error.code).toBe('API_ERROR');
    expect(error.statusCode).toBe(500);
  });

  it('should handle error field instead of message', () => {
    const error = parseAPIError(400, { error: 'Bad request' });

    expect(error.message).toBe('Bad request');
  });

  it('should use default message for empty body', () => {
    const error = parseAPIError(500, {});

    expect(error.message).toBe('Unknown error');
  });

  it('should preserve error body as details', () => {
    const body = { message: 'Test', extra: 'data' };
    const error = parseAPIError(400, body);

    expect(error.details).toEqual(body);
  });
});

describe('Error inheritance chain', () => {
  it('all errors should be catchable as Error', () => {
    const errors = [
      new AIMError('test'),
      new AuthenticationError(),
      new AuthorizationError(),
      new ActionDeniedError('action', 'reason'),
      new VerificationError(),
      new ConfigurationError('config'),
      new NetworkError('network'),
      new NotFoundError('type', 'id'),
      new RateLimitError(1, 1, 0),
    ];

    for (const error of errors) {
      expect(error).toBeInstanceOf(Error);
    }
  });

  it('specialized errors should be catchable as AIMError', () => {
    const errors = [
      new AuthenticationError(),
      new AuthorizationError(),
      new ActionDeniedError('action', 'reason'),
      new VerificationError(),
      new ConfigurationError('config'),
      new NetworkError('network'),
      new NotFoundError('type', 'id'),
      new RateLimitError(1, 1, 0),
    ];

    for (const error of errors) {
      expect(error).toBeInstanceOf(AIMError);
    }
  });
});
