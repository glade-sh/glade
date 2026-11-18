package apextest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/filestamp"
)

func TestVisualforceOpenFilePathStampIdentity(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "input")
	if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	before, ok := filestamp.Fstat(file)
	if !ok {
		return // the filesystem exposes no trusted file stamp; nothing to assert
	}
	if !sameVisualforcePathStamp(path, before) {
		t.Fatal("unchanged path did not match open file")
	}
	replacement := filepath.Join(root, "replacement")
	if err := os.WriteFile(replacement, []byte("other"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	if sameVisualforcePathStamp(path, before) {
		t.Fatal("replacement matched open file")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if sameVisualforcePathStamp(path, before) {
		t.Fatal("deleted path matched open file")
	}
	if err := os.Symlink(replacement, path); err != nil {
		t.Fatal(err)
	}
	if sameVisualforcePathStamp(path, before) {
		t.Fatal("dangling link matched open file")
	}
}
