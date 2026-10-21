package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCacheStaticEntryAcrossProjectSources(t *testing.T) {
	for _, api := range []string{"62.0", "66.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("../vm/testdata/cache_static_dispatch/project")); err != nil {
				t.Fatal(err)
			}
			if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil || entry.IsDir() {
					return walkErr
				}
				if entry.Name() != "sfdx-project.json" && !strings.HasSuffix(path, ".cls-meta.xml") {
					return nil
				}
				source, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				return os.WriteFile(path, []byte(strings.ReplaceAll(string(source), "66.0", api)), 0600)
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if index.Project.SourceAPIVersion != api {
				t.Fatalf("source API=%s, want %s", index.Project.SourceAPIVersion, api)
			}
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
			if got := run.Summary(); got.Total != 4 || got.Passed != 4 {
				t.Fatalf("summary=%+v, problem=%s", got, firstRunProblem(run))
			}
		})
	}
}
