import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { rotateLogIfNeeded, LOG_ROTATE_BYTES, LOG_ROTATED_COPIES_KEPT } from './log-rotation';

/** A cap of a few bytes, so no test here writes a large file. */
const CAP = 16;

let dir: string;

beforeEach(() => {
  dir = fs.mkdtempSync(path.join(os.tmpdir(), 'arp-log-rotation-'));
});

afterEach(() => {
  fs.rmSync(dir, { recursive: true, force: true });
});

/** Names of the rotated copies of `name` in the scratch directory, oldest first. */
function copiesOf(name: string): string[] {
  return fs
    .readdirSync(dir)
    .filter((entry) => entry.startsWith(name + '.'))
    .sort();
}

describe('rotateLogIfNeeded', () => {
  it('has the defaults the changelog states: 50 MiB and five rotated copies', () => {
    expect(LOG_ROTATE_BYTES).toBe(50 * 1024 * 1024);
    expect(LOG_ROTATED_COPIES_KEPT).toBe(5);
  });

  it('returns false for a file that does not exist and creates nothing', () => {
    const file = path.join(dir, 'log.jsonl');

    expect(rotateLogIfNeeded(file, CAP, 5)).toBe(false);

    expect(fs.readdirSync(dir)).toEqual([]);
  });

  it('leaves a file exactly at the cap in place', () => {
    const file = path.join(dir, 'log.jsonl');
    fs.writeFileSync(file, 'x'.repeat(CAP));

    expect(rotateLogIfNeeded(file, CAP, 5)).toBe(false);

    expect(fs.readFileSync(file, 'utf-8')).toBe('x'.repeat(CAP));
    expect(copiesOf('log.jsonl')).toEqual([]);
  });

  it('renames a file one byte above the cap to <name>.<timestamp>.<pid>.<8 hex digits>', () => {
    const file = path.join(dir, 'log.jsonl');
    fs.writeFileSync(file, 'x'.repeat(CAP + 1));
    const before = Date.now();

    expect(rotateLogIfNeeded(file, CAP, 5)).toBe(true);

    expect(fs.existsSync(file)).toBe(false);
    const copies = copiesOf('log.jsonl');
    expect(copies).toHaveLength(1);
    const parts = /^log\.jsonl\.(\d{13})\.(\d+)\.([0-9a-f]{8})$/.exec(copies[0]);
    expect(parts).not.toBeNull();
    expect(Number(parts![1])).toBeGreaterThanOrEqual(before);
    expect(Number(parts![2])).toBe(process.pid);
    expect(fs.readFileSync(path.join(dir, copies[0]), 'utf-8')).toBe('x'.repeat(CAP + 1));
  });

  it('keeps the newest `keep` rotated copies and leaves files with other names alone', () => {
    const file = path.join(dir, 'log.jsonl');
    const earlier = [1, 2, 3].map((i) => `log.jsonl.${1700000000000 + i}.1.aaaaaaaa`);
    for (const name of earlier) fs.writeFileSync(path.join(dir, name), 'old');
    // Names that begin the same way but belong to other files.
    const others = ['log.jsonl2.1700000000000.1.aaaaaaaa', 'log.json', 'other.jsonl'];
    for (const name of others) fs.writeFileSync(path.join(dir, name), 'other');
    fs.writeFileSync(file, 'x'.repeat(CAP + 1));

    expect(rotateLogIfNeeded(file, CAP, 2)).toBe(true);

    const copies = copiesOf('log.jsonl');
    expect(copies).toHaveLength(2);
    expect(copies[0]).toBe(earlier[2]);
    expect(copies[1] > earlier[2]).toBe(true);
    for (const name of others) {
      expect(fs.readFileSync(path.join(dir, name), 'utf-8')).toBe('other');
    }
  });

  it('keeps the newest five rotated copies when a user file such as <name>.bak sits beside them', () => {
    const file = path.join(dir, 'events.jsonl');
    const earlier = [1, 2, 3, 4, 5].map((i) => `events.jsonl.${1700000000000 + i}.1.aaaaaaaa`);
    for (const name of earlier) fs.writeFileSync(path.join(dir, name), 'old');
    // Letters sort after digits, so a prefix match would count this as the newest copy.
    fs.writeFileSync(path.join(dir, 'events.jsonl.bak'), 'mine');
    fs.writeFileSync(file, 'x'.repeat(CAP + 1));

    expect(rotateLogIfNeeded(file, CAP, 5)).toBe(true);

    const copies = fs
      .readdirSync(dir)
      .filter((entry) => /^events\.jsonl\.\d{13}\.\d+\.[0-9a-f]{8}$/.test(entry))
      .sort();
    expect(copies).toHaveLength(5);
    expect(copies.slice(0, 4)).toEqual(earlier.slice(1));
    expect(fs.readFileSync(path.join(dir, 'events.jsonl.bak'), 'utf-8')).toBe('mine');
  });

  it('rotates the target of a symbolic link beside the target and leaves the link pointing at it', () => {
    const targetDir = path.join(dir, 'elsewhere');
    fs.mkdirSync(targetDir);
    const target = path.join(targetDir, 'events.jsonl');
    const link = path.join(dir, 'events.jsonl');
    fs.writeFileSync(target, 'x'.repeat(CAP + 1));
    fs.symlinkSync(target, link);

    expect(rotateLogIfNeeded(link, CAP, 5)).toBe(true);

    expect(fs.lstatSync(link).isSymbolicLink()).toBe(true);
    expect(fs.readlinkSync(link)).toBe(target);
    expect(fs.existsSync(target)).toBe(false);
    const rotated = fs.readdirSync(targetDir).filter((entry) => entry !== 'events.jsonl');
    expect(rotated).toHaveLength(1);
    expect(fs.readFileSync(path.join(targetDir, rotated[0]), 'utf-8')).toBe('x'.repeat(CAP + 1));
    expect(fs.readdirSync(dir).sort()).toEqual(['elsewhere', 'events.jsonl']);

    // The next append through the link lands in the linked location.
    fs.appendFileSync(link, '{"event":1}\n');
    expect(fs.readFileSync(target, 'utf-8')).toBe('{"event":1}\n');
  });

  it('rotates on each call that finds the file above the cap and never holds more than `keep` copies', () => {
    const file = path.join(dir, 'log.jsonl');

    for (let i = 0; i < 4; i++) {
      fs.writeFileSync(file, 'x'.repeat(CAP + 1));
      expect(rotateLogIfNeeded(file, CAP, 2)).toBe(true);
      expect(copiesOf('log.jsonl').length).toBeLessThanOrEqual(2);
    }

    expect(copiesOf('log.jsonl')).toHaveLength(2);
  });
});
