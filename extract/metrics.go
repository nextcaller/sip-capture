package extract

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics contains all the observability data for the packet capture process,
// in the form of Prometheus metrics.
type Metrics struct {
	Incoming   prometheus.Counter
	Invalid    prometheus.Counter
	Fragments  prometheus.Counter
	ShortFrags prometheus.Counter
	BadDefrag  prometheus.Counter
	Defrag     prometheus.Counter

	Seen       *prometheus.CounterVec
	Incomplete *prometheus.CounterVec
	Discarded  *prometheus.CounterVec
	Captured   *prometheus.CounterVec
}

// NewMetrics creates, but does not register, a set of Prometheus.Collector metrics.
// Use Metrics.Register to add this to a Prometheus registry to expose the metrics.
func NewMetrics() *Metrics {
	m := &Metrics{
		Incoming: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "packets_incoming_total",
			Help: "incoming packets after bpf filtering",
		}),
		Invalid: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "packets_invalid_total",
			Help: "packets with invalid transport or network layers",
		}),
		Fragments: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "packets_fragment_total",
			Help: "packet fragments (IP-level)",
		}),
		ShortFrags: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "packets_short_fragment_total",
			Help: "packet fragments with under minimum spec length",
		}),
		BadDefrag: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "packets_defragment_failed_total",
			Help: "IP packet reassembly failure",
		}),
		Defrag: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "packets_defragmented_total",
			Help: "packet fragments successfully reassembled into whole packets",
		}),
		Seen: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "msgs_seen_total",
			Help: "SIP messages encountered",
		}, []string{"transport"}),
		Incomplete: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "msgs_incomplete_total",
			Help: "SIP messages ignored as incomplete",
		}, []string{"transport"}),
		Discarded: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "msgs_discarded_total",
			Help: "SIP messages discarded as unparseable",
		}, []string{"transport"}),
		Captured: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "msgs_captured_total",
			Help: "SIP messages successfully prepared for capture",
		}, []string{"transport"}),
	}

	// Ensure our important transport labels are 0 filled so they always show
	// up, even if they haven't yet received data.
	for _, s := range []string{"udp", "tcp"} {
		m.Seen.WithLabelValues(s)
		m.Incomplete.WithLabelValues(s)
		m.Discarded.WithLabelValues(s)
		m.Captured.WithLabelValues(s)
	}

	return m
}

// List returns a slice containing each Prometheus metric, for adding to a prometheus.Registry.
func (m Metrics) List() []prometheus.Collector {
	return []prometheus.Collector{
		m.Incoming,
		m.Invalid,
		m.Fragments,
		m.ShortFrags,
		m.BadDefrag,
		m.Seen,
		m.Incomplete,
		m.Discarded,
		m.Captured,
	}
}
