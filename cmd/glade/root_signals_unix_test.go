//go:build unix

package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// Exercise real signals in an isolated child so the test runner's own signal
// handlers cannot mask a missing subscription or terminate the parent process.
func TestRootContextHandlesTerminationSignals(t *testing.T) {
	if os.Getenv("GLADE_SIGNAL_TEST_CHILD") == "1" {
		ctx, stop := rootContext()
		defer stop()
		fmt.Println("ready")
		<-ctx.Done()
		if ctx.Err() != context.Canceled {
			t.Fatalf("root cancellation = %v", ctx.Err())
		}
		fmt.Println("canceled")
		return
	}
	for _, signal := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		t.Run(signal.String(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRootContextHandlesTerminationSignals$")
			command.Env = append(os.Environ(), "GLADE_SIGNAL_TEST_CHILD=1")
			var stderr bytes.Buffer
			command.Stderr = &stderr
			stdout, err := command.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			defer command.Process.Kill()
			reader := bufio.NewReader(stdout)
			if ready, err := reader.ReadString('\n'); err != nil || ready != "ready\n" {
				t.Fatalf("signal readiness = %q, %v", ready, err)
			}
			if err := command.Process.Signal(signal); err != nil {
				t.Fatal(err)
			}
			line, readErr := reader.ReadString('\n')
			if err := command.Wait(); err != nil || readErr != nil || line != "canceled\n" {
				t.Fatalf("signal %v did not cancel root: output=%q read=%v wait=%v stderr=%s", signal, line, readErr, err, stderr.String())
			}
		})
	}
}
