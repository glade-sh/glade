package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testreport"
)

func TestGracefulStopQueuedConnectedRoutes(t *testing.T) {
	for _, route := range []string{"connect", "auto-connect"} {
		for _, format := range []string{"json", "console"} {
			for _, cause := range []string{context.Canceled.Error(), context.DeadlineExceeded.Error()} {
				for _, payload := range []string{"absent", "empty"} {
					t.Run(route+"/"+format+"/"+cause+"/"+payload, func(t *testing.T) {
						root := t.TempDir()
						writeServeTestProject(t, root)
						stub := startProtocolV1Stub(t, root, func(request testdaemon.RequestV1) testdaemon.ResponseV1 {
							response := testdaemon.ResponseV1{Version: testdaemon.ProtocolVersionV1, Op: testdaemon.OpPong, OK: true, Ready: true}
							if request.Op == testdaemon.OpRun {
								response.Op, response.OK, response.Error = testdaemon.OpError, false, cause
								if payload == "empty" {
									response.Run = &testreport.Run{}
								}
							}
							return response
						})
						defer stub.Close()
						args := []string{"test", "--project", root, "--no-progress"}
						if route == "connect" {
							args = append(args, "--connect")
						}
						if format == "json" {
							args = append(args, "--json")
						}
						var stdout, stderr bytes.Buffer
						if code := Run(context.Background(), args, &stdout, &stderr); code != 1 {
							t.Fatalf("queued stop exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
						}
						assertQueuedStopOutput(t, stdout.Bytes(), format)
					})
				}
			}
		}
	}
}

func TestGracefulStopQueuedConnectedRejectsNonStopResponses(t *testing.T) {
	for _, route := range []string{"connect", "auto-connect"} {
		for _, failure := range []string{"busy", "missing-result", "wrong-operation"} {
			t.Run(route+"/"+failure, func(t *testing.T) {
				root := t.TempDir()
				writeServeTestProject(t, root)
				stub := startProtocolV1Stub(t, root, func(request testdaemon.RequestV1) testdaemon.ResponseV1 {
					response := testdaemon.ResponseV1{Version: testdaemon.ProtocolVersionV1, Op: testdaemon.OpPong, OK: true, Ready: true}
					if request.Op == testdaemon.OpRun {
						response.Op, response.OK, response.Error = testdaemon.OpError, false, "test server is busy; retry the request"
						if failure == "missing-result" {
							response.Op, response.OK, response.Error = testdaemon.OpRunResult, true, ""
						} else if failure == "wrong-operation" {
							response.Op, response.Error = testdaemon.OpRunResult, context.Canceled.Error()
						}
					}
					return response
				})
				defer stub.Close()
				args := []string{"test", "--project", root, "--json", "--no-progress"}
				if route == "connect" {
					args = append(args, "--connect")
				}
				var stdout, stderr bytes.Buffer
				if code := Run(context.Background(), args, &stdout, &stderr); code != 1 || stdout.Len() != 0 || stderr.Len() == 0 {
					t.Fatalf("non-stop response accepted: exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
				}
			})
		}
	}
}

func TestGracefulStopQueuedConnectedShardPlanRoutes(t *testing.T) {
	for _, route := range []string{"connect", "auto-connect"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			writeServeTestProject(t, root)
			stub := startProtocolV1Stub(t, root, func(request testdaemon.RequestV1) testdaemon.ResponseV1 {
				response := testdaemon.ResponseV1{Version: testdaemon.ProtocolVersionV1, Op: testdaemon.OpPong, OK: true, Ready: true}
				if request.Op == testdaemon.OpRun {
					response.Op, response.OK, response.Error = testdaemon.OpError, false, context.Canceled.Error()
				}
				return response
			})
			defer stub.Close()
			shards := filepath.Join(root, "shards")
			args := []string{"test", "--project", root, "--json", "--no-progress", "--write-class-shards", shards, "--shard-count", "2"}
			if route == "connect" {
				args = append(args, "--connect")
			}
			var stdout, stderr bytes.Buffer
			if code := Run(context.Background(), args, &stdout, &stderr); code != 1 {
				t.Errorf("queued shard stop exit=%d", code)
			}
			assertQueuedStopOutput(t, stdout.Bytes(), "json")
			if _, err := os.Stat(shards); !os.IsNotExist(err) {
				t.Errorf("stopped request wrote shard artifacts: %v", err)
			}
		})
	}
}

func assertQueuedStopOutput(t *testing.T, output []byte, format string) {
	t.Helper()
	if format == "console" {
		if !strings.Contains(string(output), "Test run stopped (") || !strings.Contains(string(output), "0 completed, 0 cancelled, 0 timed out") {
			t.Fatalf("queued stop has no final zero-case console status: %s", output)
		}
		return
	}
	var envelope struct {
		Status   string             `json:"status"`
		ExitCode int                `json:"exitCode"`
		Summary  testreport.Summary `json:"summary"`
		Tests    []testreport.Case  `json:"tests"`
	}
	if err := json.Unmarshal(output, &envelope); err != nil || envelope.Status != "stopped" || envelope.ExitCode != 1 || envelope.Summary.Total != 0 || len(envelope.Tests) != 0 {
		t.Fatalf("queued stop has no final zero-case JSON status: err=%v stdout=%s", err, output)
	}
}

func TestGracefulStopQueuedConnectedNormalEmptyRun(t *testing.T) {
	for _, route := range []string{"connect", "auto-connect"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			writeServeTestProject(t, root)
			stub := startProtocolV1Stub(t, root, func(request testdaemon.RequestV1) testdaemon.ResponseV1 {
				response := testdaemon.ResponseV1{Version: testdaemon.ProtocolVersionV1, Op: testdaemon.OpPong, OK: true, Ready: true}
				if request.Op == testdaemon.OpRun {
					response.Op, response.Run = testdaemon.OpRunResult, &testreport.Run{}
				}
				return response
			})
			defer stub.Close()
			args := []string{"test", "--project", root, "--json", "--no-progress"}
			if route == "connect" {
				args = append(args, "--connect")
			}
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), args, &stdout, &stderr)
			var envelope struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || code != 0 || envelope.Status != "empty" || stderr.Len() != 0 {
				t.Fatalf("normal empty run became a stop: exit=%d decode=%v stdout=%s stderr=%s", code, err, stdout.String(), stderr.String())
			}
		})
	}
}
