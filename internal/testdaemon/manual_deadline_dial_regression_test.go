package testdaemon

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestManualClockDoesNotExpireRealDial(t *testing.T) {
	clock := newManualDeadlineContext(time.Millisecond)
	defer clock.Close()
	clock.epoch = time.Now().Add(-time.Hour)
	socket, listener := protocolWiringUnixListener(t)
	defer listener.Close()
	conn, err := (&net.Dialer{}).DialContext(clock, "unix", socket)
	if err != nil {
		t.Fatalf("unadvanced manual clock expired real dial: %v", err)
	}
	conn.Close()
	clock.Advance(time.Millisecond)
	if clock.Err() != context.DeadlineExceeded {
		t.Fatalf("manual deadline error = %v", clock.Err())
	}
}
