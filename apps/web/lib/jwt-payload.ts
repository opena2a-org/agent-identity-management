/**
 * Decodes the payload segment of a JWT for display and routing decisions only; nothing here
 * verifies the signature, and no authorization may rest on the result (the backend verifies).
 * JWT segments use the base64url alphabet, which atob() rejects; a token that cannot be
 * decoded yields null so callers treat it as invalid instead of crashing.
 */
/** The claims this dashboard reads from its own session token (apps/backend/internal/infrastructure/auth/jwt.go). */
export interface JwtClaims {
  user_id?: string;
  organization_id?: string;
  email?: string;
  role?: string;
  exp?: number;
  iat?: number;
  sub?: string;
  [claim: string]: unknown;
}

export function decodeJwtPayload(token: string | null | undefined): JwtClaims | null {
  const segment = token?.split(".")[1];
  if (!segment) return null;
  const standard = segment.replace(/-/g, "+").replace(/_/g, "/");
  try {
    const parsed: unknown = JSON.parse(atob(standard));
    return parsed && typeof parsed === "object" ? (parsed as JwtClaims) : null;
  } catch {
    return null;
  }
}
