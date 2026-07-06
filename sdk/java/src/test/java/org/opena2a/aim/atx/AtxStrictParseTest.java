package org.opena2a.aim.atx;

import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.nio.charset.StandardCharsets;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;

/** Unit coverage for the fold-aware strict parse (AtxStrictParse). */
class AtxStrictParseTest {

    @Test
    void foldKeyMatchesReferenceFold() {
        assertEquals("trustlevel", AtxStrictParse.foldKey("trustLevel"));
        assertEquals("trustlevel", AtxStrictParse.foldKey("TRUSTLEVEL"));
        assertEquals("trustlevel", AtxStrictParse.foldKey("TrustLevel"));
        assertEquals("category", AtxStrictParse.foldKey("Category"));
        assertEquals("agentid", AtxStrictParse.foldKey("agentId"));
        // The two non-ASCII runes that fold onto ASCII.
        assertEquals("k", AtxStrictParse.foldKey("K")); // Kelvin sign -> k
        assertEquals("s", AtxStrictParse.foldKey("ſ")); // long s -> s
        // Case variants and their fold targets collide; distinct names do not.
        assertEquals(AtxStrictParse.foldKey("trustLevel"), AtxStrictParse.foldKey("TRUSTLEVEL"));
        assertEquals(AtxStrictParse.foldKey("K"), AtxStrictParse.foldKey("K"));
    }

    @Test
    void detectsSameNameAndFoldDuplicatesAtAnyDepth() throws IOException {
        assertEquals("trustLevel", firstDup("{\"trustLevel\":4,\"trustLevel\":9}"));
        assertEquals("trustLevel", firstDup("{\"TRUSTLEVEL\":9,\"trustLevel\":4}"));
        assertEquals("category", firstDup("{\"a\":{\"Category\":\"x\",\"category\":\"y\"}}"));
        assertEquals("k", firstDup("{\"a\":[{\"K\":1,\"k\":2}]}")); // Kelvin/k fold in an array element
    }

    @Test
    void passesDuplicateFreeCredentials() throws IOException {
        assertNull(firstDup("{\"trustLevel\":4,\"trustScore\":9.5}"));
        assertNull(firstDup("{\"a\":{\"x\":1},\"b\":{\"x\":2}}"));
        assertNull(firstDup("{}"));
        assertNull(firstDup("{\"caps\":[\"read\",\"read\"]}")); // duplicate VALUES are fine, not member names
    }

    @Test
    void rejectsPathologicallyDeepNesting() {
        // Jackson's StreamReadConstraints caps nesting (default 1000); a deeper
        // credential makes the parser throw before the scan can recurse unbounded,
        // so verify(byte[]) surfaces it as MALFORMED rather than overflowing.
        StringBuilder deep = new StringBuilder();
        int depth = 5000;
        for (int i = 0; i < depth; i++) {
            deep.append("{\"a\":");
        }
        deep.append("1");
        for (int i = 0; i < depth; i++) {
            deep.append("}");
        }
        assertThrows(IOException.class, () -> firstDup(deep.toString()));
    }

    private static String firstDup(String json) throws IOException {
        return AtxStrictParse.firstDuplicateMember(json.getBytes(StandardCharsets.UTF_8));
    }
}
