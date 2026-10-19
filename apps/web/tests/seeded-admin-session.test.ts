import { describe, expect, it } from 'vitest';
import {
  intermediatePasswordFor,
  prepareSeededAdmin,
  type SeededAdminRequester,
} from './e2e/fixtures/seeded-admin-session';

// prepareSeededAdmin against a model of the API's password-change contract: a
// sign-in answers the row's forcePasswordChange; a session whose sign-in second
// is before the first whole second after the last change is answered 401; while
// the change is required every route but GET /api/v1/auth/me and
// POST /api/v1/auth/change-password answers 403 passwordChangeRequired; a change
// whose current password does not match answers 400; the change's answer carries
// a pair minted at the boundary. The fixture's workers sign in with the seeded
// password afterwards, so each case checks that sign-in and the calls made.

const EMAIL = 'admin@example.test';
const SEEDED = 'FAKE-seeded-Pw1';
const INTERMEDIATE = intermediatePasswordFor(SEEDED);

type Answer = { status: number; body: unknown };

function label(password: unknown): string {
  return password === SEEDED ? 'seeded' : password === INTERMEDIATE ? 'intermediate' : 'other';
}

class FakeAim {
  password: string;
  forced: boolean;
  /** Instant of the last change, in seconds with a fraction; null before the first. */
  changedAt: number | null = null;
  /** The server's clock, in seconds with a fraction. */
  clock = 100.4;
  pairOnChange = true;
  loginStatus = 200;
  meStatus = 200;
  /** Whether each wait advances the clock; a false models a wait the server's clock did not follow. */
  waitAdvances: boolean[] = [];
  beforeChange: (() => void) | null = null;
  calls: string[] = [];
  failedSignIns = 0;
  waits: number[] = [];
  private tokens = new Map<string, number>();
  private minted = 0;

  constructor(password: string, forced: boolean) {
    this.password = password;
    this.forced = forced;
  }

  wait = async (ms: number): Promise<void> => {
    const advances = this.waitAdvances.length > this.waits.length ? this.waitAdvances[this.waits.length] : true;
    this.waits.push(ms);
    if (advances) this.clock += ms / 1000;
  };

  private mint(signedInAt: number): { accessToken: string; refreshToken: string } {
    const n = ++this.minted;
    this.tokens.set(`access-${n}`, signedInAt);
    return { accessToken: `access-${n}`, refreshToken: `refresh-${n}` };
  }

  private refusal(headers?: Record<string, string>): Answer | null {
    const token = headers?.Authorization?.replace(/^Bearer /, '');
    const signedInAt = token === undefined ? undefined : this.tokens.get(token);
    if (signedInAt === undefined) return { status: 401, body: { error: 'Invalid or expired token' } };
    if (this.changedAt !== null && signedInAt < Math.floor(this.changedAt) + 1) {
      return { status: 401, body: { error: "Token has been revoked: the account's password changed after this sign-in" } };
    }
    return null;
  }

  answer(method: 'GET' | 'POST', url: string, data?: Record<string, unknown>, headers?: Record<string, string>): Answer {
    if (method === 'POST' && url === '/api/v1/auth/login/local') {
      this.calls.push(`login ${label(data?.password)}`);
      if (this.loginStatus !== 200) return { status: this.loginStatus, body: { error: 'not today' } };
      if (data?.email !== EMAIL || data?.password !== this.password) {
        this.failedSignIns++;
        return { status: 401, body: { error: 'Invalid email or password' } };
      }
      const pair = this.mint(Math.floor(this.clock));
      return { status: 200, body: { ...pair, user: { email: EMAIL, forcePasswordChange: this.forced } } };
    }
    const refused = this.refusal(headers);
    if (method === 'POST' && url === '/api/v1/auth/change-password') {
      this.beforeChange?.();
      const again = refused ?? this.refusal(headers);
      if (again) {
        this.calls.push('change refused');
        return again;
      }
      this.calls.push(`change ${label(data?.currentPassword)} -> ${label(data?.newPassword)}`);
      if (data?.currentPassword !== this.password) return { status: 400, body: { error: 'currentPassword is incorrect' } };
      // bcrypt hashes at most 72 bytes; the server answers a longer one 500.
      if (Buffer.byteLength(String(data?.newPassword), 'utf8') > 72) {
        return { status: 500, body: { error: 'An internal error occurred. Please try again later.' } };
      }
      this.password = String(data?.newPassword);
      this.forced = false;
      this.changedAt = this.clock;
      const pair = this.pairOnChange ? this.mint(Math.floor(this.changedAt) + 1) : {};
      return { status: 200, body: { message: 'Password changed successfully', ...pair } };
    }
    if (refused) {
      this.calls.push(`${method} ${url} refused`);
      return refused;
    }
    if (method === 'GET' && url === '/api/v1/auth/me') {
      this.calls.push(this.meStatus === 200 ? 'me' : `me ${this.meStatus}`);
      return { status: this.meStatus, body: this.meStatus === 200 ? { email: EMAIL } : { error: 'refused' } };
    }
    this.calls.push(`${method} ${url}`);
    return this.forced
      ? { status: 403, body: { code: 'passwordChangeRequired', error: 'The account\'s password must be changed first' } }
      : { status: 404, body: { error: 'no such route' } };
  }

