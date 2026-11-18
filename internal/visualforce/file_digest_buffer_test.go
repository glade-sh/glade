package visualforce

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func digestBufferFixture(t *testing.T, sizes []int) []string {
	t.Helper()
	dir := t.TempDir()
	var paths []string
	for i, size := range sizes {
		content := make([]byte, size)
		for j := range content {
			content[j] = byte(i*31 + j*7)
		}
		path := filepath.Join(dir, fmt.Sprintf("f%d.cls", i))
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	return paths
}

func TestFileContentDigestEqualsReadFileDigestAcrossBufferSizes(t *testing.T) {
	resetFileDigestMemo()
	t.Cleanup(resetFileDigestMemo)
	sizes := []int{0, 1, 100, 32*1024 - 1, 32 * 1024, 32*1024 + 1, 100 * 1024}
	for _, path := range digestBufferFixture(t, sizes) {
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := fileContentDigest(path)
		if err != nil {
			t.Fatal(err)
		}
		if got != sha256.Sum256(want) {
			t.Fatalf("%s: digest differs from the sha256 of its content", path)
		}
	}
}

func TestFileContentDigestReportsUnreadableFiles(t *testing.T) {
	dir := t.TempDir()
	if _, err := fileContentDigest(filepath.Join(dir, "missing.cls")); err == nil {
		t.Fatal("missing file hashed without error")
	}
	if _, err := fileContentDigest(dir); err == nil {
		t.Fatal("directory hashed without error")
	}
}

// Hashing used to allocate a fresh 32 KiB copy buffer for every file.
func TestFileContentDigestAllocationBudget(t *testing.T) {
	const files = 200
	sizes := make([]int, files)
	for i := range sizes {
		sizes[i] = 4096
	}
	paths := digestBufferFixture(t, sizes)
	run := func() {
		resetFileDigestMemo()
		for _, path := range paths {
			if _, err := fileContentDigest(path); err != nil {
				t.Fatal(err)
			}
		}
	}
	run()
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	run()
	runtime.ReadMemStats(&after)
	t.Cleanup(resetFileDigestMemo)
	perFile := (after.TotalAlloc - before.TotalAlloc) / files
	if perFile > 8*1024 {
		t.Fatalf("hashing allocated %d bytes per file; want at most 8192", perFile)
	}
}
