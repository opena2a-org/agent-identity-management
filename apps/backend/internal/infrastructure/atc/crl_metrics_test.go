package atc

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

const thirdDID = "did:web:agents.example:third-agent"

// requireGauges fails the test unless reg gathers exactly the samples want,
// each written "<name> <value>", every family a gauge with one sample.
func requireGauges(t *testing.T, reg prometheus.Gatherer, want ...string) {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	var got []string
	for _, mf := range families {
		if mf.GetType().String() != "GAUGE" || len(mf.GetMetric()) != 1 {
			t.Fatalf("%s is a %s with %d samples, want a gauge with one", mf.GetName(), mf.GetType(), len(mf.GetMetric()))
		}
		got = append(got, fmt.Sprintf("%s %g", mf.GetName(), mf.GetMetric()[0].GetGauge().GetValue()))
	}
	sort.Strings(got)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("the collector gave %q, want %q", got, want)
	}
}

// The collector gives the seconds since the list held was last read and the
// entries of its revocations array, and no sample before a list is held. A
// scrape sends no request. A refresh answered 304 to the list's ETag is a
// read of the list held. A refused refresh is not, and the age passes 300
// while the list held is no longer answered from.
func TestCRLCollector_GivesTheAgeAndTheEntriesOfTheListHeld(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	a := newCRLAuthority(t, discoveryFor(signer))
	start := time.Now().UTC().Truncate(time.Second)
	now := start
	hourLong := func(at time.Time, dids ...string) []byte {
		doc := testList(at, dids...)
		doc["nextUpdate"] = at.Add(time.Hour).Format(time.RFC3339)
		return signList(t, doc, signer)
	}
	// An agentDid the list names twice is two entries.
	a.set(func(a *crlAuthority) { a.list, a.etag = hourLong(start, revokedDID, otherDID, revokedDID), `"list-1"` })
	c := newTestCRLClient(t, a, "", &now)
	reg := prometheus.NewPedanticRegistry()
	if err := reg.Register(NewCRLCollector(NewCRLRefresher(c))); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	requireGauges(t, reg)

	if err := c.Refresh(ctx); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	requireGauges(t, reg, "aim_atc_revocation_list_age_seconds 0", "aim_atc_revocation_list_entries 3")

	now = start.Add(90 * time.Second)
	requireGauges(t, reg, "aim_atc_revocation_list_age_seconds 90", "aim_atc_revocation_list_entries 3")
	if paths, _ := a.requests(); len(paths) != 1 || len(a.discoveryRequests()) != 1 {
		t.Fatalf("after one refresh and three scrapes the authority was asked for the list %d times and the discovery document %d times, want once each",
			len(paths), len(a.discoveryRequests()))
	}

	if err := c.Refresh(ctx); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if _, ifNoneMatch := a.requests(); len(ifNoneMatch) != 2 || ifNoneMatch[1] != `"list-1"` {
		t.Fatalf("the list requests carried If-None-Match %q, want the second to carry the list's ETag", ifNoneMatch)
	}
	requireGauges(t, reg, "aim_atc_revocation_list_age_seconds 0", "aim_atc_revocation_list_entries 3")

	a.set(func(a *crlAuthority) { a.listStatus, a.etag = http.StatusServiceUnavailable, "" })
	now = start.Add(90*time.Second + crlMaxStaleAge + time.Second)
	requireRefreshOutcome(t, c.Refresh(ctx), CRLOutcomeStatus)
	requireGauges(t, reg, "aim_atc_revocation_list_age_seconds 301", "aim_atc_revocation_list_entries 3")
	requireUnknown(t, c, revokedDID)

	a.set(func(a *crlAuthority) { a.listStatus, a.list = http.StatusOK, hourLong(now, thirdDID) })
	if err := c.Refresh(ctx); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	requireGauges(t, reg, "aim_atc_revocation_list_age_seconds 0", "aim_atc_revocation_list_entries 1")
	requireAnswer(t, c, thirdDID, true)
}

// The collector of a nil refresher collects none and describes the series of
// the collector of any refresher, so unregistering it removes that collector.
func TestCRLCollector_OfANilRefresherCollectsNone(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	a := newCRLAuthority(t, discoveryFor(signer))
	now := time.Now().UTC().Truncate(time.Second)
	a.setList(signList(t, testList(now, revokedDID), signer))
	c := newTestCRLClient(t, a, "", &now)
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	reg := prometheus.NewPedanticRegistry()
	if err := reg.Register(NewCRLCollector(nil)); err != nil {
		t.Fatal(err)
	}
	requireGauges(t, reg)
	if !reg.Unregister(NewCRLCollector(nil)) {
		t.Fatal("the collector of a nil refresher was not unregistered")
	}

	if err := reg.Register(NewCRLCollector(NewCRLRefresher(c))); err != nil {
		t.Fatal(err)
	}
	requireGauges(t, reg, "aim_atc_revocation_list_age_seconds 0", "aim_atc_revocation_list_entries 1")
	if !reg.Unregister(NewCRLCollector(nil)) {
		t.Fatal("unregistering the collector of a nil refresher left the collector of a refresher registered")
	}
	requireGauges(t, reg)
}

// The gauges pass the checks promtool check metrics runs
// (https://prometheus.io/docs/practices/naming/).
func TestCRLCollector_SeriesPassPromlint(t *testing.T) {
	signer := newEd25519Signer(t, testListKeyID)
	a := newCRLAuthority(t, discoveryFor(signer))
	now := time.Now().UTC().Truncate(time.Second)
	a.setList(signList(t, testList(now, revokedDID), signer))
	c := newTestCRLClient(t, a, "", &now)
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	collector := NewCRLCollector(NewCRLRefresher(c))

	problems, err := testutil.CollectAndLint(collector)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Errorf("%s: %s", p.Metric, p.Text)
	}
	if n := testutil.CollectAndCount(collector); n != 2 {
		t.Fatalf("the collector gave %d samples, want 2", n)
	}
}
