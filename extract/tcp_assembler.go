package extract

import (
	"bufio"
	"io"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"
	"github.com/nextcaller/sip-capture/sipsplitter"

	"github.com/rs/zerolog"
)

/*
  Things I know:
  - a single SIP message may be spread across multiple TCP packets.
  - TCP streams between SIP agents may be long lived.
  - Multiple SIP messages may appear in a single TCP stream.
  - a SIP message does not have to start on a TCP IP packet boundary.
  - Multiple SIP sessions may pass over the same TCP stream.
  - The world would be a much better place if everyone used SCTP.
  - If not SCTP, then at least self describing protocols using
    something like netstrings instead of putting content-length
	inside the message itself and having it only apply to part
	of the overall message. Grr.
*/

// SIPStreamFactory is used by a tcpassembly.StreamPool to create a new SIP
// extraction stream when a tcp flow begins.
type sipStreamFactory struct {
	accept  func(*layers.SIP) error
	metrics *Metrics
	log     zerolog.Logger
	trace   *sipsplitter.Trace
}

// newStreamFactory creates a SIPStreamFactory that's initialized with tracing functions.
func newStreamFactory(log zerolog.Logger, metrics *Metrics, accepter func(*layers.SIP) error) *sipStreamFactory {
	return &sipStreamFactory{
		metrics: metrics,
		log:     log,
		accept:  accepter,
		trace: &sipsplitter.Trace{
			Discard: func(d []byte) {
				log.Warn().Str("contents", string(d)).Msg("invalid SIP message discarded")
				metrics.Discarded.WithLabelValues("tcp").Inc()
			},
			NoStartLine: func() {
				log.Debug().Msg("no SIP request or status line found.")
				metrics.Incomplete.WithLabelValues("tcp").Inc()
			},
			NoHeaders: func() {
				log.Debug().Msg("incomplete SIP headers")
				metrics.Incomplete.WithLabelValues("tcp").Inc()
			},
			NoBody: func() {
				log.Debug().Msg("incomplete SIP body")
				metrics.Incomplete.WithLabelValues("tcp").Inc()
			},
			Complete: func(b []byte) {
				log.Debug().Str("SIP", string(b)).Msg("complete message found")
			},
		},
	}
}

// New creates a SIPStreamFactory that generates tcpassembly.Streams.  It uses
// tcpreader.ReaderStream provide the correct tcpassembly.Stream interface.
// The each stream runs scanStream in a goroutine to locate and extract
// individual SIP messages out of the TCP byte stream.
func (s *sipStreamFactory) New(net, transport gopacket.Flow) tcpassembly.Stream {
	log := s.log.With().Str("component", "sip-stream").Str("flow", transport.String()).Logger()
	r := tcpreader.NewReaderStream()
	go s.scanStream(&r, log)

	return &r
}

func (s *sipStreamFactory) scanStream(r io.Reader, log zerolog.Logger) {
	splitter := &sipsplitter.Splitter{
		ExitOnError: false,
		Trace:       s.trace,
	}

	sc := bufio.NewScanner(r)
	sc.Split(splitter.SplitSIP)

	for sc.Scan() {
		msg := layers.NewSIP()
		if err := msg.DecodeFromBytes(sc.Bytes(), gopacket.NilDecodeFeedback); err != nil {
			s.metrics.Discarded.WithLabelValues("tcp").Inc()
			log.Err(err).
				Bytes("sip", sc.Bytes()).
				Msg("error decoding tcp SIP layer bytes, skipping.")
			continue
		}
		if err := s.accept(msg); err != nil {
			log.Err(err).Msg("unable to accept TCP SIP message")
		}
		s.metrics.Captured.WithLabelValues("tcp").Inc()
	}
	if err := sc.Err(); err != nil {
		// This can only happen if the tcpreader stream is broken.
		log.Err(err).Msg("failed to fully scan tcp strem")
	}
}
