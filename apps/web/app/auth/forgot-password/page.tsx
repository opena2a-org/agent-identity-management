"use client";

import { useState } from "react";
import Link from "next/link";
import { AlertCircle, ArrowLeft, CheckCircle2, Mail, Shield } from "lucide-react";
import { AimLogo } from "@/components/sidebar";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { toast } from "sonner";

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isSubmitted, setIsSubmitted] = useState(false);
  const [error, setError] = useState("");

  const validateEmail = () => {
    if (!email) {
      setError("Email is required");
      return false;
    }
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      setError("Invalid email address");
      return false;
    }
    setError("");
    return true;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validateEmail()) return;

    setIsLoading(true);
    setError("");

    try {
      const response = await api.forgotPassword({ email });

      if (response.success) {
        setIsSubmitted(true);
        toast.success("Check your email", {
          description: "If an account exists, you'll receive a password reset link.",
        });
      }
    } catch (error: any) {
      // For security, we don't reveal if the email exists or not
      // So even on error, we show success
      setIsSubmitted(true);
      toast.success("Check your email", {
        description: "If an account exists, you'll receive a password reset link.",
      });
    } finally {
      setIsLoading(false);
    }
  };

  const inputClass = (invalid: boolean) =>
    `w-full rounded-inset border bg-glass-inset py-2.5 pl-10 pr-4 text-sm text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-0 ${
      invalid ? "border-danger" : "border-stroke"
    }`;

  if (isSubmitted) {
    return (
      <main className="glass-page relative flex min-h-screen items-center justify-center overflow-hidden p-4">
        <div className="w-full max-w-md">
          <div className="mb-6 flex flex-col items-center text-center">
            <span className="inline-flex h-12 w-12 items-center justify-center rounded-full bg-success-fill text-success-text">
              <CheckCircle2 className="h-6 w-6" aria-hidden="true" />
            </span>
            <h1 className="mt-4 text-[26px] font-bold tracking-[-0.03em] text-ink">Check your email</h1>
            <p className="mt-1 text-sm text-ink-secondary">
              If an account exists for <span className="font-semibold text-ink">{email}</span>, a reset link is on its way.
            </p>
          </div>

          <div className="glass-chrome p-6 sm:p-8">
            <p className="text-sm font-bold text-ink">Next steps</p>
            <ul className="mt-2 space-y-1.5 text-xs text-ink-body">
              <li>Open the message from Agent Identity Management and follow the reset link.</li>
              <li>The link expires after one hour; request another if it has.</li>
              <li>Check your spam folder if nothing arrives.</li>
            </ul>
            <div className="mt-6 flex flex-col gap-2">
              <Link href="/auth/login" className="inline-flex h-11 items-center justify-center rounded-pill bg-brand text-sm font-bold text-white shadow-accent hover:bg-brand-hover">
                Back to sign in
              </Link>
              <button
                type="button"
                onClick={() => {
                  setIsSubmitted(false);
                  setEmail("");
                }}
                className="inline-flex h-11 items-center justify-center rounded-pill border border-stroke bg-glass text-sm font-bold text-ink hover:bg-glass-inset"
              >
                Send another email
              </button>
            </div>
          </div>
        </div>
      </main>
    );
  }

  return (
    <main className="glass-page relative flex min-h-screen items-center justify-center overflow-hidden p-4">
      <div className="w-full max-w-md">
        <div className="mb-6 flex flex-col items-center text-center">
          <AimLogo size={48} className="shadow-[0_10px_26px_rgba(56,189,248,0.35)]" />
          <h1 className="mt-4 text-[26px] font-bold tracking-[-0.03em] text-ink">Forgot your password?</h1>
          <p className="mt-1 text-sm text-ink-secondary">Enter your email and we will send a reset link.</p>
        </div>

        <div className="glass-chrome p-6 sm:p-8">
          <form onSubmit={handleSubmit} className="space-y-5" noValidate>
            <div>
              <label htmlFor="email" className="mb-1 block text-xs font-semibold text-ink-body">
                Email address
              </label>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-ink-tertiary" aria-hidden="true" />
                <input
                  id="email"
                  type="email"
                  autoComplete="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  onBlur={validateEmail}
                  className={inputClass(!!error)}
                  placeholder="you@example.com"
                  autoFocus
                  aria-invalid={!!error}
                  aria-describedby={error ? "email-error" : "email-hint"}
                />
              </div>
              {error ? (
                <p id="email-error" className="mt-1 flex items-center gap-1 text-xs font-semibold text-danger-text">
                  <AlertCircle className="h-3.5 w-3.5" aria-hidden="true" />
                  {error}
                </p>
              ) : (
                <p id="email-hint" className="mt-1 text-xs text-ink-tertiary">The address you sign in with.</p>
              )}
            </div>
            <Button type="submit" disabled={isLoading} className="w-full" size="lg">
              {isLoading ? "Sending..." : "Send reset link"}
            </Button>
          </form>

          <div className="mt-5 flex gap-3 rounded-inset bg-brand-soft p-3.5">
            <Shield className="mt-0.5 h-4 w-4 flex-shrink-0 text-brand-text" aria-hidden="true" />
            <p className="text-xs leading-relaxed text-ink-body">
              The response is the same whether or not an account exists, so an address cannot be probed here. A message is sent only to registered accounts.
            </p>
          </div>

          <p className="mt-5 border-t border-divider pt-4 text-center text-sm">
            <Link href="/auth/login" className="inline-flex items-center gap-1.5 font-semibold text-brand-text hover:underline">
              <ArrowLeft className="h-3.5 w-3.5" aria-hidden="true" />
              Back to sign in
            </Link>
          </p>
        </div>
      </div>
    </main>
  );
}
