package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testreport"
)

func TestGracefulStopJSONRetainsCompletedTests(t *testing.T) {
	run := testreport.Run{Suites: []testreport.Suite{{Name: "StoppableTest", Cases: []testreport.Case{
		{ClassName: "StoppableTest", MethodName: "aPass", Status: testreport.StatusPass},
		{ClassName: "StoppableTest", MethodName: "zWait", Status: testreport.StatusUnsupported, Reason: testreport.ReasonCancelled},
	}}}}
	var output bytes.Buffer
	if err := writeTestJSONEnvelope(&output, run, ""); err != nil {
		t.Fatal(err)
	}
	assertGracefulStopEnvelope(t, output.Bytes())
}

func TestGracefulStopProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"local", "daemon", "connect"} {
			for _, cause := range []string{"cancel", "deadline", "daemon-stop"} {
				if cause == "daemon-stop" && route != "connect" {
					continue
				}
				t.Run(api+"/"+route+"/"+cause, func(t *testing.T) {
					t.Cleanup(apextest.InvalidateRuntimeCaches)
					root := writeGracefulStopProject(t, api)
					args := []string{"test", "--project", root, "--json", "--progress-json", "--parallelism", "1", "--test-timeout", "0"}
					// The served route streams no progress to the client, so observe
					// the in-process server. Armed only after the warm run.
					var armed atomic.Pointer[func()]
					if route == "connect" {
						defer startDaemonEquivalenceServerWithProgress(t, root, func(progress apextest.TestProgress) {
							if progress.Event == "test_done" && progress.Status == string(testreport.StatusPass) {
								if fire := armed.Load(); fire != nil {
									(*fire)()
								}
							}
						})()
					}
					switch route {
					case "local":
						args = append(args, "--no-serve")
					case "daemon":
						args = append(args, "--daemon")
					case "connect":
						args = append(args, "--connect")
					}
					// Warm every method before starting the interruption clock.
					var warmOut, warmErr bytes.Buffer
					warmArgs := append([]string{}, args...)
					for i := range warmArgs {
						if warmArgs[i] == "--test-timeout" {
							warmArgs[i+1] = "20ms"
						}
					}
					if code := Run(context.Background(), warmArgs, &warmOut, &warmErr); code != 1 || !strings.Contains(warmOut.String(), `"reason": "timeout"`) {
						t.Fatalf("warm: exit=%d %s %s", code, warmOut.String(), warmErr.String())
					}
					// History schedules slow methods first. Keep aPass first so
					// this route proves that a completed result survives stopping.
					if err := os.Remove(defaultCLIDurationHistoryPath(root)); err != nil {
						t.Fatal(err)
					}
					// Fire the stop once progress reports the first completed
					// test, so slow hosts cannot stop the run before aPass ends.
					ctx, stopNow := newGracefulStopContext(cause == "deadline")
					defer stopNow()
					stopped := make(chan error, 1)
					firstDone := make(chan struct{})
					var stdout bytes.Buffer
					var fireOnce sync.Once
					fire := func() { fireOnce.Do(func() { close(firstDone) }) }
					armed.Store(&fire)
					stderr := &gracefulStopProgressWriter{onFirstDone: fire}
					go func() {
						select {
						case <-firstDone:
						case <-time.After(2 * time.Minute):
							stopped <- fmt.Errorf("no completed test progress before stop")
							stopNow()
							return
						}
						if cause == "daemon-stop" {
							stopCtx, stopCancel := context.WithTimeout(context.Background(), 3*time.Second)
							defer stopCancel()
							_, err := testdaemon.ShutdownV1(stopCtx, testdaemon.ServeSocketPath(root))
							stopped <- err
						} else {
							stopNow()
							stopped <- nil
						}
					}()
					code := Run(ctx, args, &stdout, stderr)
					if err := <-stopped; err != nil {
						t.Fatal(err)
					}
					if code != 1 {
						t.Fatalf("interrupted exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
					}
					assertGracefulStopEnvelope(t, stdout.Bytes())
					if extra := stderr.nonProgress(); extra != "" {
						t.Errorf("bare cancellation diagnostic: %s", extra)
					}
				})
			}
		}
	}
}

// gracefulStopContext reports Canceled or DeadlineExceeded once stopped.
type gracefulStopContext struct {
	context.Context
	done chan struct{}
	once sync.Once
	err  error
}

func newGracefulStopContext(deadline bool) (*gracefulStopContext, func()) {
	c := &gracefulStopContext{Context: context.Background(), done: make(chan struct{}), err: context.Canceled}
	if deadline {
		c.err = context.DeadlineExceeded
	}
	return c, func() { c.once.Do(func() { close(c.done) }) }
}

func (c *gracefulStopContext) Done() <-chan struct{} { return c.done }

func (c *gracefulStopContext) Err() error {
	select {
	case <-c.done:
		return c.err
	default:
		return nil
	}
}

// gracefulStopProgressWriter collects NDJSON progress and signals the first
// completed test. Lines that are not progress events count as diagnostics.
type gracefulStopProgressWriter struct {
	mu          sync.Mutex
	buf         bytes.Buffer
	pending     string
	extra       []string
	fired       bool
	onFirstDone func()
}

