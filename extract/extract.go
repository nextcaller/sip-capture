package extract

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/tcpassembly"
	"github.com/nextcaller/sip-capture/defrag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

const (
	flushInterval = time.Second * 60
)

var (
	errIncomplete      = errors.New("incomplete ipv4 packet")
	errNoPacketBuilder = errors.New("unable to create packerbuilder interface")
)

// Defragmenter is the minimum parts of a gopacket/ip4defrag that we care
// about for packet handling.   Use this so we can accept either a default
// defragmenter or our custom degfragger which handles IP packets with shorter
// fragments than required by gopacket.
type Defragmenter interface {
	DiscardOlderThan(time.Time) int
	DefragIPv4(*layers.IPv4) (*layers.IPv4, error)
}

// Extracter converts incoming packets into gopacket *layers.SIP structs.
// It handles reassembling any IP fragments into whole packets, reassembling
// TCP message segments into a full stream, and then identifying and extracting
// SIP message data from any UDP packets or TCP streams.
type Extracter struct {
	metrics   *Metrics
	defragger Defragmenter
	flush     time.Duration
}

// NewExtracter creates a Extracter, using the given defragmenter.  If the
// defragmenter is nil, it instantiates a sip-capture/defrag, which handles
// short packet fragments, unlike the normal gopacket/ip4defrag.
func NewExtracter(defragger Defragmenter) *Extracter {
	if defragger == nil {
		defragger = defrag.NewIPv4Defragmenter()
	}
	p := &Extracter{
		defragger: defragger,
		flush:     flushInterval,
		metrics:   NewMetrics(),
	}
	return p
}

// Metrics returns a slice of prometheus.Collector objects that can be registered.
// to expose packet capture metrics via Prometheus.
func (e Extracter) Metrics() []prometheus.Collector { return e.metrics.List() }

// check if an ipv4 packet is a fragment
func someAssemblyRequired(ip4 *layers.IPv4) bool {
	return (ip4.Flags&layers.IPv4DontFragment == 0 &&
		(ip4.Flags&layers.IPv4MoreFragments != 0 || ip4.FragOffset != 0))
}

// attempt to reassemble an ipv4 packet if it's been fragmented.
func (e *Extracter) reassembleIPv4(ip4 *layers.IPv4) (*layers.IPv4, error) {
	l := ip4.Length
	if l < 28 && ip4.FragOffset > 0 {
		// gopacket believes this is an error; we have a custom defragmenter
		// that doesn't; track that it's happening to understand how often.
		e.metrics.ShortFrags.Inc()
	}

	nip4, err := e.defragger.DefragIPv4(ip4)
	if err != nil {
		return nil, fmt.Errorf("defragmenter failed: %w", err)
	} else if nip4 == nil {
		// not complete, but saved for future reassembly.
		return nil, errIncomplete
	}

	return nip4, nil
}

// reinsert a new defragged ip4 layer from assembleIPv4 back into a gopacket.
func reinsertIPv4(packet gopacket.Packet, ip4 *layers.IPv4) error {
	pb, ok := packet.(gopacket.PacketBuilder)
	if !ok {
		return errNoPacketBuilder
	}
	nDecoder := ip4.NextLayerType()
	if err := nDecoder.Decode(ip4.Payload, pb); err != nil {
		return fmt.Errorf("packet encoder failed: %w", err)
	}
	return nil
}

func (e *Extracter) rebuildPacket(packet gopacket.Packet, ip4 *layers.IPv4) error {
	// if we got called, we know we're part of a fragmented packet
	l := ip4.Length
	ip4, err := e.reassembleIPv4(ip4)
	if err != nil {
		// This may include errIncomplete, so let reassembleIPv4 handle metrics.
		return err
	}

	if ip4.Length != l {
		if err := reinsertIPv4(packet, ip4); err != nil {
			return fmt.Errorf("reinserting ip4 back into packet: %w", err)
		}
	}
	return nil
}

