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
 * Filesystem monitor — watches for file access outside expected paths.
 * Uses fs.watch (cross-platform, efficient, no polling).
 */
export class FilesystemMonitor implements Monitor {
  readonly type: MonitorType = 'filesystem';
  private watchers: fs.FSWatcher[] = [];
  private readonly engine: EventEngine;
  private readonly watchPaths: string[];
  private readonly allowedPaths: Set<string>;
  private readonly debounceMap = new Map<string, ReturnType<typeof setTimeout>>();
  private readonly debounceMs = 1000;

  constructor(
    engine: EventEngine,
    watchPaths?: string[],
    allowedPaths?: string[],
  ) {
    this.engine = engine;
    this.watchPaths = watchPaths ?? [process.cwd()];
    this.allowedPaths = new Set(allowedPaths ?? []);
  }

  async start(): Promise<void> {
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

  private handleEvent(eventType: string, filename: string, basePath: string): void {
    // Debounce: many editors trigger multiple events per save
    const key = `${eventType}:${filename}`;
    if (this.debounceMap.has(key)) return;

    this.debounceMap.set(key, setTimeout(() => {
      this.debounceMap.delete(key);
    }, this.debounceMs));

    const fullPath = path.join(basePath, filename);

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
