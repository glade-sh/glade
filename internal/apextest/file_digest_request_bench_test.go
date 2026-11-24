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

func TestRequestFileDigestLargeProjectIdentity(t *testing.T) {
	trustedClock(t)
	dir := t.TempDir()
	var parent, actual requestFileDigests
	for i := 0; i < 1531; i++ {
		for _, suffix := range []string{".cls", ".cls-meta.xml"} {
			path := filepath.Join(dir, fmt.Sprintf("Class%04d%s", i, suffix))
			content := []byte(fmt.Sprintf("file %d %s", i, suffix))
			if err := os.WriteFile(path, content, 0600); err != nil {
				t.Fatal(err)
			}
			want, parentErr := parentRequestDigest(&parent, path)
			got, err := actual.digest(path)
			if parentErr != nil || err != nil || got != want || got != sha256.Sum256(content) {
				t.Fatalf("%s: got %x, %v; parent %x, %v", path, got, err, want, parentErr)
			}
		}
	}
	if !reflect.DeepEqual(parent.entries, actual.entries) {
		t.Fatal("request cache entries differ from parent")
	}
}

func TestRequestFileDigestRereadsMidRequestEdit(t *testing.T) {
	resetFileDigestTrust()
	oldNow, oldHook := fileDigestNow, fileDigestReadHook.Load()
	fileDigestNow = time.Now
	t.Cleanup(func() { fileDigestNow = oldNow; fileDigestReadHook.Store(oldHook); resetFileDigestTrust() })
	path := filepath.Join(t.TempDir(), "A.cls")
	first := []byte("public class A {}")
	second := []byte("public class B {}")
	if err := os.WriteFile(path, first, 0600); err != nil {
		t.Fatal(err)
	}
	reads := countReads(t, path)
	var request requestFileDigests
	if got, err := request.digest(path); err != nil || got != sha256.Sum256(first) {
		t.Fatalf("first digest %x, %v", got, err)
	}
	ctimeTick()
	if err := os.WriteFile(path, second, 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := request.digest(path); err != nil || got != sha256.Sum256(second) {
		t.Fatalf("edited digest %x, %v", got, err)
	}
	if *reads != 2 {
		t.Fatalf("file was read %d times, want a fresh read after the edit", *reads)
	}
}

func TestRequestFileDigestInvalidatesStampFields(t *testing.T) {
	trustedClock(t)
	path := filepath.Join(t.TempDir(), "A.cls")
	content := []byte("public class A {}")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	stamp, ok := filestamp.Stat(path)
	if !ok {
		t.Fatal("file system without a trusted stamp")
	}
	for name, alter := range map[string]func(*filestamp.Stamp){
		"device": func(s *filestamp.Stamp) { s.Dev++ },
		"inode":  func(s *filestamp.Stamp) { s.Ino++ },
		"size":   func(s *filestamp.Stamp) { s.Size++ },
		"mtime":  func(s *filestamp.Stamp) { s.Mtime++ },
		"ctime":  func(s *filestamp.Stamp) { s.Ctime++ },
	} {
		t.Run(name, func(t *testing.T) {
			stale := stamp
			alter(&stale)
			request := requestFileDigests{entries: map[string]requestFileDigestEntry{path: {stamp: stale}}}
			got, err := request.digest(path)
			if err != nil || got != sha256.Sum256(content) {
				t.Fatalf("stale %s: %x, %v", name, got, err)
			}
		})
	}
}

func BenchmarkRequestFileDigestWarmProject(b *testing.B) {
	resetFileDigestTrust()
	oldNow := fileDigestNow
	fileDigestNow = func() time.Time { return time.Now().Add(time.Hour) }
	b.Cleanup(func() { fileDigestNow = oldNow; resetFileDigestTrust() })
	dir := b.TempDir()
	type input struct {
		path       string
		boundaries int
	}
	paths := make([]input, 0, 1531*2)
	for i := 0; i < 1531; i++ {
		for _, suffix := range []string{".cls", ".cls-meta.xml"} {
			path := filepath.Join(dir, fmt.Sprintf("Class%04d%s", i, suffix))
			if err := os.WriteFile(path, []byte("public class A {}"), 0600); err != nil {
				b.Fatal(err)
			}
			if _, err := currentFileDigest(path); err != nil {
				b.Fatal(err)
			}
			boundaries := 10
			if suffix == ".cls-meta.xml" {
				boundaries = 8
			}
			paths = append(paths, input{path, boundaries})
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		var request requestFileDigests
		for boundary := 0; boundary < 10; boundary++ {
			for _, item := range paths {
				if boundary >= item.boundaries {
					continue
				}
				if _, err := request.digest(item.path); err != nil {
					b.Fatal(err)
				}
			}
		}
	}
}
