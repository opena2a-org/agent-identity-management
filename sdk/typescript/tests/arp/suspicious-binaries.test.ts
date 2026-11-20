import { describe, it, expect } from 'vitest';
import { SUSPICIOUS_BINARIES, isSuspiciousBinary } from '../../src/arp/suspicious-binaries';

describe('isSuspiciousBinary', () => {
  it('matches each entry of the list as written and in upper case', () => {
    for (const entry of SUSPICIOUS_BINARIES) {
      expect(isSuspiciousBinary(entry)).toBe(true);
      expect(isSuspiciousBinary(entry.toUpperCase())).toBe(true);
    }
  });

  it.each(['Python', 'python3.12', 'Python3.13', 'python3.14.0', 'PERL', 'perl5.34', 'ruby3.3', 'wget2'])(
    'matches %s',
    (name) => {
      expect(isSuspiciousBinary(name)).toBe(true);
    },
  );

  it.each(['', 'sleep', 'python3-config', 'ncdu', 'sshd', 'ssh-agent', 'python3.', 'perl.5', 'Python.app'])(
    'does not match %j',
    (name) => {
      expect(isSuspiciousBinary(name)).toBe(false);
    },
  );
});
