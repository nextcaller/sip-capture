package sipsplitter

import (
	"bytes"
	"errors"
	"strconv"
)

var (
	// crlf is a convenience variable for matching text lines.
	crlf = []byte("\r\n")
	// crlfcrlf is a convenience variable for matching the boundary between SIP
	// headers and body.
	crlfcrlf = []byte("\r\n\r\n")

	// ErrBadContentLength indicates a SIP message without a Content-Length
	// header, or one that was unparsable as an integer number.
	ErrBadContentLength = errors.New("invalid Content-Length")
)

// Trace is a set of hooks that run during at various stages during attempts to
// split the SIP message stream.  Any particular hook may be nil.
type Trace struct {
	// Discard is called whenever non-valid SIP bytes are found.  Usually this
	// will only be in cases where the stream has initial non-SIP bytes, or an
	// error in parsing requires a resynchronization.  The bytes being discarded
	// are passed as an argument should they need to be logged.
	Discard func([]byte)
	// NoStartLine is called if the current stream does not yet have a
	// valid-looking SIP Request or Response line.
	NoStartLine func()
	// StarLine is called when the stream contains something that could
	// be a SIP Request or Response.  The line is the entirety of the Request
	// or Response line.
	StartLine func(line []byte)
	// NoHeaders is called if the stream has a starting line, but not yet a
	// full header block, upto and including the CRLF CRLF that delimits the
	// headers from the body.
	NoHeaders func()
	// Headers is called when an entire set of SIP headers has been read, up to
	// the CRLF CRLF that delimits the body.  headers contains the full header
	// bytes, not including the start line nor the empty line with CRLF.
	Headers func(headers []byte)
	// NoBody is called if the stream has a start line and headers, but does
	// not yet have enough bytes to constitute the full body as described by the
	// Content-Length header.
	NoBody func()
	// Body is called when an entire SIP body is read. The body contains all
	// the bytes after the empty line and CRLF, equal to the total
	// Content-Length of the SIP message.
	Body func(body []byte)
	// Complete is called every time a full, complete, SIP message could be
	// identified within the stream.  The bytes will match the
	// value of bufio.Scanner.Bytes() for the current token, which is the whole
	// message.
	Complete func([]byte)
}

// Splitter provides SplitSIP for using bufio.Scanner to extract individual SIP
// messages as tokens from an io.Reader stream.  Its zero value is usable, and
// will operate without errors, discarding any invalid messages.
type Splitter struct {
	// Trace hooks operate similarly to httptrace.ClientTrace, allowing hooks
	// that run at the various stages of splitting for instrumentation.
	// If Trace is nil, no hooks will be run.
	Trace *Trace
	// ExitOnError indicates that instead of trying to resynchronize, the
	// splitter should exit on any messages that are not parseable.  Currently
	// the only error case is a missing or incorrect Content-Length header.
	ExitOnError bool

	// internal state used to not double run trace hooks when atEOF.
	last int
}

// SplitSIP provides a splitter function suitable to set as a
// bufio.Scanner.Split method.  It returns each SIP message as a token.
// Bytes that are not part of a valid SIP message are discarded, possibly
// without error depending on the value of Splitter.ExitOnError.
func (s *Splitter) SplitSIP(b []byte, atEOF bool) (int, []byte, error) {
	// If we don't check s.last when atEOF, we will double invoke our final
	// set of Trace hooks.
	if atEOF && (len(b) == 0 || len(b) == s.last) {
		return 0, nil, nil
	}
	s.last = len(b)

	advance, endstart := findStartLine(b)
	if advance > 0 {
		// We found a SIP message, but there was junk in the reader before it.
		// Discard it and let the scanner try again.
		if s.Trace != nil && s.Trace.Discard != nil {
			s.Trace.Discard(b[:advance])
		}
		return advance, nil, nil
	}

	if advance < 0 {
		// Nothing that looks like the beginning of a SIP header, wait for more.
		if s.Trace != nil && s.Trace.NoStartLine != nil {
			s.Trace.NoStartLine()
		}
		return 0, nil, nil
	}

	if s.Trace != nil && s.Trace.StartLine != nil {
		s.Trace.StartLine(b[:endstart])
	}

	advance = bytes.Index(b, crlfcrlf)
	if advance == -1 {
		// Have some headers, but they're not complete yet, wait for more.
		if s.Trace != nil && s.Trace.NoHeaders != nil {
			s.Trace.NoHeaders()
		}
		return 0, nil, nil
	}

	// the CRLF termination is part of the final header.
	advance += len(crlf)

	if s.Trace != nil && s.Trace.Headers != nil {
		s.Trace.Headers(b[endstart:advance])
	}

	// Advance past the empty line delimiting headers.
	advance += len(crlf)

	contentLen := getContentLength(b[:advance])
	if contentLen == -1 {
		// Content-Length is junk; this message is irrecoverable.
		// If ExitOnError is set, this is a fatal error.
		if s.ExitOnError {
			return len(b), nil, ErrBadContentLength
		}
		// Otherwise, discard everything up to the end of the current headers.
		// We'll get one more Discard for the remaining body on re-enter.
		if s.Trace != nil && s.Trace.Discard != nil {
			s.Trace.Discard(b[:advance])
		}
		return advance, nil, nil
	}

	if advance+contentLen > len(b) {
		// Don't yet have enough bytes for the full body, wait for more.
		if s.Trace != nil && s.Trace.NoBody != nil {
			s.Trace.NoBody()
		}
		return 0, nil, nil
	}
	if s.Trace != nil && s.Trace.Body != nil {
		s.Trace.Body(b[advance : advance+contentLen])
	}

	advance += contentLen

	if s.Trace != nil && s.Trace.Complete != nil {
		s.Trace.Complete(b[:advance])
	}
	return advance, b[:advance], nil
}

