"use client";

import { useState } from "react";
import Link from "next/link";
import { Shield, Mail, AlertCircle, CheckCircle2, ArrowLeft } from "lucide-react";
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

  if (isSubmitted) {
    return (
      <div className="min-h-screen bg-gradient-to-br from-gray-50 to-gray-100 flex items-center justify-center p-4">
        <div className="w-full max-w-md">
          {/* Logo and Branding */}
          <div className="text-center mb-8">
            <div className="inline-flex items-center justify-center w-16 h-16 bg-gradient-to-br from-green-600 to-emerald-600 rounded-2xl mb-4">
              <CheckCircle2 className="w-8 h-8 text-white" />
            </div>
            <h1 className="text-3xl font-bold text-gray-900 mb-2">
              Check Your Email
            </h1>
            <p className="text-gray-600">
              We've sent you password reset instructions
            </p>
          </div>

          {/* Success Card */}
          <div className="bg-white rounded-2xl shadow-xl border border-gray-200 p-8">
            <div className="text-center space-y-4">
              <div className="bg-green-50 border border-green-100 rounded-lg p-4">
                <p className="text-sm text-green-900">
                  If an account exists for <strong>{email}</strong>, you will
                  receive an email with instructions to reset your password.
                </p>
              </div>

              <div className="text-left space-y-2 text-sm text-gray-600">
                <p className="font-medium text-gray-900">Next steps:</p>
                <ul className="list-disc list-inside space-y-1 ml-2">
                  <li>Check your email inbox for a message from Agent Identity Management</li>
                  <li>Click the password reset link in the email</li>
                  <li>The link will expire in 24 hours for security</li>
                  <li>Check your spam folder if you don't see it</li>
                </ul>
              </div>

              <div className="pt-4 space-y-3">
                <Link
                  href="/auth/login"
                  className="block w-full py-3 px-4 bg-blue-600 text-white font-medium rounded-lg hover:bg-blue-700 transition-colors text-center"
                >
                  Return to Login
                </Link>
                <button
                  onClick={() => {
                    setIsSubmitted(false);
                    setEmail("");
                  }}
                  className="block w-full py-3 px-4 border border-gray-300 text-gray-700 font-medium rounded-lg hover:bg-gray-50 transition-colors text-center"
                >
                  Send Another Email
                </button>
              </div>
            </div>
          </div>

          {/* Help Text */}
          <div className="mt-6 text-center text-sm text-gray-600">
            <p>
              Didn't receive an email?{" "}
              <button
                onClick={() => {
                  setIsSubmitted(false);
                  setEmail("");
                }}
                className="text-blue-600 hover:underline font-medium"
              >
                Try again
              </button>
            </p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray-50 to-gray-100 flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        {/* Logo and Branding */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-16 h-16 bg-gradient-to-br from-blue-600 to-purple-600 rounded-2xl mb-4">
            <Shield className="w-8 h-8 text-white" />
          </div>
          <h1 className="text-3xl font-bold text-gray-900 mb-2">
            Forgot Password?
          </h1>
          <p className="text-gray-600">
            No worries, we'll send you reset instructions
          </p>
        </div>

        {/* Main Card */}
        <div className="bg-white rounded-2xl shadow-xl border border-gray-200 p-8">
          <form onSubmit={handleSubmit} className="space-y-6">
            <div>
              <label
                htmlFor="email"
                className="block text-sm font-medium text-gray-700 mb-1"
              >
                Email Address
              </label>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 transform -translate-y-1/2 h-5 w-5 text-gray-400" />
                <input
                  id="email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  onBlur={validateEmail}
                  className={`w-full pl-10 pr-4 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 ${
                    error ? "border-red-500" : "border-gray-300"
                  }`}
                  placeholder="you@example.com"
                  autoFocus
                />
              </div>
              {error && (
                <p className="mt-1 text-sm text-red-600 flex items-center gap-1">
                  <AlertCircle className="h-4 w-4" />
                  {error}
                </p>
              )}
              <p className="mt-2 text-xs text-gray-500">
                Enter the email address associated with your account
              </p>
            </div>

            <button
              type="submit"
              disabled={isLoading}
              className="w-full py-3 px-4 bg-blue-600 text-white font-medium rounded-lg hover:bg-blue-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isLoading ? "Sending..." : "Send Reset Link"}
            </button>
          </form>

          {/* Security Info */}
          <div className="bg-blue-50 border border-blue-100 rounded-lg p-4 mt-6">
            <div className="flex gap-3">
              <Shield className="w-5 h-5 text-blue-600 flex-shrink-0 mt-0.5" />
              <div className="text-sm text-blue-900">
                <p className="font-medium mb-1">Secure Reset Process</p>
                <p className="text-blue-700">
                  For security, we don't reveal whether an email exists in our
                  system. You'll only receive an email if your account is
                  registered.
                </p>
              </div>
            </div>
          </div>

          {/* Back to Login */}
          <div className="text-center pt-6 border-t border-gray-200 mt-6">
            <Link
              href="/auth/login"
              className="inline-flex items-center gap-2 text-blue-600 hover:text-blue-700 font-medium hover:underline"
            >
              <ArrowLeft className="w-4 h-4" />
              Back to Login
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
