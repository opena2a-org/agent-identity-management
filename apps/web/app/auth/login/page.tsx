"use client";

import { useEffect, useRef, useState, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import { AlertCircle, Building2, CheckCircle2, Eye, EyeOff, Lock, Mail } from "lucide-react";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { AimLogo } from "@/components/sidebar";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { safeReturnUrl } from "@/lib/return-url";
import { SIGN_IN_NEUTRAL_MESSAGE, signInFailure, type SignInFailure } from "@/lib/sign-in-failure";
import {
  OIDC_PROVIDER_ADDRESS_REFUSED_MESSAGE,
  oidcStartFailure,
  organizationIdParam,
  providerAuthorizationUrl,
  rememberOidcSignIn,
} from "@/lib/oidc-sign-in";

function LoginPageContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const returnUrl = safeReturnUrl(searchParams.get("returnUrl"));
  // An organization's sign-in link carries its id; the page then offers sign-in through
  // that organization's identity provider beside the password form.
  const organizationId = organizationIdParam(searchParams.get("organization"));
  const [isStartingProviderSignIn, setIsStartingProviderSignIn] = useState(false);
  const [isLoadingPassword, setIsLoadingPassword] = useState(false);
  const [formData, setFormData] = useState({
    email: "",
    password: "",
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  // A server-side failure, shown once in the alert at the top of the card. It stays until
  // the next submit; field errors under the inputs are for client-side validation only.
  const [failure, setFailure] = useState<SignInFailure | null>(null);
  const failureRef = useRef<HTMLDivElement>(null);
  const [showPassword, setShowPassword] = useState(false);

  // Move focus to the alert so it is announced and scrolled into view on a small screen.
  useEffect(() => {
    if (failure) failureRef.current?.focus();
  }, [failure]);

  // Back from the provider, a page restored from the back-forward cache gets its
  // provider sign-in button back.
  useEffect(() => {
    const onPageShow = (event: PageTransitionEvent) => {
      if (event.persisted) setIsStartingProviderSignIn(false);
    };
    window.addEventListener("pageshow", onPageShow);
    return () => window.removeEventListener("pageshow", onPageShow);
  }, []);

  const validateForm = () => {
    const newErrors: Record<string, string> = {};

    if (!formData.email) {
      newErrors.email = "Email is required";
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.email)) {
      newErrors.email = "Invalid email address";
    }

    if (!formData.password) {
      newErrors.password = "Password is required";
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handlePasswordLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setFailure(null);

    if (!validateForm()) return;

    setIsLoadingPassword(true);
    setErrors({});

    try {
      const response = await api.loginWithPassword({
        email: formData.email,
        password: formData.password,
      });

      // Check if user must change password (security policy requirement)
      if (response.requiresPasswordChange || response.user?.forcePasswordChange) {
        toast.info("You must change your password before continuing.");
        // Store user info temporarily for password change flow
        if (response.user) {
          localStorage.setItem('temp_user_id', response.user.id);
          localStorage.setItem('temp_user_email', response.user.email);
        }
        router.push("/auth/change-password");
        return;
      }

      if (response.success) {
        if (response.isApproved) {
          // User is approved, redirect to return URL or dashboard
          toast.success("Signed in");
          router.push(returnUrl);
        } else {
          // User exists but not approved yet - redirect to pending page
          toast.info("Your account is pending admin approval.");
          router.push("/auth/registration-pending");
        }
      }
    } catch (error) {
      setFailure(signInFailure(error));
    } finally {
      setIsLoadingPassword(false);
    }
  };

  // The flow goes into this tab's storage before the browser leaves for the provider:
  // the callback page sends it back, and a tab that holds none completes no sign-in.
  const handleProviderSignIn = async () => {
    if (!organizationId) return;
    setFailure(null);
    setIsStartingProviderSignIn(true);
    try {
      const start = await api.startOidcSignIn(organizationId);
      const destination = providerAuthorizationUrl(start?.authorizationUrl);
      const flow = typeof start?.flow === "string" ? start.flow : "";
      if (!destination || !flow) {
        const message = destination ? SIGN_IN_NEUTRAL_MESSAGE : OIDC_PROVIDER_ADDRESS_REFUSED_MESSAGE;
        setFailure({ message, marksCredentials: false });
        setIsStartingProviderSignIn(false);
        return;
      }
      rememberOidcSignIn({ organizationId, flow, returnUrl });
      // The button stays disabled while the browser leaves the page.
      window.location.assign(destination);
    } catch (error) {
      setFailure({ message: oidcStartFailure(error), marksCredentials: false });
      setIsStartingProviderSignIn(false);
    }
  };

  const credentialsRefused = !!failure?.marksCredentials;

  const inputClass = (invalid: boolean) =>
    `w-full rounded-inset border bg-glass-inset py-2.5 pl-10 pr-10 text-sm text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-0 ${
      invalid ? "border-danger" : "border-stroke"
    }`;

  return (
    <main className="glass-page relative flex min-h-screen items-center justify-center overflow-hidden p-4">
      <div className="w-full max-w-md">
        <div className="mb-6 flex flex-col items-center text-center">
          <AimLogo size={48} className="shadow-[0_10px_26px_rgba(56,189,248,0.35)]" />
          <h1 className="mt-4 text-2xl font-bold tracking-[-0.03em] text-ink">Welcome back</h1>
          <p className="mt-1 text-sm text-ink-secondary">Sign in to manage your agents and MCP servers.</p>
        </div>

        <div className="glass-chrome p-6 sm:p-8">
          {failure && (
            <Alert
              ref={failureRef}
              tabIndex={-1}
              variant={failure.marksCredentials ? "destructive" : "default"}
              className="mb-4 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              <AlertCircle className="h-4 w-4" aria-hidden="true" />
              <AlertTitle>Could not sign in</AlertTitle>
              <AlertDescription id="sign-in-failure">{failure.message}</AlertDescription>
            </Alert>
          )}
          {organizationId && (
            <>
              <Button
                type="button"
                variant="outline"
                size="lg"
                onClick={handleProviderSignIn}
                disabled={isStartingProviderSignIn}
                className="h-auto min-h-11 w-full whitespace-normal py-2.5"
              >
                <Building2 className="h-4 w-4 flex-shrink-0" aria-hidden="true" />
                {isStartingProviderSignIn ? "Opening your identity provider..." : "Sign in with your identity provider"}
              </Button>
              <div className="my-5 flex items-center gap-3">
                <span className="h-px flex-1 bg-divider" aria-hidden="true" />
                <span className="text-sm text-ink-secondary">or with your email and password</span>
                <span className="h-px flex-1 bg-divider" aria-hidden="true" />
              </div>
            </>
          )}
          <form onSubmit={handlePasswordLogin} className="space-y-4" noValidate>
            <div>
              <label htmlFor="email" className="mb-1 block text-sm font-semibold text-ink-body">
                Email address
              </label>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-ink-tertiary" aria-hidden="true" />
                <input
                  id="email"
                  type="email"
                  autoComplete="email"
                  value={formData.email}
                  onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                  className={inputClass(!!errors.email || credentialsRefused)}
                  placeholder="you@example.com"
                  aria-invalid={!!errors.email || credentialsRefused}
                  aria-describedby={errors.email ? "email-error" : credentialsRefused ? "sign-in-failure" : undefined}
                />
              </div>
              {errors.email && (
                <p id="email-error" className="mt-1 flex items-center gap-1 text-sm font-semibold text-danger-text">
                  <AlertCircle className="h-3.5 w-3.5" aria-hidden="true" />
                  {errors.email}
                </p>
              )}
            </div>

            <div>
              <div className="mb-1 flex items-center justify-between">
                <label htmlFor="password" className="block text-sm font-semibold text-ink-body">
                  Password
                </label>
                <Link href="/auth/forgot-password" className="text-sm font-semibold text-brand-text hover:underline">
                  Forgot password?
                </Link>
              </div>
              <div className="relative">
                <Lock className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-ink-tertiary" aria-hidden="true" />
                <input
                  id="password"
                  type={showPassword ? "text" : "password"}
                  autoComplete="current-password"
                  value={formData.password}
                  onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                  className={inputClass(!!errors.password || credentialsRefused)}
                  placeholder="Enter your password"
                  aria-invalid={!!errors.password || credentialsRefused}
                  aria-describedby={errors.password ? "password-error" : credentialsRefused ? "sign-in-failure" : undefined}
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
              {errors.password && (
                <p id="password-error" className="mt-1 flex items-center gap-1 text-sm font-semibold text-danger-text">
                  <AlertCircle className="h-3.5 w-3.5" aria-hidden="true" />
                  {errors.password}
                </p>
              )}
            </div>

            <Button type="submit" disabled={isLoadingPassword} className="w-full" size="lg">
              {isLoadingPassword ? "Signing in..." : "Sign in"}
            </Button>
          </form>

          <div className="mt-5 flex gap-3 rounded-inset bg-brand-soft p-3.5">
            <CheckCircle2 className="mt-0.5 h-4 w-4 flex-shrink-0 text-brand-text" aria-hidden="true" />
            <p className="text-sm leading-relaxed text-ink-body">
              New accounts are reviewed by an administrator before they can sign in. Passwords are stored as bcrypt hashes, never in plain text.
            </p>
          </div>

          <p className="mt-5 border-t border-divider pt-4 text-center text-sm text-ink-secondary">
            Don&apos;t have an account?{" "}
            <Link href="/auth/register" className="font-semibold text-brand-text hover:underline">
              Create one
            </Link>
          </p>
        </div>
      </div>
    </main>
  );
}

export default function LoginPage() {
  return (
    <Suspense fallback={<div className="flex min-h-screen items-center justify-center text-sm text-ink-secondary">Loading...</div>}>
      <LoginPageContent />
    </Suspense>
  );
}
