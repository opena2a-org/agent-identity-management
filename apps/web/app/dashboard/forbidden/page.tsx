"use client";

import { ShieldAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useRouter } from "next/navigation";

export default function ForbiddenPage() {
  const router = useRouter();

  return (
    <div className="flex flex-col items-center justify-center min-h-screen bg-gray-50 dark:bg-gray-900">
      <div className="text-center space-y-6 max-w-md px-4">
        <div className="flex justify-center">
          <div className="relative">
            <div className="absolute inset-0 bg-red-500 opacity-20 blur-3xl rounded-full"></div>
            <ShieldAlert className="relative h-24 w-24 text-red-500 mx-auto" />
          </div>
        </div>

        <div className="space-y-2">
          <h1 className="text-4xl font-bold text-gray-900 dark:text-white">
            403 Forbidden
          </h1>
          <p className="text-lg text-gray-600 dark:text-gray-400">
            You don't have permission to access this resource.
          </p>
        </div>

        <div className="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4">
          <p className="text-sm text-yellow-800 dark:text-yellow-200">
            <strong>Need access?</strong> Contact your organization administrator
            to request the appropriate permissions.
          </p>
        </div>

        <div className="flex flex-col sm:flex-row gap-3 justify-center pt-4">
          <Button onClick={() => router.push("/dashboard")} size="lg">
            Return to Dashboard
          </Button>
          <Button
            onClick={() => router.back()}
            variant="outline"
            size="lg"
          >
            Go Back
          </Button>
        </div>

        <div className="text-xs text-gray-500 dark:text-gray-400 pt-4">
          If you believe this is an error, please contact support.
        </div>
      </div>
    </div>
  );
}

