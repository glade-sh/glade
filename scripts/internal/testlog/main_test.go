package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIQuietLogSuppressesSuccessfulTestDetails(t *testing.T) {
	input := strings.Join([]string{
		`{"Action":"run","Package":"example.test/pkg","Test":"TestQuiet"}`,
		`{"Action":"output","Package":"example.test/pkg","Test":"TestQuiet","Output":"=== RUN   TestQuiet\n"}`,
		`{"Action":"output","Package":"example.test/pkg","Test":"TestQuiet","Output":"    quiet_test.go:12: noisy success log\n"}`,
		`{"Action":"pass","Package":"example.test/pkg","Test":"TestQuiet","Elapsed":0.01}`,
		`{"Action":"output","Package":"example.test/pkg","Output":"ok  \texample.test/pkg\t0.010s\n"}`,
		`{"Action":"pass","Package":"example.test/pkg","Elapsed":0.01}`,
	}, "\n") + "\n"
	out, raw, err := runRenderer(t, input, false)
	if err != nil {
		t.Fatalf("renderer failed: %v\n%s", err, out)
	}
	if strings.Contains(out, "noisy success log") || strings.Contains(out, "=== RUN") {
		t.Fatalf("quiet output retained per-test details:\n%s", out)
	}
	if !strings.Contains(out, "ok  \texample.test/pkg\t0.010s") {
		t.Fatalf("quiet output omitted package summary:\n%s", out)
	}
	if raw != input {
		t.Fatalf("raw artifact changed\ngot:  %q\nwant: %q", raw, input)
	}
}

func TestCIVerboseLogRestoresSuccessfulTestDetails(t *testing.T) {
	input := `{"Action":"output","Package":"example.test/pkg","Test":"TestVerbose","Output":"    verbose_test.go:9: verbose detail\n"}` + "\n"
	out, raw, err := runRenderer(t, input, true)
	if err != nil {
		t.Fatalf("renderer failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "verbose_test.go:9: verbose detail") {
		t.Fatalf("verbose output omitted test detail:\n%s", out)
	}
	if raw != input {
		t.Fatalf("raw artifact changed\ngot:  %q\nwant: %q", raw, input)
	}
}

func TestCIFailureLogRetainsNameFileMessageAndEmittedLog(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ci-testlog", "failure.json"))
	if err != nil {
		t.Fatal(err)
	}
	out, raw, err := runRenderer(t, string(fixture), false)
	if err != nil {
		t.Fatalf("renderer failed: %v\n%s", err, out)
	}
	for _, want := range []string{"TestDeliberateFailure", "failure_test.go:17", "values differ", "diagnostic log line"} {
		if !strings.Contains(out, want) {
			t.Errorf("failure output missing %q:\n%s", want, out)
		}
	}
	if raw != string(fixture) {
		t.Fatal("raw failure artifact changed")
	}
}

func TestCIPackageFailureFlushesUnterminatedTestLog(t *testing.T) {
	input := strings.Join([]string{
		`{"Action":"output","Package":"example.test/pkg","Test":"TestInterrupted","Output":"=== RUN   TestInterrupted\n"}`,
		`{"Action":"output","Package":"example.test/pkg","Test":"TestInterrupted","Output":"    interrupted_test.go:21: log before abort\n"}`,
		`{"Action":"fail","Package":"example.test/pkg","Elapsed":30}`,
	}, "\n") + "\n"
	out, _, err := runRenderer(t, input, false)
	if err != nil {
		t.Fatalf("renderer failed: %v\n%s", err, out)
	}
	for _, want := range []string{"TestInterrupted", "interrupted_test.go:21", "log before abort"} {
		if !strings.Contains(out, want) {
			t.Errorf("package failure output missing %q:\n%s", want, out)
		}
	}
}

func TestCIMalformedLogFailsAfterPreservingInput(t *testing.T) {
	input := "not json\n" + `{"Action":"output","Package":"example.test/pkg","Output":"FAIL\texample.test/pkg\n"}` + "\n"
	out, raw, err := runRenderer(t, input, false)
	if err == nil {
		t.Fatalf("malformed input unexpectedly succeeded:\n%s", out)
	}
	if !strings.Contains(out, "invalid JSON event at line 1") {
		t.Fatalf("malformed input error is not explicit:\n%s", out)
	}
	if raw != input {
		t.Fatalf("renderer did not drain and preserve malformed stream\ngot:  %q\nwant: %q", raw, input)
	}
}

func runRenderer(t *testing.T, input string, verbose bool) (string, string, error) {
	t.Helper()
	artifact := filepath.Join(t.TempDir(), "events.json")
	args := []string{"run", ".", "-output", artifact}
	if verbose {
		args = append(args, "-verbose")
	}
	cmd := exec.Command("go", args...)
	cmd.Stdin = strings.NewReader(input)
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	err := cmd.Run()
	raw, readErr := os.ReadFile(artifact)
	if readErr != nil {
		t.Fatalf("read raw artifact: %v", readErr)
	}
	return combined.String(), string(raw), err
}
