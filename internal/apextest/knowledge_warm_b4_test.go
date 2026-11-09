package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Supplemental warm route for history_* rows in knowledge_publishing/org.tsv.
func TestKnowledgePublishingB4WarmProject(t *testing.T) {
	t.Cleanup(EnableDiskCacheForTesting())
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join("testdata", "conformance", "knowledge_publishing", "project"))); err != nil {
				t.Fatal(err)
			}
			classes := filepath.Join(root, "force-app", "main", "default", "classes")
			entries, err := os.ReadDir(classes)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if !strings.HasPrefix(entry.Name(), "B4History.cls") {
					if err := os.Remove(filepath.Join(classes, entry.Name())); err != nil {
						t.Fatal(err)
					}
				}
			}
			classPath := filepath.Join(classes, "B4History.cls")
			source, err := os.ReadFile(classPath)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, classPath, licenseKnowledgeTestClass(string(source)))
			meta := classPath + "-meta.xml"
			contents, err := os.ReadFile(meta)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, meta, strings.ReplaceAll(string(contents), "62.0", api))
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			index := loadTestIndex(t, root)
			opts := Options{SelectedClasses: []string{"B4History"}, Parallelism: 1, PerfCounters: true}
			for pass := 0; pass < 3; pass++ {
				if pass == 2 {
					InvalidateRuntimeCaches()
				}
				ResetPerfCounters()
				result := Run(index, opts)
				if s := result.Summary(); s.Total != 1 || s.Passed != 1 {
					t.Fatalf("pass %d: %+v %s", pass, s, firstRunProblem(result))
				}
				if pass == 1 && SnapshotPerfCounters().Phases.MemoryCacheHits != 1 {
					t.Fatal("warm runner missed memory cache")
				}
				if pass == 2 && SnapshotPerfCounters().Phases.DiskCacheHits != 1 {
					t.Fatal("restored runner missed disk cache")
				}
			}
		})
	}
}
