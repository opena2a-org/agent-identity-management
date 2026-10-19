package main

import (
	"context"
	"errors"
	"go/ast"
	"go/types"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/config"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// driftFixture stands in for the MCP capability repository and the webhook
// service the drift monitor reads through and sends through.
type driftFixture struct {
	mu sync.Mutex

	orgs    []uuid.UUID
	listErr error
	alerts  map[uuid.UUID][]repository.CapabilityDriftAlert
	readErr map[uuid.UUID]error
	sendErr error
	onRead  func(orgID uuid.UUID)

	listings int
	read     []uuid.UUID
	days     []int
	sent     []sentDrift
}

// sentDrift is one event the monitor asked the webhook service to send.
type sentDrift struct {
	orgID uuid.UUID
	event domain.WebhookEvent
	data  map[string]interface{}
}

func newDriftFixture(orgs ...uuid.UUID) *driftFixture {
	return &driftFixture{
		orgs:    orgs,
		alerts:  map[uuid.UUID][]repository.CapabilityDriftAlert{},
		readErr: map[uuid.UUID]error{},
	}
}

func (f *driftFixture) monitor() *capabilityDriftMonitor {
	return &capabilityDriftMonitor{
		listOrganizations: func(ctx context.Context) ([]uuid.UUID, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.listings++
			return f.orgs, f.listErr
		},
		drift:      f,
		webhooks:   f,
		dispatched: map[string]bool{},
	}
}

func (f *driftFixture) GetCapabilityDriftAlerts(orgID uuid.UUID, days int) ([]repository.CapabilityDriftAlert, *repository.CapabilityDriftStats, error) {
	f.mu.Lock()
	f.read = append(f.read, orgID)
	f.days = append(f.days, days)
	alerts, err, onRead := f.alerts[orgID], f.readErr[orgID], f.onRead
	f.mu.Unlock()
	if onRead != nil {
		onRead(orgID)
	}
	if err != nil {
		return nil, nil, err
	}
	return alerts, &repository.CapabilityDriftStats{TotalAlerts: len(alerts)}, nil
}

func (f *driftFixture) TriggerEvent(_ context.Context, orgID uuid.UUID, event domain.WebhookEvent, data map[string]interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sendErr != nil {
		return f.sendErr
	}
	f.sent = append(f.sent, sentDrift{orgID: orgID, event: event, data: data})
	return nil
}

func (f *driftFixture) listingCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.listings
}

// driftAlert is a drift of capability on server.
func driftAlert(server uuid.UUID, serverName, capability, driftType, severity string) repository.CapabilityDriftAlert {
	return repository.CapabilityDriftAlert{
		ID:             uuid.NewString(),
		MCPServerID:    server.String(),
		MCPServerName:  serverName,
		CapabilityName: capability,
		DriftType:      driftType,
		Severity:       severity,
	}
}

// Each drift is sent once, as security.capability_drift to the webhooks of
// the organization whose MCP server drifted, and a later run does not send it
// again. Every run reads every listed organization over the last day.
func TestCapabilityDriftMonitor_SendsEachDriftOnce(t *testing.T) {
	orgA, orgB := uuid.New(), uuid.New()
	serverA, serverB := uuid.New(), uuid.New()
	f := newDriftFixture(orgA, orgB)
	f.alerts[orgA] = []repository.CapabilityDriftAlert{
		driftAlert(serverA, "files-mcp", "write_file", "added", "medium"),
		driftAlert(serverA, "files-mcp", "read_file", "stale", "high"),
	}
	f.alerts[orgB] = []repository.CapabilityDriftAlert{
		driftAlert(serverB, "search-mcp", "web_search", "removed", "high"),
	}
	monitor := f.monitor()

	before := time.Now().UTC().Truncate(time.Second)
	monitor.check(context.Background())
	after := time.Now().UTC()

	require.Len(t, f.sent, 3)
	for _, sent := range f.sent {
		assert.Equal(t, domain.WebhookEventCapabilityDrift, sent.event)
	}
	assert.Equal(t, []uuid.UUID{orgA, orgA, orgB}, []uuid.UUID{f.sent[0].orgID, f.sent[1].orgID, f.sent[2].orgID})

	data := f.sent[0].data
	assert.Len(t, data, 5)
	assert.Equal(t, "files-mcp", data["serverName"])
	assert.Equal(t, "write_file", data["capabilityName"])
	assert.Equal(t, "added", data["driftType"])
	assert.Equal(t, "medium", data["severity"])
	raw, ok := data["detectedAt"].(string)
	require.True(t, ok, "detectedAt is an RFC 3339 string")
	detectedAt, err := time.Parse(time.RFC3339, raw)
	require.NoError(t, err)
	assert.False(t, detectedAt.Before(before) || detectedAt.After(after), "detectedAt %s is the time of the run", raw)
	assert.Equal(t, "removed", f.sent[2].data["driftType"])

	monitor.check(context.Background())
	assert.Len(t, f.sent, 3, "a later run does not send a drift again")
	assert.Equal(t, []uuid.UUID{orgA, orgB, orgA, orgB}, f.read)
	assert.Equal(t, []int{1, 1, 1, 1}, f.days, "each read covers the last day")
}

