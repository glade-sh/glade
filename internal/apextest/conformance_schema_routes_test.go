package apextest

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Compile only executable row declarations. The schema base already links the
// passive runtime; the product request compiler would also rebuild a row org.
func conformanceSchemaRequestRuntime(t *testing.T, index typesys.Index) CompiledProjectRuntime {
	t.Helper()
	digests, err := authoritativeRuntimeSourceDigests(index, nil)
	if err != nil {
		t.Fatal(err)
	}
	sources := newSourceCache()
	if _, err := prepareRuntimeGeneration(index, digests, sources); err != nil {
		t.Fatal(err)
	}
	include := func(typ typesys.TypeSymbol) bool { return typ.HasSourceSnapshot() }
	methods := compileProjectMethodsWhere(index, include, sources)
	classes := compileProjectClassesWhere(index, methods, include, sources)
	triggers, _ := compileProjectTriggers(index, sources)
	if err := sources.validateCapturedSourceGeneration(); err != nil {
		t.Fatal(err)
	}
	return CompiledProjectRuntime{Methods: methods, Classes: classes, Triggers: triggers,
		PageNames: visualforcePageNames(index), expressionTypes: sema.RuntimeExpressionTypeResolver(index)}
}

func TestConformanceSchemaRequestLinksOnlyRowSources(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := soqlBindNativeIndex(t, api, map[string]string{"SchemaRow": "public class SchemaRow {public static Integer value(){return 1;}}"}, nil)
			runtime := conformanceSchemaRequestRuntime(t, index)
			if len(runtime.Classes) != 1 || runtime.Classes[0].Name != "SchemaRow" {
				t.Errorf("row request compiled %d classes; want only its source class", len(runtime.Classes))
			}
		})
	}
}

func conformanceSchemaCompareProjectRows(t *testing.T, api, value string, index typesys.Index, base, reused, fresh *conformanceRunner) {
	t.Helper()
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("project diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	t.Run(value+"/anonymous", func(t *testing.T) {
		program, err := vm.CompileAnonymousWithOptions("System.assertEquals('"+value+"',SchemaRow.value); System.assertEquals(0,[SELECT count() FROM SchemaEntry__c]); insert new SchemaEntry__c(Name='row'); System.debug([SELECT count() FROM SchemaEntry__c]);", vm.CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		got, gotErr := reused.execute(program)
		want, wantErr := fresh.execute(program)
		if gotErr != nil || wantErr != nil || !reflect.DeepEqual(got.Debug, want.Debug) {
			t.Fatalf("reused=%v/%v fresh=%v/%v", got.Debug, gotErr, want.Debug, wantErr)
		}
	})
	cases := Discover(index, Options{SelectedClasses: []string{"SchemaRowTest"}})
	if len(cases) != 1 {
		t.Fatalf("discovered %d methods", len(cases))
	}
	methods, methodErrors := compileTestMethods(cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	key := testCaseKey(cases[0])
	if methodErrors[key] != nil || programErrors[key] != nil {
		t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
	}
	t.Run(value+"/request", func(t *testing.T) {
		rowRuntime := conformanceSchemaRequestRuntime(t, index)
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, Org: base.orgSource, RequestRuntime: &rowRuntime})
		program, err := vm.CompileAnonymousWithOptions("System.assertEquals('"+value+"',SchemaRow.value); System.assertEquals(0,[SELECT count() FROM SchemaEntry__c]);", vm.CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := runner.execute(program); err != nil {
			t.Fatal(err)
		}
	})
	t.Run(value+"/isTest", func(t *testing.T) {
		for _, runner := range []*conformanceRunner{reused, fresh} {
			machine := runner.newMachine()
			machine.EnableTestContext()
			if err := machine.RegisterMethod(methods[key]); err != nil {
				t.Fatal(err)
			}
			if _, err := machine.ExecuteInClass(programs[key], cases[0].ClassName); err != nil {
				t.Fatal(err)
			}
		}
	})
	t.Run(value+"/runner", func(t *testing.T) {
		run := Run(index, Options{SelectedClasses: []string{"SchemaRowTest"}, NoDiskCache: true})
		if !run.Summary().OK() || run.Summary().Passed != 1 {
			t.Fatalf("project run: %#v", run)
		}
	})
}
