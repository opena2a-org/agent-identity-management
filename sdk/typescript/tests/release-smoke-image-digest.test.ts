import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';

/**
 * scripts/release-smoke.sh starts its throwaway PostgreSQL from an image it
 * names by digest. A tag such as latest-pg16 moves, so two runs of the smoke
 * could start two different images, and a run could fail on the image and not
 * on the SDK under test. With the digest in the reference every run starts
 * the same image, and moving to a newer one is a change to the script.
 *
 * The script is read as the shell reads it: a line ending in a backslash
 * continues on the next, and comment lines are skipped.
 */

const SCRIPT_PATH = join(__dirname, '..', 'scripts', 'release-smoke.sh');
const SCRIPT_LABEL = 'scripts/release-smoke.sh';

/** A repository path with at least one slash, an optional tag and an optional digest. */
const IMAGE_REFERENCE = /^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:\/[a-z0-9]+(?:[._-][a-z0-9]+)*)+(?::[A-Za-z0-9_][A-Za-z0-9_.-]*)?(?:@[a-z0-9]+:[0-9a-f]+)?$/;
const SHA256_DIGEST = /@sha256:[0-9a-f]{64}$/;

interface Command {
  /** 1-based line the command starts on. */
  line: number;
  text: string;
}

function commands(script: string): Command[] {
  const lines = script.split('\n');
  const out: Command[] = [];
  let i = 0;
  while (i < lines.length) {
    const start = i;
    if (lines[i].trimStart().startsWith('#')) {
      i++;
      continue;
    }
    let text = lines[i];
    while (text.endsWith('\\') && i + 1 < lines.length) {
      i++;
      text = text.slice(0, -1) + ' ' + lines[i].trim();
    }
    out.push({ line: start + 1, text });
    i++;
  }
  return out;
}

/** The image references a `docker run` command names, with the line it starts on. */
function dockerRunImages(script: string): Array<{ line: number; image: string }> {
  const out: Array<{ line: number; image: string }> = [];
  for (const command of commands(script)) {
    const run = /\bdocker run\b(.*)$/.exec(command.text);
    if (run === null) continue;
    for (const word of run[1].split(/\s+/)) {
      const bare = word.replace(/^["']|["']$/g, '');
      if (IMAGE_REFERENCE.test(bare)) out.push({ line: command.line, image: bare });
    }
  }
  return out;
}

describe('the TypeScript SDK release smoke names its PostgreSQL image by digest', () => {
  it('the reader finds the image of a docker run, on one line or continued', () => {
    const sample = [
      '# docker run -d example/commented:1 is not a command',
      'docker run -d --name "$NAME" \\',
      '    -p "127.0.0.1:${PORT}:5432" \\',
      '    example/database:latest-pg16 >/dev/null || exit 2',
      `docker run --rm example/pinned:1.2@sha256:${'ab'.repeat(32)} true`,
    ].join('\n');
    expect(dockerRunImages(sample)).toEqual([
      { line: 2, image: 'example/database:latest-pg16' },
      { line: 5, image: `example/pinned:1.2@sha256:${'ab'.repeat(32)}` },
    ]);
  });

  it('every docker run in the script names its image with a sha256 digest', () => {
    const images = dockerRunImages(readFileSync(SCRIPT_PATH, 'utf-8'));
    expect(images.length, `${SCRIPT_LABEL} starts a container with docker run`).toBeGreaterThan(0);
    const unpinned = images
      .filter(({ image }) => !SHA256_DIGEST.test(image))
      .map(({ line, image }) => `${SCRIPT_LABEL}:${line}: ${image}`);
    expect(unpinned, 'an image named by tag alone can change between two runs').toEqual([]);
  });

  it('the PostgreSQL image keeps the tag it was read from beside the digest', () => {
    const images = dockerRunImages(readFileSync(SCRIPT_PATH, 'utf-8')).map(({ image }) => image);
    expect(images.filter((image) => image.startsWith('timescale/timescaledb:latest-pg16@'))).toHaveLength(1);
  });
});