// A drift is the server, the capability and the drift type: the same
// capability drifting another way, or the same name on another server, is
// sent as a drift of its own, and a drift that appears after a run is sent by
// the next one.
func TestCapabilityDriftMonitor_SendsEachNewDrift(t *testing.T) {
	org := uuid.New()
	server, other := uuid.New(), uuid.New()
	f := newDriftFixture(org)
	f.alerts[org] = []repository.CapabilityDriftAlert{driftAlert(server, "files-mcp", "write_file", "added", "medium")}
	monitor := f.monitor()
	monitor.check(context.Background())
	require.Len(t, f.sent, 1)

	f.alerts[org] = append(f.alerts[org],
		driftAlert(server, "files-mcp", "write_file", "stale", "high"),
		driftAlert(other, "files-mcp-2", "write_file", "added", "medium"),
	)
	monitor.check(context.Background())

	require.Len(t, f.sent, 3)
	assert.Equal(t, "stale", f.sent[1].data["driftType"])
	assert.Equal(t, "files-mcp-2", f.sent[2].data["serverName"])
}

// A drift the webhook service did not take is sent again on the next run.
func TestCapabilityDriftMonitor_SendsAFailedDriftAgain(t *testing.T) {
	logs := captureLog(t)
	org := uuid.New()
	f := newDriftFixture(org)
	f.alerts[org] = []repository.CapabilityDriftAlert{driftAlert(uuid.New(), "files-mcp", "write_file", "added", "medium")}
	monitor := f.monitor()

	f.sendErr = errors.New("webhook lookup failed")
	monitor.check(context.Background())
	assert.Empty(t, f.sent)
	assert.Contains(t, logs.String(), "Drift monitor: webhook dispatch failed for org "+org.String()+": webhook lookup failed")

	f.sendErr = nil
	monitor.check(context.Background())
	assert.Len(t, f.sent, 1)
}

// An organization whose drift cannot be read is logged and skipped; the run
// goes on to the next one.
func TestCapabilityDriftMonitor_AnOrganizationThatFailsDoesNotStopTheRun(t *testing.T) {
	logs := captureLog(t)
	broken, fine := uuid.New(), uuid.New()
	f := newDriftFixture(broken, fine)
	f.readErr[broken] = errors.New("connection reset")
	f.alerts[fine] = []repository.CapabilityDriftAlert{driftAlert(uuid.New(), "files-mcp", "write_file", "added", "medium")}

	f.monitor().check(context.Background())

	assert.Equal(t, []uuid.UUID{broken, fine}, f.read)
	require.Len(t, f.sent, 1)
	assert.Equal(t, fine, f.sent[0].orgID)
	assert.Contains(t, logs.String(), "Drift monitor: failed to get drift alerts for org "+broken.String()+": connection reset")
}

// A failed listing is logged and the run reads nothing.
func TestCapabilityDriftMonitor_AFailedListingReadsNothing(t *testing.T) {
	logs := captureLog(t)
	f := newDriftFixture(uuid.New())
	f.listErr = errors.New("connection refused")

	f.monitor().check(context.Background())

	assert.Empty(t, f.read)
	assert.Contains(t, logs.String(), "Drift monitor: failed to list organizations: connection refused")
}

