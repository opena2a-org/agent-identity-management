"use client";

import { useState, useEffect, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import { AlertCircle, CheckCircle2, Eye, EyeOff, Lock, Shield, XCircle } from "lucide-react";
import { AimLogo } from "@/components/sidebar";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { toast } from "sonner";

function ResetPasswordPageContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const token = searchParams.get("token");

  const [formData, setFormData] = useState({
    newPassword: "",
    confirmPassword: "",
  });
  const [isLoading, setIsLoading] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [showNewPassword, setShowNewPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  const [isSuccess, setIsSuccess] = useState(false);
  const [tokenValid, setTokenValid] = useState(true);

  useEffect(() => {
    if (!token) {
      setTokenValid(false);
    }
  }, [token]);

  const validateForm = () => {
    const newErrors: Record<string, string> = {};

    if (!formData.newPassword) {
      newErrors.newPassword = "Password is required";
    } else if (formData.newPassword.length < 8) {
      newErrors.newPassword = "Password must be at least 8 characters";
    } else if (!/(?=.*[a-z])(?=.*[A-Z])(?=.*\d)/.test(formData.newPassword)) {
      newErrors.newPassword =
        "Password must contain uppercase, lowercase, and number";
    }

    if (!formData.confirmPassword) {
      newErrors.confirmPassword = "Please confirm your password";
    } else if (formData.newPassword !== formData.confirmPassword) {
      newErrors.confirmPassword = "Passwords do not match";
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validateForm() || !token) return;

    setIsLoading(true);
    setErrors({});

    try {
      const response = await api.resetPassword({
        resetToken: token,
        newPassword: formData.newPassword,
        confirmPassword: formData.confirmPassword,
      });

      if (response.success) {
        setIsSuccess(true);
        toast.success("Password updated", {
          description: "Sign in with your new password.",
        });

        // Redirect to login after 3 seconds
        setTimeout(() => {
          router.push("/auth/login");
        }, 3000);
      }
    } catch (error: any) {
      let errorMessage = "Failed to reset password";

      if (error?.message) {
        errorMessage = error.message;
      } else if (typeof error === "string") {
        errorMessage = error;
      } else if (error?.error) {
        errorMessage = error.error;
      }

      // Check if token is invalid or expired
      if (
        errorMessage.toLowerCase().includes("invalid") ||
        errorMessage.toLowerCase().includes("expired")
      ) {
        setTokenValid(false);
      }

      toast.error("Could not reset the password", {
        description: errorMessage,
        duration: 5000,
      });

      setErrors({ newPassword: errorMessage });
    } finally {
      setIsLoading(false);
    }
  };

  const inputClass = (invalid: boolean) =>
    `w-full rounded-inset border bg-glass-inset py-2.5 pl-10 pr-10 text-sm text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-0 ${
      invalid ? "border-danger" : "border-stroke"
    }`;

  // Invalid or missing token
  if (!tokenValid) {
    return (
      <main className="glass-page relative flex min-h-screen items-center justify-center overflow-hidden p-4">
        <div className="w-full max-w-md">
          <div className="mb-6 flex flex-col items-center text-center">
            <span className="inline-flex h-12 w-12 items-center justify-center rounded-full bg-danger-fill text-danger-text">
              <XCircle className="h-6 w-6" aria-hidden="true" />
            </span>
            <h1 className="mt-4 text-[26px] font-bold tracking-[-0.03em] text-ink">This reset link no longer works</h1>
            <p className="mt-1 text-sm text-ink-secondary">Reset links expire one hour after they are sent.</p>
          </div>
          <div className="glass-chrome p-6 sm:p-8">
            <p className="text-sm font-bold text-ink">What to do</p>
            <ul className="mt-2 space-y-1.5 text-xs text-ink-body">
              <li>Request a new link and open the most recent email.</li>
              <li>Use the link within an hour of receiving it.</li>
            </ul>
            <div className="mt-6 flex flex-col gap-2">
              <Link href="/auth/forgot-password" className="inline-flex h-11 items-center justify-center rounded-pill bg-brand text-sm font-bold text-white shadow-glow hover:bg-brand-hover">
                Request a new link
              </Link>
              <Link href="/auth/login" className="inline-flex h-11 items-center justify-center rounded-pill border border-stroke bg-glass text-sm font-bold text-ink hover:bg-glass-inset">
                Back to sign in
              </Link>
            </div>
          </div>
        </div>
      </main>
    );
  }

  // Success state
  if (isSuccess) {
    return (
      <main className="glass-page relative flex min-h-screen items-center justify-center overflow-hidden p-4">
        <div className="w-full max-w-md">
          <div className="mb-6 flex flex-col items-center text-center">
            <span className="inline-flex h-12 w-12 items-center justify-center rounded-full bg-success-fill text-success-text">
              <CheckCircle2 className="h-6 w-6" aria-hidden="true" />
            </span>
            <h1 className="mt-4 text-[26px] font-bold tracking-[-0.03em] text-ink">Password updated</h1>
            <p className="mt-1 text-sm text-ink-secondary">You can sign in with the new password now.</p>
          </div>
          <div className="glass-chrome p-6 text-center sm:p-8" role="status" aria-live="polite">
            <p className="text-xs text-ink-secondary">Taking you to sign in in a few seconds.</p>
            <Link href="/auth/login" className="mt-4 inline-flex h-11 w-full items-center justify-center rounded-pill bg-brand text-sm font-bold text-white shadow-glow hover:bg-brand-hover">
              Go to sign in now
            </Link>
          </div>
        </div>
      </main>
    );
  }

  const passwordField = (
    id: "newPassword" | "confirmPassword",
    label: string,
    placeholder: string,
    shown: boolean,
    toggle: () => void,
    hint?: string
  ) => (
    <div>
      <label htmlFor={id} className="mb-1 block text-xs font-semibold text-ink-body">
        {label}
      </label>
      <div className="relative">
        <Lock className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-ink-tertiary" aria-hidden="true" />
        <input
          id={id}
          type={shown ? "text" : "password"}
          autoComplete="new-password"
          value={formData[id]}
          onChange={(e) => setFormData({ ...formData, [id]: e.target.value })}
          className={inputClass(!!errors[id])}
          placeholder={placeholder}
          autoFocus={id === "newPassword"}
          aria-invalid={!!errors[id]}
          aria-describedby={errors[id] ? `${id}-error` : hint ? `${id}-hint` : undefined}
        />
        <button type="button" onClick={toggle} className="absolute right-3 top-1/2 -translate-y-1/2 text-ink-tertiary hover:text-ink" aria-label={shown ? "Hide password" : "Show password"}>
          {shown ? <EyeOff className="h-4 w-4" aria-hidden="true" /> : <Eye className="h-4 w-4" aria-hidden="true" />}
        </button>
      </div>
      {errors[id] ? (
        <p id={`${id}-error`} className="mt-1 flex items-center gap-1 text-xs font-semibold text-danger-text">
          <AlertCircle className="h-3.5 w-3.5" aria-hidden="true" />
          {errors[id]}
        </p>
      ) : hint ? (
        <p id={`${id}-hint`} className="mt-1 text-xs text-ink-tertiary">{hint}</p>
      ) : null}
    </div>
  );

  // Reset password form
  return (
    <main className="glass-page relative flex min-h-screen items-center justify-center overflow-hidden p-4">
      <div className="w-full max-w-md">
        <div className="mb-6 flex flex-col items-center text-center">
          <AimLogo size={48} className="shadow-[0_10px_26px_rgba(56,189,248,0.35)]" />
          <h1 className="mt-4 text-[26px] font-bold tracking-[-0.03em] text-ink">Choose a new password</h1>
          <p className="mt-1 text-sm text-ink-secondary">At least 8 characters with an uppercase letter, a lowercase letter and a number.</p>
        </div>

        <div className="glass-chrome p-6 sm:p-8">
          <form onSubmit={handleSubmit} className="space-y-4" noValidate>
            {passwordField("newPassword", "New password", "Enter a new password", showNewPassword, () => setShowNewPassword((v) => !v))}
            {passwordField("confirmPassword", "Confirm password", "Enter it again", showConfirmPassword, () => setShowConfirmPassword((v) => !v))}
            <Button type="submit" disabled={isLoading} className="w-full" size="lg">
              {isLoading ? "Saving..." : "Set new password"}
            </Button>
          </form>
          <div className="mt-5 flex gap-3 rounded-inset bg-brand-soft p-3.5">
            <Shield className="mt-0.5 h-4 w-4 flex-shrink-0 text-brand-text" aria-hidden="true" />
            <p className="text-xs leading-relaxed text-ink-body">Passwords are stored as bcrypt hashes. This reset link expires one hour after it was sent.</p>
          </div>
        </div>
      </div>
    </main>
  );
}

export default function ResetPasswordPage() {
  return (
    <Suspense fallback={<div className="flex min-h-screen items-center justify-center text-sm text-ink-secondary">Loading...</div>}>
      <ResetPasswordPageContent />
    </Suspense>
  );
}
