"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { AlertCircle, CheckCircle2, Eye, EyeOff, Lock, Shield } from "lucide-react";
import { AimLogo } from "@/components/sidebar";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { toast } from "sonner";

export default function ChangePasswordPage() {
  const router = useRouter();
  const [isLoading, setIsLoading] = useState(false);
  const [userEmail, setUserEmail] = useState("");
  const [formData, setFormData] = useState({
    currentPassword: "",
    newPassword: "",
    confirmPassword: "",
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [showCurrentPassword, setShowCurrentPassword] = useState(false);
  const [showNewPassword, setShowNewPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);

  useEffect(() => {
    // Get user info from temporary storage
    const email = localStorage.getItem("temp_user_email");
    if (!email) {
      toast.error("Session expired. Please login again.");
      router.push("/auth/login");
      return;
    }
    setUserEmail(email);
  }, [router]);

  const validatePassword = (password: string): string[] => {
    const issues: string[] = [];
    if (password.length < 8) {
      issues.push("At least 8 characters");
    }
    if (!/[A-Z]/.test(password)) {
      issues.push("One uppercase letter");
    }
    if (!/[a-z]/.test(password)) {
      issues.push("One lowercase letter");
    }
    if (!/[0-9]/.test(password)) {
      issues.push("One number");
    }
    if (!/[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]/.test(password)) {
      issues.push("One special character");
    }
    return issues;
  };

  const validateForm = () => {
    const newErrors: Record<string, string> = {};

    if (!formData.currentPassword) {
      newErrors.currentPassword = "Current password is required";
    }

    if (!formData.newPassword) {
      newErrors.newPassword = "New password is required";
    } else {
      const passwordIssues = validatePassword(formData.newPassword);
      if (passwordIssues.length > 0) {
        newErrors.newPassword = `Password must have: ${passwordIssues.join(", ")}`;
      }
    }

    if (!formData.confirmPassword) {
      newErrors.confirmPassword = "Please confirm your new password";
    } else if (formData.newPassword !== formData.confirmPassword) {
      newErrors.confirmPassword = "Passwords do not match";
    }

    if (formData.newPassword === formData.currentPassword) {
      newErrors.newPassword = "New password must be different from current password";
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validateForm()) return;

    setIsLoading(true);
    setErrors({});

    try {
      const email = localStorage.getItem("temp_user_email");
      if (!email) {
        throw new Error("Session expired");
      }

      // Call API to change password (public endpoint - no auth required)
      await api.changePassword({
        email,
        currentPassword: formData.currentPassword,
        newPassword: formData.newPassword,
      });

      // Clear temporary storage
      localStorage.removeItem("temp_user_id");
      localStorage.removeItem("temp_user_email");

      toast.success("Password changed successfully!", {
        description: "You can now login with your new password.",
      });

      router.push("/auth/login");
    } catch (error: any) {
      let errorMessage = "Failed to change password";

      if (error?.message) {
        errorMessage = error.message;
      } else if (typeof error === "string") {
        errorMessage = error;
      } else if (error?.error) {
        errorMessage = error.error;
      }

      toast.error("Password Change Failed", {
        description: errorMessage,
        duration: 5000,
      });

      setErrors({ currentPassword: errorMessage });
    } finally {
      setIsLoading(false);
    }
  };

  const passwordStrength = formData.newPassword
    ? validatePassword(formData.newPassword)
    : null;

  const inputClass = (invalid: boolean) =>
    `w-full rounded-inset border bg-glass-inset py-2.5 pl-10 pr-10 text-sm text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-0 ${
      invalid ? "border-danger" : "border-stroke"
    }`;

  const field = (
    id: "currentPassword" | "newPassword" | "confirmPassword",
    label: string,
    placeholder: string,
    shown: boolean,
    toggle: () => void,
    autoComplete: string
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
          autoComplete={autoComplete}
          value={formData[id]}
          onChange={(e) => setFormData({ ...formData, [id]: e.target.value })}
          className={inputClass(!!errors[id])}
          placeholder={placeholder}
          aria-invalid={!!errors[id]}
          aria-describedby={errors[id] ? `${id}-error` : undefined}
        />
        <button type="button" onClick={toggle} className="absolute right-3 top-1/2 -translate-y-1/2 text-ink-tertiary hover:text-ink" aria-label={shown ? "Hide password" : "Show password"}>
          {shown ? <EyeOff className="h-4 w-4" aria-hidden="true" /> : <Eye className="h-4 w-4" aria-hidden="true" />}
        </button>
      </div>
      {errors[id] && (
        <p id={`${id}-error`} className="mt-1 flex items-center gap-1 text-xs font-semibold text-danger-text">
          <AlertCircle className="h-3.5 w-3.5" aria-hidden="true" />
          {errors[id]}
        </p>
      )}
    </div>
  );

  return (
    <main className="glass-page relative flex min-h-screen items-center justify-center overflow-hidden p-4">
      <div className="w-full max-w-md">
        <div className="mb-6 flex flex-col items-center text-center">
          <AimLogo size={48} className="shadow-[0_10px_26px_rgba(56,189,248,0.35)]" />
          <h1 className="mt-4 text-[26px] font-bold tracking-[-0.03em] text-ink">Set a new password</h1>
          <p className="mt-1 text-sm text-ink-secondary">
            Your account{userEmail ? ` (${userEmail})` : ""} needs a new password before you continue.
          </p>
        </div>

        <div className="glass-chrome p-6 sm:p-8">
          <form onSubmit={handleSubmit} className="space-y-4" noValidate>
            {field("currentPassword", "Current password", "Enter your current password", showCurrentPassword, () => setShowCurrentPassword((v) => !v), "current-password")}
            {field("newPassword", "New password", "Enter a new password", showNewPassword, () => setShowNewPassword((v) => !v), "new-password")}
            {formData.newPassword && passwordStrength && (
              <ul className="rounded-inset bg-glass-inset-gray p-3 text-xs" aria-live="polite">
                {passwordStrength.length === 0 ? (
                  <li className="flex items-center gap-1.5 font-semibold text-success-text">
                    <CheckCircle2 className="h-3.5 w-3.5" aria-hidden="true" />
                    All requirements met
                  </li>
                ) : (
                  passwordStrength.map((issue) => (
                    <li key={issue} className="flex items-center gap-1.5 text-ink-secondary">
                      <span className="h-1.5 w-1.5 rounded-full bg-warning" aria-hidden="true" />
                      {issue}
                    </li>
                  ))
                )}
              </ul>
            )}
            {field("confirmPassword", "Confirm new password", "Enter it again", showConfirmPassword, () => setShowConfirmPassword((v) => !v), "new-password")}
            <Button type="submit" disabled={isLoading} className="w-full" size="lg">
              {isLoading ? "Saving..." : "Change password"}
            </Button>
          </form>

          <div className="mt-5 flex gap-3 rounded-inset bg-brand-soft p-3.5">
            <Shield className="mt-0.5 h-4 w-4 flex-shrink-0 text-brand-text" aria-hidden="true" />
            <p className="text-xs leading-relaxed text-ink-body">
              At least 8 characters with an uppercase letter, a lowercase letter, a number and a special character, and different from the current one.
            </p>
          </div>

          <p className="mt-5 border-t border-divider pt-4 text-center text-sm">
            <Link href="/auth/login" className="font-semibold text-brand-text hover:underline">
              Back to sign in
            </Link>
          </p>
        </div>
      </div>
    </main>
  );
}