// Stopping the monitor ends a run before its next organization: the drift of
// the organization being read is still sent, the next one is not read, and a
// listing the stop cancelled is not logged as a failure.
func TestCapabilityDriftMonitor_StoppingEndsARunBeforeTheNextOrganization(t *testing.T) {
	logs := captureLog(t)
	orgA, orgB := uuid.New(), uuid.New()
	f := newDriftFixture(orgA, orgB)
	f.alerts[orgA] = []repository.CapabilityDriftAlert{driftAlert(uuid.New(), "files-mcp", "write_file", "added", "medium")}
	f.alerts[orgB] = []repository.CapabilityDriftAlert{driftAlert(uuid.New(), "search-mcp", "web_search", "removed", "high")}
	ctx, cancel := context.WithCancel(context.Background())
	f.onRead = func(uuid.UUID) { cancel() }

	f.monitor().check(ctx)

	assert.Equal(t, []uuid.UUID{orgA}, f.read, "the next organization is not read")
	require.Len(t, f.sent, 1)
	assert.Equal(t, orgA, f.sent[0].orgID)

	cancelled := newDriftFixture(uuid.New())
	cancelled.listErr = errors.New("pq: canceling statement due to user request")
	cancelled.monitor().check(ctx)
	assert.NotContains(t, logs.String(), "failed to list organizations")
}

// The started monitor runs on its interval with no request. Stopping it
// returns once its goroutine has exited: the stop line is written and no run
// starts afterwards. Calling stop again does nothing.
func TestCapabilityDriftMonitor_RunsOnItsIntervalUntilStopped(t *testing.T) {
	logs := captureLog(t)
	f := newDriftFixture(uuid.New())

	stop := startCapabilityDriftMonitor(f.monitor(), 5*time.Millisecond)
	require.Eventually(t, func() bool { return f.listingCount() >= 2 }, 10*time.Second, 5*time.Millisecond,
		"the monitor runs on its interval")
	stop()

	runs := f.listingCount()
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, runs, f.listingCount(), "no run starts after stop returns")
	assert.Contains(t, logs.String(), "Capability drift monitor started (runs every 5ms)")
	assert.Contains(t, logs.String(), "Capability drift monitor stopped")
	stop()
}

// A run through the MCP capability repository with an alert repository, as
// main wires it: reading an organization's drift records an alert for a drift
// that has no unacknowledged one, and the monitor sends the drift.
func TestCapabilityDriftMonitor_ARunRecordsTheDriftAlertAndSendsTheDrift(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	org, server, capability := uuid.New(), uuid.New(), uuid.New()
	capabilities := repository.NewMCPServerCapabilityRepository(db)
	capabilities.SetAlertRepository(repository.NewAlertRepository(db))

	mock.ExpectQuery(`c\.last_verified_at < NOW\(\)`).WithArgs(org, driftMonitorDays).
		WillReturnRows(sqlmock.NewRows([]string{"id", "mcp_server_id", "server_name", "name", "capability_type", "detected_at", "last_verified_at"}))
	mock.ExpectQuery(`c\.last_verified_at IS NULL`).WithArgs(org, driftMonitorDays).
		WillReturnRows(sqlmock.NewRows([]string{"id", "mcp_server_id", "server_name", "name", "capability_type", "detected_at"}).
			AddRow(capability.String(), server.String(), "files-mcp", "write_file", "tool", time.Now()))
	mock.ExpectQuery(`c\.is_active = false`).WithArgs(org, driftMonitorDays).
		WillReturnRows(sqlmock.NewRows([]string{"id", "mcp_server_id", "server_name", "name", "capability_type", "updated_at", "last_verified_at"}))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM alerts`).
		WithArgs(org, string(domain.AlertMCPCapabilityDrift), capability.String(), "added").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(`INSERT INTO alerts`).WillReturnResult(sqlmock.NewResult(0, 1))

	webhooks := newDriftFixture()
	monitor := &capabilityDriftMonitor{
		listOrganizations: func(context.Context) ([]uuid.UUID, error) { return []uuid.UUID{org}, nil },
		drift:             capabilities,
		webhooks:          webhooks,
		dispatched:        map[string]bool{},
	}
	monitor.check(context.Background())

	require.NoError(t, mock.ExpectationsWereMet(), "the run records one alert for the drift")
	require.Len(t, webhooks.sent, 1)
	assert.Equal(t, org, webhooks.sent[0].orgID)
	assert.Equal(t, "write_file", webhooks.sent[0].data["capabilityName"])
	assert.Equal(t, "added", webhooks.sent[0].data["driftType"])
}

// The organizations a run reads: every one with a registered MCP server, in
// one query.
func TestDriftMonitorOrganizations_ListsEveryOrganizationWithAnMCPServer(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	first, second := uuid.New(), uuid.New()
	mock.ExpectQuery(`SELECT DISTINCT organization_id FROM mcp_servers WHERE organization_id IS NOT NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"organization_id"}).AddRow(first.String()).AddRow(second.String()))

	orgs, err := driftMonitorOrganizations(db)(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{first, second}, orgs)
	require.NoError(t, mock.ExpectationsWereMet())
}

