package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testreport"
)

func TestGracefulStopDiscoveryResultWriterRoutes(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		for _, format := range []string{"json", "console"} {
			t.Run(cause.Error()+"/"+format, func(t *testing.T) {
				ctx, cancel := discoveryStopContext(t, cause)
				cancel()
				var output bytes.Buffer
				if err := writeTestRunResult(ctx, &output, testreport.Run{}, format, ""); err != nil {
					t.Fatal(err)
				}
				assertDiscoveryStoppedOutput(t, output.Bytes(), format)
			})
		}
	}
}

// These routes share discovery but have different setup, output and return paths.
// Cancel from the synchronous progress callback, before any test can execute.
func TestGracefulStopDiscoveryProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{
			"no-serve", "no-cache", "read-only", "watch", "watch-once",
			"daemon", "daemon-watch", "daemon-watch-once", "connect", "auto-connect", "auto-local",
			"perf", "impact", "shards", "dev-test", "dev-watch", "dev-watch-once",
		} {
			for _, phase := range []string{"before-command", "discovering"} {
				// Dev watch has no discovery progress renderer; its shared watch
				// setup boundary is also exercised directly below.
				if strings.HasPrefix(route, "dev-watch") && phase == "discovering" {
					continue
				}
				for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
					for _, format := range []string{"json", "console"} {
						if (strings.Contains(route, "watch") || strings.HasPrefix(route, "dev-")) && format == "json" {
							continue
						}
						t.Run(api+"/"+route+"/"+phase+"/"+cause.Error()+"/"+format, func(t *testing.T) {
							root := writeDiscoveryStopProject(t, api)
							if route == "connect" || route == "auto-connect" {
								defer startDaemonEquivalenceServer(t, root)()
							}
							ctx, cancel := discoveryStopContext(t, cause)
							progress := &cancelOnPhaseWriter{phase: "Discovering tests", cancel: cancel}
							if phase == "before-command" {
								cancel()
							}
							args := discoveryStopArgs(t, root, route, format)
							var stdout bytes.Buffer
							code := Run(ctx, args, &stdout, progress)
							if ctx.Err() != cause {
								t.Fatalf("did not reach cancellation boundary: exit=%d stdout=%s", code, &stdout)
							}
							if code != 1 {
								t.Errorf("stopped exit=%d, want 1", code)
							}
							if strings.Contains(route, "watch") && !strings.HasPrefix(route, "dev-") {
								format = "watch"
							}
							assertDiscoveryStoppedOutput(t, stdout.Bytes(), format)
						})
					}
				}
			}
		}
	}
}

func TestGracefulStopDiscoveryWatchSetupRoutes(t *testing.T) {
	for _, route := range []string{"local", "daemon"} {
		for _, once := range []bool{false, true} {
			for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
				t.Run(fmt.Sprintf("%s/once=%t/%s", route, once, cause), func(t *testing.T) {
					root := writeDiscoveryStopProject(t, "67.0")
					ctx, cancel := discoveryStopContext(t, cause)
					cancel()
					var output bytes.Buffer
					result, err := runGracefulStopWatchRoute(t, ctx, root, route, apextest.Options{}, once, &output)
					if !errors.Is(err, cause) || result.Summary().Total != 0 {
						t.Fatalf("startup stop: result=%+v err=%v", result, err)
					}
					assertDiscoveryStoppedOutput(t, output.Bytes(), "watch")
				})
			}
		}
	}
}

