package apextest

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func runCase159ProjectVersions(t *testing.T, family string, check func(*testing.T, typesys.Index)) {
	t.Helper()
	runCase159ProjectVersionsPrepared(t, family, nil, check)
}

// runCase159ProjectVersionsPrepared lets prepare add capture setup (such as a
// recorded grant) to the copied project before it is indexed.
func runCase159ProjectVersionsPrepared(t *testing.T, family string, prepare func(t *testing.T, root string), check func(*testing.T, typesys.Index)) {
	t.Helper()
	project := filepath.Join("testdata", "conformance", family, "project")
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if prepare != nil {
				prepare(t, root)
			}
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatalf("synthetic project index: %v", index.Diagnostics)
			}
			check(t, index)
		})
	}
}

func TestCommunityPageURLNativeProject(t *testing.T) {
	runCase159ProjectVersions(t, "case159_community_page", func(t *testing.T, index typesys.Index) {
		run := Run(index, Options{SelectedClasses: []string{"Case159CommunityTest"}, NoDiskCache: true})
		if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
			t.Fatalf("community page URL: summary=%#v problem=%s", summary, firstRunProblem(run))
		}
	})
}
