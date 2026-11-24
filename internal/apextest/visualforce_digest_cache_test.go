package apextest

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/inputview"
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
	oldNow, oldHook := fileDigestClock.Load(), fileDigestReadHook.Load()
	setFileDigestClockForTesting(func() time.Time { return time.Now().Add(time.Hour) })
	reads := 0
	SetFileDigestReadHookForTesting(func(path string) {
		if path == resource {
			reads++
		}
	})
	t.Cleanup(func() { fileDigestClock.Store(oldNow); fileDigestReadHook.Store(oldHook); resetFileDigestTrust() })
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

func TestVisualforceInputPermissionErrorIdentifiesFile(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "Home.page")
	input := filepath.Join(root, "Labels.labels")
	writeFile(t, page, "<apex:page/>")
	writeFile(t, input, "labels")
	if err := os.Chmod(input, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(input, 0600) })
	if f, err := os.Open(input); err == nil {
		_ = f.Close()
		return // permissions are not enforced for this user; nothing to assert
	}
	for _, withView := range []bool{false, true} {
		p := project.Project{Root: root, VisualforcePageFiles: []string{page}, LabelFiles: []string{input}}
		if withView {
			p = p.WithInputs(inputview.New())
		}
		err := captureVisualforceRuntimeInputs(p, &runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}})
		if !errors.Is(err, fs.ErrPermission) || !strings.Contains(err.Error(), input) {
			t.Fatalf("withView=%v: error does not identify permission and path: %v", withView, err)
		}
	}
}