// The local loader checks context before loading and after project, metadata,
// and index construction. Exercise each boundary without wall-clock sleeps.
func TestGracefulStopDiscoveryLoadPhaseRoutes(t *testing.T) {
	for _, route := range []string{"no-serve", "no-cache", "read-only", "perf", "impact", "shards", "watch", "watch-once"} {
		for _, phase := range []int{1, 2, 3, 4} {
			t.Run(fmt.Sprintf("%s/checkpoint-%d", route, phase), func(t *testing.T) {
				root := writeDiscoveryStopProject(t, "67.0")
				base, cancel := context.WithCancel(context.Background())
				defer cancel()
				ctx := &discoveryCheckpointContext{Context: base, remaining: phase, cancel: cancel}
				progress := &cancelOnPhaseWriter{phase: "Discovering tests", cancel: func() { ctx.armed = true }}
				var output bytes.Buffer
				code := Run(ctx, discoveryStopArgs(t, root, route, "json"), &output, progress)
				if code != 1 || ctx.remaining != 0 {
					t.Fatalf("checkpoint not reached: exit=%d remaining=%d output=%s", code, ctx.remaining, &output)
				}
				format := "json"
				if strings.Contains(route, "watch") {
					format = "watch"
				}
				assertDiscoveryStoppedOutput(t, output.Bytes(), format)
			})
		}
	}
}

func TestGracefulStopDiscoveryConnectedEmptyResultExit(t *testing.T) {
	for _, route := range []string{"connect", "auto-connect"} {
		t.Run(route, func(t *testing.T) {
			root := writeDiscoveryStopProject(t, "67.0")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stub := startProtocolV1Stub(t, root, func(request testdaemon.RequestV1) testdaemon.ResponseV1 {
				response := testdaemon.ResponseV1{Version: testdaemon.ProtocolVersionV1, Op: testdaemon.OpPong, OK: true, Ready: true}
				if request.Op == testdaemon.OpRun {
					response.Op, response.Run = testdaemon.OpRunResult, &testreport.Run{}
					cancel()
				}
				return response
			})
			defer stub.Close()
			args := []string{"test", "--project", root, "--json", "--no-progress"}
			if route == "connect" {
				args = append(args, "--connect")
			}
			var stdout, stderr bytes.Buffer
			if code := Run(ctx, args, &stdout, &stderr); code != 1 {
				t.Errorf("stopped empty response exit=%d stdout=%s stderr=%s", code, &stdout, &stderr)
			}
			assertDiscoveryStoppedOutput(t, stdout.Bytes(), "json")
		})
	}
}

type discoveryCheckpointContext struct {
	context.Context
	armed      bool
	remaining  int
	cancel     context.CancelFunc
	afterCheck bool
}

func (c *discoveryCheckpointContext) Err() error {
	observed := c.Context.Err()
	if c.armed && c.remaining > 0 {
		c.remaining--
		if c.remaining == 0 {
			c.cancel()
		}
	}
	if c.afterCheck {
		// Model a signal arriving immediately after the caller sampled Err.
		return observed
	}
	return c.Context.Err()
}

func TestGracefulStopDiscoverySuccessfulShardPlanRoutes(t *testing.T) {
	for _, route := range []string{"local", "local-changed", "daemon", "connect", "auto-connect"} {
		t.Run(route, func(t *testing.T) {
			root := writeDiscoveryStopProject(t, "67.0")
			base, cancel := context.WithCancel(context.Background())
			defer cancel()
			var ctx context.Context = base
			var stderr bytes.Buffer
			var progress io.Writer = &stderr
			args := []string{"test", "--project", root, "--json", "--write-class-shards", filepath.Join(t.TempDir(), "shards"), "--shard-count", "2"}
			if route == "connect" || route == "auto-connect" {
				stub := startProtocolV1Stub(t, root, func(request testdaemon.RequestV1) testdaemon.ResponseV1 {
					response := testdaemon.ResponseV1{Version: testdaemon.ProtocolVersionV1, Op: testdaemon.OpPong, OK: true, Ready: true}
					if request.Op == testdaemon.OpRun {
						response.Op = testdaemon.OpRunResult
						response.ShardPlan = &testdaemon.ClassShardPlanV1{Shards: []testdaemon.ClassShardV1{{Index: 0}, {Index: 1}}}
						cancel()
					}
					return response
				})
				defer stub.Close()
				if route == "connect" {
					args = append(args, "--connect")
				}
			} else {
				checks := 4 // The last local loader checkpoint precedes discovery.
				if route == "daemon" {
					checks = 7 // The last daemon checkpoint follows shard planning.
					args = append(args, "--daemon")
				} else {
					args = append(args, "--no-serve")
				}
				if route == "local-changed" {
					runGitCLI(t, root, "init", "--quiet")
					runGitCLI(t, root, "add", ".")
					runGitCLI(t, root, "-c", "user.name=Glade Test", "-c", "user.email=glade@example.test", "commit", "--quiet", "-m", "synthetic baseline")
					args = append(args, "--changed-since", "HEAD")
				}
				checkpoint := &discoveryCheckpointContext{Context: base, remaining: checks, cancel: cancel, afterCheck: true}
				ctx = checkpoint
				progress = &cancelOnPhaseWriter{phase: "Discovering tests", cancel: func() { checkpoint.armed = true }}
				args = append(args, "--progress")
			}
			var stdout bytes.Buffer
			code := Run(ctx, args, &stdout, progress)
			if base.Err() != context.Canceled || code != 1 {
				t.Errorf("shard plan stop: context=%v exit=%d stdout=%s stderr=%s", base.Err(), code, &stdout, &stderr)
			}
			assertDiscoveryStoppedOutput(t, stdout.Bytes(), "json")
		})
	}
}

