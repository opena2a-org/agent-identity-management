import { useState, useCallback, useRef, useEffect } from "react";

interface UseClipboardOptions {
  /** Duration in ms to show "copied" state before resetting (default: 2000) */
  successDuration?: number;
  /** Callback when copy succeeds */
  onSuccess?: () => void;
  /** Callback when copy fails */
  onError?: (error: Error) => void;
}

interface UseClipboardReturn {
  /** Whether the last copy was successful and still showing */
  copied: boolean;
  /** Whether a copy operation is in progress */
  copying: boolean;
  /** Error from the last copy attempt, if any */
  error: Error | null;
  /** Copy text to clipboard */
  copy: (text: string) => Promise<void>;
  /** Reset the copied/error state */
  reset: () => void;
}

/**
 * Custom hook for copying text to clipboard with proper error handling
 * and automatic cleanup of timeouts to prevent memory leaks.
 *
 * Features:
 * - Error handling for clipboard API failures (insecure context, permissions denied)
 * - Automatic reset of "copied" state after configurable duration
 * - Cleanup of timeouts on unmount to prevent memory leaks
 * - Fallback for older browsers using execCommand
 *
 * @example
 * const { copied, copy, error } = useClipboard();
 *
 * <button onClick={() => copy("text to copy")}>
 *   {copied ? "Copied!" : "Copy"}
 * </button>
 */
export function useClipboard(options: UseClipboardOptions = {}): UseClipboardReturn {
  const { successDuration = 2000, onSuccess, onError } = options;

  const [copied, setCopied] = useState(false);
  const [copying, setCopying] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  // Use ref to track timeout for cleanup
  const timeoutRef = useRef<NodeJS.Timeout | null>(null);

  // Cleanup timeout on unmount
  useEffect(() => {
    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, []);

  const reset = useCallback(() => {
    setCopied(false);
    setError(null);
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
      timeoutRef.current = null;
    }
  }, []);

  const copy = useCallback(
    async (text: string): Promise<void> => {
      // Clear any existing timeout
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
        timeoutRef.current = null;
      }

      setCopying(true);
      setError(null);
      setCopied(false);

      try {
        // Try modern clipboard API first
        if (navigator.clipboard && typeof navigator.clipboard.writeText === "function") {
          await navigator.clipboard.writeText(text);
        } else {
          // Fallback for older browsers or insecure contexts
          const textArea = document.createElement("textarea");
          textArea.value = text;
          textArea.style.position = "fixed";
          textArea.style.left = "-9999px";
          textArea.style.top = "-9999px";
          document.body.appendChild(textArea);
          textArea.focus();
          textArea.select();

          const success = document.execCommand("copy");
          document.body.removeChild(textArea);

          if (!success) {
            throw new Error("Fallback clipboard copy failed");
          }
        }

        setCopied(true);
        onSuccess?.();

        // Set timeout to reset copied state
        timeoutRef.current = setTimeout(() => {
          setCopied(false);
          timeoutRef.current = null;
        }, successDuration);
      } catch (err) {
        const error = err instanceof Error ? err : new Error("Failed to copy to clipboard");
        setError(error);
        onError?.(error);
        console.error("Clipboard copy failed:", error);
      } finally {
        setCopying(false);
      }
    },
    [successDuration, onSuccess, onError]
  );

  return { copied, copying, error, copy, reset };
}

/**
 * Simple copy function for one-off use cases where hook state isn't needed.
 * Includes error handling and console logging.
 *
 * @param text - Text to copy to clipboard
 * @returns Promise<boolean> - true if copy succeeded, false otherwise
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard && typeof navigator.clipboard.writeText === "function") {
      await navigator.clipboard.writeText(text);
      return true;
    }

    // Fallback for older browsers
    const textArea = document.createElement("textarea");
    textArea.value = text;
    textArea.style.position = "fixed";
    textArea.style.left = "-9999px";
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();

    const success = document.execCommand("copy");
    document.body.removeChild(textArea);

    return success;
  } catch (err) {
    console.error("Failed to copy to clipboard:", err);
    return false;
  }
}
