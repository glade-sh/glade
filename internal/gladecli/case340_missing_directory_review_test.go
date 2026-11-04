//go:build darwin || linux

package gladecli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestCase340MissingDiscoveryDirectoryReportsStopped(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing-project")
	var status bytes.Buffer
	if err := writeTestDaemonStatus(context.Background(), root, &status); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status.String(), "test daemon: stopped\n") || !strings.Contains(status.String(), "pid: none\n") {
		t.Fatalf("unexpected status: %q", status.String())
	}
	var stop bytes.Buffer
	if err := stopTestDaemon(context.Background(), root, &stop); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stop.String(), "test daemon: stopped\n") {
		t.Fatalf("unexpected stop output: %q", stop.String())
	}
}
