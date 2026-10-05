import Link from "next/link";
import { AlertTriangle, CheckCircle, X } from "lucide-react";
import type { ActionOutcome as Outcome } from "@/lib/action-outcome";

interface ActionOutcomeProps {
  outcome: Outcome | null;
  onDismiss?: () => void;
  className?: string;
}

/**
 * The outcome of an action, rendered beside the control that caused it. A failure
 * is announced at once (role="alert"); a success is announced politely (role="status").
 */
export function ActionOutcome({ outcome, onDismiss, className = "" }: ActionOutcomeProps) {
  if (!outcome) return null;
  const failed = outcome.kind === "failure";
  const Icon = failed ? AlertTriangle : CheckCircle;

  return (
    <div
      role={failed ? "alert" : "status"}
      className={`flex items-start gap-2 rounded-md border p-3 text-sm ${
        failed ? "border-danger-border bg-danger-fill" : "border-success-border bg-success-fill"
      } ${className}`}
    >
      <Icon
        aria-hidden="true"
        className={`mt-0.5 h-4 w-4 flex-shrink-0 ${failed ? "text-danger-text" : "text-success-text"}`}
      />
      <div className="min-w-0 flex-1 space-y-1 break-words text-ink-body">
        {failed ? (
          <>
            <p className="font-medium text-danger-text">{outcome.reason}</p>
            <p>
              {outcome.nextStep}
              {outcome.link && (
                <>
                  {" "}
                  <Link
                    href={outcome.link.href}
                    className="font-medium text-brand-text underline-offset-2 hover:underline"
                  >
                    {outcome.link.label}
                  </Link>
                </>
              )}
            </p>
          </>
        ) : (
          <p className="font-medium text-success-text">{outcome.message}</p>
        )}
      </div>
      {onDismiss && (
        <button
          type="button"
          onClick={onDismiss}
          aria-label="Dismiss"
          className="-m-1 rounded p-1 text-ink-body hover:bg-glass-inset-gray hover:text-ink focus-visible:outline focus-visible:outline-2"
        >
          <X aria-hidden="true" className="h-4 w-4" />
        </button>
      )}
    </div>
  );
}
