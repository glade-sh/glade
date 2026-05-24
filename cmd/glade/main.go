package main

import (
	"context"
	"os"

	"github.com/glade-sh/glade/internal/gladecli"
)

func main() {
	os.Exit(gladecli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
