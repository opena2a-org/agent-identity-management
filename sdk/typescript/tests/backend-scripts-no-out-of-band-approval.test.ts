import { describe, it, expect } from 'vitest';
import { readFileSync, readdirSync } from 'fs';
import { join, relative, sep } from 'path';

/**
 * Nothing under apps/backend/scripts approves a registration or signs a token.
 *
 * The service approves a pending registration at
 * POST /api/v1/admin/registration-requests/:id/approve and writes an audit_logs
 * row when it does. apps/backend/scripts carried a Go program and three SQL
 * files that approved a registration by writing users and
 * user_registration_requests directly, and a Go program that signed a 24-hour
 * admin JWT with JWT_SECRET. None of them wrote an audit_logs row, none refused
 * a production database, and none was referenced by the build, the image or the
 * docs. They are removed. This cell keeps it that way:
 *
 *   - no file under apps/backend/scripts writes user_registration_requests;
 *   - no file under apps/backend/scripts signs a JWT;
 *   - a file under apps/backend/scripts that inserts into users refuses, before
 *     that insert, to run against a database holding a production-shaped
 *     organization, as the seed files do.
 *
 * The checks also run over literal fixtures in the removed files' shape, so they
 * keep refusing that shape after the tree stops carrying it.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');
const SCRIPTS_DIR = join(REPO_ROOT, 'apps', 'backend', 'scripts');

// ---------------------------------------------------------------------------
// The checks
// ---------------------------------------------------------------------------

const REGISTRATION_WRITE =
  /\b(?:UPDATE|INSERT\s+INTO|DELETE\s+FROM)\s+(?:public\.)?user_registration_requests\b/gi;

const TOKEN_SIGNING: RegExp[] = [
  // golang-jwt
  /\.SignedString\s*\(/g,
  /\bjwt\.(?:New|NewWithClaims)\s*\(/g,
  // jsonwebtoken (Node) and PyJWT
  /\bjwt\.(?:sign|encode)\s*\(/g,
];

const USERS_INSERT = /\bINSERT\s+INTO\s+(?:public\.)?users\b/gi;

/** The seed files' fence: a check of organizations that raises before any row is written. */
const PRODUCTION_FENCE = /FROM\s+organizations\b[\s\S]*?\bRAISE\s+EXCEPTION\b/i;

function lineOf(text: string, index: number): number {
  return text.slice(0, index).split('\n').length;
}

/** Returns one finding per out-of-band approval, token signing or unfenced users insert in `text`. */
function outOfBandWrites(text: string): string[] {
  const findings: string[] = [];
  for (const m of text.matchAll(REGISTRATION_WRITE)) {
    findings.push(`line ${lineOf(text, m.index as number)}: writes user_registration_requests`);
  }
  for (const pattern of TOKEN_SIGNING) {
    for (const m of text.matchAll(pattern)) {
      findings.push(`line ${lineOf(text, m.index as number)}: signs a JWT`);
    }
  }
  const firstInsert = [...text.matchAll(USERS_INSERT)][0];
  if (firstInsert && !PRODUCTION_FENCE.test(text.slice(0, firstInsert.index))) {
    findings.push(
      `line ${lineOf(text, firstInsert.index as number)}: inserts into users with no production-organization fence before it`,
    );
  }
  return findings.sort((a, b) => parseInt(a.slice(5), 10) - parseInt(b.slice(5), 10));
}

function filesUnder(directory: string): string[] {
  const found: string[] = [];
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) {
      found.push(...filesUnder(path));
    } else if (entry.isFile()) {
      found.push(path);
    }
  }
  return found.sort();
}

