package org.opena2a.aim.json;

import com.fasterxml.jackson.core.JsonParser;
import com.fasterxml.jackson.core.JsonToken;
import com.fasterxml.jackson.databind.DeserializationContext;
import com.fasterxml.jackson.databind.JsonDeserializer;
import com.fasterxml.jackson.databind.deser.std.DelegatingDeserializer;
import com.fasterxml.jackson.databind.exc.MismatchedInputException;
import com.fasterxml.jackson.databind.jsontype.TypeDeserializer;

import java.io.IOException;

/**
 * Wraps Jackson's standard deserializer for a floating-point type and refuses a JSON string token
 * before that deserializer sees it. Every other token, a JSON number or null included, goes to
 * the standard deserializer unchanged.
 */
final class JsonStringNumberGuard extends DelegatingDeserializer {

    private static final long serialVersionUID = 1L;
    private static final int MAX_FIELD_LENGTH = 160;

    JsonStringNumberGuard(JsonDeserializer<?> delegate) {
        super(delegate);
    }

    @Override
    protected JsonDeserializer<?> newDelegatingInstance(JsonDeserializer<?> newDelegatee) {
        return new JsonStringNumberGuard(newDelegatee);
    }

    @Override
    public Object deserialize(JsonParser p, DeserializationContext ctxt) throws IOException {
        refuseString(p);
        return super.deserialize(p, ctxt);
    }

    @Override
    public Object deserialize(JsonParser p, DeserializationContext ctxt, Object intoValue) throws IOException {
        refuseString(p);
        return super.deserialize(p, ctxt, intoValue);
    }

    @Override
    public Object deserializeWithType(JsonParser p, DeserializationContext ctxt,
                                      TypeDeserializer typeDeserializer) throws IOException {
        refuseString(p);
        return super.deserializeWithType(p, ctxt, typeDeserializer);
    }

    private void refuseString(JsonParser p) throws IOException {
        if (!p.hasToken(JsonToken.VALUE_STRING)) {
            return;
        }
        String message = "Field \"" + field(p) + "\": a JSON string of " + p.getTextLength()
                + " characters was refused where a number is bound; a JSON number is expected";
        throw MismatchedInputException.from(p, handledType(), message);
    }

    /** The JSON Pointer of the current value, shortened so a long member name cannot grow the message. */
    private static String field(JsonParser p) {
        String pointer = p.getParsingContext().pathAsPointer().toString();
        if (pointer.isEmpty()) {
            return "(root)";
        }
        if (pointer.length() > MAX_FIELD_LENGTH) {
            return pointer.substring(0, MAX_FIELD_LENGTH - 3) + "...";
        }
        return pointer;
    }
}
