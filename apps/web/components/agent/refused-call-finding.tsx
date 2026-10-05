import Link from "next/link";
import type { RefusedCallFinding as Finding } from "@/lib/refused-call";

interface RefusedCallFindingProps {
  finding: Finding;
  /** The linked fix pages are administrator-only; other roles see the text and the API request. */
  showAdminLinks: boolean;
}

/** A refused call in finding shape: what was refused, why, and the fix. */
export function RefusedCallFinding({ finding, showAdminLinks }: RefusedCallFindingProps) {
  const { fix } = finding;
  return (
    <dl
      aria-label={`Refused call: ${finding.capability}`}
      className="mt-2 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 rounded-md border border-danger-border bg-danger-fill p-2 text-sm"
    >
      <dt className="font-medium text-danger-text">What</dt>
      <dd className="min-w-0 break-words text-ink-body">{finding.what}</dd>
      <dt className="font-medium text-danger-text">Why</dt>
      <dd className="min-w-0 break-words text-ink-body">{finding.why}</dd>
      <dt className="font-medium text-danger-text">Fix</dt>
      <dd className="min-w-0 space-y-1 break-words text-ink-body">
        <p>{fix.text}</p>
        {fix.request && (
          <code className="block break-all rounded bg-glass-inset-gray px-1.5 py-1 text-xs text-ink-code">
            {fix.request}
          </code>
        )}
        {fix.href && showAdminLinks && (
          <Link href={fix.href} className="inline-block font-medium text-brand-text underline-offset-2 hover:underline">
            {fix.linkLabel}
          </Link>
        )}
      </dd>
    </dl>
  );
}
