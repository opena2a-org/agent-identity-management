"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { AlertCircle, Copy } from "lucide-react";
import { AuthGuard } from "@/components/auth-guard";
import { ActionOutcome } from "@/components/ui/action-outcome";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { api, type OidcProvisionedRole, type OidcSignInConfiguration } from "@/lib/api";
import {
  actionSuccess,
  describeActionFailure,
  type ActionFailure,
  type ActionOutcome as Outcome,
} from "@/lib/action-outcome";
import { decodeJwtPayload } from "@/lib/jwt-payload";
import {
  PROVISIONED_ROLES,
  identityProviderForm,
  identityProviderFormErrors,
  identityProviderInput,
  identityProviderSaveFailure,
  keepsStoredClientSecret,
  organizationSignInLink,
  type IdentityProviderField,
  type IdentityProviderForm,
  type IdentityProviderFormErrors,
} from "@/lib/identity-provider-settings";

// The organization's OpenID Connect provider: where an administrator stores it, sets what a
// sign-in may create and whether password sign-in is refused, finds the redirect URI to
// register and the sign-in link to share, and removes it.

const FIELD_IDS: Record<IdentityProviderField, string> = {
  issuer: "idp-issuer",
  clientId: "idp-client-id",
  clientSecret: "idp-client-secret",
  emailDomains: "idp-email-domains",
};

const FIELD_ORDER: IdentityProviderField[] = ["issuer", "clientId", "clientSecret", "emailDomains"];

const inputClass = (invalid: boolean) =>
  `min-h-11 w-full rounded-inset border bg-glass-inset px-3 py-2.5 text-sm text-ink placeholder:text-ink-tertiary focus:outline-none focus:ring-2 focus:ring-ring ${
    invalid ? "border-danger" : "border-stroke"
  }`;

function FieldError({ id, message }: { id: string; message?: string }) {
  if (!message) return null;
  return (
    <p id={id} className="flex items-start gap-1 text-sm font-semibold text-danger-text">
      <AlertCircle className="mt-0.5 h-3.5 w-3.5 flex-shrink-0" aria-hidden="true" />
      {message}
    </p>
  );
}

/** A value to copy, with its button and the outcome of the last copy. */
function CopyValue({ label, value }: { label: string; value: string }) {
  const [copied, setCopied] = useState<"copied" | "failed" | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    []
  );

  const copy = async () => {
    try {
      // Absent outside a secure context, such as a deployment served over plain http.
      if (!navigator.clipboard) throw new Error("no clipboard");
      await navigator.clipboard.writeText(value);
      setCopied("copied");
    } catch {
      setCopied("failed");
    }
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => setCopied(null), 4000);
  };

  return (
    <div className="space-y-2">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <code className="min-w-0 flex-1 break-all rounded-inset border border-stroke bg-glass-inset px-3 py-2.5 font-mono text-sm text-ink">
          {value}
        </code>
        <Button type="button" variant="outline" size="lg" onClick={copy} aria-label={`Copy the ${label}`} className="self-start sm:self-auto">
          <Copy className="h-4 w-4" aria-hidden="true" />
          Copy
        </Button>
      </div>
      <p role="status" className="text-sm text-ink-secondary">
        {copied === "copied" ? `The ${label} is copied.` : copied === "failed" ? "The browser did not copy it. Select the text and copy it." : ""}
      </p>
    </div>
  );
}

