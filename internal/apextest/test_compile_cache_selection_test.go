package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// A one-method run and a later whole-class run in the same process share the
// compiled test methods, but not the invocation programs: each selected
// method must still run. The cache once returned the one-method run's
// programs, so the other methods passed without executing.
func TestTestCompileCacheKeepsInvocationsPerSelection(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/classes/SelectionTest.cls"), `@IsTest
private class SelectionTest {
    @IsTest static void passes() {
        System.assert(true);
    }
    @IsTest static void fails() {
        System.assert(false, 'must run');
    }
}`)
	index := loadTestIndex(t, root)
	opts := Options{NoDiskCache: true, Parallelism: 1, SelectedClasses: []string{"SelectionTest"}}
	one := opts
	one.SelectedMethod = "passes"
	if summary := Run(index, one).Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("one-method run = %+v", summary)
	}
	run := Run(index, opts)
	statuses := map[string]testreport.Status{}
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			statuses[c.MethodName] = c.Status
		}
	}
	if statuses["passes"] != testreport.StatusPass || statuses["fails"] != testreport.StatusFail {
		t.Fatalf("whole-class run after a one-method run = %v", statuses)
	}
}
