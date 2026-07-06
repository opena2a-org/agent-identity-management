package org.opena2a.aim.atx;

import com.fasterxml.jackson.core.JsonFactory;
import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.core.JsonToken;

import java.io.IOException;
import java.util.HashMap;
import java.util.Map;

/**
 * Strict pre-parse for ATX credentials: rejects a credential that carries a
 * duplicate object member at ANY depth, where "duplicate" is judged under the
 * field folding a lenient consumer's JSON parser would apply.
 *
 * <p>Every ATX field feeds a signed canonical form (the v1.1 JCS(TBS)
 * projection, the v1.0 pipe fields), so there is no layer with sanctioned
 * RFC 7519 last-wins semantics — a duplicate member anywhere in the credential
 * is the RFC 8259 §4 first-wins/last-wins parser-divergence smuggling split and
 * MUST be rejected before any field is interpreted. This mirrors the strict
 * parse in the reference verifiers (atx-conformance {@code verifiers/go},
 * {@code verifiers/python}) and {@code opena2a-registry/pkg/atcverify}.
 *
 * <p>Folding matters because a lenient struct/case-insensitive parser collapses
 * a case-variant pair like {@code {"trustLevel":9,"TRUSTLEVEL":1}} to one field
 * last-wins. Jackson's own {@code STRICT_DUPLICATE_DETECTION} is case-SENSITIVE
 * and would miss that collapse, so this scan folds member names (matching the Go
 * reference verifier's {@code foldKey}) and catches both exact duplicates and
 * fold-colliding case variants in a single pass.
 *
 * <p>Depth is bounded by Jackson's own {@code StreamReadConstraints}
 * (default max nesting 1000): a credential nested deeper makes {@link JsonParser}
 * throw before this scan can recurse unbounded, so no manual depth guard is
 * needed here (unlike Go's {@code encoding/json.Decoder.Token}, which has no
 * limit and forced an explicit bound in the reference verifier).
 */
public final class AtxStrictParse {

    private static final JsonFactory JSON = new JsonFactory();

    private AtxStrictParse() {
    }

    /**
     * Scans raw ATX credential JSON for a member name that collides — under field
     * folding — with an earlier member of the same object, at any depth, and
     * returns the first such name. Returns {@code null} when the credential has no
     * duplicate members.
     *
     * @throws IOException if the bytes are not well-formed JSON (or exceed
     *     Jackson's nesting limit); the caller surfaces that as a rejection.
     */
    public static String firstDuplicateMember(byte[] credentialJson) throws IOException {
        try (JsonParser p = JSON.createParser(credentialJson)) {
            JsonToken first = p.nextToken();
            if (first == null) {
                throw new IOException("empty credential");
            }
            return scanValue(p, first);
        }
    }

    /**
     * Consumes exactly one JSON value from {@code p} (whose current token is
     * {@code t}, the value's first token) and returns the first fold-duplicate
     * member found within it, or {@code null}. On return the parser's current
     * token is the last token of that value (its scalar, or its matching
     * END_OBJECT / END_ARRAY), so the caller advances past it with one
     * {@code nextToken()}.
     */
    private static String scanValue(JsonParser p, JsonToken t) throws IOException {
        if (t == JsonToken.START_OBJECT) {
            Map<String, String> seen = new HashMap<>(); // foldKey -> first raw name
            JsonToken ft;
            while ((ft = p.nextToken()) != JsonToken.END_OBJECT) {
                // ft is FIELD_NAME here.
                String name = p.currentName();
                String fk = foldKey(name);
                if (seen.containsKey(fk)) {
                    return name;
                }
                seen.put(fk, name);
                String dup = scanValue(p, p.nextToken());
                if (dup != null) {
                    return dup;
                }
            }
        } else if (t == JsonToken.START_ARRAY) {
            JsonToken et;
            while ((et = p.nextToken()) != JsonToken.END_ARRAY) {
                String dup = scanValue(p, et);
                if (dup != null) {
                    return dup;
                }
            }
        }
        // Scalar: nothing to consume; the token itself is the whole value.
        return null;
    }

    /**
     * Folds a JSON member name the way a lenient JSON-to-struct parser folds field
     * names: ASCII letters lowercased, plus the two non-ASCII letters that fold
     * onto ASCII (Kelvin sign U+212A -&gt; k, long s U+017F -&gt; s); any other code
     * point via {@link Character#toLowerCase(int)} (simple 1:1 case mapping, matching
     * Go's {@code unicode.ToLower} used by the reference verifier's {@code foldKey}).
     * Two names with the same fold key are the same field to such a parser, so
     * treating them as a duplicate here catches the case-variant collapse. Real ATX
     * member names are ASCII, so the fold is exact on every honest credential.
     */
    static String foldKey(String s) {
        StringBuilder b = new StringBuilder(s.length());
        int i = 0;
        while (i < s.length()) {
            int cp = s.codePointAt(i);
            i += Character.charCount(cp);
            if (cp >= 'A' && cp <= 'Z') {
                b.appendCodePoint(cp + ('a' - 'A'));
            } else if (cp == 0x212A) { // Kelvin sign -> k
                b.append('k');
            } else if (cp == 0x017F) { // latin small letter long s -> s
                b.append('s');
            } else {
                b.appendCodePoint(Character.toLowerCase(cp));
            }
        }
        return b.toString();
    }
}
