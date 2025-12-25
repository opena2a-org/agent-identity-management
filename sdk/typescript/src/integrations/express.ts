/**
 * Express.js middleware for AIM SDK
 */

import type { Request, Response, NextFunction, RequestHandler } from 'express';
import { AIMClient } from '../client/AIMClient';
import type { AIMClientConfig, VerifyActionOptions } from '../types';
import { ActionDeniedError, AuthenticationError } from '../exceptions';

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
export interface AIMMiddlewareOptions extends AIMClientConfig {
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
 *
 * const app = express();
 * app.use(createAIMMiddleware({
 *   baseUrl: 'https://aim.example.com',
 *   apiKey: 'your-api-key',
 * }));
 * ```
 */
export function createAIMMiddleware(options: AIMMiddlewareOptions = {}): RequestHandler {
  const client = new AIMClient(options);
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

      next();
    } catch (error) {
      if (error instanceof ActionDeniedError) {
        res.status(403).json({
          error: 'Action denied',
          action: error.action,
          reason: error.reason,
          trustScore: error.trustScore,
        });
        return;
      }

      if (error instanceof AuthenticationError) {
        res.status(401).json({
          error: 'Authentication required',
          message: error.message,
        });
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
 * Error handler for AIM errors
 */
export function aimErrorHandler(
  error: Error,
  _req: Request,
  res: Response,
  next: NextFunction
): void {
  if (error instanceof ActionDeniedError) {
    res.status(403).json({
      error: 'Action denied',
      action: error.action,
      reason: error.reason,
      trustScore: error.trustScore,
    });
    return;
  }

  if (error instanceof AuthenticationError) {
    res.status(401).json({
      error: 'Authentication required',
      message: error.message,
    });
    return;
  }

  next(error);
}

export { AIMClient } from '../client/AIMClient';
export * from '../types';
export * from '../exceptions';