// intervalVariables reads interval variables from vars, "" for a name it does
// not hold, as config.JobsConfig.Value does.
func intervalVariables(vars map[string]string) func(string) string {
	return func(name string) string { return vars[name] }
}

func TestDriftMonitorInterval_ReadsTheVariable(t *testing.T) {
	cases := []struct {
		name  string
		value string
		set   bool
		want  time.Duration
		warns bool
	}{
		{name: "unset uses the 15-minute default", want: 15 * time.Minute},
		{name: "empty uses the 15-minute default", value: "", set: true, want: 15 * time.Minute},
		{name: "only spaces uses the 15-minute default", value: "   ", set: true, want: 15 * time.Minute},
		{name: "minutes", value: "5m", set: true, want: 5 * time.Minute},
		{name: "hours", value: "1h", set: true, want: time.Hour},
		{name: "surrounding spaces are ignored", value: " 30m ", set: true, want: 30 * time.Minute},
		{name: "the floor itself is used as given", value: "1m", set: true, want: time.Minute},
		{name: "under the floor runs at the floor", value: "30s", set: true, want: time.Minute, warns: true},
		{name: "zero uses the default", value: "0", set: true, want: 15 * time.Minute, warns: true},
		{name: "negative uses the default", value: "-5m", set: true, want: 15 * time.Minute, warns: true},
		{name: "a number without a unit uses the default", value: "15", set: true, want: 15 * time.Minute, warns: true},
		{name: "text uses the default", value: "often", set: true, want: 15 * time.Minute, warns: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vars := map[string]string{}
			if tc.set {
				vars["DRIFT_MONITOR_INTERVAL"] = tc.value
			}
			got, warning := driftMonitorInterval(intervalVariables(vars))
			assert.Equal(t, tc.want, got)
			if tc.warns {
				assert.Contains(t, warning, "DRIFT_MONITOR_INTERVAL", "the warning names the variable to fix")
				assert.Contains(t, warning, got.String(), "the warning states the interval in effect")
			} else {
				assert.Empty(t, warning)
			}
		})
	}
}

// The reader asks for DRIFT_MONITOR_INTERVAL only, and config.JobsConfig
// holds the monitor's interval under that name.
func TestDriftMonitorInterval_ReadsTheNameConfigHolds(t *testing.T) {
	var asked []string
	driftMonitorInterval(func(name string) string {
		asked = append(asked, name)
		return ""
	})
	assert.Equal(t, []string{"DRIFT_MONITOR_INTERVAL"}, asked)

	got, warning := driftMonitorInterval(config.JobsConfig{DriftMonitorInterval: "5m"}.Value)
	assert.Equal(t, 5*time.Minute, got)
	assert.Empty(t, warning)
}

