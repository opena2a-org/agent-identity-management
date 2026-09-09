/**
 * AIM-12 item 8: a verified request should know WHO was verified. The express
 * and fastify verifyAction hooks set `aim.verified` and `aim.trustScore` from
 * the verification result but never `aim.agentId`, so a handler behind the
 * hook could not attribute the request to the verified agent unless the
 * separate base middleware had already resolved it (which requires ambient
 * credentials and a network round trip).
 */
import { it, expect, vi } from 'vitest';
import type { Response, NextFunction } from 'express';
import type { FastifyRequest, FastifyReply } from 'fastify';
import { verifyAction as expressVerifyAction, type AIMRequest } from './express';
import { verifyAction as fastifyVerifyAction } from './fastify';

const RESULT = {
  verified: true,
  agentId: 'agent-42',
  agentName: 'did:aim:agent-42',
  trustScore: 0.9,
  riskLevel: 'low',
  actionAllowed: true,
  timestamp: '2026-09-09T00:00:00.000Z',
};

it('AIM-12.AC3 item 8: express verifyAction sets req.aim.agentId from the verification result', async () => {
  const client = { verifyAction: vi.fn(async () => ({ ...RESULT })) };
  const req = {
    aim: { client, verified: false },
    path: '/x',
    method: 'GET',
    query: {},
    ip: '127.0.0.1',
  } as unknown as AIMRequest;
  const next = vi.fn() as NextFunction;
  const res = { status: vi.fn().mockReturnThis(), json: vi.fn() } as unknown as Response;

  await expressVerifyAction('thing:read')(req, res, next);

  expect(next).toHaveBeenCalled();
  expect(req.aim?.verified).toBe(true);
  expect(req.aim?.trustScore).toBe(0.9);
  expect(req.aim?.agentId).toBe('agent-42');
});

it('AIM-12.AC3 item 8: fastify verifyAction sets request.aim.agentId from the verification result', async () => {
  const client = { verifyAction: vi.fn(async () => ({ ...RESULT })) };
  const request = {
    aim: { client, verified: false },
    url: '/x',
    method: 'GET',
    query: {},
    ip: '127.0.0.1',
  } as unknown as FastifyRequest;
  const reply = { status: vi.fn().mockReturnThis(), send: vi.fn() } as unknown as FastifyReply;

  await fastifyVerifyAction('thing:read')(request, reply);

  expect(request.aim?.verified).toBe(true);
  expect(request.aim?.agentId).toBe('agent-42');
});

it('a result without an agentId leaves the field unset rather than clobbering it with undefined-as-string', async () => {
  const { agentId: _dropped, ...withoutAgentId } = RESULT;
  const client = { verifyAction: vi.fn(async () => withoutAgentId) };
  const req = {
    aim: { client, verified: false, agentId: 'from-base-middleware' },
    path: '/x',
    method: 'GET',
    query: {},
    ip: '127.0.0.1',
  } as unknown as AIMRequest;
  const next = vi.fn() as NextFunction;
  const res = { status: vi.fn().mockReturnThis(), json: vi.fn() } as unknown as Response;

  await expressVerifyAction('thing:read')(req, res, next);

  expect(req.aim?.agentId).toBe('from-base-middleware');
});
