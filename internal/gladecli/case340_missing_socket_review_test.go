//go:build darwin || linux

package gladecli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testdaemon"
)

func TestCase340MissingSocketCannotReadOrRemoveUnsafePID(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("HOME", cache)
	t.Setenv("XDG_CACHE_HOME", cache)
	root, err := os.MkdirTemp(os.TempDir(), "r340-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	dir := filepath.Dir(testdaemon.ServeSocketPath(root))
	if !strings.HasPrefix(dir, root+string(filepath.Separator)) {
		defer os.RemoveAll(dir)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	pid := testdaemon.ServePIDPath(root)
	if err := os.WriteFile(pid, []byte("999999999\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0777); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		call func(*bytes.Buffer) error
	}{
		{"status", func(out *bytes.Buffer) error { return writeTestDaemonStatus(context.Background(), root, out) }},
		{"stop", func(out *bytes.Buffer) error { return stopTestDaemon(context.Background(), root, out) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := tc.call(&out)
			if err != nil {
				t.Errorf("error=%v, output=%q", err, out.String())
			}
			if strings.Contains(out.String(), "999999999") {
				t.Errorf("read untrusted pid: %q", out.String())
			}
			if _, err := os.Stat(pid); err != nil {
				t.Errorf("removed pid: %v", err)
			}
		})
	}
}
