import type { APIRequestContext } from '@playwright/test';

// The admin that cmd/bootstrap --default seeds must change its default password
// before the API admits its session: while users.force_password_change is set,
// the auth middleware answers every route but GET /api/v1/auth/me and
// POST /api/v1/auth/change-password with 403 passwordChangeRequired, and a
// password change ends every session signed in before it.
//
// prepareSeededAdmin, which tests/e2e/global-setup.ts runs once before the
// workers start, completes that change and then sets the password back to the
// seeded one, so the workers keep signing in with AIM_TEST_ADMIN_PASSWORD and a
// rerun against the same database finds nothing to do. The account locks after
// five failed sign-ins (domain.DefaultAuthFailureConfig), so each round signs in
// with the seeded password first and falls back to the intermediate one only
// when a previous run was interrupted between the two changes.

export type Session = { accessToken: string; refreshToken: string };

export type SeededAdminRequester = Pick<APIRequestContext, 'get' | 'post'>;

export interface SeededAdmin {
  email: string;
  /** The password cmd/bootstrap --default seeded (AIM_TEST_ADMIN_PASSWORD). */
  seededPassword: string;
}

export interface PrepareSeededAdminOptions extends SeededAdmin {
  /** The password the account holds between the two changes; derived from the seeded one when unset. */
  intermediatePassword?: string;
  /** Rounds before giving up; each refused round waits for the next whole second first. */
  attempts?: number;
  /** Waits ms; replaced in tests. */
  wait?: (ms: number) => Promise<void>;
}

export interface PrepareSeededAdminResult {
  /** True when this run completed the required change (and set the seeded password back). */
  restored: boolean;
}

const LOGIN_ROUTE = '/api/v1/auth/login/local';
const CHANGE_PASSWORD_ROUTE = '/api/v1/auth/change-password';
const ME_ROUTE = '/api/v1/auth/me';

// Token times are whole seconds and a token minted in the second of a change
// is refused, so a round that changed the password waits this long before it
// signs in again.
const NEXT_WHOLE_SECOND_MS = 1100;

export function seededAdminFromEnv(): SeededAdmin {
  return {
    email: process.env.AIM_TEST_ADMIN_EMAIL ?? 'admin@opena2a.org',
    seededPassword: process.env.AIM_TEST_ADMIN_PASSWORD ?? process.env.DEFAULT_ADMIN_PASSWORD ?? '',
  };
}

// The seeded password satisfies the server's rule (8 or more characters with an
// upper-case letter, a lower-case letter, a digit and a special character), so
// the seeded password followed by this suffix does too, and it differs.
export function intermediatePasswordFor(seededPassword: string): string {
  return `${seededPassword}-Changed1`;
}

function bearer(session: Session): { Authorization: string } {
  return { Authorization: `Bearer ${session.accessToken}` };
}

function sessionOf(body: unknown): Session | null {
  const pair = body as { accessToken?: unknown; refreshToken?: unknown } | null;
  return typeof pair?.accessToken === 'string' && typeof pair.refreshToken === 'string'
    ? { accessToken: pair.accessToken, refreshToken: pair.refreshToken }
    : null;
}

type SignIn = { session: Session; password: string; forcePasswordChange: boolean };

// Signs in with the first password the API accepts. A 401 moves to the next
// password; any other refusal is reported with its status and body.
async function signInWith(request: SeededAdminRequester, email: string, passwords: string[]): Promise<SignIn> {
  for (const password of passwords) {
    const res = await request.post(LOGIN_ROUTE, { data: { email, password } });
    if (res.status() === 401) continue;
    if (!res.ok()) {
      throw new Error(`login/local failed: ${res.status()} ${await res.text()}`);
    }
    const body = (await res.json()) as { user?: { forcePasswordChange?: unknown } };
    const session = sessionOf(body);
    if (!session) {
      throw new Error('login/local answered 200 without accessToken and refreshToken');
    }
    return { session, password, forcePasswordChange: body.user?.forcePasswordChange === true };
  }
  throw new Error(
    `The seeded admin ${email} signs in with none of the ${passwords.length} password(s) tried, or the account is ` +
      'locked after failed sign-ins. Set AIM_TEST_ADMIN_PASSWORD to the password the account holds.',
  );
}

