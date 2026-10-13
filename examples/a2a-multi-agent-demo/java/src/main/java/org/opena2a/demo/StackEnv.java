package org.opena2a.demo;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.List;

/**
 * How the A2A demo's Java program reaches the local AIM stack.
 *
 * Each value comes from the environment first, then from the repository's .env
 * (the file docker compose reads for docker-compose.yml), then from the default
 * the compose files use:
 *
 *     AIM_BACKEND_PORT=8080    the backend's host port
 *     AIM_FRONTEND_PORT=3000   the dashboard's host port
 *
 * The .env is found three levels above the working directory, the java/
 * directory the demo runs from, as the sample data is. A2AMultiAgentDemo takes
 * the backend from AIM_URL first, when it is set.
 */
final class StackEnv {

    private static final Path DOTENV = Paths.get("../../../.env");

    static final String BACKEND_URL = "http://localhost:" + stackValue("AIM_BACKEND_PORT", "8080");
    static final String FRONTEND_URL = "http://localhost:" + stackValue("AIM_FRONTEND_PORT", "3000");

    private StackEnv() {}

    /** name from the environment, then the repository's .env, then defaultValue. */
    static String stackValue(String name, String defaultValue) {
        String value = System.getenv(name);
        if (value == null || value.isEmpty()) value = dotenvValue(name);
        return value.isEmpty() ? defaultValue : value;
    }

    /** The last value the repository's .env assigns to name, or "". */
    private static String dotenvValue(String name) {
        List<String> lines;
        try {
            lines = Files.readAllLines(DOTENV);
        } catch (IOException e) {
            return "";
        }
        String prefix = name + "=";
        String value = "";
        for (String line : lines) {
            if (line.startsWith(prefix)) value = line.substring(prefix.length());
        }
        return value;
    }
}
