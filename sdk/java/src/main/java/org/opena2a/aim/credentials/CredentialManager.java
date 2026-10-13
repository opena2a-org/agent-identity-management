package org.opena2a.aim.credentials;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.opena2a.aim.exceptions.CredentialException;
import org.opena2a.aim.json.SdkObjectMappers;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.File;
import java.io.IOException;
import java.io.InputStream;
import java.net.URL;
import java.nio.ByteBuffer;
import java.nio.channels.FileChannel;
import java.nio.channels.FileLock;
import java.nio.channels.OverlappingFileLockException;
import java.nio.file.AccessDeniedException;
import java.nio.file.AtomicMoveNotSupportedException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.nio.file.StandardCopyOption;
import java.nio.file.StandardOpenOption;
import java.nio.file.attribute.PosixFilePermission;
import java.nio.file.attribute.PosixFilePermissions;
import java.time.Duration;
import java.util.Collections;
import java.util.EnumSet;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.locks.ReentrantLock;

/**
 * Manages AIM SDK credentials with intelligent discovery and migration.
 *
 * <p>Directory Structure:</p>
 * <pre>
 *   ~/.aim/
 *   ├── sdk_credentials.json      # SDK OAuth tokens
 *   ├── agents/
 *   │   ├── my-agent.json         # Agent "my-agent" keys
 *   │   └── other-agent.json      # Agent "other-agent" keys
 *   └── credentials.json          # LEGACY (auto-migrated)
 * </pre>
 *
 * <p>Credentials are loaded from (in order):</p>
 * <ol>
 *   <li>Environment variables (AIM_URL, AIM_REFRESH_TOKEN, AIM_SDK_TOKEN_ID)</li>
 *   <li>~/.aim/sdk_credentials.json (primary location)</li>
 *   <li>~/.aim/credentials.json (legacy - auto-migrates)</li>
 *   <li>SDK package .aim/sdk_credentials.json (fresh download - auto-installs)</li>
 *   <li>Current directory .aim/sdk_credentials.json</li>
 * </ol>
 *
 * @see CredentialType
 */
@SuppressWarnings("javadoc")
public class CredentialManager {

    private static final Logger logger = LoggerFactory.getLogger(CredentialManager.class);
    private static final ObjectMapper objectMapper = SdkObjectMappers.create();

    private static final String AIM_DIR = ".aim";
    private static final String AGENTS_DIR = "agents";
    private static final String SDK_CREDENTIALS_FILE = "sdk_credentials.json";
    private static final String CREDENTIALS_FILE = "credentials.json";
    private static final String SCHEMA_VERSION = "1.0";

    /**
     * Credentials key of the access token issued with the stored refresh token. A process
     * that refreshes stores it, so another process sharing the file uses it instead of
     * refreshing again. In the file it is written as an object, {@code "sharedAccessToken":
     * {"token": ..., "refreshTokenSha256": ...}}, as the Python SDK writes it.
     */
    public static final String SHARED_ACCESS_TOKEN = "sharedAccessToken";

    /**
     * Credentials key of the SHA-256, in lowercase hex, of the refresh token the
     * {@link #SHARED_ACCESS_TOKEN shared access token} was issued with. It is stored with the
     * access token, as its {@code refreshTokenSha256} field.
     */
    public static final String SHARED_ACCESS_TOKEN_REFRESH_TOKEN_SHA256 = "sharedAccessTokenRefreshTokenSha256";

    /**
     * How long another holder of the SDK credentials lock is waited for before the save or
     * action that needs it is given up, in milliseconds. The Python SDK waits as long.
     */
    public static final long SDK_CREDENTIALS_LOCK_TIMEOUT_MILLIS = 60_000L;

    /** The wait used where no timeout is passed; tests shorten it. */
    static volatile long defaultLockTimeoutMillis = SDK_CREDENTIALS_LOCK_TIMEOUT_MILLIS;

    private static final long LOCK_POLL_MILLIS = 50L;

    private static final Set<PosixFilePermission> OWNER_READ_WRITE =
            EnumSet.of(PosixFilePermission.OWNER_READ, PosixFilePermission.OWNER_WRITE);

    /**
     * One lock per lock file for the threads of this JVM. A file lock belongs to the whole
     * JVM, so it does not keep two threads apart; this lock does, and it lets the thread
     * holding it take it again.
     */
    private static final ConcurrentHashMap<Path, ReentrantLock> THREAD_LOCKS = new ConcurrentHashMap<>();

