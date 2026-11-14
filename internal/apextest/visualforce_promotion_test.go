package apextest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestRecentVisualforceDigestPromotesAfterAge(t *testing.T) {
	root := t.TempDir()
	page, input := filepath.Join(root, "Home.page"), filepath.Join(root, "asset.js")
	writeFile(t, page, "<apex:page/>")
	writeFile(t, input, "first")
	p := project.Project{Root: root, VisualforcePageFiles: []string{page}, StaticResourceFiles: []string{input}}
	oldNow, oldHook := fileDigestNow, fileDigestReadHook
	now := time.Now()
	fileDigestNow = func() time.Time { return now }
	reads := 0
	fileDigestReadHook = func(path string) {
		if path == input {
			reads++
		}
	}
	t.Cleanup(func() { fileDigestNow, fileDigestReadHook = oldNow, oldHook; resetFileDigestTrust() })
	resetFileDigestTrust()
	capture := func() typesys.ApexMetadataInput {
		generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
		if err := captureVisualforceRuntimeInputs(p, &generation); err != nil {
			t.Fatal(err)
		}
		return generation.inputs[input]
	}
	first := capture()
	now = now.Add(fileDigestTrustAge + time.Second)
	if got := capture(); got != first || reads != 1 {
		t.Fatalf("aged generation: reads=%d", reads)
	}
	if _, ok := trustedCurrentFileDigest(input); !ok {
		t.Fatal("verified aged input was not promoted")
	}
}

func TestRecentVisualforcePromotionRejectsChangedBytes(t *testing.T) {
	root := t.TempDir()
	page, input := filepath.Join(root, "Home.page"), filepath.Join(root, "asset.js")
	writeFile(t, page, "<apex:page/>")
	writeFile(t, input, "first")
	p := project.Project{Root: root, VisualforcePageFiles: []string{page}, StaticResourceFiles: []string{input}}
	oldNow := fileDigestNow
	now := time.Now()
	fileDigestNow = func() time.Time { return now }
	t.Cleanup(func() { fileDigestNow = oldNow; resetFileDigestTrust() })
	resetFileDigestTrust()
	capture := func() typesys.ApexMetadataInput {
		generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
		if err := captureVisualforceRuntimeInputs(p, &generation); err != nil {
			t.Fatal(err)
		}
		return generation.inputs[input]
	}
	first := capture()
	replacement := filepath.Join(root, "replacement")
	writeFile(t, replacement, "other")
	if err := os.Rename(replacement, input); err != nil {
		t.Fatal(err)
	}
	now = now.Add(fileDigestTrustAge + time.Second)
	if got := capture(); got == first {
		t.Fatal("changed file reused the recent digest")
	}
}
