package org.opena2a.aim.isolation;

public enum SandboxType {
    NONE("none"),
    DOCKER("docker"),
    VM("vm"),
    GVISOR("gvisor"),
    FIRECRACKER("firecracker"),
    WASM("wasm"),
    KATA("kata");

    private final String value;

    SandboxType(String value) {
        this.value = value;
    }

    public String getValue() {
        return value;
    }

    public static SandboxType fromValue(String value) {
        for (SandboxType type : values()) {
            if (type.value.equals(value)) {
                return type;
            }
        }
        return NONE;
    }
}
