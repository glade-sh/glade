package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBusinessAccountPermissionReviewNativeConformance(t *testing.T) {
	const fixture = "../../testdata/conformance/person_account_followups"
	f, err := os.Open(filepath.Join(fixture, "review2", "permissions.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{}
	for _, row := range rows[1:] {
		key := row[0] + "/" + row[1]
		want[key] = append(want[key], row[2]+"|"+row[3])
	}
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(fixture, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				rel, err := filepath.Rel(fixture, path)
				if err != nil {
					return err
				}
				rel = filepath.ToSlash(rel)
				if !strings.HasPrefix(rel, "force-app/") && !strings.HasPrefix(rel, "config/") {
					return nil
				}
				source := readFixtureFile(t, path)
				if strings.HasSuffix(rel, ".cls-meta.xml") {
					source = strings.ReplaceAll(strings.ReplaceAll(source, "62.0", api), "67.0", api)
				}
				writeFile(t, filepath.Join(root, rel), source)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			index := loadTestIndex(t, root)
			for _, route := range []string{"project", "disk_restore"} {
				t.Run(route, func(t *testing.T) {
					if route == "disk_restore" {
						InvalidateRuntimeCaches()
					}
					result := Run(index, Options{SelectedClasses: []string{"BusinessPermissionReviewTest"}, Parallelism: 1, PerfCounters: route == "disk_restore"})
					if route == "disk_restore" && SnapshotPerfCounters().Phases.DiskCacheHits != 1 {
						t.Fatalf("disk restore was not used: %+v", SnapshotPerfCounters().Phases)
					}
					seen := 0
					for _, suite := range result.Suites {
						for _, c := range suite.Cases {
							if c.Problem == nil {
								t.Fatalf("missing capture: %s", c.MethodName)
							}
							_, payload, ok := strings.Cut(c.Problem.Message, "Actual: ")
							if !ok {
								t.Fatalf("unexpected failure: %s", c.Problem.Message)
							}
							got := strings.Split(payload, "~")
							expected := want[api+"/"+strings.ToLower(c.MethodName)]
							if len(expected) != 3 || len(got) != 3 {
								t.Fatalf("%s: rows got %d want %d", c.MethodName, len(got), len(expected))
							}
							for i := range got {
								if got[i] != expected[i] {
									t.Errorf("%s: got %q want %q", c.MethodName, got[i], expected[i])
								}
							}
							seen++
						}
					}
					if seen != 2 {
						t.Fatalf("got %d permission methods, want 2", seen)
					}
				})
			}
		})
	}
}
