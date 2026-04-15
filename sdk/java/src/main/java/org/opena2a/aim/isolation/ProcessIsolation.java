package org.opena2a.aim.isolation;

public enum ProcessIsolation {
    NONE("none"),
    PIDNS("pidns"),
    SECCOMP("seccomp"),
    APPARMOR("apparmor"),
    SELINUX("selinux"),
    FULL("full");

    private final String value;

    ProcessIsolation(String value) {
        this.value = value;
    }

    public String getValue() {
        return value;
    }

    public static ProcessIsolation fromValue(String value) {
        for (ProcessIsolation type : values()) {
            if (type.value.equals(value)) {
                return type;
            }
        }
        return NONE;
    }
}
