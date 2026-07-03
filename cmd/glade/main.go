package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/glade-sh/glade/internal/gladecli"
)

func main() {
	ctx, stop := rootContext()
	defer stop()
	os.Exit(gladecli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func rootContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}