    /** The file lock this JVM holds on each lock file, owned by the thread holding its thread lock. */
    private static final ConcurrentHashMap<Path, FileLock> FILE_LOCKS = new ConcurrentHashMap<>();

    /**
     * An action run while the SDK credentials lock is held.
     *
     * @param <T> what the action returns
     * @param <E> the checked exception the action may throw
     */
    @FunctionalInterface
    public interface LockedAction<T, E extends Exception> {
        /**
         * Runs the action.
         *
         * @return the action's result
         * @throws E when the action fails
         */
        T run() throws E;
    }

    /**
     * Credential types in the AIM ecosystem.
     */
    public enum CredentialType {
        /** SDK authentication using OAuth refresh tokens */
        SDK_OAUTH("sdk_oauth"),
        /** Agent authentication using Ed25519 keys */
        AGENT("agent"),
        /** Unrecognized credential format */
        UNKNOWN("unknown");

        private final String value;

        CredentialType(String value) {
            this.value = value;
        }

        /**
         * Gets the string value of this credential type.
         *
         * @return the credential type value
         */
        public String getValue() {
            return value;
        }
    }

    /**
     * Detect the type of credentials from their structure.
     *
     * @param data Credential data map
     * @return CredentialType constant
     */
    public static CredentialType detectCredentialType(Map<String, String> data) {
        // SDK OAuth credentials have refreshToken
        if (data.containsKey("refreshToken") || data.containsKey("refresh_token")) {
            return CredentialType.SDK_OAUTH;
        }

        // Agent credentials have agentId/agent_id and privateKey/private_key
        boolean hasAgentId = data.containsKey("agentId") || data.containsKey("agent_id");
        boolean hasPrivateKey = data.containsKey("privateKey") || data.containsKey("private_key");
        if (hasAgentId || hasPrivateKey) {
            return CredentialType.AGENT;
        }

        // Client credentials (legacy)
        if (data.containsKey("clientId") || data.containsKey("client_id")) {
            return CredentialType.SDK_OAUTH;
        }

        return CredentialType.UNKNOWN;
    }

    /**
     * Load SDK credentials with intelligent discovery and migration.
     *
     * <p>Lookup order:</p>
     * <ol>
     *   <li>Environment variables (AIM_URL, AIM_REFRESH_TOKEN, etc.)</li>
     *   <li>~/.aim/sdk_credentials.json (primary location)</li>
     *   <li>~/.aim/credentials.json (legacy - auto-migrates if SDK type)</li>
     *   <li>SDK package .aim/sdk_credentials.json (fresh download - auto-installs)</li>
     *   <li>Current directory .aim/sdk_credentials.json</li>
     * </ol>
     *
     * @return Map containing SDK credentials
     */
    public static Map<String, String> loadSdkCredentials() {
        // 1. Try environment variables first
        Map<String, String> envCreds = loadFromEnvironment();
        if (!envCreds.isEmpty()) {
            boolean hasAuth = envCreds.containsKey("refreshToken") ||
                             (envCreds.containsKey("clientId") && envCreds.containsKey("clientSecret"));
            if (hasAuth) {
                logger.info("Loaded credentials from environment variables");
                return envCreds;
            }
        }

        // 2. Try home directory SDK credentials (primary location)
        Path homeSdkPath = Paths.get(System.getProperty("user.home"), AIM_DIR, SDK_CREDENTIALS_FILE);
        Map<String, String> homeCreds = loadFromFile(homeSdkPath);
        if (!homeCreds.isEmpty() && detectCredentialType(homeCreds) == CredentialType.SDK_OAUTH) {
            logger.info("Loaded SDK credentials from {}", homeSdkPath);
            return homeCreds;
        }

        // 3. Check legacy location and auto-migrate if SDK type
        Path legacyPath = Paths.get(System.getProperty("user.home"), AIM_DIR, CREDENTIALS_FILE);
        Map<String, String> legacyCreds = loadFromFile(legacyPath);
        if (!legacyCreds.isEmpty()) {
            CredentialType type = detectCredentialType(legacyCreds);
            if (type == CredentialType.SDK_OAUTH) {
                // Migrate to new location
                migrateSdkCredentials(legacyCreds);
                logger.info("Migrated SDK credentials from legacy location to {}", homeSdkPath);
                return legacyCreds;
            } else if (type == CredentialType.AGENT) {
                // Wrong type in legacy location - agent creds, not SDK
                logger.debug("Found agent credentials in legacy location, continuing search...");
            }
        }

        // 4. Try SDK package location (for fresh downloads)
        Map<String, String> packageCreds = findPackageCredentials();
        if (!packageCreds.isEmpty()) {
            // Auto-install to home directory
            installSdkCredentials(packageCreds);
            logger.info("Auto-installed SDK credentials from package");
            return packageCreds;
        }

        // 5. Try current directory .aim/
        Path localPath = Paths.get(AIM_DIR, SDK_CREDENTIALS_FILE);
        Map<String, String> localCreds = loadFromFile(localPath);
        if (!localCreds.isEmpty() && detectCredentialType(localCreds) == CredentialType.SDK_OAUTH) {
            // Auto-install to home directory for convenience
            installSdkCredentials(localCreds);
            logger.info("Auto-installed SDK credentials from local directory");
            return localCreds;
        }

        // Also check legacy local path
        Path localLegacyPath = Paths.get(AIM_DIR, CREDENTIALS_FILE);
        Map<String, String> localLegacyCreds = loadFromFile(localLegacyPath);
        if (!localLegacyCreds.isEmpty() && detectCredentialType(localLegacyCreds) == CredentialType.SDK_OAUTH) {
            installSdkCredentials(localLegacyCreds);
            logger.info("Auto-installed SDK credentials from local directory");
            return localLegacyCreds;
        }

        logger.warn("No SDK credentials found in any location");
        return Collections.emptyMap();
    }

