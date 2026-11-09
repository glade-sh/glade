package apextest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunHttpHeaderIdNativeProject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	for _, name := range []string{"Pr163ApexCapture62", "Pr163ApexCapture67"} {
		for _, suffix := range []string{".cls", ".cls-meta.xml"} {
			data, err := os.ReadFile(filepath.Join("testdata/conformance/pr163_apex_basics/project/classes", name+suffix))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+suffix), string(data))
		}
	}
	run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"Pr163ApexCapture62", "Pr163ApexCapture67"}, SelectedMethod: "idHeader", NoDiskCache: true, Parallelism: 1})
	if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 {
		t.Fatalf("summary=%#v, problem=%s", summary, firstRunProblem(run))
	}
}
