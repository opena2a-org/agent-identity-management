package siem

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gatherSeries registers c on a registry that checks each collected series
// against its description and returns every sample, keyed by name and labels.
// aim_siem_forwarding_failing is a gauge, and every other series a counter.
func gatherSeries(t *testing.T, c prometheus.Collector) map[string]float64 {
	t.Helper()
	reg := prometheus.NewPedanticRegistry()
	require.NoError(t, reg.Register(c))
	families, err := reg.Gather()
	require.NoError(t, err)
	samples := map[string]float64{}
	for _, family := range families {
		gauge := family.GetName() == "aim_siem_forwarding_failing"
		if gauge {
			assert.Equal(t, "GAUGE", family.GetType().String(), "%s is a gauge", family.GetName())
		} else {
			assert.Equal(t, "COUNTER", family.GetType().String(), "%s is a counter", family.GetName())
		}
		for _, metric := range family.GetMetric() {
			key := family.GetName()
			for _, label := range metric.GetLabel() {
				key += "{" + label.GetName() + "=" + label.GetValue() + "}"
			}
			if gauge {
				samples[key] = metric.GetGauge().GetValue()
			} else {
				samples[key] = metric.GetCounter().GetValue()
			}
		}
	}
	return samples
}

func TestCollector_CountsEachAdapterInEvents(t *testing.T) {
	adapter := &fakeAdapter{name: "fake", errs: []error{&StatusError{Service: "fake", StatusCode: 403}}}
	f := NewForwarder([]BatchAdapter{adapter}, testConfig(), nil)
	t.Cleanup(func() { closeForwarder(t, f) })
	collector := NewCollector(f)

	assert.Equal(t, map[string]float64{
		"aim_siem_events_sent_total{adapter=fake}":   0,
		"aim_siem_events_failed_total{adapter=fake}": 0,
		"aim_siem_events_dropped_total":              0,
		"aim_siem_forwarding_failing":                0,
	}, gatherSeries(t, collector), "each series is there before the first delivery")

	require.True(t, f.Enqueue(event("a", SeverityInfo)))
	require.True(t, f.Enqueue(event("b", SeverityInfo)))
	flushForwarder(t, f)
	require.True(t, f.Enqueue(event("c", SeverityInfo)))
	flushForwarder(t, f)

	assert.Equal(t, map[string]float64{
		"aim_siem_events_sent_total{adapter=fake}":   1,
		"aim_siem_events_failed_total{adapter=fake}": 2,
		"aim_siem_events_dropped_total":              0,
		"aim_siem_forwarding_failing":                0,
	}, gatherSeries(t, collector), "a refused batch of two, then one accepted event")
}

// The Collector gives the Forwarder's Health as the gauge
// aim_siem_forwarding_failing: 0 before the first delivery, 1 after a delivery
// the adapter did not accept, and 0 again after one it accepted.
func TestCollector_GivesTheFailingStateAsAGauge(t *testing.T) {
	adapter := &fakeAdapter{name: "fake", errs: []error{&StatusError{Service: "fake", StatusCode: 403}}}
	f := NewForwarder([]BatchAdapter{adapter}, testConfig(), nil)
	t.Cleanup(func() { closeForwarder(t, f) })
	collector := NewCollector(f)
	failing := func() float64 {
		t.Helper()
		value, ok := gatherSeries(t, collector)["aim_siem_forwarding_failing"]
		require.True(t, ok, "the Collector gives aim_siem_forwarding_failing")
		return value
	}

	assert.Equal(t, float64(0), failing(), "before the first delivery")

	require.True(t, f.Enqueue(event("a", SeverityInfo)))
	flushForwarder(t, f)
	require.True(t, f.Health().Failing)
	assert.Equal(t, float64(1), failing(), "the adapter did not accept the last delivery")

	require.True(t, f.Enqueue(event("b", SeverityInfo)))
	flushForwarder(t, f)
	require.False(t, f.Health().Failing)
	assert.Equal(t, float64(0), failing(), "the adapter accepted the last delivery")
}

func TestCollector_CountsADropWithAQueueOfOne(t *testing.T) {
	adapter := &fakeAdapter{name: "fake", entered: make(chan struct{}, 4), block: make(chan struct{})}
	cfg := testConfig()
	cfg.QueueSize = 1
	cfg.BatchSize = 1
	f := NewForwarder([]BatchAdapter{adapter}, cfg, nil)
	t.Cleanup(func() { closeForwarder(t, f) })

	require.True(t, f.Enqueue(event("a", SeverityInfo)))
	<-adapter.entered // the worker holds "a" and waits on the adapter
	require.True(t, f.Enqueue(event("b", SeverityInfo)), "the queue holds one event")
	require.False(t, f.Enqueue(event("c", SeverityInfo)), "the queue is full")

	dropped := gatherSeries(t, NewCollector(f))
	assert.Equal(t, float64(1), dropped["aim_siem_events_dropped_total"])
	assert.Equal(t, float64(1), dropped["aim_siem_forwarding_failing"], "an event was dropped after the last accepted delivery")

	close(adapter.block)
	flushForwarder(t, f)
	assert.Equal(t, map[string]float64{
		"aim_siem_events_sent_total{adapter=fake}":   2,
		"aim_siem_events_failed_total{adapter=fake}": 0,
		"aim_siem_events_dropped_total":              1,
		"aim_siem_forwarding_failing":                0,
	}, gatherSeries(t, NewCollector(f)), "the deliveries after the drop were accepted")
}

func TestCollector_AdaptersThatShareANameAreOneSeries(t *testing.T) {
	first, second := &fakeAdapter{name: "splunk"}, &fakeAdapter{name: "splunk"}
	f := NewForwarder([]BatchAdapter{first, second}, testConfig(), nil)
	t.Cleanup(func() { closeForwarder(t, f) })

	require.True(t, f.Enqueue(event("a", SeverityInfo)))
	flushForwarder(t, f)

	assert.Equal(t, map[string]float64{
		"aim_siem_events_sent_total{adapter=splunk}":   2,
		"aim_siem_events_failed_total{adapter=splunk}": 0,
		"aim_siem_events_dropped_total":                0,
		"aim_siem_forwarding_failing":                  0,
	}, gatherSeries(t, NewCollector(f)))
}

func TestCollector_OfNoForwarderCollectsNothingAndUnregistersAnyOther(t *testing.T) {
	f := NewForwarder([]BatchAdapter{&fakeAdapter{name: "fake"}}, testConfig(), nil)
	t.Cleanup(func() { closeForwarder(t, f) })

	assert.Empty(t, gatherSeries(t, NewCollector(nil)))

	reg := prometheus.NewRegistry()
	require.NoError(t, reg.Register(NewCollector(f)))
	assert.True(t, reg.Unregister(NewCollector(nil)), "the series are described alike, whatever the forwarder")
	families, err := reg.Gather()
	require.NoError(t, err)
	assert.Empty(t, families)
}
