package gladecli

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE rows: ../apextest/testdata/conformance/flow_create_unset_variable_reference/org.tsv,
// route anonymous, API 62.0 and 67.0. An after-save Product2 flow creates a
// PricebookEntry from the standard pricebook it looked up. The anonymous script
// never names PricebookEntry, yet native creates the entry (one pricebook, one
// entry).
func TestExecFlowCreatesStandardObjectNamedOnlyByTheFlow(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	fixture := "../apextest/testdata/conformance/flow_create_unset_variable_reference"
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	want := map[string]string{}
	for _, row := range strings.Split(strings.TrimSpace(read("org.tsv")), "\n")[1:] {
		cols := strings.Split(row, "\t")
		if len(cols) >= 9 && cols[2] == "anonymous" {
			want[cols[1]] = strings.ReplaceAll(cols[5], "&#124;", "|")
		}
	}
	if len(want) != 2 {
		t.Fatalf("native anonymous rows: %v", want)
	}
	for api, outcome := range want {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			err := filepath.WalkDir(filepath.Join(fixture, "project/force-app/main/default/flows"), func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeTestFile(t, filepath.Join(root, "force-app/main/default/flows", entry.Name()), strings.ReplaceAll(string(body), "__API__", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			ok, output := caseFLSExec(root, read("run.apex"))
			if !ok || !strings.Contains(output, outcome) {
				t.Fatalf("NATIVE flow_create_unset_variable_reference/org.tsv %s anonymous: want %q; Glade ok=%v %s", api, outcome, ok, output)
			}
		})
	}
}