function repoRelative(absolute: string): string {
  return relative(REPO_ROOT, absolute).split(sep).join('/');
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

/** The removed Go approval program's writes, with placeholder values. */
const REMOVED_GO_APPROVAL = `package main

func approve(ctx context.Context, db *sql.DB) {
	_, err = db.ExecContext(ctx, \`
		INSERT INTO users (id, organization_id, email, name, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'active', $6, $7)
	\`, userID, orgID, email, fullName, role, now, now)

	_, err = db.ExecContext(ctx, \`
		UPDATE user_registration_requests
		SET status = 'approved', reviewed_at = NOW(), updated_at = NOW()
		WHERE id = $1
	\`, requestID)
}
`;

/** The removed SQL approval files' writes, with placeholder values. */
const REMOVED_SQL_APPROVAL = `-- Script to approve a pending OAuth registration request
INSERT INTO users (id, organization_id, email, name, role, status, created_at, updated_at)
SELECT gen_random_uuid(), org.org_id, 'user@example.com', 'User', 'admin', 'active', NOW(), NOW()
FROM org;

UPDATE user_registration_requests
SET status = 'approved', reviewed_at = NOW(), updated_at = NOW()
WHERE email = 'user@example.com' AND status = 'pending';
`;

/** The removed token generator's signing call. */
const REMOVED_TOKEN_GENERATOR = `package main

func main() {
	claims := jwt.MapClaims{"role": "admin", "exp": time.Now().Add(24 * time.Hour).Unix()}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}
`;

/** A seed file's shape: the fence runs before the users insert. */
const FENCED_SEED = `-- TEST DATA ONLY.
DO $guard$
BEGIN
    IF EXISTS (
        SELECT 1 FROM organizations
        WHERE plan_type = 'enterprise'
    ) THEN
        RAISE EXCEPTION 'Refusing to seed test data.';
    END IF;
END
$guard$;

INSERT INTO organizations (id, name, created_at, updated_at) VALUES ('11111111-1111-1111-1111-111111111111', 'Test', NOW(), NOW());
INSERT INTO users (id, organization_id, email, name, role, created_at, updated_at) VALUES ('22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111', 'test@example.com', 'Test', 'admin', NOW(), NOW());
`;

// ---------------------------------------------------------------------------

describe('nothing under apps/backend/scripts approves a registration or signs a token', () => {
  it('the check refuses the removed Go approval program at each write', () => {
    expect(outOfBandWrites(REMOVED_GO_APPROVAL)).toEqual([
      'line 5: inserts into users with no production-organization fence before it',
      'line 10: writes user_registration_requests',
    ]);
  });

  it('the check refuses the removed SQL approval files at each write', () => {
    expect(outOfBandWrites(REMOVED_SQL_APPROVAL)).toEqual([
      'line 2: inserts into users with no production-organization fence before it',
      'line 6: writes user_registration_requests',
    ]);
  });

  it('the check refuses the removed token generator at its signing calls', () => {
    expect(outOfBandWrites(REMOVED_TOKEN_GENERATOR)).toEqual([
      'line 5: signs a JWT',
      'line 6: signs a JWT',
    ]);
  });

  it('the check refuses each planted signing call and registration write', () => {
    const cases: Array<[string, string]> = [
      ['a Node jsonwebtoken sign', "const t = jwt.sign({ role: 'admin' }, secret);\n"],
      ['a PyJWT encode', "t = jwt.encode({'role': 'admin'}, secret, algorithm='HS256')\n"],
      ['a bare golang-jwt New', 'token := jwt.New(jwt.SigningMethodHS256)\n'],
      ['a schema-qualified update', "update public.user_registration_requests set status = 'approved';\n"],
      ['a delete', 'DELETE FROM user_registration_requests WHERE id = $1;\n'],
    ];
    for (const [name, text] of cases) {
      expect(outOfBandWrites(text), name).toHaveLength(1);
    }
  });

  it('the check passes a seed file whose fence runs before its users insert', () => {
    expect(outOfBandWrites(FENCED_SEED)).toEqual([]);
  });

  it('the check refuses a users insert whose fence comes after it', () => {
    const lines = FENCED_SEED.split('\n');
    const insert = lines.findIndex((l) => l.startsWith('INSERT INTO users'));
    const reordered = [lines[insert], ...lines.slice(0, insert), ...lines.slice(insert + 1)].join('\n');
    expect(outOfBandWrites(reordered)).toEqual([
      'line 1: inserts into users with no production-organization fence before it',
    ]);
  });

  it('no file under apps/backend/scripts approves a registration, signs a token or inserts users unfenced', () => {
    const files = filesUnder(SCRIPTS_DIR);
    expect(files.length).toBeGreaterThan(0);
    const findings = files.flatMap((file) =>
      outOfBandWrites(readFileSync(file, 'utf8')).map((f) => `${repoRelative(file)} ${f}`),
    );
    expect(findings).toEqual([]);
  });
});
