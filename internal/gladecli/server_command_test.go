package gladecli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

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
