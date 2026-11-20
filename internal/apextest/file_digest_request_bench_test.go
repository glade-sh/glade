package apextest

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/filestamp"
)

// parentRequestDigest preserves the baseline request lookup for differential tests.
func parentRequestDigest(r *requestFileDigests, path string) ([sha256.Size]byte, error) {
	if stamp, ok := filestamp.Stat(path); ok {
		r.mu.Lock()
		entry, hit := r.entries[path]
		r.mu.Unlock()
		if hit && entry.stamp == stamp {
			return entry.digest, nil
		}
	}
	digest, err, stamp, stamped := currentFileDigestStamped(path)
	if stamped && err == nil {
		r.mu.Lock()
		if r.entries == nil {
			r.entries = make(map[string]requestFileDigestEntry)
		}
		r.entries[path] = requestFileDigestEntry{stamp: stamp, digest: digest}
		r.mu.Unlock()
	}
	return digest, err
}

func TestRequestFileDigestMatchesParent(t *testing.T) {
	trustedClock(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "A.cls")
	if err := os.WriteFile(path, []byte("public class A {}"), 0600); err != nil {
		t.Fatal(err)
	}
	var parent, actual requestFileDigests
	check := func(path string) {
		t.Helper()
		want, wantErr := parentRequestDigest(&parent, path)
		got, gotErr := actual.digest(path)
		if got != want || fmt.Sprint(gotErr) != fmt.Sprint(wantErr) || !reflect.DeepEqual(parent.entries, actual.entries) {
			t.Fatalf("%s: digest %x/%x errors %v/%v entries equal %v", path, got, want, gotErr, wantErr, reflect.DeepEqual(parent.entries, actual.entries))
		}
	}
	check(path)
	check(path)
	ctimeTick()
	if err := os.WriteFile(path, []byte("public class B {}"), 0600); err != nil {
		t.Fatal(err)
	}
	check(path)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	check(path)
	check(dir)
}

func TestRequestFileDigestConcurrentHits(t *testing.T) {
	trustedClock(t)
	path := filepath.Join(t.TempDir(), "A.cls")
	content := []byte("public class A {}")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	var request requestFileDigests
	if _, err := request.digest(path); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				got, err := request.digest(path)
				if err != nil || got != sha256.Sum256(content) {
					t.Errorf("digest = %x, %v", got, err)
				}
			}
		}()
	}
	wg.Wait()
}

func BenchmarkRequestFileDigestWarmProject(b *testing.B) {
	resetFileDigestTrust()
	oldNow := fileDigestNow
	fileDigestNow = func() time.Time { return time.Now().Add(time.Hour) }
	b.Cleanup(func() { fileDigestNow = oldNow; resetFileDigestTrust() })
	dir := b.TempDir()
	paths := make([]string, 0, 1531*2)
	for i := 0; i < 1531; i++ {
		for _, suffix := range []string{".cls", ".cls-meta.xml"} {
			path := filepath.Join(dir, fmt.Sprintf("Class%04d%s", i, suffix))
			if err := os.WriteFile(path, []byte("public class A {}"), 0600); err != nil {
				b.Fatal(err)
			}
			if _, err := currentFileDigest(path); err != nil {
				b.Fatal(err)
			}
			paths = append(paths, path)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		var request requestFileDigests
		for _, path := range paths {
			if _, err := request.digest(path); err != nil {
				b.Fatal(err)
			}
		}
	}
}
