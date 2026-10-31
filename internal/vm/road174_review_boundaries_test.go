package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Uncaptured invalid fields must retain the pre-ticket query error.
func TestRoad174OrdinaryInvalidFieldPreservesShipping(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, field := range []string{"NoSuchField", "Order.NoSuchField"} {
			for _, route := range []string{"argument", "binds", "clause"} {
				t.Run(api+"/"+field+"/"+route, func(t *testing.T) {
					machine := road174ControlVM(t, api, "std-object", "anonymous")
					q := "SELECT " + field + " FROM Order"
					call := "Database.query(q,AccessLevel.SYSTEM_MODE)"
					if route == "binds" {
						call = "Database.queryWithBinds(q,new Map<String,Object>(),AccessLevel.SYSTEM_MODE)"
					}
					if route == "clause" {
						q += " WITH SYSTEM_MODE"
						call = "Database.query(q)"
					}
					expected := "System.QueryException"
					if route == "binds" {
						// Shipping validates this access-level argument before parsing fields.
						expected = "System.SecurityException"
					}
					src := fmt.Sprintf("String q='%s';String got='';try{%s;}catch(Exception e){got=e.getTypeName();}System.assertEquals('%s',got);", q, call, expected)
					prog, err := CompileAnonymousWithOptions(src, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err = machine.Execute(prog); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

// Nested projections of these objects have no native rows; the base runtime's
// record access result remains the local contract for this path.
func TestRoad174NestedProjectionPreservesShipping(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, target := range []string{"Asset", "Contract", "Order", "Pricebook2"} {
			t.Run(api+"/"+target, func(t *testing.T) {
				machine := road174ControlVM(t, api, "admin", "named")
				storage.EnsureStandardObject(machine.Org, target)
				obj := machine.Org.Objects[target]
				id := storage.ID(obj.Definition.KeyPrefix + "000000000001")
				rec := storage.Record{ID: id, Object: target, Fields: map[string]storage.Value{}, System: storage.SystemFields{OwnerID: storage.DefaultUserID}}
				obj.Records = map[storage.ID]storage.Record{id: rec}
				machine.Org.Objects[target] = obj
				machine.Org.Objects["Road174Child__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Road174Child__c", Fields: map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}, "Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{target}}}, Relations: []storage.Relationship{{Field: "Parent__c", ParentObjects: []string{target}, ParentRelationship: "Parent__r"}}}, Records: map[storage.ID]storage.Record{"a01000000000001": {ID: "a01000000000001", Object: "Road174Child__c", Fields: map[string]storage.Value{"Parent__c": storage.IDValue(id)}, System: storage.SystemFields{OwnerID: storage.DefaultUserID}}}}
				want := machine.userRecordAccessForRecord(target, rec).Fields["HasAllAccess"].Boolean
				src := fmt.Sprintf("for(Integer i=0;i<2;i++){List<SObject> rows=Database.query('SELECT Parent__r.UserRecordAccess.HasAllAccess FROM Road174Child__c WITH SYSTEM_MODE');System.assertEquals(%t,rows[0].getSObject('Parent__r').getSObject('UserRecordAccess').get('HasAllAccess'));}", want)
				prog, err := CompileAnonymousWithOptions(src, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = machine.Execute(prog); err != nil {
					t.Fatal(err)
				}
				if _, err = machine.ExecuteInClass(prog, "Road174Context"); err != nil {
					t.Fatal(err)
				}
				if err = machine.RegisterClass(Class{Name: "Road174Test", APIVersion: api, IsTest: true}); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				if _, err = machine.ExecuteInClass(prog, "Road174Test"); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
