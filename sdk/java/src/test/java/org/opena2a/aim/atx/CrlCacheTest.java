package org.opena2a.aim.atx;

import org.junit.jupiter.api.Test;

import java.time.Duration;
import java.util.List;
import java.util.concurrent.atomic.AtomicLong;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

class CrlCacheTest {

    private static AtxTrustAnchors.Crl list(String agentId) {
        return new AtxTrustAnchors.Crl(List.of(new AtxTrustAnchors.Crl.Entry(agentId, "key compromise")));
    }

    @Test
    void servesWhileFreshAndNullOnceStale() {
        AtomicLong clock = new AtomicLong(0);
        CrlCache cache = CrlCache.builder(() -> list("agent-1"))
                .ttl(Duration.ofMillis(1000))
                .nowMillis(clock::get)
                .build();

        assertNull(cache.current(), "nothing fetched yet");
        assertFalse(cache.status().loaded());

        cache.refreshNow();
        assertNotNull(cache.current());
        assertTrue(cache.isFresh());

        clock.set(1000); // exactly at TTL is still fresh (age <= ttl)
        assertNotNull(cache.current());

        clock.set(1001); // beyond TTL
        assertNull(cache.current());
        assertFalse(cache.isFresh());
        assertTrue(cache.status().loaded(), "still loaded, just stale");
    }

    @Test
    void retainsPreviousListAndRecordsErrorWhenRefreshFails() {
        AtomicLong clock = new AtomicLong(0);
        AtomicLong call = new AtomicLong(0);
        CrlCache cache = CrlCache.builder(() -> {
                    if (call.incrementAndGet() == 1) {
                        return list("agent-1");
                    }
                    throw new RuntimeException("feed down");
                })
                .ttl(Duration.ofMillis(1000))
                .nowMillis(clock::get)
                .onError(t -> { /* silence in test */ })
                .build();

        cache.refreshNow();
        assertNotNull(cache.current());

        clock.set(500);
        cache.refreshNow(); // fails
        assertNotNull(cache.current(), "previous list retained within TTL");
        assertNotNull(cache.status().lastError());

        clock.set(1001); // past TTL from the original successful fetch
        assertNull(cache.current(), "stale -> caller applies stale policy");
    }

    @Test
    void updatesToNewestListOnSuccessfulRefresh() {
        AtomicLong clock = new AtomicLong(0);
        AtomicReference<AtxTrustAnchors.Crl> source = new AtomicReference<>(list("agent-1"));
        CrlCache cache = CrlCache.builder(source::get)
                .ttl(Duration.ofMillis(1000))
                .nowMillis(clock::get)
                .build();

        cache.refreshNow();
        assertEquals("agent-1", cache.current().entries().get(0).agentId());

        source.set(list("agent-2"));
        cache.refreshNow();
        assertEquals("agent-2", cache.current().entries().get(0).agentId());
    }

    @Test
    void nullFromSourceIsCapturedNotThrown() {
        CrlCache cache = CrlCache.builder(() -> null)
                .ttl(Duration.ofMillis(1000))
                .onError(t -> { })
                .build();
        cache.refreshNow(); // must not throw
        assertNull(cache.current());
        assertNotNull(cache.status().lastError());
    }

    @Test
    void validatesConfiguration() {
        assertThrows(NullPointerException.class, () -> CrlCache.builder(null));
        assertThrows(IllegalArgumentException.class,
                () -> CrlCache.builder(() -> list("a")).ttl(Duration.ZERO).build());
        assertThrows(IllegalArgumentException.class,
                () -> CrlCache.builder(() -> list("a")).refreshInterval(Duration.ZERO).build());
    }

    @Test
    void defaultsToSoftOpenAndHonorsReject() {
        assertEquals(CrlCache.StalePolicy.SOFT_OPEN,
                CrlCache.builder(() -> list("a")).build().onStale());
        assertEquals(CrlCache.StalePolicy.REJECT,
                CrlCache.builder(() -> list("a")).onStale(CrlCache.StalePolicy.REJECT).build().onStale());
    }

    @Test
    void startRefreshesAndStopIsIdempotent() throws Exception {
        AtomicLong calls = new AtomicLong(0);
        CrlCache cache = CrlCache.builder(() -> {
                    calls.incrementAndGet();
                    return list("agent-1");
                })
                .ttl(Duration.ofSeconds(10))
                .refreshInterval(Duration.ofSeconds(10))
                .build();
        cache.start();
        // The immediate scheduled refresh runs on a background thread; give it a moment.
        for (int i = 0; i < 50 && calls.get() == 0; i++) {
            Thread.sleep(10);
        }
        assertTrue(calls.get() >= 1, "start() should trigger an immediate refresh");
        cache.stop();
        cache.stop(); // no throw
    }

    // A Crl whose entry list is null is MALFORMED, not empty, and the cache must refuse it.
    //
    // This is the shape a decoder produces when the feed carries its list under a different
    // key — which is exactly what the AIM server does: it serves `revocations`, and this
    // record binds `entries`. Caching it marks the cache FRESH, and LocalAtxVerifier then
    // reads null entries as "nothing is revoked", so every revoked credential verifies with
    // no error, no log line, and a healthy-looking status(). That is the silent fail-open
    // the stale policy exists to prevent, arriving through a shape mismatch rather than a
    // network fault.
    @Test
    void refusesToCacheACrlWithNullEntries() {
        AtomicLong clock = new AtomicLong(0);
        AtomicReference<AtxTrustAnchors.Crl> next = new AtomicReference<>(new AtxTrustAnchors.Crl(null));

        CrlCache cache = CrlCache.builder(next::get)
                .ttl(Duration.ofMinutes(5))
                .nowMillis(clock::get)
                .build();

        cache.refreshNow();

        assertNull(cache.current(), "a malformed CRL must not become the served list");
        assertFalse(cache.isFresh(), "a malformed CRL must not mark the cache fresh");
        assertNotNull(cache.status().lastError(), "refusing a malformed CRL must surface an error");
        assertTrue(cache.status().lastError().getMessage().contains("null entries"));
    }

    // The control, and the line the guard must not cross: an EMPTY list is a legitimate
    // answer meaning "nothing is revoked right now", and it must still be cached and served.
    @Test
    void stillAcceptsAGenuinelyEmptyCrl() {
        AtomicLong clock = new AtomicLong(0);

        CrlCache cache = CrlCache.builder(() -> new AtxTrustAnchors.Crl(List.of()))
                .ttl(Duration.ofMinutes(5))
                .nowMillis(clock::get)
                .build();

        cache.refreshNow();

        assertNotNull(cache.current(), "an empty CRL is a valid answer and must be served");
        assertTrue(cache.isFresh());
        assertNull(cache.status().lastError());
        assertEquals(0, cache.current().entries().size());
    }

    // A malformed refresh must not destroy a good list that is still within its TTL.
    @Test
    void retainsThePreviousGoodListWhenARefreshIsMalformed() {
        AtomicLong clock = new AtomicLong(0);
        AtomicReference<AtxTrustAnchors.Crl> next = new AtomicReference<>(list("agent-1"));

        CrlCache cache = CrlCache.builder(next::get)
                .ttl(Duration.ofMinutes(5))
                .nowMillis(clock::get)
                .build();

        cache.refreshNow();
        assertNotNull(cache.current());

        next.set(new AtxTrustAnchors.Crl(null));
        cache.refreshNow();

        assertNotNull(cache.current(), "a malformed refresh must not discard a still-fresh good list");
        assertEquals(1, cache.current().entries().size());
        assertNotNull(cache.status().lastError(), "the malformed refresh must still be recorded");
    }
}
