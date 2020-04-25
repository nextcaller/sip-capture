package collect

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/gopacket/layers"
	"github.com/matryer/is"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type testFilter struct {
	sync.Mutex
	cnt int
}

func (f *testFilter) filterHalf(msg *layers.SIP) bool {
	f.Lock()
	defer f.Unlock()
	f.cnt++
	return f.cnt%2 == 0
}

type testPublisher struct {
	sync.Mutex
	msgs []*Msg
}

func (p *testPublisher) Publish(_ context.Context, m *Msg) error {
	p.Lock()
	defer p.Unlock()

	if p.msgs == nil {
		p.msgs = []*Msg{}
	}
	p.msgs = append(p.msgs, m)
	return nil
}

func TestAcceptLimit(t *testing.T) {
	is := is.New(t)

	f := &testFilter{}
	p := &testPublisher{}
	m := &layers.SIP{}

	c := NewCollecter(f.filterHalf, p.Publish, 1)

	err := c.Accept(m)
	is.NoErr(err)
	is.Equal(testutil.ToFloat64(c.metrics.Dropped), 0.0)

	err = c.Accept(m)
	is.True(errors.Is(err, ErrFull))
	is.Equal(testutil.ToFloat64(c.metrics.Dropped), 1.0)
}

func TestCollectMetrics(t *testing.T) {
	is := is.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f := &testFilter{}
	p := &testPublisher{}

	c := NewCollecter(f.filterHalf, p.Publish, 10)

	done := make(chan bool, 2)

	go func() {
		c.Publish(ctx)
		done <- true
	}()

	go func() {
		for x := 0; x < 10; x++ {
			c.Accept(&layers.SIP{})
		}
		time.Sleep(time.Millisecond * 10)
		cancel()
	}()

	select {
	case <-time.After(time.Second):
		t.Fatal("error waiting for messages to be collected")
	case <-done:
	}

	t.Log(f.cnt)
	is.True(f.cnt == 10)

	is.Equal(testutil.ToFloat64(c.metrics.Rejected), 5.0)
	is.Equal(testutil.ToFloat64(c.metrics.Published), 5.0)
}