// scan a byte slice, locate a Content-Length (or compact form l) header, parse
// the number out and return it.  Returns -1 if no header was found or could
// not be parsed.  This must be called on the full message, not just headers,
// as it relies on the crlf terminating the start line if content-length/l is
// the first actual header, and/or the crlfcrlf sequence at the end of the
// headers if it's the last.
//   Content-Length  =  ( "Content-Length" / "l" ) HCOLON 1*DIGIT
func getContentLength(b []byte) int {
	// Check for Content-Length first:
	pos := bytes.Index(b, []byte("\r\nContent-Length:"))

	// check for l: if there's no Content-Length
	if pos == -1 {
		pos = bytes.Index(b, []byte("\r\nl:"))
		if pos == -1 {
			return -1
		}
	}

	pos += len(crlf)                         // skip over previous crlf
	pos += bytes.IndexRune(b[pos:], ':') + 1 // skip to past :

	eol := bytes.Index(b[pos:], crlf) // find crlf
	if eol == -1 {
		// Assuming we came in from SplitSIP, this can't actually happen.
		// We'll have already assured that we have a CRLFCRLF that ends the
		// headers before calling here.
		return -1
	}

	clen := string(bytes.Trim(b[pos:pos+eol], " \t"))
	l, err := strconv.Atoi(clen)
	if err != nil {
		return -1
	}
	return l
}

// Search a byte slice until we find a line that could be the beginning of a
// SIP message.  Matching is only "good enough", not full validation of
// legitimate start lines.  However, it's much faster than doing complete
// parsing of every SIP message, and much less code than writing a fully
// compliant SIP lexer and parser.  This can be fooled if someone is doing
// something adversarial or weird like tunneling raw SIP messages in a MIME
// envelop of the body of other SIP messages, and we get desynced on
// content-lengths.  In the worst case, we'd discard extra bytes we otherwise
// wouldn't have discarded.
func findStartLine(b []byte) (int, int) {
	eol := 0
	for start := 0; start < len(b); start += eol {
		eol = bytes.Index(b[start:], crlf)
		if eol == -1 {
			return -1, -1
		}
		eol += len(crlf) // line includes CRLF.
		line := b[start : start+eol]
		if isRequest(line) {
			return start, eol
		}
		if isResponse(line) {
			return start, eol
		}
	}
	return -1, -1
}

// sipMethods are the names of each SIP message type we could encounter.
// They're ordered by approximate likelihood to be seen, since they'll be
// scanned sequentially.
var sipMethods = [][]byte{
	[]byte("INVITE"),
	[]byte("ACK"),
	[]byte("BYE"),
	[]byte("OPTIONS"),
	[]byte("REGISTER"),
	[]byte("CANCEL"),
	[]byte("PUBLISH"),
	[]byte("PRACK"),
	[]byte("INFO"),
	[]byte("SUBSCRIBE"),
	[]byte("NOTIFY"),
	[]byte("UPDATE"),
	[]byte("MESSAGE"),
	[]byte("REFER"),
}

// Check if a line could possibly be a SIP request line that begins a SIP
// message.
//	Request-Line   =  Method SP Request-URI SP SIP-Version CRLF
//	SIP-Version    =  "SIP" "/" 1*DIGIT "." 1*DIGIT
func isRequest(line []byte) bool {
	// if the line is too short to possibly be a response `ACK x SIP/2.0\r\n`
	// or the first character isn't the beginning of a method name,
	// we can short circuit out.
	if len(line) < 15 || bytes.IndexAny(line[0:1], "BACONPURISM") != 0 {
		return false
	}

	s1 := bytes.IndexByte(line, ' ')
	s2 := bytes.IndexByte(line[s1+1:], ' ')
	if s1 < 0 || s2 < 0 {
		return false
	}
	s2 += s1 + 1

	method := line[0:s1]
	found := false
	for _, validmethod := range sipMethods {
		if bytes.Equal(method, validmethod) {
			found = true
			break
		}
	}
	if !found {
		return false
	}

	if !bytes.HasPrefix(line[s2+1:], []byte("SIP/")) {
		return false
	}
	// Could validate SIP URI or exact SIP version here.
	return true
}

// Determine if a line could possibly be a status line that indicates the start
// of a SIP response.
//  Status-Line     =  SIP-Version SP Status-Code SP Reason-Phrase CRLF
//	SIP-Version    =  "SIP" "/" 1*DIGIT "." 1*DIGIT
func isResponse(line []byte) bool {
	if len(line) < 14 || !bytes.HasPrefix(line, []byte("SIP/")) {
		return false
	}
	s1 := bytes.IndexByte(line, ' ')
	s2 := bytes.IndexByte(line[s1+1:], ' ')
	if s1 < 0 || s2 < 0 {
		return false
	}
	// could validate sip version and/or status code here.
	return true
}
