package vm

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Replay only the field/object describe facets from the owned native matrix.
// Record-type IDs and availability are retained as configuration provenance,
// not generalized into record-type assignment or DML behavior.
func TestUserFieldPermissionFallbackConformance(t *testing.T) {
	var data struct {
		Cells []struct {
			APIVersion    string            `json:"apiVersion"`
			Route         string            `json:"route"`
			Configuration string            `json:"configuration"`
			Fields        map[string][]bool `json:"fields"`
			Object        []bool            `json:"object"`
		} `json:"cells"`
	}
	raw, err := os.ReadFile("../apextest/testdata/conformance/user_field_permission_fallback.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cells) != 12 {
		t.Fatalf("native matrix has %d cells, want 12", len(data.Cells))
	}
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		key := cell.Configuration + "/" + cell.APIVersion + "/" + cell.Route
		if seen[key] || (cell.Route != "anonymous" && cell.Route != "istest") ||
			(cell.APIVersion != "62.0" && cell.APIVersion != "67.0" && cell.APIVersion != "68.0") ||
			(cell.Configuration != "granted" && cell.Configuration != "profile-only") {
			t.Fatalf("invalid or duplicate cell %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			machine := fieldPermissionFallbackMachine(cell.Configuration == "granted")
			machine.currentMethod = Method{APIVersion: cell.APIVersion}
			if cell.Route == "istest" {
				actor := machine.executionUser
				machine.EnableTestContext()
				machine.testContext.CurrentUser = actor
			}
			var source strings.Builder
			fields := []string{"UnlistedField__c", "GrantedField__c", "DeniedField__c", "RecordTypeId", "RequiredChoice__c"}
			facets := []string{"isAccessible", "isCreateable", "isUpdateable", "isPermissionable", "isNillable", "isDefaultedOnCreate"}
			if len(cell.Fields) != len(fields) || len(cell.Object) != 4 {
				t.Fatal("incomplete native describe observations")
			}
			for i, field := range fields {
				values := cell.Fields[field]
				if len(values) != len(facets) {
					t.Fatalf("incomplete native field %s", field)
				}
				fmt.Fprintf(&source, "Schema.DescribeFieldResult f%d=FallbackProbe__c.%s.getDescribe();\n", i, field)
				fmt.Fprintf(&source, "List<Boolean> field%d=new List<Boolean>{", i)
				for j, facet := range facets {
					if j > 0 {
						source.WriteString(",")
					}
					fmt.Fprintf(&source, "f%d.%s()", i, facet)
				}
				source.WriteString("};\n")
			}
			source.WriteString("List<Boolean> objectFacets=new List<Boolean>{")
			for i, facet := range []string{"isAccessible", "isCreateable", "isUpdateable", "isDeletable"} {
				if i > 0 {
					source.WriteString(",")
				}
				fmt.Fprintf(&source, "FallbackProbe__c.SObjectType.getDescribe().%s()", facet)
			}
			source.WriteString("};\n")
			program, err := CompileAnonymousWithOptions(source.String(), CompileOptions{APIVersion: cell.APIVersion})
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			compare := func(label string, value Value, want []bool) {
				t.Helper()
				if value.Kind != ValueList || len(value.List) != len(want) {
					t.Fatalf("%s result is not the captured boolean list", label)
				}
				for i, expected := range want {
					if got := value.List[i]; got.Kind != ValueBool || got.Bool != expected {
						t.Errorf("%s facet %d = %v; native %v", label, i, got, expected)
					}
				}
			}
			for i, field := range fields {
				compare(field, result.Vars[fmt.Sprintf("field%d", i)], cell.Fields[field])
			}
			compare("object", result.Vars["objectFacets"], cell.Object)
		})
	}
}

func fieldPermissionFallbackMachine(granted bool) *VM {
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	definition := storage.ObjectDefinition{APIName: "FallbackProbe__c", KeyPrefix: "a00", Fields: map[string]storage.Field{
		"UnlistedField__c": {APIName: "UnlistedField__c", Type: storage.FieldString},
		"GrantedField__c":  {APIName: "GrantedField__c", Type: storage.FieldString},
		"DeniedField__c":   {APIName: "DeniedField__c", Type: storage.FieldString},
		"RequiredChoice__c": {APIName: "RequiredChoice__c", Type: storage.FieldPicklist, Required: true, RestrictedPicklist: true,
			PicklistValues: []storage.PicklistValue{{Value: "First", Active: true}, {Value: "Second", Active: true}}},
	}, RecordTypes: []storage.RecordTypeInfo{
		{ID: "012000000000091", DeveloperName: "Available", Active: true, Available: true, Default: true},
		{ID: "012000000000092", DeveloperName: "Unavailable", Active: true, Available: false},
	}}
	storage.EnsureRecordTypeIDField(&definition)
	org.Objects[definition.APIName] = storage.ObjectState{Definition: definition, Records: map[storage.ID]storage.Record{}}
	put := func(object, id string, fields map[string]storage.Value) {
		state := org.Objects[object]
		state.Records[storage.ID(id)] = storage.Record{ID: storage.ID(id), Object: object, Fields: fields}
		org.Objects[object] = state
	}
	// Native Standard User has no profile-owned custom-object or field rows.
	// The declared false profile field serializes as absence, not a false row.
	put("PermissionSet", "0PS000000000099", map[string]storage.Value{
		"Name": storage.StringValue("FallbackProbeAccess"), "IsOwnedByProfile": storage.BooleanValue(false),
	})
	put("ObjectPermissions", "110000000000099", map[string]storage.Value{
		"ParentId": storage.IDValue("0PS000000000099"), "SObjectType": storage.StringValue("FallbackProbe__c"),
		"PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true),
		"PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(true),
	})
	put("FieldPermissions", "0FP000000000099", map[string]storage.Value{
		"ParentId": storage.IDValue("0PS000000000099"), "SObjectType": storage.StringValue("FallbackProbe__c"),
		"Field":           storage.StringValue("FallbackProbe__c.GrantedField__c"),
		"PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true),
	})
	if granted {
		put("PermissionSetAssignment", "0Pa000000000099", map[string]storage.Value{
			"AssigneeId": storage.IDValue("005000000000099"), "PermissionSetId": storage.IDValue("0PS000000000099"),
		})
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.executionUser = Object("User")
	machine.executionUser.Fields["Id"] = String("005000000000099")
	machine.executionUser.Fields["ProfileId"] = String("00e000000000005")
	return machine
}
