package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Native rows: testdata/native-capture/standard_ext_upsert/org.tsv (see native-run.md).
// An external-id upsert as an administrator without a grant on the key field
// throws SObjectException before any record check in user mode. Rows replay through the isTest route.
func TestStandardExtUpsertNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "native-capture", "standard_ext_upsert")
	body, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	type key struct{ api string }
	want := map[key][][2]string{}
	for _, line := range strings.Split(strings.TrimRight(string(body), "\n"), "\n")[1:] {
		f := strings.SplitN(line, "\t", 3)
		want[key{f[0]}] = append(want[key{f[0]}], [2]string{f[1], f[2]})
	}
	if len(want) != 2 {
		t.Fatalf("expected 2 api groups, got %d", len(want))
	}
	for k, rows := range want {
		k, rows := k, rows
		t.Run(k.api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.Walk(project, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(project, path)
				if strings.HasSuffix(rel, ".apex") || strings.HasSuffix(rel, "scratch-def.json") {
					return nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(data), "67.0", k.api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"ExtProbeTest"}, SelectedMethod: "extProbe", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("expected one observation: %+v", run.Summary())
			}
			_, observed, _ := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "OBSERVED_EXT\n")
			got := map[string]string{}
			for _, line := range strings.Split(observed, "\n") {
				name, value, ok := strings.Cut(line, "=")
				if ok {
					got[name] = value
				}
			}
			if len(rows) != 48 {
				t.Fatalf("expected 48 native rows, got %d", len(rows))
			}
			for _, row := range rows {
				if g, ok := got[row[0]]; !ok {
					t.Errorf("%s: Glade produced no observation", row[0])
				} else if g != row[1] {
					t.Errorf("%s:\n native: %s\n Glade:  %s", row[0], row[1], g)
				}
			}
		})
	}
}
