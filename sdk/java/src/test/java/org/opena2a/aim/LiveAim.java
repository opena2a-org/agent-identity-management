package org.opena2a.aim;

import java.net.HttpURLConnection;
import java.net.URL;
import java.util.Map;

/**
 * Decides whether the tests that call a live AIM server can run with the SDK
 * credential this machine holds, so that a credential left from a server that
 * no longer runs skips them instead of failing {@code mvn install}.
 */
public final class LiveAim {

    /** The server {@code AIMClient.secure()} calls when the credential names none. */
    static final String DEFAULT_AIM_URL = "http://localhost:8080";

    private static final int TIMEOUT_MILLIS = 2000;

    private LiveAim() {
    }

    /**
     * Whether a live test can sign in with these credentials: they carry a
     * refresh token, or a client ID and a client secret, none of them empty,
     * and the AIM server they name ({@code aimUrl}, else
     * {@value #DEFAULT_AIM_URL}) answers {@code GET /health} with 200 within
     * two seconds.
     */
    public static boolean canSignIn(Map<String, String> credentials) {
        boolean hasSignIn = !isBlank(credentials.get("refreshToken"))
                || (!isBlank(credentials.get("clientId")) && !isBlank(credentials.get("clientSecret")));
        return hasSignIn && answersHealth(credentials.getOrDefault("aimUrl", DEFAULT_AIM_URL));
    }

    /** Whether {@code GET <aimUrl>/health} answers 200 within two seconds. */
    static boolean answersHealth(String aimUrl) {
        try {
            HttpURLConnection conn = (HttpURLConnection) new URL(aimUrl + "/health").openConnection();
            conn.setRequestMethod("GET");
            conn.setConnectTimeout(TIMEOUT_MILLIS);
            conn.setReadTimeout(TIMEOUT_MILLIS);
            try {
                return conn.getResponseCode() == 200;
            } finally {
                conn.disconnect();
            }
        } catch (Exception e) {
            return false;
        }
    }

    /**
     * An identifier shortened for a log line: its first eight characters
     * followed by "...", the whole value when it has eight characters or
     * fewer, and "n/a" for null.
     */
    public static String head(String value) {
        if (value == null) {
            return "n/a";
        }
        return value.length() > 8 ? value.substring(0, 8) + "..." : value;
    }

    private static boolean isBlank(String value) {
        return value == null || value.isBlank();
    }
}
