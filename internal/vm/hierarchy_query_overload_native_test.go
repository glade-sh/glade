package vm

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestHierarchyQueryOverloadsDirectVMNativeRoutes(t *testing.T) {
	file, err := os.Open("../apextest/testdata/conformance/hierarchy_query_boundary/overloads/org.tsv")
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
	if len(rows) != 7 {
		t.Fatal("incomplete native overload matrix")
	}
	routes := []struct{ name, source string }{
		{"queryDefault", "Database.query(q).size()"},
		{"queryUser", "Database.query(q,AccessLevel.USER_MODE).size()"},
		{"querySystem", "Database.query(q,AccessLevel.SYSTEM_MODE).size()"},
		{"queryBindsUser", "Database.queryWithBinds(q,b,AccessLevel.USER_MODE).size()"},
		{"queryBindsSystem", "Database.queryWithBinds(q,b,AccessLevel.SYSTEM_MODE).size()"},
		{"countDefault", "Database.countQuery(c)"},
		{"countUser", "Database.countQuery(c,AccessLevel.USER_MODE)"},
		{"countSystem", "Database.countQuery(c,AccessLevel.SYSTEM_MODE)"},
		{"countBindsUser", "Database.countQueryWithBinds(c,b,AccessLevel.USER_MODE)"},
		{"countBindsSystem", "Database.countQueryWithBinds(c,b,AccessLevel.SYSTEM_MODE)"},
	}
	for _, row := range rows[1:] {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(row[2]), &raw); err != nil {
			t.Fatal(err)
		}
		native := map[string]map[string]any{}
		for context, payload := range raw {
			if context == "boundary" {
				continue
			}
			var nativeContext map[string]any
			if err := json.Unmarshal(payload, &nativeContext); err != nil {
				t.Fatal(err)
			}
			native[context] = nativeContext
		}
		for _, context := range []string{"adminWith", "adminWithout", "readerWith", "readerWithout"} {
			if _, ok := native[context]; !ok {
				continue
			}
			sharing := "without sharing"
			if strings.HasSuffix(context, "With") {
				sharing = "with sharing"
			}
			for _, route := range routes {
				t.Run(row[0]+"/"+row[1]+"/"+context+"/"+route.name, func(t *testing.T) {
					org := testDataOrg()
					storage.EnsureDeterministicPlatformData(&org)
					org.Objects["BoundaryPreference__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
						APIName: "BoundaryPreference__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"},
						Fields: map[string]storage.Field{"Disabled__c": {APIName: "Disabled__c", Type: storage.FieldBoolean}, "SetupOwnerId": {APIName: "SetupOwnerId", Type: storage.FieldReference, ReferenceTo: []string{"User", "Profile", "Organization"}}},
					}, Records: map[storage.ID]storage.Record{"a00000000000001": {ID: "a00000000000001", Object: "BoundaryPreference__c", Fields: map[string]storage.Value{"Disabled__c": storage.BooleanValue(true), "SetupOwnerId": storage.IDValue("00D000000000001")}, System: storage.SystemFields{OwnerID: "005000000000001"}}}}
					machine := New(nil)
					machine.SetOrg(&org)
					if row[1] != "anonymous" {
						machine.EnableTestContext()
					}
					if strings.HasPrefix(context, "reader") {
						profile := Object("Profile")
						profile.Fields["Name"] = String("Standard User")
						actor := Object("User")
						actor.Fields["Id"] = String("005000000000099")
						actor.Fields["ProfileId"] = String("00e000000000099")
						actor.Fields["Profile"] = profile
						machine.testContext.CurrentUser = actor
						machine.testContext.RunAsDepth = 1
						if row[1] == "isTestGrant" {
							sets := machine.Org.Objects["PermissionSet"]
							sets.Records["0PS000000000099"] = storage.Record{ID: "0PS000000000099", Object: "PermissionSet", Fields: map[string]storage.Value{"PermissionsViewAllCustomSettings": storage.BooleanValue(true)}}
							machine.Org.Objects["PermissionSet"] = sets
							assignments := machine.Org.Objects["PermissionSetAssignment"]
							assignments.Records["0Pa000000000099"] = storage.Record{ID: "0Pa000000000099", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue("005000000000099"), "PermissionSetId": storage.IDValue("0PS000000000099")}}
							machine.Org.Objects["PermissionSetAssignment"] = assignments
						}
					}
					source := "Id owner='00D000000000001';String q='SELECT Disabled__c FROM BoundaryPreference__c WHERE SetupOwnerId=:owner';String c='SELECT COUNT() FROM BoundaryPreference__c WHERE SetupOwnerId=:owner';Map<String,Object> b=new Map<String,Object>{'owner'=>owner};return " + route.source + ";"
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[0]})
					if err != nil {
						t.Fatal(err)
					}
					got, err := machine.callMethod(Method{Name: "OverloadReader.read", ClassName: "OverloadReader", IsStatic: true, APIVersion: row[0], SourceContextBound: true, SharingMode: sharing, Program: program}, nil, &Result{})
					want, ok := native[context][route.name]
					if !ok {
						t.Fatal("missing native route")
					}
					if denied, ok := want.(string); ok {
						var thrown *apexThrowError
						if !errors.As(err, &thrown) || exceptionQualifiedTypeName(thrown.value.Type) != denied {
							t.Fatalf("native=%s; got=%s error=%v", denied, got.String(), err)
						}
					} else if err != nil || got.String() != fmt.Sprint(want) {
						t.Fatalf("native=%v; got=%s error=%v", want, got.String(), err)
					}
				})
			}
		}
	}
}
