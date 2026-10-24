package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// WITH SECURITY_ENFORCED has no native hierarchy-setting row. The boundary
// policy must leave its record-sharing behavior at the pre-existing baseline.
func TestHierarchyQuerySecurityEnforcedKeepsBaselineSharing(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, sharing := range []string{"with sharing", "without sharing"} {
			for _, route := range []string{"static", "dynamic"} {
				t.Run(api+"/"+sharing+"/"+route, func(t *testing.T) {
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
					machine.testContext.CurrentUser = actor
					machine.testContext.RunAsDepth = 1
					body := "[SELECT Disabled__c FROM BoundaryPreference__c WITH SECURITY_ENFORCED]"
					if route == "dynamic" {
						body = "Database.query('SELECT Disabled__c FROM BoundaryPreference__c WITH SECURITY_ENFORCED')"
					}
					program, err := CompileAnonymousWithOptions("return "+body+";", CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					got, err := machine.callMethod(Method{Name: "R.read", ClassName: "R", IsStatic: true, APIVersion: api, SourceContextBound: true, SharingMode: sharing, Program: program}, nil, &Result{})
					if api == "67.0" { // baseline: SECURITY_ENFORCED is rejected under secure defaults
						if err == nil {
							t.Fatalf("expected rejection, got %s", got.String())
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					wantRows := 1
					if sharing == "with sharing" {
						wantRows = 0 // baseline record sharing hides the org row; no native row changes it
					}
					if len(got.List) != wantRows {
						t.Fatalf("rows=%d want %d", len(got.List), wantRows)
					}
				})
			}
		}
	}
}
