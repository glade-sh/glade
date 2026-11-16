package inputview

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestFingerprintUsesPinnedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source")
	if err := os.WriteFile(path, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	v := New()
	if _, err := v.ReadFile(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("after"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Fingerprint(path); err != nil {
		t.Fatalf("pinned fingerprint: %v", err)
	}
	if err := v.Recheck(); err == nil {
		t.Fatal("recheck did not detect edit")
	}
}

func TestConcurrentViewRecheckAndClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source")
	if err := os.WriteFile(path, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	v := New()
	var wg, started sync.WaitGroup
	started.Add(8)
	release := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_, _ = v.ReadFile(path)
				_, _ = v.Stat(path)
				_, _ = v.ReadDir(dir)
				_ = v.WalkDir(dir, func(string, fs.DirEntry, error) error { return nil })
				_ = v.Err()
				_ = v.CheckDirectories()
				_ = v.Observations()
				_ = v.Recheck()
				if j == 0 {
					started.Done()
					<-release
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		started.Wait()
		close(release)
		v.Close()
	}()
	wg.Wait()
	if got := v.Observations(); len(got) != 0 {
		t.Fatalf("closed view retained %d observations", len(got))
	}
}

func TestReadDirErrorOnFileAgreesWithStat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	v := New()
	if _, err := v.ReadDir(path); err == nil {
		t.Fatal("expected a directory error")
	}
	if _, err := v.Stat(path); err != nil {
		t.Fatalf("stat: %v", err)
	}
	if err := v.Recheck(); err != nil {
		t.Fatalf("unchanged error: %v", err)
	}
}
