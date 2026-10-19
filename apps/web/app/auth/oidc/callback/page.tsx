"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import { AlertCircle, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { AimLogo } from "@/components/sidebar";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { api } from "@/lib/api";
import { SIGN_IN_NEUTRAL_MESSAGE } from "@/lib/sign-in-failure";
import {
  OIDC_PROVIDER_ANSWER_INCOMPLETE_MESSAGE,
  OIDC_SIGN_IN_NOT_STARTED_MESSAGE,
  oidcCallbackFailure,
  providerErrorMessage,
  signInPageHref,
  takeOidcSignIn,
} from "@/lib/oidc-sign-in";

// The page an organization's identity provider redirects the browser to. It sends the
// code and state of the redirect, with the flow this tab kept when the sign-in page
// started the sign-in, to the server, which answers a login pair for the user of that
// organization.
const CALLBACK_PATH = "/auth/oidc/callback";

interface CallbackFailure {
  message: string;
  /** The sign-in page to start again from: the organization's sign-in link when this tab started one. */
  signInHref: string;
}

function OidcCallbackContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const started = useRef(false);
  const [failure, setFailure] = useState<CallbackFailure | null>(null);
  const failureRef = useRef<HTMLDivElement>(null);

  // Move focus to the alert so it is announced and scrolled into view on a small screen.
  useEffect(() => {
    if (failure) failureRef.current?.focus();
  }, [failure]);

  useEffect(() => {
    // Once per page load: the flow leaves storage as it is read, and the development
    // build runs each effect twice.
    if (started.current) return;
    started.current = true;

    const code = searchParams.get("code");
    const state = searchParams.get("state");
    const providerError = searchParams.get("error");
    const pending = takeOidcSignIn();
    // This load spends the code and state, so they leave the address bar and the history.
    window.history.replaceState(null, "", CALLBACK_PATH);

    if (!pending) {
      setFailure({ message: OIDC_SIGN_IN_NOT_STARTED_MESSAGE, signInHref: signInPageHref(null) });
      return;
    }
    const signInHref = signInPageHref(pending.organizationId, pending.returnUrl);
    if (providerError !== null) {
      setFailure({ message: providerErrorMessage(providerError), signInHref });
      return;
    }
    if (!code || !state) {
      setFailure({ message: OIDC_PROVIDER_ANSWER_INCOMPLETE_MESSAGE, signInHref });
      return;
    }

    void (async () => {
      try {
        const answer = await api.completeOidcSignIn({ code, state, flow: pending.flow });
        if (!answer?.accessToken || !answer.user) {
          setFailure({ message: SIGN_IN_NEUTRAL_MESSAGE, signInHref });
          return;
        }
        if (answer.user.forcePasswordChange) {
          // The same step a password sign-in takes for an account that must change its password.
          toast.info("You must change your password before continuing.");
          localStorage.setItem("temp_user_id", answer.user.id);
          localStorage.setItem("temp_user_email", answer.user.email);
          router.replace("/auth/change-password");
          return;
        }
        toast.success("Signed in");
        router.replace(pending.returnUrl);
      } catch (error) {
        setFailure({ message: oidcCallbackFailure(error), signInHref });
      }
    })();
  }, [router, searchParams]);

  return (
    <main className="glass-page relative flex min-h-screen items-center justify-center overflow-hidden p-4">
      <div className="w-full max-w-md">
        <div className="mb-6 flex flex-col items-center text-center">
          <AimLogo size={48} className="shadow-[0_10px_26px_rgba(56,189,248,0.35)]" />
          <h1 className="mt-4 text-2xl font-bold tracking-[-0.03em] text-ink">
            {failure ? "Sign-in did not complete" : "Signing you in"}
          </h1>
          <p className="mt-1 text-sm text-ink-secondary">Through your organization&apos;s identity provider.</p>
        </div>

        <div className="glass-chrome p-6 sm:p-8">
          {failure ? (
            <>
              <Alert
                ref={failureRef}
                tabIndex={-1}
                className="focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <AlertCircle className="h-4 w-4" aria-hidden="true" />
                <AlertTitle>Could not sign in</AlertTitle>
                <AlertDescription>{failure.message}</AlertDescription>
              </Alert>
              <Link
                href={failure.signInHref}
                className="mt-5 inline-flex h-11 w-full items-center justify-center rounded-pill bg-brand text-sm font-bold text-white shadow-glow hover:bg-brand-hover"
              >
                Back to sign in
              </Link>
            </>
          ) : (
            <p role="status" className="flex items-center justify-center gap-2 text-sm text-ink-secondary">
              <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
              Completing the sign-in...
            </p>
          )}
        </div>
      </div>
    </main>
  );
}

export default function OidcCallbackPage() {
  return (
    <Suspense fallback={<div className="flex min-h-screen items-center justify-center text-sm text-ink-secondary">Loading...</div>}>
      <OidcCallbackContent />
    </Suspense>
  );
}
