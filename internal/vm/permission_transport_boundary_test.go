package vm

import (
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Uncaptured input shapes must retain the earlier permission path.
func TestPermissionTransportUncapturedShapes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*VM, *Value)
	}{
		{"auto-number", func(vm *VM, _ *Value) {
			s := vm.Org.Objects["PermissionProbe__c"]
			s.Definition.Metadata["nameFieldType"] = "AutoNumber"
			f := s.Definition.Fields["Name"]
			f.AutoNumber = true
			s.Definition.Fields["Name"] = f
		}},
		{"id", func(_ *VM, v *Value) { v.Fields["Id"] = String("a00000000000001") }},
		{"extra field", func(_ *VM, v *Value) { v.Fields["Other__c"] = String("x") }},
		{"null name", func(_ *VM, v *Value) { v.Fields["Name"] = Null }},
		{"missing name", func(_ *VM, v *Value) { delete(v.Fields, "Name") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := permissionTransportVM("62.0", "ungranted")
			v := Object("PermissionProbe__c")
			v.Fields["Name"] = String("x")
			v.Fields["Summary__c"] = String("value")
			tc.mutate(vm, &v)
			if _, _, ok := vm.userModeInsertObjectDenial(v, true); ok {
				t.Fatal("uncaptured shape accepted")
			}
		})
	}
}

func TestSecurePermissionTransportAutoNumberName(t *testing.T) {
	vm := permissionTransportVM("62.0", "ungranted")
	vm.dynamicSOQLBindRoute = "Database.query"
	s := vm.Org.Objects["PermissionProbe__c"]
	s.Definition.Metadata["nameFieldType"] = "AutoNumber"
	f := s.Definition.Fields["Name"]
	f.Type = storage.FieldString
	f.AutoNumber = true
	s.Definition.Fields["Name"] = f
	q := soql.Query{Object: "PermissionProbe__c", Fields: []string{"Name", "Summary__c"}, SecurityMode: "SECURITY_ENFORCED"}
	if vm.api62DynamicSecurePermissionDenial(q, "") {
		t.Fatal("uncaptured AutoNumber name accepted")
	}
}
