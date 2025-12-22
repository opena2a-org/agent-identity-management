package org.opena2a.aim.credentials;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.opena2a.aim.exceptions.CredentialException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.File;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.Collections;
import java.util.HashMap;
import java.util.Map;

/**
 * Manages AIM SDK credentials.
 *
 * <p>Credentials are loaded from:</p>
 * <ol>
 *   <li>Environment variables (AIM_URL, AIM_CLIENT_ID, AIM_CLIENT_SECRET, AIM_ORG_ID)</li>
 *   <li>SDK credentials file (~/.aim/sdk_credentials.json)</li>
 *   <li>Local .aim/credentials.json file</li>
 * </ol>
 */
public class CredentialManager {

    private static final Logger logger = LoggerFactory.getLogger(CredentialManager.class);
    private static final ObjectMapper objectMapper = new ObjectMapper();

    private static final String AIM_DIR = ".aim";
    private static final String SDK_CREDENTIALS_FILE = "sdk_credentials.json";
    private static final String CREDENTIALS_FILE = "credentials.json";

    /**
     * Load SDK credentials from environment or file.
     *
     * @return Map containing credentials (aimUrl, clientId, clientSecret, organizationId)
     */
    public static Map<String, String> loadSdkCredentials() {
        // First, try environment variables
        Map<String, String> envCreds = loadFromEnvironment();
        if (!envCreds.isEmpty() && envCreds.containsKey("clientId")) {
            logger.debug("Loaded credentials from environment variables");
            return envCreds;
        }

        // Try home directory SDK credentials
        Path homeSdkPath = Paths.get(System.getProperty("user.home"), AIM_DIR, SDK_CREDENTIALS_FILE);
        Map<String, String> homeCreds = loadFromFile(homeSdkPath);
        if (!homeCreds.isEmpty()) {
            logger.debug("Loaded credentials from {}", homeSdkPath);
            return homeCreds;
        }

        // Try local .aim directory
        Path localPath = Paths.get(AIM_DIR, CREDENTIALS_FILE);
        Map<String, String> localCreds = loadFromFile(localPath);
        if (!localCreds.isEmpty()) {
            logger.debug("Loaded credentials from {}", localPath);
            return localCreds;
        }

        // Try current directory
        Path currentPath = Paths.get(SDK_CREDENTIALS_FILE);
        Map<String, String> currentCreds = loadFromFile(currentPath);
        if (!currentCreds.isEmpty()) {
            logger.debug("Loaded credentials from {}", currentPath);
            return currentCreds;
        }

        logger.warn("No SDK credentials found");
        return Collections.emptyMap();
    }

    /**
     * Load credentials from environment variables.
     */
    private static Map<String, String> loadFromEnvironment() {
        Map<String, String> creds = new HashMap<>();

        String aimUrl = System.getenv("AIM_URL");
        String clientId = System.getenv("AIM_CLIENT_ID");
        String clientSecret = System.getenv("AIM_CLIENT_SECRET");
        String orgId = System.getenv("AIM_ORG_ID");

        if (aimUrl != null) creds.put("aimUrl", aimUrl);
        if (clientId != null) creds.put("clientId", clientId);
        if (clientSecret != null) creds.put("clientSecret", clientSecret);
        if (orgId != null) creds.put("organizationId", orgId);

        return creds;
    }

    /**
     * Load credentials from a JSON file.
     */
    private static Map<String, String> loadFromFile(Path path) {
        if (!Files.exists(path)) {
            return Collections.emptyMap();
        }

        try {
            String content = Files.readString(path);
            JsonNode json = objectMapper.readTree(content);

            Map<String, String> creds = new HashMap<>();

            if (json.has("aimUrl")) creds.put("aimUrl", json.get("aimUrl").asText());
            if (json.has("aim_url")) creds.put("aimUrl", json.get("aim_url").asText());

            if (json.has("clientId")) creds.put("clientId", json.get("clientId").asText());
            if (json.has("client_id")) creds.put("clientId", json.get("client_id").asText());

            if (json.has("clientSecret")) creds.put("clientSecret", json.get("clientSecret").asText());
            if (json.has("client_secret")) creds.put("clientSecret", json.get("client_secret").asText());

            if (json.has("organizationId")) creds.put("organizationId", json.get("organizationId").asText());
            if (json.has("organization_id")) creds.put("organizationId", json.get("organization_id").asText());
            if (json.has("orgId")) creds.put("organizationId", json.get("orgId").asText());

            return creds;
        } catch (IOException e) {
            logger.warn("Failed to read credentials from {}: {}", path, e.getMessage());
            return Collections.emptyMap();
        }
    }

    /**
     * Save SDK credentials to file.
     *
     * @param credentials Map of credentials to save
     * @param path        Path to save to (default: ~/.aim/sdk_credentials.json)
     */
    public static void saveSdkCredentials(Map<String, String> credentials, Path path) {
        if (path == null) {
            path = Paths.get(System.getProperty("user.home"), AIM_DIR, SDK_CREDENTIALS_FILE);
        }

        try {
            // Ensure directory exists
            Files.createDirectories(path.getParent());

            // Write credentials
            String json = objectMapper.writerWithDefaultPrettyPrinter().writeValueAsString(credentials);
            Files.writeString(path, json);

            // Set file permissions (600 - owner read/write only)
            File file = path.toFile();
            file.setReadable(false, false);
            file.setWritable(false, false);
            file.setReadable(true, true);
            file.setWritable(true, true);

            logger.info("Saved credentials to {}", path);
        } catch (IOException e) {
            throw new CredentialException("Failed to save credentials: " + e.getMessage(), e);
        }
    }

    /**
     * Load agent-specific credentials.
     *
     * @param agentName Name of the agent
     * @return Map containing agent credentials
     */
    public static Map<String, String> loadAgentCredentials(String agentName) {
        Path path = Paths.get(System.getProperty("user.home"), AIM_DIR, "agents", agentName + ".json");
        return loadFromFile(path);
    }

    /**
     * Save agent-specific credentials.
     *
     * @param agentName   Name of the agent
     * @param credentials Credentials to save
     */
    public static void saveAgentCredentials(String agentName, Map<String, String> credentials) {
        Path path = Paths.get(System.getProperty("user.home"), AIM_DIR, "agents", agentName + ".json");
        saveSdkCredentials(credentials, path);
    }

    /**
     * Delete agent credentials.
     *
     * @param agentName Name of the agent
     */
    public static void deleteAgentCredentials(String agentName) {
        Path path = Paths.get(System.getProperty("user.home"), AIM_DIR, "agents", agentName + ".json");
        try {
            Files.deleteIfExists(path);
            logger.info("Deleted credentials for agent: {}", agentName);
        } catch (IOException e) {
            throw new CredentialException("Failed to delete credentials: " + e.getMessage(), e);
        }
    }
}
