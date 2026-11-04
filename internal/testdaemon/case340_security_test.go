package testdaemon

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCase340RejectsUntrustedSocketDirectory(t *testing.T) {
	parent, err := os.MkdirTemp(os.TempDir(), "case340-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(parent)
	dir := filepath.Join(parent, "daemon")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(dir, "serve.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := os.Chmod(dir, 0777); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = PingV1(ctx, socket)
	if err == nil || !strings.Contains(err.Error(), "insecure") {
		t.Fatalf("expected insecure socket directory rejection, got %v", err)
	}
}

func TestCase340AllowsGroupWritableDirectoryWithAuthenticatedPeer(t *testing.T) {
	parent, err := os.MkdirTemp(os.TempDir(), "case340-group-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(parent)
	socket := filepath.Join(parent, "serve.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := os.Chmod(parent, 0775); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = PingV1(ctx, socket)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("same-UID peer should reach protocol deadline, got %v", err)
	}
}

func TestCase340RejectsReplaceableDiscoveryDirectories(t *testing.T) {
	for _, rel := range []string{"glade-test-serve-synthetic", filepath.Join(".glade", "test")} {
		t.Run(rel, func(t *testing.T) {
			root, err := os.MkdirTemp(os.TempDir(), "r340-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(root)
			dir := filepath.Join(root, rel)
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			socket := filepath.Join(dir, "serve.sock")
			listener, err := net.Listen("unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			if err := os.Chmod(dir, 0775); err != nil {
				t.Fatal(err)
			}
			err = ValidateDiscoverySocket(socket)
			if err == nil || !strings.Contains(err.Error(), "insecure") {
				t.Fatalf("replaceable discovery directory: %v", err)
			}
		})
	}
}

func TestCase340DiscoveryAllowsReadableNonWritableDirectory(t *testing.T) {
	root, err := os.MkdirTemp(os.TempDir(), "r340-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	socket := filepath.Join(root, "serve.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := ValidateDiscoverySocket(socket); err != nil {
		t.Skipf("runner temporary path is not trusted: %v", err)
	}
	if err := os.Chmod(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDiscoverySocket(socket); err != nil {
		t.Fatalf("non-writable discovery directory: %v", err)
	}
}
