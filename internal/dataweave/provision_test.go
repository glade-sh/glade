package dataweave

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestArtifactDownloadRejectsChangedBytesAndExcess(t *testing.T) {
	original := []byte("owned artifact bytes")
	sum := sha256.Sum256(original)
	for _, test := range []struct {
		name string
		body []byte
		ok   bool
	}{{"exact", original, true}, {"changed", []byte("changed artifact byt"), false}, {"excess", append(append([]byte(nil), original...), 0), false}} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(test.body) }))
			defer server.Close()
			a := artifact{Path: "owned.jar", URL: server.URL, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(original))}
			err := downloadArtifact(context.Background(), a, filepath.Join(t.TempDir(), "owned.jar"))
			if (err == nil) != test.ok {
				t.Fatalf("result=%v, want success=%v", err, test.ok)
			}
		})
	}
}
func TestVerifyEngineRequiresExactSourceOffers(t *testing.T) {
	// Build a complete local graph from the explicitly provided integration cache;
	// no download is performed by this test.
	directory := os.Getenv("GLADE_DATAWEAVE_TEST_INSTALLED_ENGINE")
	if directory == "" {
		t.Skip("explicit provisioned engine required")
	}
	if paths, err := VerifyEngine(directory); err != nil || len(paths) != 17 {
		t.Fatalf("verified graph=%v: %v", paths, err)
	}
}
func TestArtifactManifestIncludesSourcesAndExactPins(t *testing.T) {
	count, sourceCount := 0, 0
	seen := map[string]bool{}
	for _, a := range artifacts() {
		if seen[a.Path] || !filepath.IsLocal(a.Path) || len(a.SHA256) != 64 || a.Bytes <= 0 {
			t.Fatalf("invalid pinned artifact: %+v", a)
		}
		seen[a.Path] = true
		if a.ClassPath {
			count++
		} else {
			sourceCount++
		}
	}
	if count != 17 || sourceCount != 2 {
		t.Fatalf("runtime=%d source=%d", count, sourceCount)
	}
}

func TestPinnedEngineInstallation(t *testing.T) {
	directory := os.Getenv("GLADE_DATAWEAVE_TEST_INSTALL_DIRECTORY")
	if directory == "" {
		t.Skip("explicit engine installation destination required")
	}
	paths, err := InstallEngine(context.Background(), directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 17 {
		t.Fatalf("classpath count %d", len(paths))
	}
	if _, err := os.Stat(filepath.Join(directory, "THIRD-PARTY-NOTICES.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory, "sources", "angus-mail-2.0.5-sources.jar")); err != nil {
		t.Fatal(err)
	}
	// Removing a source offer from an isolated mirror must invalidate the graph,
	// even when all runtime jars remain present and executable.
	mirror := t.TempDir()
	for _, a := range artifacts() {
		if !a.ClassPath {
			continue
		}
		dest := filepath.Join(mirror, filepath.FromSlash(a.Path))
		if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(a.Path)))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dest, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := VerifyEngine(mirror); err == nil {
		t.Fatal("missing corresponding sources accepted")
	}
}
