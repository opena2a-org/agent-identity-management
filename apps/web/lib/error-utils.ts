/**
 * Utility function to extract error messages from different error formats
 * Handles Error objects, strings, and objects with message properties
 */
export function extractErrorMessage(
  error: unknown,
  fallbackMessage: string
): string {
  if (error instanceof Error) {
    return error.message;
  }

  if (typeof error === "string") {
    return error;
  }

  if (error && typeof error === "object" && "message" in error) {
    return (error as any).message;
  }

  return fallbackMessage;
}

/**
 * Common error messages for different operations
 */
export const ERROR_MESSAGES = {
  MCP_SERVER_SAVE: "Failed to save MCP server",
  API_KEY_CREATE: "Failed to create API key",
  AGENT_SAVE: "Failed to save agent",
  TAG_CREATE: "Failed to create tag",
  TAGS_UPDATE: "Could not update agent tags. Please try again.",
  SDK_DOWNLOAD:
    "Failed to download SDK. Please try again or use Manual Integration.",
  CREDENTIALS_LOAD:
    "Could not fetch agent credentials. Please try again or contact support.",
  CLIPBOARD_COPY: "Failed to copy to clipboard. Please copy manually.",
} as const;



