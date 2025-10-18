export interface ErrorContext {
  resource?: string;
  action?: string;
}

function getStatusCode(error: any): number | null {
  if (error?.response?.status) return error.response.status;
  if (error?.status) return error.status;
  if (error?.statusCode) return error.statusCode;
  const match = error?.message?.match(/HTTP (\d{3})/i);
  if (match) return parseInt(match[1]);

  return null;
}

export function getErrorMessage(
  error: unknown,
  context: ErrorContext = {}
): string {
  const { resource = "data", action = "load" } = context;

  if (!error) {
    return `Failed to ${action} ${resource}. Please try again.`;
  }

  if (error instanceof Error) {
    const statusCode = getStatusCode(error);

    if (statusCode) {
      switch (statusCode) {
        case 400:
          return `Invalid request. Please check your input and try again.`;

        case 401:
          return `Authentication required. Please log in again to continue.`;

        case 403:
          return `Access denied. You don't have permission to ${action} ${resource}.`;

        case 404:
          return `${resource.charAt(0).toUpperCase() + resource.slice(1)} not found. It may have been deleted.`;

        case 409:
          return `Conflict detected. ${resource.charAt(0).toUpperCase() + resource.slice(1)} may already exist.`;

        case 422:
          return `Invalid data provided. Please check your input and try again.`;

        case 429:
          return `Too many requests. Please wait a moment and try again.`;

        case 500:
          return `Server error occurred. Our team has been notified. Please try again later.`;

        case 502:
        case 503:
          return `Service temporarily unavailable. Please try again in a few moments.`;

        case 504:
          return `Request timed out. Please check your connection and try again.`;

        default:
          if (statusCode >= 500) {
            return `Server error occurred. Please try again later.`;
          }
          if (statusCode >= 400) {
            return `Unable to ${action} ${resource}. Please try again.`;
          }
      }
    }

    if (
      error.message.toLowerCase().includes("network") ||
      error.message.toLowerCase().includes("fetch") ||
      error.message.toLowerCase().includes("connection")
    ) {
      return `Network connection failed. Please check your internet connection and try again.`;
    }

    if (error.message.toLowerCase().includes("timeout")) {
      return `Request timed out. The server is taking too long to respond. Please try again.`;
    }

    if (error.message.toLowerCase().includes("cors")) {
      return `Connection blocked by security settings. Please contact support.`;
    }

    if (
      error.message &&
      error.message.length > 0 &&
      error.message.length < 200
    ) {
      const cleanMessage = error.message
        .replace(/HTTP \d{3}:?/gi, "")
        .replace(/Error:?/gi, "")
        .trim();

      if (cleanMessage.length > 10) {
        return cleanMessage;
      }
    }
  }

  if (typeof error === "string") {
    return error;
  }

  if (typeof error === "object" && error !== null) {
    const errorObj = error as any;

    if (errorObj.message) {
      return getErrorMessage(new Error(errorObj.message), context);
    }

    if (errorObj.error) {
      return getErrorMessage(errorObj.error, context);
    }
  }

  return `Unable to ${action} ${resource}. Please try again later.`;
}

export function getRetryMessage(error: unknown): string {
  const statusCode = getStatusCode(error);

  if (!statusCode) {
    return "Please try again";
  }

  switch (statusCode) {
    case 401:
      return "Please log in again";

    case 403:
      return "Contact your administrator";

    case 404:
      return "Go back";

    case 429:
      return "Wait a moment and retry";

    case 500:
    case 502:
    case 503:
      return "Try again in a few minutes";

    case 504:
      return "Check your connection and retry";

    default:
      return "Try again";
  }
}

export function isRetryableError(error: unknown): boolean {
  const statusCode = getStatusCode(error);

  if (!statusCode) {
    return true;
  }

  if (statusCode >= 400 && statusCode < 500 && statusCode !== 429) {
    return false;
  }

  return statusCode >= 500 || statusCode === 429;
}
