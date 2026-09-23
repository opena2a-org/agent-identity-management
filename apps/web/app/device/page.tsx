"use client";

import { useCallback, useEffect, useState, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { AlertTriangle, CheckCircle2, Loader2, Terminal, XCircle } from "lucide-react";
import { AimLogo } from "@/components/sidebar";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { toast } from "sonner";

/**
 * Consent page for the CLI device login (OAuth 2.0 device grant, RFC 8628).
 *
 * `aim-sdk login` prints a user code and opens `/device?user_code=XXXX-XXXX`.
 * The code on the URL is a hint, not an authorization: nothing is approved on
 * load, the requesting client is named only from an allow-list (the client id
 * arrives from an unauthenticated request and is never rendered raw), and the
 * approval happens on one explicit click by a signed-in user, through the api
 * client so the dashboard's own session model applies.
 */

type DeviceState = "enter_code" | "confirm" | "approving" | "approved" | "denied" | "expired" | "error";

// Clients this dashboard knows how to name. Anything else is "an unrecognised client".
const KNOWN_CLIENTS: Record<string, string> = {
  "aim-sdk": "aim-sdk (the AIM Python SDK command line)",
  "opena2a-cli": "opena2a (the OpenA2A command line)",
};

function formatUserCode(value: string): string {
  const clean = value.replace(/[^A-Z0-9]/gi, "").toUpperCase().slice(0, 8);
  return clean.length > 4 ? `${clean.slice(0, 4)}-${clean.slice(4)}` : clean;
}

function DevicePageContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const userCodeParam = formatUserCode(searchParams.get("user_code") ?? "");

  const [userCode, setUserCode] = useState(userCodeParam);
  const [deviceState, setDeviceState] = useState<DeviceState>(userCodeParam.length === 9 ? "confirm" : "enter_code");
  const [clientLabel, setClientLabel] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState("");
  const [signedIn, setSignedIn] = useState(false);

  useEffect(() => {
    setSignedIn(Boolean(api.getToken()));
  }, []);

  // Read the pending request so the page can name the client. This is a read;
  // it approves nothing.
  const describe = useCallback(async (code: string) => {
    try {
      const pending = await api.getDeviceVerification(code);
      setClientLabel(KNOWN_CLIENTS[pending.clientId] ?? "an unrecognised client");
      if (pending.status && pending.status !== "pending") {
        setDeviceState(pending.status === "expired" ? "expired" : "error");
        if (pending.status !== "expired") {
          setErrorMessage("This code is no longer waiting for approval. Run aim-sdk login again to get a new one.");
        }
      }
    } catch (err: any) {
      const msg: string = err?.message ?? "";
      if (/expired/i.test(msg)) {
        setDeviceState("expired");
      } else {
        setClientLabel("an unrecognised client");
        setErrorMessage("This code was not found. Check the code shown in your terminal and try again.");
        setDeviceState("enter_code");
      }
    }
  }, []);

  useEffect(() => {
    if (deviceState === "confirm" && userCode.length === 9) {
      void describe(userCode);
    }
  }, [deviceState, userCode, describe]);

  const handleCodeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setUserCode(formatUserCode(e.target.value));
    setErrorMessage("");
  };

  const handleCodeSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (userCode.replace(/-/g, "").length !== 8) {
      setErrorMessage("The code has 8 characters, shown as XXXX-XXXX in your terminal.");
      return;
    }
    setClientLabel(null);
    setDeviceState("confirm");
  };

  const goToLogin = () => {
    const returnUrl = `/device?user_code=${encodeURIComponent(userCode)}`;
    router.push(`/auth/login?returnUrl=${encodeURIComponent(returnUrl)}`);
  };

  const handleApprove = async () => {
    if (!api.getToken()) {
      goToLogin();
      return;
    }
    setDeviceState("approving");
    setErrorMessage("");
    try {
      await api.approveDevice(userCode);
      setDeviceState("approved");
      toast.success("Command line signed in");
    } catch (err: any) {
      const msg: string = err?.message ?? "";
      if (/expired/i.test(msg)) {
        setDeviceState("expired");
      } else if (/not found|no longer pending|invalid/i.test(msg)) {
        setErrorMessage("This code was not found or is no longer waiting. Check your terminal and try again.");
        setDeviceState("enter_code");
      } else if (/unauthorized|401/i.test(msg)) {
        goToLogin();
      } else {
        setErrorMessage(msg || "The approval did not go through. Try again.");
        setDeviceState("error");
      }
    }
  };

  const shell = (children: React.ReactNode) => (
    <main className="glass-page relative flex min-h-screen items-center justify-center overflow-hidden p-4">
      <div className="w-full max-w-md">{children}</div>
    </main>
  );

  if (deviceState === "approved") {
    return shell(
      <div className="glass-chrome p-8 text-center">
        <span className="inline-flex h-14 w-14 items-center justify-center rounded-full bg-success-fill text-success-text">
          <CheckCircle2 className="h-7 w-7" aria-hidden="true" />
        </span>
        <h1 className="mt-4 text-[22px] font-bold tracking-[-0.03em] text-ink">Command line signed in</h1>
        <p className="mt-1 text-sm text-ink-secondary">
          The terminal that showed code {userCode} now holds your credentials. You can close this tab.
        </p>
        <div className="mt-5 rounded-inset bg-glass-inset-gray p-3.5 text-xs text-ink-secondary">
          Run <code className="rounded bg-glass-inset px-1.5 py-0.5 font-mono text-ink">aim-sdk status</code> to confirm.
        </div>
      </div>
    );
  }

  if (deviceState === "expired") {
    return shell(
      <div className="glass-chrome p-8 text-center">
        <span className="inline-flex h-14 w-14 items-center justify-center rounded-full bg-warning-fill text-warning-text">
          <XCircle className="h-7 w-7" aria-hidden="true" />
        </span>
        <h1 className="mt-4 text-[22px] font-bold tracking-[-0.03em] text-ink">Code expired</h1>
        <p className="mt-1 text-sm text-ink-secondary">
          This code is no longer valid. Run{" "}
          <code className="rounded bg-glass-inset px-1.5 py-0.5 font-mono text-xs text-ink">aim-sdk login</code> again
          to get a new one.
        </p>
      </div>
    );
  }

  return shell(
    <>
      <div className="mb-6 flex flex-col items-center text-center">
        <AimLogo size={48} className="shadow-[0_10px_26px_rgba(56,189,248,0.35)]" />
        <h1 className="mt-4 text-[26px] font-bold tracking-[-0.03em] text-ink">Sign in a command line</h1>
        <p className="mt-1 inline-flex items-center gap-2 text-sm text-ink-secondary">
          <Terminal className="h-4 w-4" aria-hidden="true" />
          A command line is asking to act as your account.
        </p>
      </div>

      <div className="glass-chrome p-6 sm:p-8">
        {deviceState === "enter_code" ? (
          <>
            <h2 className="text-[15px] font-bold tracking-[-0.02em] text-ink">Enter the code from your terminal</h2>
            <p className="mt-1 text-xs text-ink-secondary">
              <code className="font-mono">aim-sdk login</code> prints it as XXXX-XXXX.
            </p>
            <form onSubmit={handleCodeSubmit} className="mt-5" noValidate>
              <label htmlFor="user-code" className="sr-only">
                Verification code
              </label>
              <input
                id="user-code"
                type="text"
                value={userCode}
                onChange={handleCodeChange}
                placeholder="XXXX-XXXX"
                className="w-full rounded-inset border border-stroke bg-glass-inset px-4 py-4 text-center font-mono text-3xl tracking-[0.3em] text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-ring"
                maxLength={9}
                autoFocus
                autoComplete="off"
                spellCheck={false}
                aria-invalid={!!errorMessage}
              />
              {errorMessage && <p className="mt-2 text-xs font-semibold text-danger-text">{errorMessage}</p>}
              <Button type="submit" className="mt-4 w-full" size="lg">
                Continue
              </Button>
            </form>
          </>
        ) : (
          <>
            <h2 className="text-[15px] font-bold tracking-[-0.02em] text-ink">Confirm the code</h2>
            <p className="mt-1 text-xs text-ink-secondary">
              {clientLabel ? <>Requested by {clientLabel}.</> : <>Looking up the request…</>}
            </p>
            <div className="mt-4 rounded-inset bg-glass-inset-gray p-4 text-center">
              <span className="font-mono text-2xl font-bold tracking-[0.2em] text-ink">{userCode}</span>
            </div>

            <div className="mt-4 flex items-start gap-2 rounded-inset border border-warning-border bg-warning-fill p-3">
              <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-warning-text" aria-hidden="true" />
              <p className="text-xs text-ink-body">
                Approve only if you started this login yourself, just now, and this code matches the one in your
                terminal. Approving signs that command line in as you.
              </p>
            </div>

            {errorMessage && deviceState === "error" && (
              <div className="mt-4 rounded-inset border border-danger-border bg-danger-fill p-3">
                <p className="text-xs font-semibold text-danger-text">{errorMessage}</p>
              </div>
            )}

            <div className="mt-5 space-y-3">
              {!signedIn ? (
                <>
                  <p className="text-center text-xs text-ink-secondary">Sign in to continue.</p>
                  <Button type="button" className="w-full" size="lg" onClick={goToLogin}>
                    Sign in to authorize
                  </Button>
                </>
              ) : (
                <Button
                  type="button"
                  onClick={handleApprove}
                  disabled={deviceState === "approving"}
                  className="w-full"
                  size="lg"
                >
                  {deviceState === "approving" ? (
                    <>
                      <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
                      Authorizing…
                    </>
                  ) : (
                    <>Authorize {clientLabel ? clientLabel.split(" ")[0] : "this command line"}</>
                  )}
                </Button>
              )}
              <button
                type="button"
                onClick={() => {
                  setDeviceState("enter_code");
                  setUserCode("");
                  setClientLabel(null);
                  setErrorMessage("");
                }}
                className="w-full py-2 text-xs font-semibold text-ink-secondary hover:text-ink"
              >
                Enter a different code
              </button>
            </div>
          </>
        )}
      </div>

      <p className="mt-5 text-center text-xs text-ink-tertiary">
        Approving signs that command line in as you. Only approve a code you asked for.
      </p>
    </>
  );
}

export default function DevicePage() {
  return (
    <Suspense
      fallback={
        <main className="glass-page flex min-h-screen items-center justify-center">
          <Loader2 className="h-8 w-8 animate-spin text-ink-tertiary" aria-hidden="true" />
        </main>
      }
    >
      <DevicePageContent />
    </Suspense>
  );
}
