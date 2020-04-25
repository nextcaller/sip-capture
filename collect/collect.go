package collect

import (
	"context"
	"fmt"

	"github.com/google/gopacket/layers"
	"github.com/nextcaller/sip-capture/filters"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

type constError string

func (e constError) Error() string { return string(e) }

const (
	// ErrFull indicates that more outstanding messages await publishing than
	// the internal structure of the Collecter can support; the message passed
	// to Accept will not be published.
	ErrFull = constError("publish queue is full")
)

type publisher func(context.Context, *Msg) error

// Collecter receives incoming layers.SIP messages, discarding those that don't
// match the configured filter, and then publishes the accepted ones.
// It uses an internal channel to queue so that Accept won't block, making it
// suitable for use in a capture loop driven by gopacket.
type Collecter struct {
	metrics *Metrics
	match   filters.Filter
	publish publisher
	msgs    chan *layers.SIP
}

// NewCollecter returns a Collector that accepts messages that pass the match
// filter, then uses publish to emit them.  depth controls how many messages
// may be internally queued before discarding excess.
func NewCollecter(match filters.Filter, publish publisher, depth int) *Collecter {
	return &Collecter{
		match:   match,
		publish: publish,
		metrics: NewMetrics(),
		msgs:    make(chan *layers.SIP, depth),
	}
}

// Accept receives an incoming SIP message and enqueues it for filtering and
// publishing.  If for any reason the internal channel used for queueing is
// full, it will discard the message and return an error.
func (c *Collecter) Accept(sip *layers.SIP) error {
	select {
	case c.msgs <- sip:
		return nil
	default:
		c.metrics.Dropped.Inc()
		return fmt.Errorf("dropping message %v: %w", sip, ErrFull)
	}
}

// Publish blocks, consuming the internal queue, filtering out unwanted SIP
// messages, creating the appropriate JSON envelope and then publishes them
// using the provided publisher.
func (c *Collecter) Publish(ctx context.Context) {
	log := zerolog.Ctx(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case sip, ok := <-c.msgs:
			if sip == nil || !ok {
				log.Info().Msg("channel closed, accepter exiting")
				return
			}
			if !c.match(sip) {
				c.metrics.Rejected.Inc()
				log.Debug().Msg("discarding SIP message that does not match filter")
				continue
			}
			msg := NewMsg(sip)
			if err := c.publish(ctx, msg); err != nil {
				log.Err(err).Interface("msg", msg).Msg("publish failed")
			}
			c.metrics.Published.Inc()
		}
	}
}

// Metrics returns a list of prometheus.Collecter interfaces, suitable for
// passing to prometheus.Registry to export message collection metrics.
func (c *Collecter) Metrics() []prometheus.Collector { return c.metrics.List() }
