package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/watch"
)

// Each route uses the real runner with cancellation at a deterministic method boundary.
func TestGracefulStopWatchRunnerRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"local", "daemon"} {
			for _, once := range []bool{false, true} {
				mode := "watch"
				if once {
					mode = "watch-once"
				}
				t.Run(api+"/"+route+"/"+mode, func(t *testing.T) {
					t.Cleanup(apextest.InvalidateRuntimeCaches)
					root := writeGracefulStopProject(t, api)
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					drained := make(chan struct{}, 1)
					opts := apextest.Options{Parallelism: 1, TimeoutMS: 0, Progress: func(p apextest.TestProgress) {
						if p.Event == "test_start" && p.MethodName == "zWait" {
							cancel()
						}
						if p.Event == "test_done" && p.MethodName == "zWait" {
							drained <- struct{}{}
						}
					}}
					var output bytes.Buffer
					result, err := runGracefulStopWatchRoute(t, ctx, root, route, opts, once, &output)
					// The unfixed route returns before the worker; do not let it outlive this case.
					select {
					case <-drained:
					case <-time.After(5 * time.Second):
						t.Fatal("cancelled worker did not drain")
					}
					if !errors.Is(err, context.Canceled) {
						t.Errorf("stop error = %v", err)
					}
					if s := result.Summary(); s.Passed != 1 || s.Total != 2 {
						t.Errorf("lost completed watch tests: summary=%+v", s)
					}
					assertWatchStoppedOutput(t, output.Bytes(), 2)
				})
			}
		}
	}
}

func TestGracefulStopWatchProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"local", "daemon", "dev"} {
			for _, mode := range []string{"--watch", "--watch-once"} {
				t.Run(api+"/"+route+"/"+mode, func(t *testing.T) {
					t.Cleanup(apextest.InvalidateRuntimeCaches)
					root := writeGracefulStopProject(t, api)
					// Warm every method before starting the interruption clock; a filtered
					// warmup leaves the full-run runtime compilation cold.
					var warmOut, warmErr bytes.Buffer
					if code := Run(context.Background(), []string{"test", "--project", root, "--no-serve", "--test-timeout", "20ms", "--parallelism", "1", "--no-progress"}, &warmOut, &warmErr); code != 1 {
						t.Fatalf("warm exit=%d: %s %s", code, &warmOut, &warmErr)
					}
					if err := os.Remove(defaultCLIDurationHistoryPath(root)); err != nil {
						t.Fatal(err)
					}
					args := []string{"test", "--project", root, "--no-progress", "--parallelism", "1", "--test-timeout", "0", mode}
					runsDir := filepath.Join(t.TempDir(), "runs")
					switch route {
					case "local":
						args = append(args, "--no-serve")
					case "daemon":
						args = append(args, "--daemon")
					case "dev":
						args = []string{"dev", "watch", "--project", root, "--out", runsDir, "--test-timeout", "0"}
						if mode == "--watch-once" {
							args = append(args, mode)
						}
					}
					ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
					defer cancel()
					var stdout, stderr bytes.Buffer
					code := Run(ctx, args, &stdout, &stderr)
					if code != 1 {
						t.Errorf("stopped exit=%d: %s %s", code, &stdout, &stderr)
					}
					if route == "dev" {
						latest, err := readLatest(runsDir)
						if err != nil {
							t.Fatalf("stopped dev watch lost artifacts: %v; stdout=%s stderr=%s", err, &stdout, &stderr)
						}
						result, err := loadLatestTestResult(latest)
						if err != nil || result.Summary().Passed != 1 || result.Summary().Total != 2 {
							t.Fatalf("lost stopped dev results: err=%v summary=%+v", err, result.Summary())
						}
						events, err := os.ReadFile(filepath.Join(latest.RunDir, "events.ndjson"))
						if err != nil {
							t.Fatal(err)
						}
						assertWatchStoppedOutput(t, events, 2)
						if !strings.Contains(stdout.String(), "Test run stopped") || !strings.Contains(stdout.String(), "1 completed") {
							t.Errorf("missing dev stop summary: %s", &stdout)
						}
					} else {
						assertWatchStoppedOutput(t, stdout.Bytes(), 2)
					}
				})
			}
		}
	}
}

