import { api } from "./api";

/**
 * Downloads the SDK for a specific agent
 * @param agentId - The ID of the agent
 * @param agentName - The name of the agent (used for filename)
 * @param language - The programming language for the SDK (default: python)
 * @throws Error if download fails
 */
export async function downloadSDK(
  agentId: string,
  agentName: string,
  language: 'python' | 'nodejs' | 'go' = 'python'
): Promise<void> {
  const token = api.getToken();
  if (!token) {
    throw new Error('Not authenticated');
  }

  // Get runtime-detected API URL from api client's baseURL
  const apiBaseURL = (api as any).baseURL || process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

  const response = await fetch(
    `${apiBaseURL}/api/v1/agents/${agentId}/sdk?lang=${language}`,
    {
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${token}`,
      },
    }
  );

  if (!response.ok) {
    throw new Error(`Failed to download SDK: ${response.statusText}`);
  }

  // Extract filename from Content-Disposition header (includes version)
  const contentDisposition = response.headers.get("Content-Disposition");
  let filename = `aim-sdk-${language}.zip`; // fallback
  if (contentDisposition) {
    const match = contentDisposition.match(/filename=([^;]+)/);
    if (match) {
      filename = match[1].replace(/"/g, "").trim();
    }
  }

  // Download the file with versioned filename from backend
  const blob = await response.blob();
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  window.URL.revokeObjectURL(url);
  document.body.removeChild(a);
}
