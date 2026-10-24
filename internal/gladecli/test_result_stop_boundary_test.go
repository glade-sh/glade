package gladecli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
)

// A failed write is still a command error even when the context also stops
// during that write. The command must not attempt a second terminal report.
func TestGracefulStopCompletedWriterErrorRoutes(t *testing.T) {
	for _, format := range []string{"json", "console"} {
		t.Run(format, func(t *testing.T) {
			classes := filepath.Join(t.TempDir(), "classes.txt")
			writeTestFile(t, classes, generatedEmptyClassShardMarker+"\n")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			writeErr := errors.New("terminal writer failed")
			output := &completedResultErrorWriter{cancel: cancel, err: writeErr}
			args := []string{"--class-file", classes, "--no-progress"}
			if format == "json" {
				args = append(args, "--json")
			}
			_, err := runTest(ctx, args, output, io.Discard)
			if !errors.Is(err, writeErr) || errors.Is(err, context.Canceled) || output.calls != 1 {
				t.Fatalf("terminal write failure changed: error=%v writes=%d", err, output.calls)
			}
		})
	}
}

// The other side of the output boundary must still report a stopped request
// with exit 1, even when every test case completed successfully.
func TestGracefulStopBeforeFinalReportProjectRoutes(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"no-serve", "read-only", "daemon", "connect", "auto-connect"} {
			for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
				for _, format := range []string{"json", "console"} {
					t.Run(api+"/"+route+"/"+cause.Error()+"/"+format, func(t *testing.T) {
						root := writeDiscoveryStopProject(t, api)
						ctx, cancel := discoveryStopContext(t, cause)
						progress := &cancelOnPhaseWriter{phase: "1 passed", cancel: cancel}
						args := []string{"test", "--project", root, "--progress"}
						if route == "connect" || route == "auto-connect" {
							startCompletedStopPeer(t, root, nil, cancel)
						}
						if route != "auto-connect" {
							args = append(args, "--"+route)
						}
						if format == "json" {
							args = append(args, "--json")
						}
						var output bytes.Buffer
						code := Run(ctx, args, &output, progress)
						if code != 1 || ctx.Err() != cause {
							t.Fatalf("pre-report stop lost: exit=%d context=%v stdout=%s stderr=%s", code, ctx.Err(), &output, progress.text.String())
						}
						assertStoppedReport(t, output.Bytes(), format, 1)
					})
				}
			}
		}
	}
}

func TestGracefulStopFinalReportWatchProjectRoutes(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"no-serve", "daemon"} {
			for _, mode := range []string{"watch", "watch-once"} {
				t.Run(api+"/"+route+"/"+mode, func(t *testing.T) {
					root := writeDiscoveryStopProject(t, api)
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					output := &cancelAfterWatchFinish{cancel: cancel}
					var stderr bytes.Buffer
					args := []string{"test", "--project", root, "--no-progress", "--watch-backend", "poll", "--" + route, "--" + mode}
					if code := Run(ctx, args, output, &stderr); code != 1 {
						t.Fatalf("watch stop exit=%d stdout=%s stderr=%s", code, output, &stderr)
					}
					assertWatchStoppedOutput(t, output.Bytes(), 1)
				})
			}
		}
	}
}

type completedResultErrorWriter struct {
	cancel context.CancelFunc
	err    error
	calls  int
}

func (w *completedResultErrorWriter) Write(p []byte) (int, error) {
	w.calls++
	w.cancel()
	return 0, w.err
}
