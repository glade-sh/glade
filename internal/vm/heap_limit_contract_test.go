package vm

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// heapGrowthProgram holds n distinct 100,001-character strings, as the HEAP2
// rows do, then asserts the heap passed the reported cap.
func heapGrowthProgram(t *testing.T, n int) ir.Program {
	t.Helper()
	program, err := CompileAnonymous(`
List<String> parts = new List<String>();
String chunk = 'x'.repeat(100000);
for (Integer i = 0; i < ` + strconv.Itoa(n) + `; i++) { parts.add(chunk + i); }
System.assert(Limits.getHeapSize() > Limits.getLimitHeapSize());
`)
	if err != nil {
		t.Fatal(err)
	}
	return program
}

// Native sync Apex reports a 10000000 heap cap but completed holding about
// 30 MB (HEAP2 HEAP-SR/SRE, HEAP3-L30A/T). Under the native default cap,
// strict mode records the violation without throwing below the threshold.
func TestStrictHeapLimitRecordsNativeDefaultCapViolation(t *testing.T) {
	machine := New(nil)
	machine.SetLimitMode(LimitModeStrict)
	result, err := machine.Execute(heapGrowthProgram(t, 300))
	if err != nil {
		t.Fatalf("native default cap: %v", err)
	}
	violation := LimitViolation{}
	for _, item := range result.LimitViolations {
		if item.Name == "heapSize" {
			violation = item
		}
	}
	if violation.Limit != syncHeapLimit || violation.Used <= 29_000_000 || violation.Used > nativeHeapThreshold {
		t.Fatalf("native cap violations = %#v", result.LimitViolations)
	}
}

// Salesforce throws "Apex heap size too large: <heap>" once the heap first
// exceeds about 50 MiB, sync and async alike (HEAP3-L60A/T, HEAP3-Q60/Q100).
// Permissive mode never throws a LimitException.
func TestStrictHeapLimitThrowsNativeMessageAboveThreshold(t *testing.T) {
	program := heapGrowthProgram(t, 530)
	machine := New(nil)
	machine.SetLimitMode(LimitModeStrict)
	_, err := machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.LimitException" || !strings.HasPrefix(runtimeErr.Message, "Apex heap size too large: ") {
		t.Fatalf("strict: %v", err)
	}
	used, convErr := strconv.Atoi(strings.TrimPrefix(runtimeErr.Message, "Apex heap size too large: "))
	if convErr != nil || used <= nativeHeapThreshold || used > nativeHeapThreshold+200_000 {
		t.Fatalf("strict threw at %q, want the first allocation past %d", runtimeErr.Message, nativeHeapThreshold)
	}
	permissive := New(nil)
	if _, err := permissive.Execute(program); err != nil {
		t.Fatalf("permissive: %v", err)
	}
}

// An explicit cap is local and always throws in strict mode, including one
// equal to a native default (--limit-heap-size 10000000 or 25000000).
func TestStrictHeapLimitThrowsForExplicitCaps(t *testing.T) {
	for _, tc := range []struct {
		name     string
		heapSize int
		strings  int
	}{
		{"local 2MiB", 2 * 1024 * 1024, 30},
		{"explicit 10000000", syncHeapLimit, 110},
		{"explicit 25000000", asyncHeapLimit, 260},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			machine.SetLimitMode(LimitModeStrict)
			caps := defaultLimitCaps()
			caps.HeapSize = tc.heapSize
			machine.SetLimitCaps(caps)
			_, err := machine.Execute(heapGrowthProgram(t, tc.strings))
			if err == nil || !strings.Contains(err.Error(), "System.LimitException: Too many heapSize") || !strings.Contains(err.Error(), "out of "+strconv.Itoa(tc.heapSize)) {
				t.Fatalf("explicit cap %d: %v", tc.heapSize, err)
			}
		})
	}
}

// Async entry keeps an explicit heap cap as configured: a Queueable, @future
// or Scheduled job under an explicit 10000000 cap throws past it instead of
// widening to the 25000000 async default.
func TestStrictHeapLimitKeepsExplicitCapsAcrossAsyncEntry(t *testing.T) {
	worker, err := CompileAnonymous(`
List<String> parts = new List<String>();
String chunk = 'x'.repeat(100000);
for (Integer i = 0; i < 110; i++) { parts.add(chunk + i); }
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, start string
		class       Class
	}{
		{"Queueable", "System.enqueueJob(new HeapWorker());", Class{Name: "HeapWorker", Interfaces: []string{"Queueable"}, Methods: map[string]Method{
			"execute": {Name: "HeapWorker.execute", ClassName: "HeapWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: worker},
		}}},
		{"future", "HeapWorker.run();", Class{Name: "HeapWorker", Methods: map[string]Method{
			"run": {Name: "HeapWorker.run", ClassName: "HeapWorker", ReturnType: "void", IsStatic: true, Modifiers: []string{"future"}, Program: worker},
		}}},
		{"Scheduled", "System.schedule('heap', '0 0 0 1 1 ? 2099', new HeapWorker());", Class{Name: "HeapWorker", Interfaces: []string{"Schedulable"}, Methods: map[string]Method{
			"execute": {Name: "HeapWorker.execute", ClassName: "HeapWorker", ReturnType: "void", Params: []Param{{Name: "context", Type: "SchedulableContext"}}, Program: worker},
		}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous("Test.startTest();\n" + tc.start + "\nTest.stopTest();\n")
			if err != nil {
				t.Fatal(err)
			}
			for _, explicit := range []bool{false, true} {
				machine := New(nil)
				org := testDataOrg()
				machine.SetOrg(&org)
				machine.EnableTestContext()
				machine.SetLimitMode(LimitModeStrict)
				if explicit {
					caps := defaultLimitCaps()
					caps.HeapSize = syncHeapLimit
					machine.SetLimitCaps(caps)
				}
				if err := machine.RegisterClass(tc.class); err != nil {
					t.Fatal(err)
				}
				_, err := machine.Execute(program)
				if !explicit {
					// The native default cap records about 11 MB without throwing.
					if err != nil {
						t.Fatalf("native default cap: %v", err)
					}
					continue
				}
				if err == nil || !strings.Contains(err.Error(), "Too many heapSize") || !strings.Contains(err.Error(), "out of 10000000") {
					t.Fatalf("explicit 10000000 cap: %v", err)
				}
			}
		})
	}
}
