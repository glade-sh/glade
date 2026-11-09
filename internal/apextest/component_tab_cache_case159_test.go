package apextest

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComponentTabsCase159DiskCache(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	root := t.TempDir()
	source := filepath.Join("testdata", "conformance", "case159_component_tab_identity", "project")
	if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, name), string(body))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	options := Options{SelectedClasses: []string{"Case159TabEqualityTest"}, Parallelism: 1, PerfCounters: true}
	for _, route := range []string{"cold", "disk"} {
		InvalidateRuntimeCaches()
		ResetPerfCounters()
		run := Run(index, options)
		if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
			t.Fatalf("%s missing native transport: %+v", route, run.Summary())
		}
		message := run.Suites[0].Cases[0].Problem.Message
		for _, part := range []string{`"count":5`, `"sameTarget":true`, `"distinctLwc":true`, `"distinctAura":true`} {
			if !strings.Contains(message, part) {
				t.Errorf("%s missing %s: %s", route, part, message)
			}
		}
		if route == "disk" && SnapshotPerfCounters().Phases.DiskCacheHits == 0 {
			t.Fatal("disk route did not restore the startup cache")
		}
	}
}
