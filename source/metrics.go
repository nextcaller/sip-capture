package source

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics contains a Prometheus metric recording a packet source
// descriptor and BPF filter as labels on a constant gauge.
type Metrics struct {
	CapSource *prometheus.GaugeVec
}

// NewMetrics creates a new Metrics object.
func NewMetrics() *Metrics {
	m := &Metrics{
		CapSource: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "packets_source_info",
			Help: "Constant, labeled with BPF filter and capture interface",
		}, []string{"source", "bpf_filter"}),
	}

	return m
}

// List the items contained with a Metrics so that they can be exposed via a
// prometheus.Registry
func (m Metrics) List() []prometheus.Collector {
	return []prometheus.Collector{
		m.CapSource,
	}
}
