package apextest

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func testContext(parent context.Context, opts Options) (context.Context, context.CancelFunc) {
	if opts.TimeoutMS <= 0 {
		return parent, nil
	}
	duration := time.Duration(opts.TimeoutMS) * time.Millisecond
	if opts.TimeoutSource == "" {
		return context.WithTimeout(parent, duration)
	}
	return context.WithTimeoutCause(parent, duration, &testTimeoutError{duration: duration, source: opts.TimeoutSource})
}

type testTimeoutError struct {
	duration time.Duration
	source   string
}

func (e *testTimeoutError) Error() string {
	setting := "default"
	guidance := fmt.Sprintf("raise GLADE_TEST_TIMEOUT above %s or pass a larger --test-timeout; disable it with GLADE_TEST_TIMEOUT=0 or --test-timeout 0", e.duration)
	switch e.source {
	case "flag":
		setting = "--test-timeout flag"
		guidance = fmt.Sprintf("raise --test-timeout above %s, or disable it with --test-timeout 0", e.duration)
	case "environment":
		setting = "GLADE_TEST_TIMEOUT environment variable"
	}
	return fmt.Sprintf("context deadline exceeded: per-test wall-clock cap %s (%s) exceeded; %s", e.duration, setting, guidance)
}

func (e *testTimeoutError) Unwrap() error { return context.DeadlineExceeded }

func testContextError(ctx context.Context, err error) error {
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == context.DeadlineExceeded {
		// Only our cap supplies setting guidance. A caller's earlier deadline
		// or cancellation must keep its existing message and classification.
		if cause, ok := context.Cause(ctx).(*testTimeoutError); ok {
			return cause
		}
	}
	return err
}
