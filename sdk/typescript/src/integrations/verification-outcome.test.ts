/**
 * A server-side denial must surface as a denial, in every integration.
 *
 * `parseAPIError` maps HTTP 403 to `AuthorizationError` (see `../exceptions`).
 * Before 1.2.0 the express middleware, the fastify preHandler, fastify's
 * `verifyRoute` and `aimErrorHandler` each checked only for `ActionDeniedError`
 * and `AuthenticationError`, so an `AuthorizationError` matched no branch and
 * fell through to `next(error)` / `throw` — the caller got an opaque 500 with no
 * denial reason, for the ordinary case of AIM refusing the action over the wire.
 *
 * Every assertion below fails against the pre-1.2.0 ladders.
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { Response, NextFunction } from 'express';
import Fastify from 'fastify';

import {
  ActionDeniedError,
  AuthenticationError,
  AuthorizationError,
  NetworkError,
  parseAPIError,
} from '../exceptions';
import { classifyVerificationFailure } from './verification-outcome';
import { verifyAction as expressVerifyAction, aimErrorHandler, AIMRequest } from './express';
import { verifyAction as fastifyVerifyAction, verifyRoute } from './fastify';

describe('parseAPIError still produces the type the middlewares must handle', () => {
  it('maps a wire 403 to AuthorizationError, not ActionDeniedError', () => {
    // Guards the premise. If this ever changes, the branch below is dead code
    // and this whole file would pass while testing nothing.
    const err = parseAPIError(403, { error: 'policy refused the action' });
    expect(err).toBeInstanceOf(AuthorizationError);
    expect(err).not.toBeInstanceOf(ActionDeniedError);
  });
});

describe('classifyVerificationFailure', () => {
  it('treats a wire 403 as a denial', () => {
    const failure = classifyVerificationFailure(
      parseAPIError(403, { error: 'policy refused the action' })
    );
    expect(failure).not.toBeNull();
    expect(failure!.status).toBe(403);
    expect(failure!.body.error).toBe('Action denied');
  });

  it('treats an explicit ActionDeniedError as a denial and keeps its detail', () => {
    const failure = classifyVerificationFailure(
      new ActionDeniedError('db:delete', 'blocked by policy', 0.4)
    );
    expect(failure!.status).toBe(403);
    expect(failure!.body).toMatchObject({
      error: 'Action denied',
      action: 'db:delete',
      reason: 'blocked by policy',
      trustScore: 0.4,
    });
  });

  it('treats a 401 as an authentication problem, not a denial', () => {
    const failure = classifyVerificationFailure(new AuthenticationError('bad key'));
    expect(failure!.status).toBe(401);
  });

  it('returns null for anything that is not a verification outcome', () => {
    // A transport failure is not a decision, and must not be reported as one.
    // Returning null re-throws it to the host framework rather than dressing it
    // up as a 403.
    expect(classifyVerificationFailure(new NetworkError('connection refused'))).toBeNull();
    expect(classifyVerificationFailure(new Error('something else'))).toBeNull();
  });
});

function clientThrowing(error: unknown) {
  return {
    verifyAction: vi.fn().mockRejectedValue(error),
    getAgent: vi.fn().mockResolvedValue(null),
  };
}

describe('express verifyAction', () => {
  let req: Partial<AIMRequest>;
  let res: Partial<Response>;
  let next: NextFunction;

  beforeEach(() => {
    res = { status: vi.fn().mockReturnThis(), json: vi.fn() };
    next = vi.fn();
  });

  it('answers 403 on a server-side denial instead of falling through to next(error)', async () => {
    const error = parseAPIError(403, { error: 'policy refused the action' });
    req = {
      path: '/api/x',
      method: 'POST',
      query: {},
      aim: { client: clientThrowing(error) as never, verified: false },
    };

    await expressVerifyAction('db:delete')(req as AIMRequest, res as Response, next);

    expect(res.status).toHaveBeenCalledWith(403);
    expect(next).not.toHaveBeenCalled();
  });

  it('never marks the request verified when verification failed', async () => {
    const error = parseAPIError(403, { error: 'policy refused' });
    req = {
      path: '/api/x',
      method: 'POST',
      query: {},
      aim: { client: clientThrowing(error) as never, verified: false },
    };

    await expressVerifyAction('db:delete')(req as AIMRequest, res as Response, next);

    expect(req.aim!.verified).toBe(false);
  });

  it('passes a transport failure to next(error) rather than reporting a denial', async () => {
    req = {
      path: '/api/x',
      method: 'POST',
      query: {},
      aim: { client: clientThrowing(new NetworkError('refused')) as never, verified: false },
    };

    await expressVerifyAction('db:delete')(req as AIMRequest, res as Response, next);

    expect(res.status).not.toHaveBeenCalled();
    expect(next).toHaveBeenCalled();
  });
});

describe('aimErrorHandler', () => {
  it('answers 403 on a server-side denial', () => {
    const res = { status: vi.fn().mockReturnThis(), json: vi.fn() };
    const next = vi.fn();
    aimErrorHandler(
      parseAPIError(403, { error: 'policy refused' }) as Error,
      {} as never,
      res as never,
      next as never
    );
    expect(res.status).toHaveBeenCalledWith(403);
    expect(next).not.toHaveBeenCalled();
  });
});

describe('fastify', () => {
  async function statusFor(
    hookFactory: () => ReturnType<typeof fastifyVerifyAction>,
    error: unknown
  ) {
    const app = Fastify();
    const client = clientThrowing(error);
    app.addHook('onRequest', async (request) => {
      request.aim = { client: client as never, verified: false };
    });
    app.post('/api/x', { preHandler: hookFactory() }, async () => ({ ok: true }));
    const res = await app.inject({ method: 'POST', url: '/api/x' });
    await app.close();
    return res;
  }

  it('verifyAction answers 403 on a server-side denial', async () => {
    const res = await statusFor(
      () => fastifyVerifyAction('db:delete'),
      parseAPIError(403, { error: 'policy refused the action' })
    );
    expect(res.statusCode).toBe(403);
    expect(res.json().error).toBe('Action denied');
  });

  it('verifyRoute answers 403 on a server-side denial too', async () => {
    // verifyRoute used to carry its own copy of the catch ladder. It now
    // delegates, so it cannot drift from verifyAction.
    const res = await statusFor(
      () => verifyRoute('db'),
      parseAPIError(403, { error: 'policy refused the action' })
    );
    expect(res.statusCode).toBe(403);
    expect(res.json().error).toBe('Action denied');
  });

  it('does not run the route handler when verification fails', async () => {
    const app = Fastify();
    const handler = vi.fn().mockResolvedValue({ ok: true });
    app.addHook('onRequest', async (request) => {
      request.aim = {
        client: clientThrowing(parseAPIError(403, { error: 'refused' })) as never,
        verified: false,
      };
    });
    app.post('/api/x', { preHandler: fastifyVerifyAction('db:delete') }, handler);
    await app.inject({ method: 'POST', url: '/api/x' });
    await app.close();
    expect(handler).not.toHaveBeenCalled();
  });
});

describe('direction agreement across integrations', () => {
  const cases: Array<{ name: string; error: unknown; expected: number | null }> = [
    { name: 'explicit denial', error: new ActionDeniedError('db:delete', 'nope'), expected: 403 },
    { name: 'wire 403', error: parseAPIError(403, { error: 'refused' }), expected: 403 },
    { name: 'wire 401', error: parseAPIError(401, { error: 'bad key' }), expected: 401 },
    { name: 'transport failure', error: new NetworkError('refused'), expected: null },
  ];

  it.each(cases)('express and fastify agree on $name', async ({ error, expected }) => {
    // express
    const res = { status: vi.fn().mockReturnThis(), json: vi.fn() };
    const next = vi.fn();
    const req = {
      path: '/api/x',
      method: 'POST',
      query: {},
      aim: { client: clientThrowing(error) as never, verified: false },
    };
    await expressVerifyAction('db:delete')(req as never, res as never, next as never);
    const expressStatus = (res.status as ReturnType<typeof vi.fn>).mock.calls[0]?.[0] ?? null;

    // fastify
    const app = Fastify();
    app.addHook('onRequest', async (request) => {
      request.aim = { client: clientThrowing(error) as never, verified: false };
    });
    app.post('/api/x', { preHandler: fastifyVerifyAction('db:delete') }, async () => ({ ok: true }));
    const injected = await app.inject({ method: 'POST', url: '/api/x' });
    await app.close();
    // A rethrown error becomes fastify's own 500; express hands it to next().
    const fastifyStatus = injected.statusCode === 500 ? null : injected.statusCode;

    expect(expressStatus).toBe(expected);
    expect(fastifyStatus).toBe(expected);
  });
});
