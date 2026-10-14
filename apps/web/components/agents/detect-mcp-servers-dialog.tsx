"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Check, Copy, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { api, type Agent } from "@/lib/api";

// How often the open dialog reads the agent while it waits for the command to report.
export const DETECT_POLL_MS = 5000;

const CODE = "overflow-x-auto rounded-inset-sm bg-glass-inset-gray p-3 pr-12 font-mono text-xs text-ink-body";

/**
 * The command that registers an agent's MCP servers from the machine it runs on. The name is
 * escaped for the single-quoted Python string, then for the double-quoted shell argument.
 */
export function detectCommand(name: string): string {
  const python = name.replace(/[\\']/g, "\\$&");
  const code = `from aim_sdk import secure; secure('${python}', auto_detect_mcp=True)`;
  return `python -c "${code.replace(/[\\"$`]/g, "\\$&")}"`;
}

/**
 * Reads the agent every DETECT_POLL_MS while mounted. The first read is the baseline; a later
 * read whose talksTo holds entries the baseline did not replaces the waiting line with them.
 * A failed read changes nothing, so an error never reads as a result.
 */
function DetectListener({ agentId, onRegistered }: { agentId: string; onRegistered: () => void }) {
  const [registered, setRegistered] = useState<string[] | null>(null);

  useEffect(() => {
    let cancelled = false;
    let baseline: Set<string> | null = null;
    let timer: ReturnType<typeof setInterval> | undefined;
    const tick = async () => {
      try {
        const { talksTo } = await api.getAgent(agentId);
        if (cancelled) return;
        const entries = talksTo ?? [];
        if (baseline === null) {
          baseline = new Set(entries);
          return;
        }
        const known = baseline;
        const fresh = entries.filter((entry) => !known.has(entry));
        if (fresh.length === 0) return;
        cancelled = true;
        clearInterval(timer);
        setRegistered(fresh);
        onRegistered();
      } catch {
        // The next read tries again; the waiting line stays as it is.
      }
    };
    tick();
    timer = setInterval(tick, DETECT_POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [agentId, onRegistered]);

  return (
    <p role="status" className="flex items-center gap-2 text-sm text-ink-secondary">
      {registered ? (
        <Check className="h-4 w-4 flex-shrink-0 text-success-text" aria-hidden="true" />
      ) : (
        <Loader2 className="h-4 w-4 flex-shrink-0 animate-spin" aria-hidden="true" />
      )}
      <span>{registered ? `Registered to the agent: ${registered.join(", ")}` : "Waiting for the command to report."}</span>
    </p>
  );
}

/**
 * Detecting an agent's MCP servers runs where the agent runs, so the dialog gives the command
 * for that machine and listens for its result instead of asking for a configuration path.
 */
export function DetectMCPServersDialog({
  agent,
  open,
  onOpenChange,
  onDetected,
}: {
  agent: Pick<Agent, "id" | "name" | "displayName">;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDetected: () => void;
}) {
  const [copied, setCopied] = useState(false);
  const detected = useRef(false);
  const markDetected = useCallback(() => {
    detected.current = true;
  }, []);

  // `aim-sdk login` defaults to the hosted service; a self-hosted dashboard passes its own origin.
  const origin = typeof window === "undefined" ? "" : window.location.origin;
  const commands = [origin ? `aim-sdk login --url ${origin}` : "aim-sdk login", detectCommand(agent.name)].join("\n");

  // The page reload shows its loading view in place of the page, which would close this dialog
  // before the registered names can be read, so the page is refreshed when the dialog closes.
  const handleOpenChange = (next: boolean) => {
    if (!next && detected.current) {
      detected.current = false;
      onDetected();
    }
    onOpenChange(next);
  };

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(commands);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // The clipboard is unavailable outside a secure context; the commands stay selectable.
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Detect MCP servers</DialogTitle>
          <DialogDescription>
            {`Run this on the machine where ${agent.displayName || agent.name} runs. The MCP servers in its Claude Desktop configuration are registered to the agent.`}
          </DialogDescription>
        </DialogHeader>
        <div className="relative min-w-0">
          <pre className={CODE}>
            <code>{commands}</code>
          </pre>
          <Button
            variant="ghost"
            size="sm"
            aria-label="Copy commands"
            onClick={copy}
            className="absolute right-1 top-1"
          >
            {copied ? <Check className="h-4 w-4 text-success-text" /> : <Copy className="h-4 w-4" />}
          </Button>
        </div>
        <DetectListener agentId={agent.id} onRegistered={markDetected} />
      </DialogContent>
    </Dialog>
  );
}
