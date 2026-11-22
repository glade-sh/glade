package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The captured test updates Company; another field keeps the baseline result.
func TestLeadReadModelUpdateUncapturedField(t *testing.T) {
	const base = "testdata/conformance/lead_read_model_recalculation"
	const api = "67.0"
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{ "sourceApiVersion":"67.0", "packageDirectories":[{"path":"force-app","default":true}] }`)
	copyTree(t, filepath.Join(base, "project"), filepath.Join(root, "force-app/main/default"), api)
	copyTree(t, filepath.Join(base, "project-scratch-def.json"), filepath.Join(root, "config/project-scratch-def.json"), api)
	path := filepath.Join(root, "force-app/main/default/classes/PclEditTest.cls")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(source), "l.Company = 'X'", "l.LastName = 'X'", 1)
	if changed == string(source) {
		t.Fatal("missing update")
	}
	writeFile(t, path, changed)
	run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"PclEditTest"}, SelectedMethod: "run", NoDiskCache: true, Parallelism: 1})
	if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
		t.Fatalf("expected observation: %+v", run.Summary())
	}
	got := run.Suites[0].Cases[0].Problem.Message
	if !strings.Contains(got, "updLead=System.SecurityException:") {
		t.Fatalf("uncaptured field result changed: %s", got)
	}
}
