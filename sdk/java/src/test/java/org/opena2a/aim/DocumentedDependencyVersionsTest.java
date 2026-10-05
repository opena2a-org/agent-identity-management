package org.opena2a.aim;

import org.junit.jupiter.api.Test;
import org.w3c.dom.Document;
import org.w3c.dom.Element;
import org.w3c.dom.Node;
import org.w3c.dom.NodeList;

import javax.xml.XMLConstants;
import javax.xml.parsers.DocumentBuilderFactory;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assertions.fail;

/**
 * The SDK's documentation states no dependency version that its pom contradicts.
 *
 * The README ships inside the SDK download next to the pom it describes, so a
 * stale version in it gives a reader two answers. Both documents point at the
 * pom for versions; if either one states a version again, it has to be the one
 * the pom pins.
 */
class DocumentedDependencyVersionsTest {

    /** Repo-relative location of the SDK's pom. */
    static final String POM = "sdk/java/pom.xml";

    /** Repo-relative documents that describe the SDK's dependencies. */
    static final List<String> DOCUMENTS = List.of("sdk/java/README.md", "docs/sdk/java.md");

    /** Name a document uses for a library, lower-cased without spaces, and the pom dependency that pins it. */
    private static final Map<String, String> LIBRARIES = Map.of(
            "okhttp", "com.squareup.okhttp3:okhttp",
            "jackson", "com.fasterxml.jackson.core:jackson-databind",
            "bouncycastle", "org.bouncycastle:bcprov-jdk18on",
            "aspectj", "org.aspectj:aspectjrt",
            "slf4j", "org.slf4j:slf4j-api");

    /** A library name followed by a version, as prose, a list item or a table cell writes it. */
    private static final Pattern STATED_VERSION = Pattern.compile(
            "\\b(OkHttp|Jackson|Bouncy\\s?Castle|AspectJ|SLF4J)\\b[\\s|:*(,-]*(?:version\\s+)?v?(\\d+(?:\\.\\d+)+)",
            Pattern.CASE_INSENSITIVE);

    private static final Pattern LINK_TARGET = Pattern.compile("\\]\\(([^)\\s#]+)");

    private static final Pattern PROPERTY_REFERENCE = Pattern.compile("\\$\\{([^}]+)}");

    @Test
    void noDocumentStatesAVersionThePomContradicts() throws Exception {
        Path root = repoRoot();
        Map<String, String> pinned = pinnedVersions(root.resolve(POM));

        List<String> contradictions = new ArrayList<>();
        for (String document : DOCUMENTS) {
            contradictions.addAll(contradictions(document, read(root.resolve(document)), pinned));
        }

        if (!contradictions.isEmpty()) {
            fail("Documented dependency versions differ from " + POM
                    + ". Point at the pom instead of repeating a version:\n  "
                    + String.join("\n  ", contradictions));
        }
    }

    @Test
    void everyDocumentPointsAtThePomForVersions() throws IOException {
        Path root = repoRoot();
        Path pom = root.resolve(POM).normalize();

        for (String document : DOCUMENTS) {
            Path path = root.resolve(document);
            boolean pointsAtPom = false;
            Matcher link = LINK_TARGET.matcher(read(path));
            while (link.find()) {
                String target = link.group(1);
                if (target.endsWith("pom.xml") && path.getParent().resolve(target).normalize().equals(pom)) {
                    pointsAtPom = true;
                }
            }
            assertTrue(pointsAtPom, document + " has no link that resolves to " + POM);
        }
    }

    @Test
    void aStatedVersionThatDiffersFromThePomIsReported() {
        Map<String, String> pinned = Map.of(
                "com.fasterxml.jackson.core:jackson-databind", "2.18.11",
                "org.bouncycastle:bcprov-jdk18on", "1.85",
                "com.squareup.okhttp3:okhttp", "4.12.0");

        assertEquals(
                List.of("doc.md:2 states Jackson 2.16; the pom pins com.fasterxml.jackson.core:jackson-databind 2.18.11",
                        "doc.md:3 states BouncyCastle 1.79; the pom pins org.bouncycastle:bcprov-jdk18on 1.85"),
                contradictions("doc.md",
                        "| Library | Purpose |\n| Jackson 2.16 | JSON |\n- **BouncyCastle 1.79** - Ed25519\n",
                        pinned));

        assertEquals(List.of(),
                contradictions("doc.md", "OkHttp 4.12, Jackson 2.18.11 and Bouncy Castle 1.85\n", pinned),
                "a version the pom pins, written with fewer components, is not a contradiction");
    }