func TestGracefulStopDiscoveryEmptyClassFileRoutes(t *testing.T) {
	for _, format := range []string{"json", "console"} {
		t.Run(format, func(t *testing.T) {
			root := writeDiscoveryStopProject(t, "67.0")
			classes := filepath.Join(t.TempDir(), "classes.txt")
			writeTestFile(t, classes, generatedEmptyClassShardMarker+"\n")
			base, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx := &discoveryCheckpointContext{Context: base, armed: true, remaining: 1, cancel: cancel, afterCheck: true}
			args := []string{"test", "--project", root, "--class-file", classes, "--no-progress"}
			if format == "json" {
				args = append(args, "--json")
			}
			var stdout, stderr bytes.Buffer
			if code := Run(ctx, args, &stdout, &stderr); code != 1 {
				t.Errorf("empty class-file stop exit=%d stdout=%s stderr=%s", code, &stdout, &stderr)
			}
			assertDiscoveryStoppedOutput(t, stdout.Bytes(), format)
		})
	}
}

func TestGracefulStopDiscoveryDevArtifactErrorRoutes(t *testing.T) {
	for _, mode := range []string{"test", "watch", "watch-once"} {
		t.Run(mode, func(t *testing.T) {
			root := writeDiscoveryStopProject(t, "67.0")
			out := filepath.Join(t.TempDir(), "blocked-output")
			writeTestFile(t, out, "existing file")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			progress := &cancelOnPhaseWriter{phase: "Discovering tests", cancel: cancel}
			args := []string{"--project", root, "--out", out, "--progress"}
			if mode != "test" {
				args = append(args, "--"+mode)
				cancel()
			}
			var stdout bytes.Buffer
			_, err := runDevTest(ctx, args, &stdout, progress)
			if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "blocked-output") {
				t.Errorf("lost cancellation or artifact failure: %v", err)
			}
			assertDiscoveryStoppedOutput(t, stdout.Bytes(), "console")
		})
	}
}

func TestGracefulStopDiscoveryControlCommandsDoNotRun(t *testing.T) {
	for _, command := range []string{"clear-cache", "daemon", "serve", "--ui", "--help", "changed --help", "failed --help", "changed -h", "failed -h"} {
		t.Run(command, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			var stdout, stderr bytes.Buffer
			_, err := runTest(ctx, strings.Fields(command), &stdout, &stderr)
			if !errors.Is(err, context.Canceled) || stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("cancelled control command ran: err=%v stdout=%s stderr=%s", err, &stdout, &stderr)
			}
		})
	}
}

