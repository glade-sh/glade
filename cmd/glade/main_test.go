package main

import (
	"testing"
)

func TestRootContextStopCancelsContext(t *testing.T) {
	ctx, stop := rootContext()
	stop()
	select {
	case <-ctx.Done():
	default:
		t.Fatal("root context was not canceled by stop")
	}
}