    /**
     * Find SDK credentials bundled with the SDK package.
     */
    private static Map<String, String> findPackageCredentials() {
        // Check current working directory
        Path[] searchPaths = new Path[] {
            Paths.get(".aim", SDK_CREDENTIALS_FILE),
            Paths.get(".aim", CREDENTIALS_FILE),
            Paths.get(SDK_CREDENTIALS_FILE),
            Paths.get(CREDENTIALS_FILE)
        };

        for (Path path : searchPaths) {
            if (Files.exists(path)) {
                Map<String, String> creds = loadFromFile(path);
                if (!creds.isEmpty() && detectCredentialType(creds) == CredentialType.SDK_OAUTH) {
                    logger.debug("Found package credentials at {}", path);
                    return creds;
                }
            }
        }

        // Also check classpath resources (for embedded SDK credentials in JARs)
        try {
            URL resourceUrl = CredentialManager.class.getClassLoader()
                    .getResource(".aim/" + SDK_CREDENTIALS_FILE);
            if (resourceUrl != null) {
                try (InputStream is = resourceUrl.openStream()) {
                    String content = new String(is.readAllBytes());
                    JsonNode json = objectMapper.readTree(content);
                    Map<String, String> creds = parseCredentialsJson(json);
                    if (detectCredentialType(creds) == CredentialType.SDK_OAUTH) {
                        logger.debug("Found package credentials in classpath");
                        return creds;
                    }
                }
            }
        } catch (Exception e) {
            logger.debug("No credentials found in classpath: {}", e.getMessage());
        }

        return Collections.emptyMap();
    }

    /**
     * Install SDK credentials to home directory.
     */
    private static void installSdkCredentials(Map<String, String> credentials) {
        Path targetPath = Paths.get(System.getProperty("user.home"), AIM_DIR, SDK_CREDENTIALS_FILE);
        try {
            saveSdkCredentials(credentials, targetPath);
            System.out.println("SDK credentials installed to " + targetPath);
        } catch (Exception e) {
            logger.warn("Failed to install SDK credentials: {}", e.getMessage());
        }
    }

    /**
     * Migrate SDK credentials from legacy location.
     */
    private static void migrateSdkCredentials(Map<String, String> credentials) {
        Path targetPath = Paths.get(System.getProperty("user.home"), AIM_DIR, SDK_CREDENTIALS_FILE);
        try {
            saveSdkCredentials(credentials, targetPath);
            System.out.println("SDK credentials migrated to " + targetPath);
        } catch (Exception e) {
            logger.warn("Failed to migrate SDK credentials: {}", e.getMessage());
        }
    }

