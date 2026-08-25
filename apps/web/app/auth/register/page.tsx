"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import {
  Shield,
  Mail,
  User,
  Lock,
  AlertCircle,
  Eye,
  EyeOff,
} from "lucide-react";
import Link from "next/link";
import { api } from "@/lib/api";
import { toast } from "sonner";

// Values must match the backend vocabulary in domain/signup_profile.go
const ROLE_OPTIONS = [
  { value: "developer", label: "Developer" },
  { value: "security-engineer", label: "Security engineer" },
  { value: "founder-or-exec", label: "Founder or executive" },
  { value: "student-or-researcher", label: "Student or researcher" },
  { value: "other", label: "Other" },
];

const USE_CASE_OPTIONS = [
  { value: "securing-production-agents", label: "Securing AI agents in production" },
  { value: "evaluating-for-team", label: "Evaluating AIM for my team" },
  { value: "research-or-learning", label: "Research or learning" },
  { value: "personal-project", label: "Personal project" },
  { value: "other", label: "Other" },
];

const REFERRAL_OPTIONS = [
  { value: "github", label: "GitHub" },
  { value: "search", label: "Search engine" },
  { value: "social-media", label: "Social media" },
  { value: "colleague-or-friend", label: "Colleague or friend" },
  { value: "blog-or-article", label: "Blog or article" },
  { value: "other", label: "Other" },
];

