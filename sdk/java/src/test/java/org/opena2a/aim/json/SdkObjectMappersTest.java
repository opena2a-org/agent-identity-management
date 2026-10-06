package org.opena2a.aim.json;

import com.fasterxml.jackson.databind.JsonMappingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.exc.InvalidFormatException;
import com.fasterxml.jackson.databind.exc.MismatchedInputException;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;

import java.io.IOException;
import java.math.BigDecimal;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.*;

@DisplayName("SDK object mappers")
class SdkObjectMappersTest {

    private static final String LONG_STRING = "1".repeat(40_000) + "x";

    public static class Holder {
        public Double wrapperDouble;
        public double primitiveDouble;
        public Float wrapperFloat;
        public float primitiveFloat;
        public BigDecimal bigDecimal;
        public Number number;
        public Map<String, Double> doubleMap;
        public List<Double> doubleList;
    }

    private static final ObjectMapper GUARDED = SdkObjectMappers.create();
    private static final ObjectMapper STOCK = new ObjectMapper();

    private static Object read(ObjectMapper mapper, String field, String json) throws Exception {
        Holder holder = mapper.readValue(json, Holder.class);
        Object value = Holder.class.getField(field).get(holder);
        if (value instanceof Map<?, ?> map) {
            return map.get("k");
        }
        if (value instanceof List<?> list) {
            return list.get(0);
        }
        return value;
    }

    private static String document(String field, String jsonValue) {
        if (field.equals("doubleMap")) {
            return "{\"doubleMap\":{\"k\":" + jsonValue + "}}";
        }
        if (field.equals("doubleList")) {
            return "{\"doubleList\":[" + jsonValue + "]}";
        }
        return "{\"" + field + "\":" + jsonValue + "}";
    }

    private static void assertRefused(String field, String value, String forbiddenRun) {
        long start = System.nanoTime();
        MismatchedInputException e = assertThrows(MismatchedInputException.class,
                () -> read(GUARDED, field, document(field, "\"" + value + "\"")));
        long elapsedMs = (System.nanoTime() - start) / 1_000_000;
        assertTrue(elapsedMs < 1_000, "took " + elapsedMs + " ms");
        assertFalse(e instanceof InvalidFormatException, "refused before Jackson's conversion");
        String message = e.getMessage();
        assertTrue(message.contains("refused where a number is bound"), message);
        assertTrue(message.contains(field), "names the field: " + message);
        assertTrue(message.contains(value.length() + " characters"), "states the character count: " + message);
        assertTrue(message.contains("a JSON number is expected"), message);
        assertTrue(message.length() < 512, "message length " + message.length());
        if (!forbiddenRun.isEmpty()) {
            assertFalse(message.contains(forbiddenRun), "message carries the value: " + message);
        }
    }

    @ParameterizedTest(name = "{0}")
    @ValueSource(strings = {"wrapperDouble", "primitiveDouble", "wrapperFloat", "primitiveFloat",
            "bigDecimal", "number", "doubleMap", "doubleList"})
    @DisplayName("a JSON string is refused at every length; a number and null bind as with a stock mapper")
    void everyGuardedType(String field) throws Exception {
        assertRefused(field, "0.75", "0.75");
        assertRefused(field, LONG_STRING, "1".repeat(32));
        assertRefused(field, "", "");

        Object number = read(GUARDED, field, document(field, "0.75"));
        assertEquals(read(STOCK, field, document(field, "0.75")), number);
        assertEquals(0.75, ((Number) number).doubleValue(), 0.0);

        assertEquals(read(STOCK, field, document(field, "null")), read(GUARDED, field, document(field, "null")));
    }

    @Test
    @DisplayName("a long member name does not carry into the message")
    void longMemberNameIsShortened() {
        String key = "k".repeat(10_000);
        JsonMappingException e = assertThrows(JsonMappingException.class,
                () -> GUARDED.readValue("{\"doubleMap\":{\"" + key + "\":\"0.75\"}}", Holder.class));
        assertTrue(e.getOriginalMessage().length() < 512, "length " + e.getOriginalMessage().length());
    }

    @Test
    @DisplayName("C4: every SDK mapper is constructed by the factory")
    void mappersAreConstructedOnlyByTheFactory() throws IOException {
        Path main = Path.of(System.getProperty("basedir", ".")).resolve("src/main/java");
        assertTrue(Files.isDirectory(main), "source root " + main.toAbsolutePath());
        List<String> constructing;
        try (Stream<Path> files = Files.walk(main)) {
            constructing = files
                    .filter(p -> p.toString().endsWith(".java"))
                    .filter(p -> {
                        try {
                            String source = Files.readString(p);
                            return source.contains("new ObjectMapper(") || source.contains("JsonMapper.builder(")
                                    || source.contains("new JsonMapper(");
                        } catch (IOException e) {
                            throw new IllegalStateException(e);
                        }
                    })
                    .map(p -> main.relativize(p).toString().replace('\\', '/'))
                    .sorted()
                    .collect(Collectors.toList());
        }
        assertEquals(List.of("org/opena2a/aim/json/SdkObjectMappers.java"), constructing);
    }
}
