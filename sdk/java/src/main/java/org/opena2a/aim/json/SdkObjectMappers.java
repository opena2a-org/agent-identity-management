package org.opena2a.aim.json;

import com.fasterxml.jackson.core.StreamReadFeature;
import com.fasterxml.jackson.databind.JsonDeserializer;
import com.fasterxml.jackson.databind.Module;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.deser.std.NumberDeserializers;
import com.fasterxml.jackson.databind.json.JsonMapper;
import com.fasterxml.jackson.databind.module.SimpleModule;

import java.math.BigDecimal;

/**
 * Constructs every Jackson {@link ObjectMapper} the SDK uses.
 *
 * <p>Each mapper registers {@link #numberGuardModule()}: where the SDK binds a floating-point
 * number ({@code Double}, {@code double}, {@code Float}, {@code float}, {@code BigDecimal} or
 * {@code Number}), a JSON string is refused, whatever its length, before Jackson converts it. A
 * JSON number binds as before. The refusal is a
 * {@link com.fasterxml.jackson.databind.exc.MismatchedInputException} whose message names the
 * field and the string's length and never contains the string itself.
 *
 * <p>This class is internal to the SDK and is not part of its supported API.
 */
public final class SdkObjectMappers {

    private static final String MODULE_NAME = "aim-sdk-number-guard";

    private SdkObjectMappers() {}

    /**
     * A mapper with the SDK's defaults.
     *
     * @return a new mapper
     */
    public static ObjectMapper create() {
        return builder().build();
    }

    /**
     * A builder with the SDK's defaults, for a site that enables further features.
     *
     * @return a new builder
     */
    public static JsonMapper.Builder builder() {
        return JsonMapper.builder()
                .disable(StreamReadFeature.INCLUDE_SOURCE_IN_LOCATION)
                .addModule(numberGuardModule());
    }

    /**
     * The module that refuses a JSON string where a floating-point number is bound.
     *
     * @return a new module instance
     */
    public static Module numberGuardModule() {
        SimpleModule module = new SimpleModule(MODULE_NAME);
        guard(module, Double.class);
        guard(module, Double.TYPE);
        guard(module, Float.class);
        guard(module, Float.TYPE);
        guard(module, BigDecimal.class);
        guard(module, Number.class);
        return module;
    }

    @SuppressWarnings({"unchecked", "rawtypes"})
    private static void guard(SimpleModule module, Class<?> type) {
        JsonDeserializer<?> stock = NumberDeserializers.find(type, type.getName());
        if (stock == null) {
            throw new IllegalStateException("Jackson has no standard deserializer for " + type.getName());
        }
        module.addDeserializer((Class) type, (JsonDeserializer) new JsonStringNumberGuard(stock));
    }
}
