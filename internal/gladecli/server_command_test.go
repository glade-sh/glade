package gladecli

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDevServersGuardPublicBindsBeforeProjectLoading(t *testing.T) {
	t.Setenv("GLADE_SERVER_PUBLIC", "")
	missingProject := filepath.Join(t.TempDir(), "missing")
	writeTestFile(t, filepath.Join(missingProject, "sfdx-project.json"), "{")
	for name, run := range map[string]func(context.Context, []string, io.Writer, io.Writer) error{
		"vf":  runDevVF,
		"lwc": runDevLWC,
	} {
		t.Run(name, func(t *testing.T) {
			err := run(t.Context(), []string{"--project", missingProject, "--addr", "0.0.0.0:0"}, io.Discard, io.Discard)
			if err == nil || !strings.Contains(err.Error(), "GLADE_SERVER_PUBLIC=1") {
				t.Fatalf("public bind error = %v, want opt-in guidance before project loading", err)
			}
		})
	}
}

func TestDevServersAllowPublicBindOptInBeforeProjectLoading(t *testing.T) {
	t.Setenv("GLADE_SERVER_PUBLIC", "1")
	missingProject := filepath.Join(t.TempDir(), "missing")
	writeTestFile(t, filepath.Join(missingProject, "sfdx-project.json"), "{")
	for name, run := range map[string]func(context.Context, []string, io.Writer, io.Writer) error{
		"vf":  runDevVF,
		"lwc": runDevLWC,
	} {
		t.Run(name, func(t *testing.T) {
			err := run(t.Context(), []string{"--project", missingProject, "--addr", "0.0.0.0:0"}, io.Discard, io.Discard)
			if err == nil || strings.Contains(err.Error(), "GLADE_SERVER_PUBLIC=1") {
				t.Fatalf("opt-in error = %v, want later project loading error", err)
			}
		})
	}
}

func TestLocalHTTPServersSetReadHeaderTimeout(t *testing.T) {
	if localHTTPReadHeaderTimeout != 10*time.Second {
		t.Fatalf("read header timeout = %s, want 10s", localHTTPReadHeaderTimeout)
	}
}

func TestRunServerRejectsPublicBindWithoutOptIn(t *testing.T) {
	t.Setenv("GLADE_SERVER_PUBLIC", "")
	err := validateServerBindAllowed("0.0.0.0:0")
	if err == nil {
		t.Fatal("expected public bind to be rejected")
	}
	if !strings.Contains(err.Error(), "GLADE_SERVER_PUBLIC=1") {
		t.Fatalf("error missing public bind guidance: %v", err)
	}
}

func TestRunPlaygroundPublicRequiresOptIn(t *testing.T) {
	t.Setenv("GLADE_SERVER_PUBLIC", "")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"playground", "--public", "--once", "--no-open", "--data-root", t.TempDir()}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "GLADE_SERVER_PUBLIC=1") {
		t.Fatalf("stderr missing public bind guidance: %q", stderr.String())
	}
}

func TestRunPlaygroundPublicOptInWarns(t *testing.T) {
	t.Setenv("GLADE_SERVER_PUBLIC", "1")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"playground", "--public", "--once", "--no-open", "--data-root", t.TempDir()}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "WARNING") || !strings.Contains(stdout.String(), "network-reachable") {
		t.Fatalf("stdout missing public warning: %q", stdout.String())
	}
}
