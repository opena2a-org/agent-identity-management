package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A new FGA engine holds no Registry ASC address. The server sets one only
// from FGA_REGISTRY_ASC_URL, so a server started without it, as the
// quickstart starts it, names no Registry in its FGA engine.
func TestNewFGAEngine_HoldsNoRegistryASCURL(t *testing.T) {
	engine := NewFGAEngine(nil, nil, nil)
	t.Cleanup(func() { _ = engine.Shutdown(context.Background()) })

	assert.Empty(t, engine.registryASCURL, "a new FGA engine holds a Registry ASC address that FGA_REGISTRY_ASC_URL did not set")
}

// The control: SetRegistryASCURL, which the server calls with
// FGA_REGISTRY_ASC_URL when it is set, sets the address.
func TestFGAEngine_SetRegistryASCURLSetsTheAddress(t *testing.T) {
	engine := NewFGAEngine(nil, nil, nil)
	t.Cleanup(func() { _ = engine.Shutdown(context.Background()) })

	engine.SetRegistryASCURL("https://registry.example.com/asc")
	assert.Equal(t, "https://registry.example.com/asc", engine.registryASCURL)
}
