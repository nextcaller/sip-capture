package collect

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/matryer/is"
)

func loadSIP(is *is.I, file string) *layers.SIP {
	data, err := ioutil.ReadFile(filepath.Join("testdata", file))
	is.NoErr(err) // loaded sip from file.
	sip := layers.NewSIP()
	err = sip.DecodeFromBytes(data, gopacket.NilDecodeFeedback)
	is.NoErr(err) // can load test SIP packet
	return sip
}

func TestFunction(t *testing.T) {
	testCases := map[string]struct {
		sourceFile string
		expectedID string
	}{
		"no-call-id": {
			"sip_packet_no_call_id.txt",
			"1512482db8d96c1c",
		},
		"call-id header": {
			"sip_packet.txt",
			"12345678@foo.com",
		},
		"call-id short header": {
			"sip_packet_short_call_id.txt",
			"12345678@foo.com",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			is := is.New(t)
			sip := loadSIP(is, tc.sourceFile)

			msg := NewMsg(sip)

			t.Logf("[test:%s] [id:%v] %+v", name, sip.GetFirstHeader("Call-ID"), msg)
			is.Equal(msg.ID, tc.expectedID)                                        // MsgID should match
			is.True(bytes.Contains(msg.SIPData, []byte("INVITE foo@bar SIP/2.0"))) // SIPData contains expected invite.

		})
	}
}

func BenchmarkNewMsg(b *testing.B) {
	is := is.New(b)
	sip := loadSIP(is, "sip_packet.txt")
	for i := 0; i < b.N; i++ {
		NewMsg(sip)
	}
}

func BenchmarkNewMsgHashed(b *testing.B) {
	is := is.New(b)
	sip := loadSIP(is, "sip_packet_no_call_id.txt")
	for i := 0; i < b.N; i++ {
		NewMsg(sip)
	}
}
