package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseUserInfo sets OAuthUser.EmailVerified from Google's verified_email and
// Okta's email_verified, so a caller can refuse to link or register an account
// by an email the provider has not verified. A claim that is missing, or is
// not a JSON boolean, reads as not verified, and the Microsoft case reads no
// claim.
func TestParseUserInfo_EmailVerified(t *testing.T) {
	service := NewOAuthService()

	cases := []struct {
		name     string
		provider OAuthProvider
		info     map[string]interface{}
		want     bool
	}{
		{
			name:     "google verified_email true",
			provider: ProviderGoogle,
			info:     map[string]interface{}{"id": "google-user-123", "email": "user@gmail.com", "verified_email": true},
			want:     true,
		},
		{
			name:     "google verified_email false",
			provider: ProviderGoogle,
			info:     map[string]interface{}{"id": "google-user-123", "email": "user@gmail.com", "verified_email": false},
			want:     false,
		},
		{
			name:     "google without the claim",
			provider: ProviderGoogle,
			info:     map[string]interface{}{"id": "google-user-123", "email": "user@gmail.com"},
			want:     false,
		},
		{
			name:     "google claim sent as a string",
			provider: ProviderGoogle,
			info:     map[string]interface{}{"id": "google-user-123", "email": "user@gmail.com", "verified_email": "true"},
			want:     false,
		},
		{
			name:     "okta email_verified true",
			provider: ProviderOkta,
			info:     map[string]interface{}{"sub": "okta-user-789", "email": "user@okta.com", "email_verified": true},
			want:     true,
		},
		{
			name:     "okta email_verified false",
			provider: ProviderOkta,
			info:     map[string]interface{}{"sub": "okta-user-789", "email": "user@okta.com", "email_verified": false},
			want:     false,
		},
		{
			name:     "okta without the claim",
			provider: ProviderOkta,
			info:     map[string]interface{}{"sub": "okta-user-789", "email": "user@okta.com"},
			want:     false,
		},
		{
			name:     "microsoft graph profile",
			provider: ProviderMicrosoft,
			info:     map[string]interface{}{"id": "ms-user-123", "userPrincipalName": "user@microsoft.com", "displayName": "Microsoft User"},
			want:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user, err := service.parseUserInfo(tc.provider, tc.info)
			require.NoError(t, err)
			assert.Equal(t, tc.want, user.EmailVerified)
		})
	}
}

// GetUserInfo sets the field from the JSON body the userinfo endpoint answers.
func TestGetUserInfo_GoogleEmailVerified(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"verified_email true", `{"id":"google-user-123","email":"user@gmail.com","verified_email":true}`, true},
		{"verified_email false", `{"id":"google-user-123","email":"user@gmail.com","verified_email":false}`, false},
		{"without the claim", `{"id":"google-user-123","email":"user@gmail.com"}`, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			service := &OAuthService{configs: map[OAuthProvider]*OAuthConfig{
				ProviderGoogle: {UserInfoURL: srv.URL},
			}}

			user, err := service.GetUserInfo(context.Background(), ProviderGoogle, "not-a-real-token")
			require.NoError(t, err)
			assert.Equal(t, "user@gmail.com", user.Email)
			assert.Equal(t, tc.want, user.EmailVerified)
		})
	}
}
