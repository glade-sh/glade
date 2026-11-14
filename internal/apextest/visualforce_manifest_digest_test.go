package apextest

import (
	"os"
	"path/filepath"
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
