package vm

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// This is a local candidate for an omitted constructor default. Native stripping
// observations for this shape are pending; descriptor permission stays denied.
func TestImplicitCheckboxStripCandidate(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, ns := range []string{"", "parcel"} {
			t.Run(api+"/"+ns, func(t *testing.T) {
				machine, object, ready := implicitCheckboxStripMachine(api, ns, true, false, true)
				body := fmt.Sprintf(`
%s row=new %s(Name='Owned',%s='value');
Assert.isTrue(%s.SObjectType.getDescribe().isCreateable());
Assert.isTrue(%s.%s.getDescribe().isAccessible());
Assert.isFalse(%s.%s.getDescribe().isCreateable());
Assert.isFalse(row.getPopulatedFieldsAsMap().containsKey('%s'));
String before=JSON.serialize(row);
SObjectAccessDecision decision=Security.stripInaccessible(AccessType.CREATABLE,new List<SObject>{row});
Assert.areEqual(0,decision.getRemovedFields().size(),'Local omitted-default candidate');
Assert.areEqual(0,decision.getModifiedIndexes().size());
Assert.areEqual(before,JSON.serialize(row),'Source must stay unchanged');
Assert.areEqual(false,decision.getRecords()[0].get('%s'));
Assert.isFalse(decision.getRecords()[0].getPopulatedFieldsAsMap().containsKey('%s'));
Assert.isFalse(%s.%s.getDescribe().isCreateable());
`, object, object, implicitCheckboxField(ns, "Payload__c"), object, object, ready, object, ready, ready, ready, ready, object, ready)
				program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatalf("local omitted-default candidate failed: %v", err)
				}
			})
		}
	}
}

// These controls preserve local permission and provenance boundaries. They do
// not claim native equivalence for constructed marker combinations.
func TestImplicitCheckboxStripCandidateControls(t *testing.T) {
	for _, ns := range []string{"", "parcel"} {
		for _, shape := range []string{"explicitFalse", "explicitTrue", "queried", "cloned", "trigger", "persisted", "intrinsicReadonly", "intrinsicReadonlyAlias", "intrinsicUnreadable", "noRead", "unassignedUnenforced", "scoped", "update"} {
			t.Run(ns+"/"+shape, func(t *testing.T) {
				read, assigned := shape != "noRead", shape != "unassignedUnenforced"
				machine, object, ready := implicitCheckboxStripMachine("67.0", ns, read, false, assigned)
				row := implicitCheckboxConstruct(t, machine, object, ns)
				switch shape {
				case "explicitFalse", "explicitTrue":
					row.Fields[ready] = Bool(shape == "explicitTrue")
					markExplicitSObjectField(&row, ready)
				case "queried":
					row.Fields[sobjectQueriedFieldsField] = Map()
					markQueriedSObjectField(&row, ready)
				case "cloned":
					row.Fields[sobjectCloneMarkerField] = Bool(false)
				case "trigger":
					markTriggerSObject(&row)
				case "persisted":
					row.Fields["Id"] = String("a01000000000701")
				case "intrinsicReadonly", "intrinsicReadonlyAlias", "intrinsicUnreadable":
					state := machine.Org.Objects[object]
					field := state.Definition.Fields[ready]
					denied := false
					if shape == "intrinsicUnreadable" {
						field.Accessible = &denied
					} else {
						field.Createable = &denied
					}
					state.Definition.Fields[ready] = field
					machine.Org.Objects[object] = state
					if shape == "intrinsicReadonlyAlias" {
						row.Fields[strings.ToLower(ready)] = row.Fields[ready]
						delete(row.Fields, ready)
					}
				}
				access, permissionSet := "CREATABLE", ""
				if shape == "update" {
					access, permissionSet = "UPDATABLE", "0PS000000000701"
				}
				if shape == "scoped" {
					permissionSet = "0PS000000000701"
				}
				removed := Map()
				modified, err := machine.stripInaccessibleRecord(access, &row, shape != "unassignedUnenforced", removed, permissionSet)
				if err != nil {
					t.Fatal(err)
				}
				if !modified || !implicitCheckboxRemoved(removed, object, ready) {
					t.Fatalf("excluded provenance/permission shape changed: modified=%t, removed=%v", modified, removed)
				}
			})
		}
		for _, assigned := range []bool{false, true} {
			t.Run(ns+fmt.Sprintf("/grant-%t", assigned), func(t *testing.T) {
				machine, object, ready := implicitCheckboxStripMachine("67.0", ns, true, true, assigned)
				row := implicitCheckboxConstruct(t, machine, object, ns)
				removed := Map()
				modified, err := machine.stripInaccessibleRecord("CREATABLE", &row, true, removed, "")
				if !assigned {
					var thrown *apexThrowError
					if !errors.As(err, &thrown) || exceptionQualifiedTypeName(thrown.value.Type) != "System.NoAccessException" {
						t.Fatalf("unassigned root CRUD must stay denied: %v", err)
					}
				} else if err != nil || modified || implicitCheckboxRemoved(removed, object, ready) {
					t.Fatalf("writable grant changed: modified=%t, removed=%v, err=%v", modified, removed, err)
				}
			})
		}
	}
}