func TestGracefulStopDiscoveryPreservesProjectErrors(t *testing.T) {
	for _, route := range []string{"no-serve", "read-only", "perf", "watch", "daemon", "dev-test", "dev-watch"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), "{")
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), discoveryStopArgs(t, root, route, "console"), &stdout, &stderr)
			if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "unexpected end of JSON input") {
				t.Fatalf("project error changed into stop: exit=%d stdout=%s stderr=%s", code, &stdout, &stderr)
			}
		})
	}
}

func discoveryStopArgs(t *testing.T, root, route, format string) []string {
	t.Helper()
	args := []string{"test", "--project", root, "--progress"}
	if format == "json" {
		args = append(args, "--json")
	}
	switch route {
	case "no-serve", "no-cache", "read-only", "watch", "watch-once", "daemon", "connect":
		args = append(args, "--"+route)
	case "daemon-watch", "daemon-watch-once":
		args = append(args, "--daemon", "--"+strings.TrimPrefix(route, "daemon-"))
	case "perf", "impact":
		args = append(args, "--"+route+"-json", filepath.Join(t.TempDir(), route+".json"))
	case "shards":
		args = append(args, "--no-serve", "--write-class-shards", filepath.Join(t.TempDir(), "shards"), "--shard-count", "2")
	case "dev-test", "dev-watch", "dev-watch-once":
		args = []string{"dev", "test", "--project", root, "--progress", "--out", filepath.Join(t.TempDir(), "runs")}
		if route != "dev-test" {
			args = append(args, "--"+strings.TrimPrefix(route, "dev-"))
		}
	}
	return args
}

func writeDiscoveryStopProject(t *testing.T, api string) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
	classes := filepath.Join(root, "force-app/main/default/classes")
	writeTestFile(t, filepath.Join(classes, "DiscoveryTest.cls"), `@IsTest private class DiscoveryTest {
 @IsTest static void passes() { System.assertEquals(2, 1 + 1); }
}`)
	writeTestFile(t, filepath.Join(classes, "DiscoveryTest.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
	return root
}

// Give cancellation and deadline expiry the same deterministic trigger. Done
// still closes normally; Err exposes the requested terminal context error.
type discoveryCauseContext struct {
	context.Context
	cause error
}

func (c discoveryCauseContext) Err() error {
	if c.Context.Err() != nil {
		return c.cause
	}
	return nil
}

func discoveryStopContext(t *testing.T, cause error) (context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return discoveryCauseContext{Context: ctx, cause: cause}, cancel
}

func assertDiscoveryStoppedOutput(t *testing.T, output []byte, format string) {
	t.Helper()
	if format == "console" {
		assertQueuedStopOutput(t, output, format)
		if strings.Count(string(output), "Test run stopped (") != 1 {
			t.Fatalf("expected exactly one final status: %s", output)
		}
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	var envelope struct {
		Status    string             `json:"status"`
		Event     string             `json:"event"`
		Reason    string             `json:"reason"`
		RunID     int                `json:"runId"`
		ExitCode  int                `json:"exitCode"`
		Summary   testreport.Summary `json:"summary"`
		Tests     []testreport.Case  `json:"tests"`
		Artifacts []json.RawMessage  `json:"artifacts"`
	}
	if err := decoder.Decode(&envelope); err != nil || envelope.Status != "stopped" || envelope.ExitCode != 1 || envelope.Summary.Total != 0 || len(envelope.Tests) != 0 {
		t.Fatalf("missing final discovery stop: decode=%v stdout=%s", err, output)
	}
	if format == "watch" && (envelope.Event != "watch.stopped" || envelope.Reason == "" || envelope.RunID != 0) {
		t.Errorf("startup watch event=%q reason=%q runId=%d, want watch.stopped before any run starts", envelope.Event, envelope.Reason, envelope.RunID)
	}
	if len(envelope.Artifacts) != 0 {
		t.Errorf("startup stop advertised artifacts it did not produce: %s", output)
	}
	if err := decoder.Decode(&envelope); err != io.EOF {
		t.Errorf("expected exactly one final status: decode=%v stdout=%s", err, output)
	}
}
