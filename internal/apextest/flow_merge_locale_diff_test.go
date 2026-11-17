package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A Number or Date merged into flow text renders only for a user locale that
// was captured natively. Any other locale (ru_RU, pt_PT, a bare de) keeps the
// text it had before typed merges (d5ae42fedb^), byte for byte: the expected
// lines for those locales were produced by a binary built from d5ae42fedb^ on
// this project. Literal text with a newline, a tab, an escaped ampersand and
// quotes must survive on both paths. The captured locales (en_US, de_DE) render.
func TestFlowTypedMergeLocaleDifferential(t *testing.T) {
	dir := filepath.Join("testdata", "flow_merge_locale_diff")
	root := t.TempDir()
	err := filepath.WalkDir(filepath.Join(dir, "project"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(filepath.Join(dir, "project"), path)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, "force-app/main/default", rel), string(body))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	want, err := os.ReadFile(filepath.Join(dir, "expected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	result := Run(index, Options{SelectedClasses: []string{"FlowDiff"}, NoDiskCache: true, Parallelism: 1})
	var message string
	for _, suite := range result.Suites {
		for _, c := range suite.Cases {
			if c.Problem != nil {
				message = c.Problem.Message
			}
		}
	}
	got := strings.TrimPrefix(message, "Assertion Failed: FlowDiff\n")
	if fmt.Sprintf("%s\n", strings.TrimRight(got, "\n")) != string(want) {
		t.Fatalf("flow merge text:\n%s\nwant:\n%s", got, want)
	}
}