// jobStartCall checks how main starts a background job: it reads the job's
// interval once with reader, from cfg.Jobs.Value, keeps the interval and the
// warning, logs the warning with logIntervalWarning, starts the job once with
// start at the interval it read, and defers the stop function start returns.
// It returns the start call.
func jobStartCall(t *testing.T, reader, start string) *ast.CallExpr {
	t.Helper()
	main := funcDecl(t, "main.go", "main")

	reads := callsIn(main.Body, reader)
	require.Len(t, reads, 1, "main calls %s once", reader)
	require.Len(t, reads[0].Args, 1)
	assert.Equal(t, "cfg.Jobs.Value", types.ExprString(reads[0].Args[0]),
		"%s reads the variable as config.Load read it", reader)

	var interval, warning string
	ast.Inspect(main.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if ok && len(assign.Rhs) == 1 && assign.Rhs[0] == reads[0] && len(assign.Lhs) == 2 {
			interval, warning = identName(assign.Lhs[0]), identName(assign.Lhs[1])
		}
		return true
	})
	require.NotContains(t, []string{"", "_"}, interval, "main keeps the interval %s returns", reader)
	require.NotContains(t, []string{"", "_"}, warning, "main keeps the warning %s returns", reader)

	logged := false
	for _, call := range callsIn(main.Body, "logIntervalWarning") {
		if len(call.Args) == 1 && identName(call.Args[0]) == warning {
			logged = true
		}
	}
	assert.True(t, logged, "main logs the warning %s returns", reader)

	starts := callsIn(main.Body, start)
	require.Len(t, starts, 1, "main calls %s once", start)
	require.Len(t, starts[0].Args, 2)
	assert.Equal(t, interval, identName(starts[0].Args[1]), "%s runs at the interval %s read", start, reader)

	var stop string
	ast.Inspect(main.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if ok && len(assign.Rhs) == 1 && assign.Rhs[0] == starts[0] && len(assign.Lhs) == 1 {
			stop = identName(assign.Lhs[0])
		}
		return true
	})
	require.NotContains(t, []string{"", "_"}, stop, "main keeps the stop function %s returns", start)
	deferred := false
	ast.Inspect(main.Body, func(n ast.Node) bool {
		d, ok := n.(*ast.DeferStmt)
		if ok && len(d.Call.Args) == 0 && identName(d.Call.Fun) == stop {
			deferred = true
		}
		return true
	})
	assert.True(t, deferred, "main defers %s, so the job stops when main returns", stop)
	return starts[0]
}

// jobLiteralFields returns the fields of the &typeName{...} literal e, each
// rendered as source.
func jobLiteralFields(t *testing.T, e ast.Expr, typeName string) map[string]string {
	t.Helper()
	unary, ok := e.(*ast.UnaryExpr)
	require.True(t, ok, "the job is passed as &%s{...}", typeName)
	lit, ok := unary.X.(*ast.CompositeLit)
	require.True(t, ok, "the job is passed as &%s{...}", typeName)
	require.Equal(t, typeName, identName(lit.Type))
	fields := map[string]string{}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		require.True(t, ok)
		fields[identName(kv.Key)] = types.ExprString(kv.Value)
	}
	return fields
}

// main starts the drift monitor at the interval DRIFT_MONITOR_INTERVAL sets,
// over the request pool, the MCP capability repository and the webhook
// service, and gives that repository the alert repository before it starts
// the monitor, so a run records the drift alerts.
func TestMain_StartsTheCapabilityDriftMonitor(t *testing.T) {
	start := jobStartCall(t, "driftMonitorInterval", "startCapabilityDriftMonitor")
	assert.Equal(t, map[string]string{
		"listOrganizations": "driftMonitorOrganizations(db)",
		"drift":             "repos.MCPCapability",
		"webhooks":          "services.Webhook",
		"dispatched":        "map[string]bool{}",
	}, jobLiteralFields(t, start.Args[0], "capabilityDriftMonitor"))

	main := funcDecl(t, "main.go", "main")
	var alertRepositorySet ast.Node
	for _, call := range callsIn(main.Body, "SetAlertRepository") {
		if types.ExprString(call.Fun) == "repos.MCPCapability.SetAlertRepository" &&
			len(call.Args) == 1 && types.ExprString(call.Args[0]) == "repos.Alert" {
			alertRepositorySet = call
		}
	}
	require.NotNil(t, alertRepositorySet, "main calls repos.MCPCapability.SetAlertRepository(repos.Alert)")
	assert.Less(t, alertRepositorySet.Pos(), start.Pos(), "the alert repository is set before the monitor starts")
}
