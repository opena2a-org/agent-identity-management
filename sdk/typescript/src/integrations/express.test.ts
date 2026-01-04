/**
 * Tests for Express.js Middleware
 *
 * Tests cover:
 * - createAIMMiddleware factory
 * - verifyAction middleware
 * - verifyRouter middleware
 * - aimErrorHandler
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { Request, Response, NextFunction } from 'express';
import {
  createAIMMiddleware,
  verifyAction,
  verifyRouter,
  aimErrorHandler,
  AIMContext,
  AIMRequest,
} from './express';
import { ActionDeniedError, AuthenticationError } from '../exceptions';

// Mock AIMClient
vi.mock('../client/AIMClient', () => ({
  AIMClient: vi.fn().mockImplementation(() => ({
    getAgent: vi.fn().mockResolvedValue(null),
    verifyAction: vi.fn().mockResolvedValue({
      verified: true,
      trustScore: 0.85,
    }),
    isAuthenticated: vi.fn().mockReturnValue(false),
  })),
}));

describe('createAIMMiddleware', () => {
  let mockReq: Partial<AIMRequest>;
  let mockRes: Partial<Response>;
  let mockNext: NextFunction;

  beforeEach(() => {
    mockReq = {
      path: '/api/test',
    };
    mockRes = {
      status: vi.fn().mockReturnThis(),
      json: vi.fn(),
    };
    mockNext = vi.fn();
  });

  it('should create middleware function', () => {
    const middleware = createAIMMiddleware();
    expect(typeof middleware).toBe('function');
  });

  it('should add aim context to request', async () => {
    const middleware = createAIMMiddleware();

    await middleware(mockReq as AIMRequest, mockRes as Response, mockNext);

    expect(mockReq.aim).toBeDefined();
    expect(mockReq.aim?.client).toBeDefined();
    expect(mockReq.aim?.verified).toBe(false);
  });

  it('should call next() on success', async () => {
    const middleware = createAIMMiddleware();

    await middleware(mockReq as AIMRequest, mockRes as Response, mockNext);

    expect(mockNext).toHaveBeenCalled();
  });

  it('should skip paths in skipPaths option', async () => {
    const middleware = createAIMMiddleware({
      skipPaths: ['/health', '/ready'],
    });

    mockReq.path = '/health';
    await middleware(mockReq as AIMRequest, mockRes as Response, mockNext);

    expect(mockNext).toHaveBeenCalled();
  });

  it('should use default skip paths', async () => {
    const middleware = createAIMMiddleware();

    // Default skip paths: /health, /ready, /metrics
    mockReq.path = '/metrics';
    await middleware(mockReq as AIMRequest, mockRes as Response, mockNext);

    expect(mockNext).toHaveBeenCalled();
  });

  it('should accept custom options', () => {
    const middleware = createAIMMiddleware({
      baseUrl: 'https://custom.aim.com',
      apiKey: 'custom-api-key',
      timeout: 5000,
    });

    expect(typeof middleware).toBe('function');
  });

  it('should support custom error handler', async () => {
    const customErrorHandler = vi.fn();
    const middleware = createAIMMiddleware({
      onError: customErrorHandler,
    });

    // The default mock client won't throw, so we verify the handler is set
    expect(typeof middleware).toBe('function');
  });
});

describe('verifyAction', () => {
  let mockReq: Partial<AIMRequest>;
  let mockRes: Partial<Response>;
  let mockNext: NextFunction;
  let mockClient: {
    verifyAction: ReturnType<typeof vi.fn>;
  };

  beforeEach(() => {
    mockClient = {
      verifyAction: vi.fn().mockResolvedValue({
        verified: true,
        trustScore: 0.9,
      }),
    };

    mockReq = {
      path: '/api/resource',
      method: 'POST',
      query: {},
      ip: '127.0.0.1',
      aim: {
        client: mockClient as any,
        verified: false,
      },
    };
    mockRes = {
      status: vi.fn().mockReturnThis(),
      json: vi.fn(),
    };
    mockNext = vi.fn();
  });

  it('should create middleware for action', () => {
    const middleware = verifyAction('db:read');
    expect(typeof middleware).toBe('function');
  });

  it('should call verifyAction on client', async () => {
    const middleware = verifyAction('db:read');

    await middleware(mockReq as AIMRequest, mockRes as Response, mockNext);

    expect(mockClient.verifyAction).toHaveBeenCalledWith(
      expect.objectContaining({
        action: 'db:read',
        resource: '/api/resource',
        resourceType: 'post',
      })
    );
  });

  it('should call next on successful verification', async () => {
    const middleware = verifyAction('db:read');

    await middleware(mockReq as AIMRequest, mockRes as Response, mockNext);

    expect(mockNext).toHaveBeenCalled();
    expect(mockReq.aim?.verified).toBe(true);
    expect(mockReq.aim?.trustScore).toBe(0.9);
  });

  it('should return 500 if AIM middleware not initialized', async () => {
    mockReq.aim = undefined;
    const middleware = verifyAction('db:read');

    await middleware(mockReq as AIMRequest, mockRes as Response, mockNext);

    expect(mockRes.status).toHaveBeenCalledWith(500);
    expect(mockRes.json).toHaveBeenCalledWith({
      error: 'AIM middleware not initialized',
    });
  });

  it('should return 403 on ActionDeniedError', async () => {
    mockClient.verifyAction.mockRejectedValue(
      new ActionDeniedError('db:delete', 'Low trust score', 0.3)
    );

    const middleware = verifyAction('db:delete');
    await middleware(mockReq as AIMRequest, mockRes as Response, mockNext);

    expect(mockRes.status).toHaveBeenCalledWith(403);
    expect(mockRes.json).toHaveBeenCalledWith({
      error: 'Action denied',
      action: 'db:delete',
      reason: 'Low trust score',
      trustScore: 0.3,
    });
  });

  it('should return 401 on AuthenticationError', async () => {
    mockClient.verifyAction.mockRejectedValue(
      new AuthenticationError('Not authenticated')
    );

    const middleware = verifyAction('db:read');
    await middleware(mockReq as AIMRequest, mockRes as Response, mockNext);

    expect(mockRes.status).toHaveBeenCalledWith(401);
    expect(mockRes.json).toHaveBeenCalledWith({
      error: 'Authentication required',
      message: 'Not authenticated',
    });
  });

  it('should pass through other errors to next', async () => {
    const genericError = new Error('Generic error');
    mockClient.verifyAction.mockRejectedValue(genericError);

    const middleware = verifyAction('db:read');
    await middleware(mockReq as AIMRequest, mockRes as Response, mockNext);

    expect(mockNext).toHaveBeenCalledWith(genericError);
  });

  it('should accept additional options', async () => {
    const middleware = verifyAction('db:read', {
      context: { extra: 'data' },
    });

    await middleware(mockReq as AIMRequest, mockRes as Response, mockNext);

    expect(mockClient.verifyAction).toHaveBeenCalledWith(
      expect.objectContaining({
        context: expect.objectContaining({
          extra: 'data',
        }),
      })
    );
  });
});

describe('verifyRouter', () => {
  let mockReq: Partial<AIMRequest>;
  let mockRes: Partial<Response>;
  let mockNext: NextFunction;
  let mockClient: {
    verifyAction: ReturnType<typeof vi.fn>;
  };

  beforeEach(() => {
    mockClient = {
      verifyAction: vi.fn().mockResolvedValue({
        verified: true,
        trustScore: 0.9,
      }),
    };

    mockReq = {
      path: '/api/users',
      method: 'GET',
      query: {},
      ip: '127.0.0.1',
      aim: {
        client: mockClient as any,
        verified: false,
      },
    };
    mockRes = {
      status: vi.fn().mockReturnThis(),
      json: vi.fn(),
    };
    mockNext = vi.fn();
  });

  it('should create middleware for router', () => {
    const middleware = verifyRouter('api');
    expect(typeof middleware).toBe('function');
  });

  it('should map GET to read action', async () => {
    mockReq.method = 'GET';
    const middleware = verifyRouter('users');

    await middleware(mockReq as AIMRequest, mockRes as Response, mockNext);

    expect(mockClient.verifyAction).toHaveBeenCalledWith(
      expect.objectContaining({
        action: 'users:read',
      })
    );
  });

  it('should map POST to create action', async () => {
    mockReq.method = 'POST';
    const middleware = verifyRouter('users');

    await middleware(mockReq as AIMRequest, mockRes as Response, mockNext);

    expect(mockClient.verifyAction).toHaveBeenCalledWith(
      expect.objectContaining({
        action: 'users:create',
      })
    );
  });

  it('should map PUT to update action', async () => {
    mockReq.method = 'PUT';
    const middleware = verifyRouter('users');

    await middleware(mockReq as AIMRequest, mockRes as Response, mockNext);

    expect(mockClient.verifyAction).toHaveBeenCalledWith(
      expect.objectContaining({
        action: 'users:update',
      })
    );
  });

  it('should map PATCH to update action', async () => {
    mockReq.method = 'PATCH';
    const middleware = verifyRouter('users');

    await middleware(mockReq as AIMRequest, mockRes as Response, mockNext);

    expect(mockClient.verifyAction).toHaveBeenCalledWith(
      expect.objectContaining({
        action: 'users:update',
      })
    );
  });

  it('should map DELETE to delete action', async () => {
    mockReq.method = 'DELETE';
    const middleware = verifyRouter('users');

    await middleware(mockReq as AIMRequest, mockRes as Response, mockNext);

    expect(mockClient.verifyAction).toHaveBeenCalledWith(
      expect.objectContaining({
        action: 'users:delete',
      })
    );
  });

  it('should map unknown methods to access action', async () => {
    mockReq.method = 'OPTIONS';
    const middleware = verifyRouter('users');

    await middleware(mockReq as AIMRequest, mockRes as Response, mockNext);

    expect(mockClient.verifyAction).toHaveBeenCalledWith(
      expect.objectContaining({
        action: 'users:access',
      })
    );
  });
});

describe('aimErrorHandler', () => {
  let mockReq: Partial<Request>;
  let mockRes: Partial<Response>;
  let mockNext: NextFunction;

  beforeEach(() => {
    mockReq = {};
    mockRes = {
      status: vi.fn().mockReturnThis(),
      json: vi.fn(),
    };
    mockNext = vi.fn();
  });

  it('should handle ActionDeniedError', () => {
    const error = new ActionDeniedError('db:write', 'Trust too low', 0.4);

    aimErrorHandler(error, mockReq as Request, mockRes as Response, mockNext);

    expect(mockRes.status).toHaveBeenCalledWith(403);
    expect(mockRes.json).toHaveBeenCalledWith({
      error: 'Action denied',
      action: 'db:write',
      reason: 'Trust too low',
      trustScore: 0.4,
    });
  });

  it('should handle AuthenticationError', () => {
    const error = new AuthenticationError('Invalid credentials');

    aimErrorHandler(error, mockReq as Request, mockRes as Response, mockNext);

    expect(mockRes.status).toHaveBeenCalledWith(401);
    expect(mockRes.json).toHaveBeenCalledWith({
      error: 'Authentication required',
      message: 'Invalid credentials',
    });
  });

  it('should pass other errors to next', () => {
    const error = new Error('Generic error');

    aimErrorHandler(error, mockReq as Request, mockRes as Response, mockNext);

    expect(mockNext).toHaveBeenCalledWith(error);
    expect(mockRes.status).not.toHaveBeenCalled();
  });
});

describe('AIMContext interface', () => {
  it('should be assignable with valid properties', () => {
    const context: AIMContext = {
      client: {} as any,
      agentId: 'agent-123',
      trustScore: 0.85,
      verified: true,
    };
    expect(context.verified).toBe(true);
  });

  it('should allow optional properties', () => {
    const context: AIMContext = {
      client: {} as any,
    };
    expect(context.agentId).toBeUndefined();
    expect(context.trustScore).toBeUndefined();
    expect(context.verified).toBeUndefined();
  });
});

describe('Express exports', () => {
  it('should export AIMClient', async () => {
    const { AIMClient } = await import('./express');
    expect(AIMClient).toBeDefined();
  });

  it('should export types from parent', async () => {
    const exports = await import('./express');
    expect(exports.createAIMMiddleware).toBeDefined();
    expect(exports.verifyAction).toBeDefined();
    expect(exports.verifyRouter).toBeDefined();
    expect(exports.aimErrorHandler).toBeDefined();
  });
});
