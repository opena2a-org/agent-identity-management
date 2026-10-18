import { request, type FullConfig } from '@playwright/test';
import { prepareSeededAdmin, seededAdminFromEnv } from './fixtures/seeded-admin-session';

// Runs once before the workers, after the web server is up. The admin that
// cmd/bootstrap --default seeds must change its default password before the API
// admits its session anywhere but GET /api/v1/auth/me and
// POST /api/v1/auth/change-password: every other route answers 403
// passwordChangeRequired. prepareSeededAdmin completes that change and sets the
// seeded password back, so the workers sign in with AIM_TEST_ADMIN_PASSWORD and
// their sessions are admitted.
export default async function globalSetup(config: FullConfig): Promise<void> {
  const baseURL = config.projects[0]?.use.baseURL ?? process.env.PLAYWRIGHT_BASE_URL ?? 'http://localhost:3000';
  const context = await request.newContext({ baseURL });
  try {
    const { restored } = await prepareSeededAdmin(context, seededAdminFromEnv());
    if (restored) {
      console.log(
        "[e2e] The seeded admin's required password change is complete; the account signs in with AIM_TEST_ADMIN_PASSWORD.",
      );
    }
  } finally {
    await context.dispose();
  }
}
