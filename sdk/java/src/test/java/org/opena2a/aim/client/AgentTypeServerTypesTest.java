package org.opena2a.aim.client;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertSame;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assertions.fail;

/**
 * Checks AgentType against the agent types the server defines, read from the
 * backend source: the types isValidAgentType accepts at registration, with
 * their values from the domain constants. An AgentType value outside that set
 * is refused at registration and, as a listAgents filter, matches no agent.
 */
class AgentTypeServerTypesTest {

    private static final String VALIDATOR = "apps/backend/internal/application/agent_service.go";
    private static final String CONSTANTS = "apps/backend/internal/domain/agent.go";

    private static final Pattern VALIDATOR_BODY =
            Pattern.compile("func isValidAgentType\\(.*?\\{(.*?)return validTypes", Pattern.DOTALL);
    private static final Pattern VALIDATOR_ENTRY = Pattern.compile("domain\\.(AgentType\\w+):\\s*true");
    private static final Pattern CONSTANT = Pattern.compile("(AgentType\\w+)\\s+AgentType\\s*=\\s*\"([^\"]*)\"");

    @Test
    @DisplayName("every AgentType is sent as an agent type the server defines")
    void everyAgentTypeIsAServerType() throws IOException {
        Set<String> serverTypes = serverAgentTypes();

        List<String> outside = new ArrayList<>();
        for (AgentType type : AgentType.values()) {
            if (!serverTypes.contains(type.getValue())) {
                outside.add(type.name() + "(\"" + type.getValue() + "\")");
            }
        }
        assertTrue(outside.isEmpty(),
                "AgentType values the server does not define: " + outside + "; server types: " + serverTypes);
    }

    @Test
    @DisplayName("fromString returns the AgentType of every agent type the server defines")
    void fromStringRecognisesEveryServerType() throws IOException {
        List<String> unrecognised = new ArrayList<>();
        for (String serverType : serverAgentTypes()) {
            if (!serverType.equals(AgentType.fromString(serverType).getValue())) {
                unrecognised.add(serverType);
            }
        }
        assertTrue(unrecognised.isEmpty(), "server agent types fromString does not return: " + unrecognised);
    }

    @Test
    @DisplayName("UNKNOWN is sent as ai_agent, and fromString returns it only for a value no other type has")
    void unknownIsSentAsAiAgent() {
        assertEquals("ai_agent", AgentType.UNKNOWN.getValue());
        assertSame(AgentType.AI_AGENT, AgentType.fromString("ai_agent"));
        assertSame(AgentType.UNKNOWN, AgentType.fromString("unknown"));
        assertSame(AgentType.UNKNOWN, AgentType.fromString("not-an-agent-type"));
        assertSame(AgentType.UNKNOWN, AgentType.fromString(null));
    }

    /** The values of the agent types isValidAgentType accepts. */
    private static Set<String> serverAgentTypes() throws IOException {
        Map<String, String> constants = new HashMap<>();
        Matcher constant = CONSTANT.matcher(read(CONSTANTS));
        while (constant.find()) {
            constants.put(constant.group(1), constant.group(2));
        }

        Matcher body = VALIDATOR_BODY.matcher(read(VALIDATOR));
        if (!body.find()) {
            fail("isValidAgentType was not found in " + VALIDATOR);
        }
        Set<String> types = new TreeSet<>();
        Matcher entry = VALIDATOR_ENTRY.matcher(body.group(1));
        while (entry.find()) {
            String value = constants.get(entry.group(1));
            assertNotNull(value, entry.group(1) + " is not a string constant in " + CONSTANTS);
            types.add(value);
        }
        assertFalse(types.isEmpty(), "no agent types found in isValidAgentType in " + VALIDATOR);
        return types;
    }

    private static String read(String repoRelative) throws IOException {
        Path start = Paths.get("").toAbsolutePath();
        for (Path dir = start; dir != null; dir = dir.getParent()) {
            Path candidate = dir.resolve(repoRelative);
            if (Files.isRegularFile(candidate)) {
                return new String(Files.readAllBytes(candidate), StandardCharsets.UTF_8);
            }
        }
        return fail(repoRelative + " was not found above " + start
                + ". This test reads the backend's agent types and has to run inside the repository.");
    }
}
