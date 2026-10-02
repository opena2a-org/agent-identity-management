/**
 * How the Express middleware and the Fastify plugin obtain their AIMClient.
 */

import { AIMClient } from '../client/AIMClient';
import { ConfigurationError } from '../exceptions';
import type { AgentCredentials, AIMClientConfig } from '../types';

/** Options both integrations accept for their client. */
export interface IntegrationClientOptions extends AIMClientConfig {
  /** An existing client to use as-is, instead of constructing one. */
  client?: AIMClient;
  /**
   * Registered-agent credentials for the client the integration constructs.
   * Without these, or the four AIM_AGENT_ID / AIM_PRIVATE_KEY / AIM_PUBLIC_KEY
   * / AIM_ORGANIZATION_ID environment variables, the client has no agent
   * identity and every verified route answers 401 (#449).
   */
  credentials?: AgentCredentials;
}

/**
 * The client an integration uses: `options.client` when given, else a new
 * client from the options with `options.credentials` applied. Passing both is
 * refused: credentials cannot be applied to a caller-owned client silently.
 */
export function clientFromOptions(options: IntegrationClientOptions): AIMClient {
  if (options.client && options.credentials) {
    throw new ConfigurationError(
      'Pass either `client` or `credentials`, not both: set the credentials on your client with client.setCredentials().',
    );
  }
  if (options.client) return options.client;
  const client = new AIMClient(options);
  if (options.credentials) client.setCredentials(options.credentials);
  return client;
}
