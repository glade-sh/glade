//go:build linux

package pluginhost

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestManifestExecutableBusy(t *testing.T) {
	script := `#!/bin/sh
printf '{"apiVersion":"glade.plugin.v1","name":"compat","version":"0.1.0","commands":[{"path":["compat"],"summary":"Compat commands."}]}'
`
	for _, mode := range []string{"released", "persistent", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			exe := writeShellPlugin(t, t.TempDir(), "compat", script)
			f, err := os.OpenFile(exe, os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			ctx := context.Background()
			if mode == "released" {
				done := make(chan struct{})
				go func() { time.Sleep(30 * time.Millisecond); f.Close(); close(done) }()
				defer func() { <-done }()
			}
			if mode == "canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 30*time.Millisecond)
				defer cancel()
			}
			start := time.Now()
			manifest, err := LoadManifestFromExecutable(ctx, exe)
			switch mode {
			case "released":
				if err != nil || manifest.Name != "compat" {
					t.Fatalf("manifest = %#v, error = %v; want success after writer closes", manifest, err)
				}
			case "persistent":
				if !errors.Is(err, syscall.ETXTBSY) {
					t.Fatalf("error = %v, want ETXTBSY", err)
				}
				if elapsed := time.Since(start); elapsed < 30*time.Millisecond || elapsed > 2*time.Second {
					t.Fatalf("retry duration = %v, want bounded retries", elapsed)
				}
			case "canceled":
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("error = %v, want deadline exceeded", err)
				}
			}
		})
	}
}
