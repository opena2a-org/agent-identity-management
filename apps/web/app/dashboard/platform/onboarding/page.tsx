"use client";

import { OnboardingBaselinePanel } from "@/components/platform/onboarding-baseline-panel";

/**
 * Platform view of onboarding: how long new organizations take to register a first agent.
 * The panel is a shared component so the hosted product's /dashboard/platform page can
 * mount the same panel.
 */
export default function PlatformOnboardingPage() {
  return (
    <div className="flex flex-col gap-4">
      <OnboardingBaselinePanel />
    </div>
  );
}
