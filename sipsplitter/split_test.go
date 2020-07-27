package sipsplitter

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/matryer/is"
	"github.com/nextcaller/sip-capture/testhelpers"
)

type captureStats struct {
	Original    string
	Discarded   int
	NoStartLine int
	StartLine   int
	NoHeaders   int
	Header      int
	NoBody      int
	Body        int
	Complete    int
	Discards    []string
	Starts      []string
	Headers     []string
	Bodies      []string
	Messages    []string
}

func NewTestTrace(s *captureStats) *Trace {
	return &Trace{
		Discard:     func(m []byte) { s.Discarded++; s.Discards = append(s.Discards, string(m)) },
		NoStartLine: func() { s.NoStartLine++ },
		StartLine:   func(m []byte) { s.StartLine++; s.Starts = append(s.Starts, string(m)) },
		NoHeaders:   func() { s.NoHeaders++ },
		Headers:     func(m []byte) { s.Header++; s.Headers = append(s.Headers, string(m)) },
		NoBody:      func() { s.NoBody++ },
		Body:        func(m []byte) { s.Body++; s.Bodies = append(s.Bodies, string(m)) },
		Complete:    func(m []byte) { s.Complete++; s.Messages = append(s.Messages, string(m)) },
	}
}

func TestSplit(t *testing.T) {
	testCases := map[string]struct {
		fname string
	}{
		"empty stream":                           {"empty"},
		"random junk":                            {"randomjunk"},
		"junk line":                              {"junkline"},
		"bad status line":                        {"badresponseline"},
		"bad request line":                       {"badrequestline"},
		"initial junk, then SIP":                 {"bad_then_sip"},
		"incorrect SIP request":                  {"bad_status_version"},
		"incomplete SIP Headers":                 {"incomplete_headers"},
		"incomplete SIP Body":                    {"incomplete_body"},
		"missing CL":                             {"clen_missing"},
		"CL not numeric":                         {"clen_non_numeric"},
		"missing CL, discard to next":            {"clen_missing_then_good"},
		"complete response":                      {"complete_response"},
		"complete request":                       {"complete_request"},
		"complete with body":                     {"complete_with_body"},
		"two complete requests":                  {"complete_two_requests"},
		"two complete requests, then incomplete": {"complete_two_incomplete"},
		"request_with_full_headers":              {"full_request"},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			is := is.New(t)

			done := make(chan bool, 1)

			rawsip, err := ioutil.ReadFile(filepath.Join("testdata", tc.fname+".stream"))
			is.NoErr(err) // loaded raw sip file.

			stats := captureStats{
				Original: string(rawsip),
				Messages: []string{},
				Bodies:   []string{},
				Headers:  []string{},
				Starts:   []string{},
				Discards: []string{},
			}
			scanner := bufio.NewScanner(bytes.NewReader(rawsip))
			splitter := &Splitter{Trace: NewTestTrace(&stats)}
			scanner.Split(splitter.SplitSIP)

			go func() {
				for scanner.Scan() {
				}
				done <- true
			}()
			select {
			case <-done:
			case <-time.After(time.Millisecond * 20):
				t.Error("timed out waiting for stream scan")
			}

			is.NoErr(scanner.Err())

			jbytes, err := json.MarshalIndent(stats, "", "  ")
			is.NoErr(err) // marshall test stats
			testhelpers.CompareGolden(t, name, tc.fname+".expected", jbytes)
		})
	}
}

func TestExitOnError(t *testing.T) {
	is := is.New(t)

	done := make(chan bool, 1)

	scanner := bufio.NewScanner(strings.NewReader("INVITE foo@bar SIP/2.0\r\nContent-Length: a\r\n\r\n"))
	splitter := &Splitter{ExitOnError: true}
	scanner.Split(splitter.SplitSIP)

	go func() {
		for scanner.Scan() {
		}
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(time.Millisecond * 20):
		t.Error("timed out waiting for stream scan")
	}

	t.Log(scanner.Err())
	is.True(errors.Is(scanner.Err(), ErrBadContentLength))
}

func BenchmarkFindSIPMessage(b *testing.B) {
	data := []byte("blahlblah blah\r\nnSIP/2.0 200 OK\r\nINVITE foo@bar SIP/2.0\r\nMore-HEADERs: blah\r\nContent-Length: 1\r\n\r\n1\r\n")
	for i := 0; i < b.N; i++ {
		pos, end := findStartLine(data)
		if pos <= 0 || end <= 0 {
			b.Fatal("whoops")
		}
	}
}
