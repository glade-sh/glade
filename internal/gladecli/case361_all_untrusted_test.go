//go:build darwin || linux

package gladecli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testdaemon"
)

// Every discovery location is untrusted: optional discovery runs in-process,
// required discovery and the status and stop commands refuse.
func TestCase361AllDiscoveryLocationsUntrusted(t *testing.T) {
	volume, err := os.MkdirTemp("/tmp", "r361")
	if err != nil {
		t.Skip("no short temp directory")
	}
	defer os.RemoveAll(volume)
	var dirs []string
	for _, name := range []string{"h", "t", "p"} {
		dir := filepath.Join(volume, name)
		if err := os.Mkdir(dir, 0o775); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, 0o775); err != nil {
			t.Fatal(err)
		}
		dirs = append(dirs, dir)
	}
	t.Setenv("HOME", dirs[0])
	t.Setenv("XDG_CACHE_HOME", dirs[0])
	t.Setenv("TMPDIR", dirs[1])
	root := dirs[2]
	_, connected, err := tryTestServerRequest(context.Background(), root, false, testdaemon.RunRequestV1{})
	if err != nil || connected {
		t.Fatalf("optional discovery: connected=%v err=%v", connected, err)
	}
	if _, connected, err = tryTestServerRequest(context.Background(), root, true, testdaemon.RunRequestV1{}); err == nil || !strings.Contains(err.Error(), "insecure") {
		t.Fatalf("required discovery: connected=%v err=%v", connected, err)
	}
	if err := writeTestDaemonStatus(context.Background(), root, io.Discard); err == nil || !strings.Contains(err.Error(), "insecure") {
		t.Fatalf("status: %v", err)
	}
	if err := stopTestDaemon(context.Background(), root, io.Discard); err == nil || !strings.Contains(err.Error(), "insecure") {
		t.Fatalf("stop: %v", err)
	}
}