type Change = { ok: true; session: Session | null } | { ok: false; refusal: string };

// Changes the password through the authenticated route. A 400, 401 or 403 is a
// refusal the caller repeats the round for (the current password no longer
// matches, or the session was ended by a change made in between); any other
// failure is thrown.
async function changePassword(
  request: SeededAdminRequester,
  session: Session,
  currentPassword: string,
  newPassword: string,
): Promise<Change> {
  const res = await request.post(CHANGE_PASSWORD_ROUTE, {
    headers: bearer(session),
    data: { currentPassword, newPassword },
  });
  if (res.ok()) {
    return { ok: true, session: sessionOf(await res.json()) };
  }
  if ([400, 401, 403].includes(res.status())) {
    return { ok: false, refusal: `change-password answered ${res.status()} ${await res.text()}` };
  }
  throw new Error(`change-password failed: ${res.status()} ${await res.text()}`);
}

export async function prepareSeededAdmin(
  request: SeededAdminRequester,
  options: PrepareSeededAdminOptions,
): Promise<PrepareSeededAdminResult> {
  const { email, seededPassword } = options;
  if (!seededPassword) {
    throw new Error(
      'AIM_TEST_ADMIN_PASSWORD (or DEFAULT_ADMIN_PASSWORD) must be set to the seeded admin password.',
    );
  }
  const intermediate = options.intermediatePassword ?? intermediatePasswordFor(seededPassword);
  const attempts = options.attempts ?? 4;
  const wait = options.wait ?? ((ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms)));

  let restored = false;
  let refusal = 'no sign-in was tried';
  for (let round = 1; round <= attempts; round++) {
    if (round > 1) await wait(NEXT_WHOLE_SECOND_MS);

    const signedIn = await signInWith(request, email, [seededPassword, intermediate]);
    let session: Session | null = signedIn.session;

    if (signedIn.forcePasswordChange || signedIn.password !== seededPassword) {
      if (signedIn.password === seededPassword) {
        // The required change: to the intermediate password, which clears the
        // requirement and admits the session the answer carries everywhere.
        const first = await changePassword(request, session, seededPassword, intermediate);
        if (!first.ok) {
          refusal = first.refusal;
          continue;
        }
        if (!first.session) {
          // No pair in the answer: sign in with the password just set, after
          // the second of the change, which refuses the tokens minted in it.
          await wait(NEXT_WHOLE_SECOND_MS);
          session = (await signInWith(request, email, [intermediate])).session;
        } else {
          session = first.session;
        }
      }
      // Back to the seeded password, which the workers and the next run use.
      const back = await changePassword(request, session, intermediate, seededPassword);
      if (!back.ok) {
        refusal = back.refusal;
        continue;
      }
      restored = true;
      await wait(NEXT_WHOLE_SECOND_MS);
      // What a worker does next: sign in with the seeded password.
      const proof = await signInWith(request, email, [seededPassword]);
      if (proof.forcePasswordChange) {
        refusal = 'the account still requires a password change after the change';
        continue;
      }
      session = proof.session;
    }

    const me = await request.get(ME_ROUTE, { headers: bearer(session) });
    if (me.ok()) return { restored };
    if ([401, 403].includes(me.status())) {
      refusal = `auth/me answered ${me.status()} ${await me.text()}`;
      continue;
    }
    throw new Error(`auth/me failed: ${me.status()} ${await me.text()}`);
  }
  throw new Error(`The seeded admin's session was not admitted after ${attempts} round(s); last refusal: ${refusal}`);
}
