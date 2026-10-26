package apextest

import (
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// The same bulk statement reaches the event boundary from anonymous execution,
// a named method, and the project test runner, with concrete and erased lists.
func TestStatementInsertBulkRouteTwins(t *testing.T) {
	const body = `
List<Account> concrete = new List<Account>();
List<SObject> erased = new List<SObject>();
for(Integer i=0;i<64;i++) {
 concrete.add(new Account(Name='Concrete '+i));
 erased.add(new Account(Name='Erased '+i));
}
insert concrete;
insert erased;
System.assertEquals(128,[SELECT COUNT() FROM Account]);
for(Account row:concrete) System.assertNotEquals(null,row.Id);
for(SObject row:erased) System.assertNotEquals(null,row.Id);
`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := nativeBoundaryIndex(t, api, map[string]string{
				"BulkInsertProbe":     "public class BulkInsertProbe { public static void observe() {" + body + "} }",
				"BulkInsertProbeTest": "@IsTest private class BulkInsertProbeTest { @IsTest static void observe() { BulkInsertProbe.observe(); } }",
			}, nil)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("project compile: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			for _, route := range []string{"anonymous", "namedAnonymous", "isTest"} {
				t.Run(route, func(t *testing.T) {
					if route == "isTest" {
						run := Run(index, Options{SelectedClasses: []string{"BulkInsertProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
						if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
							t.Fatalf("project test route: %+v", run)
						}
						return
					}
					source := body
					if route == "namedAnonymous" {
						source = "BulkInsertProbe.observe();"
					}
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					if _, err := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org}).execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}
