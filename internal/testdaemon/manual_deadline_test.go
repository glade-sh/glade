package testdaemon

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

// manualDeadlineContext models elapsed time only for connected OpRun tests.
// It exposes cancellation, but no wall-clock deadline to real network operations.
type manualDeadlineContext struct {
	mu                sync.Mutex
	epoch             time.Time
	deadline, elapsed time.Duration
	done              chan struct{}
	err               error
}

func newManualDeadlineContext(after time.Duration) *manualDeadlineContext {
	if after <= 0 {
		panic("manual deadline must be positive")
	}
	return &manualDeadlineContext{epoch: time.Now(), deadline: after, done: make(chan struct{})}
}
func (c *manualDeadlineContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *manualDeadlineContext) Done() <-chan struct{}       { return c.done }
func (c *manualDeadlineContext) Value(any) any               { return nil }
func (c *manualDeadlineContext) Err() error                  { c.mu.Lock(); defer c.mu.Unlock(); return c.err }
func (c *manualDeadlineContext) Advance(d time.Duration) {
	if d < 0 {
		panic("manual elapsed time cannot move backward")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.elapsed += d
	if c.err == nil && c.elapsed >= c.deadline {
		c.err = context.DeadlineExceeded
		close(c.done)
	}
}
func (c *manualDeadlineContext) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err == nil {
		c.err = context.Canceled
		close(c.done)
	}
}

func TestManualDeadlineContext(t *testing.T) {
	c := newManualDeadlineContext(time.Second)
	if _, ok := c.Deadline(); ok {
		t.Fatal("manual clock must not expose a wall-clock deadline")
	}
	c.Advance(time.Second - time.Nanosecond)
	select {
	case <-c.Done():
		t.Fatal("closed before deadline")
	default:
	}
	if c.Err() != nil {
		t.Fatalf("early error %v", c.Err())
	}
	c.Advance(time.Nanosecond)
	select {
	case <-c.Done():
	default:
		t.Fatal("deadline did not close Done")
	}
	c.Advance(time.Second)
	c.Close()
	if c.Err() != context.DeadlineExceeded {
		t.Fatalf("deadline cause changed: %v", c.Err())
	}
	if _, ok := c.Deadline(); ok {
		t.Fatal("manual clock exposed a wall-clock deadline after expiry")
	}
	canceled := newManualDeadlineContext(time.Second)
	canceled.Close()
	canceled.Advance(time.Second)
	if canceled.Err() != context.Canceled {
		t.Fatalf("cancellation cause changed: %v", canceled.Err())
	}
}

// Dial must not interpret the manually advanced deadline as real wall time.
func TestManualDeadlineDialContext(t *testing.T) {
	clock := newManualDeadlineContext(time.Millisecond)
	defer clock.Close()
	clock.epoch = time.Now().Add(-time.Hour)
	ctx := dialContextWithoutDeadline{clock}
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("manual deadline leaked to real dialer")
	}
	if ctx.Err() != nil {
		t.Fatalf("unadvanced manual context expired: %v", ctx.Err())
	}
	socket, listener := protocolWiringUnixListener(t)
	defer listener.Close()
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", socket)
	if err != nil {
		t.Fatalf("unadvanced manual clock blocked real dial: %v", err)
	}
	_ = conn.Close()
	clock.Advance(time.Millisecond)
	if ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("deadline cause not forwarded: %v", ctx.Err())
	}
}

// dialContextWithoutDeadline forwards cancellation while hiding the simulated
// absolute deadline from real net.Dialer timers.
type dialContextWithoutDeadline struct{ context.Context }

func (dialContextWithoutDeadline) Deadline() (time.Time, bool) { return time.Time{}, false }