    /**
     * Load credentials from environment variables.
     *
     * <p>Supported environment variables:</p>
     * <ul>
     *   <li>AIM_URL - AIM server URL</li>
     *   <li>AIM_REFRESH_TOKEN - SDK OAuth refresh token</li>
     *   <li>AIM_SDK_TOKEN_ID - SDK token ID (for tracking)</li>
     *   <li>AIM_CLIENT_ID - OAuth client ID (legacy)</li>
     *   <li>AIM_CLIENT_SECRET - OAuth client secret (legacy)</li>
     *   <li>AIM_ORG_ID - Organization ID</li>
     * </ul>
     */
    private static Map<String, String> loadFromEnvironment() {
        Map<String, String> creds = new HashMap<>();

        String aimUrl = System.getenv("AIM_URL");
        String refreshToken = System.getenv("AIM_REFRESH_TOKEN");
        String sdkTokenId = System.getenv("AIM_SDK_TOKEN_ID");
        String clientId = System.getenv("AIM_CLIENT_ID");
        String clientSecret = System.getenv("AIM_CLIENT_SECRET");
        String orgId = System.getenv("AIM_ORG_ID");

        if (aimUrl != null) creds.put("aimUrl", aimUrl);
        if (refreshToken != null) creds.put("refreshToken", refreshToken);
        if (sdkTokenId != null) creds.put("sdkTokenId", sdkTokenId);
        if (clientId != null) creds.put("clientId", clientId);
        if (clientSecret != null) creds.put("clientSecret", clientSecret);
        if (orgId != null) creds.put("organizationId", orgId);

        return creds;
    }

    /**
     * Parse a JSON node into credentials map.
     */
    private static Map<String, String> parseCredentialsJson(JsonNode json) {
        Map<String, String> creds = new HashMap<>();

        // AIM URL
        if (json.has("aimUrl")) creds.put("aimUrl", json.get("aimUrl").asText());
        if (json.has("aim_url")) creds.put("aimUrl", json.get("aim_url").asText());

        // SDK OAuth credentials (refresh token flow) - PREFERRED
        if (json.has("refreshToken")) creds.put("refreshToken", json.get("refreshToken").asText());
        if (json.has("refresh_token")) creds.put("refreshToken", json.get("refresh_token").asText());

        if (json.has("sdkTokenId")) creds.put("sdkTokenId", json.get("sdkTokenId").asText());
        if (json.has("sdk_token_id")) creds.put("sdkTokenId", json.get("sdk_token_id").asText());

        if (json.has("userId")) creds.put("userId", json.get("userId").asText());
        if (json.has("user_id")) creds.put("userId", json.get("user_id").asText());

        if (json.has("email")) creds.put("email", json.get("email").asText());
        if (json.has("type")) creds.put("type", json.get("type").asText());
        if (json.has("schemaVersion")) creds.put("schemaVersion", json.get("schemaVersion").asText());

        // Client credentials (OAuth client credentials flow) - LEGACY
        if (json.has("clientId")) creds.put("clientId", json.get("clientId").asText());
        if (json.has("client_id")) creds.put("clientId", json.get("client_id").asText());

        if (json.has("clientSecret")) creds.put("clientSecret", json.get("clientSecret").asText());
        if (json.has("client_secret")) creds.put("clientSecret", json.get("client_secret").asText());

        if (json.has("organizationId")) creds.put("organizationId", json.get("organizationId").asText());
        if (json.has("organization_id")) creds.put("organizationId", json.get("organization_id").asText());
        if (json.has("orgId")) creds.put("organizationId", json.get("orgId").asText());

        // Access token another process obtained with the stored refresh token
        JsonNode shared = json.path(SHARED_ACCESS_TOKEN);
        if (shared.path("token").isTextual() && shared.path("refreshTokenSha256").isTextual()) {
            creds.put(SHARED_ACCESS_TOKEN, shared.get("token").asText());
            creds.put(SHARED_ACCESS_TOKEN_REFRESH_TOKEN_SHA256, shared.get("refreshTokenSha256").asText());
        }

        return creds;
    }

    /**
     * Load credentials from a JSON file.
     * Supports both SDK OAuth (refreshToken) and client credentials (clientId/clientSecret).
     */
    private static Map<String, String> loadFromFile(Path path) {
        if (!Files.exists(path)) {
            return Collections.emptyMap();
        }

        try {
            String content = Files.readString(path);
            JsonNode json = objectMapper.readTree(content);
            return parseCredentialsJson(json);
        } catch (IOException e) {
            logger.warn("Failed to read credentials from {}: {}", path, e.getMessage());
            return Collections.emptyMap();
        }
    }

    /**
     * The SDK credentials file in the home directory, {@code ~/.aim/sdk_credentials.json}.
     *
     * @return the path of the SDK credentials file
     */
    public static Path getSdkCredentialsPath() {
        return Paths.get(System.getProperty("user.home"), AIM_DIR, SDK_CREDENTIALS_FILE);
    }

