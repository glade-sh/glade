package gladecli

import (
	"context"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/watch"
)

func TestWatchRunCoordinatorWaitsForCanceledRunBeforeStartingPending(t *testing.T) {
	var starts []watch.TestSelection
	cancels := make([]context.CancelFunc, 0, 2)
	starter := func(runID int, selection watch.TestSelection) (context.CancelFunc, <-chan watchRunResult) {
		starts = append(starts, selection)
		done := make(chan watchRunResult, 1)
		cancelled := false
		cancel := func() {
			cancelled = true
		}
		cancels = append(cancels, func() {
			if !cancelled {
				t.Fatalf("run %d was not canceled", runID)
			}
		})
		return cancel, done
	}

	coordinator := newWatchRunCoordinator(1)
	first := coordinator.Start(watch.TestSelection{Mode: watch.SelectionAll, Reason: "initial"}, starter)
	if !first.Started || first.RunID != 1 || len(starts) != 1 {
		t.Fatalf("initial start = %#v, starts=%#v", first, starts)
	}

	pending := coordinator.Request(watch.TestSelection{Mode: watch.SelectionDirect, TestClasses: []string{"InvoiceTest"}}, starter)
	if pending.Started {
		t.Fatalf("pending run started before active run drained: %#v", pending)
	}
	if len(starts) != 1 {
		t.Fatalf("starts = %d, want only the active run", len(starts))
	}
	cancels[0]()

	emit, next := coordinator.Complete(watchRunResult{RunID: 1, Result: testreport.Run{Name: "old"}})
	if emit {
		t.Fatalf("canceled run should be drained without emitting a finish event")
	}
	if !next.Started || next.RunID != 2 {
		t.Fatalf("next start = %#v, want run 2", next)
	}
	if len(starts) != 2 || starts[1].TestClasses[0] != "InvoiceTest" {
		t.Fatalf("starts = %#v", starts)
	}
}

func TestWatchRunCoordinatorCoalescesPendingDirectSelections(t *testing.T) {
	var starts []watch.TestSelection
	starter := func(runID int, selection watch.TestSelection) (context.CancelFunc, <-chan watchRunResult) {
		starts = append(starts, selection)
		return func() {}, make(chan watchRunResult, 1)
	}

	coordinator := newWatchRunCoordinator(1)
	coordinator.Start(watch.TestSelection{Mode: watch.SelectionAll, Reason: "initial"}, starter)
	coordinator.Request(watch.TestSelection{Mode: watch.SelectionDirect, TestClasses: []string{"BillingTest"}}, starter)
	coordinator.Request(watch.TestSelection{Mode: watch.SelectionDirect, TestClasses: []string{"AccountTest"}}, starter)

	_, next := coordinator.Complete(watchRunResult{RunID: 1, Result: testreport.Run{Name: "old"}})
	if !next.Started {
		t.Fatalf("coalesced run did not start")
	}
	if len(starts) != 2 {
		t.Fatalf("starts = %d, want active plus coalesced pending", len(starts))
	}
	if got, want := starts[1].Mode, watch.SelectionDirect; got != want {
		t.Fatalf("mode = %s, want %s", got, want)
	}
	if got, want := starts[1].TestClasses, []string{"AccountTest", "BillingTest"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("test classes = %#v, want %#v", got, want)
	}
}
