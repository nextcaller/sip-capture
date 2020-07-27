package collect

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics contains Prometheus metrics about SIP filtering, including the
// current filter and how many messages have been rejected and published.
type Metrics struct {
	Filter    *prometheus.GaugeVec
	Rejected  prometheus.Counter
	Published prometheus.Counter
	Dropped   prometheus.Counter
}

// NewMetrics creates a newly initialied Metrics.
func NewMetrics() *Metrics {
	m := &Metrics{
		Filter: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "msgs_filter_info",
			Help: "Constant, labeled with SIP filter setting",
		}, []string{"sip_filter"}),
		Rejected: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "msgs_rejected_total",
			Help: "Number of messages rejected by SIP filter",
		}),
		Published: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "msgs_published_total",
			Help: "Number of messages published to MQTT",
		}),
		Dropped: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "msgs_dropped_total",
			Help: "Number of messages dropped due to full publishing queue",
		}),
	}

	return m
}

// List the items contained with a metrics so they can be exposed via a
// prometheus.Registry.
func (m Metrics) List() []prometheus.Collector {
	return []prometheus.Collector{
		m.Filter,
		m.Rejected,
		m.Published,
		m.Dropped,
	}
}