func (w *gracefulStopProgressWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf.Write(p)
	w.pending += string(p)
	for {
		i := strings.IndexByte(w.pending, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimSpace(w.pending[:i])
		w.pending = w.pending[i+1:]
		if line == "" {
			continue
		}
		var event struct {
			Kind    string `json:"kind"`
			Label   string `json:"label"`
			Current int    `json:"current"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil || event.Kind == "" {
			w.extra = append(w.extra, line)
			continue
		}
		if !w.fired && event.Kind == "phase_tick" && event.Current >= 1 && event.Label != "tests complete" {
			w.fired = true
			w.onFirstDone()
		}
	}
	return len(p), nil
}

func (w *gracefulStopProgressWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func (w *gracefulStopProgressWriter) nonProgress() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return strings.Join(w.extra, "\n") + w.pending
}

func TestGracefulStopConnectedPartialErrorAndRejection(t *testing.T) {
	for _, route := range []string{"cancelled-partial", "deadline-partial", "completed-partial", "failed-response"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			writeServeTestProject(t, root)
			run := testreport.Run{Suites: []testreport.Suite{{Name: "StoppableTest", Cases: []testreport.Case{
				{ClassName: "StoppableTest", MethodName: "aPass", Status: testreport.StatusPass},
				{ClassName: "StoppableTest", MethodName: "zWait", Status: testreport.StatusUnsupported, Reason: testreport.ReasonCancelled},
			}}}}
			if route == "completed-partial" {
				run.Suites[0].Cases = run.Suites[0].Cases[:1]
			}
			if route == "deadline-partial" {
				run.Suites[0].Cases[1].Reason = testreport.ReasonTimeout
			}
			stub := startProtocolV1Stub(t, root, func(request testdaemon.RequestV1) testdaemon.ResponseV1 {
				response := testdaemon.ResponseV1{Version: testdaemon.ProtocolVersionV1, Op: testdaemon.OpPong, OK: true, Ready: true, Project: root}
				if request.Op == testdaemon.OpRun {
					response.Op, response.OK, response.Error, response.Run = testdaemon.OpError, false, context.Canceled.Error(), &run
					if route == "deadline-partial" {
						response.Error = context.DeadlineExceeded.Error()
					}
					if route == "failed-response" {
						response.Op, response.Error = testdaemon.OpRunResult, "deliberately failed response"
					}
				}
				return response
			})
			defer stub.Close()
			var stdout, stderr bytes.Buffer
			if code := Run(context.Background(), []string{"test", "--project", root, "--connect", "--json", "--no-progress"}, &stdout, &stderr); code != 1 {
				t.Fatalf("failed response exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			if route == "cancelled-partial" || route == "deadline-partial" {
				assertGracefulStopEnvelope(t, stdout.Bytes())
			} else if route == "completed-partial" {
				var envelope struct {
					Status   string             `json:"status"`
					ExitCode int                `json:"exitCode"`
					Summary  testreport.Summary `json:"summary"`
				}
				if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || envelope.Status != "stopped" || envelope.ExitCode != 1 || envelope.Summary.Passed != 1 || envelope.Summary.Total != 1 {
					t.Fatalf("lost completed-only partial result: err=%v stdout=%s", err, stdout.String())
				}
			} else if stdout.Len() != 0 || !strings.Contains(stderr.String(), "deliberately failed response") {
				t.Fatalf("accepted failed run result: stdout=%s stderr=%s", stdout.String(), stderr.String())
			}
		})
	}
}

func TestGracefulStopProjectConsole(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root := writeGracefulStopProject(t, "67.0")
	// Compile the full selection before measuring interruption of execution.
	var warmOut, warmErr bytes.Buffer
	if code := Run(context.Background(), []string{"test", "--project", root, "--no-serve", "--no-progress", "--test-timeout", "20ms", "--json"}, &warmOut, &warmErr); code != 1 || !strings.Contains(warmOut.String(), `"reason": "timeout"`) {
		t.Fatalf("warm: exit=%d stdout=%s stderr=%s", code, &warmOut, &warmErr)
	}
	if err := os.Remove(defaultCLIDurationHistoryPath(root)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var stdout, stderr bytes.Buffer
	code := Run(ctx, []string{"test", "--project", root, "--no-serve", "--no-progress", "--test-timeout", "0"}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stdout.String(), "Test run stopped") || !strings.Contains(stdout.String(), "1 completed") {
		t.Fatalf("missing final stop summary: exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func writeGracefulStopProject(t *testing.T, api string) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
	classes := filepath.Join(root, "force-app/main/default/classes")
	writeTestFile(t, filepath.Join(classes, "StoppableTest.cls"), `@IsTest private class StoppableTest {
 @IsTest static void aPass() { System.assertEquals(2, 1 + 1); }
 @IsTest static void zWait() { Integer work = 0; while (true) {
`+strings.Repeat("work = work + 1;\n", 64)+` } }
}`)
	writeTestFile(t, filepath.Join(classes, "StoppableTest.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
	return root
}

func assertGracefulStopEnvelope(t *testing.T, output []byte) {
	t.Helper()
	var envelope struct {
		Status   string             `json:"status"`
		ExitCode int                `json:"exitCode"`
		Summary  testreport.Summary `json:"summary"`
		Tests    []testreport.Case  `json:"tests"`
	}
	if err := json.Unmarshal(output, &envelope); err != nil {
		t.Fatalf("missing final JSON: %v output=%s", err, output)
	}
	if envelope.Status != "stopped" || envelope.ExitCode != 1 || envelope.Summary.Passed != 1 || envelope.Summary.Total != 2 || len(envelope.Tests) != 2 {
		t.Fatalf("lost final status/completed tests: %s", output)
	}
	for _, testCase := range envelope.Tests {
		switch testCase.MethodName {
		case "aPass":
			if testCase.Status != testreport.StatusPass {
				t.Fatalf("completed method changed after stop: %#v", testCase)
			}
		case "zWait":
			if testCase.Reason != testreport.ReasonCancelled && testCase.Reason != testreport.ReasonTimeout {
				t.Fatalf("interrupted method lost its reason: %#v", testCase)
			}
		default:
			t.Fatalf("unexpected final method: %#v", testCase)
		}
	}
}
