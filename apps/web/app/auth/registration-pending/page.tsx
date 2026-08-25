"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import { CheckCircle2, Clock, Mail } from "lucide-react";
import { AimLogo } from "@/components/sidebar";

function RegistrationPendingContent() {
  const searchParams = useSearchParams();
  const requestId = searchParams.get("request_id");
  const supportEmail = process.env.NEXT_PUBLIC_SUPPORT_EMAIL || "info@opena2a.org";

  return (
    <main className="glass-page relative flex min-h-screen items-center justify-center overflow-hidden p-4">
      <div className="w-full max-w-xl">
        <div className="mb-6 flex flex-col items-center text-center">
          <AimLogo size={48} className="shadow-[0_10px_26px_rgba(56,189,248,0.35)]" />
          <h1 className="mt-4 text-[26px] font-bold tracking-[-0.03em] text-ink">Your request is in.</h1>
          <p className="mt-1 text-sm text-ink-secondary">An administrator reviews new accounts before they can sign in.</p>
        </div>

        <div className="glass-chrome p-6 sm:p-8">
          <div className="flex items-start gap-3 rounded-inset bg-success-fill p-4">
            <CheckCircle2 className="mt-0.5 h-5 w-5 flex-shrink-0 text-success-text" aria-hidden="true" />
            <div>
              <p className="text-sm font-bold text-ink">Registration submitted</p>
              {requestId && (
                <p className="mt-1 text-xs text-ink-secondary">
                  Request ID <span className="break-all font-mono text-ink">{requestId}</span>
                </p>
              )}
            </div>
          </div>

          <h2 className="mt-6 text-[15px] font-bold tracking-[-0.02em] text-ink">What happens next</h2>
          <ol className="mt-3 space-y-3">
            <li className="flex items-start gap-3">
              <span className="mt-0.5 inline-flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full bg-brand-soft text-brand-text">
                <Clock className="h-3.5 w-3.5" aria-hidden="true" />
              </span>
              <div>
                <p className="text-sm font-bold text-ink">An administrator reviews the request</p>
                <p className="text-xs text-ink-secondary">Approval happens in this AIM deployment's admin area; the time it takes depends on your administrator.</p>
              </div>
            </li>
            <li className="flex items-start gap-3">
              <span className="mt-0.5 inline-flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full bg-brand-soft text-brand-text">
                <Mail className="h-3.5 w-3.5" aria-hidden="true" />
              </span>
              <div>
                <p className="text-sm font-bold text-ink">Check back by signing in</p>
                <p className="text-xs text-ink-secondary">Once approved, signing in takes you straight to your dashboard. Until then the sign-in page will tell you the request is still pending. If the request is declined you will not receive a message; contact the administrator if you have not been approved.</p>
              </div>
            </li>
          </ol>

          <div className="mt-5 rounded-inset bg-glass-inset-gray px-4 py-3">
            <p className="text-xs leading-relaxed text-ink-secondary">
              Running this deployment yourself? Accounts whose email is listed in the{" "}
              <code className="font-mono text-ink">AIM_PLATFORM_ADMINS</code> environment variable
              (comma-separated, for example{" "}
              <code className="font-mono text-ink">AIM_PLATFORM_ADMINS=you@example.com</code>) are
              approved automatically and become administrators of their own organization.
              Administrators approve everyone else under Admin &gt; Registrations.
            </p>
          </div>

          <div className="mt-6 flex flex-col gap-2 sm:flex-row">
            <Link href="/auth/login" className="inline-flex h-11 flex-1 items-center justify-center rounded-pill bg-brand text-sm font-bold text-white shadow-glow hover:bg-brand-hover">
              Go to sign in
            </Link>
            <a
              href={`mailto:${supportEmail}?subject=${encodeURIComponent("AIM account registration")}`}
              className="inline-flex h-11 flex-1 items-center justify-center rounded-pill border border-stroke bg-glass text-sm font-bold text-ink hover:bg-glass-inset"
            >
              Contact the administrator
            </a>
          </div>
        </div>
      </div>
    </main>
  );
}

export default function RegistrationPendingPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center" aria-busy="true">
          <span className="h-8 w-8 animate-spin rounded-full border-2 border-track border-t-brand" aria-label="Loading" />
        </div>
      }
    >
      <RegistrationPendingContent />
    </Suspense>
  );
}
