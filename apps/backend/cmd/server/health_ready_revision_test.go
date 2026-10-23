package main

// GET /health/ready reports the Azure Container Apps revision the process runs
// in, as "revision": the value of CONTAINER_APP_REVISION, which config.Load
// reads at startup, and "" where that variable is unset or empty. A revision
// that changes only a secret binding serves the same commit as the revision
// it replaces; its name tells the two apart.

import (
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
)

// revisionTestName has the shape of a Container Apps revision name and names
// no real revision.
const revisionTestName = "ca-aim-backend-test--0000001"

// setServingRevision sets the revision the readiness body reports for the rest
// of the test, as main does from the loaded configuration, and restores the
// previous value once the test ends.
func setServingRevision(t *testing.T, revision string) {
	t.Helper()
	prev := servingRevision
	t.Cleanup(func() { servingRevision = prev })
	servingRevision = revision
}

// revisionReadyBody reads GET /health/ready, validates the body against the
// vendored contract and returns its top-level keys and values, with checkedAt
// and each latencyMs normalised to 0.
func revisionReadyBody(t *testing.T, schema *aim07Schema) map[string]any {
	t.Helper()
	_, _, body := aim07Get(t, aim07App(aim07CheckOK, nil))
	aim07Validate(t, schema, body)
	return aim07TopLevel(t, readinessClockFields.ReplaceAll(body, []byte(`"$1":0`)))
}

// unsetRevisionEnv unsets CONTAINER_APP_REVISION for the rest of the test and
// restores the value it had once the test ends.
func unsetRevisionEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CONTAINER_APP_REVISION", "")
	require.NoError(t, os.Unsetenv("CONTAINER_APP_REVISION"))
	_, set := os.LookupEnv("CONTAINER_APP_REVISION")
	require.False(t, set, "CONTAINER_APP_REVISION must be unset for this cell")
}

func TestHealthReadyRevision(t *testing.T) {
	schema := aim07LoadSchema(t)

	t.Run("CONTAINER_APP_REVISION set: revision is its value", func(t *testing.T) {
		t.Setenv("CONTAINER_APP_REVISION", revisionTestName)
		setServingRevision(t, config.LoadServer().Revision)
		body := revisionReadyBody(t, schema)
		revision, present := body["revision"]
		require.True(t, present, "the body must carry the revision key: %v", body)
		assert.Equal(t, revisionTestName, revision)
	})

	t.Run("CONTAINER_APP_REVISION unset: revision is the empty string", func(t *testing.T) {
		unsetRevisionEnv(t)
		setServingRevision(t, config.LoadServer().Revision)
		body := revisionReadyBody(t, schema)
		revision, present := body["revision"]
		require.True(t, present, "the body must carry the revision key when the variable is unset: %v", body)
		assert.Equal(t, "", revision)
	})

	t.Run("the variable moves revision and no other field", func(t *testing.T) {
		prev := buildSyncedFrom
		t.Cleanup(func() { buildSyncedFrom = prev })
		buildSyncedFrom = ""

		unsetRevisionEnv(t)
		setServingRevision(t, config.LoadServer().Revision)
		unset := revisionReadyBody(t, schema)
		t.Setenv("CONTAINER_APP_REVISION", revisionTestName)
		setServingRevision(t, config.LoadServer().Revision)
		set := revisionReadyBody(t, schema)

		keys := make([]string, 0, len(set))
		for key := range set {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		assert.Equal(t,
			[]string{"checkedAt", "commit", "degraded", "dependencies", "ready", "revision", "service", "version"},
			keys, "an unstamped build's body carries the seven required keys and revision")

		assert.Equal(t, revisionTestName, set["revision"])
		assert.Equal(t, "", unset["revision"])
		delete(set, "revision")
		delete(unset, "revision")
		assert.Equal(t, unset, set, "apart from revision the body is the same with the variable set and unset")
	})
}

// main sets the revision the readiness body reports from the configuration
// config.Load read, once, and before it registers the readiness route.
func TestMain_HandsTheReadinessRouteTheConfiguredRevision(t *testing.T) {
	mainFunc := funcDecl(t, "main.go", "main")

	var assigns []*ast.AssignStmt
	ast.Inspect(mainFunc.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if ok && len(assign.Lhs) == 1 && identName(assign.Lhs[0]) == "servingRevision" {
			assigns = append(assigns, assign)
		}
		return true
	})
	require.Len(t, assigns, 1, "main assigns servingRevision once")
	assert.Equal(t, token.ASSIGN, assigns[0].Tok, "main assigns the package variable; := would declare a local one")
	require.Len(t, assigns[0].Rhs, 1)
	assert.Equal(t, "cfg.Server.Revision", types.ExprString(assigns[0].Rhs[0]))

	handlers := callsIn(mainFunc.Body, "newHealthReadyHandler")
	require.Len(t, handlers, 1)
	assert.Less(t, assigns[0].Pos(), handlers[0].Pos(), "main sets servingRevision before it registers the readiness route")
}
