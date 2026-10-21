package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// These are candidate lifecycle assertions, not native conformance rows. The
// executable capture matrix retains an unset native expectation for every cell.
func TestTaskPriorityFlagCandidateProjectRegression(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := taskFlagCaptureProject(t, api)
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"TaskFlagRegression"}, NoDiskCache: true})
			if summary := run.Summary(); summary.Total != 6 || summary.Passed != 6 {
				data, _ := json.Marshal(run)
				t.Fatalf("candidate regression: %s", data)
			}
		})
	}
}

func taskFlagCaptureProject(t *testing.T, api string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	base := filepath.Join(root, "force-app/main/default")
	source := filepath.Join("testdata", "task_priority_flag")
	for _, name := range []string{"TaskFlagCapture", "TaskFlagRegression"} {
		body, err := os.ReadFile(filepath.Join(source, name+".cls"))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(base, "classes", name+".cls"), string(body))
		writeFile(t, filepath.Join(base, "classes", name+".cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	}
	body, err := os.ReadFile(filepath.Join(source, "TaskFlagWitness.trigger"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(base, "triggers", "TaskFlagWitness.trigger"), string(body))
	writeFile(t, filepath.Join(base, "triggers", "TaskFlagWitness.trigger-meta.xml"), "<ApexTrigger><apiVersion>"+api+"</apiVersion><status>Active</status></ApexTrigger>")
	body, err = os.ReadFile(filepath.Join(source, "LyricTile__c.object"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(base, "objects", "LyricTile__c.object"), string(body))
	return root
}
