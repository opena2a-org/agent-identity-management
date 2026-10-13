package org.opena2a.aim.json;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.regex.Pattern;
import java.util.stream.Collectors;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.*;

/**
 * C5: every line of the SDK that declares or reads a floating-point value is in the checked-in
 * census {@code src/test/resources/floating-point-census.txt}, and every line the census says
 * binds a number from JSON names a cell that exists. A new declaration or read fails here until
 * it is listed, and it can be listed as a JSON binding only with a cell that drives it.
 */
@DisplayName("C5: floating-point census")
class FloatingPointCensusTest {

    private static final String CENSUS = "/floating-point-census.txt";
    private static final String SEPARATOR = " | ";
    private static final Pattern FLOATING_POINT = Pattern.compile(
            "\\b(Double|double|Float|float|BigDecimal|Number)\\b"
                    + "|\\.(asDouble|doubleValue|floatValue|decimalValue|numberValue"
                    + "|getDoubleValue|getFloatValue|getDecimalValue|getNumberValue)\\(");

    /** The test class whose {@code cells()} holds each cover's cells. */
    private static final Map<String, String> CELL_CLASSES = Map.of(
            "C1", "org.opena2a.aim.a2a.A2ATrustNumberStringTest",
            "C6", "org.opena2a.aim.json.TrustNumberStringOutsideA2ATest");

    record Entry(String file, String cover, String line) {
        String key() {
            return file + SEPARATOR + line;
        }
    }

    private static Path sourceRoot() {
        Path root = Path.of(System.getProperty("basedir", ".")).resolve("src/main/java/org/opena2a/aim");
        assertTrue(Files.isDirectory(root), "source root " + root.toAbsolutePath());
        return root;
    }

    /** Each floating-point line under the source root, as "file | trimmed line", once per occurrence. */
    private static List<String> scan(Path root) throws IOException {
        List<String> found = new ArrayList<>();
        List<Path> files;
        try (Stream<Path> walk = Files.walk(root)) {
            files = walk.filter(p -> p.toString().endsWith(".java")).sorted().collect(Collectors.toList());
        }
        for (Path file : files) {
            String name = root.relativize(file).toString().replace('\\', '/');
            for (String raw : Files.readAllLines(file, StandardCharsets.UTF_8)) {
                String line = raw.trim();
                if (line.startsWith("*") || line.startsWith("//") || line.startsWith("/*")) {
                    continue;
                }
                if (FLOATING_POINT.matcher(line).find()) {
                    found.add(name + SEPARATOR + line);
                }
            }
        }
        return found;
    }

    private static List<Entry> census() throws IOException {
        List<Entry> entries = new ArrayList<>();
        try (InputStream in = FloatingPointCensusTest.class.getResourceAsStream(CENSUS)) {
            assertNotNull(in, "census not found on the test classpath: " + CENSUS);
            BufferedReader reader = new BufferedReader(new InputStreamReader(in, StandardCharsets.UTF_8));
            String row;
            while ((row = reader.readLine()) != null) {
                if (row.isBlank() || row.startsWith("#")) {
                    continue;
                }
                String[] columns = row.split(Pattern.quote(SEPARATOR), 3);
                assertEquals(3, columns.length, "census row has three columns: " + row);
                entries.add(new Entry(columns[0].trim(), columns[1].trim(), columns[2].trim()));
            }
        }
        return entries;
    }

    private static Map<String, Integer> count(List<String> keys) {
        Map<String, Integer> counts = new TreeMap<>();
        keys.forEach(k -> counts.merge(k, 1, Integer::sum));
        return counts;
    }

    /** The keys of {@code a} that occur more often in {@code a} than in {@code b}. */
    private static List<String> excess(Map<String, Integer> a, Map<String, Integer> b) {
        return a.entrySet().stream()
                .filter(e -> e.getValue() > b.getOrDefault(e.getKey(), 0))
                .map(Map.Entry::getKey)
                .collect(Collectors.toList());
    }

    @Test
    @DisplayName("every floating-point declaration or read is listed, and every listed line still exists")
    void sourceMatchesTheCensus() throws IOException {
        Map<String, Integer> found = count(scan(sourceRoot()));
        Map<String, Integer> listed = count(census().stream().map(Entry::key).collect(Collectors.toList()));

        List<String> unlisted = excess(found, listed);
        assertTrue(unlisted.isEmpty(), "floating-point lines not in " + CENSUS + ". Add each with the C1 or C6 "
                + "cell that drives it from JSON, or with \"- <reason>\" if it binds no number from JSON:\n  "
                + String.join("\n  ", unlisted));

        List<String> stale = excess(listed, found);
        assertTrue(stale.isEmpty(), "census rows with no matching source line; remove or update them:\n  "
                + String.join("\n  ", stale));
    }

    @Test
    @DisplayName("every row names a cell that exists, or gives a reason")
    void everyCoverResolves() throws Exception {
        Map<String, Set<String>> cellsByCover = new TreeMap<>();
        for (Map.Entry<String, String> e : CELL_CLASSES.entrySet()) {
            cellsByCover.put(e.getKey(), cellNames(e.getValue()));
        }

        List<String> problems = new ArrayList<>();
        for (Entry entry : census()) {
            String cover = entry.cover();
            if (cover.startsWith("- ") && !cover.substring(2).isBlank()) {
                continue;
            }
            int space = cover.indexOf(' ');
            String kind = space < 0 ? cover : cover.substring(0, space);
            Set<String> cells = cellsByCover.get(kind);
            if (cells == null) {
                problems.add(entry.key() + ": cover \"" + cover + "\" is neither a C1 or C6 cell nor \"- <reason>\"");
            } else if (!cells.contains(cover.substring(space + 1))) {
                problems.add(entry.key() + ": no " + kind + " cell named \"" + cover.substring(space + 1)
                        + "\" in " + CELL_CLASSES.get(kind));
            }
        }
        assertTrue(problems.isEmpty(), String.join("\n", problems));
    }

    private static Set<String> cellNames(String className) throws Exception {
        Method cells = Class.forName(className).getDeclaredMethod("cells");
        cells.setAccessible(true);
        try (Stream<?> stream = (Stream<?>) cells.invoke(null)) {
            Set<String> names = stream.map(Object::toString).collect(Collectors.toSet());
            assertFalse(names.isEmpty(), className + " has no cells");
            return names;
        }
    }
}
