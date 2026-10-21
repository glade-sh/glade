package vm

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWhileLoopObservesCancellation(t *testing.T) {
	for _, source := range []string{"while (true) {}", "while (true) { continue; }", "while (true) { Integer value = 1; }"} {
		t.Run(source, func(t *testing.T) {
			program, err := CompileAnonymous(source)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
			defer cancel()
			machine := New(nil)
			machine.SetContext(ctx)
			if _, err := machine.Execute(program); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expected cancellation, got %v", err)
			}
		})
	}
}

func TestEmptyWhileLoopObservesStrictCPUBudget(t *testing.T) {
	program, err := CompileAnonymous("while (true) {}")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	machine := New(nil)
	machine.SetContext(ctx)
	machine.SetLimitMode(LimitModeStrict)
	caps := defaultLimitCaps()
	caps.CPUTimeMS = 10
	machine.SetLimitCaps(caps)
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.LimitException" {
		t.Fatalf("expected strict CPU limit, got %v", err)
	}
}
