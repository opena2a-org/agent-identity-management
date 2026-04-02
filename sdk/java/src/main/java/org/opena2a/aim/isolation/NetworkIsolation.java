package org.opena2a.aim.isolation;

public enum NetworkIsolation {
    NONE("none"),
    FIREWALL("firewall"),
    NAMESPACE("namespace"),
    VPC("vpc"),
    AIRGAP("airgap");

    private final String value;

    NetworkIsolation(String value) {
        this.value = value;
    }

    public String getValue() {
        return value;
    }

    public static NetworkIsolation fromValue(String value) {
        for (NetworkIsolation type : values()) {
            if (type.value.equals(value)) {
                return type;
            }
        }
        return NONE;
    }
}
