package gladecli

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/dap"
)

func TestDAPLaunchCancelEndsWhileProjectLoadIsRunning(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	finished := make(chan struct{})
	previous := dapStartupLoader
	dapStartupLoader = func(string) dapStartup {
		close(started)
		<-release
		close(finished)
		return dapStartup{err: errors.New("released")}
	}
	t.Cleanup(func() {
		dapStartupLoader = previous
		select {
		case <-release:
		default:
			close(release)
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		handler := dap.NewHandler(dap.Snapshot{})
		result <- handleDAPLaunch(ctx, handler, dap.LaunchRequest{AnonymousBody: "System.debug(1);"}, t.TempDir(), "", false)
	}()
	<-started
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("launch error = %v, want context canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("launch stayed blocked in the project load after cancel")
	}
	select {
	case <-finished:
		t.Fatal("the load should still be running")
	default:
	}
	close(release)
	<-finished
}
