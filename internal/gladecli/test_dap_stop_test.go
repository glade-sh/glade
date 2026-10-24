package gladecli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/dap"
)

func TestGracefulStopDAPBeforeCommandRoutes(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		for _, format := range []string{"console", "json"} {
			t.Run(cause.Error()+"/"+format, func(t *testing.T) {
				ctx, cancel := discoveryStopContext(t, cause)
				cancel()
				args := []string{"--debug", "--no-progress"}
				if format == "json" {
					args = append(args, "--json")
				}
				var stdout, stderr bytes.Buffer
				_, err := runTest(ctx, args, &stdout, &stderr)
				if !errors.Is(err, cause) {
					t.Fatalf("stop error = %v, want %v", err, cause)
				}
				assertDAPStopFrames(t, stdout.Bytes(), 0)
				assertDAPStopStderr(t, stderr.String(), "0 completed")
			})
		}
	}
}

func TestGracefulStopDAPSuccessfulProjectRoutes(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root := writeDiscoveryStopProject(t, "67.0")
	for _, route := range []string{"no-serve", "daemon"} {
		for _, format := range []string{"console", "json"} {
			t.Run(route+"/"+format, func(t *testing.T) {
				args := []string{"test", "--project", root, "--debug", "--no-progress", "--" + route}
				if format == "json" {
					args = append(args, "--json")
				}
				var stdout, stderr bytes.Buffer
				if code := Run(context.Background(), args, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
					t.Fatalf("successful debug exit=%d stderr=%s", code, &stderr)
				}
				assertDAPStopFrames(t, stdout.Bytes(), 1)
			})
		}
	}
}

// Stop as the snapshot starts writing: the test has completed, and the deferred
// reporter must preserve both the DAP frame and the completed result.
func TestGracefulStopDAPProjectRoutes(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		root := writeDiscoveryStopProject(t, api)
		for _, route := range []string{"automatic-local", "no-serve", "no-cache", "perf", "daemon"} {
			for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
				for _, format := range []string{"console", "json"} {
					t.Run(api+"/"+route+"/"+cause.Error()+"/"+format, func(t *testing.T) {
						ctx, cancel := discoveryStopContext(t, cause)
						stdout := &dapStopWriter{cancel: cancel}
						var stderr bytes.Buffer
						args := []string{"test", "--project", root, "--debug", "--no-progress"}
						switch route {
						case "no-serve", "no-cache", "daemon":
							args = append(args, "--"+route)
						case "perf":
							args = append(args, "--perf-json", filepath.Join(t.TempDir(), "perf.json"))
						}
						if format == "json" {
							args = append(args, "--json")
						}
						if code := Run(ctx, args, stdout, &stderr); code != 1 || ctx.Err() != cause {
							t.Fatalf("stop exit=%d context=%v stderr=%s", code, ctx.Err(), &stderr)
						}
						assertDAPStopFrames(t, stdout.Bytes(), 1)
						assertDAPStopStderr(t, stderr.String(), "1 completed")
						if !strings.Contains(stderr.String(), "DiscoveryTest.passes") {
							t.Fatalf("completed test missing from stderr: %s", &stderr)
						}
					})
				}
			}
		}
	}
}

// Exercise the real os.Stdout/stdin DAP session, rather than the embedded
// writer's initialize-only snapshot. Signal only after a valid response proves
// that the synthetic project's test completed and the adapter is serving.
func TestGracefulStopDAPStdioProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"no-serve", "daemon"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := writeDiscoveryStopProject(t, api)
				ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGracefulStopDAPStdioHelper$", "--",
					"test", "--project", root, "--debug", "--no-progress", "--"+route)
				cmd.Env = append(os.Environ(), "GLADE_TEST_DAP_STDIO_HELPER=1")
				stdin, err := cmd.StdinPipe()
				if err != nil {
					t.Fatal(err)
				}
				defer stdin.Close()
				stdoutPipe, err := cmd.StdoutPipe()
				if err != nil {
					t.Fatal(err)
				}
				var stdout, stderr bytes.Buffer
				stderrPipe, err := cmd.StderrPipe()
				if err != nil {
					t.Fatal(err)
				}
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				defer func() {
					cancel()
					if cmd.ProcessState == nil {
						_ = cmd.Wait()
					}
				}()
				if err := dap.Write(stdin, dap.Request{Seq: 1, Type: dap.MessageTypeRequest, Command: dap.CommandInitialize}); err != nil {
					t.Fatal(err)
				}
				reader := dap.NewReader(io.TeeReader(stdoutPipe, &stdout))
				if _, err := reader.Read(); err != nil {
					t.Fatalf("initial DAP frame: %v", err)
				}
				if err := cmd.Process.Signal(os.Interrupt); err != nil {
					t.Fatal(err)
				}
				stderrReader := bufio.NewReader(io.TeeReader(stderrPipe, &stderr))
				if line, err := stderrReader.ReadString('\n'); err != nil || line != "debug stop signal received\n" {
					t.Fatalf("signal acknowledgement: %q (%v)", line, err)
				}
				if err := dap.Write(stdin, dap.Request{Seq: 2, Type: dap.MessageTypeRequest, Command: dap.CommandDisconnect}); err != nil {
					t.Fatal(err)
				}
				for {
					_, err := reader.Read()
					if err != nil {
						if err != io.EOF {
							t.Errorf("stdout is not a DAP stream after interrupt: %v", err)
						}
						break
					}
				}
				_, _ = io.Copy(io.Discard, stderrReader)
				if err := cmd.Wait(); cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 1 || ctx.Err() != nil {
					t.Fatalf("stopped child: err=%v context=%v stderr=%s", err, ctx.Err(), &stderr)
				}
				assertDAPStopFrames(t, stdout.Bytes(), 4)
				assertDAPStopStderr(t, stderr.String(), "1 completed")
			})
		}
	}
}

func TestGracefulStopDAPStdioHelper(t *testing.T) {
	if os.Getenv("GLADE_TEST_DAP_STDIO_HELPER") != "1" {
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	go func() {
		<-ctx.Done()
		// Let the client disconnect only once cancellation is observable.
		fmt.Fprintln(os.Stderr, "debug stop signal received")
	}()
	for i, arg := range os.Args {
		if arg == "--" {
			os.Exit(Run(ctx, os.Args[i+1:], os.Stdout, os.Stderr))
		}
	}
	t.Fatal("missing helper command")
}

type dapStopWriter struct {
	bytes.Buffer
	cancel context.CancelFunc
}

func (w *dapStopWriter) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if bytes.Contains(p, []byte("Content-Length:")) {
		w.cancel()
	}
	return n, err
}

func assertDAPStopFrames(t *testing.T, stream []byte, want int) {
	t.Helper()
	reader := dap.NewReader(bytes.NewReader(stream))
	count := 0
	for {
		raw, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stdout is not a DAP stream after %d frames: %v\n%s", count, err, stream)
		}
		var message struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &message); err != nil || (message.Type != "response" && message.Type != "event") {
			t.Fatalf("invalid DAP message: %s (%v)", raw, err)
		}
		count++
	}
	if count != want {
		t.Fatalf("DAP frame count=%d, want %d: %s", count, want, stream)
	}
}

func assertDAPStopStderr(t *testing.T, stderr, completed string) {
	t.Helper()
	if strings.Count(stderr, "Test run stopped (") != 1 || !strings.Contains(stderr, completed) {
		t.Fatalf("missing single stop summary with %s on stderr: %s", completed, stderr)
	}
}
