package vm

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestHierarchyQueryBoundaryDirectVMNativeRows(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "apextest", "testdata", "conformance", "hierarchy_query_boundary", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 15 {
		t.Fatal("incomplete native matrix")
	}
	for _, row := range rows[1:] {
		if row[1] == "anonymous" {
			continue
		}
		var native map[string]map[string]map[string]any
		if err := json.Unmarshal([]byte(row[2]), &native); err != nil {
			t.Fatal(err)
		}
		for _, sharing := range []string{"with sharing", "without sharing"} {
			context := "readerWithout"
			if sharing == "with sharing" {
				context = "readerWith"
			}
			for _, route := range []struct{ name, source, key string }{
				{"static", "return [SELECT Disabled__c FROM BoundaryPreference__c];", "static"},
				{"dynamic", "return Database.query('SELECT Disabled__c FROM BoundaryPreference__c');", "dynamic"},
				{"explicitUser", "return [SELECT Disabled__c FROM BoundaryPreference__c WITH USER_MODE];", "user"},
				{"explicitSystem", "return [SELECT Disabled__c FROM BoundaryPreference__c WITH SYSTEM_MODE];", "system"},
				{"count", "return [SELECT COUNT() FROM BoundaryPreference__c];", "count"},
				{"boundUser", "return Database.queryWithBinds('SELECT Disabled__c FROM BoundaryPreference__c',new Map<String,Object>{'orgId'=>'00D000000000001'},AccessLevel.USER_MODE);", "bound"},
			} {
				t.Run(row[0]+"/"+row[1]+"/"+sharing+"/"+route.name, func(t *testing.T) {
					org := testDataOrg()
					storage.EnsureDeterministicPlatformData(&org)
					org.Objects["BoundaryPreference__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
						APIName: "BoundaryPreference__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"},
						Fields: map[string]storage.Field{"Disabled__c": {APIName: "Disabled__c", Type: storage.FieldBoolean}, "SetupOwnerId": {APIName: "SetupOwnerId", Type: storage.FieldReference, ReferenceTo: []string{"User", "Profile", "Organization"}}},
					}, Records: map[storage.ID]storage.Record{"a00000000000001": {ID: "a00000000000001", Object: "BoundaryPreference__c", Fields: map[string]storage.Value{"Disabled__c": storage.BooleanValue(true), "SetupOwnerId": storage.IDValue("00D000000000001")}, System: storage.SystemFields{OwnerID: "005000000000001"}}}}
					machine := New(nil)
					machine.SetOrg(&org)
					machine.EnableTestContext()
					profile := Object("Profile")
					profile.Fields["Name"] = String("Standard User")
					actor := Object("User")
					actor.Fields["Id"] = String("005000000000099")
					actor.Fields["ProfileId"] = String("00e000000000099")
					actor.Fields["Profile"] = profile
					if row[1] == "isTestGrant" {
						sets := machine.Org.Objects["PermissionSet"]
						sets.Records["0PS000000000099"] = storage.Record{ID: "0PS000000000099", Object: "PermissionSet", Fields: map[string]storage.Value{"PermissionsViewAllCustomSettings": storage.BooleanValue(true)}}
						machine.Org.Objects["PermissionSet"] = sets
						assignments := machine.Org.Objects["PermissionSetAssignment"]
						assignments.Records["0Pa000000000099"] = storage.Record{ID: "0Pa000000000099", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue("005000000000099"), "PermissionSetId": storage.IDValue("0PS000000000099")}}
						machine.Org.Objects["PermissionSetAssignment"] = assignments
					}
					machine.testContext.CurrentUser = actor
					machine.testContext.RunAsDepth = 1

					source := "Id orgId='00D000000000001';" + strings.ReplaceAll(route.source, " FROM BoundaryPreference__c", " FROM BoundaryPreference__c WHERE SetupOwnerId = :orgId")
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[0]})
					if err != nil {
						t.Fatal(err)
					}
					result, err := machine.callMethod(Method{Name: "BoundaryReader.read", ClassName: "BoundaryReader", IsStatic: true, APIVersion: row[0], SourceContextBound: true, SharingMode: sharing, Program: program}, nil, &Result{})
					want := native["organization"][context]
					if message, denied := want[route.key+"Error"]; denied {
						var thrown *apexThrowError
						if !errors.As(err, &thrown) || exceptionQualifiedTypeName(thrown.value.Type) != message {
							t.Fatalf("native=%s; Glade rows=%d error=%v", message, len(result.List), err)
						}
						if route.key == "static" && thrown.value.Fields["message"].Text != want["staticMessage"] {
							t.Fatalf("native message=%v; Glade=%s", want["staticMessage"], thrown.value.Fields["message"].Text)
						}
					} else if route.key == "count" {
						if err != nil || result.String() != fmt.Sprint(want["countRows"]) {
							t.Fatalf("native count=%v; Glade=%s error=%v", want["countRows"], result.String(), err)
						}
					} else if err != nil || len(result.List) != int(want[route.key+"Rows"].(float64)) {
						t.Fatalf("native rows=%v; Glade rows=%d error=%v", want[route.key+"Rows"], len(result.List), err)
					}
				})
			}
		}
	}
}