  requester(): SeededAdminRequester {
    const respond = ({ status, body }: Answer) => ({
      ok: () => status >= 200 && status < 300,
      status: () => status,
      json: async () => body,
      text: async () => JSON.stringify(body),
    });
    const fake = {
      post: async (url: string, options?: { data?: unknown; headers?: Record<string, string> }) =>
        respond(this.answer('POST', url, options?.data as Record<string, unknown> | undefined, options?.headers)),
      get: async (url: string, options?: { headers?: Record<string, string> }) =>
        respond(this.answer('GET', url, undefined, options?.headers)),
    };
    return fake as unknown as SeededAdminRequester;
  }
}

function prepare(aim: FakeAim, options: { seededPassword?: string; attempts?: number } = {}) {
  return prepareSeededAdmin(aim.requester(), {
    email: EMAIL,
    seededPassword: options.seededPassword ?? SEEDED,
    attempts: options.attempts,
    wait: aim.wait,
  });
}

describe('intermediatePasswordFor', () => {
  // The server's rule (apps/backend/internal/infrastructure/auth/password.go):
  // 8 or more bytes with an upper-case letter, a lower-case letter, a digit and
  // a special character; bcrypt then hashes at most 72 bytes.
  const satisfiesServerRule = (password: string) =>
    password.length >= 8 &&
    /[A-Z]/.test(password) &&
    /[a-z]/.test(password) &&
    /[0-9]/.test(password) &&
    /[!@#$%^&*()_+\-=[\]{};':"\\|,.<>/?]/.test(password);

  const seededOfLength = (bytes: number) => `Aa1-${'x'.repeat(bytes - 4)}`;

  it('derives a password that differs from the seeded one and satisfies the server rule', () => {
    expect(INTERMEDIATE).not.toBe(SEEDED);
    expect(satisfiesServerRule(INTERMEDIATE)).toBe(true);
    expect(intermediatePasswordFor(SEEDED)).toBe(INTERMEDIATE);
  });

  it('stays within 72 bytes for seeded passwords of 8 to 72 bytes and beyond', () => {
    for (const bytes of [8, 9, 63, 64, 65, 71, 72, 73, 200]) {
      const seeded = seededOfLength(bytes);
      const intermediate = intermediatePasswordFor(seeded);
      expect(Buffer.byteLength(intermediate, 'utf8')).toBeLessThanOrEqual(72);
      expect(intermediate).not.toBe(seeded);
      expect(satisfiesServerRule(intermediate)).toBe(true);
    }
    const multiByte = `Ä1-${'ü'.repeat(40)}a`;
    expect(Buffer.byteLength(intermediatePasswordFor(multiByte), 'utf8')).toBeLessThanOrEqual(72);
  });

  it('derives different passwords for different seeded ones', () => {
    expect(intermediatePasswordFor(seededOfLength(64))).not.toBe(intermediatePasswordFor(seededOfLength(65)));
  });
});

describe('prepareSeededAdmin', () => {
  it('completes the required change, sets the seeded password back, and the seeded sign-in is admitted', async () => {
    const aim = new FakeAim(SEEDED, true);
    await expect(prepare(aim)).resolves.toEqual({ restored: true });
    expect(aim.forced).toBe(false);
    expect(aim.password).toBe(SEEDED);
    expect(aim.calls).toEqual([
      'login seeded',
      'change seeded -> intermediate',
      'change intermediate -> seeded',
      'login seeded',
      'me',
    ]);
    expect(aim.failedSignIns).toBe(0);
    // A token minted in the second of a change is refused: the sign-in after
    // the change waits for the next whole second.
    expect(aim.waits).toEqual([1100]);
  });

  it('completes the required change for a seeded password of 64 to 72 bytes', async () => {
    for (const bytes of [64, 72]) {
      const seeded = `FAKE-Pw1-${'x'.repeat(bytes - 9)}`;
      expect(seeded.length).toBe(bytes);
      const aim = new FakeAim(seeded, true);
      await expect(prepare(aim, { seededPassword: seeded })).resolves.toEqual({ restored: true });
      expect(aim.forced).toBe(false);
      expect(aim.password).toBe(seeded);
      expect(aim.calls).toEqual(['login other', 'change other -> other', 'change other -> other', 'login other', 'me']);
    }
  });

  it('changes nothing when the account holds the seeded password and requires no change', async () => {
    const aim = new FakeAim(SEEDED, false);
    await expect(prepare(aim)).resolves.toEqual({ restored: false });
    expect(aim.calls).toEqual(['login seeded', 'me']);
    expect(aim.failedSignIns).toBe(0);
    expect(aim.waits).toEqual([]);
  });

  it('restores the seeded password after a run interrupted between the two changes, failing one sign-in', async () => {
    const aim = new FakeAim(INTERMEDIATE, false);
    await expect(prepare(aim)).resolves.toEqual({ restored: true });
    expect(aim.password).toBe(SEEDED);
    expect(aim.calls).toEqual([
      'login seeded',
      'login intermediate',
      'change intermediate -> seeded',
      'login seeded',
      'me',
    ]);
    expect(aim.failedSignIns).toBe(1);
  });

  it('repeats the round with the other password when a change made in between ends its session', async () => {
    const aim = new FakeAim(SEEDED, true);
    let interposed = false;
    aim.beforeChange = () => {
      if (interposed) return;
      interposed = true;
      // Another run completes the change after this one signed in.
      aim.password = INTERMEDIATE;
      aim.forced = false;
      aim.changedAt = aim.clock;
    };
    await expect(prepare(aim)).resolves.toEqual({ restored: true });
    expect(aim.password).toBe(SEEDED);
    expect(aim.calls).toEqual([
      'login seeded',
      'change refused',
      'login seeded',
      'login intermediate',
      'change intermediate -> seeded',
      'login seeded',
      'me',
    ]);
    expect(aim.waits).toEqual([1100, 1100]);
  });

  it('signs in with the password just set when a change answers without a pair', async () => {
    const aim = new FakeAim(SEEDED, true);
    aim.pairOnChange = false;
    await expect(prepare(aim)).resolves.toEqual({ restored: true });
    expect(aim.calls).toEqual([
      'login seeded',
      'change seeded -> intermediate',
      'login intermediate',
      'change intermediate -> seeded',
      'login seeded',
      'me',
    ]);
    // Both sign-ins after a change wait for the next whole second.
    expect(aim.waits).toEqual([1100, 1100]);
  });

  it('signs in again after the next whole second when the sign-in after the change is still refused', async () => {
    const aim = new FakeAim(SEEDED, true);
    aim.waitAdvances = [false, true];
    await expect(prepare(aim)).resolves.toEqual({ restored: true });
    expect(aim.calls).toEqual([
      'login seeded',
      'change seeded -> intermediate',
      'change intermediate -> seeded',
      'login seeded',
      'GET /api/v1/auth/me refused',
      'login seeded',
      'me',
    ]);
    expect(aim.waits).toEqual([1100, 1100]);
  });

  it('reports a sign-in refused for another reason than the password with its status', async () => {
    const aim = new FakeAim(SEEDED, true);
    aim.loginStatus = 500;
    await expect(prepare(aim)).rejects.toThrow(/login\/local failed: 500/);
  });

  it('names the variables when no seeded password is set', async () => {
    const aim = new FakeAim(SEEDED, true);
    await expect(prepare(aim, { seededPassword: '' })).rejects.toThrow(/AIM_TEST_ADMIN_PASSWORD/);
    expect(aim.calls).toEqual([]);
  });

  it('says which variable to set when neither password signs in', async () => {
    const aim = new FakeAim('FAKE-other-Pw1', false);
    await expect(prepare(aim)).rejects.toThrow(/Set AIM_TEST_ADMIN_PASSWORD/);
    expect(aim.failedSignIns).toBe(2);
  });

  it('gives up after the rounds it is given and names the last refusal', async () => {
    const aim = new FakeAim(SEEDED, false);
    aim.meStatus = 403;
    await expect(prepare(aim, { attempts: 2 })).rejects.toThrow(/after 2 round\(s\); last refusal: auth\/me answered 403/);
    expect(aim.calls).toEqual(['login seeded', 'me 403', 'login seeded', 'me 403']);
  });
});
