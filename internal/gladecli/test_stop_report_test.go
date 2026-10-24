package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testreport"
)

func TestGracefulStopFinalizerArtifactErrorRoutes(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		for _, format := range []string{"json", "console", "watch"} {
			t.Run(cause.Error()+"/"+format, func(t *testing.T) {
				ctx, cancel := discoveryStopContext(t, cause)
				cancel()
				artifactErr := errors.New("cannot write JUnit")
				var output bytes.Buffer
				err := reportTestStartupStop(ctx, &output, completedStopReport(), artifactErr, format, format == "watch")
				if !errors.Is(err, artifactErr) || !errors.Is(err, cause) {
					t.Errorf("lost artifact error or stop cause: %v", err)
				}
				assertStoppedReport(t, output.Bytes(), format, 1)
			})
		}
	}
}

func TestGracefulStopDevDaemonResponseRoutes(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		for _, population := range []string{"completed", "interrupted", "empty"} {
			t.Run(cause.Error()+"/"+population, func(t *testing.T) {
				root := writeDiscoveryStopProject(t, "67.0")
				result := completedStopReport()
				if population == "empty" {
					result = testreport.Run{}
				} else if population == "interrupted" {
					reason := testreport.ReasonCancelled
					if cause == context.DeadlineExceeded {
						reason = testreport.ReasonTimeout
					}
					result.Suites[0].Cases = append(result.Suites[0].Cases, testreport.Case{
						ClassName: "DiscoveryTest", MethodName: "interrupted", Status: testreport.StatusUnsupported, Reason: reason,
					})
				}
				stub := startProtocolV1Stub(t, root, func(request testdaemon.RequestV1) testdaemon.ResponseV1 {
					response := testdaemon.ResponseV1{Version: testdaemon.ProtocolVersionV1, Op: testdaemon.OpPong, OK: true, Ready: true}
					if request.Op == testdaemon.OpRun {
						response.Op, response.OK, response.Error, response.Run = testdaemon.OpError, false, cause.Error(), &result
					}
					return response
				})
				defer stub.Close()
				outRoot := filepath.Join(t.TempDir(), "runs")
				var output bytes.Buffer
				got, err := runDevTest(context.Background(), []string{"--project", root, "--out", outRoot, "--no-progress"}, &output, io.Discard)
				if !errors.Is(err, cause) || got.Summary() != result.Summary() {
					t.Errorf("lost stopped result: err=%v summary=%+v", err, got.Summary())
				}
				assertStoppedReport(t, output.Bytes(), "console", result.Summary().Passed)
				assertDevStopArtifacts(t, outRoot, result.Summary())
			})
		}
	}
}

// The protocol peer runs the synthetic project through the real daemon runner,
// then stops its own request after the passing method. The CLI context stays live.
func TestGracefulStopDevDaemonProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
			for _, artifacts := range []string{"written", "blocked"} {
				t.Run(api+"/"+cause.Error()+"/"+artifacts, func(t *testing.T) {
					t.Cleanup(apextest.InvalidateRuntimeCaches)
					root := writeDiscoveryStopProject(t, api)
					startCompletedStopPeer(t, root, cause, nil)
					outRoot := filepath.Join(t.TempDir(), "runs")
					if artifacts == "blocked" {
						writeTestFile(t, outRoot, "occupied output")
					}
					var stdout, stderr bytes.Buffer
					code := Run(context.Background(), []string{"dev", "test", "--project", root, "--out", outRoot, "--no-progress"}, &stdout, &stderr)
					if code != 1 {
						t.Errorf("stop exit=%d stdout=%s stderr=%s", code, &stdout, &stderr)
					}
					if artifacts == "blocked" && !strings.Contains(stderr.String(), outRoot) {
						t.Errorf("lost artifact error: %s", &stderr)
					}
					assertStoppedReport(t, stdout.Bytes(), "console", 1)
					if artifacts == "written" {
						assertDevStopArtifacts(t, outRoot, completedStopReport().Summary())
					}
				})
			}
		}
	}
}

func TestGracefulStopJUnitFailureProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"no-serve", "no-cache", "read-only", "perf", "daemon", "auto-local", "connect", "auto-connect", "server-stop"} {
			for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
				for _, format := range []string{"json", "console"} {
					t.Run(api+"/"+route+"/"+cause.Error()+"/"+format, func(t *testing.T) {
						t.Cleanup(apextest.InvalidateRuntimeCaches)
						root := writeDiscoveryStopProject(t, api)
						ctx, cancel := discoveryStopContext(t, cause)
						progress := &cancelOnPhaseWriter{phase: "1 passed", cancel: cancel}
						args := []string{"test", "--project", root, "--parallelism", "1", "--progress"}
						switch route {
						case "no-serve", "no-cache", "read-only", "daemon":
							args = append(args, "--"+route)
						case "perf":
							args = append(args, "--perf-json", filepath.Join(t.TempDir(), "perf.json"))
						case "connect", "auto-connect":
							startCompletedStopPeer(t, root, nil, cancel)
							if route == "connect" {
								args = append(args, "--connect")
							}
						case "server-stop":
							startCompletedStopPeer(t, root, cause, nil)
						}
						blocked := filepath.Join(t.TempDir(), "blocked-junit")
						if err := os.Mkdir(blocked, 0o755); err != nil {
							t.Fatal(err)
						}
						args = append(args, "--junit", blocked)
						if format == "json" {
							args = append(args, "--json")
						}
						var stdout bytes.Buffer
						code := Run(ctx, args, &stdout, progress)
						if code != 1 || !strings.Contains(progress.text.String(), "blocked-junit") {
							t.Errorf("lost JUnit failure: exit=%d stderr=%s", code, progress.text.String())
						}
						if route != "server-stop" && ctx.Err() != cause {
							t.Fatalf("did not stop after completed method: stderr=%s", progress.text.String())
						}
						assertStoppedReport(t, stdout.Bytes(), format, 1)
					})
				}
			}
		}
	}
}

func TestGracefulStopFinalizerOrdinaryErrorRoutes(t *testing.T) {
	for _, format := range []string{"json", "console", "watch"} {
		t.Run(format, func(t *testing.T) {
			artifactErr := errors.New("cannot write JUnit")
			var output bytes.Buffer
			err := reportTestStartupStop(context.Background(), &output, completedStopReport(), artifactErr, format, format == "watch")
			if !errors.Is(err, artifactErr) || output.Len() != 0 {
				t.Fatalf("ordinary artifact error became a stop: err=%v output=%s", err, &output)
			}
		})
	}
}

func TestGracefulStopDevDaemonNonStopRoutes(t *testing.T) {
	for _, route := range []string{"completed", "per-test-timeout", "rejected"} {
		t.Run(route, func(t *testing.T) {
			root := writeDiscoveryStopProject(t, "67.0")
			result := completedStopReport()
			if route != "completed" {
				result.Suites[0].Cases = append(result.Suites[0].Cases, testreport.Case{
					ClassName: "DiscoveryTest", MethodName: "interrupted", Status: testreport.StatusUnsupported, Reason: testreport.ReasonTimeout,
				})
			}
			stub := startProtocolV1Stub(t, root, func(request testdaemon.RequestV1) testdaemon.ResponseV1 {
				response := testdaemon.ResponseV1{Version: testdaemon.ProtocolVersionV1, Op: testdaemon.OpPong, OK: true, Ready: true}
				if request.Op == testdaemon.OpRun {
					response.Op, response.Run = testdaemon.OpRunResult, &result
					if route == "rejected" {
						response.Op, response.OK, response.Error = testdaemon.OpError, false, "deliberately rejected"
					}
				}
				return response
			})
			defer stub.Close()
			outRoot := filepath.Join(t.TempDir(), "runs")
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"dev", "test", "--project", root, "--out", outRoot, "--no-progress"}, &stdout, &stderr)
			wantCode := 1
			if route == "completed" {
				wantCode = 0
			}
			if code != wantCode || strings.Contains(stdout.String(), "Test run stopped") {
				t.Errorf("ordinary response became a stop: exit=%d stdout=%s stderr=%s", code, &stdout, &stderr)
			}
			if route == "rejected" {
				if stdout.Len() != 0 || !strings.Contains(stderr.String(), "deliberately rejected") {
					t.Errorf("lost rejection: stdout=%s stderr=%s", &stdout, &stderr)
				}
				if _, err := readLatest(outRoot); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("rejected response produced artifacts: %v", err)
				}
			} else {
				assertDevStopArtifacts(t, outRoot, result.Summary())
			}
		})
	}
}

