"use client";

import { useState, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import { AlertCircle, CheckCircle2, Eye, EyeOff, Lock, Mail } from "lucide-react";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { AimLogo } from "@/components/sidebar";
import { Button } from "@/components/ui/button";
import { safeReturnUrl } from "@/lib/return-url";

function LoginPageContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const returnUrl = safeReturnUrl(searchParams.get("returnUrl"));
  const [isLoadingPassword, setIsLoadingPassword] = useState(false);
  const [formData, setFormData] = useState({
    email: "",
    password: "",
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [showPassword, setShowPassword] = useState(false);

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
    } catch (error: any) {
      // Extract error message from different possible error formats
      let errorMessage = "Invalid email or password";

      if (error?.message) {
        errorMessage = error.message;
      } else if (typeof error === "string") {
        errorMessage = error;
      } else if (error?.error) {
        errorMessage = error.error;
      }

      // Show toast notification with the exact backend error
      toast.error("Could not sign in", {
        description: errorMessage,
        duration: 5000,
      });

      setErrors({ password: errorMessage });
    } finally {
      setIsLoadingPassword(false);
    }
  };

  const inputClass = (invalid: boolean) =>
    `w-full rounded-inset border bg-glass-inset py-2.5 pl-10 pr-10 text-sm text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-0 ${
      invalid ? "border-danger" : "border-stroke"
    }`;

  return (
    <main className="glass-page relative flex min-h-screen items-center justify-center overflow-hidden p-4">
      <div className="w-full max-w-md">
        <div className="mb-6 flex flex-col items-center text-center">
          <AimLogo size={48} className="shadow-[0_10px_26px_rgba(56,189,248,0.35)]" />
          <h1 className="mt-4 text-[26px] font-bold tracking-[-0.03em] text-ink">Welcome back</h1>
          <p className="mt-1 text-sm text-ink-secondary">Sign in to manage your agents and MCP servers.</p>
        </div>

        <div className="glass-chrome p-6 sm:p-8">
          <form onSubmit={handlePasswordLogin} className="space-y-4" noValidate>
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
                  value={formData.email}
                  onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                  className={inputClass(!!errors.email)}
                  placeholder="you@example.com"
                  aria-invalid={!!errors.email}
                  aria-describedby={errors.email ? "email-error" : undefined}
                />
              </div>
              {errors.email && (
                <p id="email-error" className="mt-1 flex items-center gap-1 text-xs font-semibold text-danger-text">
                  <AlertCircle className="h-3.5 w-3.5" aria-hidden="true" />
                  {errors.email}
                </p>
              )}
            </div>

            <div>
              <div className="mb-1 flex items-center justify-between">
                <label htmlFor="password" className="block text-xs font-semibold text-ink-body">
                  Password
                </label>
                <Link href="/auth/forgot-password" className="text-xs font-semibold text-brand-text hover:underline">
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
                  className={inputClass(!!errors.password)}
                  placeholder="Enter your password"
                  aria-invalid={!!errors.password}
                  aria-describedby={errors.password ? "password-error" : undefined}
                />
                <button
                  type="button"
                  onClick={() => setShowPassword((v) => !v)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-ink-tertiary hover:text-ink"
                  aria-label={showPassword ? "Hide password" : "Show password"}
                >
                  {showPassword ? <EyeOff className="h-4 w-4" aria-hidden="true" /> : <Eye className="h-4 w-4" aria-hidden="true" />}
                </button>
              </div>
              {errors.password && (
                <p id="password-error" className="mt-1 flex items-center gap-1 text-xs font-semibold text-danger-text">
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
            <p className="text-xs leading-relaxed text-ink-body">
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