    /**
     * The lock file that serializes changes to an SDK credentials file: the file's name with
     * {@code .lock} appended, in the same directory ({@code ~/.aim/sdk_credentials.json.lock}
     * for the default file). It is never replaced or removed,
     * because the credentials file itself is replaced on every save and a lock taken on it
     * would not be seen by a process that opened the new file.
     *
     * @param credentialsPath the SDK credentials file
     * @return the path of its lock file
     */
    public static Path getSdkCredentialsLockPath(Path credentialsPath) {
        return credentialsPath.resolveSibling(credentialsPath.getFileName() + ".lock");
    }

    /**
     * Read the SDK credentials file in the home directory alone, with no environment
     * variables, migration or discovery: what the file holds now, which another process
     * sharing it may have changed since this one loaded it.
     *
     * @return the credentials, or an empty map when the file is missing, unreadable or does
     *         not hold SDK credentials
     */
    public static Map<String, String> readSdkCredentialsFile() {
        return readSdkCredentialsFile(getSdkCredentialsPath());
    }

    /**
     * Read one SDK credentials file alone, with no environment variables, migration or
     * discovery.
     *
     * @param path the SDK credentials file
     * @return the credentials, or an empty map when the file is missing, unreadable or does
     *         not hold SDK credentials
     */
    public static Map<String, String> readSdkCredentialsFile(Path path) {
        Map<String, String> credentials = loadFromFile(path);
        if (detectCredentialType(credentials) != CredentialType.SDK_OAUTH) {
            return Collections.emptyMap();
        }
        return credentials;
    }

    /**
     * Run an action while holding the cross-process lock on the SDK credentials file in the
     * home directory, waiting up to {@link #SDK_CREDENTIALS_LOCK_TIMEOUT_MILLIS} for another
     * holder.
     *
     * @param action what to run under the lock
     * @param <T>    what the action returns
     * @param <E>    the checked exception the action may throw
     * @return the action's result
     * @throws E                   when the action throws it
     * @throws CredentialException when the lock is not acquired in time; the action is not run
     * @see #withSdkCredentialsLock(Path, Duration, LockedAction)
     */
    public static <T, E extends Exception> T withSdkCredentialsLock(LockedAction<T, E> action) throws E {
        return withSdkCredentialsLock(getSdkCredentialsPath(), action);
    }

    /**
     * Run an action while holding the cross-process lock on an SDK credentials file, waiting
     * up to {@link #SDK_CREDENTIALS_LOCK_TIMEOUT_MILLIS} for another holder.
     *
     * @param credentialsPath the SDK credentials file
     * @param action          what to run under the lock
     * @param <T>             what the action returns
     * @param <E>             the checked exception the action may throw
     * @return the action's result
     * @throws E                   when the action throws it
     * @throws CredentialException when the lock is not acquired in time; the action is not run
     * @see #withSdkCredentialsLock(Path, Duration, LockedAction)
     */
    public static <T, E extends Exception> T withSdkCredentialsLock(
            Path credentialsPath, LockedAction<T, E> action) throws E {
        return withSdkCredentialsLock(credentialsPath, Duration.ofMillis(defaultLockTimeoutMillis), action);
    }

    /**
     * Run an action while holding the cross-process lock on an SDK credentials file.
     *
     * <p>The server rotates a refresh token when it is presented and refuses it after that,
     * so processes sharing a credentials file must not both present the token it holds: the
     * one that loses the race presents a token the other already rotated. Reading the file,
     * refreshing and saving under this lock makes one process do it while the others wait,
     * and a waiting process that reads the file once it holds the lock finds the new token.</p>
     *
     * <p>The lock is {@link #getSdkCredentialsLockPath(Path) a file beside the credentials
     * file}, locked with {@link FileChannel#tryLock()}, so it keeps processes of this SDK
     * apart; within one process it also keeps threads apart. A thread already holding it takes it
     * again without waiting, so a save made by the action does not wait on the action.</p>
     *
     * <p>On Linux it does not keep a process of this SDK apart from a process of the Python
     * SDK. That SDK locks the same file with {@code flock}, which Linux holds separately from
     * the record lock {@code tryLock()} takes, so each takes its lock while the other holds
     * its own. On macOS the two locks exclude each other.</p>
     *
     * @param credentialsPath the SDK credentials file
     * @param timeout         how long to wait for another holder
     * @param action          what to run under the lock
     * @param <T>             what the action returns
     * @param <E>             the checked exception the action may throw
     * @return the action's result
     * @throws E                   when the action throws it
     * @throws CredentialException when the lock is not acquired within {@code timeout}, or
     *                             the lock file cannot be opened; the action is not run
     */
    public static <T, E extends Exception> T withSdkCredentialsLock(
            Path credentialsPath, Duration timeout, LockedAction<T, E> action) throws E {
        Path lockPath = getSdkCredentialsLockPath(credentialsPath.toAbsolutePath().normalize());
        ReentrantLock threadLock = THREAD_LOCKS.computeIfAbsent(lockPath, p -> new ReentrantLock());
        long deadline = System.nanoTime() + timeout.toNanos();
        try {
            if (!threadLock.tryLock(timeout.toNanos(), TimeUnit.NANOSECONDS)) {
                throw lockNotAcquired(lockPath, timeout);
            }
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new CredentialException("Interrupted while waiting for " + lockPath, e);
        }
        try {
            if (threadLock.getHoldCount() == 1) {
                FILE_LOCKS.put(lockPath, lockFile(lockPath, deadline, timeout));
            }
            return action.run();
        } finally {
            if (threadLock.getHoldCount() == 1) {
                FileLock fileLock = FILE_LOCKS.remove(lockPath);
                if (fileLock != null) {
                    closeQuietly(fileLock.channel());
                }
            }
            threadLock.unlock();
        }
    }

