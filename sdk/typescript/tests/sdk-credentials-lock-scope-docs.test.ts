import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * The Java SDK and the Python SDK each take a lock on the same file,
 * ~/.aim/sdk_credentials.json.lock, before they change the SDK credentials
 * file, and they take it with two different calls: FileChannel.tryLock(), a
 * record lock, and fcntl.flock(). Linux holds the two separately, so there a
 * process of one SDK takes its lock while a process of the other holds its
 * own. Measured with the calls each SDK makes, on Linux 7.0.12 with Java
 * 21.0.12 and Python 3.12.15, in both orders; on macOS 26.6.2, with Java
 * 25.0.2 and Python 3.14.3, each was refused while the other held its own.
 *
 * Each lock therefore keeps apart the processes of its own SDK, and on Linux
 * not a process of the other. While the two calls differ, the javadoc, the
 * docstring and both changelogs say so, and the changelogs say it in the same
 * words. When both SDKs take the same kind of lock, the first test fails:
 * these statements no longer hold then and go, with this file.
 */

const REPO_ROOT = join(__dirname, '..', '..', '..');
const JAVA_SOURCE = join(
  REPO_ROOT,
  'sdk',
  'java',
  'src',
  'main',
  'java',
  'org',
  'opena2a',
  'aim',
  'credentials',
  'CredentialManager.java',
);
const PYTHON_SOURCE = join(REPO_ROOT, 'sdk', 'python', 'aim_sdk', 'credentials.py');
const JAVA_CHANGELOG = join(REPO_ROOT, 'sdk', 'java', 'CHANGELOG.md');
const PYTHON_CHANGELOG = join(REPO_ROOT, 'sdk', 'python', 'CHANGELOG.md');

/** What both changelogs say, as one line each. */
const CHANGELOG_STATEMENT = [
  'On Linux it does not keep a process of the Java SDK apart from a process of the Python SDK: ' +
    'the Python SDK locks the same file with `flock`, the Java SDK with a record lock (`FileChannel.tryLock`), ' +
    'and Linux holds the two separately.',
  'Measured with the calls each SDK makes, on Linux 7.0.12 with Java 21.0.12 and Python 3.12.15: ' +
    'each took its lock while the other held its own, in both orders.',
  'On macOS 26.6.2, with Java 25.0.2 and Python 3.14.3, each was refused its lock while the other held its own, ' +
    'in both orders (#722).',
].join(' ');

/** `text` with every run of whitespace as one space. */
function oneLine(text: string): string {
  return text.replace(/\s+/g, ' ').trim();
}

/** The javadoc comment that ends right above `declaration`, without its ` * ` margins. */
function javadocAbove(source: string, declaration: string): string {
  const at = source.indexOf(declaration);
  expect(at, `CredentialManager.java declares ${declaration}`).toBeGreaterThan(-1);
  const open = source.lastIndexOf('/**', at);
  const close = source.indexOf('*/', open);
  expect(source.slice(close + 2, at).trim(), 'the javadoc sits right above the declaration').toBe('');
  return oneLine(
    source
      .slice(open + 3, close)
      .split('\n')
      .map((line) => line.replace(/^\s*\*\s?/, ''))
      .join('\n'),
  );
}

/** The docstring of the Python function `name`. */
function docstringOf(source: string, name: string): string {
  const at = source.indexOf(`def ${name}(`);
  expect(at, `credentials.py defines ${name}`).toBeGreaterThan(-1);
  const open = source.indexOf('"""', at);
  const close = source.indexOf('"""', open + 3);
  expect(source.slice(at, open).split('\n').length, 'the docstring opens the function body').toBe(2);
  return oneLine(source.slice(open + 3, close));
}

describe('each SDK says its credentials lock does not serialize it with the other SDK on Linux', () => {
  const java = readFileSync(JAVA_SOURCE, 'utf-8');
  const python = readFileSync(PYTHON_SOURCE, 'utf-8');

  it('the two SDKs lock the file with different calls', () => {
    expect(java, 'the Java SDK takes a record lock').toContain('lock = channel.tryLock();');
    expect(python, 'the Python SDK takes a flock').toContain('fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)');
    expect(python, 'the Python SDK takes no record lock').not.toContain('fcntl.lockf(');
  });

  it('the two SDKs lock the same file, the credentials file name with .lock appended', () => {
    expect(java).toContain('return credentialsPath.resolveSibling(credentialsPath.getFileName() + ".lock");');
    expect(python).toContain('return SDK_CREDENTIALS_FILE.with_name(SDK_CREDENTIALS_FILE.name + ".lock")');
    expect(java).toContain('private static final String SDK_CREDENTIALS_FILE = "sdk_credentials.json";');
    expect(python).toContain('SDK_CREDENTIALS_FILE = AIM_DIR / "sdk_credentials.json"');
  });

  it('the javadoc of withSdkCredentialsLock says so', () => {
    const javadoc = javadocAbove(
      java,
      'public static <T, E extends Exception> T withSdkCredentialsLock(\n            Path credentialsPath, Duration timeout,',
    );
    expect(javadoc).toContain(
      'On Linux it does not keep a process of this SDK apart from a process of the Python SDK.',
    );
    expect(javadoc).toContain('That SDK locks the same file with {@code flock}');
    expect(javadoc).toContain('On macOS the two locks exclude each other.');
  });

  it('the docstring of sdk_credentials_lock says so', () => {
    const docstring = docstringOf(python, 'sdk_credentials_lock');
    expect(docstring).toContain('The lock keeps processes of this SDK apart.');
    expect(docstring).toContain(
      'On Linux it does not keep a process of this SDK apart from a process of the Java SDK.',
    );
    expect(docstring).toContain('That SDK locks the same file with a record lock (``FileChannel.tryLock``)');
    expect(docstring).toContain('On macOS the two locks exclude each other.');
  });

  it('the Java and the Python changelog say it in the same words', () => {
    for (const [label, path] of [
      ['sdk/java/CHANGELOG.md', JAVA_CHANGELOG],
      ['sdk/python/CHANGELOG.md', PYTHON_CHANGELOG],
    ] as Array<[string, string]>) {
      expect(oneLine(readFileSync(path, 'utf-8')), label).toContain(CHANGELOG_STATEMENT);
    }
  });
});