func implicitCheckboxStripMachine(api, namespace string, read, edit, assigned bool) (*VM, string, string) {
	org := storage.NewOrgState()
	org.Namespace, org.APIVersion = namespace, api
	storage.EnsureDeterministicPlatformData(&org)
	object, ready := implicitCheckboxField(namespace, "ParcelEntry__c"), implicitCheckboxField(namespace, "Ready__c")
	payload := implicitCheckboxField(namespace, "Payload__c")
	org.Objects[object] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: object, KeyPrefix: "a01", Fields: map[string]storage.Field{
			"Id": {APIName: "Id", Type: storage.FieldID}, "Name": {APIName: "Name", Type: storage.FieldString},
			payload: {APIName: payload, Type: storage.FieldString},
			ready:   {APIName: ready, Type: storage.FieldBoolean, DefaultValue: "false"},
		},
	}, Records: map[storage.ID]storage.Record{}}
	seed := func(name string, id storage.ID, fields map[string]storage.Value) {
		storage.EnsureStandardObject(&org, name)
		state := org.Objects[name]
		state.Records[id] = storage.Record{ID: id, Object: name, Fields: fields}
		org.Objects[name] = state
	}
	profileID := ""
	for id, row := range org.Objects["Profile"].Records {
		if value, ok := row.GetField("Name"); ok && value.String == "Standard User" {
			profileID = string(id)
			break
		}
	}
	seed("PermissionSet", "0PS000000000701", map[string]storage.Value{"Name": storage.StringValue("ParcelRead"), "Label": storage.StringValue("Parcel Read")})
	seed("ObjectPermissions", "0OP000000000701", map[string]storage.Value{
		"ParentId": storage.IDValue("0PS000000000701"), "SObjectType": storage.StringValue("ParcelEntry__c"),
		"PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true),
		"PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(true),
	})
	for i, grant := range []struct {
		name       string
		read, edit bool
	}{{"Payload__c", true, true}, {"Ready__c", read, edit}} {
		seed("FieldPermissions", storage.ID(fmt.Sprintf("0FP00000000070%d", i+1)), map[string]storage.Value{
			"ParentId": storage.IDValue("0PS000000000701"), "SObjectType": storage.StringValue("ParcelEntry__c"),
			"Field":           storage.StringValue("ParcelEntry__c." + grant.name),
			"PermissionsRead": storage.BooleanValue(grant.read), "PermissionsEdit": storage.BooleanValue(grant.edit),
		})
	}
	if assigned {
		seed("PermissionSetAssignment", "0Pa000000000701", map[string]storage.Value{"AssigneeId": storage.IDValue("005000000000701"), "PermissionSetId": storage.IDValue("0PS000000000701")})
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.executionUser = Object("User")
	machine.executionUser.Fields["Id"], machine.executionUser.Fields["ProfileId"] = String("005000000000701"), String(profileID)
	return machine, object, ready
}

func implicitCheckboxConstruct(t *testing.T, machine *VM, object, namespace string) Value {
	t.Helper()
	program, err := CompileAnonymous(fmt.Sprintf("%s row=new %s(Name='Owned',%s='value');", object, object, implicitCheckboxField(namespace, "Payload__c")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	return machine.Globals["row"]
}

func implicitCheckboxField(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "__" + name
}

func implicitCheckboxRemoved(removed Value, object, field string) bool {
	fields := removed.Map[mapKey(String(object))]
	for _, value := range fields.Set {
		if value.Kind == ValueString && strings.EqualFold(value.Text, field) {
			return true
		}
	}
	return false
}
