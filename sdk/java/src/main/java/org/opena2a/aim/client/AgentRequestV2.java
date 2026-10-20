package org.opena2a.aim.client;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import okhttp3.HttpUrl;
import org.opena2a.aim.crypto.pqc.KeySizes;
import org.opena2a.aim.crypto.pqc.MLDSAKeyPair;
import org.opena2a.aim.crypto.pqc.PQCOperations;
import org.opena2a.aim.exceptions.ConfigurationException;
import org.opena2a.aim.json.SdkObjectMappers;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.security.SecureRandom;
import java.time.Instant;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.Arrays;
import java.util.Base64;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Set;
import java.util.regex.Pattern;

/**
 * agent-request-v2: a request signed as a statement about itself, the form AIM's
 * AgentRequestV2Middleware verifies.
 *
 * <p>The statement is a JSON object whose members name the request: {@code method},
 * {@code target} (the path and query string OkHttp sends for the url), {@code audience} (the
 * url's origin), {@code body_sha256} (the unpadded base64url SHA-256 of the body, present exactly
 * when the body is not empty), {@code agent_id}, {@code timestamp} (RFC 3339 in UTC) and
 * {@code nonce} (16 bytes, unpadded base64url). It is sent as unpadded base64url in
 * {@code X-Agent-Request}. Each signature covers the DSSE v1 pre-authentication encoding of the
 * payload type and the statement, and is sent in {@code X-Agent-Request-Signature} as its
 * algorithm, a colon and the unpadded base64url signature. docs/specs/agent-request-v2-vectors.json
 * in the AIM repository holds requests signed this way and the answer AIM's verifier gives each.
 *
 * <p>The class is not public, and no SDK code calls it.
 */
final class AgentRequestV2 {

    /** The header that carries the statement. */
    static final String STATEMENT_HEADER = "X-Agent-Request";

    /** The header that carries the signatures. */
    static final String SIGNATURE_HEADER = "X-Agent-Request-Signature";

    /** The DSSE payload type of a statement. */
    static final String PAYLOAD_TYPE = "application/vnd.opena2a.agent-request.v2+json";

    /** The length of a statement's nonce, in bytes. */
    static final int NONCE_BYTES = 16;

    private static final Set<String> METHODS = Set.of("GET", "POST", "PUT", "PATCH", "DELETE");

    /** An agent ID in the form AIM's verifier reads: a UUID of 36 characters. */
    private static final Pattern AGENT_ID =
            Pattern.compile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$");

    private static final DateTimeFormatter TIMESTAMP =
            DateTimeFormatter.ofPattern("uuuu-MM-dd'T'HH:mm:ss'Z'").withZone(ZoneOffset.UTC);

    private static final Base64.Encoder BASE64URL = Base64.getUrlEncoder().withoutPadding();
    private static final ObjectMapper JSON = SdkObjectMappers.create();
    private static final SecureRandom RANDOM = new SecureRandom();

    private static final String METHOD_FIX = "Pass the method in upper case as one of GET, POST, PUT, PATCH or DELETE.";
    private static final String URL_FIX =
            "Pass the absolute http or https URL the request is sent to, with no username or password in it.";
    private static final String AGENT_FIX =
            "Pass the agent ID AIM returned at registration and the agent's Ed25519 private key, its 32-byte seed.";
    private static final String ML_DSA_FIX =
            "Pass the agent's registered ML-DSA key pair as PQCOperations.generateMLDSAKeyPair returns it, "
                    + "whose private key is its 32-byte seed.";

    private AgentRequestV2() {
    }

    /**
     * Signs a request as the agent and returns the two headers to send with it,
     * {@code X-Agent-Request} and then {@code X-Agent-Request-Signature}. The statement names the
     * method, the path and query string OkHttp sends for {@code url}, the origin of {@code url} as
     * the audience, the SHA-256 of {@code body} when it is not empty, the agent ID, this host's
     * clock in whole seconds and a random 16-byte nonce drawn for this call.
     *
     * <p>AIM's verifier refuses a statement whose nonce it has already admitted for the agent, and
     * one whose time is more than 30 seconds from its database clock, earlier or later. It compares
     * the audience byte for byte with the origin it is configured with.
     *
     * <p>The Ed25519 key signs the statement. Given {@code mlDsa}, for an agent in hybrid mode, the
     * ML-DSA key signs it too, and {@code X-Agent-Request-Signature} carries the two signatures
     * separated by a comma and a space.
     *
     * <p>A method, url, agent ID or key it cannot sign for throws {@link ConfigurationException}
     * before anything is signed. Send the same method, url and body bytes with these headers, and
     * no Authorization, X-API-Key or agent-request-v1 header: AIM refuses a statement sent beside
     * another credential. Do not log them.
     *
     * @param method      GET, POST, PUT, PATCH or DELETE, in upper case
     * @param url         the absolute http or https URL the request is sent to
     * @param body        the body exactly as sent; null or empty when there is none
     * @param agentId     the agent's ID
     * @param ed25519Seed the agent's Ed25519 private key, its 32-byte seed
     * @param mlDsa       the agent's ML-DSA key pair, for an agent in hybrid mode; otherwise null
     * @return the two headers, in that order
     */
    static Map<String, String> headers(String method, String url, byte[] body, String agentId,
                                       byte[] ed25519Seed, MLDSAKeyPair mlDsa) {
        // The host clock only; never a time taken from a response or the network.
        long signedAt = Instant.now().getEpochSecond();
        byte[] nonce = new byte[NONCE_BYTES];
        RANDOM.nextBytes(nonce);
        return sign(method, url, body, agentId, ed25519Seed, mlDsa, signedAt, nonce);
    }

