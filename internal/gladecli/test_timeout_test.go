package gladecli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testreport"
)

func unsetTestTimeoutEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GLADE_TEST_TIMEOUT", "unused")
	if err := os.Unsetenv("GLADE_TEST_TIMEOUT"); err != nil {
		t.Fatal(err)
	}
}

func TestTestTimeoutPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		env    string
		args   []string
		wantMS int64
		source string
	}{
		{name: "default", wantMS: 600_000, source: "default"},
		{name: "environment", env: "90s", wantMS: 90_000, source: "environment"},
		{name: "flag_over_environment", env: "90s", args: []string{"--test-timeout", "15m"}, wantMS: 900_000, source: "flag"},
		{name: "flag_over_invalid_environment", env: "invalid", args: []string{"--test-timeout", "2m"}, wantMS: 120_000, source: "flag"},
		{name: "flag_over_negative_environment", env: "-1s", args: []string{"--test-timeout", "2m"}, wantMS: 120_000, source: "flag"},
		{name: "environment_zero", env: "0", wantMS: 0, source: "environment"},
		{name: "flag_zero", env: "90s", args: []string{"--test-timeout", "0"}, wantMS: 0, source: "flag"},
		{name: "flag_over_environment_zero", env: "0", args: []string{"--test-timeout", "2m"}, wantMS: 120_000, source: "flag"},
		{name: "sub_millisecond_cap", args: []string{"--test-timeout=1ns"}, wantMS: 1, source: "flag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unsetTestTimeoutEnv(t)
			if tc.env != "" {
				t.Setenv("GLADE_TEST_TIMEOUT", tc.env)
			}
			root := t.TempDir()
			writeServeTestProject(t, root)
			stub := startProtocolV1Stub(t, root, func(request testdaemon.RequestV1) testdaemon.ResponseV1 {
				response := testdaemon.ResponseV1{Version: testdaemon.ProtocolVersionV1, OK: true, Ready: true, Project: root, ID: request.ID}
				if request.Op == testdaemon.OpPing {
					response.Op = testdaemon.OpPong
				} else {
					response.Op = testdaemon.OpRunResult
					response.Run = &testreport.Run{}
				}
				return response
			})
			defer stub.Close()
			args := []string{"test", "--project", root, "--connect", "--json", "--no-progress"}
			args = append(args, tc.args...)
			var stdout, stderr bytes.Buffer
			if code := Run(context.Background(), args, &stdout, &stderr); code != 0 {
				t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			request := stub.WaitFor(t, testdaemon.OpRun)
			if request.Run == nil || request.Run.TimeoutMS != tc.wantMS || request.Run.TimeoutSource != tc.source {
				t.Fatalf("run policy = %#v, want timeout=%d source=%q", request.Run, tc.wantMS, tc.source)
			}
		})
	}
}

func TestTestTimeoutInvalidEnvironment(t *testing.T) {
	for _, value := range []string{"", "invalid", "-1s", "10", "999999999999999999999h"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("GLADE_TEST_TIMEOUT", value)
			for _, args := range [][]string{{"test"}, {"dev", "test"}, {"dev", "watch", "--watch-once"}} {
				var stdout, stderr bytes.Buffer
				code := Run(context.Background(), args, &stdout, &stderr)
				if code == 0 || !strings.Contains(stderr.String(), "GLADE_TEST_TIMEOUT") {
					t.Fatalf("args=%v exit=%d stdout=%s stderr=%s", args, code, stdout.String(), stderr.String())
				}
			}
		})
	}
}

func TestTestTimeoutInvalidFlag(t *testing.T) {
	t.Setenv("GLADE_TEST_TIMEOUT", "15m")
	for _, value := range []string{"", "invalid", "-1s"} {
		for _, command := range [][]string{{"test"}, {"dev", "test"}, {"dev", "watch", "--watch-once"}} {
			args := append(append([]string{}, command...), "--test-timeout", value)
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), args, &stdout, &stderr)
			if code == 0 || !strings.Contains(stderr.String(), "--test-timeout") {
				t.Fatalf("args=%v exit=%d stdout=%s stderr=%s", args, code, stdout.String(), stderr.String())
			}
		}
	}
}

func TestTestTimeoutMilliseconds(t *testing.T) {
	for _, tc := range []struct {
		duration time.Duration
		wantMS   int64
	}{
		{0, 0},
		{time.Nanosecond, 1},
		{90 * time.Second, 90_000},
		{time.Duration(1<<63 - 1), 9_223_372_036_854},
	} {
		if got := testTimeoutOptions(tc.duration, "flag"); got.TimeoutMS != tc.wantMS {
			t.Fatalf("duration=%s options=%#v, want %d ms", tc.duration, got, tc.wantMS)
		}
	}
}

func TestTestTimeoutChangedForwarding(t *testing.T) {
	for _, value := range []string{"2m", "0", ""} {
		args, err := rewriteChangedTestArgs([]string{"--test-timeout=" + value})
		want := []string{"--project", ".", "--changed-since", "HEAD", "--test-timeout", value}
		if err != nil || !reflect.DeepEqual(args, want) {
			t.Fatalf("changed timeout=%q args=%v err=%v, want %v", value, args, err, want)
		}
	}
}

func TestTestTimeoutDevWatch(t *testing.T) {
	for _, tc := range []struct {
		name    string
		env     string
		flag    []string
		setting string
	}{
		{name: "environment", env: "20ms", setting: "GLADE_TEST_TIMEOUT environment variable"},
		{name: "flag_over_invalid_environment", env: "invalid", flag: []string{"--test-timeout=20ms"}, setting: "--test-timeout flag"},
		{name: "zero_flag_over_invalid_environment", env: "invalid", flag: []string{"--test-timeout", "0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GLADE_TEST_TIMEOUT", tc.env)
			root := t.TempDir()
			writeServeTestProject(t, root)
			if tc.setting != "" {
				// Execute enough instructions to poll cancellation before the
				// separate loop iteration guard can end the method.
				source := "@IsTest private class TimeoutPolicyTest { @IsTest static void hangs() { Integer work = 0; while (true) {\n" + strings.Repeat("work = work + 1;\n", 64) + "} } }"
				writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/TimeoutPolicyTest.cls"), source)
			}
			args := []string{"--project", root, "--out", t.TempDir(), "--watch-once", "--no-progress"}
			args = append(args, tc.flag...)
			var stdout, stderr bytes.Buffer
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			run, err := runDevTest(ctx, args, &stdout, &stderr)
			if err != nil {
				t.Fatalf("dev watch: %v stdout=%s stderr=%s", err, stdout.String(), stderr.String())
			}
			if tc.setting == "" {
				if summary := run.Summary(); summary.Passed != 1 || !summary.OK() {
					t.Fatalf("zero cap did not run the test: %#v", summary)
				}
				return
			}
			for _, suite := range run.Suites {
				for _, testCase := range suite.Cases {
					if testCase.ClassName == "TimeoutPolicyTest" {
						if testCase.Reason != testreport.ReasonTimeout || testCase.Problem == nil || !strings.Contains(testCase.Problem.Message, tc.setting) {
							t.Fatalf("dev watch lost cap or setting: %#v", testCase)
						}
						return
					}
				}
			}
			t.Fatalf("dev watch omitted timed-out test: %#v", run)
		})
	}
}
