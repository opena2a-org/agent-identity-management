"use client";

import { useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { ArrowRight, Book, Check, CheckCircle, Copy, ExternalLink } from "lucide-react";
import { Button, buttonVariants } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { Agent } from "@/lib/api";
import { SDK_TABS } from "@/lib/sdk-tabs";
import { cn } from "@/lib/utils";

/**
 * An agent is in its first run from registration until its first authenticated call: the
 * server stamps lastActive on that call and leaves it empty before it. A suspended or
 * revoked agent cannot connect, so it is never shown the steps.
 */
export function isFirstRun(agent: Pick<Agent, "status" | "lastActive">): boolean {
  return !agent.lastActive && agent.status !== "suspended" && agent.status !== "revoked";
}

type Language = "python" | "java";

// The Java install line and example are the shared quickstart's, so they change in one place.
const JAVA = SDK_TABS.find((tab) => tab.key === "java")!;

const CODE = "mt-2 overflow-x-auto rounded-inset-sm bg-glass-inset-gray p-3 font-mono text-xs text-ink-body";
const EXAMPLE = "glass-contrast overflow-x-auto rounded-inset p-4 font-mono text-xs text-ink-code";

function Step({ n, title, children }: { n: number; title: string; children: ReactNode }) {
  return (
    <li className="flex gap-4">
      <span className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-pill bg-brand-soft font-bold text-brand-text">
        {n}
      </span>
      <div className="min-w-0 flex-1">
        <h4 className="mb-1 font-semibold">{title}</h4>
        {children}
      </div>
    </li>
  );
}

/**
 * What a person who has just registered an agent needs next: its identifier and public
 * key, the commands that connect it from code, where its private key ends up, and where
 * to go from here. The Python steps name this agent by its identifier and the Java steps
 * by its name; both SDKs then connect to the agent that exists instead of registering one.
 * The agent detail page renders it; it shows nothing once the agent has connected.
 */
export function FirstRunPanel({ agent }: { agent: Agent }) {
  const [language, setLanguage] = useState<Language>("python");
  const [copied, setCopied] = useState<string | null>(null);
  // `aim-sdk login` defaults to the hosted service; a self-hosted dashboard passes its own origin.
  const [origin, setOrigin] = useState("");
  useEffect(() => {
    setOrigin(window.location.origin);
  }, []);

  if (!isFirstRun(agent)) return null;

  const copy = async (text: string, field: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(field);
      setTimeout(() => setCopied(null), 2000);
    } catch {
      // The clipboard is unavailable outside a secure context; the value stays selectable.
    }
  };

  const identity = [
    { field: "agent-id", label: "Agent ID", value: agent.id },
    ...(agent.publicKey ? [{ field: "public-key", label: "Public key (Ed25519)", value: agent.publicKey }] : []),
  ];

  return (
    <Card role="region" aria-labelledby="first-run-title">
      <CardHeader>
        <CardTitle id="first-run-title" className="flex items-center gap-2 leading-snug">
          <CheckCircle className="h-5 w-5 flex-shrink-0 text-success-text" />
          Agent registered. Next, connect it from your code
        </CardTitle>
        <p className="text-sm text-ink-secondary">
          <span className="font-semibold">{agent.displayName || agent.name}</span> is registered with AIM and has not
          connected yet. Follow the steps below; actions it performs through the SDK are then verified against its
          identity. This panel goes away once the agent makes its first call.
        </p>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="grid gap-3 md:grid-cols-2">
          {identity.map(({ field, label, value }) => (
            <div key={field} className="flex items-center justify-between gap-2 rounded-inset bg-glass-inset-gray p-3">
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium text-ink-body">{label}</p>
                <p className="truncate font-mono text-sm text-ink-secondary" title={value}>
                  {value}
                </p>
              </div>
              <Button variant="ghost" size="sm" aria-label={`Copy ${label}`} onClick={() => copy(value, field)}>
                {copied === field ? <Check className="h-4 w-4 text-success-text" /> : <Copy className="h-4 w-4" />}
              </Button>
            </div>
          ))}
        </div>

        <div>
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <h3 className="flex items-center gap-2 text-[15px] font-bold text-ink">
              <Book className="h-5 w-5" />
              Quick start
            </h3>
            <div className="flex gap-2" role="group" aria-label="SDK language">
              {(["python", "java"] as const).map((lang) => (
                <Button
                  key={lang}
                  size="sm"
                  variant={language === lang ? "default" : "outline"}
                  aria-pressed={language === lang}
                  onClick={() => setLanguage(lang)}
                >
                  {lang === "python" ? "Python" : "Java"}
                </Button>
              ))}
            </div>
          </div>

          {language === "python" ? (
            <>
              <ol className="space-y-4">
                <Step n={1} title="Install the SDK">
                  <p className="text-sm text-ink-secondary">
                    From PyPI. For machines without registry access, use the offline install on the SDK page.
                  </p>
                  <pre className={CODE}>
                    <code>pip install aim-sdk</code>
                  </pre>
                </Step>
                <Step n={2} title="Sign in once">
                  <p className="text-sm text-ink-secondary">Links your machine to this account; no API key needed.</p>
                  <pre className={CODE}>
                    <code>{origin ? `aim-sdk login --url ${origin}` : "aim-sdk login"}</code>
                  </pre>
                </Step>
                <Step n={3} title="Connect this agent">
                  <p className="text-sm text-ink-secondary">
                    Use this agent&apos;s identifier. The SDK creates a new key pair on this machine, stores it under
                    ~/.aim/ and replaces the key created at registration, so no other copy of a private key has to
                    exist.
                  </p>
                  <pre className={CODE}>
                    <code>{`from aim_sdk import secure

agent = secure("${agent.id}")`}</code>
                  </pre>
                </Step>
              </ol>
              <h4 className="mb-2 mt-6 font-semibold">Example usage</h4>
              <pre className={EXAMPLE}>
                <code>{`from aim_sdk import secure

agent = secure("${agent.id}")

# Each call is verified against the agent's capabilities before it runs
@agent.perform_action("read_database", resource="users_table")
def get_users():
    # Your agent code here
    return database.query("SELECT * FROM users")

users = get_users()`}</code>
              </pre>
            </>
          ) : (
            <>
              <ol className="space-y-4">
                <Step n={1} title="Install the SDK">
                  <p className="text-sm text-ink-secondary">From source, with Maven.</p>
                  <pre className={CODE}>
                    <code>{JAVA.install(origin)}</code>
                  </pre>
                </Step>
                <Step n={2} title="Connect this agent">
                  <p className="text-sm text-ink-secondary">
                    Use this agent&apos;s name. The SDK finds the agent registered here and reconnects to it instead of
                    registering a new one.
                  </p>
                  <pre className={CODE}>
                    <code>{`AIMClient agent = AIMClient.secure(${JSON.stringify(agent.name)});`}</code>
                  </pre>
                </Step>
              </ol>
              <h4 className="mb-2 mt-6 font-semibold">Example usage</h4>
              <pre className={EXAMPLE}>
                <code>{JAVA.code(origin)}</code>
              </pre>
              <a
                href={JAVA.docsHref}
                target="_blank"
                rel="noopener noreferrer"
                className="mt-3 inline-flex items-center gap-1 text-sm font-semibold text-brand-text hover:underline"
              >
                {JAVA.docsLabel}
                <ExternalLink className="h-3.5 w-3.5" />
              </a>
            </>
          )}
        </div>

        <div className="flex flex-wrap gap-3">
          <a
            href="https://opena2a.org/docs"
            target="_blank"
            rel="noopener noreferrer"
            className={cn(buttonVariants({ variant: "outline", size: "sm" }))}
          >
            <Book className="h-4 w-4" />
            View documentation
          </a>
          <Link href="/dashboard/agents" className={cn(buttonVariants({ variant: "outline", size: "sm" }))}>
            View all agents
          </Link>
          <Link href="/dashboard" className={cn(buttonVariants({ variant: "outline", size: "sm" }))}>
            Go to dashboard
            <ArrowRight className="h-4 w-4" />
          </Link>
        </div>
      </CardContent>
    </Card>
  );
}
