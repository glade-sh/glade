package apextest

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Enum values() and valueOf(String) are members of every enum type. Native
// rows accept them unqualified on inner enums: sema inner_type_scope/org.tsv:16-21
// (EventBus.values(), named and @IsTest) and inner_type_shapes/org.tsv:6-9,26-29
// (enum_receivers and qualified_enum_factories, anonymous and @IsTest). The
// names Status and Message also name non-implicit platform types; they must
// keep the same enum statics in the class, a subclass and an anonymous block.
func TestInnerEnumBuiltinStaticsRoutes(t *testing.T) {
	classes := map[string]string{
		"CapStatus": `public virtual class CapStatus {
    public enum Status { OPEN, DONE }
    public enum Message { HELLO }
    public static String value() {
        return String.valueOf(Status.values().size()) + Status.valueOf('DONE').name() + Message.values().size() + Message.valueOf('HELLO').name();
    }
}`,
		"CapStatusChild": `public class CapStatusChild extends CapStatus {
    public static String child() { return String.valueOf(Status.values().size()) + Status.valueOf('OPEN').name() + Message.valueOf('HELLO').ordinal(); }
}`,
	}
	testClasses := map[string]string{
		"CapStatus":      classes["CapStatus"],
		"CapStatusChild": classes["CapStatusChild"],
		"CapStatusTest": `@IsTest private class CapStatusTest {
    private enum Status { OPEN }
    @IsTest static void statics() {
        System.assertEquals('2DONE1HELLO', CapStatus.value());
        System.assertEquals('2OPEN0', CapStatusChild.child());
        System.assertEquals(1, Status.values().size());
        System.assertEquals(Status.OPEN, Status.valueOf('OPEN'));
    }
}`,
	}
	anonymous := "public enum Status { OPEN, DONE }\nSystem.debug('OBS|' + Status.values().size() + Status.valueOf('DONE').name() + CapStatus.value() + CapStatusChild.child());\n"
	errors := func(diagnostics []diagnostic.Diagnostic) []string {
		var out []string
		for _, d := range diagnostics {
			if d.Severity == diagnostic.Error {
				out = append(out, d.Code+" "+d.Message)
			}
		}
		return out
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api+"/named", func(t *testing.T) {
			index := nativeBoundaryIndex(t, api, classes, nil)
			if got := errors(append(index.Diagnostics, sema.Analyze(index).Diagnostics...)); len(got) != 0 {
				t.Fatalf("named check: %v", got)
			}
		})
		t.Run(api+"/isTest", func(t *testing.T) {
			index := nativeBoundaryIndex(t, api, testClasses, nil)
			if got := errors(append(index.Diagnostics, sema.Analyze(index).Diagnostics...)); len(got) != 0 {
				t.Fatalf("test project check: %v", got)
			}
			run := Run(index, Options{SelectedClasses: []string{"CapStatusTest"}, NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("runner: %+v %+v", summary, run.Suites)
			}
		})
		t.Run(api+"/anonymous", func(t *testing.T) {
			index := nativeBoundaryIndex(t, api, classes, nil)
			prepared := innerShapePrepareAnonymous(t, api, index, anonymous)
			if got := errors(prepared.diagnostics); len(got) != 0 {
				t.Fatalf("anonymous check: %v", got)
			}
			program, err := vm.CompileAnonymousWithOptions(prepared.body, vm.CompileOptions{APIVersion: api, ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(prepared.merged, prepared.body, api)})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(prepared.merged)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if len(prepared.transient.Types) > 0 {
				runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(prepared.transient, nil)
				if err != nil {
					t.Fatal(err)
				}
				runner = newConformanceRunner(t, prepared.transient, conformanceRunnerOptions{Base: runner, RequestRuntime: &runtime, Org: &org})
			}
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			observed := ""
			for _, line := range result.Debug {
				if _, value, ok := strings.Cut(line, "OBS|"); ok {
					observed = value
				}
			}
			if observed != "2DONE2DONE1HELLO2OPEN0" {
				t.Fatalf("anonymous output %q", observed)
			}
		})
	}
}
