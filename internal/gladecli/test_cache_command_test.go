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
	var stdout bytes.Buffer
	code := Run(context.Background(), []string{"test", "clear-cache", "--project", root}, &stdout, &stderrDiscard{})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Cleared test startup cache") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".glade", "test", "startup.gob")); !os.IsNotExist(err) {
		t.Fatalf("startup.gob still present: %v", err)
	}
}

type stderrDiscard struct{}

func (stderrDiscard) Write(p []byte) (int, error) { return len(p), nil }
