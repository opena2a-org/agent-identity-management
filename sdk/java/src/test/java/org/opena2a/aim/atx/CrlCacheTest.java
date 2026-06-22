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
}
