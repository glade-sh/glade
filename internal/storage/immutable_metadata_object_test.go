package storage

import (
	"reflect"
	"strings"
	"testing"
)

// referenceIsImmutableMetadataObject is the implementation the allocation-free
// check replaced. It stays here as the oracle.
func referenceIsImmutableMetadataObject(objectName string) bool {
	name := strings.TrimSpace(objectName)
	if name == "" {
		return false
	}
	switch strings.ToLower(name) {
	case "apexclass", "apextrigger", "apexpage", "apexcomponent",
		"fieldpermissions", "objectpermissions", "setupentityaccess",
		"permissionset", "permissionsetgroup", "permissionsetgroupcomponent",
		"profile", "userrole",
		"recordtype", "layout", "staticresource",
		"customapplication", "apptabmember", "tabdefinition",
		"entitydefinition", "fielddefinition":
		return true
	default:
		return false
	}
}

func TestIsImmutableMetadataObjectMatchesReference(t *testing.T) {
	names := []string{
		"", " ", "\t\n", "PermissionSet", " permissionset ", "PERMISSIONSETGROUPCOMPONENT",
		"PermissionSetGroupComponentX", "PermissionSetGroupComponent ", " Profile ",
		"\u0085Layout", "LayKout", "Kayout", "Profİle", "Profıle", "FİeldDefinition",
		"Account", "account__c", "ns__ApexClass__c", "ApexClass__mdt", strings.Repeat("A", 200),
		strings.Repeat("a", 30) + "é", "Apex\u0000Class", "apexclassé",
	}
	for name := range immutableMetadataObjectNames {
		names = append(names, name, strings.ToUpper(name), " "+strings.ToUpper(name[:1])+name[1:]+"\t")
	}
	names = append(names, KnownStandardObjectNames()...)
	for _, name := range names {
		if got, want := IsImmutableMetadataObject(name), referenceIsImmutableMetadataObject(name); got != want {
			t.Errorf("IsImmutableMetadataObject(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestIsImmutableMetadataObjectDoesNotAllocate(t *testing.T) {
	names := []string{"PermissionSet", "Account", " FieldDefinition ", "PermissionSetGroupComponentLonger"}
	if allocs := testing.AllocsPerRun(100, func() {
		for _, name := range names {
			_ = IsImmutableMetadataObject(name)
		}
	}); allocs != 0 {
		t.Fatalf("IsImmutableMetadataObject allocated %.1f times per run", allocs)
	}
}

// referenceCloneRuntimeFrozenShared is the per-object rule of
// CloneRuntimeFrozenShared, evaluated with the reference name check.
func referenceCloneRuntimeFrozenSharedObject(name string, object ObjectState) ObjectState {
	if referenceIsImmutableMetadataObject(object.Definition.APIName) || referenceIsImmutableMetadataObject(name) {
		return object.CloneRuntimeSnapshot()
	}
	return object.CloneRuntimeFrozenDefinition()
}

func TestCloneRuntimeFrozenSharedMatchesDeepCloneForEveryObject(t *testing.T) {
	org := NewOrgState()
	for _, objectName := range KnownStandardObjectNames() {
		EnsureStandardObject(&org, objectName)
	}
	seedRecords := map[string]Record{
		"Account":       {ID: "001000000000001AAA", Object: "Account"},
		"PermissionSet": {ID: "0PS000000000001AAA", Object: "PermissionSet"},
		"Profile":       {ID: "00e000000000001AAA", Object: "Profile"},
	}
	for objectName, record := range seedRecords {
		object := org.Objects[objectName]
		object.Records[record.ID] = record
		org.Objects[objectName] = object
	}
	// A custom object stored under a key that differs from its API name, and
	// one without record or index maps.
	org.Objects["ns__Thing__c"] = ObjectState{Definition: ObjectDefinition{APIName: "ns__Thing__c"}, Records: map[ID]Record{}}
	org.Objects["ProfileAlias"] = ObjectState{Definition: ObjectDefinition{APIName: "Profile"}}
	deep := org.CloneRuntime()
	clone := org.CloneRuntimeFrozenShared()
	if len(clone.Objects) != len(org.Objects) {
		t.Fatalf("clone has %d objects, want %d", len(clone.Objects), len(org.Objects))
	}
	shared := 0
	for name, source := range org.Objects {
		got, ok := clone.Objects[name]
		if !ok {
			t.Fatalf("clone is missing %s", name)
		}
		want := referenceCloneRuntimeFrozenSharedObject(name, source)
		if got.RecordsShared != want.RecordsShared || got.IndexesShared != want.IndexesShared {
			t.Fatalf("%s shared flags = %v/%v, want %v/%v", name, got.RecordsShared, got.IndexesShared, want.RecordsShared, want.IndexesShared)
		}
		if got.RecordsShared {
			shared++
		}
		if !reflect.DeepEqual(got.Definition, deep.Objects[name].Definition) ||
			!reflect.DeepEqual(got.Records, deep.Objects[name].Records) ||
			!reflect.DeepEqual(got.Indexes, deep.Objects[name].Indexes) {
			t.Fatalf("%s differs from a deep clone", name)
		}
		if (got.Records == nil) != (source.Records == nil) || (got.Indexes == nil) != (source.Indexes == nil) {
			t.Fatalf("%s changed nil record or index maps", name)
		}
		sameRecords := source.Records != nil && reflect.ValueOf(got.Records).Pointer() == reflect.ValueOf(source.Records).Pointer()
		if sameRecords != got.RecordsShared {
			t.Fatalf("%s shares records = %v, flag = %v", name, sameRecords, got.RecordsShared)
		}
	}
	if shared == 0 {
		t.Fatal("no immutable metadata object shared its records")
	}
}
