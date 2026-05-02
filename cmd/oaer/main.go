package main

import (
	"context"
	"os"

	"github.com/open-aer/oaer/internal/oaercli"
)

func main() {
	os.Exit(oaercli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
