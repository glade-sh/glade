package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testreport"
)

// The empty-selection route isolates command finalization from project loading
// and execution. A stop inside Write must not revise an already chosen result.
func TestGracefulStopCompletedEmptySelectionWriterRoutes(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		for _, format := range []string{"json", "console"} {
			t.Run(cause.Error()+"/"+format, func(t *testing.T) {
				classes := filepath.Join(t.TempDir(), "classes.txt")
				writeTestFile(t, classes, generatedEmptyClassShardMarker+"\n")
				ctx, cancel := discoveryStopContext(t, cause)
				output := &completedResultStopWriter{cancel: cancel}
				args := []string{"--class-file", classes, "--no-progress"}
				if format == "json" {
					args = append(args, "--json")
				}
				result, err := runTest(ctx, args, output, io.Discard)
				if ctx.Err() != cause || result.Summary().Total != 0 {
					t.Fatalf("writer boundary not reached: context=%v summary=%+v", ctx.Err(), result.Summary())
				}
				if err != nil {
					t.Errorf("completed report changed to error after writing: %v", err)
				}
				assertCompletedResultOutput(t, output.Bytes(), format, "empty", 0)
			})
		}
	}
}

// Run the owned project through every one-shot command output path, including
// aliases and the dev wrapper. The writer stops only after execution completed.
func TestGracefulStopCompletedProjectWriterRoutes(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{
			"auto-local", "no-serve", "no-cache", "read-only", "perf", "daemon",
			"connect", "auto-connect", "named-method", "cached-parallel", "changed", "failed", "dev-test",
		} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := writeDiscoveryStopProject(t, api)
				args := []string{"test", "--project", root, "--no-progress"}
				switch route {
				case "no-serve", "no-cache", "read-only", "daemon":
					args = append(args, "--"+route)
				case "perf":
					args = append(args, "--perf-json", filepath.Join(t.TempDir(), "perf.json"))
				case "connect", "auto-connect":
					defer startDaemonEquivalenceServer(t, root)()
					if route == "connect" {
						args = append(args, "--connect")
					}
				case "named-method":
					args = append(args, "--no-serve", "--class", "DiscoveryTest", "--method", "passes")
				case "cached-parallel":
					args = append(args, "--no-serve", "--parallelism", "2")
					if code := Run(context.Background(), args, io.Discard, io.Discard); code != 0 {
						t.Fatalf("warm run exit=%d", code)
					}
				case "changed":
					runGitCLI(t, root, "init", "--quiet")
					runGitCLI(t, root, "add", ".")
					runGitCLI(t, root, "-c", "user.name=Glade Test", "-c", "user.email=glade@example.test", "commit", "--quiet", "-m", "synthetic baseline")
					writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/DiscoveryTest.cls"), `@IsTest private class DiscoveryTest {
 @IsTest static void passes() { System.assertEquals(3, 1 + 2); }
}`)
					args = append([]string{"test", "changed"}, args[1:]...)
				case "failed":
					args = append([]string{"test", "failed"}, args[1:]...)
					args = append(args, "--no-serve")
				case "dev-test":
					args = []string{"dev", "test", "--project", root, "--out", filepath.Join(t.TempDir(), "runs"), "--no-progress"}
				}
				for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
					for _, format := range []string{"json", "console"} {
						if route == "dev-test" && format == "json" {
							continue // The dev command exposes a console report only.
						}
						t.Run(cause.Error()+"/"+format, func(t *testing.T) {
							if route == "failed" {
								previous := completedStopReport()
								previous.Suites[0].Cases[0].Status = testreport.StatusFail
								if err := writeLastFailedTests(root, previous); err != nil {
									t.Fatal(err)
								}
							}
							ctx, cancel := discoveryStopContext(t, cause)
							output := &completedResultStopWriter{cancel: cancel}
							var stderr bytes.Buffer
							command := append([]string(nil), args...)
							if format == "json" {
								command = append(command, "--json")
							}
							code := Run(ctx, command, output, &stderr)
							if ctx.Err() != cause {
								t.Fatalf("writer did not stop: exit=%d stdout=%s stderr=%s", code, output, &stderr)
							}
							assertCompletedResultOutput(t, output.Bytes(), format, "passed", 1)
							if code != 0 || stderr.Len() != 0 {
								t.Errorf("published success disagrees with command: exit=%d stderr=%s", code, &stderr)
							}
						})
					}
				}
			})
		}
	}
}

type completedResultStopWriter struct {
	bytes.Buffer
	cancel context.CancelFunc
}

func (w *completedResultStopWriter) Write(p []byte) (int, error) {
	w.cancel()
	return w.Buffer.Write(p)
}

func (w *completedResultStopWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func assertCompletedResultOutput(t *testing.T, data []byte, format, status string, total int) {
	t.Helper()
	if format == "console" {
		if strings.Contains(string(data), "Test run stopped") || !strings.Contains(string(data), "Glade test") {
			t.Fatalf("completed console report changed to a stop: %s", data)
		}
		if total > 0 && !strings.Contains(string(data), "DiscoveryTest.passes") {
			t.Fatalf("completed method missing: %s", data)
		}
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var envelope struct {
		Status   string             `json:"status"`
		ExitCode int                `json:"exitCode"`
		Summary  testreport.Summary `json:"summary"`
		Tests    []testreport.Case  `json:"tests"`
	}
	if err := decoder.Decode(&envelope); err != nil {
		t.Fatalf("invalid final JSON: %v; %s", err, data)
	}
	if envelope.Status != status || envelope.ExitCode != 0 || envelope.Summary.Passed != total || envelope.Summary.Total != total || len(envelope.Tests) != total {
		t.Fatalf("incorrect completed result: %s", data)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("extra output after terminal report: %v; %s", err, data)
	}
}