    /**
     * {@link #headers} with the signing time, in Unix seconds, and the nonce given, which the
     * conformance vectors fix. A signer takes them from the host clock and a random source, as
     * {@link #headers} does.
     */
    static Map<String, String> sign(String method, String url, byte[] body, String agentId,
                                    byte[] ed25519Seed, MLDSAKeyPair mlDsa, long signedAt, byte[] nonce) {
        if (method == null || !METHODS.contains(method)) {
            throw refusal("the method is not one of GET, POST, PUT, PATCH or DELETE in upper case", METHOD_FIX);
        }
        HttpUrl parsed = url == null ? null : HttpUrl.parse(url);
        if (parsed == null) {
            throw refusal("the url is not an absolute http or https URL", URL_FIX);
        }
        if (!parsed.username().isEmpty() || !parsed.password().isEmpty()) {
            throw refusal("the url carries a username or a password", URL_FIX);
        }
        if (agentId == null || agentId.isEmpty()) {
            throw refusal("no agent ID is configured", AGENT_FIX);
        }
        if (!AGENT_ID.matcher(agentId).matches()) {
            throw refusal("the agent ID is not a UUID in its 36-character form", AGENT_FIX);
        }
        if (ed25519Seed == null) {
            throw refusal("no agent private key is configured", AGENT_FIX);
        }
        if (ed25519Seed.length != KeySizes.ED25519_PRIVATE_KEY) {
            throw refusal("the agent private key is " + ed25519Seed.length
                    + " bytes long, not 32, the length of an Ed25519 seed", AGENT_FIX);
        }
        byte[] mlDsaSeed = mlDsa == null ? null : mlDsa.getPrivateKey();
        if (mlDsaSeed != null && mlDsaSeed.length != KeySizes.ML_DSA_SEED) {
            throw refusal("the ML-DSA private key is " + mlDsaSeed.length
                    + " bytes long, not 32, the length of an ML-DSA seed", ML_DSA_FIX);
        }
        byte[] content = body == null ? new byte[0] : body;

        // The members in the order the format lists them.
        Map<String, String> statement = new LinkedHashMap<>();
        statement.put("method", method);
        statement.put("target", target(parsed));
        statement.put("audience", origin(parsed));
        if (content.length > 0) {
            statement.put("body_sha256", BASE64URL.encodeToString(sha256(content)));
        }
        statement.put("agent_id", agentId);
        statement.put("timestamp", TIMESTAMP.format(Instant.ofEpochSecond(signedAt)));
        statement.put("nonce", BASE64URL.encodeToString(nonce));

        byte[] payload = compactJson(statement);
        byte[] message = preAuthenticationEncoding(payload);
        StringBuilder signatures = new StringBuilder("Ed25519:")
                .append(BASE64URL.encodeToString(PQCOperations.signEd25519(ed25519Seed, message)));
        if (mlDsa != null) {
            signatures.append(", ")
                    .append(mlDsa.getAlgorithm().getAlgorithmId())
                    .append(':')
                    .append(BASE64URL.encodeToString(PQCOperations.signMLDSA(mlDsa.getAlgorithm(), mlDsaSeed, message)));
        }

        Map<String, String> headers = new LinkedHashMap<>();
        headers.put(STATEMENT_HEADER, BASE64URL.encodeToString(payload));
        headers.put(SIGNATURE_HEADER, signatures.toString());
        return Collections.unmodifiableMap(headers);
    }

    /**
     * The request target OkHttp sends for {@code url}: its encoded path, then {@code ?} and its
     * encoded query when it has one.
     */
    private static String target(HttpUrl url) {
        String query = url.encodedQuery();
        return query == null ? url.encodedPath() : url.encodedPath() + "?" + query;
    }

    /** The origin of {@code url}: its scheme, its host, and its port when it is not the scheme's default. */
    private static String origin(HttpUrl url) {
        String host = url.host().contains(":") ? "[" + url.host() + "]" : url.host();
        String origin = url.scheme() + "://" + host;
        return url.port() == HttpUrl.defaultPort(url.scheme()) ? origin : origin + ":" + url.port();
    }

    /**
     * The DSSE v1 pre-authentication encoding of the payload type and {@code payload}: "DSSEv1",
     * the byte length of the type, the type, the byte length of the payload and the payload,
     * separated by single spaces.
     */
    private static byte[] preAuthenticationEncoding(byte[] payload) {
        byte[] head = ("DSSEv1 " + PAYLOAD_TYPE.length() + " " + PAYLOAD_TYPE + " " + payload.length + " ")
                .getBytes(StandardCharsets.US_ASCII);
        byte[] message = Arrays.copyOf(head, head.length + payload.length);
        System.arraycopy(payload, 0, message, head.length, payload.length);
        return message;
    }

    private static byte[] compactJson(Map<String, String> statement) {
        try {
            return JSON.writeValueAsBytes(statement);
        } catch (JsonProcessingException e) {
            throw new IllegalStateException("the agent-request-v2 statement could not be written as JSON", e);
        }
    }

    private static byte[] sha256(byte[] data) {
        try {
            return MessageDigest.getInstance("SHA-256").digest(data);
        } catch (NoSuchAlgorithmException e) {
            throw new IllegalStateException("SHA-256 is not available", e);
        }
    }

    private static ConfigurationException refusal(String line, String fix) {
        return new ConfigurationException(line + "; nothing was signed\nFix: " + fix);
    }
}
