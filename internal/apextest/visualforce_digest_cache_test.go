package apextest

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestVisualforceInputDigestReusesStableFiles(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "Home.page")
	resource := filepath.Join(root, "asset.js")
	writeFile(t, page, "<apex:page/>")
	writeFile(t, resource, "first")
	p := project.Project{Root: root, VisualforcePageFiles: []string{page}, StaticResourceFiles: []string{resource}}
	resetFileDigestTrust()
	oldNow, oldHook := fileDigestNow, fileDigestReadHook
	fileDigestNow = func() time.Time { return time.Now().Add(time.Hour) }
	reads := 0
	fileDigestReadHook = func(path string) {
		if path == resource {
			reads++
		}
	}
	t.Cleanup(func() { fileDigestNow, fileDigestReadHook = oldNow, oldHook; resetFileDigestTrust() })
	capture := func() typesys.ApexMetadataInput {
		generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
		if err := captureVisualforceRuntimeInputs(p, &generation); err != nil {
			t.Fatal(err)
		}
		return generation.inputs[resource]
	}
	first := capture()
	if got := capture(); got != first || reads != 1 {
		t.Fatalf("stable input: got=%v want=%v reads=%d", got, first, reads)
	}
	writeFile(t, resource, "second")
	if got := capture(); got == first || reads != 2 {
		t.Fatalf("changed input: got=%v old=%v reads=%d", got, first, reads)
	}
}
