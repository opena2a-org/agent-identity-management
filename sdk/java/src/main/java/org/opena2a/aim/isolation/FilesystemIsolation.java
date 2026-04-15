package org.opena2a.aim.isolation;

public enum FilesystemIsolation {
    NONE("none"),
    CHROOT("chroot"),
    TMPFS("tmpfs"),
    READONLY("readonly"),
    OVERLAY("overlay");

    private final String value;

    FilesystemIsolation(String value) {
        this.value = value;
    }

    public String getValue() {
        return value;
    }

    public static FilesystemIsolation fromValue(String value) {
        for (FilesystemIsolation type : values()) {
            if (type.value.equals(value)) {
                return type;
            }
        }
        return NONE;
    }
}
