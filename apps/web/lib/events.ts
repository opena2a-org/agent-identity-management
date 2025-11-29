/**
 * Simple event emitter for real-time UI updates
 * Used to notify components when data changes (e.g., alerts acknowledged, capability requests approved)
 */

type EventCallback = () => void;

class EventEmitter {
  private events: Map<string, Set<EventCallback>> = new Map();

  /**
   * Subscribe to an event
   */
  on(event: string, callback: EventCallback): () => void {
    if (!this.events.has(event)) {
      this.events.set(event, new Set());
    }
    this.events.get(event)!.add(callback);

    // Return unsubscribe function
    return () => {
      this.events.get(event)?.delete(callback);
    };
  }

  /**
   * Emit an event to all subscribers
   */
  emit(event: string): void {
    const callbacks = this.events.get(event);
    if (callbacks) {
      callbacks.forEach((callback) => callback());
    }
  }

  /**
   * Remove all listeners for an event
   */
  off(event: string): void {
    this.events.delete(event);
  }
}

// Singleton instance
export const eventEmitter = new EventEmitter();

// Predefined event types for type safety
export const Events = {
  ALERT_ACKNOWLEDGED: "alert:acknowledged",
  ALERT_RESOLVED: "alert:resolved",
  CAPABILITY_REQUEST_APPROVED: "capability-request:approved",
  CAPABILITY_REQUEST_REJECTED: "capability-request:rejected",
  VERIFICATION_APPROVED: "verification:approved",
  VERIFICATION_DENIED: "verification:denied",
} as const;
