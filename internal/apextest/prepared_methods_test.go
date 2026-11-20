package apextest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

func BenchmarkWideTestClass(b *testing.B) {
	root := b.TempDir()
	writeBenchmarkTestFile(b, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	var source strings.Builder
	source.WriteString("@IsTest private class WideTest {\n")
	for i := 0; i < 256; i++ {
		fmt.Fprintf(&source, "@IsTest static void test%03d() { System.assertEquals(%d, %d); }\n", i, i, i)
	}
	source.WriteString("}")
	writeBenchmarkTestFile(b, filepath.Join(root, "force-app/main/default/classes/WideTest.cls"), source.String())
	index := benchmarkLoadTestIndex(b, root)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run := Run(index, Options{NoDiskCache: true})
		if s := run.Summary(); s.Total != 256 || s.Passed != 256 {
			b.Fatalf("summary = %+v", s)
		}
	}
}

func TestWideClassMethodIsolation(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	var source strings.Builder
	source.WriteString(`@IsTest private class WideIsolationTest {
 static Integer value = 0;
 @TestSetup static void setup() { insert new Account(Name='seed'); }
`)
	for i := 0; i < 12; i++ {
		fmt.Fprintf(&source, `@IsTest static void test%02d() {
 System.assertEquals(0, value); value = %d;
 System.assertEquals(1, [SELECT count() FROM Account]);
 insert new Account(Name='added'); System.assertEquals(2, [SELECT count() FROM Account]);
 }
`, i, i+1)
	}
	source.WriteString("@IsTest static void fails() { System.assert(false, 'expected failure'); }\n}")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WideIsolationTest.cls"), source.String())
	index := loadTestIndex(t, root)
	var serial string
	for _, opts := range []Options{{NoDiskCache: true}, {NoDiskCache: true, Parallelism: 2, ParallelMethods: true}} {
		run := Run(index, opts)
		run.DurationMS = 0
		for i := range run.Suites {
			run.Suites[i].DurationMS = 0
			for j := range run.Suites[i].Cases {
				run.Suites[i].Cases[j].DurationMS = 0
			}
		}
		data, err := json.Marshal(run)
		if err != nil {
			t.Fatal(err)
		}
		receipt := strings.ReplaceAll(string(data), root, "<root>")
		if serial == "" {
			serial = receipt
		} else if receipt != serial {
			t.Fatalf("serial and parallel outcomes differ: %s | %s", serial, receipt)
		}
		t.Logf("outcomes: %s", receipt)
		statuses := runCaseStatuses(run)
		if len(statuses) != 13 || statuses["fails"] != testreport.StatusFail {
			t.Fatalf("statuses = %v", statuses)
		}
		for i := 0; i < 12; i++ {
			name := fmt.Sprintf("test%02d", i)
			if statuses[name] != testreport.StatusPass {
				t.Errorf("%s = %s; problem %s", name, statuses[name], firstRunProblem(run))
			}
		}
	}
}

func TestSingleSelectedMethodNeighbor(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/NeighborTest.cls"), `@IsTest private class NeighborTest {
 @IsTest static void selected() { System.assertEquals(2, 1 + 1); }
 @IsTest static void other() { System.assert(false); }
}`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, SelectedMethod: "selected"})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	run.DurationMS = 0
	for i := range run.Suites {
		run.Suites[i].DurationMS = 0
		for j := range run.Suites[i].Cases {
			run.Suites[i].Cases[j].DurationMS = 0
		}
	}
	data, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("neighbor: %s", strings.ReplaceAll(string(data), root, "<root>"))
}

func TestPreparedMethodsSourceAndCloneIsolation(t *testing.T) {
	base := vm.New(nil)
	if err := base.RegisterMethod(vm.Method{Name: "Base.run", ClassName: "Base"}); err != nil {
		t.Fatal(err)
	}
	base.FreezeClassLookup()
	cases := []TestCase{{ClassName: "One", File: "a.cls"}, {ClassName: "One", File: "a.cls"}, {ClassName: "One", File: "b.cls"}, {ClassName: "One", File: "b.cls"}, {ClassName: "Other", File: "a.cls"}, {ClassName: "Other", File: "a.cls"}, {ClassName: "Solo", File: "solo.cls"}}
	groups := indexTestRuntimeMethods(map[string]vm.Method{
		"a1": {Name: "One.a", ClassName: "One", File: "a.cls"}, "a2": {Name: "One.aa", ClassName: "One", File: "a.cls"},
		"b1": {Name: "One.b", ClassName: "One", File: "b.cls"}, "b2": {Name: "One.bb", ClassName: "One", File: "b.cls"},
		"other1": {Name: "Other.x", ClassName: "Other", File: "a.cls"}, "other2": {Name: "Other.y", ClassName: "Other", File: "a.cls"},
		"solo": {Name: "Solo.run", ClassName: "Solo", File: "solo.cls"},
	})
	prepared := prepareTestMethodMachines(base, cases, groups)
	a, b := prepared[testMethodSourceKey("One", "a.cls")], prepared[testMethodSourceKey("One", "b.cls")]
	if a == nil || b == nil || len(prepared) != 3 {
		t.Fatalf("prepared groups = %d", len(prepared))
	}
	names := func(machine *vm.VM) map[string]bool {
		result := map[string]bool{}
		machine.RangeRegisteredMethods(func(method vm.Method) { result[method.Name] = true })
		return result
	}
	if got := names(a); !got["Base.run"] || !got["One.a"] || got["One.b"] || got["Other.x"] {
		t.Fatalf("first source methods = %v", got)
	}
	if got := names(b); !got["Base.run"] || !got["One.b"] || got["One.a"] {
		t.Fatalf("second source methods = %v", got)
	}
	clone := a.CloneRuntimeFrozenShared(nil)
	if err := clone.RegisterMethod(vm.Method{Name: "One.late", ClassName: "One"}); err != nil {
		t.Fatal(err)
	}
	if names(a)["One.late"] || names(b)["One.late"] || names(base)["One.a"] {
		t.Fatal("registration escaped its clone")
	}
	if !names(clone)["One.late"] {
		t.Fatal("clone lost its registration")
	}
	// A new run derives its tables independently from the base and methods.
	changed := vm.New(nil)
	if err := changed.RegisterMethod(vm.Method{Name: "Changed.run", ClassName: "Changed"}); err != nil {
		t.Fatal(err)
	}
	changed.FreezeClassLookup()
	rebuiltBase := prepareTestMethodMachines(changed, cases, groups)[testMethodSourceKey("One", "a.cls")]
	if got := names(rebuiltBase); !got["Changed.run"] || !got["One.a"] || got["Base.run"] {
		t.Fatalf("rebuilt base methods = %v", got)
	}
	groups[testMethodSourceKey("One", "a.cls")] = []vm.Method{{Name: "One.new", ClassName: "One"}, {Name: "One.next", ClassName: "One"}}
	rebuiltMethods := prepareTestMethodMachines(base, cases, groups)[testMethodSourceKey("One", "a.cls")]
	if got := names(rebuiltMethods); !got["Base.run"] || !got["One.new"] || got["One.a"] {
		t.Fatalf("rebuilt source methods = %v", got)
	}
}
