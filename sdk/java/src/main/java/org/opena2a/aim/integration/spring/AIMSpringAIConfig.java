package org.opena2a.aim.integration.spring;

import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.client.AgentType;
import org.opena2a.aim.security.RiskDetector;
import org.opena2a.aim.security.SecurityLogger;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.List;
import java.util.Map;

/**
 * Spring AI integration configuration for AIM SDK.
 *
 * <p>Provides auto-configuration for Spring AI projects with:</p>
 * <ul>
 *   <li>Automatic AIMClient initialization</li>
 *   <li>Chat client interceptors for security logging</li>
 *   <li>Function calling advisors for capability tracking</li>
 *   <li>Observation integration for metrics</li>
 * </ul>
 *
 * <h2>Usage with Spring Boot:</h2>
 * <p>Add to your Spring Boot application:</p>
 * <pre>{@code
 * @Configuration
 * @EnableAIM
 * public class AIConfig {
 *     // AIMClient and interceptors auto-configured
 * }
 * }</pre>
 *
 * <h2>Manual Configuration:</h2>
 * <pre>{@code
 * @Bean
 * public AIMClient aimClient(AIMSpringAIProperties props) {
 *     return AIMSpringAIConfig.createClient(props);
 * }
 *
 * @Bean
 * public AIMChatClientInterceptor aimInterceptor(AIMClient client) {
 *     return AIMSpringAIConfig.createInterceptor(client);
 * }
 * }</pre>
 *
 * <h2>Properties:</h2>
 * <pre>
 * aim.agent-name=my-spring-ai-agent
 * aim.capabilities=chat:complete,embedding:create
 * aim.agent-type=CUSTOM
 * aim.auto-register=true
 * aim.security.log-prompts=false
 * aim.security.log-responses=false
 * </pre>
 *
 * <p>Note: This class provides factory methods for Spring integration.
 * For full auto-configuration, use the spring-boot-starter-aim module.</p>
 */
public class AIMSpringAIConfig {

    private static final Logger log = LoggerFactory.getLogger(AIMSpringAIConfig.class);

    private AIMSpringAIConfig() {}

    /**
     * Create an AIMClient from Spring AI properties.
     *
     * @param properties Configuration properties
     * @return Configured AIMClient
     */
    public static AIMClient createClient(AIMSpringAIProperties properties) {
        log.info("Creating AIMClient for Spring AI: {}", properties.getAgentName());

        AIMClient client = AIMClient.secure(
                properties.getAgentName(),
                properties.getCapabilities(),
                properties.getAgentType(),
                properties.getTalksTo(),
                properties.getDescription(),
                properties.getTags(),
                properties.getMetadata(),
                properties.getMcpCommands()
        );

        if (properties.isAutoRegister()) {
            log.info("Auto-registering agent with AIM server");
        }

        return client;
    }

    /**
     * Create a chat client interceptor for security logging.
     *
     * @param client AIMClient to use for logging
     * @return Configured interceptor
     */
    public static AIMChatClientInterceptor createInterceptor(AIMClient client) {
        return new AIMChatClientInterceptor(client);
    }

    /**
     * Create a chat client interceptor with custom configuration.
     *
     * @param client AIMClient to use
     * @param config Interceptor configuration
     * @return Configured interceptor
     */
    public static AIMChatClientInterceptor createInterceptor(
            AIMClient client, AIMChatClientInterceptor.Config config) {
        return new AIMChatClientInterceptor(client, config);
    }

    /**
     * Create a function calling advisor for capability tracking.
     *
     * @param client AIMClient to use
     * @return Configured advisor
     */
    public static AIMFunctionCallingAdvisor createFunctionAdvisor(AIMClient client) {
        return new AIMFunctionCallingAdvisor(client);
    }

    /**
     * Builder for Spring AI configuration.
     */
    public static Builder builder() {
        return new Builder();
    }

    public static class Builder {
        private final AIMSpringAIProperties properties = new AIMSpringAIProperties();

        public Builder agentName(String agentName) {
            properties.setAgentName(agentName);
            return this;
        }

        public Builder capabilities(List<String> capabilities) {
            properties.setCapabilities(capabilities);
            return this;
        }

        public Builder agentType(AgentType agentType) {
            properties.setAgentType(agentType);
            return this;
        }

        public Builder talksTo(List<String> talksTo) {
            properties.setTalksTo(talksTo);
            return this;
        }

        public Builder description(String description) {
            properties.setDescription(description);
            return this;
        }

        public Builder tags(List<String> tags) {
            properties.setTags(tags);
            return this;
        }

        public Builder metadata(Map<String, Object> metadata) {
            properties.setMetadata(metadata);
            return this;
        }

        public Builder mcpCommands(Map<String, String> mcpCommands) {
            properties.setMcpCommands(mcpCommands);
            return this;
        }

        public Builder autoRegister(boolean autoRegister) {
            properties.setAutoRegister(autoRegister);
            return this;
        }

        public Builder logPrompts(boolean logPrompts) {
            properties.getSecurityConfig().setLogPrompts(logPrompts);
            return this;
        }

        public Builder logResponses(boolean logResponses) {
            properties.getSecurityConfig().setLogResponses(logResponses);
            return this;
        }

        public AIMSpringAIProperties buildProperties() {
            return properties;
        }

        public AIMClient buildClient() {
            return createClient(properties);
        }

        public SpringAIIntegration build() {
            AIMClient client = buildClient();
            return new SpringAIIntegration(client, properties);
        }
    }

    /**
     * Complete Spring AI integration bundle.
     */
    public static class SpringAIIntegration {
        private final AIMClient client;
        private final AIMSpringAIProperties properties;
        private final AIMChatClientInterceptor interceptor;
        private final AIMFunctionCallingAdvisor functionAdvisor;

        public SpringAIIntegration(AIMClient client, AIMSpringAIProperties properties) {
            this.client = client;
            this.properties = properties;
            this.interceptor = new AIMChatClientInterceptor(client,
                    new AIMChatClientInterceptor.Config(
                            properties.getSecurityConfig().isLogPrompts(),
                            properties.getSecurityConfig().isLogResponses()
                    ));
            this.functionAdvisor = new AIMFunctionCallingAdvisor(client);
        }

        public AIMClient getClient() { return client; }
        public AIMSpringAIProperties getProperties() { return properties; }
        public AIMChatClientInterceptor getInterceptor() { return interceptor; }
        public AIMFunctionCallingAdvisor getFunctionAdvisor() { return functionAdvisor; }
    }
}
