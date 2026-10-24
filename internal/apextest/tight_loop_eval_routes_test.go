package apextest

import (
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// The owned project exercises positional calls, named constructor arguments,
// argument evaluation order, and a rejected list boundary on the shared evaluator.
func TestTightLoopArgumentProjectRoutes(t *testing.T) {
	const body = `
 List<Integer> pixels=new List<Integer>{1,2,3,4};
 Integer checksum=0;
 for(Integer i=0;i<4000;i++){
   Integer at=i&3;
   Integer value=(pixels.get(at)+i)&255;
   pixels.set(at,value);
   checksum+=value;
 }
 System.assertEquals(485832,checksum);
 System.assertEquals(177,pixels.get(0));
 System.assertEquals(154,pixels.get(1));
 System.assertEquals(131,pixels.get(2));
 System.assertEquals(108,pixels.get(3));
 System.assertEquals(4,pixels.size());
 System.assertEquals(4,'abcd'.length());
 Account emptyRecord=new Account();
 Account record=new Account(Name='edge');
 System.assertEquals(null,emptyRecord.Name);
 System.assertEquals('edge',record.Name);
 Integer cursor=0;
 List<Integer> ordered=new List<Integer>{0,0};
 ordered.set(cursor++,cursor++);
 System.assertEquals(2,cursor);
 System.assertEquals(1,ordered.get(0));
 Boolean rejected=false;
 try { pixels.get(-1); } catch(ListException error){ rejected=true; }
 System.assertEquals(true,rejected);
 `
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := nativeBoundaryIndex(t, api, map[string]string{
				"ArgumentLoop":     `public class ArgumentLoop { public static void run(){` + body + `} }`,
				"ArgumentLoopTest": `@IsTest private class ArgumentLoopTest { @IsTest static void run(){ArgumentLoop.run();} }`,
			}, nil)
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			for _, route := range []struct{ name, source string }{
				{"anonymous", body}, {"named", "ArgumentLoop.run();"},
			} {
				t.Run(route.name, func(t *testing.T) {
					program, err := vm.CompileAnonymousWithOptions(route.source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					if _, err := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org}).execute(program); err != nil {
						t.Fatal(err)
					}
					t.Log("ROW " + api + "/" + route.name + " checksum=485832 pixels=177,154,131,108 ordered=1 cursor=2 rejected=true")
				})
			}
			for _, tracing := range []bool{false, true} {
				name := "isTest_runner_untraced"
				if tracing {
					name = "isTest_runner_traced"
				}
				t.Run(name, func(t *testing.T) {
					report := Run(index, Options{SelectedClasses: []string{"ArgumentLoopTest"}, NoDiskCache: true, Parallelism: 1, TraceAll: tracing})
					if summary := report.Summary(); summary.Passed != 1 || summary.Failed != 0 {
						t.Fatalf("%+v", report)
					}
					t.Log("ROW " + api + "/" + name + " checksum=485832 pixels=177,154,131,108 ordered=1 cursor=2 rejected=true")
				})
			}
		})
	}
}