// Extract consumes gopackets.Packets from the packet channel, and produces all
// the capturable SIP messages as *layers.SIP objects into the msgs channel.
// It handles IPv4 packet defragmentation and TCP stream reassembly.
//
// Extract blocks and will not return until the context is canceled or the
// packets channel is closed.  Incomplete or otherwise defective packets are
// discarded without any sort of error, though they are recorded in metrics.
func (e *Extracter) Extract(ctx context.Context, packets <-chan gopacket.Packet, accept func(*layers.SIP) error) {
	log := zerolog.Ctx(ctx).With().Logger()
	ticker := time.NewTicker(e.flush)

	streamFactory := newStreamFactory(log, e.metrics, accept)
	streamPool := tcpassembly.NewStreamPool(streamFactory)
	assembler := tcpassembly.NewAssembler(streamPool)

	log = log.With().Str("component", "packet-source").Logger()

	for {
		select {
		case <-ctx.Done():
			return
		case packet, ok := <-packets:
			if packet == nil || !ok {
				flushed := assembler.FlushAll()
				log.Debug().Int("flushed", flushed).Msg("flushing tcp assembly")
				return
			}

			e.metrics.Incoming.Inc()

			if errlayer := packet.ErrorLayer(); errlayer != nil {
				e.metrics.Invalid.Inc()
				log.Err(errlayer.Error()).Str("packet", packet.String()).Msg("undecodable packet")
				continue
			}

			ip4, ok := packet.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
			if !ok {
				e.metrics.Invalid.Inc()
				log.Error().Str("packet", packet.String()).Msg("packet missing ipv4 layer")
				continue
			}

			if someAssemblyRequired(ip4) {
				err := e.rebuildPacket(packet, ip4)
				switch {
				case err == nil:
					// No err, packet is now updated with new assembled ipv4 layer.
					e.metrics.Defrag.Inc()
				case errors.Is(err, errIncomplete):
					e.metrics.Fragments.Inc()
					log.Debug().Msg("incomplete ipv4 fragment, continuing")
					continue
				default:
					e.metrics.BadDefrag.Inc()
					// Any error that isn't an incomplete packet gets reported
					log.Err(err).Str("packet", packet.String()).Msg("reassembling ipv4 packet")
					continue
				}
			}

			if packet.TransportLayer() == nil {
				// this is not TCP or UDP; probably ICMP.
				e.metrics.Invalid.Inc()
				log.Warn().Interface("next-layer", ip4.NextLayerType().String()).Msg("no transport layer after reassembly, adjust the BPF filter")
				continue
			}

			switch packet.TransportLayer().LayerType() {
			case layers.LayerTypeTCP:
				e.metrics.Seen.WithLabelValues("tcp").Inc()
				// send to reassembler, which will send to msgs channel on its own.
				log.Debug().Msg("sending tcp packet to assembler")
				tcplayer := packet.TransportLayer().(*layers.TCP)
				ip4 := packet.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
				assembler.AssembleWithTimestamp(ip4.NetworkFlow(), tcplayer, packet.Metadata().Timestamp)

			case layers.LayerTypeUDP:
				e.metrics.Seen.WithLabelValues("udp").Inc()
				sip, ok := packet.Layer(layers.LayerTypeSIP).(*layers.SIP)
				if !ok {
					// This UDP packet did not have identifiable SIP data in it.
					e.metrics.Discarded.WithLabelValues("udp").Inc()
					continue
				}
				// UDP SIP packets are complete.  Just do the thing now.
				e.metrics.Captured.WithLabelValues("udp").Inc()
				err := accept(sip)
				if err != nil {
					log.Err(err).Msg("unable to accept UDP sip packet")
				}

			default:
				// Since the TransportLayer check above will filter out stuff like ICMP,
				// this can only be sctp or rudp, according to gopacket.
				e.metrics.Seen.WithLabelValues("unknown").Inc()
				e.metrics.Discarded.WithLabelValues("unknown").Inc()
				log.Debug().Interface("layer-type", ip4.LayerType()).Msg("what type am I even getting?")
			}

		case <-ticker.C:
			// clean out IP fragments and TCP reassembly segments that are too
			// old to matter.  Even if we finally get matches, we're well past
			// caring about capturing them after 2 minutes.
			when := time.Now().Add(time.Minute * -2)
			assembler.FlushOlderThan(when)
			e.defragger.DiscardOlderThan(when)
		}
	}
}
