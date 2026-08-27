package auth

import (
	"testing"
)

// A refresh token is an identity handle, not an authorization grant. Before
// these tests existed, RefreshTokenPair re-minted the access token from the
// refresh token's own claims, and login-issued refresh tokens embed no role —
// so every refreshed browser login (Google or password) came back with role ""
// and hit the fail-closed member gate: the SDK's OAuth registration 403ed for
// every user after its first refresh (reproduced on production 2026-08-27).

// The principal supplied by the caller (read from the user record) is what the
// new access token AND the rotated refresh token carry — even when the old SDK
// refresh token embeds something else.
func TestRefreshTokenPair_MintsFromSuppliedPrincipal(t *testing.T) {
	cleanup := setupJWTTest(t)
	defer cleanup()
	svc := NewJWTService()

	sdkRefresh, err := svc.GenerateSDKRefreshToken("user-1", "org-1", "sdk@example.com", "admin")
	if err != nil {
		t.Fatalf("GenerateSDKRefreshToken: %v", err)
	}

	newAccess, newRefresh, err := svc.RefreshTokenPair(sdkRefresh, "db@example.com", "viewer")
	if err != nil {
		t.Fatalf("RefreshTokenPair: %v", err)
	}
	for name, tok := range map[string]string{"access": newAccess, "rotated refresh": newRefresh} {
		claims, err := svc.ValidateToken(tok)
		if err != nil {
			t.Fatalf("ValidateToken(%s): %v", name, err)
		}
		if claims.Role != "viewer" || claims.Email != "db@example.com" {
			t.Fatalf("%s token carries %q/%q, want the supplied principal viewer/db@example.com", name, claims.Role, claims.Email)
		}
	}
}

// The login-issuer variant: the role survives a refresh because the caller
// re-supplies it, not because the refresh token carried it.
func TestRefreshTokenPairPreservesUserRoleAndEmail(t *testing.T) {
	cleanup := setupJWTTest(t)
	defer cleanup()
	svc := NewJWTService()

	_, refresh, err := svc.GenerateTokenPair("user-1", "org-1", "person@example.com", "admin")
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}
	newAccess, _, err := svc.RefreshTokenPair(refresh, "person@example.com", "admin")
	if err != nil {
		t.Fatalf("RefreshTokenPair: %v", err)
	}
	claims, err := svc.ValidateToken(newAccess)
	if err != nil {
		t.Fatalf("ValidateToken(new access): %v", err)
	}
	if claims.Role != "admin" {
		t.Fatalf("refreshed access token lost the role: got %q, want %q", claims.Role, "admin")
	}
	if claims.Email != "person@example.com" {
		t.Fatalf("refreshed access token lost the email: got %q", claims.Email)
	}
}

// Pins the contract: a login-issued refresh token carries no authorization.
// Embedding a role here would move authorization into a 7-day credential that
// nothing revokes on demotion.
func TestGenerateTokenPair_RefreshTokenCarriesNoAuthorization(t *testing.T) {
	cleanup := setupJWTTest(t)
	defer cleanup()
	svc := NewJWTService()

	_, refresh, err := svc.GenerateTokenPair("user-1", "org-1", "person@example.com", "admin")
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}
	claims, err := svc.ValidateToken(refresh)
	if err != nil {
		t.Fatalf("ValidateToken(refresh): %v", err)
	}
	if claims.Role != "" || claims.Email != "" {
		t.Fatalf("refresh token must carry no role/email, got %q/%q", claims.Role, claims.Email)
	}
	if claims.UserID != "user-1" || claims.OrganizationID != "org-1" {
		t.Fatalf("refresh token must still identify the principal, got %q/%q", claims.UserID, claims.OrganizationID)
	}
}
