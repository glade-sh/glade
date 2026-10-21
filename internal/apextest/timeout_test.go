package apextest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
)

func TestTestTimeoutMessage(t *testing.T) {
	for _, tc := range []struct {
		source  string
		setting string
	}{
		{"flag", "--test-timeout flag"},
		{"environment", "GLADE_TEST_TIMEOUT environment variable"},
		{"default", "default"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			ctx, cancel := testContext(context.Background(), Options{TimeoutMS: 1, TimeoutSource: tc.source})
			defer cancel()
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
				t.Fatal("per-test cap did not expire")
			}
			err := testContextError(ctx, ctx.Err())
			if !errors.Is(err, context.DeadlineExceeded) || terminalReason(err) != testreport.ReasonTimeout {
				t.Fatalf("timeout identity or classification lost: %v", err)
			}
			for _, want := range []string{"per-test wall-clock cap 1ms (" + tc.setting + ")", "above 1ms", "--test-timeout", "--test-timeout 0"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("timeout error = %q, missing %q", err, want)
				}
			}
			if tc.source == "flag" && strings.Contains(err.Error(), "GLADE_TEST_TIMEOUT") {
				t.Fatalf("flag timeout recommends an ineffective environment override: %v", err)
			}
			if tc.source != "flag" && !strings.Contains(err.Error(), "GLADE_TEST_TIMEOUT=0") {
				t.Fatalf("timeout omitted environment guidance: %v", err)
			}
			problem := problemFromError(err, TestCase{})
			if problem.Type != "Canceled" || problem.Message != err.Error() {
				t.Fatalf("timeout problem lost guidance: %#v", problem)
			}
			// Setup failures must carry the same cause into each selected test.
			_, _, setupErr, _ := prepareTestSetupOrg(ctx, "SyntheticTest", nil, nil, nil, nil, nil, nil, nil, storage.OrgState{}, Options{}, nil)
			setupCase := preInvocationErrorCase(TestCase{}, setupErr)
			if setupCase.Reason != testreport.ReasonTimeout || setupCase.Problem.Message != err.Error() {
				t.Fatalf("setup timeout lost guidance: %#v", setupCase)
			}
		})
	}
}

func TestTestTimeoutLongFlagGuidance(t *testing.T) {
	err := &testTimeoutError{duration: 30 * time.Minute, source: "flag"}
	if !strings.Contains(err.Error(), "raise --test-timeout above 30m0s") || strings.Contains(err.Error(), "GLADE_TEST_TIMEOUT") {
		t.Fatalf("long flag cap has ineffective guidance: %v", err)
	}
}

func TestTestTimeoutZeroDisablesCap(t *testing.T) {
	parent, stop := context.WithCancel(context.Background())
	defer stop()
	ctx, cancel := testContext(parent, Options{TimeoutMS: 0, TimeoutSource: "flag"})
	if ctx != parent || cancel != nil {
		t.Fatal("zero timeout added a context or timer")
	}
	if _, ok := ctx.Deadline(); ok {
		t.Fatal("zero timeout added a deadline")
	}
	stop()
	if ctx.Err() != context.Canceled {
		t.Fatal("zero timeout stopped inheriting caller cancellation")
	}
}

func TestTestTimeoutPreservesCallerDeadlineAndCancellation(t *testing.T) {
	for _, timeoutMS := range []int64{0, 600_000} {
		parent, cancelParent := context.WithDeadlineCause(context.Background(), time.Now().Add(-time.Second), errors.New("caller budget"))
		ctx, cancel := testContext(parent, Options{TimeoutMS: timeoutMS, TimeoutSource: "default"})
		err := testContextError(ctx, ctx.Err())
		if err != context.DeadlineExceeded {
			t.Fatalf("caller deadline attributed to per-test cap: %v", err)
		}
		if cancel != nil {
			cancel()
		}
		cancelParent()
	}
	parent, cancelParent := context.WithCancel(context.Background())
	ctx, cancel := testContext(parent, Options{TimeoutMS: 600_000, TimeoutSource: "flag"})
	defer cancel()
	cancelParent()
	if err := testContextError(ctx, ctx.Err()); err != context.Canceled {
		t.Fatalf("caller cancellation attributed to per-test cap: %v", err)
	}
}
