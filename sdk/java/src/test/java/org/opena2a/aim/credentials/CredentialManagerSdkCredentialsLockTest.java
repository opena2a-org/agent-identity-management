package org.opena2a.aim.credentials;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.*;
import org.junit.jupiter.api.io.TempDir;
import org.opena2a.aim.exceptions.CredentialException;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.nio.channels.FileChannel;
import java.nio.channels.FileLock;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardOpenOption;
import java.nio.file.attribute.PosixFilePermission;
import java.time.Duration;
import java.util.ArrayList;
import java.util.EnumSet;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Processes sharing the SDK credentials file serialize on {@code sdk_credentials.json.lock}
 * beside it, and a save replaces the file in one step.
 */
class CredentialManagerSdkCredentialsLockTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    @TempDir
    Path tempDir;

    private Path credentialsPath() {
        return tempDir.resolve(".aim").resolve("sdk_credentials.json");
    }

    private static Map<String, String> credentialsWith(String refreshToken) {
        Map<String, String> credentials = new HashMap<>();
        credentials.put("aimUrl", "http://localhost:8080");
        credentials.put("refreshToken", refreshToken);
        return credentials;
    }

    private static String storedRefreshToken(Path path) throws IOException {
        return MAPPER.readTree(Files.readString(path)).get("refreshToken").asText();
    }

    @Test
    @DisplayName("a save waits while another process holds sdk_credentials.json.lock, then writes")
    void aSaveWaitsWhileAnotherProcessHoldsTheLock() throws Exception {
        Path path = credentialsPath();
        Files.createDirectories(path.getParent());
        CredentialManager.saveSdkCredentials(credentialsWith("not-a-real-token-before"), path);

        try (LockHolderProcess holder = LockHolderProcess.start(path.resolveSibling("sdk_credentials.json.lock"))) {
            AtomicReference<Throwable> failure = new AtomicReference<>();
            Thread saver = new Thread(() -> {
                try {
                    CredentialManager.saveSdkCredentials(credentialsWith("not-a-real-token-after"), path);
                } catch (Throwable t) {
                    failure.set(t);
                }
            });
            saver.start();
            saver.join(800);

            assertTrue(saver.isAlive(), "the save returned while another process held the lock");
            assertEquals("not-a-real-token-before", storedRefreshToken(path),
                    "the file changed while another process held the lock");

            holder.release();
            saver.join(10_000);
            assertFalse(saver.isAlive(), "the save did not finish after the other process released the lock");
            assertNull(failure.get(), () -> "the save failed: " + failure.get());
        }
        assertEquals("not-a-real-token-after", storedRefreshToken(path));
    }

    @Test
    @DisplayName("a reader in another thread never sees a partly written file")
    void aReaderNeverSeesAPartlyWrittenFile() throws Exception {
        Path path = credentialsPath();
        Files.createDirectories(path.getParent());
        String padding = "x".repeat(64 * 1024);
        Map<String, String> credentials = credentialsWith("not-a-real-token-0");
        credentials.put("email", padding);
        CredentialManager.saveSdkCredentials(credentials, path);

        AtomicBoolean stop = new AtomicBoolean(false);
        AtomicInteger reads = new AtomicInteger();
        AtomicReference<String> torn = new AtomicReference<>();
        Thread reader = new Thread(() -> {
            while (!stop.get() && torn.get() == null) {
                try {
                    String text = Files.readString(path);
                    JsonNode json = MAPPER.readTree(text);
                    if (json == null || !json.has("refreshToken") || json.get("email").asText().length() != padding.length()) {
                        torn.set("read " + text.length() + " characters without a complete credential");
                    }
                    reads.incrementAndGet();
                } catch (IOException e) {
                    torn.set(e.getClass().getSimpleName() + ": " + e.getMessage());
                }
            }
        });
        reader.start();
        for (int i = 1; i <= 300 && torn.get() == null; i++) {
            credentials.put("refreshToken", "not-a-real-token-" + i);
            CredentialManager.saveSdkCredentials(credentials, path);
        }
        stop.set(true);
        reader.join(10_000);

        assertNull(torn.get(), () -> "after " + reads.get() + " complete reads: " + torn.get());
        assertTrue(reads.get() > 0, "the reader read nothing");
        try (var files = Files.list(path.getParent())) {
            List<String> names = files.map(p -> p.getFileName().toString()).sorted().toList();
            assertTrue(names.stream().noneMatch(n -> n.endsWith(".tmp")), () -> "temporary files were left: " + names);
        }
    }

    @Test
    @DisplayName("a save writes nothing when the lock is not acquired in time")
    void aSaveWritesNothingWhenTheLockIsNotAcquiredInTime() throws Exception {
        Path path = credentialsPath();
        Files.createDirectories(path.getParent());
        CredentialManager.saveSdkCredentials(credentialsWith("not-a-real-token-before"), path);

        long saved = CredentialManager.defaultLockTimeoutMillis;
        CredentialManager.defaultLockTimeoutMillis = 300;
        try (LockHolderProcess holder = LockHolderProcess.start(CredentialManager.getSdkCredentialsLockPath(path))) {
            CredentialException refused = assertThrows(CredentialException.class,
                    () -> CredentialManager.saveSdkCredentials(credentialsWith("not-a-real-token-after"), path));
            assertTrue(refused.getMessage().contains("sdk_credentials.json.lock for more than 300 ms"),
                    refused.getMessage());
        } finally {
            CredentialManager.defaultLockTimeoutMillis = saved;
        }
        assertEquals("not-a-real-token-before", storedRefreshToken(path));
    }

    @Test
    @DisplayName("a locked action is not run when another process holds the lock past the timeout")
    void aLockedActionIsNotRunWhileAnotherProcessHoldsTheLock() throws Exception {
        Path path = credentialsPath();
        AtomicBoolean ran = new AtomicBoolean(false);
        try (LockHolderProcess holder = LockHolderProcess.start(CredentialManager.getSdkCredentialsLockPath(path))) {
            assertThrows(CredentialException.class, () -> CredentialManager.withSdkCredentialsLock(
                    path, Duration.ofMillis(300), () -> ran.getAndSet(true)));
            assertFalse(ran.get(), "the action ran while another process held the lock");

            holder.release();
            assertEquals("ran", CredentialManager.withSdkCredentialsLock(
                    path, Duration.ofSeconds(10), () -> "ran"));
        }
    }

    @Test
    @DisplayName("two processes rotating the stored token under the lock each read the other's token, never a stale one")
    void twoProcessesRotatingUnderTheLockNeverPresentAStaleToken() throws Exception {
        Path path = credentialsPath();
        Files.createDirectories(path.getParent());
        CredentialManager.saveSdkCredentials(credentialsWith("not-a-real-token-0"), path);

        int perProcess = 25;
        List<String> presented = RotateUnderLock.runTogether(path, perProcess, 2);

        assertEquals(2 * perProcess, presented.size(), () -> "tokens presented: " + presented);
        assertEquals(presented.size(), new HashSet<>(presented).size(),
                () -> "a token was presented twice: " + presented);
        assertEquals("not-a-real-token-" + (2 * perProcess), storedRefreshToken(path));
    }

    @Test
    @DisplayName("threads of one process wait for each other")
    void threadsOfOneProcessWaitForEachOther() throws Exception {
        Path path = credentialsPath();
        CountDownLatch held = new CountDownLatch(1);
        CountDownLatch release = new CountDownLatch(1);
        Thread holder = new Thread(() -> {
            try {
                CredentialManager.withSdkCredentialsLock(path, Duration.ofSeconds(10), () -> {
                    held.countDown();
                    return release.await(10, TimeUnit.SECONDS);
                });
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        });
        holder.start();
        assertTrue(held.await(10, TimeUnit.SECONDS));

        AtomicBoolean ran = new AtomicBoolean(false);
        assertThrows(CredentialException.class, () -> CredentialManager.withSdkCredentialsLock(
                path, Duration.ofMillis(200), () -> ran.getAndSet(true)));
        assertFalse(ran.get(), "a second thread ran while the first held the lock");

        release.countDown();
        holder.join(10_000);
        assertEquals("ran", CredentialManager.withSdkCredentialsLock(path, Duration.ofSeconds(10), () -> "ran"));
    }

    @Test
    @DisplayName("a save made while the lock is held by the same thread does not wait on itself")
    void aSaveUnderTheLockDoesNotWaitOnItself() throws Exception {
        Path path = credentialsPath();
        long saved = CredentialManager.defaultLockTimeoutMillis;
        CredentialManager.defaultLockTimeoutMillis = 200;
        try {
            CredentialManager.withSdkCredentialsLock(path, Duration.ofSeconds(10), () -> {
                CredentialManager.saveSdkCredentials(credentialsWith("not-a-real-token-inner"), path);
                return null;
            });
        } finally {
            CredentialManager.defaultLockTimeoutMillis = saved;
        }
        assertEquals("not-a-real-token-inner", storedRefreshToken(path));
    }

    @Test
    @DisplayName("readSdkCredentialsFile reads the file as it is now, and nothing else")
    void readSdkCredentialsFileReadsTheFileAsItIsNow() throws Exception {
        Path path = credentialsPath();
        assertTrue(CredentialManager.readSdkCredentialsFile(path).isEmpty(), "a missing file read as credentials");

        CredentialManager.saveSdkCredentials(credentialsWith("not-a-real-token-1"), path);
        assertEquals("not-a-real-token-1", CredentialManager.readSdkCredentialsFile(path).get("refreshToken"));
        CredentialManager.saveSdkCredentials(credentialsWith("not-a-real-token-2"), path);
        assertEquals("not-a-real-token-2", CredentialManager.readSdkCredentialsFile(path).get("refreshToken"));

        Files.writeString(path, "{\"agentId\": \"agent-1\", \"privateKey\": \"not-a-real-key\"}");
        assertTrue(CredentialManager.readSdkCredentialsFile(path).isEmpty(), "agent credentials read as SDK credentials");

        Files.writeString(path, "{\"refreshToken\": ");
        assertTrue(CredentialManager.readSdkCredentialsFile(path).isEmpty(), "a partial file read as credentials");
    }

    @Test
    @DisplayName("the shared access token is read and saved as the object the Python SDK writes")
    void theSharedAccessTokenRoundTripsInThePythonSdkFormat() throws Exception {
        Path path = credentialsPath();
        Files.createDirectories(path.getParent());
        Files.writeString(path, "{\"refreshToken\": \"not-a-real-token-1\", \"sharedAccessToken\": "
                + "{\"token\": \"not-a-real-access-token\", \"refreshTokenSha256\": \"not-a-real-digest\"}}");

        Map<String, String> read = CredentialManager.readSdkCredentialsFile(path);
        assertEquals("not-a-real-access-token", read.get(CredentialManager.SHARED_ACCESS_TOKEN));
        assertEquals("not-a-real-digest", read.get(CredentialManager.SHARED_ACCESS_TOKEN_REFRESH_TOKEN_SHA256));

        CredentialManager.saveSdkCredentials(read, path);
        JsonNode saved = MAPPER.readTree(Files.readString(path));
        assertEquals("not-a-real-access-token", saved.path("sharedAccessToken").path("token").asText());
        assertEquals("not-a-real-digest", saved.path("sharedAccessToken").path("refreshTokenSha256").asText());
        assertFalse(saved.has(CredentialManager.SHARED_ACCESS_TOKEN_REFRESH_TOKEN_SHA256),
                () -> "the digest was saved beside the object: " + saved);

        read.remove(CredentialManager.SHARED_ACCESS_TOKEN);
        read.remove(CredentialManager.SHARED_ACCESS_TOKEN_REFRESH_TOKEN_SHA256);
        CredentialManager.saveSdkCredentials(read, path);
        assertFalse(MAPPER.readTree(Files.readString(path)).has("sharedAccessToken"));
    }

    @Test
    @DisplayName("the credentials file and its lock file are readable and writable by the owner only")
    void theCredentialsFileAndItsLockFileAreOwnerOnly() throws Exception {
        Assumptions.assumeTrue(tempDir.getFileSystem().supportedFileAttributeViews().contains("posix"));
        Path path = credentialsPath();
        CredentialManager.saveSdkCredentials(credentialsWith("not-a-real-token-1"), path);

        Set<PosixFilePermission> ownerOnly = EnumSet.of(PosixFilePermission.OWNER_READ, PosixFilePermission.OWNER_WRITE);
        assertEquals(ownerOnly, Files.getPosixFilePermissions(path));
        assertEquals(ownerOnly, Files.getPosixFilePermissions(CredentialManager.getSdkCredentialsLockPath(path)));
    }

    private static List<String> childCommand(Class<?> main, String... args) {
        List<String> command = new ArrayList<>(List.of(
                Path.of(System.getProperty("java.home"), "bin", "java").toString(),
                "-Dorg.slf4j.simpleLogger.defaultLogLevel=warn",
                "-cp", System.getProperty("java.class.path"),
                main.getName()));
        command.addAll(List.of(args));
        return command;
    }

    /**
     * A second JVM that holds an exclusive lock on a file until it is told to release it, the way
     * another SDK process holds the credentials lock while it refreshes.
     */
    static final class LockHolderProcess implements AutoCloseable {
        private final Process process;
        private final OutputStream stdin;

        private LockHolderProcess(Process process) {
            this.process = process;
            this.stdin = process.getOutputStream();
        }

        static LockHolderProcess start(Path lockPath) throws IOException {
            Files.createDirectories(lockPath.getParent());
            Process process = new ProcessBuilder(childCommand(HoldLock.class, lockPath.toString()))
                    .redirectError(ProcessBuilder.Redirect.INHERIT)
                    .start();
            LockHolderProcess holder = new LockHolderProcess(process);
            BufferedReader out = new BufferedReader(new InputStreamReader(process.getInputStream(), StandardCharsets.UTF_8));
            String line = out.readLine();
            if (!"locked".equals(line)) {
                holder.close();
                throw new IOException("the lock holder process answered " + line + " instead of locked");
            }
            return holder;
        }

        void release() throws IOException {
            stdin.write("release\n".getBytes(StandardCharsets.UTF_8));
            stdin.flush();
        }

        @Override
        public void close() throws IOException {
            try {
                stdin.close();
                if (!process.waitFor(10, TimeUnit.SECONDS)) {
                    process.destroyForcibly();
                }
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                process.destroyForcibly();
            }
        }
    }

    /** Child process entry point: lock args[0], print "locked", hold until a line or end of input. */
    static final class HoldLock {
        public static void main(String[] args) throws IOException {
            try (FileChannel channel = FileChannel.open(Path.of(args[0]),
                    StandardOpenOption.CREATE, StandardOpenOption.WRITE);
                 FileLock ignored = channel.lock()) {
                System.out.println("locked");
                System.out.flush();
                new BufferedReader(new InputStreamReader(System.in, StandardCharsets.UTF_8)).readLine();
            }
        }
    }

    /**
     * A JVM that, args[1] times, takes the credentials lock, reads the stored refresh token, prints
     * it as the token it presents, and saves the next one, the way a refresh rotates it.
     */
    static final class RotateUnderLock {
        /**
         * Start {@code processes} JVMs, wait until each is ready, start them together so their
         * rotations overlap, and return every token they presented.
         */
        static List<String> runTogether(Path credentialsPath, int times, int processes) throws Exception {
            List<Process> started = new ArrayList<>();
            List<BufferedReader> outputs = new ArrayList<>();
            try {
                for (int i = 0; i < processes; i++) {
                    Process process = new ProcessBuilder(
                            childCommand(RotateUnderLock.class, credentialsPath.toString(), String.valueOf(times)))
                            .redirectError(ProcessBuilder.Redirect.INHERIT)
                            .start();
                    started.add(process);
                    BufferedReader out = new BufferedReader(
                            new InputStreamReader(process.getInputStream(), StandardCharsets.UTF_8));
                    assertEquals("ready", out.readLine(), "a rotating process did not start");
                    outputs.add(out);
                }
                for (Process process : started) {
                    process.getOutputStream().write("go\n".getBytes(StandardCharsets.UTF_8));
                    process.getOutputStream().flush();
                }
                List<String> presented = new ArrayList<>();
                for (int i = 0; i < processes; i++) {
                    presented.addAll(outputs.get(i).lines().toList());
                    Process process = started.get(i);
                    assertTrue(process.waitFor(60, TimeUnit.SECONDS), "a rotating process did not finish");
                    assertEquals(0, process.exitValue(), "a rotating process failed");
                }
                return presented;
            } finally {
                for (Process process : started) {
                    if (process.isAlive()) {
                        process.destroyForcibly();
                    }
                }
            }
        }

        public static void main(String[] args) throws Exception {
            Path path = Path.of(args[0]);
            int times = Integer.parseInt(args[1]);
            System.out.println("ready");
            System.out.flush();
            new BufferedReader(new InputStreamReader(System.in, StandardCharsets.UTF_8)).readLine();
            for (int i = 0; i < times; i++) {
                CredentialManager.withSdkCredentialsLock(path, Duration.ofSeconds(30), () -> {
                    String token = CredentialManager.readSdkCredentialsFile(path).get("refreshToken");
                    System.out.println(token);
                    System.out.flush();
                    Thread.sleep(5);
                    int next = Integer.parseInt(token.substring("not-a-real-token-".length())) + 1;
                    CredentialManager.saveSdkCredentials(credentialsWith("not-a-real-token-" + next), path);
                    return null;
                });
                // Outside the lock, so the other process gets it between rotations.
                Thread.sleep(20);
            }
        }
    }
}
