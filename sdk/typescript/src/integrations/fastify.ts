/**
 * Fastify plugin for AIM SDK
 */

import type {
  FastifyInstance,
  FastifyRequest,
  FastifyReply,
  FastifyPluginCallback,
  preHandlerAsyncHookHandler,
} from 'fastify';
import fp from 'fastify-plugin';
import { AIMClient } from '../client/AIMClient';
import type { AIMClientConfig, VerifyActionOptions } from '../types';
import { ActionDeniedError, AuthenticationError } from '../exceptions';

/**
 * Extended Fastify Request with AIM context
 */
declare module 'fastify' {
  interface FastifyRequest {
    aim?: {
      client: AIMClient;
      agentId?: string;
      trustScore?: number;
      verified?: boolean;
    };
  }
}

/**
 * Plugin options
 */
export interface AIMPluginOptions extends AIMClientConfig {
  /** Skip verification for certain paths */
  skipPaths?: string[];
}

const aimPluginCallback: FastifyPluginCallback<AIMPluginOptions> = (
  fastify: FastifyInstance,
  options: AIMPluginOptions,
  done: (err?: Error) => void
) => {
  const client = new AIMClient(options);
  const skipPaths = new Set(options.skipPaths ?? ['/health', '/ready', '/metrics']);

  // Decorate request with AIM context
  fastify.decorateRequest('aim', undefined);

  // Add hook to initialize AIM context
  fastify.addHook('onRequest', async (request: FastifyRequest) => {
    request.aim = {
      client,
      verified: false,
    };

    // Skip certain paths
    if (skipPaths.has(request.url)) {
      return;
    }

    // Get agent info if authenticated
    try {
      const agent = await client.getAgent();
      if (agent) {
        request.aim.agentId = agent.id;
        request.aim.trustScore = agent.trustScore;
      }
    } catch {
      // Continue without agent info
    }
  });

  // Add error handler
  fastify.setErrorHandler(async (error: Error, _request: FastifyRequest, reply: FastifyReply) => {
    if (error instanceof ActionDeniedError) {
      return reply.status(403).send({
        error: 'Action denied',
        action: error.action,
        reason: error.reason,
        trustScore: error.trustScore,
      });
    }

    if (error instanceof AuthenticationError) {
      return reply.status(401).send({
        error: 'Authentication required',
        message: error.message,
      });
    }

    throw error;
  });

  done();
};

/**
 * AIM Fastify Plugin
 *
 * Wrapped with fastify-plugin: without the skip-override wrap, everything the
 * plugin registers (decorateRequest, the onRequest hook, the error handler)
 * stays inside the plugin's own encapsulation context and never applies to
 * routes the user defines — verifyAction then fails every request with
 * "AIM plugin not initialized".
 *
 * @example
 * ```typescript
 * import Fastify from 'fastify';
 * import { aimPlugin } from '@opena2a/aim-sdk/fastify';
 *
 * const fastify = Fastify();
 *
 * await fastify.register(aimPlugin, {
 *   baseUrl: 'https://aim.example.com',
 *   apiKey: 'your-api-key',
 * });
 * ```
 */
export const aimPlugin = fp(aimPluginCallback, {
  fastify: '4.x || 5.x',
  name: '@opena2a/aim-sdk',
});

/**
 * Create a preHandler hook for action verification
 *
 * @example
 * ```typescript
 * fastify.post('/api/sensitive', {
 *   preHandler: verifyAction('api:write'),
 * }, async (request, reply) => {
 *   return { success: true };
 * });
 * ```
 */
export function verifyAction(
  action: string,
  options?: Partial<VerifyActionOptions>
): preHandlerAsyncHookHandler {
  return async (request: FastifyRequest, reply: FastifyReply): Promise<void> => {
    if (!request.aim?.client) {
      return reply.status(500).send({ error: 'AIM plugin not initialized' });
    }

    try {
      const result = await request.aim.client.verifyAction({
        action,
        resource: request.url,
        resourceType: request.method.toLowerCase(),
        context: {
          method: request.method,
          path: request.url,
          query: request.query,
          ip: request.ip,
          ...options?.context,
        },
        ...options,
      });

      request.aim.verified = true;
      request.aim.trustScore = result.trustScore;
    } catch (error) {
      if (error instanceof ActionDeniedError) {
        return reply.status(403).send({
          error: 'Action denied',
          action: error.action,
          reason: error.reason,
          trustScore: error.trustScore,
        });
      }

      if (error instanceof AuthenticationError) {
        return reply.status(401).send({
          error: 'Authentication required',
          message: error.message,
        });
      }

      throw error;
    }
  };
}

/**
 * Create a route-level verification hook based on HTTP method
 */
export function verifyRoute(
  actionPrefix: string,
  options?: Partial<VerifyActionOptions>
): preHandlerAsyncHookHandler {
  const methodAction: Record<string, string> = {
    GET: 'read',
    POST: 'create',
    PUT: 'update',
    PATCH: 'update',
    DELETE: 'delete',
  };

  return async (request: FastifyRequest, reply: FastifyReply): Promise<void> => {
    if (!request.aim?.client) {
      return reply.status(500).send({ error: 'AIM plugin not initialized' });
    }

    const action = `${actionPrefix}:${methodAction[request.method] ?? 'access'}`;

    try {
      const result = await request.aim.client.verifyAction({
        action,
        resource: request.url,
        resourceType: request.method.toLowerCase(),
        context: {
          method: request.method,
          path: request.url,
          query: request.query,
          ip: request.ip,
          ...options?.context,
        },
        ...options,
      });

      request.aim.verified = true;
      request.aim.trustScore = result.trustScore;
    } catch (error) {
      if (error instanceof ActionDeniedError) {
        return reply.status(403).send({
          error: 'Action denied',
          action: error.action,
          reason: error.reason,
          trustScore: error.trustScore,
        });
      }

      if (error instanceof AuthenticationError) {
        return reply.status(401).send({
          error: 'Authentication required',
          message: error.message,
        });
      }

      throw error;
    }
  };
}

export { AIMClient } from '../client/AIMClient';
export * from '../types';
export * from '../exceptions';
