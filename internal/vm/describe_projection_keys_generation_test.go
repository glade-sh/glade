package vm

import "testing"

// Once a worker leaves its frozen schema generation, projection values still
// belong to each call, but their verified key metadata need not be rebuilt.
// Reusing keys requires exact key equality, including raw writes and caller
// projection changes. No cached Value or writable token crosses the boundary.
func TestDescribeProjectionKeysAfterMutableGeneration(t *testing.T) {
	for _, caller := range []string{"", "zzns", "foreign"} {
		t.Run(caller, func(t *testing.T) {
			base, template := describeSharePrimedBaseFor(t, true)
			machine := describeShareClone(base, template)
			if err := machine.RegisterClass(Class{Name: "KeyCaller", Namespace: caller}); err != nil {
				t.Fatal(err)
			}
			machine.currentClass = "KeyCaller"
			machine.Org.ClearRuntimeSchemaStamp()
			first := machine.globalDescribeMapValue()
			second := machine.globalDescribeMapValue()
			if sameMapBacking(first.Map, second.Map) {
				t.Fatal("projection values escaped their call")
			}
			if !sameMapBacking(first.MapKeys, second.MapKeys) {
				t.Fatal("mutable worker rebuilt identical immutable keys")
			}
			original := len(second.MapKeys)
			if _, _, err := machine.callValueMember("", machine.schemaGlobalDescribe(), "put", []Value{String("private"), sObjectTypeToken("Account")}, &Result{}); err != nil {
				t.Fatal(err)
			}
			written := machine.globalDescribeMapValue()
			if len(written.Map) != original+1 || len(written.MapKeys) != original+1 {
				t.Fatal("raw mutation reused an incomplete key plan")
			}
			if len(first.MapKeys) != original {
				t.Fatal("new key plan changed a retained holder")
			}
			machine.clearMetadataCaches()
			next := machine.globalDescribeMapValue()
			if sameMapBacking(first.MapKeys, next.MapKeys) {
				t.Fatal("metadata reset retained old private key metadata")
			}
		})
	}
}
