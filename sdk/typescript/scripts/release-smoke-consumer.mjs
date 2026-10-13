// The consumer side of release-smoke.sh: one verifyAction call from a project
// that installed @opena2a/aim-sdk from its tarball, the way a user's project
// does.
//
// release-smoke.sh copies this file into the consumer project and runs it
// there, so the import below resolves to the installed package and not to the
// sources in this repository. The agent identity comes from the environment
// the SDK reads (AIM_AGENT_ID, AIM_PRIVATE_KEY, AIM_PUBLIC_KEY,
// AIM_ORGANIZATION_ID); the server from AIM_BASE_URL.
//
// It prints one JSON line on standard output and never the key:
//   {"outcome":"decision","decision":"allowed|denied|pending","status":...,"eventId":...,"sdkVersion":...}
//   {"outcome":"error","errorName":...,"statusCode":...,"message":...,"sdkVersion":...}
// and exits 0 for a decision, 1 for anything else.

import { createRequire } from 'node:module';
import * as sdk from '@opena2a/aim-sdk';

const require = createRequire(import.meta.url);
const { version: sdkVersion } = require('@opena2a/aim-sdk/package.json');

const action = process.env.SMOKE_ACTION || 'db:read';
const resource = process.env.SMOKE_RESOURCE || 'release-smoke';

function report(line, code) {
  process.stdout.write(`${JSON.stringify({ ...line, sdkVersion })}\n`);
  process.exit(code);
}

const client = new sdk.AIMClient({ baseUrl: process.env.AIM_BASE_URL, autoRegister: false });

try {
  const result = await client.verifyAction({ action, resource });
  report(
    {
      outcome: 'decision',
      decision: result.actionAllowed ? 'allowed' : 'denied',
      status: result.status ?? null,
      eventId: result.eventId ?? null,
    },
    0
  );
} catch (error) {
  // A denied or pending action is a decision the server made: the client
  // raises it as ActionDeniedError carrying the server's status and event ID.
  if (sdk.ActionDeniedError && error instanceof sdk.ActionDeniedError) {
    const status = error.details?.status ?? null;
    report(
      {
        outcome: 'decision',
        decision: status === 'pending' ? 'pending' : 'denied',
        status,
        eventId: error.details?.eventId ?? null,
      },
      0
    );
  }
  report(
    {
      outcome: 'error',
      errorName: error?.name ?? typeof error,
      statusCode: error?.statusCode ?? null,
      message: String(error?.message ?? error).slice(0, 300),
    },
    1
  );
}
