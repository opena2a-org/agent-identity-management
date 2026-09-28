/**
 * Express.js middleware for AIM SDK
 */

import { STATUS_CODES } from 'node:http';
import type { Request, Response, NextFunction, RequestHandler } from 'express';
import { AIMClient } from '../client/AIMClient';
import { AIMError } from '../exceptions';
import type { VerifyActionOptions } from '../types';
import {
  classifyVerificationFailure,
  type VerificationFailureResponse,
} from './verification-outcome';
import { clientFromOptions, type IntegrationClientOptions } from './client-options';

/**
 * AIM context added to requests
 */
export interface AIMContext {
  client: AIMClient;
  agentId?: string;
  trustScore?: number;
  verified?: boolean;
}

/**
 * Extended Express Request with AIM context
 */
export interface AIMRequest extends Request {
  aim?: AIMContext;
}

/**
 * Middleware options
 */
export interface AIMMiddlewareOptions extends IntegrationClientOptions {
  /** Skip verification for certain paths */
  skipPaths?: string[];
  /** Custom error handler */
  onError?: (error: Error, req: AIMRequest, res: Response, next: NextFunction) => void;
  /** Transform request to action name */
  actionFromRequest?: (req: Request) => string;
}

/**
 * Create AIM middleware for Express
 *
 * @example
 * ```typescript
 * import express from 'express';
 * import { createAIMMiddleware } from '@opena2a/aim-sdk/express';
 * import { loadCredentialsFromFile } from '@opena2a/aim-sdk';
 *
 * const app = express();
 * app.use(createAIMMiddleware({
 *   baseUrl: 'https://aim.example.com',
 *   // A registered agent's identity; omit to read the four AIM_* env vars.
 *   credentials: await loadCredentialsFromFile('./agent-credentials.json'),
 * }));
 * ```
 */
export function createAIMMiddleware(options: AIMMiddlewareOptions = {}): RequestHandler {
  const client = clientFromOptions(options);
  const skipPaths = new Set(options.skipPaths ?? ['/health', '/ready', '/metrics']);

  return async (req: AIMRequest, res: Response, next: NextFunction): Promise<void> => {
    // Add AIM context to request
    req.aim = {
      client,
      verified: false,
    };

    // Skip certain paths
    if (skipPaths.has(req.path)) {
      return next();
    }

    try {
      // Get agent info if authenticated
      const agent = await client.getAgent();
      if (agent) {
        req.aim.agentId = agent.id;
        req.aim.trustScore = agent.trustScore;
      }

      next();
    } catch (error) {
      if (options.onError) {
        options.onError(error as Error, req, res, next);
      } else {
        next(error);
      }
    }
  };
}

/**
 * Create a verification middleware for specific actions
 *
 * @example
 * ```typescript
 * import { verifyAction } from '@opena2a/aim-sdk/express';
 *
 * app.post('/api/sensitive',
 *   verifyAction('api:write'),
 *   (req, res) => {
 *     // Action has been verified
 *     res.json({ success: true });
 *   }
 * );
 * ```
 */
export function verifyAction(
  action: string,
  options?: Partial<VerifyActionOptions>
): RequestHandler {
  return async (req: AIMRequest, res: Response, next: NextFunction): Promise<void> => {
    if (!req.aim?.client) {
      res.status(500).json({ error: 'AIM middleware not initialized' });
      return;
    }

    try {
      const result = await req.aim.client.verifyAction({
        action,
        resource: req.path,
        resourceType: req.method.toLowerCase(),
        context: {
          method: req.method,
          path: req.path,
          query: req.query,
          ip: req.ip,
          ...options?.context,
        },
        ...options,
      });

      req.aim.verified = true;
      req.aim.trustScore = result.trustScore;
      // The verified request knows WHO was verified; a result without an
      // agentId (older servers) leaves whatever the base middleware resolved.
      if (result.agentId) {
        req.aim.agentId = result.agentId;
      }

      next();
    } catch (error) {
      const failure = classifyVerificationFailure(error);
      if (failure) {
        res.status(failure.status).json(failure.body);
        return;
      }

      next(error);
    }
  };
}

/**
 * Create a router-level verification middleware
 * Verifies all requests to a router based on method
 */
export function verifyRouter(
  actionPrefix: string,
  options?: Partial<VerifyActionOptions>
): RequestHandler {
  return async (req: AIMRequest, res: Response, next: NextFunction): Promise<void> => {
    const methodAction: Record<string, string> = {
      GET: 'read',
      POST: 'create',
      PUT: 'update',
      PATCH: 'update',
      DELETE: 'delete',
    };

    const action = `${actionPrefix}:${methodAction[req.method] ?? 'access'}`;
    await verifyAction(action, options)(req, res, next);
  };
}

/**
 * The response Fastify's default error handler gives an SDK error that the AIM
 * plugin rethrows: `{ statusCode, code, error, message }`, with the error's own
 * status when it is a 4xx or 5xx and 500 otherwise. Returns null for an error
 * that did not come from the SDK.
 */
function sdkErrorResponse(error: unknown): VerificationFailureResponse | null {
  if (!(error instanceof AIMError)) {
    return null;
  }

  const status =
    error.statusCode !== undefined && error.statusCode >= 400 && error.statusCode <= 599
      ? error.statusCode
      : 500;

  return {
    status,
    body: {
      statusCode: status,
      code: error.code,
      error: STATUS_CODES[status] ?? 'Error',
      message: error.message,
    },
  };
}

/**
 * Error handler for AIM errors.
 *
 * Denials answer 403 and authentication failures 401, with the bodies
 * `verifyAction` sends. Every other SDK error (an upstream 5xx, a network
 * failure, a rate limit) is answered here too, in the shape the Fastify plugin
 * gives the same error. Passing those on to Express's default handler rendered
 * an HTML page carrying the stack trace and local file paths outside
 * `NODE_ENV=production`, and printed the stack to stderr (#450).
 *
 * Errors that did not come from the SDK still go to `next(error)`, so the
 * application's own handlers see them unchanged.
 */
export function aimErrorHandler(
  error: Error,
  _req: Request,
  res: Response,
  next: NextFunction
): void {
  // Express's own handler is the only one that can close a response that has
  // already started.
  if (res.headersSent) {
    next(error);
    return;
  }

  const failure = classifyVerificationFailure(error) ?? sdkErrorResponse(error);
  if (failure) {
    res.status(failure.status).json(failure.body);
    return;
  }

  next(error);
}

export { AIMClient } from '../client/AIMClient';
export * from '../types';
export * from '../exceptions';
