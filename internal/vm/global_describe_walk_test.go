package vm

import (
	"crypto/sha256"
	"fmt"
	"runtime"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

const globalDescribeWalkSource = `
Map<String, Schema.SObjectType> g = Schema.getGlobalDescribe();
List<String> names = new List<String>(g.keySet());
names.sort();
List<String> result = new List<String>();
for (String name : names) {
    Schema.DescribeSObjectResult d = g.get(name).getDescribe();
    result.add(name + ':' + d.getName() + ':' + d.getKeyPrefix());
}
return String.join(result, '|');`

func globalDescribeWalk(tb testing.TB, eager bool) string {
	tb.Helper()
	org := namesOnlyDescribeOrg("source", []string{"Communities"}, "")
	program, err := CompileAnonymousWithOptions(globalDescribeWalkSource, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		tb.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.eagerGlobalDescribe = eager
	machine.SetCurrentUser(org.Objects["User"].Records[storage.ID(storage.DefaultUserID)])
	got, err := machine.callMethod(Method{Name: "Anonymous.run", APIVersion: "67.0", SourceContextBound: true, Program: program}, nil, &Result{})
	if err != nil {
		tb.Fatal(err)
	}
	return got.Text
}

func TestGlobalDescribeFullWalkBudgetAndParity(t *testing.T) {
	want := globalDescribeWalk(t, true)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	got := globalDescribeWalk(t, false)
	runtime.ReadMemStats(&after)
	if got != want {
		t.Fatalf("full walk differs from completed spread: %s", probeDifference(want, got))
	}
	if len(got) < 10000 {
		t.Fatalf("full walk output too short: %d bytes", len(got))
	}
	if used := after.TotalAlloc - before.TotalAlloc; used > 3_000_000_000 {
		t.Fatalf("full walk allocated %d bytes, budget 3000000000", used)
	}
}

func BenchmarkGlobalDescribeFullWalk(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		globalDescribeWalk(b, false)
	}
}

func TestLazyInstallCounterResetsWithOrg(t *testing.T) {
	machine := New(nil)
	first := namesOnlyDescribeOrg("source", nil, "")
	machine.SetOrg(&first)
	machine.lazySingleInstalls = lazySingleInstallLimit - 1
	second := namesOnlyDescribeOrg("source", nil, "")
	machine.SetOrg(&second)
	if machine.lazySingleInstalls != 0 {
		t.Fatal("SetOrg retained the previous org's install count")
	}
	machine.lazySingleInstalls = lazySingleInstallLimit - 1
	third := namesOnlyDescribeOrg("source", nil, "")
	machine.SetRuntimeTemplateOrg(&third)
	if machine.lazySingleInstalls != 0 {
		t.Fatal("SetRuntimeTemplateOrg retained the previous org's install count")
	}
	machine.lazySingleInstalls = lazySingleInstallLimit - 1
	clone := machine.CloneRuntime(nil)
	if clone.lazySingleInstalls != 0 {
		t.Fatal("runtime clone inherited an install count")
	}
}

// This small namespaced neighbor's digest was recorded at the pre-fix base.
func TestGlobalDescribeNeighborMatchesBase(t *testing.T) {
	org := namesOnlyDescribeOrg("source", []string{"Sites", "Quotes"}, "nsx")
	source := namesOnlyProbeSource([]string{"Account", "Widget__c", "NoSuchObject"})
	out := runNamesOnlyProbe(t, &org, false, "67.0", namesOnlyCaller{"owner", "nsx", "nsx"}, false, false, source)
	got := fmt.Sprintf("%x", sha256.Sum256([]byte(out)))
	const want = "d5589844944036a144b004208074a94906cfab82243562a4c7ead3d062ba450d"
	if got != want {
		t.Fatalf("neighbor digest = %s, base = %s", got, want)
	}
}
