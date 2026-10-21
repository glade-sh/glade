package vm

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/trace"
)

func TestStatementCancellationIndependentOfQueryRows(t *testing.T) {
	program, err := CompileAnonymous(`List<Account> rows = [SELECT Id FROM Account];`)
	if err != nil {
		t.Fatal(err)
	}
	for _, rows := range []int{0, 1, 31, 63, 127} {
		for _, want := range []error{context.Canceled, context.DeadlineExceeded} {
			t.Run(fmt.Sprintf("rows=%d/%s", rows, want), func(t *testing.T) {
				org := storage.NewOrgState()
				storage.EnsureStandardObject(&org, "Account")
				for i := 0; i < rows; i++ {
					id := storage.ID(fmt.Sprintf("001%012d", i+1))
					org.Objects["Account"].Records[id] = storage.Record{ID: id, Object: "Account"}
				}
				machine := New(nil)
				machine.SetOrg(&org)
				if _, err := machine.executeProgram(program, nil); err != nil {
					t.Fatal(err)
				}
				if got := machine.limits.QueryRows; got != rows {
					t.Fatalf("query rows = %d, want %d", got, rows)
				}
				var ctx context.Context
				var cancel context.CancelFunc
				if want == context.Canceled {
					ctx, cancel = context.WithCancel(context.Background())
					cancel()
				} else {
					ctx, cancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				}
				defer cancel()
				machine.SetContext(ctx)
				// Each loop body is a separate executeProgram call. Polling must
				// accumulate across bodies, independently of SOQL CPU charges.
				for statement := 0; statement < 128; statement++ {
					_, err = machine.executeProgram(program, nil)
					if err != nil {
						break
					}
				}
				if !errors.Is(err, want) {
					t.Fatalf("after 128 statements: err = %v, want %v (CPU budget %d)", err, want, machine.cpuBudgetUsed)
				}
			})
		}
	}
}

func TestAppendTraceLazySkipsBuilderWhenTraceDisabled(t *testing.T) {
	called := false
	result := &Result{}
	appendTraceLazy(result, "apex.method.Example.run", "apex.method", func() map[string]any {
		called = true
		return map[string]any{"method": "Example.run"}
	})
	if called {
		t.Fatal("trace argument builder ran while tracing was disabled")
	}
	if len(result.Trace) != 0 {
		t.Fatalf("trace length = %d, want 0", len(result.Trace))
	}
}

func TestResultForLookupDoesNotCollectTrace(t *testing.T) {
	result := resultForLookup()
	if result.traceEnabled {
		t.Fatal("lookup result should not collect trace events")
	}
}

func TestAppendResultTraceCapsAndCoalesces(t *testing.T) {
	result := &Result{traceEnabled: true}
	for i := 0; i < maxTraceEvents+3; i++ {
		appendResultTrace(result, trace.Instant("apex.statement.expr", "apex.statement", int64(i), nil))
	}
	if len(result.Trace) != maxTraceEvents+1 {
		t.Fatalf("trace length = %d, want capped trace plus truncation event", len(result.Trace))
	}
	last := result.Trace[len(result.Trace)-1]
	if last.Name != "apex.trace.truncated" {
		t.Fatalf("last trace event = %#v, want truncation marker", last)
	}
	if got := last.Args["dropped"]; got != 3 {
		t.Fatalf("dropped trace count = %#v, want 3", got)
	}
}
