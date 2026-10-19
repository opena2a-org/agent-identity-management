package main

// The Registry bridge is opt-in. A server started with no REGISTRY_ or ATC_
// variable, as the quickstart starts it, builds no bridge and starts no bridge
// job. With REGISTRY_BRIDGE_ENABLED=true it builds the bridge and starts the
// job.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
)

// loadConfigWithoutRegistryVariables loads the configuration with the baseline
// of the environment table, every REGISTRY_ and ATC_ variable unset, and then
// the variables in set.
func loadConfigWithoutRegistryVariables(t *testing.T, set map[string]string) *config.Config {
	t.Helper()
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "REGISTRY_") || strings.HasPrefix(name, "ATC_") {
			// Setenv first, so the variable is restored after the unset.
			t.Setenv(name, "")
			require.NoError(t, os.Unsetenv(name))
		}
	}
	for name, value := range readEnvironmentTable(t).Baseline {
		t.Setenv(name, value)
	}
	for name, value := range set {
		t.Setenv(name, value)
	}
	cfg, err := config.Load()
	require.NoError(t, err)
	return cfg
}

func TestRegistryBridge_NoRegistryVariablesBuildNoBridgeAndStartNoJob(t *testing.T) {
	cfg := loadConfigWithoutRegistryVariables(t, nil)

	bridge := newRegistryBridgeService(cfg.Registry, &Repositories{})
	assert.Nil(t, bridge, "with no REGISTRY_ variable set the server built a Registry bridge")

	lines := logLines("Registry bridge", func() {
		stop := startRegistryBridgeJob(bridge)
		stop()
	})
	assert.Empty(t, lines, "with no Registry bridge the server started the bridge job")
}

// The negative control: the same environment with REGISTRY_BRIDGE_ENABLED=true.
func TestRegistryBridge_EnabledBuildsTheBridgeAndStartsItsJob(t *testing.T) {
	cfg := loadConfigWithoutRegistryVariables(t, map[string]string{"REGISTRY_BRIDGE_ENABLED": "true"})

	bridge := newRegistryBridgeService(cfg.Registry, &Repositories{})
	require.NotNil(t, bridge, "REGISTRY_BRIDGE_ENABLED=true built no Registry bridge")

	lines := logLines("Registry bridge", func() {
		stop := startRegistryBridgeJob(bridge)
		stop()
	})
	assert.Equal(t, "Registry bridge enabled (runs every 6 hours)\nRegistry bridge stopped", lines)
}

// registryBridgeConstructions returns "file: function" for each call to
// NewRegistryBridgeService in the non-test files of this package, with
// "package scope" for a call outside a function.
func registryBridgeConstructions(t *testing.T) []string {
	t.Helper()
	names, err := filepath.Glob("*.go")
	require.NoError(t, err)
	var found []string
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		require.NoError(t, err)
		for _, decl := range file.Decls {
			scope := "package scope"
			if fn, ok := decl.(*ast.FuncDecl); ok {
				scope = fn.Name.Name
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "NewRegistryBridgeService" {
					found = append(found, name+": "+scope)
				}
				return true
			})
		}
	}
	return found
}

// The two tests above hold for the server main starts: the bridge is built
// only in newRegistryBridgeService, initServices sets Services.RegistryBridge
// from it with the Registry settings config.Load read, and main hands
// services.RegistryBridge to startRegistryBridgeJob.
func TestMain_StartsTheRegistryBridgeJobOnlyForTheBridgeInitServicesBuilt(t *testing.T) {
	assert.Equal(t, []string{"main.go: newRegistryBridgeService"}, registryBridgeConstructions(t),
		"the Registry bridge is built outside newRegistryBridgeService, which builds it only with REGISTRY_BRIDGE_ENABLED=true")

	initServices := funcDecl(t, "main.go", "initServices")
	var built int
	ast.Inspect(initServices.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || identName(assign.Lhs[0]) != "registryBridgeService" || len(assign.Rhs) != 1 {
			return true
		}
		built++
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		require.True(t, ok, "initServices assigns registryBridgeService from a call")
		assert.Equal(t, "newRegistryBridgeService", identName(call.Fun))
		require.Len(t, call.Args, 2)
		settings, ok := call.Args[0].(*ast.SelectorExpr)
		require.True(t, ok, "initServices builds the bridge from a field of cfg")
		assert.Equal(t, "cfg", identName(settings.X))
		assert.Equal(t, "Registry", settings.Sel.Name)
		return true
	})
	assert.Equal(t, 1, built, "initServices assigns registryBridgeService once")
	assert.Equal(t, "registryBridgeService", servicesFields(t, initServices)["RegistryBridge"])

	main := funcDecl(t, "main.go", "main")
	var started int
	ast.Inspect(main.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || identName(call.Fun) != "startRegistryBridgeJob" {
			return true
		}
		started++
		require.Len(t, call.Args, 1)
		bridge, ok := call.Args[0].(*ast.SelectorExpr)
		require.True(t, ok, "main hands startRegistryBridgeJob a field of services")
		assert.Equal(t, "services", identName(bridge.X))
		assert.Equal(t, "RegistryBridge", bridge.Sel.Name)
		return true
	})
	assert.Equal(t, 1, started, "main calls startRegistryBridgeJob once")
}
