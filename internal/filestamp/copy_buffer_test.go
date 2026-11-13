package filestamp

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestCopyPooledMatchesIoCopy(t *testing.T) {
	for _, size := range []int{0, 1, 1000, copyBufferSize - 1, copyBufferSize, copyBufferSize + 1, 3*copyBufferSize + 5} {
		content := bytes.Repeat([]byte{0xa5, 0x01, 0xfe}, size/3+1)[:size]
		var want, got bytes.Buffer
		if _, err := io.Copy(&want, bytes.NewReader(content)); err != nil {
			t.Fatal(err)
		}
		n, err := CopyPooled(&got, bytes.NewReader(content))
		if err != nil || n != int64(size) || !bytes.Equal(want.Bytes(), got.Bytes()) {
			t.Fatalf("size %d: n=%d err=%v equal=%v", size, n, err, bytes.Equal(want.Bytes(), got.Bytes()))
		}
	}
}

func TestCopyPooledReportsReadError(t *testing.T) {
	boom := errors.New("boom")
	_, err := CopyPooled(io.Discard, io.MultiReader(bytes.NewReader([]byte("abc")), errReader{boom}))
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

// Concurrent hashing of different files must not share a buffer.
func TestCopyPooledConcurrentDigestsStayIndependent(t *testing.T) {
	dir := t.TempDir()
	const files = 8
	want := make([][sha256.Size]byte, files)
	paths := make([]string, files)
	for i := range files {
		content := bytes.Repeat([]byte{byte(i + 1)}, 70000+i)
		paths[i] = filepath.Join(dir, string(rune('a'+i)))
		if err := os.WriteFile(paths[i], content, 0o600); err != nil {
			t.Fatal(err)
		}
		want[i] = sha256.Sum256(content)
	}
	var wg sync.WaitGroup
	for g := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for round := range 50 {
				i := (g + round) % files
				f, err := os.Open(paths[i])
				if err != nil {
					t.Error(err)
					return
				}
				h := sha256.New()
				_, err = CopyPooled(h, f)
				f.Close()
				var got [sha256.Size]byte
				h.Sum(got[:0])
				if err != nil || got != want[i] {
					t.Errorf("file %d: err=%v digest mismatch", i, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// The pooled copy of an *os.File must not allocate a buffer per call.
func TestCopyPooledFileAllocatesNoBuffer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, make([]byte, 4096), 0o600); err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	allocs := testing.AllocsPerRun(50, func() {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		h.Reset()
		if _, err := CopyPooled(h, f); err != nil {
			t.Fatal(err)
		}
		f.Close()
	})
	if allocs > 8 {
		t.Fatalf("%v allocations per copy", allocs)
	}
}