export default function RegisterPage() {
  const router = useRouter();
  const [isLoading, setIsLoading] = useState(false);
  const [formData, setFormData] = useState({
    email: "",
    firstName: "",
    lastName: "",
    password: "",
    confirmPassword: "",
    role: "",
    primaryUseCase: "",
    referralSource: "",
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);

  const validateForm = () => {
    const newErrors: Record<string, string> = {};

    if (!formData.email) {
      newErrors.email = "Email is required";
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(formData.email)) {
      newErrors.email = "Invalid email address";
    }

    if (!formData.firstName) newErrors.firstName = "First name is required";
    if (!formData.lastName) newErrors.lastName = "Last name is required";

    if (!formData.password) {
      newErrors.password = "Password is required";
    } else if (formData.password.length < 8) {
      newErrors.password = "Password must be at least 8 characters";
    } else if (
      !/(?=.*[a-z])(?=.*[A-Z])(?=.*\d)(?=.*[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?])/.test(
        formData.password
      )
    ) {
      newErrors.password =
        "Password must contain uppercase, lowercase, number, and special character";
    }

    if (formData.password !== formData.confirmPassword) {
      newErrors.confirmPassword = "Passwords do not match";
    }

    if (!formData.role) newErrors.role = "Please select an option";
    if (!formData.primaryUseCase)
      newErrors.primaryUseCase = "Please select an option";
    if (!formData.referralSource)
      newErrors.referralSource = "Please select an option";

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validateForm()) return;

    setIsLoading(true);

    try {
      const response = await api.register({
        email: formData.email,
        firstName: formData.firstName,
        lastName: formData.lastName,
        password: formData.password,
        provider: "local",
        signupProfile: {
          role: formData.role,
          primaryUseCase: formData.primaryUseCase,
          referralSource: formData.referralSource,
        },
      });

      if (response.success) {
        if (response.registrationRequest?.status === "approved") {
          // An allowlisted address (AIM_PLATFORM_ADMINS) is approved on the spot.
          toast.success("Account approved. Sign in to continue.");
          router.push("/auth/login");
          return;
        }
        toast.success("Registration successful. Awaiting admin approval.");
        // Redirect to pending page with request ID
        router.push(
          `/auth/registration-pending?request_id=${response.requestId}`
        );
      }
    } catch (error: any) {
      console.log("error while signup", error);
      const errorMessage =
        error.message || "Registration failed. Please try again.";
      toast.error(errorMessage);

      if (error.code === "noAdministrators") {
        // An operator instruction: keep it on the page, not only in a transient toast.
        setErrors({ form: errorMessage });
      } else if (errorMessage.includes("email already exists")) {
        setErrors({ email: "An account with this email already exists" });
      } else if (errorMessage.includes("password")) {
        setErrors({ password: errorMessage });
      }
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="glass-page relative flex min-h-screen items-center justify-center overflow-hidden p-4">
      <div className="w-full max-w-md">
        {/* Logo and Title */}
        <div className="text-center mb-8">
          <div className="flex justify-center mb-4">
            <div className="w-16 h-16 bg-logo rounded-card flex items-center justify-center shadow-glow">
              <Shield className="w-10 h-10 text-ink-inverse" />
            </div>
          </div>
          <h1 className="text-3xl font-bold tracking-[-0.03em] text-ink mb-2">
            Welcome to AIM
          </h1>
          <p className="text-ink-secondary">
            Sign up to manage AI agents and MCP servers
          </p>
        </div>

        {/* Registration Card */}
        <div className="glass-chrome p-8">
          <h2 className="text-xl font-semibold text-ink mb-6 text-center">
            Create your account
          </h2>

          {/* Email/Password Form */}
          <form onSubmit={handleSubmit} className="space-y-4 mb-6">
            <div>
              <label
                htmlFor="email"
                className="block text-sm font-medium text-ink-body mb-1"
              >
                Email address
              </label>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 transform -translate-y-1/2 h-5 w-5 text-ink-tertiary" />
                <input
                  id="email"
                  type="email"
                  value={formData.email}
                  onChange={(e) =>
                    setFormData({ ...formData, email: e.target.value })
                  }
                  className={`w-full pl-10 pr-4 py-2 rounded-inset border bg-glass-inset text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-ring ${
                    errors.email ? "border-danger" : "border-stroke"
                  }`}
                  placeholder="you@example.com"
                />
              </div>
              {errors.email && (
                <p className="mt-1 text-sm text-danger-text flex items-center gap-1">
                  <AlertCircle className="h-4 w-4" />
                  {errors.email}
                </p>
              )}
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label
                  htmlFor="firstName"
                  className="block text-sm font-medium text-ink-body mb-1"
                >
                  First name
                </label>
                <div className="relative">
                  <User className="absolute left-3 top-1/2 transform -translate-y-1/2 h-5 w-5 text-ink-tertiary" />
                  <input
                    id="firstName"
                    type="text"
                    value={formData.firstName}
                    onChange={(e) =>
                      setFormData({ ...formData, firstName: e.target.value })
                    }
                    className={`w-full pl-10 pr-4 py-2 rounded-inset border bg-glass-inset text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-ring ${
                      errors.firstName ? "border-danger" : "border-stroke"
                    }`}
                    placeholder="John"
                  />
                </div>
                {errors.firstName && (
                  <p className="mt-1 text-sm text-danger-text">
                    {errors.firstName}
                  </p>
                )}
              </div>

              <div>
                <label
                  htmlFor="lastName"
                  className="block text-sm font-medium text-ink-body mb-1"
                >
                  Last name
                </label>
                <input
                  id="lastName"
                  type="text"
                  value={formData.lastName}
                  onChange={(e) =>
                    setFormData({ ...formData, lastName: e.target.value })
                  }
                  className={`w-full px-4 py-2 rounded-inset border bg-glass-inset text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-ring ${
                    errors.lastName ? "border-danger" : "border-stroke"
                  }`}
                  placeholder="Doe"
                />
                {errors.lastName && (
                  <p className="mt-1 text-sm text-danger-text">{errors.lastName}</p>
                )}
              </div>
            </div>

            <div>
              <label
                htmlFor="password"
                className="block text-sm font-medium text-ink-body mb-1"
              >
                Password
              </label>
              <div className="relative">
                <Lock className="absolute left-3 top-1/2 transform -translate-y-1/2 h-5 w-5 text-ink-tertiary" />
                <input
                  id="password"
                  type={showPassword ? "text" : "password"}
                  value={formData.password}
                  onChange={(e) =>
                    setFormData({ ...formData, password: e.target.value })
                  }
                  className={`w-full pl-10 pr-4 py-2 rounded-inset border bg-glass-inset text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-ring ${
                    errors.password ? "border-danger" : "border-stroke"
                  }`}
                  placeholder="Min. 8 characters, uppercase, lowercase, number, special char"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword((s) => !s)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-ink-tertiary hover:text-ink"
                  aria-label={showPassword ? "Hide password" : "Show password"}
                >
                  {showPassword ? (
                    <EyeOff className="h-5 w-5" />
                  ) : (
                    <Eye className="h-5 w-5" />
                  )}
                </button>
              </div>
              {errors.password && (
                <p className="mt-1 text-sm text-danger-text flex items-center gap-1">
                  <AlertCircle className="h-4 w-4" />
                  {errors.password}
                </p>
              )}
              {!errors.password && (
                <p className="mt-1 text-xs text-ink-tertiary">
                  Must be 8+ characters with uppercase, lowercase, number &
                  special character
                </p>
              )}
            </div>

            <div>
              <label
                htmlFor="confirmPassword"
                className="block text-sm font-medium text-ink-body mb-1"
              >
                Confirm password
              </label>
              <div className="relative">
                <Lock className="absolute left-3 top-1/2 transform -translate-y-1/2 h-5 w-5 text-ink-tertiary" />
                <input
                  id="confirmPassword"
                  type={showConfirm ? "text" : "password"}
                  value={formData.confirmPassword}
                  onChange={(e) =>
                    setFormData({
                      ...formData,
                      confirmPassword: e.target.value,
                    })
                  }
                  className={`w-full pl-10 pr-4 py-2 rounded-inset border bg-glass-inset text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-ring ${
                    errors.confirmPassword
                      ? "border-danger"
                      : "border-stroke"
                  }`}
                  placeholder="Re-enter password"
                />
                <button
                  type="button"
                  onClick={() => setShowConfirm((s) => !s)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-ink-tertiary hover:text-ink"
                  aria-label={showConfirm ? "Hide password" : "Show password"}
                >
                  {showConfirm ? (
                    <EyeOff className="h-5 w-5" />
                  ) : (
                    <Eye className="h-5 w-5" />
                  )}
                </button>
              </div>
              {errors.confirmPassword && (
                <p className="mt-1 text-sm text-danger-text flex items-center gap-1">
                  <AlertCircle className="h-4 w-4" />
                  {errors.confirmPassword}
                </p>
              )}
            </div>

            <div className="pt-2 border-t border-divider space-y-4">
              <p className="text-sm text-ink-secondary">
                A few quick questions so we can improve AIM:
              </p>

              <div>
                <label
                  htmlFor="role"
                  className="block text-sm font-medium text-ink-body mb-1"
                >
                  What best describes you?
                </label>
                <select
                  id="role"
                  value={formData.role}
                  onChange={(e) =>
                    setFormData({ ...formData, role: e.target.value })
                  }
                  className={`w-full px-4 py-2 rounded-inset border bg-glass-inset focus:outline-none focus:ring-2 focus:ring-ring ${
                    errors.role ? "border-danger" : "border-stroke"
                  } ${formData.role ? "text-ink" : "text-ink-tertiary"}`}
                >
                  <option value="" disabled>
                    Select an option
                  </option>
                  {ROLE_OPTIONS.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
                {errors.role && (
                  <p className="mt-1 text-sm text-danger-text flex items-center gap-1">
                    <AlertCircle className="h-4 w-4" />
                    {errors.role}
                  </p>
                )}
              </div>

              <div>
                <label
                  htmlFor="primaryUseCase"
                  className="block text-sm font-medium text-ink-body mb-1"
                >
                  What will you use AIM for?
                </label>
                <select
                  id="primaryUseCase"
                  value={formData.primaryUseCase}
                  onChange={(e) =>
                    setFormData({ ...formData, primaryUseCase: e.target.value })
                  }
                  className={`w-full px-4 py-2 rounded-inset border bg-glass-inset focus:outline-none focus:ring-2 focus:ring-ring ${
                    errors.primaryUseCase ? "border-danger" : "border-stroke"
                  } ${formData.primaryUseCase ? "text-ink" : "text-ink-tertiary"}`}
                >
                  <option value="" disabled>
                    Select an option
                  </option>
                  {USE_CASE_OPTIONS.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
                {errors.primaryUseCase && (
                  <p className="mt-1 text-sm text-danger-text flex items-center gap-1">
                    <AlertCircle className="h-4 w-4" />
                    {errors.primaryUseCase}
                  </p>
                )}
              </div>

              <div>
                <label
                  htmlFor="referralSource"
                  className="block text-sm font-medium text-ink-body mb-1"
                >
                  How did you hear about AIM?
                </label>
                <select
                  id="referralSource"
                  value={formData.referralSource}
                  onChange={(e) =>
                    setFormData({ ...formData, referralSource: e.target.value })
                  }
                  className={`w-full px-4 py-2 rounded-inset border bg-glass-inset focus:outline-none focus:ring-2 focus:ring-ring ${
                    errors.referralSource ? "border-danger" : "border-stroke"
                  } ${formData.referralSource ? "text-ink" : "text-ink-tertiary"}`}
                >
                  <option value="" disabled>
                    Select an option
                  </option>
                  {REFERRAL_OPTIONS.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
                {errors.referralSource && (
                  <p className="mt-1 text-sm text-danger-text flex items-center gap-1">
                    <AlertCircle className="h-4 w-4" />
                    {errors.referralSource}
                  </p>
                )}
              </div>
            </div>

            {errors.form && (
              <div
                role="alert"
                className="rounded-lg border border-danger/40 bg-danger/10 px-4 py-3 text-sm text-danger-text flex items-start gap-2"
              >
                <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" />
                <span>{errors.form}</span>
              </div>
            )}

            <button
              type="submit"
              disabled={isLoading}
              className="w-full py-3 px-4 rounded-pill bg-brand text-ink-inverse font-medium shadow-glow hover:bg-brand-hover transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isLoading ? "Creating account..." : "Create account"}
            </button>
          </form>

          {/* Info Box */}
          <div className="rounded-inset bg-brand-soft p-4">
            <p className="text-sm text-ink-body">
              <strong>Note:</strong> After you sign up, an administrator reviews
              your account before you can sign in. Until then the sign-in page
              will tell you the request is still pending.
            </p>
          </div>

          {/* Footer */}
          <div className="mt-6 text-center text-sm text-ink-secondary">
            Already have an account?{" "}
            <Link
              href="/auth/login"
              className="font-semibold text-brand-text hover:underline"
            >
              Sign in
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
