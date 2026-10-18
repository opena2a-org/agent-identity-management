import * as fs from 'fs';
import * as path from 'path';
import type { Monitor, MonitorType } from '../types';
import type { EventEngine } from '../engine/event-engine';

/** Sensitive paths that should never be accessed by an agent */
const SENSITIVE_PATHS = [
  '.ssh', '.aws', '.gnupg', '.kube', '.config/gcloud',
  '.docker/config.json', '.npmrc', '.pypirc',
  '.git-credentials', 'wallet.json',
  '.bashrc', '.zshrc', '.bash_profile', '.profile',
  '.gitconfig', '.env', '.netrc', '.pgpass',
];

/**
 * Splits `target`, which may not exist yet, at its nearest existing ancestor:
 * that ancestor as resolved and as its real path, and the names below it,
 * which do not exist. The watcher reports paths under the real path of its
 * root (on macOS a root under /tmp is reported under /private/tmp), so an
 * excluded path is compared in both forms.
 */
function splitAtNearestExisting(target: string): { base: string; realBase: string; missing: string[] } {
  const missing: string[] = [];
  let base = path.resolve(target);
  for (;;) {
    try {
      return { base, realBase: fs.realpathSync(base), missing };
    } catch {
      const parent = path.dirname(base);
      if (parent === base) return { base, realBase: base, missing };
      missing.unshift(path.basename(base));
      base = parent;
    }
  }
}

/** `target` as resolved and as its real path. */
function pathForms(target: string): string[] {
  const { base, realBase, missing } = splitAtNearestExisting(target);
  return [path.join(base, ...missing), path.join(realBase, ...missing)];
}

/** The directories on the way to `target` that do not exist yet, in both forms; `target` is left out. */
function missingDirectoriesBefore(target: string): string[] {
  const { base, realBase, missing } = splitAtNearestExisting(target);
  const dirs: string[] = [];
  for (let n = 1; n < missing.length; n++) {
    const names = missing.slice(0, n);
    dirs.push(path.join(base, ...names), path.join(realBase, ...names));
  }
  return dirs;
}

/**
 * Filesystem monitor — watches for file access outside expected paths.
 * Uses fs.watch (cross-platform, efficient, no polling).
 */
export class FilesystemMonitor implements Monitor {
  readonly type: MonitorType = 'filesystem';
  private watchers: fs.FSWatcher[] = [];
  private readonly engine: EventEngine;
  private readonly watchPaths: string[];
  private readonly allowedPaths: Set<string>;
  // Directories whose changes, and changes to anything under them, produce no
  // event. AgentRuntimeProtection passes the directories it writes to itself,
  // so its own log writes are not reported as filesystem events.
  private readonly excludePaths: string[];
  private excludedRoots: string[] = [];
  // The directories on the way to an excluded one that did not exist when this
  // monitor was constructed. Whoever writes to the excluded directory creates
  // them (AgentRuntimeProtection creates its data directory in start(), the
  // runtime twin its log directory on its first write), so a change to one of
  // these directories itself is not reported. A change to anything else inside
  // them is reported as before.
  private readonly createdOnTheWay: Set<string>;
  private readonly debounceMap = new Map<string, ReturnType<typeof setTimeout>>();
  private readonly debounceMs = 1000;

  constructor(
    engine: EventEngine,
    watchPaths?: string[],
    allowedPaths?: string[],
    excludePaths?: string[],
  ) {
    this.engine = engine;
    this.watchPaths = watchPaths ?? [process.cwd()];
    this.allowedPaths = new Set(allowedPaths ?? []);
    this.excludePaths = excludePaths ?? [];
    this.createdOnTheWay = new Set(this.excludePaths.flatMap(missingDirectoriesBefore));
  }

  async start(): Promise<void> {
    // Resolved here rather than in the constructor: an excluded directory the
    // engine creates in start() exists by now, so a symbolic link in its path
    // is resolved too.
    this.excludedRoots = Array.from(new Set(this.excludePaths.flatMap(pathForms)));

    for (const watchPath of this.watchPaths) {
      try {
        const resolved = fs.realpathSync(watchPath);
        const watcher = fs.watch(resolved, { recursive: true }, (eventType, filename) => {
          if (!filename) return;
          this.handleEvent(eventType, filename, resolved);
        });

        watcher.on('error', () => {
          // Watcher error — likely permission denied or path removed
        });

        this.watchers.push(watcher);
      } catch {
        // Path doesn't exist or not accessible
      }
    }
  }

  async stop(): Promise<void> {
    for (const watcher of this.watchers) {
      watcher.close();
    }
    this.watchers = [];

    for (const timer of this.debounceMap.values()) {
      clearTimeout(timer);
    }
    this.debounceMap.clear();
  }

  isRunning(): boolean {
    return this.watchers.length > 0;
  }

  private isExcluded(fullPath: string): boolean {
    if (this.createdOnTheWay.has(fullPath)) return true;
    return this.excludedRoots.some((root) =>
      fullPath === root || fullPath.startsWith(root.endsWith(path.sep) ? root : root + path.sep)
    );
  }

  private handleEvent(eventType: string, filename: string, basePath: string): void {
    const fullPath = path.join(basePath, filename);
    if (this.isExcluded(fullPath)) return;

    // Debounce: many editors trigger multiple events per save
    const key = `${eventType}:${filename}`;
    if (this.debounceMap.has(key)) return;

    this.debounceMap.set(key, setTimeout(() => {
      this.debounceMap.delete(key);
    }, this.debounceMs));

    // Check if this touches sensitive paths
    const isSensitive = SENSITIVE_PATHS.some((sp) =>
      filename.includes(sp) || fullPath.includes(sp)
    );

    if (isSensitive) {
      this.engine.emit({
        source: 'filesystem',
        category: 'violation',
        severity: 'high',
        description: `Access to sensitive path: ${filename} (${eventType})`,
        data: { path: filename, eventType, sensitive: true },
      });
      return;
    }

    // Check if outside allowed paths
    if (this.allowedPaths.size > 0) {
      const isAllowed = Array.from(this.allowedPaths).some((ap) =>
        fullPath.startsWith(ap)
      );

      if (!isAllowed) {
        this.engine.emit({
          source: 'filesystem',
          category: 'anomaly',
          severity: 'medium',
          description: `File access outside allowed paths: ${filename}`,
          data: { path: filename, eventType, allowed: false },
        });
        return;
      }
    }

    // Normal event — only emit for creates and renames (not every write)
    if (eventType === 'rename') {
      this.engine.emit({
        source: 'filesystem',
        category: 'normal',
        severity: 'info',
        description: `File created/renamed: ${filename}`,
        data: { path: filename, eventType },
      });
    }
  }
}
