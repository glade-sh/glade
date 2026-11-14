package apextest

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestVisualforceDigestReusedByRuntimeManifest(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"62.0"}`)
	page := filepath.Join(root, "src", "pages", "Home.page")
	resource := filepath.Join(root, "src", "staticresources", "Bundle", "asset.js")
	writeFile(t, page, `<apex:page/>`)
	writeFile(t, resource, "first")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	resetFileDigestTrust()
	oldNow := fileDigestNow
	fileDigestNow = func() time.Time { return time.Now().Add(time.Hour) }
	t.Cleanup(func() { fileDigestNow = oldNow; resetFileDigestTrust() })
	generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
	if err := captureVisualforceRuntimeInputs(p, &generation); err != nil {
		t.Fatal(err)
	}
	hits := 0
	lookup := func(path string) ([32]byte, bool) {
		digest, ok := trustedCurrentFileDigest(path)
		if path == resource && ok {
			hits++
		}
		return digest, ok
	}
	cached, err := startupcache.ValidateInputWithSourceDigestsAndDigestLookup(root, nil, lookup)
	if err != nil {
		t.Fatal(err)
	}
	ordinary, err := startupcache.ValidateInputWithSourceDigests(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if hits == 0 || cached.Digest() != ordinary.Digest() {
		t.Fatalf("trusted hits=%d, manifest equality=%v", hits, cached.Digest() == ordinary.Digest())
	}
	info, err := os.Stat(resource)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, resource, "other")
	if err := os.Chtimes(resource, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	hits = 0
	cached, err = startupcache.ValidateInputWithSourceDigestsAndDigestLookup(root, nil, lookup)
	if err != nil {
		t.Fatal(err)
	}
	ordinary, err = startupcache.ValidateInputWithSourceDigests(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 0 || cached.Digest() != ordinary.Digest() {
		t.Fatalf("stale hits=%d, manifest equality=%v", hits, cached.Digest() == ordinary.Digest())
	}
}

func TestVisualforceManifestDigestLookupScope(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "Home.page")
	resource := filepath.Join(root, "asset.js")
	unrelated := filepath.Join(root, "unrelated.txt")
	for _, path := range []string{page, resource, unrelated} {
		writeFile(t, path, "content")
	}
	resetFileDigestTrust()
	oldNow := fileDigestNow
	fileDigestNow = func() time.Time { return time.Now().Add(time.Hour) }
	t.Cleanup(func() { fileDigestNow = oldNow; resetFileDigestTrust() })
	if _, err := currentFileDigest(unrelated); err != nil {
		t.Fatal(err)
	}
	withoutPages := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
	if err := captureVisualforceRuntimeInputs(project.Project{Root: root, StaticResourceFiles: []string{resource}}, &withoutPages); err != nil {
		t.Fatal(err)
	}
	if _, ok := withoutPages.trustedVisualforceDigest(unrelated); ok {
		t.Fatal("no-page generation reused an unrelated digest")
	}
	withPages := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
	if err := captureVisualforceRuntimeInputs(project.Project{Root: root, VisualforcePageFiles: []string{page}, StaticResourceFiles: []string{resource}}, &withPages); err != nil {
		t.Fatal(err)
	}
	if _, ok := withPages.trustedVisualforceDigest(resource); !ok {
		t.Fatal("Visualforce input digest was not available")
	}
	if _, ok := withPages.trustedVisualforceDigest(unrelated); ok {
		t.Fatal("generation reused a digest outside its Visualforce inputs")
	}
}

func TestRecentVisualforceInputsAvoidRepeatedHashing(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"62.0"}`)
	page := filepath.Join(root, "src", "pages", "Home.page")
	resource := filepath.Join(root, "src", "staticresources", "Bundle", "asset.js")
	writeFile(t, page, `<apex:page/>`)
	writeFile(t, resource, "first")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	resetFileDigestTrust()
	oldNow, oldHook := fileDigestNow, fileDigestReadHook
	fileDigestNow = time.Now
	reads := 0
	fileDigestReadHook = func(path string) {
		if path == resource {
			reads++
		}
	}
	t.Cleanup(func() { fileDigestNow, fileDigestReadHook = oldNow, oldHook; resetFileDigestTrust() })
	sources := newSourceCache()
	sources.seedProjectSnapshot(&p)
	generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
	if err := sources.captureProjectRuntimeMetadata(index, &generation); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := sources.validateCapturedSourceGenerationRaw(); err != nil {
			t.Fatal(err)
		}
	}
	if reads != 1 {
		t.Fatalf("resource hash reads=%d, want one for unchanged recent input", reads)
	}
	info, err := os.Stat(resource)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, resource, "other")
	if err := os.Chtimes(resource, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := sources.validateCapturedSourceGenerationRaw(); err == nil {
		t.Fatal("same-size rewrite with restored timestamp was not detected")
	}
}

func TestRecentVisualforceInputValidationDetectsChanges(t *testing.T) {
	for _, change := range []string{"rewrite", "replace", "delete", "symlink"} {
		t.Run(change, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"62.0"}`)
			page := filepath.Join(root, "src", "pages", "Home.page")
			resource := filepath.Join(root, "src", "staticresources", "Bundle", "asset.js")
			writeFile(t, page, `<apex:page/>`)
			if change == "symlink" {
				first := filepath.Join(root, "first.js")
				writeFile(t, first, "first")
				if err := os.MkdirAll(filepath.Dir(resource), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(first, resource); err != nil {
					t.Fatal(err)
				}
			} else {
				writeFile(t, resource, "first")
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			resetFileDigestTrust()
			reads := 0
			oldNow, oldHook := fileDigestNow, fileDigestReadHook
			fileDigestNow = time.Now
			fileDigestReadHook = func(path string) {
				if path == resource {
					reads++
				}
			}
			t.Cleanup(func() { fileDigestNow, fileDigestReadHook = oldNow, oldHook; resetFileDigestTrust() })
			sources := newSourceCache()
			sources.seedProjectSnapshot(&p)
			generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
			if err := sources.captureProjectRuntimeMetadata(index, &generation); err != nil {
				t.Fatal(err)
			}
			if err := sources.validateCapturedSourceGenerationRaw(); err != nil {
				t.Fatal(err)
			}
			if reads != 1 {
				t.Fatalf("resource hash reads=%d, want one", reads)
			}
			info, err := os.Stat(resource)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "rewrite":
				writeFile(t, resource, "other")
				if err := os.Chtimes(resource, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
			case "replace":
				replacement := filepath.Join(root, "replacement.js")
				writeFile(t, replacement, "other")
				if err := os.Chtimes(replacement, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(replacement, resource); err != nil {
					t.Fatal(err)
				}
			case "delete":
				if err := os.Remove(resource); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				second := filepath.Join(root, "second.js")
				writeFile(t, second, "other")
				if err := os.Remove(resource); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(second, resource); err != nil {
					t.Fatal(err)
				}
			}
			if err := sources.validateCapturedSourceGenerationRaw(); err == nil {
				t.Fatal("changed input was not detected")
			}
		})
	}
}

func TestRecentVisualforceInputConcurrentValidation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "input.js")
	writeFile(t, path, "unchanged")
	input, captured, err := captureRecentVisualforceInput(path, recentVisualforceBudget)
	if err != nil || !captured {
		t.Fatalf("capture=%v err=%v", captured, err)
	}
	var wg sync.WaitGroup
	results := make(chan bool, 8)
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- input.matches(path) }()
	}
	wg.Wait()
	close(results)
	for ok := range results {
		if !ok {
			t.Fatal("unchanged input did not match")
		}
	}
}
