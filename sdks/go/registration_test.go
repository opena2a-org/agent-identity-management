package aimsdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSecureAlias verifies that Secure() is properly aliased to RegisterAgent()
func TestSecureAlias(t *testing.T) {
	t.Run("Secure calls RegisterAgent with same parameters", func(t *testing.T) {
		// Create mock HTTP server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify endpoint
			if r.URL.Path != "/api/v1/agents/register" {
				t.Errorf("Expected path /api/v1/agents/register, got %s", r.URL.Path)
			}

			// Return mock response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{
				"id": "test-agent-id",
				"name": "test-agent",
				"api_key": "test-api-key",
				"public_key": "test-public-key"
			}`))
		}))
		defer server.Close()

		// Create client
		client := NewClient(Config{
			APIURL: server.URL,
		})

		// Call Secure()
		ctx := context.Background()
		result, err := client.Secure(ctx, RegisterOptions{
			Name: "test-agent",
			Type: "ai_agent",
		})

		// Verify result
		if err != nil {
			t.Fatalf("Secure() failed: %v", err)
		}

		if result.ID != "test-agent-id" {
			t.Errorf("Expected ID test-agent-id, got %s", result.ID)
		}

		if result.Name != "test-agent" {
			t.Errorf("Expected name test-agent, got %s", result.Name)
		}
	})

	t.Run("Secure and RegisterAgent behave identically", func(t *testing.T) {
		// Create mock HTTP server
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{
				"id": "test-agent-id",
				"name": "test-agent",
				"api_key": "test-api-key",
				"public_key": "test-public-key"
			}`))
		}))
		defer server.Close()

		// Test with Secure()
		client1 := NewClient(Config{
			APIURL: server.URL,
		})

		ctx := context.Background()
		result1, err1 := client1.Secure(ctx, RegisterOptions{
			Name: "test-agent-1",
			Type: "ai_agent",
		})

		// Test with RegisterAgent()
		client2 := NewClient(Config{
			APIURL: server.URL,
		})

		result2, err2 := client2.RegisterAgent(ctx, RegisterOptions{
			Name: "test-agent-2",
			Type: "ai_agent",
		})

		// Verify both calls succeeded
		if err1 != nil {
			t.Errorf("Secure() failed: %v", err1)
		}
		if err2 != nil {
			t.Errorf("RegisterAgent() failed: %v", err2)
		}

		// Verify both returned same structure
		if result1 == nil || result2 == nil {
			t.Fatal("Expected both results to be non-nil")
		}

		// Verify both made the same API call (call count should be 2)
		if callCount != 2 {
			t.Errorf("Expected 2 API calls, got %d", callCount)
		}
	})

	t.Run("Secure handles errors same as RegisterAgent", func(t *testing.T) {
		// Create mock HTTP server that returns error
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "Internal server error"}`))
		}))
		defer server.Close()

		// Test error handling
		client := NewClient(Config{
			APIURL: server.URL,
		})

		ctx := context.Background()
		_, err := client.Secure(ctx, RegisterOptions{
			Name: "test-agent",
			Type: "ai_agent",
		})

		// Verify error is returned
		if err == nil {
			t.Error("Expected error, got nil")
		}
	})

	t.Run("Secure respects agent type parameter", func(t *testing.T) {
		// Create mock HTTP server
		var receivedType string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Parse request body to extract type
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
				if t, ok := payload["type"].(string); ok {
					receivedType = t
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{
				"id": "test-agent-id",
				"name": "test-agent",
				"api_key": "test-api-key",
				"public_key": "test-public-key"
			}`))
		}))
		defer server.Close()

		// Test with custom type
		client := NewClient(Config{
			APIURL: server.URL,
		})

		ctx := context.Background()
		_, err := client.Secure(ctx, RegisterOptions{
			Name: "test-agent",
			Type: "mcp_server",
		})

		if err != nil {
			t.Fatalf("Secure() failed: %v", err)
		}

		// Verify type was sent correctly
		if receivedType != "mcp_server" {
			t.Errorf("Expected type mcp_server, got %s", receivedType)
		}
	})

	t.Run("Secure uses default type when not specified", func(t *testing.T) {
		// Create mock HTTP server
		var receivedType string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Parse request body to extract type
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
				if t, ok := payload["type"].(string); ok {
					receivedType = t
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{
				"id": "test-agent-id",
				"name": "test-agent",
				"api_key": "test-api-key",
				"public_key": "test-public-key"
			}`))
		}))
		defer server.Close()

		// Test without specifying type
		client := NewClient(Config{
			APIURL: server.URL,
		})

		ctx := context.Background()
		_, err := client.Secure(ctx, RegisterOptions{
			Name: "test-agent",
		})

		if err != nil {
			t.Fatalf("Secure() failed: %v", err)
		}

		// Verify default type "ai_agent" was used
		if receivedType != "ai_agent" {
			t.Errorf("Expected default type ai_agent, got %s", receivedType)
		}
	})
}

// TestSecureWithOAuthAlias verifies that SecureWithOAuth() is properly aliased
func TestSecureWithOAuthAlias(t *testing.T) {
	t.Run("SecureWithOAuth calls RegisterAgentWithOAuth", func(t *testing.T) {
		// Note: Full OAuth flow testing requires browser interaction
		// This test just verifies the function exists and has the right signature

		// Create client
		client := NewClient(Config{
			APIURL: "http://localhost:8080",
		})

		// Verify method exists (compilation test)
		ctx := context.Background()
		opts := RegisterOptions{
			Name:          "test-agent",
			OAuthProvider: OAuthProviderGoogle,
		}

		// We can't actually call this without a full OAuth setup
		// But we can verify it exists and accepts correct parameters
		_ = client.SecureWithOAuth
		_ = ctx
		_ = opts
	})
}

// TestRegisterAgentDefaults verifies default parameter handling
func TestRegisterAgentDefaults(t *testing.T) {
	t.Run("RegisterAgent sets default type", func(t *testing.T) {
		// Create mock HTTP server
		var receivedType string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
				if t, ok := payload["type"].(string); ok {
					receivedType = t
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{
				"id": "test-agent-id",
				"name": "test-agent",
				"api_key": "test-api-key",
				"public_key": "test-public-key"
			}`))
		}))
		defer server.Close()

		// Test without type
		client := NewClient(Config{
			APIURL: server.URL,
		})

		ctx := context.Background()
		_, err := client.RegisterAgent(ctx, RegisterOptions{
			Name: "test-agent",
		})

		if err != nil {
			t.Fatalf("RegisterAgent() failed: %v", err)
		}

		// Verify default type was set
		if receivedType != "ai_agent" {
			t.Errorf("Expected default type ai_agent, got %s", receivedType)
		}
	})
}
