package org.opena2a.aim.client;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.fail;

/**
 * The backend's SDK-API route table, read from the backend source.
 *
 * Tests that stand in for the server answer only the routes listed here and
 * 404 every other path, as the backend's router does. A test that chose its
 * own path would agree with the client while the real server refused the call.
 */
final class SdkApiRouteTable {

    /** Repo-relative location of the backend's SDK-API route table. */
    static final String ROUTE_TABLE = "apps/backend/cmd/server/sdk_api_routes.go";

    private static final Pattern STRING_CONSTANT = Pattern.compile("(\\w+)\\s*=\\s*\"([^\"]*)\"");
    private static final Pattern ROUTE_ENTRY =
            Pattern.compile("Method:\\s*http\\.Method(\\w+),\\s*Path:\\s*(?:\"([^\"]+)\"|(\\w+))");

    private final List<Route> routes;

    private SdkApiRouteTable(List<Route> routes) {
        this.routes = routes;
    }

    /**
     * Reads every route of the table: the method, and the full path with its
     * base prefix. A path written as a named constant is resolved from the same
     * file.
     */
    static SdkApiRouteTable read() throws IOException {
        String source = new String(Files.readAllBytes(locate()), StandardCharsets.UTF_8);

        Map<String, String> constants = new HashMap<>();
        Matcher constant = STRING_CONSTANT.matcher(source);
        while (constant.find()) {
            constants.put(constant.group(1), constant.group(2));
        }
        String basePath = constants.get("sdkAPIBasePath");
        assertNotNull(basePath, "sdkAPIBasePath is not declared in " + ROUTE_TABLE);

        List<Route> routes = new ArrayList<>();
        Matcher entry = ROUTE_ENTRY.matcher(source);
        while (entry.find()) {
            String relative = entry.group(2) != null ? entry.group(2) : constants.get(entry.group(3));
            assertNotNull(relative, "route path " + entry.group(3) + " is not a string constant in " + ROUTE_TABLE);
            routes.add(new Route(entry.group(1).toUpperCase(Locale.ROOT), basePath + relative));
        }
        assertFalse(routes.isEmpty(), "no route entries found in " + ROUTE_TABLE);
        return new SdkApiRouteTable(routes);
    }

    boolean isRegistered(String method, String path) {
        return routeFor(method, path) != null;
    }

    /** The registered route pattern a request matches, or null when none does. */
    String routeFor(String method, String path) {
        for (Route route : routes) {
            if (route.method.equals(method) && route.matcher.matcher(path).matches()) {
                return route.pattern;
            }
        }
        return null;
    }

    String format() {
        StringBuilder out = new StringBuilder();
        for (Route route : routes) {
            out.append("  ").append(route.method).append(' ').append(route.pattern).append('\n');
        }
        return out.toString();
    }

    private static Path locate() {
        Path start = Paths.get("").toAbsolutePath();
        for (Path dir = start; dir != null; dir = dir.getParent()) {
            Path candidate = dir.resolve(ROUTE_TABLE);
            if (Files.isRegularFile(candidate)) {
                return candidate;
            }
        }
        return fail(ROUTE_TABLE + " was not found above " + start
                + ". This test reads the backend's route table and has to run inside the repository.");
    }

    /** One registered route; a ":name" path segment matches any single segment. */
    private static final class Route {
        final String method;
        final String pattern;
        final Pattern matcher;

        Route(String method, String pattern) {
            this.method = method;
            this.pattern = pattern;

            StringBuilder regex = new StringBuilder();
            for (String segment : pattern.split("/")) {
                if (segment.isEmpty()) {
                    continue;
                }
                regex.append('/').append(segment.startsWith(":") ? "[^/]+" : Pattern.quote(segment));
            }
            this.matcher = Pattern.compile(regex.toString());
        }
    }
}