    private static FileLock lockFile(Path lockPath, long deadline, Duration timeout) {
        FileChannel channel;
        try {
            Files.createDirectories(lockPath.getParent());
            channel = isPosix(lockPath)
                    ? FileChannel.open(lockPath, Set.of(StandardOpenOption.CREATE, StandardOpenOption.WRITE),
                            PosixFilePermissions.asFileAttribute(OWNER_READ_WRITE))
                    : FileChannel.open(lockPath, StandardOpenOption.CREATE, StandardOpenOption.WRITE);
        } catch (IOException e) {
            throw new CredentialException("Could not open " + lockPath + ": " + e.getMessage(), e);
        }
        boolean locked = false;
        try {
            while (true) {
                FileLock lock;
                try {
                    lock = channel.tryLock();
                } catch (OverlappingFileLockException e) {
                    // Held in this JVM under another spelling of the same path.
                    lock = null;
                }
                if (lock != null) {
                    locked = true;
                    return lock;
                }
                if (System.nanoTime() - deadline >= 0) {
                    throw lockNotAcquired(lockPath, timeout);
                }
                Thread.sleep(LOCK_POLL_MILLIS);
            }
        } catch (IOException e) {
            throw new CredentialException("Could not lock " + lockPath + ": " + e.getMessage(), e);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new CredentialException("Interrupted while waiting for " + lockPath, e);
        } finally {
            if (!locked) {
                closeQuietly(channel);
            }
        }
    }

    private static CredentialException lockNotAcquired(Path lockPath, Duration timeout) {
        long millis = timeout.toMillis();
        String wait = millis % 1000 == 0 ? (millis / 1000) + " seconds" : millis + " ms";
        return new CredentialException("Another process or thread held " + lockPath + " for more than " + wait);
    }

    private static void closeQuietly(FileChannel channel) {
        try {
            channel.close();
        } catch (IOException e) {
            logger.debug("Could not close {}: {}", channel, e.getMessage());
        }
    }

    private static boolean isPosix(Path path) {
        return path.getFileSystem().supportedFileAttributeViews().contains("posix");
    }

    /**
     * Save SDK credentials to file with schema versioning.
     *
     * <p>The save holds the {@link #withSdkCredentialsLock(Path, Duration, LockedAction)
     * credentials lock}, waiting up to {@link #SDK_CREDENTIALS_LOCK_TIMEOUT_MILLIS} for
     * another holder, and writes nothing when it is not acquired. The credentials are
     * written to a temporary file in the same directory, readable and writable by the owner
     * only, which is then moved over the file, so a process reading the file sees the old
     * credentials or the new ones and never a partly written file.</p>
     *
     * @param credentials Map of credentials to save
     * @param path        Path to save to (default: ~/.aim/sdk_credentials.json)
     * @throws CredentialException when the file cannot be written or the lock is not acquired
     */
    public static void saveSdkCredentials(Map<String, String> credentials, Path path) {
        Path target = path != null ? path : getSdkCredentialsPath();

        try {
            // Ensure directory exists
            Files.createDirectories(target.toAbsolutePath().getParent());

            // Add schema info for future evolution
            Map<String, Object> toSave = new HashMap<>(credentials);
            toSave.put("schemaVersion", SCHEMA_VERSION);
            toSave.put("type", CredentialType.SDK_OAUTH.getValue());

            Object sharedToken = toSave.remove(SHARED_ACCESS_TOKEN);
            Object sharedIssuedWith = toSave.remove(SHARED_ACCESS_TOKEN_REFRESH_TOKEN_SHA256);
            if (sharedToken != null && sharedIssuedWith != null) {
                Map<String, Object> shared = new LinkedHashMap<>();
                shared.put("token", sharedToken);
                shared.put("refreshTokenSha256", sharedIssuedWith);
                toSave.put(SHARED_ACCESS_TOKEN, shared);
            }

            byte[] json = objectMapper.writerWithDefaultPrettyPrinter().writeValueAsBytes(toSave);
            withSdkCredentialsLock(target, Duration.ofMillis(defaultLockTimeoutMillis), () -> {
                replaceFile(target, json);
                return null;
            });

            logger.info("Saved SDK credentials to {}", target);
        } catch (IOException e) {
            throw new CredentialException("Failed to save credentials: " + e.getMessage(), e);
        }
    }

