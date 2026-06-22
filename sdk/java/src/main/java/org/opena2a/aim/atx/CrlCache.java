package org.opena2a.aim.atx;

import java.lang.System.Logger;
import java.lang.System.Logger.Level;
import java.time.Duration;
import java.util.Objects;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;
import java.util.function.Consumer;
import java.util.function.LongSupplier;

/**
 * Asynchronously-refreshed revocation-list (CRL) cache for local ATX verification.
 *
 * <p>Revocation is the one credential check designed to ride on a short-lived, cached
 * list (AAP §6): the decision path never makes a network call. A background timer
 * refreshes the list on an interval; {@link #current()} returns the cached list while
 * it is within its TTL and {@code null} once it goes stale.
 *
 * <p>By default a stale cache <em>soft-fails open on revocation only</em> — signature,
 * expiry and issuer-trust stay hard (see {@link RefreshingAtxVerifier}) — so a transient
 * CRL-feed outage degrades revocation freshness instead of taking down all verification.
 * The TTL bounds the window in which a revoked credential could slip through; choose
 * {@link StalePolicy#REJECT} for a fail-closed deployment.
 *
 * <p>Thread-safe: {@link #current()}/{@link #isFresh()} are lock-free reads of volatile
 * state and never throw, so they are safe on the verification decision path.
 */
public final class CrlCache {

    /** What a verifier should do when the cached CRL is stale (beyond TTL). */
    public enum StalePolicy {
        /** Skip revocation (other checks stay hard). */
        SOFT_OPEN,
        /** Fail the credential closed. */
        REJECT
    }

    /** Supplies the current CRL. Invoked only off the decision path (background timer + {@link #refreshNow()}). */
    @FunctionalInterface
    public interface CrlSource {
        AtxTrustAnchors.Crl fetch() throws Exception;
    }

    private static final Logger LOG = System.getLogger(CrlCache.class.getName());

    private final CrlSource source;
    private final long ttlMillis;
    private final long refreshIntervalMillis;
    private final StalePolicy onStale;
    private final LongSupplier nowMillis;
    private final Consumer<Throwable> onError;

    private volatile AtxTrustAnchors.Crl data;
    private volatile long fetchedAtMillis = -1L;
    private volatile Throwable lastError;
    private ScheduledExecutorService scheduler;

    private CrlCache(Builder b) {
        this.source = b.source;
        if (b.ttlMillis <= 0) {
            throw new IllegalArgumentException("ttl must be > 0");
        }
        this.ttlMillis = b.ttlMillis;
        this.refreshIntervalMillis =
                b.refreshIntervalMillis != null ? b.refreshIntervalMillis : Math.max(1L, b.ttlMillis / 2L);
        if (this.refreshIntervalMillis <= 0) {
            throw new IllegalArgumentException("refreshInterval must be > 0");
        }
        this.onStale = b.onStale;
        this.nowMillis = b.nowMillis;
        this.onError = b.onError != null
                ? b.onError
                : (t -> LOG.log(Level.WARNING, "CRL refresh failed; serving cached list until TTL", t));
    }

    /** Start building a cache around {@code source}. */
    public static Builder builder(CrlSource source) {
        return new Builder(source);
    }

    /**
     * Fetch the CRL once and update the cache. Never throws — a fetch failure is captured
     * ({@link #status()}) and the previous list is retained until it goes stale. Synchronized
     * so a manual call and the background timer cannot interleave a half-written update.
     */
    public synchronized void refreshNow() {
        try {
            AtxTrustAnchors.Crl next = source.fetch();
            if (next == null) {
                throw new IllegalStateException("CRL source returned null");
            }
            this.data = next;
            this.fetchedAtMillis = nowMillis.getAsLong();
            this.lastError = null;
        } catch (Throwable t) {
            this.lastError = t;
            onError.accept(t);
        }
    }

    /** Start the background refresh timer (idempotent) and run an immediate refresh on the timer thread. */
    public synchronized void start() {
        if (scheduler != null) {
            return;
        }
        scheduler = Executors.newSingleThreadScheduledExecutor(r -> {
            Thread t = new Thread(r, "atx-crl-refresh");
            t.setDaemon(true);
            return t;
        });
        scheduler.scheduleAtFixedRate(this::refreshNow, 0L, refreshIntervalMillis, TimeUnit.MILLISECONDS);
    }

    /** Stop the background refresh timer (idempotent). */
    public synchronized void stop() {
        if (scheduler != null) {
            scheduler.shutdownNow();
            scheduler = null;
        }
    }

    /** Whether the cache holds a list that is within its TTL. */
    public boolean isFresh() {
        long at = fetchedAtMillis;
        return at >= 0 && (nowMillis.getAsLong() - at) <= ttlMillis;
    }

    /**
     * The cached CRL while fresh (within TTL), otherwise {@code null}. Lock-free and never
     * throws — safe on the decision path. A {@code null} return means the caller must apply
     * its stale policy ({@link StalePolicy}).
     */
    public AtxTrustAnchors.Crl current() {
        return isFresh() ? data : null;
    }

    /** The configured stale policy. */
    public StalePolicy onStale() {
        return onStale;
    }

    /** Observability snapshot; never throws. */
    public Status status() {
        long at = fetchedAtMillis;
        Long ageMs = at < 0 ? null : (nowMillis.getAsLong() - at);
        return new Status(data != null, ageMs, isFresh(), lastError);
    }

    /** Cache state snapshot. {@code ageMillis} is null when nothing has been loaded yet. */
    public record Status(boolean loaded, Long ageMillis, boolean fresh, Throwable lastError) {}

    /** Fluent builder for {@link CrlCache}. */
    public static final class Builder {
        private final CrlSource source;
        private long ttlMillis = 300_000L;
        private Long refreshIntervalMillis;
        private StalePolicy onStale = StalePolicy.SOFT_OPEN;
        private LongSupplier nowMillis = System::currentTimeMillis;
        private Consumer<Throwable> onError;

        private Builder(CrlSource source) {
            this.source = Objects.requireNonNull(source, "source");
        }

        /** Max age a cached CRL is served (default 5 minutes). */
        public Builder ttl(Duration ttl) {
            this.ttlMillis = Objects.requireNonNull(ttl, "ttl").toMillis();
            return this;
        }

        /** Background refresh interval (default {@code ttl/2}). */
        public Builder refreshInterval(Duration interval) {
            this.refreshIntervalMillis = Objects.requireNonNull(interval, "interval").toMillis();
            return this;
        }

        /** Stale-cache behavior (default {@link StalePolicy#SOFT_OPEN}). */
        public Builder onStale(StalePolicy policy) {
            this.onStale = Objects.requireNonNull(policy, "policy");
            return this;
        }

        /** Injectable millisecond clock (tests). Defaults to {@code System::currentTimeMillis}. */
        public Builder nowMillis(LongSupplier nowMillis) {
            this.nowMillis = Objects.requireNonNull(nowMillis, "nowMillis");
            return this;
        }

        /** Sink for refresh errors (default logs at WARNING). */
        public Builder onError(Consumer<Throwable> onError) {
            this.onError = onError;
            return this;
        }

        public CrlCache build() {
            return new CrlCache(this);
        }
    }
}
