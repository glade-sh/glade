//go:build darwin || linux

package gladecli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testdaemon"
)

func TestCase340OptionalDiscoverySkipsUntrustedDirectory(t *testing.T) {
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
	if err := os.Chmod(dir, 0777); err != nil {
		t.Fatal(err)
	}
	_, connected, err := tryTestServerRequest(context.Background(), root, false, testdaemon.RunRequestV1{})
	if err != nil || connected {
		t.Fatalf("optional discovery should use local fallback: connected=%v err=%v", connected, err)
	}
	_, connected, err = tryTestServerRequest(context.Background(), root, true, testdaemon.RunRequestV1{})
	if err == nil || !connected {
		t.Fatalf("required discovery should reject: connected=%v err=%v", connected, err)
	}
}
