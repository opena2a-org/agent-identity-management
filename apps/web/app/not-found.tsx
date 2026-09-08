import Link from "next/link";

export default function NotFound() {
  return (
    <main className="glass-page relative flex min-h-screen items-center justify-center overflow-hidden p-6">
      <div className="glass-chrome w-full max-w-md p-8 text-center">
        <p className="text-overline">404</p>
        <h1 className="text-headline mt-2">This page does not exist.</h1>
        <p className="mt-2 text-sm text-ink-secondary">
          The link may be out of date.
        </p>
        <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
          <Link
            href="/dashboard"
            className="inline-flex h-10 items-center rounded-pill bg-brand px-5 text-sm font-bold text-white shadow-glow hover:bg-brand-hover"
          >
            Go to the dashboard
          </Link>
          <Link
            href="/auth/login"
            className="inline-flex h-10 items-center rounded-pill border border-stroke bg-glass px-5 text-sm font-bold text-ink"
          >
            Sign in
          </Link>
        </div>
      </div>
    </main>
  );
}
