package main

import (
	"go/ast"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// servicesFields returns the fields of the &Services{...} literal fn returns,
// each with the identifier it is set to.
func servicesFields(t *testing.T, fn *ast.FuncDecl) map[string]string {
	t.Helper()
	fields := map[string]string{}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok || identName(lit.Type) != "Services" {
			return true
		}
		for _, elt := range lit.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				fields[identName(kv.Key)] = identName(kv.Value)
			}
		}
		return false
	})
	require.NotEmpty(t, fields, "initServices returns no &Services{...} literal")
	return fields
}

// The alert, agent and API key services the server hands to its handlers send
// their webhook events through the webhook service the server builds, which
// bounds the delivery, and so do the trust calculator, the capability service
// and the drift detection service the verification event service runs, which
// send trust_score.changed for the scores they store.
func TestInitServices_SetsTheWebhookServiceOnTheEventServices(t *testing.T) {
	initServices := funcDecl(t, "main.go", "initServices")

	var webhookServiceBuilt bool
	ast.Inspect(initServices.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || identName(assign.Lhs[0]) != "webhookService" || len(assign.Rhs) != 1 {
			return true
		}
		if call, ok := assign.Rhs[0].(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "NewWebhookService" {
				webhookServiceBuilt = true
			}
		}
		return true
	})
	require.True(t, webhookServiceBuilt, "initServices builds webhookService with application.NewWebhookService")

	receivers := map[string]bool{}
	for _, call := range callsIn(initServices.Body, "SetWebhookService") {
		sel, ok := call.Fun.(*ast.SelectorExpr)
		require.True(t, ok)
		require.Len(t, call.Args, 1)
		assert.Equal(t, "webhookService", identName(call.Args[0]),
			"%s.SetWebhookService is given the server's webhook service", identName(sel.X))
		receivers[identName(sel.X)] = true
	}
	assert.Equal(t, map[string]bool{
		"alertService": true, "agentService": true, "apiKeyService": true,
		"trustCalculator": true, "capabilityService": true, "driftDetectionService": true,
	}, receivers)

	var driftDetectionUsed bool
	for _, call := range callsIn(initServices.Body, "NewVerificationEventService") {
		require.Len(t, call.Args, 3)
		driftDetectionUsed = identName(call.Args[2]) == "driftDetectionService"
	}
	assert.True(t, driftDetectionUsed, "the verification event service runs the drift detection service given the webhook service")

	fields := servicesFields(t, initServices)
	assert.Equal(t, "alertService", fields["Alert"])
	assert.Equal(t, "agentService", fields["Agent"])
	assert.Equal(t, "trustCalculator", fields["Trust"])
	assert.Equal(t, "capabilityService", fields["Capability"])
	assert.Equal(t, "apiKeyService", fields["APIKey"])
	assert.Equal(t, "webhookService", fields["Webhook"])
}

// Shutdown stops the webhook service, within webhookDrainTimeout, after the
// API listener has stopped taking requests.
func TestMain_StopsTheWebhookServiceAfterTheListener(t *testing.T) {
	main := funcDecl(t, "main.go", "main")

	var listenerStop, webhookStop ast.Node
	ast.Inspect(main.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			if fn.Name == "shutdownGracefully" {
				listenerStop = call
			}
		case *ast.SelectorExpr:
			owner, ok := fn.X.(*ast.SelectorExpr)
			if fn.Sel.Name == "Stop" && ok && identName(owner.X) == "services" && owner.Sel.Name == "Webhook" {
				webhookStop = call
				require.Len(t, call.Args, 1)
				assert.Equal(t, "webhookDrainTimeout", identName(call.Args[0]))
			}
		}
		return true
	})
	require.NotNil(t, listenerStop, "main calls shutdownGracefully")
	require.NotNil(t, webhookStop, "main calls services.Webhook.Stop")
	assert.Less(t, listenerStop.Pos(), webhookStop.Pos(), "the webhook service stops after the listener")
}
