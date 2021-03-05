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
	msg := append(sip.LayerContents(), sip.Payload()...)
	cid := sip.GetCallID()
	if cid == "" {
		// This "shouldn't ever happen".  By RFC3261, CallID must be present
		// and globally unique.  However, if we're on the downstream side of
		// some internal process that runs over SIP messages, we might see a
		// message whose CallID is missing or unextractable.  In which case,
		// we'll make an ID that's dependent on the contents of the message (so
		// it's unique) using a hash.  This is not sufficient in the face of a
		// determined malicious attacker with control over SIP inputs but is
		// fast and will suffice for use cases where internal SIP messages
		// might be missing a CallID.
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
