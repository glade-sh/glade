package apextest

import (
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
	"testing"
)

// Native permission-set Id acceptance: dispatch_followups/org.tsv:12-13.
// Type-token receivers: corpus_shape/native/org.tsv:6,11. Compile and execute
// both spellings without pinning the feature-dependent permission exception.
func TestPlatformConstantReceiverRoutes(t *testing.T) {
	const body = `Id permissionSetId = null;
        try { AccessLevel a = AccessLevel.USER_MODE.withPermissionSetId(permissionSetId); } catch (Exception e) {}
        try { System.AccessLevel b = System.AccessLevel.USER_MODE.withPermissionSetId(permissionSetId); } catch (Exception e) {}
        Account.SObjectType.getDescribe(); Account.Name.getDescribe();`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			classes := map[string]string{
				"ConstantRoutes":     "public class ConstantRoutes { public static void observe() {" + body + "} }",
				"ConstantRoutesTest": "@IsTest private class ConstantRoutesTest { @IsTest static void observe() {" + body + " ConstantRoutes.observe(); } }",
			}
			index := nativeBoundaryIndex(t, api, classes, nil)
			if result := sema.Analyze(index); result.HasErrors() {
				t.Fatal(result.Diagnostics)
			}
			if result := sema.AnalyzeAnonymous(index, body+" ConstantRoutes.observe();", api); result.HasErrors() {
				t.Fatal(result.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{"ConstantRoutesTest"}, NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("runner: %+v %+v", summary, run.Suites)
			}
			program, err := vm.CompileAnonymousWithOptions(body+" ConstantRoutes.observe();", vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			if _, err := runner.execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
