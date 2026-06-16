package gladecli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/startupcache"
)

func TestTestClearCache(t *testing.T) {
	root := t.TempDir()
	entry := startupcache.Entry{Version: startupcache.Version, ProjectRoot: root}
	if err := startupcache.Write(&entry, startupcache.SubdirTest); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	cacheDir := filepath.Join(root, ".glade", "test")
	if err := os.WriteFile(filepath.Join(cacheDir, "startup.gob"), []byte("legacy"), 0o644); err != nil {
		t.Fatalf("WriteFile() legacy gob error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "startup.meta.json")); err != nil {
		t.Fatalf("startup.meta.json missing before clear: %v", err)
	}
	payloads, err := filepath.Glob(filepath.Join(cacheDir, "startup.payload.*"))
	if err != nil {
		t.Fatalf("Glob() payloads error = %v", err)
	}
	if len(payloads) == 0 {
		t.Fatal("no startup payload files before clear")
	}
	var stdout bytes.Buffer
	code := Run(context.Background(), []string{"test", "clear-cache", "--project", root}, &stdout, &stderrDiscard{})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Cleared test startup cache") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "startup.gob")); !os.IsNotExist(err) {
		t.Fatalf("startup.gob still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "startup.meta.json")); !os.IsNotExist(err) {
		t.Fatalf("startup.meta.json still present: %v", err)
	}
	payloads, err = filepath.Glob(filepath.Join(cacheDir, "startup.payload.*"))
	if err != nil {
		t.Fatalf("Glob() payloads after clear error = %v", err)
	}
	if len(payloads) != 0 {
		t.Fatalf("startup payload files still present: %v", payloads)
	}
}

func TestTestClearCacheRejectsFlagTokenAsProjectValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "clear-cache", "--project", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "--project requires a value") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

type stderrDiscard struct{}

func (stderrDiscard) Write(p []byte) (int, error) { return len(p), nil }