    /** Each stated version in {@code text} that is not the pom's version or a prefix of it. */
    static List<String> contradictions(String document, String text, Map<String, String> pinned) {
        List<String> found = new ArrayList<>();
        Matcher stated = STATED_VERSION.matcher(text);
        while (stated.find()) {
            String library = stated.group(1).replaceAll("\\s", "").toLowerCase(Locale.ROOT);
            String coordinates = LIBRARIES.get(library);
            String version = stated.group(2);
            String pin = pinned.get(coordinates);
            if (pin == null) {
                found.add(document + ":" + line(text, stated.start()) + " states " + stated.group(1) + " " + version
                        + "; the pom has no dependency " + coordinates);
            } else if (!pin.equals(version) && !pin.startsWith(version + ".")) {
                found.add(document + ":" + line(text, stated.start()) + " states " + stated.group(1) + " " + version
                        + "; the pom pins " + coordinates + " " + pin);
            }
        }
        return found;
    }

    /** The version of each direct dependency of the pom, keyed "groupId:artifactId", with properties resolved. */
    static Map<String, String> pinnedVersions(Path pom) throws Exception {
        DocumentBuilderFactory factory = DocumentBuilderFactory.newInstance();
        factory.setFeature(XMLConstants.FEATURE_SECURE_PROCESSING, true);
        factory.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true);
        Document xml = factory.newDocumentBuilder().parse(pom.toFile());
        Element project = xml.getDocumentElement();

        Map<String, String> properties = new HashMap<>();
        for (Element block : children(project, "properties")) {
            NodeList entries = block.getChildNodes();
            for (int i = 0; i < entries.getLength(); i++) {
                if (entries.item(i).getNodeType() == Node.ELEMENT_NODE) {
                    properties.put(entries.item(i).getNodeName(), entries.item(i).getTextContent().trim());
                }
            }
        }

        Map<String, String> versions = new LinkedHashMap<>();
        for (Element block : children(project, "dependencies")) {
            for (Element dependency : children(block, "dependency")) {
                String coordinates = text(dependency, "groupId") + ":" + text(dependency, "artifactId");
                Matcher reference = PROPERTY_REFERENCE.matcher(text(dependency, "version"));
                StringBuilder version = new StringBuilder();
                while (reference.find()) {
                    String value = properties.get(reference.group(1));
                    if (value == null) {
                        fail(POM + " references undefined property ${" + reference.group(1) + "} for " + coordinates);
                    }
                    reference.appendReplacement(version, Matcher.quoteReplacement(value));
                }
                reference.appendTail(version);
                versions.put(coordinates, version.toString());
            }
        }
        assertTrue(versions.keySet().containsAll(LIBRARIES.values()),
                POM + " no longer declares every library this test maps: " + LIBRARIES.values());
        return versions;
    }

    private static List<Element> children(Element parent, String name) {
        List<Element> found = new ArrayList<>();
        NodeList nodes = parent.getChildNodes();
        for (int i = 0; i < nodes.getLength(); i++) {
            if (nodes.item(i) instanceof Element && nodes.item(i).getNodeName().equals(name)) {
                found.add((Element) nodes.item(i));
            }
        }
        return found;
    }

    private static String text(Element parent, String name) {
        List<Element> found = children(parent, name);
        return found.isEmpty() ? "" : found.get(0).getTextContent().trim();
    }

    private static int line(String text, int offset) {
        int line = 1;
        for (int i = 0; i < offset; i++) {
            if (text.charAt(i) == '\n') {
                line++;
            }
        }
        return line;
    }

    private static String read(Path path) throws IOException {
        return new String(Files.readAllBytes(path), StandardCharsets.UTF_8);
    }

    private static Path repoRoot() {
        Path start = Paths.get("").toAbsolutePath();
        for (Path dir = start; dir != null; dir = dir.getParent()) {
            if (Files.isRegularFile(dir.resolve(POM))) {
                return dir;
            }
        }
        return fail(POM + " was not found above " + start
                + ". This test reads the SDK's documentation and has to run inside the repository.");
    }
}