    /**
     * Replace {@code target} with {@code content} in one step. The temporary file is made
     * readable and writable by the owner only before it is moved into place: created so
     * where the file system has POSIX permissions, set so otherwise.
     */
    private static void replaceFile(Path target, byte[] content) throws IOException {
        Path dir = target.toAbsolutePath().getParent();
        String prefix = "." + target.getFileName() + ".";
        boolean posix = isPosix(dir);
        Path tmp = posix
                ? Files.createTempFile(dir, prefix, ".tmp", PosixFilePermissions.asFileAttribute(OWNER_READ_WRITE))
                : Files.createTempFile(dir, prefix, ".tmp");
        try {
            try (FileChannel out = FileChannel.open(tmp, StandardOpenOption.WRITE)) {
                ByteBuffer buffer = ByteBuffer.wrap(content);
                while (buffer.hasRemaining()) {
                    out.write(buffer);
                }
                out.force(true);
            }
            if (!posix) {
                // Owner read/write only
                File file = tmp.toFile();
                file.setReadable(false, false);
                file.setWritable(false, false);
                file.setReadable(true, true);
                file.setWritable(true, true);
            }
            try {
                Files.move(tmp, target, StandardCopyOption.ATOMIC_MOVE, StandardCopyOption.REPLACE_EXISTING);
            } catch (AtomicMoveNotSupportedException e) {
                Files.move(tmp, target, StandardCopyOption.REPLACE_EXISTING);
            } catch (AccessDeniedException e) {
                if (!System.getProperty("os.name", "").startsWith("Windows")) {
                    throw e;
                }
                // Windows refuses to replace a file another process has open without
                // delete sharing; write it in place, as the Python SDK does there.
                Files.write(target, content);
                Files.deleteIfExists(tmp);
            }
        } catch (IOException | RuntimeException e) {
            try {
                Files.deleteIfExists(tmp);
            } catch (IOException cleanup) {
                e.addSuppressed(cleanup);
            }
            throw e;
        }
    }

    // =========================================================================
    // AGENT CREDENTIALS (Ed25519 keys for registered agents)
    // =========================================================================

    /** Maximum length for agent names to avoid filesystem path limits */
    private static final int MAX_AGENT_NAME_LENGTH = 200;

    /**
     * Get the path to an agent's credentials file.
     *
     * @param agentName Name of the agent
     * @return Path to the agent credentials file
     * @throws IllegalArgumentException if agent name is null, empty, or exceeds maximum length
     */
    public static Path getAgentCredentialsPath(String agentName) {
        if (agentName == null || agentName.trim().isEmpty()) {
            throw new IllegalArgumentException("Agent name cannot be null or empty");
        }
        if (agentName.length() > MAX_AGENT_NAME_LENGTH) {
            throw new IllegalArgumentException(
                "Agent name exceeds maximum length of " + MAX_AGENT_NAME_LENGTH + " characters");
        }
        // Sanitize agent name for filesystem
        String safeName = agentName.replaceAll("[^a-zA-Z0-9\\-_]", "_");
        return Paths.get(System.getProperty("user.home"), AIM_DIR, AGENTS_DIR, safeName + ".json");
    }

    /**
     * Load agent-specific credentials.
     *
     * @param agentName Name of the agent
     * @return Map containing agent credentials
     */
    public static Map<String, String> loadAgentCredentials(String agentName) {
        Path path = getAgentCredentialsPath(agentName);
        Map<String, String> creds = loadFromFile(path);
        if (!creds.isEmpty()) {
            logger.debug("Loaded agent credentials from {}", path);
        }
        return creds;
    }

