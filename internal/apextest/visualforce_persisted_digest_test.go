package apextest

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/inputview"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestPersistedVisualforceDigestInvalidation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "asset.js")
	if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	savedNow, savedHook := fileDigestNow, fileDigestReadHook
	t.Cleanup(func() { fileDigestNow, fileDigestReadHook = savedNow, savedHook; resetFileDigestTrust() })
	fileDigestNow = func() time.Time { return time.Now().Add(time.Minute) }
	reads := 0
	fileDigestReadHook = func(string) { reads++ }
	resetFileDigestTrust()
	first, err := currentFileDigest(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveVisualforceDigestCache(root, []string{path}); err != nil {
		t.Fatal(err)
	}
	resetFileDigestTrust()
	if err := loadVisualforceDigestCache(root, []string{path}); err != nil {
		t.Fatal(err)
	}
	got, err := currentFileDigest(path)
	if err != nil || got != first || reads != 1 {
		t.Fatalf("stable cached digest: reads=%d digest=%x err=%v", reads, got, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("other"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	resetFileDigestTrust()
	if err := loadVisualforceDigestCache(root, []string{path}); err != nil {
		t.Fatal(err)
	}
	got, err = currentFileDigest(path)
	want := sha256.Sum256([]byte("other"))
	if err != nil || got != want || reads != 2 {
		t.Fatalf("rewritten file: reads=%d digest=%x err=%v", reads, got, err)
	}
}

func TestPersistedVisualforceDigestRejectsRecentAndCorruptData(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "asset.js")
	if err := os.WriteFile(path, []byte("fresh"), 0600); err != nil {
		t.Fatal(err)
	}
	savedNow, savedHook := fileDigestNow, fileDigestReadHook
	t.Cleanup(func() { fileDigestNow, fileDigestReadHook = savedNow, savedHook; resetFileDigestTrust() })
	fileDigestNow = time.Now
	reads := 0
	fileDigestReadHook = func(string) { reads++ }
	resetFileDigestTrust()
	if _, err := currentFileDigest(path); err != nil {
		t.Fatal(err)
	}
	if err := saveVisualforceDigestCache(root, []string{path}); err != nil {
		t.Fatal(err)
	}
	resetFileDigestTrust()
	if err := loadVisualforceDigestCache(root, []string{path}); err != nil {
		t.Fatal(err)
	}
	if _, err := currentFileDigest(path); err != nil {
		t.Fatal(err)
	}
	if reads != 2 {
		t.Fatalf("recent file was reused: reads=%d", reads)
	}

	fileDigestNow = func() time.Time { return time.Now().Add(time.Minute) }
	resetFileDigestTrust()
	if _, err := currentFileDigest(path); err != nil {
		t.Fatal(err)
	}
	if err := saveVisualforceDigestCache(root, []string{path}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(visualforceDigestCachePath(root), []byte("{invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	resetFileDigestTrust()
	_ = loadVisualforceDigestCache(root, []string{path})
	if _, err := currentFileDigest(path); err != nil {
		t.Fatal(err)
	}
	if reads != 4 {
		t.Fatalf("corrupt cache was reused: reads=%d", reads)
	}
}

func TestVisualforcePersistentDigestScope(t *testing.T) {
	root := t.TempDir()
	page, resource := filepath.Join(root, "Home.page"), filepath.Join(root, "asset.js")
	writeFile(t, page, "<apex:page/>")
	writeFile(t, resource, "asset")
	restore := EnableDiskCacheForTesting()
	defer restore()
	oldNow, oldHook := fileDigestNow, fileDigestReadHook
	fileDigestNow = func() time.Time { return time.Now().Add(time.Minute) }
	reads := 0
	fileDigestReadHook = func(path string) {
		if path == resource {
			reads++
		}
	}
	t.Cleanup(func() { fileDigestNow, fileDigestReadHook = oldNow, oldHook; resetFileDigestTrust() })
	capture := func(p project.Project) typesys.ApexMetadataInput {
		generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
		if err := captureVisualforceRuntimeInputs(p, &generation); err != nil {
			t.Fatal(err)
		}
		return generation.inputs[resource]
	}
	p := project.Project{Root: root, VisualforcePageFiles: []string{page}, StaticResourceFiles: []string{resource}}
	resetFileDigestTrust()
	first := capture(p)
	resetFileDigestTrust()
	if second := capture(p); first != second || reads != 1 {
		t.Fatalf("cross-process reuse: reads=%d first=%v second=%v", reads, first, second)
	}

	for _, withView := range []bool{false, true} {
		other := t.TempDir()
		page, resource := filepath.Join(other, "Home.page"), filepath.Join(other, "asset.js")
		writeFile(t, page, "<apex:page/>")
		writeFile(t, resource, "asset")
		p := project.Project{Root: other, StaticResourceFiles: []string{resource}}
		if withView {
			p.VisualforcePageFiles = []string{page}
			p = p.WithInputs(inputview.New())
		}
		resetFileDigestTrust()
		capture(p)
		if _, err := os.Stat(visualforceDigestCachePath(other)); !os.IsNotExist(err) {
			t.Fatalf("withView=%t: unexpected cache file: %v", withView, err)
		}
	}
}
