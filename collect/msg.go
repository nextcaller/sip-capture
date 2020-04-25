package collect

import (
	"fmt"
	"hash/fnv"
	"time"

	"github.com/google/gopacket/layers"
)

// Msg represents a captured SIP message and metadata.  It exists to create a
// JSON envelop for MQTT publishing.  SIPData will be base64 encoded.
type Msg struct {
	SIPData []byte    `json:"sip"`
	Time    time.Time `json:"time"`
	ID      string    `json:"id"`
}

// NewMsg creates a Msg structure from raw SIP Message data.  Its ID will be
// the SIP Call-ID (or i:) header if available, or a hash as of the entire SIP
// message if not available.
func NewMsg(sip *layers.SIP) *Msg {
	cid := sip.GetCallID()
	msg := append(sip.LayerContents(), sip.Payload()...)
	if cid == "" {
		h := fnv.New64a()
		_, _ = h.Write(msg)
		cid = fmt.Sprintf("%x", h.Sum64())
	}
	return &Msg{
		SIPData: msg,
		Time:    time.Now().UTC(),
		ID:      cid,
	}
}