    /**
     * Save agent-specific credentials with schema versioning.
     *
     * @param agentName   Name of the agent
     * @param credentials Credentials to save
     */
    public static void saveAgentCredentials(String agentName, Map<String, String> credentials) {
        Path path = getAgentCredentialsPath(agentName);
        try {
            // Ensure agents directory exists
            Files.createDirectories(path.getParent());

            // Add schema info
            Map<String, String> toSave = new HashMap<>(credentials);
            toSave.put("schemaVersion", SCHEMA_VERSION);
            toSave.put("type", CredentialType.AGENT.getValue());
            toSave.put("name", agentName);

            // Write credentials
            String json = objectMapper.writerWithDefaultPrettyPrinter().writeValueAsString(toSave);
            Files.writeString(path, json);

            // Set file permissions (600 - owner read/write only)
            File file = path.toFile();
            file.setReadable(false, false);
            file.setWritable(false, false);
            file.setReadable(true, true);
            file.setWritable(true, true);

            logger.info("Saved agent credentials to {}", path);
        } catch (IOException e) {
            throw new CredentialException("Failed to save agent credentials: " + e.getMessage(), e);
        }
    }

    /**
     * Delete agent credentials.
     *
     * @param agentName Name of the agent
     */
    public static void deleteAgentCredentials(String agentName) {
        Path path = getAgentCredentialsPath(agentName);
        try {
            Files.deleteIfExists(path);
            logger.info("Deleted credentials for agent: {}", agentName);
        } catch (IOException e) {
            throw new CredentialException("Failed to delete credentials: " + e.getMessage(), e);
        }
    }

    /**
     * List all agents with saved credentials.
     *
     * @return List of agent names with credentials
     */
    public static java.util.List<String> listAgentCredentials() {
        java.util.List<String> agents = new java.util.ArrayList<>();
        Path agentsDir = Paths.get(System.getProperty("user.home"), AIM_DIR, AGENTS_DIR);

        if (Files.exists(agentsDir) && Files.isDirectory(agentsDir)) {
            try {
                Files.list(agentsDir)
                    .filter(p -> p.toString().endsWith(".json"))
                    .forEach(p -> {
                        String name = p.getFileName().toString();
                        agents.add(name.substring(0, name.length() - 5)); // Remove .json
                    });
            } catch (IOException e) {
                logger.warn("Failed to list agent credentials: {}", e.getMessage());
            }
        }

        return agents;
    }

    // =========================================================================
    // ERROR MESSAGES
    // =========================================================================

    /**
     * Print a clear error message when SDK credentials are not found.
     *
     * @param aimUrl AIM server URL for instructions
     */
    public static void printSdkCredentialsNotFoundError(String aimUrl) {
        String border = "=".repeat(72);
        System.out.println();
        System.out.println(border);
        System.out.println("SDK CREDENTIALS NOT FOUND");
        System.out.println(border);
        System.out.println();
        System.out.println("No SDK authentication credentials found.");
        System.out.println();
        System.out.println("The SDK needs OAuth credentials to authenticate with the AIM server.");
        System.out.println("These are different from agent credentials (which store agent keys).");
        System.out.println();
        System.out.println("TO FIX:");
        System.out.println("  1. Visit your AIM dashboard: " + aimUrl);
        System.out.println("  2. Go to Settings -> SDK Downloads");
        System.out.println("  3. Download the Java SDK");
        System.out.println("  4. Extract and add to your project");
        System.out.println();
        System.out.println("Your downloaded SDK includes authentication credentials.");
        System.out.println(border);
        System.out.println();
    }

    /**
     * Print a clear error message when wrong credential type is found.
     *
     * @param foundType Type of credentials found
     * @param aimUrl    AIM server URL for instructions
     */
    public static void printWrongCredentialTypeError(CredentialType foundType, String aimUrl) {
        String border = "=".repeat(72);
        System.out.println();
        System.out.println(border);
        System.out.println("WRONG CREDENTIAL TYPE");
        System.out.println(border);
        System.out.println();

        if (foundType == CredentialType.AGENT) {
            System.out.println("Found agent credentials but SDK credentials are needed.");
            System.out.println();
            System.out.println("Agent credentials (Ed25519 keys for registered agents) and");
            System.out.println("SDK credentials (OAuth tokens for SDK authentication) are different.");
        } else {
            System.out.println("Found unrecognized credential format.");
        }

        System.out.println();
        System.out.println("TO FIX:");
        System.out.println("  1. Download a fresh SDK from: " + aimUrl);
        System.out.println("  2. Go to Settings -> SDK Downloads");
        System.out.println("  3. Extract and add to your project");
        System.out.println();
        System.out.println("Your existing agent credentials are safe and unchanged.");
        System.out.println(border);
        System.out.println();
    }
}
