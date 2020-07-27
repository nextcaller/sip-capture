package source

import (
	"fmt"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/prometheus/client_golang/prometheus"
)

// ClosableSource wraps a pcap.Handle and gopacket.PacketSource together into
// one unit which can deliver packets via Packets() and expose a Close() method
// to cleanly shut down.
type ClosableSource struct {
	handle  *pcap.Handle
	source  *gopacket.PacketSource
	metrics *Metrics
}

// Packets returns a channel of gopacket.Packets from the pcap source.
func (c *ClosableSource) Packets() chan gopacket.Packet {
	return c.source.Packets()
}

// Close stops the pcap handle which should in turn close the source.Packets()
// channel.
func (c *ClosableSource) Close() {
	c.handle.Close()
}

// Metrics returns a slice of prometheus.Collector items
// for exposing the interface and filter options via Prometheus.
func (c ClosableSource) Metrics() []prometheus.Collector { return c.metrics.List() }

// NewPCAP creates a ClosableSource with pcap configured for live capture with
// the appropriate filter.
func NewPCAP(iface string, filter string) (*ClosableSource, error) {
	handle, err := pcap.OpenLive(iface, 65535, true, pcap.BlockForever)
	if err != nil {
		return nil, fmt.Errorf("opening capture interface %v: %w", iface, err)
	}

	if err := handle.SetBPFFilter(filter); err != nil {
		return nil, fmt.Errorf("setting BPF filter to %v: %w", filter, err)
	}

	src := &ClosableSource{
		source:  gopacket.NewPacketSource(handle, handle.LinkType()),
		handle:  handle,
		metrics: NewMetrics(),
	}

	src.metrics.CapSource.WithLabelValues(iface, filter).Set(1)
	return src, nil
}

// Possible: use af_packet or pcapgo to avoid overhead of calling libpcap's C
// code from Go during runtime.  Use pcap only to compile BPF syntax into
// bpf.RawInstruction.  Remember to use `// +build linux` for this so it's
// portable-ish.

// Possible: find/write a way to compile net/bpf.RawInstruction from classic
// BPF syntax in Go without using libpcap, which would let us not have to link
// libpcap at all.
// Maybe https://github.com/cilium/ebpf via https://github.com/cloudflare/cbpfc ?
