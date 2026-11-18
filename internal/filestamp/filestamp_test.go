package filestamp

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStampChangesOnEveryRewrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("aaaa"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, ok := Stat(path)
	if !ok {
		t.Skip("file system without a trusted ctime; stamps are never produced here")
	}
	if again, _ := Stat(path); again != first {
		t.Fatal("stamp of an untouched file changed")
	}
	info, _ := os.Stat(path)
	time.Sleep(30 * time.Millisecond)
	if err := os.WriteFile(path, []byte("bbbb"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if next, _ := Stat(path); next == first {
		t.Fatal("same-size rewrite with restored mtime kept the stamp")
	}
	if _, ok := Stat(filepath.Dir(path)); ok {
		t.Fatal("directory stamped")
	}
	if _, ok := Stat(filepath.Join(filepath.Dir(path), "missing")); ok {
		t.Fatal("missing file stamped")
	}
}

func TestOlderThan(t *testing.T) {
	now := time.Unix(1000, 0)
	s := Stamp{Mtime: now.Add(-3 * time.Second).UnixNano(), Ctime: now.Add(-1 * time.Second).UnixNano()}
	if s.OlderThan(now, 2*time.Second) {
		t.Fatal("recent ctime must make the stamp young")
	}
	s.Ctime = now.Add(-2 * time.Second).UnixNano()
	if !s.OlderThan(now, 2*time.Second) {
		t.Fatal("stamp exactly at the age must count as old")
	}
}
