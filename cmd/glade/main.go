package main

import (
	"context"
	"os"
	"os/signal"
	"runtime/debug"

	"github.com/glade-sh/glade/internal/gladecli"
)

func main() {
	tuneExecGC(os.Args[1:])
	ctx, stop := rootContext()
	defer stop()
	os.Exit(gladecli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

// Tune only the standalone exec process. Library callers and test binaries keep
// their collector target; an explicit GOGC always takes precedence.
func tuneExecGC(args []string) {
	if len(args) > 0 && args[0] == "exec" && os.Getenv("GOGC") == "" {
		// Startup retains about 60 MB of platform tables. A higher target avoids
		// repeatedly marking them while the short-lived process starts up.
		debug.SetGCPercent(400)
	}
}

func rootContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}