func completedStopReport() testreport.Run {
	return testreport.Run{Suites: []testreport.Suite{{Name: "DiscoveryTest", Cases: []testreport.Case{
		{ClassName: "DiscoveryTest", MethodName: "passes", Status: testreport.StatusPass},
	}}}}
}

func startCompletedStopPeer(t *testing.T, root string, serverCause error, cancelClient context.CancelFunc) {
	t.Helper()
	daemon, err := testdaemon.New(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := discoveryStopContext(t, serverCause)
	stub := startProtocolV1Stub(t, root, func(request testdaemon.RequestV1) testdaemon.ResponseV1 {
		response := testdaemon.ResponseV1{Version: testdaemon.ProtocolVersionV1, Op: testdaemon.OpPong, OK: true, Ready: true}
		if request.Op != testdaemon.OpRun {
			return response
		}
		result, _, _, err := daemon.RunRequestV1(ctx, *request.Run, nil, nil)
		response.Op, response.Run = testdaemon.OpRunResult, &result
		if err != nil {
			response.Op, response.OK, response.Error = testdaemon.OpError, false, err.Error()
			return response
		}
		if serverCause != nil {
			cancel()
			response.Op, response.OK, response.Error = testdaemon.OpError, false, ctx.Err().Error()
		}
		if cancelClient != nil {
			cancelClient()
		}
		return response
	})
	t.Cleanup(stub.Close)
}

func assertDevStopArtifacts(t *testing.T, outRoot string, want testreport.Summary) {
	t.Helper()
	latest, err := readLatest(outRoot)
	if err != nil {
		t.Fatalf("stopped dev run lost latest artifacts: %v", err)
	}
	result, err := loadLatestTestResult(latest)
	if err != nil || result.Summary().Total != want.Total || result.Summary().Passed != want.Passed {
		t.Errorf("lost completed artifact results: err=%v summary=%+v", err, result.Summary())
	}
	for _, name := range []string{"summary.md", "junit.xml", "run.json", "selection.json", "events.ndjson"} {
		if _, err := os.Stat(filepath.Join(latest.RunDir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
	if want.Passed > 0 {
		junit, err := os.ReadFile(filepath.Join(latest.RunDir, "junit.xml"))
		if err != nil || !bytes.Contains(junit, []byte("DiscoveryTest")) || !bytes.Contains(junit, []byte("passes")) {
			t.Errorf("JUnit lost completed method: err=%v output=%s", err, junit)
		}
	}
}

func assertStoppedReport(t *testing.T, output []byte, format string, passed int) {
	t.Helper()
	if format == "console" {
		if strings.Count(string(output), "Test run stopped (") != 1 || !strings.Contains(string(output), fmt.Sprintf("%d completed", passed)) {
			t.Errorf("missing single stopped report with completed tests: %s", output)
		}
		return
	}
	var envelope struct {
		Event     string             `json:"event"`
		Reason    string             `json:"reason"`
		Status    string             `json:"status"`
		ExitCode  int                `json:"exitCode"`
		Summary   testreport.Summary `json:"summary"`
		Tests     []testreport.Case  `json:"tests"`
		Artifacts []json.RawMessage  `json:"artifacts"`
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	if err := decoder.Decode(&envelope); err != nil || envelope.Status != "stopped" || envelope.ExitCode != 1 || envelope.Summary.Passed != passed || len(envelope.Tests) < passed {
		t.Errorf("lost stopped report/completed tests: err=%v output=%s", err, output)
		return
	}
	if len(envelope.Artifacts) != 0 {
		t.Errorf("advertised unwritten artifacts: %s", output)
	}
	if format == "watch" && (envelope.Event != "watch.stopped" || envelope.Reason == "") {
		t.Errorf("lost terminal watch event or cause: %s", output)
	}
	if err := decoder.Decode(&envelope); err != io.EOF {
		t.Errorf("expected exactly one final report: err=%v output=%s", err, output)
	}
}