// These handle controls preserve integration behavior; they do not assert native
// parity. The ordinary-query capture contains no locator or cursor observations.
func TestHierarchyQueryBoundaryQueryHandleDirectVMPreservation(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, sharing := range []string{"with sharing", "without sharing"} {
			for _, mode := range []string{"USER_MODE", "SYSTEM_MODE", "DEFAULT"} {
				for _, route := range []struct{ name, source string }{
					{"locator", "Database.QueryLocator h=Database.getQueryLocator(q,AccessLevel.MODE);return h.iterator().hasNext()?1:0;"},
					{"locatorWithBinds", "Database.QueryLocator h=Database.getQueryLocatorWithBinds(q,new Map<String,Object>(),AccessLevel.MODE);return h.iterator().hasNext()?1:0;"},
					{"cursor", "Database.Cursor h=Database.getCursor(q,AccessLevel.MODE);return h.getNumRecords();"},
					{"cursorWithBinds", "Database.Cursor h=Database.getCursorWithBinds(q,new Map<String,Object>(),AccessLevel.MODE);return h.getNumRecords();"},
					{"paginationCursor", "Database.PaginationCursor h=Database.getPaginationCursor(q,AccessLevel.MODE);return h.getNumRecords();"},
					{"paginationCursorWithBinds", "Database.PaginationCursor h=Database.getPaginationCursorWithBinds(q,new Map<String,Object>(),AccessLevel.MODE);return h.getNumRecords();"},
				} {
					if mode == "DEFAULT" && strings.Contains(route.name, "WithBinds") {
						continue
					}
					t.Run(api+"/"+sharing+"/"+mode+"/"+route.name, func(t *testing.T) {
						org := testDataOrg()
						storage.EnsureDeterministicPlatformData(&org)
						org.Objects["BoundaryPreference__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "BoundaryPreference__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{"Disabled__c": {APIName: "Disabled__c", Type: storage.FieldBoolean}}}, Records: map[storage.ID]storage.Record{"a00000000000001": {ID: "a00000000000001", Object: "BoundaryPreference__c", Fields: map[string]storage.Value{"Disabled__c": storage.BooleanValue(true)}, System: storage.SystemFields{OwnerID: "005000000000001"}}}}
						machine := New(nil)
						machine.SetOrg(&org)
						machine.EnableTestContext()
						profile := Object("Profile")
						profile.Fields["Name"] = String("Standard User")
						actor := Object("User")
						actor.Fields["Id"] = String("005000000000099")
						actor.Fields["Profile"] = profile
						machine.testContext.CurrentUser = actor
						machine.testContext.RunAsDepth = 1
						call := strings.ReplaceAll(route.source, "MODE", mode)
						if mode == "DEFAULT" {
							call = strings.ReplaceAll(route.source, ",AccessLevel.MODE", "")
						}
						source := "String q='SELECT Disabled__c FROM BoundaryPreference__c';" + call
						program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						got, err := machine.callMethod(Method{Name: "HandleReader.read", ClassName: "HandleReader", IsStatic: true, APIVersion: api, SourceContextBound: true, SharingMode: sharing, Program: program}, nil, &Result{})
						want := "0"
						effectiveMode := mode
						if mode == "DEFAULT" || route.name == "paginationCursor" {
							effectiveMode = "USER_MODE"
							if api == "62.0" {
								effectiveMode = "SYSTEM_MODE"
							}
						}
						if sharing == "without sharing" && effectiveMode == "SYSTEM_MODE" {
							want = "1"
						}
						if err != nil || got.String() != want {
							t.Fatalf("integration control rows=%s; got=%s error=%v", want, got.String(), err)
						}
					})
				}
			}
		}
	}
}
