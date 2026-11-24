import { describe, it, expect } from 'vitest';
import { FilesystemInterceptor } from '../../../src/arp/interceptors/filesystem';
import { EventEngine } from '../../../src/arp/engine/event-engine';

// isExcluded / isOutsideAllowed are private; reach them through a narrow view.
type PathChecks = {
  isExcluded(filePath: string): boolean;
  isOutsideAllowed(filePath: string): boolean;
};

function create(allowed?: string[], excluded?: string[]): PathChecks {
  const engine = new EventEngine({ agentName: 'test-agent' });
  return new FilesystemInterceptor(engine, allowed, excluded) as unknown as PathChecks;
}

describe('FilesystemInterceptor path checks', () => {
  describe('isExcluded', () => {
    it('excludes the directory itself and paths beneath it', () => {
      const fsi = create(undefined, ['/srv/app']);
      expect(fsi.isExcluded('/srv/app')).toBe(true);
      expect(fsi.isExcluded('/srv/app/')).toBe(true);
      expect(fsi.isExcluded('/srv/app/logs/out.log')).toBe(true);
    });

    it('does not exclude a sibling that shares a name prefix', () => {
      const fsi = create(undefined, ['/srv/app', '/data']);
      expect(fsi.isExcluded('/srv/app-secrets')).toBe(false);
      expect(fsi.isExcluded('/srv/app-secrets/key.pem')).toBe(false);
      expect(fsi.isExcluded('/database')).toBe(false);
      expect(fsi.isExcluded('/database/dump.sql')).toBe(false);
    });

    it('does not exclude a parent or a path escaping with ..', () => {
      const fsi = create(undefined, ['/srv/app']);
      expect(fsi.isExcluded('/srv')).toBe(false);
      expect(fsi.isExcluded('/srv/app/../app-secrets/key.pem')).toBe(false);
    });

    it('excludes nothing when no exclude paths are set', () => {
      expect(create().isExcluded('/srv/app')).toBe(false);
    });
  });

  describe('isOutsideAllowed', () => {
    it('allows the directory itself and paths beneath it', () => {
      const fsi = create(['/srv/app']);
      expect(fsi.isOutsideAllowed('/srv/app')).toBe(false);
      expect(fsi.isOutsideAllowed('/srv/app/')).toBe(false);
      expect(fsi.isOutsideAllowed('/srv/app/data/file.txt')).toBe(false);
    });

    it('does not allow a sibling that shares a name prefix', () => {
      const fsi = create(['/srv/app', '/data']);
      expect(fsi.isOutsideAllowed('/srv/app-secrets')).toBe(true);
      expect(fsi.isOutsideAllowed('/srv/app-secrets/key.pem')).toBe(true);
      expect(fsi.isOutsideAllowed('/database')).toBe(true);
      expect(fsi.isOutsideAllowed('/database/dump.sql')).toBe(true);
    });

    it('does not allow a parent or a path escaping with ..', () => {
      const fsi = create(['/srv/app']);
      expect(fsi.isOutsideAllowed('/srv')).toBe(true);
      expect(fsi.isOutsideAllowed('/srv/app/../app-secrets/key.pem')).toBe(true);
    });

    it('allows a file whose name begins with two dots', () => {
      const fsi = create(['/srv/app']);
      expect(fsi.isOutsideAllowed('/srv/app/..hidden')).toBe(false);
    });

    it('allows everything when no allowed paths are set', () => {
      expect(create().isOutsideAllowed('/anywhere')).toBe(false);
    });
  });
});
