import { api } from "./api";

// The API URL the client calls, resolved by getApiUrl in lib/api.ts the first time it is
// read and kept by the client. Code that builds its own request URLs reads it here, so
// it reaches the API the client signs in to.
//
// It reads the client's baseURL getter, as components/modals/register-agent-modal.tsx
// and agent-detail-modal.tsx do, rather than a new export of lib/api.ts: lib/api.ts is
// on the built-in protected list of scripts/sync-to-cloud.sh.
//
// Browser only, like getApiUrl, which throws when called during server rendering.
export function apiBaseUrl(): string {
  return (api as unknown as { baseURL: string }).baseURL;
}
