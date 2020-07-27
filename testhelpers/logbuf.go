package testhelpers

import (
	"bytes"
	"sync"
)

// LogBuf is a synchronized io.Writer.  It's meant to prevent any race detection
// on loggers in background go routines if you use a raw bytes.Buffer.
//
// Typical usage pattern with zerolog looks something like:
// buf := testhelpers.NewLogBuf()
// log := zerolog.New(buf)
// <do test things that write to log>
// if !strings.Contains(buf.String(), "some test value") { t.Error("missing expected log") }
type LogBuf struct {
	sync.Mutex
	*bytes.Buffer
}

// Write satisfies io.Writer.
func (ml *LogBuf) Write(p []byte) (int, error) {
	ml.Lock()
	defer ml.Unlock()
	return ml.Buffer.Write(p)
}

// String satisfies Stringer
func (ml *LogBuf) String() string {
	ml.Lock()
	defer ml.Unlock()
	return ml.Buffer.String()
}

// Reset resets the log buffer.
func (ml *LogBuf) Reset() {
	ml.Lock()
	defer ml.Unlock()
	ml.Buffer.Reset()
}

// NewLogBuf returns an initialized log buffer.
func NewLogBuf() *LogBuf {
	ml := LogBuf{}
	ml.Buffer = &bytes.Buffer{}
	return &ml
}
