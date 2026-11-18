package apextest

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Two parents that both refire in the update pass fire in the order the insert
// pass fired them, on every run. No native row has two refiring parents: this
// pins determinism and the insert pass's order only.
func TestMixedUpsertRefiringParentsKeepInsertPassOrder(t *testing.T) {
	const src = `MxP__c pA = new MxP__c(Name='A'); insert pA;
insert new MxC__c(Name='ao', P__c=pA.Id, Amt__c=10, ExtId__c='ao');
MxP__c pB = new MxP__c(Name='B'); insert pB;
insert new MxC__c(Name='bo', P__c=pB.Id, Amt__c=10, ExtId__c='bo');
MxLog.reset('none');
upsert new List<MxC__c>{
  new MxC__c(ExtId__c='ao', Name='ao2', P__c=pA.Id, Amt__c=20),
  new MxC__c(ExtId__c='an', Name='an2', P__c=pA.Id, Amt__c=7),
  new MxC__c(ExtId__c='bo', Name='bo2', P__c=pB.Id, Amt__c=20),
  new MxC__c(ExtId__c='bn', Name='bn2', P__c=pB.Id, Amt__c=7)} MxC__c.ExtId__c;
System.debug('OBS|' + MxLog.seen);`
	family := gapFamily{dir: "testdata/conformance/review_round7", adminFrom: []string{"GapAccess"}}
	root := family.writeProject(t, "62.0")
	index := loadTestIndex(t, root)
	if analysis := sema.AnalyzeAnonymous(index, src, "62.0"); analysis.HasErrors() {
		t.Fatalf("anonymous: %v", analysis.Diagnostics)
	}
	var first string
	for run := 0; run < 20; run++ {
		org := orgFromIndex(index)
		org.APIVersion = "62.0"
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
		triggers, triggerErrors := compileProjectTriggers(index)
		if len(triggerErrors) != 0 {
			t.Fatalf("trigger compile: %v", triggerErrors)
		}
		for _, trigger := range triggers {
			if err := runner.base.RegisterTrigger(trigger); err != nil {
				t.Fatal(err)
			}
		}
		machine := runner.newMachine()
		machine.SetToolingExecuteAnonymous(true)
		program, err := vm.CompileAnonymousWithOptions(src, vm.CompileOptions{APIVersion: "62.0", ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(index, src, "62.0")})
		if err != nil {
			t.Fatal(err)
		}
		result, err := machine.Execute(program)
		if err != nil {
			t.Fatal(err)
		}
		var got string
		for _, line := range result.Debug {
			if _, rest, ok := strings.Cut(line, "OBS|"); ok {
				got = rest
			}
		}
		if run == 0 {
			first = got
			for _, order := range [][2]string{{"PA=", "PB="}} {
				a, b := strings.Index(got, order[0]), strings.Index(got, order[1])
				if a < 0 || b < 0 || a > b {
					t.Fatalf("parent A must fire before parent B: %s", got)
				}
				if strings.LastIndex(got, order[0]) > strings.LastIndex(got, order[1]) {
					t.Fatalf("the update pass must refire A before B: %s", got)
				}
			}
			if strings.Count(got, "PA=") != 2 || strings.Count(got, "PB=") != 2 {
				t.Fatalf("both parents fire in each pass: %s", got)
			}
		} else if got != first {
			t.Fatalf("run %d differs:\n%s\n%s", run, got, first)
		}
	}
}
