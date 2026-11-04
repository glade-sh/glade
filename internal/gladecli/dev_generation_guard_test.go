package gladecli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type corruptManifestOnIndex struct {
	path    string
	changed bool
	err     error
}

func (w *corruptManifestOnIndex) Write(p []byte) (int, error) {
	if !w.changed && bytes.Contains(p, []byte("Indexing symbols")) {
		w.changed = true
		w.err = os.WriteFile(w.path, []byte("{"), 0o600)
	}
	return len(p), nil
}

// A dev startup must not publish a project after its manifest becomes invalid
// between discovery and indexing. Both commands return before starting a server.
func TestDevStartupRejectsManifestChangedDuringIndexing(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, mode := range []string{"vf", "lwc"} {
			t.Run(api+"/"+mode, func(t *testing.T) {
				root := t.TempDir()
				manifest := filepath.Join(root, "sfdx-project.json")
				writeTestFile(t, manifest, `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				progress := &corruptManifestOnIndex{path: manifest}
				// If the guard regresses, an invalid ready-file parent stops startup
				// before either command launches its server goroutines.
				ready := filepath.Join(root, "missing-parent", "ready.json")
				var err error
				if mode == "vf" {
					err = runDevVF(context.Background(), []string{"--project", root, "--addr", "127.0.0.1:0", "--progress-json", "--ready-file", ready}, &bytes.Buffer{}, progress)
				} else {
					err = runDevLWCWithOpenURL(context.Background(), []string{"--project", root, "--addr", "127.0.0.1:0", "--progress-json", "--ready-file", ready}, &bytes.Buffer{}, progress, func(string) error { t.Error("unexpected opener"); return nil })
				}
				if progress.err != nil || !progress.changed {
					t.Fatalf("manifest mutation: changed=%v err=%v", progress.changed, progress.err)
				}
				if err == nil || !strings.Contains(err.Error(), "JSON") {
					t.Fatalf("startup error=%v, want invalid manifest rejection", err)
				}
			})
		}
	}
}
