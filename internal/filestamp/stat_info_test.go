package filestamp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatWithInfoUsesSameFileObservation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "input")
	if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	info, stamp, ok := StatWithInfo(path)
	if !ok {
		t.Skip("filesystem does not expose a trusted file stamp")
	}
	if info == nil || info.Size() != stamp.Size || info.ModTime().UnixNano() != stamp.Mtime {
		t.Fatalf("inconsistent file info and stamp: info=%v stamp=%+v", info, stamp)
	}
	want, ok := Stat(path)
	if !ok || stamp != want {
		t.Fatalf("stamp = %+v, want %+v", stamp, want)
	}
	if info, _, ok := StatWithInfo(root); ok || info == nil || !info.IsDir() {
		t.Fatalf("directory: info=%v trusted=%t", info, ok)
	}
	if info, _, ok := StatWithInfo(filepath.Join(root, "missing")); ok || info != nil {
		t.Fatalf("missing: info=%v trusted=%t", info, ok)
	}
}
