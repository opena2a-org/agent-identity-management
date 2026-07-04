/**
 * Tests for Fastify Plugin
 *
 * Tests cover:
 * - aimPlugin factory
 * - verifyAction hook
 * - verifyRoute hook
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { FastifyRequest, FastifyReply } from 'fastify';
import { aimPlugin, verifyAction, verifyRoute } from './fastify';
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

describe('aimPlugin', () => {
  it('should be a valid Fastify plugin callback', () => {
    expect(typeof aimPlugin).toBe('function');
    expect(aimPlugin.length).toBe(3); // (fastify, options, done)
  });

  it('should call done callback', () => {
    const mockFastify = {
      decorateRequest: vi.fn(),
      addHook: vi.fn(),
      setErrorHandler: vi.fn(),
    };
    const done = vi.fn();

    aimPlugin(mockFastify as any, {}, done);

    expect(done).toHaveBeenCalled();
  });

  it('should decorate request with aim property', () => {
    const mockFastify = {
      decorateRequest: vi.fn(),
      addHook: vi.fn(),
      setErrorHandler: vi.fn(),
    };

    aimPlugin(mockFastify as any, {}, vi.fn());

    expect(mockFastify.decorateRequest).toHaveBeenCalledWith('aim', undefined);
  });

  it('should add onRequest hook', () => {
    const mockFastify = {
      decorateRequest: vi.fn(),
      addHook: vi.fn(),
      setErrorHandler: vi.fn(),
    };

    aimPlugin(mockFastify as any, {}, vi.fn());

    expect(mockFastify.addHook).toHaveBeenCalledWith('onRequest', expect.any(Function));
  });

  it('should set error handler', () => {
    const mockFastify = {
      decorateRequest: vi.fn(),
      addHook: vi.fn(),
      setErrorHandler: vi.fn(),
    };

    aimPlugin(mockFastify as any, {}, vi.fn());

    expect(mockFastify.setErrorHandler).toHaveBeenCalled();
  });

  it('should accept options', () => {
    const mockFastify = {
      decorateRequest: vi.fn(),
      addHook: vi.fn(),
      setErrorHandler: vi.fn(),
    };

    aimPlugin(
      mockFastify as any,
      {
        baseUrl: 'https://aim.example.com',
        apiKey: 'test-key',
        skipPaths: ['/custom-health'],
      },
      vi.fn()
    );

    expect(mockFastify.addHook).toHaveBeenCalled();
  });
});

describe('verifyAction', () => {
  let mockRequest: Partial<FastifyRequest>;
  let mockReply: Partial<FastifyReply>;
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

    mockRequest = {
      url: '/api/resource',
      method: 'POST',
      query: {},
      ip: '127.0.0.1',
      aim: {
        client: mockClient as any,
        verified: false,
      },
    };
    mockReply = {
      status: vi.fn().mockReturnThis(),
      send: vi.fn(),
    };
  });

  it('should create preHandler hook', () => {
    const hook = verifyAction('db:read');
    expect(typeof hook).toBe('function');
  });

  it('should verify action on client', async () => {
    const hook = verifyAction('db:write');

    await hook(mockRequest as FastifyRequest, mockReply as FastifyReply);

    expect(mockClient.verifyAction).toHaveBeenCalledWith(
      expect.objectContaining({
        action: 'db:write',
        resource: '/api/resource',
        resourceType: 'post',
      })
    );
  });

  it('should update request aim context on success', async () => {
    const hook = verifyAction('db:read');

    await hook(mockRequest as FastifyRequest, mockReply as FastifyReply);

    expect(mockRequest.aim?.verified).toBe(true);
    expect(mockRequest.aim?.trustScore).toBe(0.9);
  });

  it('should return 500 if plugin not initialized', async () => {
    mockRequest.aim = undefined;
    const hook = verifyAction('db:read');

    await hook(mockRequest as FastifyRequest, mockReply as FastifyReply);

    expect(mockReply.status).toHaveBeenCalledWith(500);
    expect(mockReply.send).toHaveBeenCalledWith({
      error: 'AIM plugin not initialized',
    });
  });

  it('should return 403 on ActionDeniedError', async () => {
    mockClient.verifyAction.mockRejectedValue(
      new ActionDeniedError('db:delete', 'Insufficient trust', 0.2)
    );

    const hook = verifyAction('db:delete');
    await hook(mockRequest as FastifyRequest, mockReply as FastifyReply);

    expect(mockReply.status).toHaveBeenCalledWith(403);
    expect(mockReply.send).toHaveBeenCalledWith({
      error: 'Action denied',
      action: 'db:delete',
      reason: 'Insufficient trust',
      trustScore: 0.2,
    });
  });

  it('should return 401 on AuthenticationError', async () => {
    mockClient.verifyAction.mockRejectedValue(
      new AuthenticationError('Token expired')
    );

    const hook = verifyAction('db:read');
    await hook(mockRequest as FastifyRequest, mockReply as FastifyReply);

    expect(mockReply.status).toHaveBeenCalledWith(401);
    expect(mockReply.send).toHaveBeenCalledWith({
      error: 'Authentication required',
      message: 'Token expired',
    });
  });

  it('should throw other errors', async () => {
    const genericError = new Error('Generic error');
    mockClient.verifyAction.mockRejectedValue(genericError);

    const hook = verifyAction('db:read');

    await expect(
      hook(mockRequest as FastifyRequest, mockReply as FastifyReply)
    ).rejects.toThrow('Generic error');
  });

  it('should include context in verification', async () => {
    const hook = verifyAction('db:read', {
      context: { custom: 'value' },
    });

    await hook(mockRequest as FastifyRequest, mockReply as FastifyReply);

    expect(mockClient.verifyAction).toHaveBeenCalledWith(
      expect.objectContaining({
        context: expect.objectContaining({
          custom: 'value',
        }),
      })
    );
  });
});

describe('verifyRoute', () => {
  let mockRequest: Partial<FastifyRequest>;
  let mockReply: Partial<FastifyReply>;
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

    mockRequest = {
      url: '/api/users/123',
      method: 'GET',
      query: {},
      ip: '127.0.0.1',
      aim: {
        client: mockClient as any,
        verified: false,
      },
    };
    mockReply = {
      status: vi.fn().mockReturnThis(),
      send: vi.fn(),
    };
  });

  it('should create preHandler hook', () => {
    const hook = verifyRoute('users');
    expect(typeof hook).toBe('function');
  });

  it('should map GET to read action', async () => {
    mockRequest.method = 'GET';
    const hook = verifyRoute('users');

    await hook(mockRequest as FastifyRequest, mockReply as FastifyReply);

    expect(mockClient.verifyAction).toHaveBeenCalledWith(
      expect.objectContaining({
        action: 'users:read',
      })
    );
  });

  it('should map POST to create action', async () => {
    mockRequest.method = 'POST';
    const hook = verifyRoute('users');

    await hook(mockRequest as FastifyRequest, mockReply as FastifyReply);

    expect(mockClient.verifyAction).toHaveBeenCalledWith(
      expect.objectContaining({
        action: 'users:create',
      })
    );
  });

  it('should map PUT to update action', async () => {
    mockRequest.method = 'PUT';
    const hook = verifyRoute('users');

    await hook(mockRequest as FastifyRequest, mockReply as FastifyReply);

    expect(mockClient.verifyAction).toHaveBeenCalledWith(
      expect.objectContaining({
        action: 'users:update',
      })
    );
  });

  it('should map PATCH to update action', async () => {
    mockRequest.method = 'PATCH';
    const hook = verifyRoute('users');

    await hook(mockRequest as FastifyRequest, mockReply as FastifyReply);

    expect(mockClient.verifyAction).toHaveBeenCalledWith(
      expect.objectContaining({
        action: 'users:update',
      })
    );
  });

  it('should map DELETE to delete action', async () => {
    mockRequest.method = 'DELETE';
    const hook = verifyRoute('users');

    await hook(mockRequest as FastifyRequest, mockReply as FastifyReply);

    expect(mockClient.verifyAction).toHaveBeenCalledWith(
      expect.objectContaining({
        action: 'users:delete',
      })
    );
  });

  it('should map unknown methods to access action', async () => {
    mockRequest.method = 'HEAD';
    const hook = verifyRoute('users');

    await hook(mockRequest as FastifyRequest, mockReply as FastifyReply);

    expect(mockClient.verifyAction).toHaveBeenCalledWith(
      expect.objectContaining({
        action: 'users:access',
      })
    );
  });

  it('should return 500 if plugin not initialized', async () => {
    mockRequest.aim = undefined;
    const hook = verifyRoute('users');

    await hook(mockRequest as FastifyRequest, mockReply as FastifyReply);

    expect(mockReply.status).toHaveBeenCalledWith(500);
  });

  it('should handle ActionDeniedError', async () => {
    mockClient.verifyAction.mockRejectedValue(
      new ActionDeniedError('users:delete', 'Not allowed', 0.5)
    );

    const hook = verifyRoute('users');
    mockRequest.method = 'DELETE';

    await hook(mockRequest as FastifyRequest, mockReply as FastifyReply);

    expect(mockReply.status).toHaveBeenCalledWith(403);
  });

  it('should handle AuthenticationError', async () => {
    mockClient.verifyAction.mockRejectedValue(
      new AuthenticationError('Not logged in')
    );

    const hook = verifyRoute('users');

    await hook(mockRequest as FastifyRequest, mockReply as FastifyReply);

    expect(mockReply.status).toHaveBeenCalledWith(401);
  });

  it('should update aim context on success', async () => {
    const hook = verifyRoute('users');

    await hook(mockRequest as FastifyRequest, mockReply as FastifyReply);

    expect(mockRequest.aim?.verified).toBe(true);
    expect(mockRequest.aim?.trustScore).toBe(0.9);
  });
});

describe('Plugin error handler', () => {
  it('should handle ActionDeniedError in error handler', () => {
    const mockFastify = {
      decorateRequest: vi.fn(),
      addHook: vi.fn(),
      setErrorHandler: vi.fn(),
    };

    aimPlugin(mockFastify as any, {}, vi.fn());

    // Get the error handler that was set
    const errorHandler = mockFastify.setErrorHandler.mock.calls[0][0];
    expect(typeof errorHandler).toBe('function');
  });
});

describe('Fastify exports', () => {
  it('should export aimPlugin', async () => {
    const { aimPlugin } = await import('./fastify');
    expect(aimPlugin).toBeDefined();
  });

  it('should export verifyAction', async () => {
    const { verifyAction } = await import('./fastify');
    expect(verifyAction).toBeDefined();
  });

  it('should export verifyRoute', async () => {
    const { verifyRoute } = await import('./fastify');
    expect(verifyRoute).toBeDefined();
  });

  it('should export AIMClient', async () => {
    const { AIMClient } = await import('./fastify');
    expect(AIMClient).toBeDefined();
  });
});

// Regression tests for the encapsulation defect: the mocked-instance tests
// above invoke the plugin function directly, which cannot catch decorations
// being trapped in the plugin's own child context. These register the plugin
// through a REAL Fastify instance and exercise routes defined on the root
// instance — the way the README tells users to wire it.
//
// Both peer majors are exercised: 'fastify' resolves the v5 devDependency,
// 'fastify4' is an npm alias for fastify@^4. fastify-plugin@6 declares no
// peerDependencies — the '4.x || 5.x' range passed to fp() is validated by
// fastify itself at register time, so this matrix is what actually proves
// the peer range in package.json.
describe.each([
  ['fastify 5', 'fastify'],
  ['fastify 4', 'fastify4'],
])('aimPlugin through %s register (real instance)', (_label, fastifyModule) => {
  // Non-literal specifier: resolved at runtime (both aliases are installed),
  // and TypeScript does not demand declarations for the alias.
  const loadFastify = async () =>
    ((await import(fastifyModule)) as { default: typeof import('fastify').default }).default;

  it('populates request.aim on routes registered outside the plugin', async () => {
    const Fastify = await loadFastify();
    const app = Fastify();
    await app.register(aimPlugin, { baseUrl: 'http://localhost:9' });
    app.get('/whoami', async (request) => ({
      hasClient: Boolean(request.aim?.client),
      verified: request.aim?.verified ?? null,
    }));

    const res = await app.inject({ method: 'GET', url: '/whoami' });

    expect(res.statusCode).toBe(200);
    expect(res.json()).toEqual({ hasClient: true, verified: false });
    await app.close();
  });

  it('verifyAction preHandler works on a root-instance route (was 500 "AIM plugin not initialized")', async () => {
    const Fastify = await loadFastify();
    const app = Fastify();
    await app.register(aimPlugin, { baseUrl: 'http://localhost:9' });
    app.post(
      '/api/data',
      { preHandler: verifyAction('data:write') },
      async () => ({ success: true })
    );

    const res = await app.inject({ method: 'POST', url: '/api/data' });

    expect(res.statusCode).toBe(200);
    expect(res.json()).toEqual({ success: true });
    await app.close();
  });

  it('maps ActionDeniedError thrown by a root-instance handler to 403', async () => {
    const Fastify = await loadFastify();
    const app = Fastify();
    await app.register(aimPlugin, { baseUrl: 'http://localhost:9' });
    app.get('/deny', async () => {
      throw new ActionDeniedError('file:delete', 'trust score below threshold', 0.2);
    });

    const res = await app.inject({ method: 'GET', url: '/deny' });

    expect(res.statusCode).toBe(403);
    expect(res.json()).toMatchObject({
      error: 'Action denied',
      action: 'file:delete',
      reason: 'trust score below threshold',
    });
    await app.close();
  });
});
