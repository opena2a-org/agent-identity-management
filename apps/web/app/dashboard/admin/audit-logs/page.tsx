"use client";

import { useCallback, useEffect, useState } from "react";
import { Bot, ChevronLeft, ChevronRight, CircleSlash, FileText, RefreshCw, User } from "lucide-react";
import { AuthGuard } from "@/components/auth-guard";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import {
  AUDIT_PAGE_SIZE,
  auditActionLabel,
  auditActor,
  auditTargetLabel,
  formatAuditTime,
  newestFirst,
  type AuditActor,
  type AuditRecord,
} from "@/lib/audit-records";

const COLUMNS = "md:grid md:grid-cols-[11rem_minmax(0,1fr)_minmax(0,1fr)_minmax(0,1.3fr)] md:gap-4";

function ActorIcon({ actor }: { actor: AuditActor }) {
  const className = "h-4 w-4 flex-shrink-0 text-ink-tertiary";
  if (actor.kind === "user") return <User className={className} aria-hidden="true" />;
  if (actor.kind === "agent") return <Bot className={className} aria-hidden="true" />;
  return <CircleSlash className={className} aria-hidden="true" />;
}

function CellLabel({ children }: { children: string }) {
  return <span className="text-xs font-medium text-ink-tertiary md:sr-only">{children}</span>;
}

function AuditRow({ record }: { record: AuditRecord }) {
  const actor = auditActor(record);
  return (
    <li className={`space-y-2 px-4 py-3 md:space-y-0 ${COLUMNS} md:items-center`} data-testid="audit-record">
      <div className="flex flex-col">
        <CellLabel>Time</CellLabel>
        <time dateTime={record.timestamp} className="font-mono text-xs text-ink-body">
          {formatAuditTime(record.timestamp)}
        </time>
      </div>
      <div className="flex min-w-0 flex-col">
        <CellLabel>Actor</CellLabel>
        <span className="flex min-w-0 items-center gap-2 text-sm text-ink" title={actor.kind === "none" ? undefined : actor.id}>
          <ActorIcon actor={actor} />
          <span className="truncate">{actor.label}</span>
        </span>
      </div>
      <div className="flex min-w-0 flex-col">
        <CellLabel>Action</CellLabel>
        <span className="truncate text-sm text-ink" title={record.action}>
          {auditActionLabel(record.action)}
        </span>
      </div>
      <div className="flex min-w-0 flex-col">
        <CellLabel>Target</CellLabel>
        <span className="text-sm text-ink">{auditTargetLabel(record.resourceType)}</span>
        <span className="break-all font-mono text-xs text-ink-secondary">{record.resourceId}</span>
      </div>
    </li>
  );
}

export default function AuditLogsPage() {
  const [records, setRecords] = useState<AuditRecord[]>([]);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async (at: number) => {
    setLoading(true);
    setError(null);
    try {
      const logs: AuditRecord[] = await api.getAuditLogs(AUDIT_PAGE_SIZE, at);
      setRecords(newestFirst(logs));
    } catch (err: any) {
      setRecords([]);
      setError(err?.message || "The audit records could not be loaded.");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load(offset);
  }, [load, offset]);

  const hasOlder = !loading && !error && records.length === AUDIT_PAGE_SIZE;
  const hasNewer = offset > 0;

  return (
    <AuthGuard>
      <div className="space-y-6">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <h1 className="text-2xl font-bold text-ink">Audit log</h1>
            <p className="mt-1 text-sm text-ink-secondary">
              Every action recorded for your organization, newest first. Times are in UTC.
            </p>
          </div>
          <Button variant="outline" size="sm" onClick={() => load(offset)} disabled={loading} className="self-start">
            <RefreshCw className={`mr-2 h-4 w-4 ${loading ? "animate-spin" : ""}`} aria-hidden="true" />
            Refresh
          </Button>
        </div>

        <section className="glass overflow-hidden" aria-label="Audit records">
          <div
            className={`hidden border-b border-divider px-4 py-2 text-xs font-medium uppercase tracking-wide text-ink-tertiary ${COLUMNS}`}
            aria-hidden="true"
          >
            <span>Time</span>
            <span>Actor</span>
            <span>Action</span>
            <span>Target</span>
          </div>

          {loading ? (
            <ul className="divide-y divide-divider" aria-busy="true">
              {Array.from({ length: 5 }, (_, i) => (
                <li key={i} className="px-4 py-3">
                  <div className="h-4 w-full animate-pulse rounded bg-glass-inset" />
                </li>
              ))}
            </ul>
          ) : error ? (
            <div className="px-4 py-12 text-center" role="alert">
              <p className="text-sm text-ink">{error}</p>
              <Button variant="outline" size="sm" className="mt-4" onClick={() => load(offset)}>
                Try again
              </Button>
            </div>
          ) : records.length === 0 ? (
            <div className="px-4 py-12 text-center">
              <FileText className="mx-auto h-8 w-8 text-ink-tertiary" aria-hidden="true" />
              <p className="mt-2 text-sm text-ink-secondary">
                {offset > 0 ? "No older audit records." : "No audit records yet."}
              </p>
            </div>
          ) : (
            <ul className="divide-y divide-divider">
              {records.map((record) => (
                <AuditRow key={record.id} record={record} />
              ))}
            </ul>
          )}
        </section>

        {(hasNewer || hasOlder) && (
          <nav className="flex items-center justify-between gap-2" aria-label="Audit log pages">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setOffset(Math.max(0, offset - AUDIT_PAGE_SIZE))}
              disabled={!hasNewer || loading}
            >
              <ChevronLeft className="mr-1 h-4 w-4" aria-hidden="true" />
              Newer
            </Button>
            <span className="text-xs text-ink-secondary">
              {records.length > 0 ? `Records ${offset + 1}-${offset + records.length}` : null}
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setOffset(offset + AUDIT_PAGE_SIZE)}
              disabled={!hasOlder}
            >
              Older
              <ChevronRight className="ml-1 h-4 w-4" aria-hidden="true" />
            </Button>
          </nav>
        )}
      </div>
    </AuthGuard>
  );
}