export default function IdentityProviderPage() {
  const [config, setConfig] = useState<OidcSignInConfiguration | null>(null);
  const [form, setForm] = useState<IdentityProviderForm>(() => identityProviderForm(null));
  const [errors, setErrors] = useState<IdentityProviderFormErrors>({});
  const [loading, setLoading] = useState(true);
  const [loadFailure, setLoadFailure] = useState<ActionFailure | null>(null);
  const [busy, setBusy] = useState<"saving" | "removing" | null>(null);
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [confirmingRemoval, setConfirmingRemoval] = useState(false);
  const [origin, setOrigin] = useState("");
  const [organizationId, setOrganizationId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadFailure(null);
    try {
      const loaded = await api.getOidcSignInConfiguration();
      setConfig(loaded);
      setForm(identityProviderForm(loaded));
      setErrors({});
    } catch (err) {
      setConfig(null);
      setLoadFailure(describeActionFailure(err, { action: "load the identity provider settings" }));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    setOrigin(window.location.origin);
    // For the sign-in link only; the server reads the organization from the session.
    setOrganizationId(decodeJwtPayload(api.getToken())?.organization_id ?? null);
    load();
  }, [load]);

  const update = <K extends keyof IdentityProviderForm>(key: K, value: IdentityProviderForm[K]) => {
    setForm((current) => ({ ...current, [key]: value }));
    setOutcome(null);
  };

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    const found = identityProviderFormErrors(config, form);
    setErrors(found);
    setOutcome(null);
    const firstInvalid = FIELD_ORDER.find((field) => found[field]);
    if (firstInvalid) {
      document.getElementById(FIELD_IDS[firstInvalid])?.focus();
      return;
    }
    setBusy("saving");
    try {
      const saved = await api.saveOidcSignInConfiguration(identityProviderInput(form));
      setConfig(saved);
      setForm(identityProviderForm(saved));
      setOutcome(actionSuccess("The identity provider is saved."));
    } catch (err) {
      setOutcome(identityProviderSaveFailure(err));
    } finally {
      setBusy(null);
    }
  };

  const remove = async () => {
    setConfirmingRemoval(false);
    setOutcome(null);
    setBusy("removing");
    try {
      await api.removeOidcSignInConfiguration();
      const removed: OidcSignInConfiguration = { configured: false, redirectUrl: config?.redirectUrl ?? "" };
      setConfig(removed);
      setForm(identityProviderForm(removed));
      setErrors({});
      setOutcome(actionSuccess("The identity provider is removed."));
    } catch (err) {
      setOutcome(describeActionFailure(err, { action: "remove the identity provider" }));
    } finally {
      setBusy(null);
    }
  };

  const describedBy = (field: IdentityProviderField, hintId: string) =>
    errors[field] ? `${FIELD_IDS[field]}-error ${hintId}` : hintId;

  const secretStored = !!(config?.configured && config.clientSecretSet);
  const keepSecret = keepsStoredClientSecret(config, form);
  const secretHint = !secretStored
    ? "The client secret your provider issued. It is stored encrypted and never shown again."
    : keepSecret
      ? "A client secret is stored. Leave this empty to keep it, or enter a new one to replace it."
      : "The stored client secret is kept only with the saved issuer and client ID. Enter the client secret for the new ones.";
  const signInLink = config?.configured ? organizationSignInLink(origin, organizationId) : null;

  return (
    <AuthGuard>
      <div className="max-w-3xl space-y-6">
        <div>
          <h1 className="text-2xl font-bold text-ink">Identity provider</h1>
          <p className="mt-1 text-sm text-ink-secondary">
            Your organization&apos;s users can sign in through your OpenID Connect provider. Administrators keep
            password sign-in, so they can change or remove the provider at any time.
          </p>
        </div>

        {loading ? (
          <div className="glass space-y-3 p-6" aria-busy="true" aria-label="Loading the identity provider settings">
            {Array.from({ length: 4 }, (_, i) => (
              <div key={i} className="h-4 w-full animate-pulse rounded bg-glass-inset" />
            ))}
          </div>
        ) : loadFailure || !config ? (
          <div className="glass space-y-4 p-6">
            <ActionOutcome outcome={loadFailure} />
            <Button type="button" variant="outline" size="lg" onClick={load}>
              Try again
            </Button>
          </div>
        ) : (
          <>
            <section aria-labelledby="idp-register-heading" className="glass space-y-3 p-4 sm:p-6">
              <h2 id="idp-register-heading" className="text-base font-semibold text-ink">
                Register this deployment with your provider
              </h2>
              <p className="text-sm text-ink-body">
                At your provider, register a web application that uses the authorization code flow, with this redirect
                URI. Sign-in asks for the scopes openid, email and profile, and needs the provider to assert a verified
                email address.
              </p>
              <CopyValue label="redirect URI" value={config.redirectUrl} />
            </section>

            <form onSubmit={save} noValidate aria-labelledby="idp-provider-heading" className="glass space-y-5 p-4 sm:p-6">
              <h2 id="idp-provider-heading" className="text-base font-semibold text-ink">
                Provider
              </h2>

              <div className="space-y-1.5">
                <label htmlFor={FIELD_IDS.issuer} className="block text-sm font-semibold text-ink-body">
                  Issuer URL
                </label>
                <input
                  id={FIELD_IDS.issuer}
                  type="url"
                  inputMode="url"
                  autoComplete="off"
                  spellCheck={false}
                  value={form.issuer}
                  onChange={(e) => update("issuer", e.target.value)}
                  placeholder="https://idp.example.com"
                  className={inputClass(!!errors.issuer)}
                  aria-invalid={!!errors.issuer}
                  aria-describedby={describedBy("issuer", "idp-issuer-hint")}
                />
                <FieldError id={`${FIELD_IDS.issuer}-error`} message={errors.issuer} />
                <p id="idp-issuer-hint" className="text-sm text-ink-secondary">
                  The issuer your provider names in its ID tokens. Before it saves, the server reads the issuer&apos;s
                  configuration document.
                </p>
              </div>

              <div className="space-y-1.5">
                <label htmlFor={FIELD_IDS.clientId} className="block text-sm font-semibold text-ink-body">
                  Client ID
                </label>
                <input
                  id={FIELD_IDS.clientId}
                  type="text"
                  autoComplete="off"
                  spellCheck={false}
                  value={form.clientId}
                  onChange={(e) => update("clientId", e.target.value)}
                  className={inputClass(!!errors.clientId)}
                  aria-invalid={!!errors.clientId}
                  aria-describedby={describedBy("clientId", "idp-client-id-hint")}
                />
                <FieldError id={`${FIELD_IDS.clientId}-error`} message={errors.clientId} />
                <p id="idp-client-id-hint" className="text-sm text-ink-secondary">
                  The client ID your provider registered for this deployment.
                </p>
              </div>

              <div className="space-y-1.5">
                <label htmlFor={FIELD_IDS.clientSecret} className="block text-sm font-semibold text-ink-body">
                  Client secret
                </label>
                <input
                  id={FIELD_IDS.clientSecret}
                  type="password"
                  autoComplete="new-password"
                  value={form.clientSecret}
                  onChange={(e) => update("clientSecret", e.target.value)}
                  placeholder={keepSecret ? "Stored" : undefined}
                  className={inputClass(!!errors.clientSecret)}
                  aria-invalid={!!errors.clientSecret}
                  aria-describedby={describedBy("clientSecret", "idp-client-secret-hint")}
                />
                <FieldError id={`${FIELD_IDS.clientSecret}-error`} message={errors.clientSecret} />
                <p id="idp-client-secret-hint" className="text-sm text-ink-secondary">
                  {secretHint}
                </p>
              </div>

              <fieldset className="space-y-3 border-t border-divider pt-5">
                <legend className="sr-only">Accounts at the first sign-in</legend>
                <div className="space-y-1">
                  <label className="flex min-h-11 cursor-pointer items-center gap-3">
                    <input
                      type="checkbox"
                      checked={form.provisioningEnabled}
                      onChange={(e) => update("provisioningEnabled", e.target.checked)}
                      className="h-5 w-5 flex-shrink-0 accent-brand"
                      aria-describedby="idp-provisioning-hint"
                    />
                    <span className="text-sm font-semibold text-ink-body">Create an account at the first sign-in</span>
                  </label>
                  <p id="idp-provisioning-hint" className="pl-8 text-sm text-ink-secondary">
                    When no user of your organization has the email address your provider asserts, the sign-in creates
                    one.
                  </p>
                </div>

                {form.provisioningEnabled && (
                  <div className="space-y-4 pl-8">
                    <div className="space-y-1.5">
                      <label htmlFor="idp-role" className="block text-sm font-semibold text-ink-body">
                        Role of a created account
                      </label>
                      <select
                        id="idp-role"
                        value={form.provisioningRole}
                        onChange={(e) => update("provisioningRole", e.target.value as OidcProvisionedRole)}
                        className={inputClass(false)}
                        aria-describedby="idp-role-hint"
                      >
                        {PROVISIONED_ROLES.map((role) => (
                          <option key={role.value} value={role.value}>
                            {role.label}
                          </option>
                        ))}
                      </select>
                      <p id="idp-role-hint" className="text-sm text-ink-secondary">
                        An account is never created as an administrator.
                      </p>
                    </div>

                    <div className="space-y-1.5">
                      <label htmlFor={FIELD_IDS.emailDomains} className="block text-sm font-semibold text-ink-body">
                        Email domains
                      </label>
                      <input
                        id={FIELD_IDS.emailDomains}
                        type="text"
                        autoComplete="off"
                        spellCheck={false}
                        value={form.emailDomains}
                        onChange={(e) => update("emailDomains", e.target.value)}
                        placeholder="example.com"
                        className={inputClass(!!errors.emailDomains)}
                        aria-invalid={!!errors.emailDomains}
                        aria-describedby={describedBy("emailDomains", "idp-email-domains-hint")}
                      />
                      <FieldError id={`${FIELD_IDS.emailDomains}-error`} message={errors.emailDomains} />
                      <p id="idp-email-domains-hint" className="text-sm text-ink-secondary">
                        Accounts are created only for addresses at these domains, not at their subdomains. Separate up
                        to 20 with commas.
                      </p>
                    </div>
                  </div>
                )}
              </fieldset>

              <fieldset className="border-t border-divider pt-5">
                <legend className="sr-only">Password sign-in</legend>
                <div className="space-y-1">
                  <label className="flex min-h-11 cursor-pointer items-center gap-3">
                    <input
                      type="checkbox"
                      checked={form.requireProviderSignIn}
                      onChange={(e) => update("requireProviderSignIn", e.target.checked)}
                      className="h-5 w-5 flex-shrink-0 accent-brand"
                      aria-describedby="idp-require-hint"
                    />
                    <span className="text-sm font-semibold text-ink-body">Require sign-in through the provider</span>
                  </label>
                  <p id="idp-require-hint" className="pl-8 text-sm text-ink-secondary">
                    Users who are not administrators can no longer sign in with their password, and the sign-in page
                    offers them the provider. Administrators keep password sign-in.
                  </p>
                </div>
              </fieldset>

              <ActionOutcome outcome={outcome} onDismiss={() => setOutcome(null)} />

              <div className="flex flex-col gap-3 border-t border-divider pt-5 sm:flex-row sm:items-center sm:justify-between">
                <Button type="submit" size="lg" disabled={busy !== null}>
                  {busy === "saving" ? "Saving..." : "Save"}
                </Button>
                {config.configured && (
                  <Button
                    type="button"
                    variant="outline"
                    size="lg"
                    onClick={() => setConfirmingRemoval(true)}
                    disabled={busy !== null}
                  >
                    {busy === "removing" ? "Removing..." : "Remove identity provider"}
                  </Button>
                )}
              </div>
            </form>

            {signInLink && (
              <section aria-labelledby="idp-link-heading" className="glass space-y-3 p-4 sm:p-6">
                <h2 id="idp-link-heading" className="text-base font-semibold text-ink">
                  Sign-in link
                </h2>
                <p className="text-sm text-ink-body">
                  Share this link with your organization&apos;s users. Its sign-in page offers your identity provider
                  beside the email and password form.
                </p>
                <CopyValue label="sign-in link" value={signInLink} />
              </section>
            )}
          </>
        )}

        <AlertDialog open={confirmingRemoval} onOpenChange={setConfirmingRemoval}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Remove the identity provider?</AlertDialogTitle>
              <AlertDialogDescription>
                Your users can no longer sign in through it, and the requirement to sign in through it is removed with
                it. Accounts created at sign-in have no password.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction onClick={remove}>Remove</AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </AuthGuard>
  );
}
