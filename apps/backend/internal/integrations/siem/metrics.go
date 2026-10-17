package siem

import "github.com/prometheus/client_golang/prometheus"

// The series a Collector gives. Each counts events and ends in _total, the
// suffix of a counter.
var (
	eventsSentDesc = prometheus.NewDesc("aim_siem_events_sent_total",
		"SIEM events an adapter accepted.",
		[]string{"adapter"}, nil)
	eventsFailedDesc = prometheus.NewDesc("aim_siem_events_failed_total",
		"SIEM events an adapter did not accept after every attempt. They are not sent again.",
		[]string{"adapter"}, nil)
	eventsDroppedDesc = prometheus.NewDesc("aim_siem_events_dropped_total",
		"SIEM events not queued, because the queue was full or forwarding was stopping. They are not sent.",
		nil, nil)
)

// Collector gives a Forwarder's counts as Prometheus counters:
// aim_siem_events_sent_total and aim_siem_events_failed_total for each
// adapter, and aim_siem_events_dropped_total.
type Collector struct {
	forwarder *Forwarder
}

// NewCollector returns the Collector of forwarder. The Collector of a nil
// Forwarder describes the same series and collects none, so unregistering it
// unregisters the Collector of any Forwarder.
func NewCollector(forwarder *Forwarder) *Collector {
	return &Collector{forwarder: forwarder}
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- eventsSentDesc
	ch <- eventsFailedDesc
	ch <- eventsDroppedDesc
}

// Collect implements prometheus.Collector. Adapters that share a name are
// counted as one, so the series stay unique.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	if c.forwarder == nil {
		return
	}
	var names []string
	sent, failed := map[string]int64{}, map[string]int64{}
	for _, stats := range c.forwarder.AdapterStats() {
		if _, seen := sent[stats.Adapter]; !seen {
			names = append(names, stats.Adapter)
		}
		sent[stats.Adapter] += stats.Sent
		failed[stats.Adapter] += stats.Failed
	}
	for _, name := range names {
		ch <- prometheus.MustNewConstMetric(eventsSentDesc, prometheus.CounterValue, float64(sent[name]), name)
		ch <- prometheus.MustNewConstMetric(eventsFailedDesc, prometheus.CounterValue, float64(failed[name]), name)
	}
	ch <- prometheus.MustNewConstMetric(eventsDroppedDesc, prometheus.CounterValue, float64(c.forwarder.Stats().Dropped))
}