func TestGracefulStopWatchIdleAndPerTestTimeoutRoutes(t *testing.T) {
	for _, route := range []string{"local", "daemon"} {
		for _, boundary := range []string{"idle-cancel", "per-test-timeout"} {
			t.Run(route+"/"+boundary, func(t *testing.T) {
				t.Cleanup(apextest.InvalidateRuntimeCaches)
				root := writeGracefulStopProject(t, "67.0")
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				opts := apextest.Options{Parallelism: 1, TimeoutMS: 10}
				output := &cancelAfterWatchFinish{cancel: cancel}
				if boundary == "idle-cancel" {
					opts.Filter = "aPass"
				} else {
					output.cancel = nil
				}
				result, err := runGracefulStopWatchRoute(t, ctx, root, route, opts, boundary == "per-test-timeout", output)
				if boundary == "idle-cancel" {
					if !errors.Is(err, context.Canceled) || result.Summary().Passed != 1 {
						t.Errorf("idle stop: err=%v summary=%+v", err, result.Summary())
					}
					assertWatchStoppedOutput(t, output.Bytes(), 1)
				} else if err != nil || result.Summary().Passed != 1 || !strings.Contains(output.String(), `"event":"watch.run_finished"`) || strings.Contains(output.String(), `"status":"stopped"`) {
					t.Errorf("per-test timeout stopped the watch: err=%v output=%s", err, output)
				}
			})
		}
	}
}

type cancelAfterWatchFinish struct {
	bytes.Buffer
	cancel context.CancelFunc
}

func (w *cancelAfterWatchFinish) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if w.cancel != nil && bytes.Contains(p, []byte(`"event":"watch.run_finished"`)) {
		w.cancel()
	}
	return n, err
}

func runGracefulStopWatchRoute(t *testing.T, ctx context.Context, root, route string, opts apextest.Options, once bool, output io.Writer) (testreport.Run, error) {
	t.Helper()
	cfg := watch.Config{Root: root, Backend: watch.BackendPoll}
	if route == "daemon" {
		daemon, err := testdaemon.New(root)
		if err != nil {
			t.Fatal(err)
		}
		return runWatchTestsDaemon(ctx, root, daemon, opts, cfg, once, output)
	}
	p, index, err := loadProjectIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	return runWatchTests(ctx, root, p, index, opts, cfg, once, output)
}

func assertWatchStoppedOutput(t *testing.T, output []byte, total int) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(output))
	stops := 0
	for {
		var event struct {
			Event    string             `json:"event"`
			ExitCode int                `json:"exitCode"`
			Reason   string             `json:"reason"`
			Status   string             `json:"status"`
			Summary  testreport.Summary `json:"summary"`
			Tests    []testreport.Case  `json:"tests"`
		}
		if err := decoder.Decode(&event); err != nil {
			if err != io.EOF {
				t.Fatalf("invalid watch JSON: %v: %s", err, output)
			}
			break
		}
		if event.Status != "stopped" {
			continue
		}
		stops++
		if event.Event != "watch.stopped" || event.ExitCode != 1 || event.Reason == "" {
			t.Errorf("invalid final watch status: %+v", event)
		}
		if total == 2 {
			data, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			assertGracefulStopEnvelope(t, data)
		}
		if event.Summary.Passed != 1 || event.Summary.Total != total || len(event.Tests) != total {
			t.Errorf("stopped event lost completed tests: %+v", event)
		}
	}
	if stops != 1 {
		t.Errorf("stopped events=%d, want 1: %s", stops, output)
	}
}
