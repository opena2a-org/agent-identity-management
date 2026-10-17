/**
 * Size-capped JSON-lines logs.
 *
 * A log the SDK appends to for as long as an agent runs needs a cap, or it
 * grows until the disk is full. `rotateLogIfNeeded` gives such a log the cap
 * the telemetry writer's correlated-events log has: once the file is above
 * `maxBytes` it is renamed aside, the next append starts a new file, and only
 * the newest `keep` renamed copies are kept.
 */
import * as crypto from 'crypto';
import * as fs from 'fs';
import * as path from 'path';

/** A log is rotated once it is above this many bytes: 50 MiB. */
export const LOG_ROTATE_BYTES = 50 * 1024 * 1024;

/** How many rotated copies of a log are kept beside it; older ones are deleted. */
export const LOG_ROTATED_COPIES_KEPT = 5;

/**
 * Rotate `filePath` when it is larger than `maxBytes`.
 *
 * The file is renamed to `<name>.<millisecond timestamp>.<pid>.<8 hex digits>`.
 * The pid and the random digits keep two processes that rotate the same file
 * in the same millisecond from renaming to the same name, which matters for a
 * log every agent on the machine shares. After the rename, the rotated copies
 * beyond the newest `keep` are deleted; their names sort by timestamp, so
 * lexical order is age order. Only names of that form count as rotated copies:
 * another file whose name starts with `<name>.` is neither counted nor deleted.
 *
 * When `filePath` is a symbolic link, the file it points to is rotated, beside
 * that file, and the link is left in place, so the next append through the
 * link starts the new file where the link points.
 *
 * Returns true when the file was renamed. Never throws: a missing file, a
 * rename lost to another process or a failed delete leaves the caller free to
 * append as it would have.
 */
export function rotateLogIfNeeded(
  filePath: string,
  maxBytes: number = LOG_ROTATE_BYTES,
  keep: number = LOG_ROTATED_COPIES_KEPT,
): boolean {
  let size: number;
  try {
    // Renaming the link itself would leave its target growing and put a
    // regular file at the link's path, so later appends miss the target.
    filePath = fs.realpathSync(filePath);
    size = fs.statSync(filePath).size;
  } catch {
    return false; // Nothing to rotate yet.
  }
  if (size <= maxBytes) return false;

  const suffix = `${Date.now()}.${process.pid}.${crypto.randomBytes(4).toString('hex')}`;
  try {
    fs.renameSync(filePath, `${filePath}.${suffix}`);
  } catch {
    return false; // Another process rotated it first.
  }
  deleteRotatedCopiesBeyond(filePath, keep);
  return true;
}

/**
 * Delete the rotated copies of `filePath` beyond the newest `keep`: the names
 * `<name>.<13 digits>.<digits>.<8 hex digits>`, as `rotateLogIfNeeded` writes
 * them. Best effort.
 */
function deleteRotatedCopiesBeyond(filePath: string, keep: number): void {
  try {
    const dir = path.dirname(filePath);
    const base = path.basename(filePath).replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const rotatedCopy = new RegExp(`^${base}\\.\\d{13}\\.\\d+\\.[0-9a-f]{8}$`);
    const copies = fs
      .readdirSync(dir)
      .filter((name) => rotatedCopy.test(name))
      .sort();
    for (const name of copies.slice(0, Math.max(copies.length - keep, 0))) {
      try {
        fs.unlinkSync(path.join(dir, name));
      } catch {
        // Already gone, or not ours to delete.
      }
    }
  } catch {
    // The directory could not be read; the copies stay.
  }
}
