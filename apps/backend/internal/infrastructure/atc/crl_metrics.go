package atc

import "github.com/prometheus/client_golang/prometheus"

// The series a CRLCollector gives, two gauges of the list held.
var (
	crlListAgeDesc = prometheus.NewDesc("aim_atc_revocation_list_age_seconds",
		"Seconds since the revocation list held was last read: by the refresh that held it, or by a later one answered 304 to its ETag.",
		nil, nil)
	crlListEntriesDesc = prometheus.NewDesc("aim_atc_revocation_list_entries",
		"Entries in the revocations array of the revocation list held.",
		nil, nil)
)

// CRLCollector gives the list a CRLRefresher's client holds as Prometheus
// gauges: aim_atc_revocation_list_age_seconds, the seconds since that list was
// last read, and aim_atc_revocation_list_entries, the entries of its
// revocations array, an agentDid named twice counted twice. It gives no sample
// while no list is held. It reads the list held and the client's clock, and
// sends no request.
type CRLCollector struct {
	refresher *CRLRefresher
}

// NewCRLCollector returns the collector of refresher. The collector of a nil
// refresher describes the same series and collects none, so unregistering it
// unregisters the collector of any refresher.
func NewCRLCollector(refresher *CRLRefresher) *CRLCollector {
	return &CRLCollector{refresher: refresher}
}

// Describe implements prometheus.Collector.
func (c *CRLCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- crlListAgeDesc
	ch <- crlListEntriesDesc
}

// Collect implements prometheus.Collector.
func (c *CRLCollector) Collect(ch chan<- prometheus.Metric) {
	if c.refresher == nil {
		return
	}
	client := c.refresher.client
	held := client.held.Load()
	if held == nil {
		return
	}
	ch <- prometheus.MustNewConstMetric(crlListAgeDesc, prometheus.GaugeValue, client.now().Sub(held.readAt).Seconds())
	ch <- prometheus.MustNewConstMetric(crlListEntriesDesc, prometheus.GaugeValue, float64(held.entries))
}
