"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { AlertCircle, CheckCircle2, Eye, EyeOff, Lock } from "lucide-react";
import { AimLogo } from "@/components/sidebar";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { api, type AccountDeletion } from "@/lib/api";
import {
  DELETE_ACCOUNT_PATH,
  accountDeletionFailure,
  type AccountDeletionFailure,
} from "@/lib/account-deletion";
import { USERS_PAGE_HREF } from "@/lib/alert-resource";
import { formatDateTime } from "@/lib/date-utils";
import { decodeJwtPayload } from "@/lib/jwt-payload";

// Where a visitor without a session is sent: the sign-in page, which returns here.
const SIGN_IN_AND_RETURN = `/auth/login?returnUrl=${encodeURIComponent(DELETE_ACCOUNT_PATH)}`;

export default function DeleteAccountPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [passwordError, setPasswordError] = useState<string | null>(null);
  // A refusal or a request that did not complete, shown once in the alert at the top of
  // the card. It stays until the next submit.
  const [failure, setFailure] = useState<AccountDeletionFailure | null>(null);
  const failureRef = useRef<HTMLDivElement>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [deletion, setDeletion] = useState<AccountDeletion | null>(null);

  // The request is made with the stored session; without one there is no account to ask for.
  useEffect(() => {
    const token = api.getToken();
    if (!token) {
      router.replace(SIGN_IN_AND_RETURN);
      return;
    }
    setEmail(decodeJwtPayload(token)?.email ?? "");
  }, [router]);

  // Move focus to the alert so it is announced and scrolled into view on a small screen.
  useEffect(() => {
    if (failure) failureRef.current?.focus();
  }, [failure]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setFailure(null);
    if (!password) {
      setPasswordError("Enter your password");
      return;
    }
    setPasswordError(null);
    setIsSubmitting(true);
    try {
      setDeletion(await api.requestAccountDeletion(password));
      setPassword("");
    } catch (error) {
      setFailure(accountDeletionFailure(error, !!api.getToken()));
    } finally {
      setIsSubmitting(false);
    }
  };

  const passwordInvalid = !!passwordError || !!failure?.marksPassword;

  return (
    <main className="glass-page relative flex min-h-screen items-center justify-center overflow-hidden p-4">
      <div className="w-full max-w-md">
        <div className="mb-6 flex flex-col items-center text-center">
          <AimLogo size={48} className="shadow-[0_10px_26px_rgba(56,189,248,0.35)]" />
          <h1 className="mt-4 text-2xl font-bold tracking-[-0.03em] text-ink">
            {deletion ? "Your account is deactivated" : "Delete your account"}
          </h1>
          {!deletion && email && <p className="mt-1 text-sm text-ink-secondary">Signed in as {email}</p>}
        </div>

        <div className="glass-chrome p-6 sm:p-8">
          {deletion ? (
            <div className="space-y-4" role="status">
              <p className="flex gap-2 text-sm leading-relaxed text-ink-body">
                <CheckCircle2 className="mt-0.5 h-4 w-4 flex-shrink-0 text-success-text" aria-hidden="true" />
                <span>
                  It is scheduled for deletion on {formatDateTime(deletion.deletionDueAt)}, and you are signed out.
                </span>
              </p>
              <p className="text-sm leading-relaxed text-ink-body">
                Until then, an admin of your organization can activate your account again, which cancels the deletion.
              </p>
              <p className="border-t border-divider pt-4 text-center text-sm">
                <Link href="/auth/login" className="font-semibold text-brand-text hover:underline">
                  Go to sign in
                </Link>
              </p>
            </div>
          ) : (
            <>
              {failure && (
                <Alert
                  ref={failureRef}
                  tabIndex={-1}
                  variant="destructive"
                  className="mb-4 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >
                  <AlertCircle className="h-4 w-4" aria-hidden="true" />
                  <AlertTitle>Could not request the deletion</AlertTitle>
                  <AlertDescription id="deletion-failure">
                    <p>{failure.message}</p>
                    {failure.linksUsers && (
                      <Link href={USERS_PAGE_HREF} className="mt-1 inline-block font-semibold underline">
                        Open Users
                      </Link>
                    )}
                  </AlertDescription>
                </Alert>
              )}

              <ul className="space-y-2 rounded-inset bg-glass-inset-gray p-3.5 text-sm leading-relaxed text-ink-body">
                <li>Your account is deactivated at once, and every session of it ends.</li>
                <li>
                  It is deleted 29 days later, with the API keys you created. The agents and MCP servers you created stay
                  with your organization.
                </li>
                <li>
                  If no other user is left in your organization then, the organization is deleted too, with its agents and
                  MCP servers.
                </li>
                <li>Until then, an admin of your organization can activate your account again, which cancels the deletion.</li>
              </ul>

              <form onSubmit={handleSubmit} className="mt-4 space-y-4" noValidate>
                <div>
                  <label htmlFor="password" className="mb-1 block text-sm font-semibold text-ink-body">
                    Password
                  </label>
                  <div className="relative">
                    <Lock className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-ink-tertiary" aria-hidden="true" />
                    <input
                      id="password"
                      type={showPassword ? "text" : "password"}
                      autoComplete="current-password"
                      value={password}
                      onChange={(e) => setPassword(e.target.value)}
                      className={`w-full rounded-inset border bg-glass-inset py-2.5 pl-10 pr-10 text-sm text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-0 ${
                        passwordInvalid ? "border-danger" : "border-stroke"
                      }`}
                      placeholder="The password you sign in with"
                      aria-invalid={passwordInvalid}
                      aria-describedby={passwordError ? "password-error" : failure ? "deletion-failure" : undefined}
                    />
                    <button
                      type="button"
                      onClick={() => setShowPassword((v) => !v)}
                      className="absolute right-0 top-1/2 inline-flex h-11 w-11 -translate-y-1/2 items-center justify-center text-ink-tertiary hover:text-ink"
                      aria-label={showPassword ? "Hide password" : "Show password"}
                    >
                      {showPassword ? <EyeOff className="h-4 w-4" aria-hidden="true" /> : <Eye className="h-4 w-4" aria-hidden="true" />}
                    </button>
                  </div>
                  {passwordError && (
                    <p id="password-error" className="mt-1 flex items-center gap-1 text-sm font-semibold text-danger-text">
                      <AlertCircle className="h-3.5 w-3.5" aria-hidden="true" />
                      {passwordError}
                    </p>
                  )}
                </div>
                <Button type="submit" variant="destructive" disabled={isSubmitting} className="w-full" size="lg">
                  {isSubmitting ? "Sending request..." : "Delete my account"}
                </Button>
              </form>

              <p className="mt-5 border-t border-divider pt-4 text-center text-sm">
                <Link href="/dashboard" className="font-semibold text-brand-text hover:underline">
                  Keep my account
                </Link>
              </p>
            </>
          )}
        </div>
      </div>
    </main>
  );
}
