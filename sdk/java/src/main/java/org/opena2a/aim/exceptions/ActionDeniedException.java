package org.opena2a.aim.exceptions;

/**
 * Exception thrown when an action is denied due to insufficient capabilities.
 */
public class ActionDeniedException extends AIMException {

    private final String capability;
    private final String resource;

    public ActionDeniedException(String message) {
        super(message, "ACTION_DENIED", 403);
        this.capability = null;
        this.resource = null;
    }

    public ActionDeniedException(String message, String capability, String resource) {
        super(message, "ACTION_DENIED", 403);
        this.capability = capability;
        this.resource = resource;
    }

    public String getCapability() {
        return capability;
    }

    public String getResource() {
        return resource;
    }
}
