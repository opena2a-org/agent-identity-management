"use client";

import { Key, Monitor } from "lucide-react";
import { AuthGuard } from "@/components/auth-guard";
import { APIKeysSection } from "@/components/credentials/api-keys-section";
import { SDKTokensSection } from "@/components/credentials/sdk-tokens-section";

/**
 * Developers → Credentials: the two kinds of credential that call AIM, explained side by
 * side and then listed one section each. The two pages this replaces redirect here
 * (lib/redirects.ts); each section keeps the data calls and role checks its page had.
 */
const KINDS = [
  {
    id: "api-keys",
    name: "API keys",
    icon: Key,
    purpose:
      "An API key belongs to one agent, which sends it to prove its identity when it calls AIM.",
  },
  {
    id: "sdk-tokens",
    name: "SDK tokens",
    icon: Monitor,
    purpose:
      "An SDK token is issued with each SDK download and lets the SDK on that device register agents and MCP servers on your behalf.",
  },
] as const;

export default function CredentialsPage() {
  return (
    <AuthGuard>
      <div className="space-y-10">
        <div className="space-y-6">
          <div>
            <h1 className="text-headline">Credentials</h1>
            <p className="mt-1 text-sm text-ink-secondary">
              The two kinds of credential your agents and SDK installs use to call AIM.
            </p>
          </div>

          <dl className="grid grid-cols-1 gap-4 md:grid-cols-2">
            {KINDS.map((kind) => (
              <div key={kind.id} className="glass p-5">
                <dt className="flex items-center gap-2">
                  <kind.icon className="h-5 w-5 text-ink-tertiary" aria-hidden="true" />
                  <a
                    href={`#${kind.id}`}
                    className="text-sm font-semibold text-ink hover:text-brand-text transition-colors"
                  >
                    {kind.name}
                  </a>
                </dt>
                <dd className="mt-2 text-sm text-ink-secondary">{kind.purpose}</dd>
              </div>
            ))}
          </dl>
        </div>

        <section id="api-keys" aria-labelledby="api-keys-heading" className="scroll-mt-4">
          <APIKeysSection headingId="api-keys-heading" />
        </section>

        <section id="sdk-tokens" aria-labelledby="sdk-tokens-heading" className="scroll-mt-4">
          <SDKTokensSection headingId="sdk-tokens-heading" />
        </section>
      </div>
    </AuthGuard>
  );
}
