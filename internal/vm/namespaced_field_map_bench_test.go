package vm

import "testing"

func BenchmarkCapturedFieldMapContains(b *testing.B) {
	machine := describeShareUnprimedFor(true)
	machine.currentMethod = Method{APIVersion: "67.0", Namespace: "zzns", SourceContextBound: true}
	object := machine.Org.Objects["zzns__Gadget__c"]
	fields := machine.describeSObjectFieldMap("zzns__Gadget__c", object.Definition)
	for _, tc := range []struct {
		name, key string
		want      bool
	}{
		{"qualified", "zzns__Plain__c", true},
		{"standard", "Name", true},
		{"absent", "Missing__c", false},
	} {
		b.Run(tc.name, func(b *testing.B) {
			args := []Value{String(tc.key)}
			v, handled, err := machine.callMapValueMember("", fields, "containsKey", args, &Result{})
			if err != nil || !handled || v.Kind != ValueBool || v.Bool != tc.want {
				b.Fatalf("outcome: %+v %t %v", v, handled, err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				machine.callMapValueMember("", fields, "containsKey", args, &Result{})
			}
		})
	}
}
