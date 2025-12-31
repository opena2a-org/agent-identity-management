package org.opena2a.aim.a2a;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.time.Instant;
import java.util.List;
import java.util.Map;

/**
 * A2A Agent Card - Self-describing manifest for A2A protocol.
 * Based on Google's A2A specification with AIM security enhancements.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
public class A2AAgentCard {

    @JsonProperty("id")
    private String id;

    @JsonProperty("agentId")
    private String agentId;

    @JsonProperty("name")
    private String name;

    @JsonProperty("description")
    private String description;

    @JsonProperty("url")
    private String url;

    @JsonProperty("version")
    private String version;

    @JsonProperty("provider")
    private Provider provider;

    @JsonProperty("capabilities")
    private Capabilities capabilities;

    @JsonProperty("skills")
    private List<Skill> skills;

    @JsonProperty("authentication")
    private Authentication authentication;

    @JsonProperty("defaultInputModes")
    private List<String> defaultInputModes;

    @JsonProperty("defaultOutputModes")
    private List<String> defaultOutputModes;

    @JsonProperty("supportsAuthenticatedExtendedCard")
    private boolean supportsAuthenticatedExtendedCard;

    @JsonProperty("aimAttestation")
    private String aimAttestation;

    @JsonProperty("aimAttestationExpiresAt")
    private Instant aimAttestationExpiresAt;

    @JsonProperty("publicKey")
    private String publicKey;

    @JsonProperty("verified")
    private boolean verified;

    @JsonProperty("createdAt")
    private Instant createdAt;

    @JsonProperty("updatedAt")
    private Instant updatedAt;

    // Default constructor for Jackson
    public A2AAgentCard() {}

    // Builder pattern
    public static Builder builder() {
        return new Builder();
    }

    // Getters
    public String getId() { return id; }
    public String getAgentId() { return agentId; }
    public String getName() { return name; }
    public String getDescription() { return description; }
    public String getUrl() { return url; }
    public String getVersion() { return version; }
    public Provider getProvider() { return provider; }
    public Capabilities getCapabilities() { return capabilities; }
    public List<Skill> getSkills() { return skills; }
    public Authentication getAuthentication() { return authentication; }
    public List<String> getDefaultInputModes() { return defaultInputModes; }
    public List<String> getDefaultOutputModes() { return defaultOutputModes; }
    public boolean isSupportsAuthenticatedExtendedCard() { return supportsAuthenticatedExtendedCard; }
    public String getAimAttestation() { return aimAttestation; }
    public Instant getAimAttestationExpiresAt() { return aimAttestationExpiresAt; }
    public String getPublicKey() { return publicKey; }
    public boolean isVerified() { return verified; }
    public Instant getCreatedAt() { return createdAt; }
    public Instant getUpdatedAt() { return updatedAt; }

    // Setters for Jackson
    public void setId(String id) { this.id = id; }
    public void setAgentId(String agentId) { this.agentId = agentId; }
    public void setName(String name) { this.name = name; }
    public void setDescription(String description) { this.description = description; }
    public void setUrl(String url) { this.url = url; }
    public void setVersion(String version) { this.version = version; }
    public void setProvider(Provider provider) { this.provider = provider; }
    public void setCapabilities(Capabilities capabilities) { this.capabilities = capabilities; }
    public void setSkills(List<Skill> skills) { this.skills = skills; }
    public void setAuthentication(Authentication authentication) { this.authentication = authentication; }
    public void setDefaultInputModes(List<String> defaultInputModes) { this.defaultInputModes = defaultInputModes; }
    public void setDefaultOutputModes(List<String> defaultOutputModes) { this.defaultOutputModes = defaultOutputModes; }
    public void setSupportsAuthenticatedExtendedCard(boolean supportsAuthenticatedExtendedCard) { this.supportsAuthenticatedExtendedCard = supportsAuthenticatedExtendedCard; }
    public void setAimAttestation(String aimAttestation) { this.aimAttestation = aimAttestation; }
    public void setAimAttestationExpiresAt(Instant aimAttestationExpiresAt) { this.aimAttestationExpiresAt = aimAttestationExpiresAt; }
    public void setPublicKey(String publicKey) { this.publicKey = publicKey; }
    public void setVerified(boolean verified) { this.verified = verified; }
    public void setCreatedAt(Instant createdAt) { this.createdAt = createdAt; }
    public void setUpdatedAt(Instant updatedAt) { this.updatedAt = updatedAt; }

    /**
     * Provider information.
     */
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class Provider {
        @JsonProperty("organization")
        private String organization;

        @JsonProperty("url")
        private String url;

        public Provider() {}

        public Provider(String organization, String url) {
            this.organization = organization;
            this.url = url;
        }

        public String getOrganization() { return organization; }
        public void setOrganization(String organization) { this.organization = organization; }
        public String getUrl() { return url; }
        public void setUrl(String url) { this.url = url; }
    }

    /**
     * Agent capabilities.
     */
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class Capabilities {
        @JsonProperty("streaming")
        private boolean streaming;

        @JsonProperty("pushNotifications")
        private boolean pushNotifications;

        @JsonProperty("stateTransitionHistory")
        private boolean stateTransitionHistory;

        public Capabilities() {}

        public boolean isStreaming() { return streaming; }
        public void setStreaming(boolean streaming) { this.streaming = streaming; }
        public boolean isPushNotifications() { return pushNotifications; }
        public void setPushNotifications(boolean pushNotifications) { this.pushNotifications = pushNotifications; }
        public boolean isStateTransitionHistory() { return stateTransitionHistory; }
        public void setStateTransitionHistory(boolean stateTransitionHistory) { this.stateTransitionHistory = stateTransitionHistory; }
    }

    /**
     * Agent skill definition.
     */
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class Skill {
        @JsonProperty("id")
        private String id;

        @JsonProperty("name")
        private String name;

        @JsonProperty("description")
        private String description;

        @JsonProperty("tags")
        private List<String> tags;

        @JsonProperty("examples")
        private List<String> examples;

        @JsonProperty("inputModes")
        private List<String> inputModes;

        @JsonProperty("outputModes")
        private List<String> outputModes;

        public Skill() {}

        public String getId() { return id; }
        public void setId(String id) { this.id = id; }
        public String getName() { return name; }
        public void setName(String name) { this.name = name; }
        public String getDescription() { return description; }
        public void setDescription(String description) { this.description = description; }
        public List<String> getTags() { return tags; }
        public void setTags(List<String> tags) { this.tags = tags; }
        public List<String> getExamples() { return examples; }
        public void setExamples(List<String> examples) { this.examples = examples; }
        public List<String> getInputModes() { return inputModes; }
        public void setInputModes(List<String> inputModes) { this.inputModes = inputModes; }
        public List<String> getOutputModes() { return outputModes; }
        public void setOutputModes(List<String> outputModes) { this.outputModes = outputModes; }
    }

    /**
     * Authentication configuration.
     */
    @JsonIgnoreProperties(ignoreUnknown = true)
    public static class Authentication {
        @JsonProperty("schemes")
        private List<String> schemes;

        @JsonProperty("credentials")
        private String credentials;

        public Authentication() {}

        public List<String> getSchemes() { return schemes; }
        public void setSchemes(List<String> schemes) { this.schemes = schemes; }
        public String getCredentials() { return credentials; }
        public void setCredentials(String credentials) { this.credentials = credentials; }
    }

    /**
     * Builder for A2AAgentCard.
     */
    public static class Builder {
        private final A2AAgentCard card = new A2AAgentCard();

        public Builder id(String id) { card.id = id; return this; }
        public Builder agentId(String agentId) { card.agentId = agentId; return this; }
        public Builder name(String name) { card.name = name; return this; }
        public Builder description(String description) { card.description = description; return this; }
        public Builder url(String url) { card.url = url; return this; }
        public Builder version(String version) { card.version = version; return this; }
        public Builder provider(Provider provider) { card.provider = provider; return this; }
        public Builder capabilities(Capabilities capabilities) { card.capabilities = capabilities; return this; }
        public Builder skills(List<Skill> skills) { card.skills = skills; return this; }
        public Builder authentication(Authentication authentication) { card.authentication = authentication; return this; }
        public Builder defaultInputModes(List<String> defaultInputModes) { card.defaultInputModes = defaultInputModes; return this; }
        public Builder defaultOutputModes(List<String> defaultOutputModes) { card.defaultOutputModes = defaultOutputModes; return this; }
        public Builder supportsAuthenticatedExtendedCard(boolean supportsAuthenticatedExtendedCard) { card.supportsAuthenticatedExtendedCard = supportsAuthenticatedExtendedCard; return this; }
        public Builder aimAttestation(String aimAttestation) { card.aimAttestation = aimAttestation; return this; }
        public Builder aimAttestationExpiresAt(Instant aimAttestationExpiresAt) { card.aimAttestationExpiresAt = aimAttestationExpiresAt; return this; }
        public Builder publicKey(String publicKey) { card.publicKey = publicKey; return this; }
        public Builder verified(boolean verified) { card.verified = verified; return this; }

        public A2AAgentCard build() { return card; }
    }

    @Override
    public String toString() {
        return "A2AAgentCard{" +
                "id='" + id + '\'' +
                ", agentId='" + agentId + '\'' +
                ", name='" + name + '\'' +
                ", url='" + url + '\'' +
                ", verified=" + verified +
                '}';
    }
}
