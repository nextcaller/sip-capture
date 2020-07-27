package extract

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/ip4defrag"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/matryer/is"
	"github.com/nextcaller/sip-capture/testhelpers"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rs/zerolog"
)

type firstErr struct {
	err error
}

func (e *firstErr) testCounter(c prometheus.Counter, name string, expected int) {
	if e.err != nil {
		return
	}
	if v := int(testutil.ToFloat64(c)); v != expected {
		e.err = fmt.Errorf("failed checking [%s], value %v, expected %v", name, v, expected)
	}
}

// helper to deal with prometheus metrics not being serializable for easy testing.
func testMetrics(expected map[string]int, m *Metrics) error {
	e := &firstErr{}
	for name, cnt := range expected {
		switch name {
		case "incoming":
			e.testCounter(m.Incoming, name, cnt)
		case "fragments":
			e.testCounter(m.Fragments, name, cnt)
		case "defrag":
			e.testCounter(m.Defrag, name, cnt)
		case "invalid":
			e.testCounter(m.Invalid, name, cnt)
		case "seen:udp":
			e.testCounter(m.Seen.WithLabelValues("udp"), name, cnt)
		case "seen:tcp":
			e.testCounter(m.Seen.WithLabelValues("tcp"), name, cnt)
		case "captured:udp":
			e.testCounter(m.Captured.WithLabelValues("udp"), name, cnt)
		case "captured:tcp":
			e.testCounter(m.Captured.WithLabelValues("tcp"), name, cnt)
		default:
			e.err = fmt.Errorf("don't know field %v", name)
		}
	}
	return e.err
}

func TestIPPacketAssembler(t *testing.T) {
	testCases := map[string]struct {
		input   string
		msgs    int
		metrics map[string]int
	}{
		"unfragmented": {
			"no-to-tag.pcap",
			1,
			map[string]int{
				"incoming":     1,
				"invalid":      0,
				"fragments":    0,
				"defrag":       0,
				"seen:udp":     1,
				"seen:tcp":     0,
				"captured:udp": 1,
				"captured:tcp": 0,
			},
		},
		"fragmented complete": {
			"sip-i.pcap",
			2,
			map[string]int{
				"incoming":     3,
				"invalid":      0,
				"fragments":    1, // request is fragmented
				"defrag":       1,
				"seen:udp":     2,
				"seen:tcp":     0,
				"captured:udp": 2, // request and response
				"captured:tcp": 0,
			},
		},
		"fragmented incomplete": {
			"sip-frag.pcap",
			0,
			map[string]int{
				"incoming":     1,
				"invalid":      0,
				"fragments":    1,
				"defrag":       0,
				"seen:udp":     0,
				"seen:tcp":     0,
				"captured:udp": 0,
				"captured:tcp": 0,
			},
		},
		"tcp stream": {
			"sip-tcp.pcap",
			30, // 5 complete TCP calls with INVTE/ACK/BYE req and Ringing/OK/OK resp
			map[string]int{
				"incoming":     30,
				"invalid":      0,
				"fragments":    0,
				"defrag":       0,
				"seen:udp":     0,
				"seen:tcp":     30,
				"captured:udp": 0,
				"captured:tcp": 30,
			},
		},
		"not sip": {
			"smtp.pcap",
			0,
			map[string]int{
				"incoming":     60,
				"invalid":      4, // has ICMP packets
				"fragments":    0,
				"defrag":       0,
				"seen:udp":     3,
				"seen:tcp":     53,
				"captured:udp": 0,
				"captured:tcp": 0,
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			is := is.New(t)
			buf := testhelpers.NewLogBuf()
			log := zerolog.New(buf)
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
			ctx := log.WithContext(context.Background())
			ext := NewExtracter(ip4defrag.NewIPv4Defragmenter())
			handle, err := pcap.OpenOffline(filepath.Join("testdata", tc.input))
			is.NoErr(err)
			defer handle.Close()
			source := gopacket.NewPacketSource(handle, handle.LinkType())

			msgs := make([]*layers.SIP, 0, 1000)
			accept := func(s *layers.SIP) error { msgs = append(msgs, s); return nil }

			done := make(chan bool)
			go func() {
				ext.Extract(ctx, source.Packets(), accept)
				done <- true
			}()

			select {
			case <-time.After(time.Second):
				t.Error(name, "timed out waiting for processing", buf.String())
			case <-done:
			}

			t.Log("[test]:", name, "[log]:", buf.String())
			captured := len(msgs)
			for _, x := range msgs {
				t.Log("[message]:", string(x.LayerContents())+string(x.LayerPayload()))
			}
			is.Equal(captured, tc.msgs)                    // count of captured vs expected
			is.NoErr(testMetrics(tc.metrics, ext.metrics)) // fields as expected
		})
	}
}
