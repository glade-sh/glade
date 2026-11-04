package vm

import "testing"

// BenchmarkNamedGlobalDescribeGet measures 1000 getGlobalDescribe().get(...)
// calls made by named code, where the reference objects extend the base map.
func BenchmarkNamedGlobalDescribeGet(b *testing.B) {
	machine := describeShareUnprimedFor(false)
	if err := machine.RegisterClass(Class{Name: "NamedDescribeCaller"}); err != nil {
		b.Fatal(err)
	}
	machine.currentClass = "NamedDescribeCaller"
	probe := machine.globalDescribeMapValue()
	if _, ok := probe.Map[mapKey(String("fieldservicemobileextension"))]; !ok {
		b.Skip("named reference objects are not exposed")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for range 1000 {
			view := machine.globalDescribeMapValue()
			if _, ok := view.Map[mapKey(String("account"))]; !ok {
				b.Fatal("missing account")
			}
		}
	}
}

func TestNamedGlobalDescribeExtensionTracksBaseWrites(t *testing.T) {
	machine := describeShareUnprimedFor(false)
	if err := machine.RegisterClass(Class{Name: "NamedDescribeCaller"}); err != nil {
		t.Fatal(err)
	}
	machine.currentClass = "NamedDescribeCaller"
	fsme := mapKey(String("fieldservicemobileextension"))
	first := machine.globalDescribeMapValue()
	if _, ok := first.Map[fsme]; !ok {
		t.Fatal("named view lost the reference object")
	}
	// A raw write to the base map is visible in the next named view.
	size := len(machine.globalDescribeMapValue().Map)
	if _, _, err := machine.callValueMember("", machine.schemaGlobalDescribe(), "put", []Value{String("private"), sObjectTypeToken("Account")}, &Result{}); err != nil {
		t.Fatal(err)
	}
	after := machine.globalDescribeMapValue()
	if len(after.Map) != size+1 || len(after.MapKeys) != size+1 {
		t.Fatal("named view is stale after a base write")
	}
	machine.currentClass = ""
	if _, ok := machine.globalDescribeMapValue().Map[fsme]; ok {
		t.Fatal("anonymous view exposes the reference object")
	}
}
