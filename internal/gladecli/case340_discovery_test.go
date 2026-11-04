package gladecli

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testdaemon"
)

func TestCase340DiscoveryCallersRejectReplaceableDirectory(t *testing.T) {
	root, err := os.MkdirTemp(os.TempDir(), "r340-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	dir := filepath.Join(root, ".glade", "test")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", testdaemon.ServeSocketPath(root))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := os.Chmod(dir, 0775); err != nil {
		t.Fatal(err)
	}
	for name, call := range map[string]func() error{
		"status": func() error { return writeTestDaemonStatus(context.Background(), root, io.Discard) },
		"stop":   func() error { return stopTestDaemon(context.Background(), root, io.Discard) },
		"wizard": func() error { return writeTestWizard(context.Background(), root, io.Discard) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil || !strings.Contains(err.Error(), "insecure") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestCase340MissingDiscoverySocketFallsBack(t *testing.T) {
	root, err := os.MkdirTemp(os.TempDir(), "r340-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	dir := filepath.Join(root, ".glade", "test")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0775); err != nil {
		t.Fatal(err)
	}
	if err := writeTestDaemonStatus(context.Background(), root, io.Discard); err == nil || !strings.Contains(err.Error(), "insecure") {
		t.Fatalf("unsafe missing-socket directory: %v", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	_, connected, err := tryTestServerRequest(context.Background(), root, false, testdaemon.RunRequestV1{Parallelism: 1})
	if err != nil || connected {
		t.Fatalf("missing daemon: connected=%v err=%v", connected, err)
	}
	if err := writeTestDaemonStatus(context.Background(), root, io.Discard); err != nil {
		t.Fatalf("status: %v", err)
	}
}
