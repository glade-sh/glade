package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestRecentVisualforceDigestReusedAcrossGenerations(t *testing.T) {
	root := t.TempDir()
	page, input := filepath.Join(root, "Home.page"), filepath.Join(root, "asset.js")
	writeFile(t, page, "<apex:page/>")
	writeFile(t, input, "first")
	p := project.Project{Root: root, VisualforcePageFiles: []string{page}, StaticResourceFiles: []string{input}}
	oldNow, oldHook := fileDigestNow, fileDigestReadHook
	fileDigestNow = time.Now
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
	if second := capture(); second != first || reads != 1 {
		t.Fatalf("unchanged generations: reads=%d first=%v second=%v", reads, first, second)
	}
	info, err := os.Stat(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, []byte("other"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(input, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if changed := capture(); changed == first || reads != 2 {
		t.Fatalf("changed generation: reads=%d", reads)
	}
}

func TestRecentVisualforceDigestConcurrentGenerations(t *testing.T) {
	root := t.TempDir()
	page, input := filepath.Join(root, "Home.page"), filepath.Join(root, "asset.js")
	writeFile(t, page, "<apex:page/>")
	writeFile(t, input, "content")
	p := project.Project{Root: root, VisualforcePageFiles: []string{page}, StaticResourceFiles: []string{input}}
	resetFileDigestTrust()
	t.Cleanup(resetFileDigestTrust)
	capture := func() (typesys.ApexMetadataInput, error) {
		generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
		err := captureVisualforceRuntimeInputs(p, &generation)
		return generation.inputs[input], err
	}
	first, err := capture()
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < cap(errs); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := capture()
			if err != nil {
				errs <- err
			} else if got != first {
				errs <- fmt.Errorf("generation changed: %v", got)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestRecentVisualforceSharedSnapshotBound(t *testing.T) {
	resetFileDigestTrust()
	t.Cleanup(resetFileDigestTrust)
	input := recentVisualforceInput{content: make([]byte, recentVisualforceFileLimit)}
	for i := 0; i < 40; i++ {
		rememberRecentVisualforceInput(fmt.Sprintf("path-%d", i), input)
	}
	fileDigestTrustMu.Lock()
	defer fileDigestTrustMu.Unlock()
	if recentVisualforceSharedBytes > recentVisualforceBudget {
		t.Fatalf("cached %d bytes over %d limit", recentVisualforceSharedBytes, recentVisualforceBudget)
	}
	if len(recentVisualforceShared) > 4096 {
		t.Fatalf("cached %d entries", len(recentVisualforceShared))
	}
}
