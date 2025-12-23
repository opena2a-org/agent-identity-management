package org.opena2a.aim.exceptions;

/**
 * Exception thrown when an action is denied due to insufficient capabilities.
 */
public class ActionDeniedException extends AIMException {

    private final String capability;
    private final String resource;

    /**
     * Creates a new ActionDeniedException with a message.
     *
     * @param message the error message
     */
    public ActionDeniedException(String message) {
        super(message, "ACTION_DENIED", 403);
        this.capability = null;
        this.resource = null;
    }

    /**
     * Creates a new ActionDeniedException with details about the denied action.
     *
     * @param message the error message
     * @param capability the capability that was required but not granted
     * @param resource the resource that was being accessed
     */
    public ActionDeniedException(String message, String capability, String resource) {
        super(message, "ACTION_DENIED", 403);
        this.capability = capability;
        this.resource = resource;
    }

    /**
     * Gets the capability that was required but not granted.
     *
     * @return the capability name, or null if not specified
     */
    public String getCapability() {
        return capability;
    }

    /**
     * Gets the resource that was being accessed when the action was denied.
     *
     * @return the resource identifier, or null if not specified
     */
    public String getResource() {
        return resource;
    }
}
