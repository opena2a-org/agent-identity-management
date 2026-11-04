/**
 * The binary list the process monitor and the process interceptor share, and
 * the one match both use to compare a binary's base name with it.
 */

/** Binaries commonly used for exfiltration, lateral movement, or exploitation */
export const SUSPICIOUS_BINARIES: readonly string[] = [
  'curl', 'wget', 'nc', 'ncat', 'nmap', 'ssh', 'scp',
  'python', 'python3', 'perl', 'ruby', 'base64',
  'socat', 'telnet', 'ftp', 'rsync',
];

/** A version after a binary's name: groups of digits separated by dots, as the
 *  `3.12` of `python3.12` or the `5.34` of `perl5.34`. */
const VERSION = /^\d+(?:\.\d+)*$/;

/**
 * Whether a binary's base name is on SUSPICIOUS_BINARIES. The name is compared
 * in lower case, and matches an entry it equals or that it extends with a
 * version. On macOS, `ps` lists the Homebrew and Command Line Tools python3 as
 * `.../Python.app/Contents/MacOS/Python`, whose base name `Python` matches
 * `python`; `python3.12`, `perl5.34` and `PERL` match too, while
 * `python3-config`, `ncdu` and `sshd` do not.
 */
export function isSuspiciousBinary(baseName: string): boolean {
  const name = baseName.toLowerCase();
  return SUSPICIOUS_BINARIES.some(
    (entry) => name === entry || (name.startsWith(entry) && VERSION.test(name.slice(entry.length))),
  );
}
