package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Benchmark the field-access portion of a user-mode Account insert with
// multiple written fields and a populated permission inventory. The multi-field
// route retains the same successful outcome on the baseline and this branch.
func BenchmarkAccountFieldAccessInventory(b *testing.B) {
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "PermissionSetAssignment")
	profile := org.Objects["Profile"]
	profileRow := profile.Records["00e000000000002"]
	profileRow.Fields["Name"] = storage.StringValue("System Administrator")
	profile.Records["00e000000000002"] = profileRow
	org.Objects["Profile"] = profile
	account := org.Objects["Account"]
	for i := 0; i < 30; i++ {
		name := fmt.Sprintf("Field_%02d__c", i)
		account.Definition.Fields[name] = storage.Field{APIName: name, Type: storage.FieldString, DisplayType: "STRING", Length: 255}
	}
	org.Objects["Account"] = account
	perms := org.Objects["FieldPermissions"]
	for i := 0; i < 1000; i++ {
		id := storage.ID(fmt.Sprintf("0FP%012d", i))
		perms.Records[id] = storage.Record{ID: id, Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(storage.ID(fmt.Sprintf("0PS%012d", i))), "SObjectType": storage.StringValue("Account"), "Field": storage.StringValue(fmt.Sprintf("Account.Field_%02d__c", i%30)), "PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true)}}
	}
	org.Objects["FieldPermissions"] = perms
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.currentMethod.APIVersion = "67.0"
	user := Object("User")
	user.Fields["Id"] = String("005000000000998")
	user.Fields["ProfileId"] = String("00e000000000002")
	machine.testContext.CurrentUser = user
	record := Object("Account")
	setExplicitSObjectField(&record, "Name", String("Inventory"))
	for i := 0; i < 30; i++ {
		setExplicitSObjectField(&record, fmt.Sprintf("Field_%02d__c", i), String("v"))
	}
	if err := machine.enforceUserModeDMLAccessDeferring("insert", record, Null, false); err != nil {
		b.Fatalf("expected allowed record: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := machine.enforceUserModeDMLAccessDeferring("insert", record, Null, false); err != nil {
			b.Fatal(err)
		}
	}
}

// The narrow explicit USER_MODE partial route writes Name and one Text(255)
// field. It keeps the same successful outcome on the baseline and this branch.
func BenchmarkAccountFieldAccessNarrowPartial(b *testing.B) {
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "PermissionSetAssignment")
	profile := org.Objects["Profile"]
	profileRow := profile.Records["00e000000000002"]
	profileRow.Fields["Name"] = storage.StringValue("System Administrator")
	profile.Records["00e000000000002"] = profileRow
	org.Objects["Profile"] = profile
	account := org.Objects["Account"]
	for i := 0; i < 30; i++ {
		name := fmt.Sprintf("Field_%02d__c", i)
		account.Definition.Fields[name] = storage.Field{APIName: name, Type: storage.FieldString, DisplayType: "STRING", Length: 255}
	}
	org.Objects["Account"] = account
	perms := org.Objects["FieldPermissions"]
	for i := 0; i < 1000; i++ {
		id := storage.ID(fmt.Sprintf("0FP%012d", i))
		perms.Records[id] = storage.Record{ID: id, Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(storage.ID(fmt.Sprintf("0PS%012d", i))), "SObjectType": storage.StringValue("Account"), "Field": storage.StringValue(fmt.Sprintf("Account.Field_%02d__c", i%30)), "PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true)}}
	}
	org.Objects["FieldPermissions"] = perms
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.currentMethod.APIVersion = "67.0"
	user := Object("User")
	user.Fields["Id"] = String("005000000000998")
	user.Fields["ProfileId"] = String("00e000000000002")
	machine.testContext.CurrentUser = user
	record := Object("Account")
	setExplicitSObjectField(&record, "Name", String("Inventory"))
	setExplicitSObjectField(&record, "Field_00__c", String("v"))
	userMode := accessLevelValue("USER_MODE")
	if err := machine.enforceUserModeDMLAccessDeferring("insert", record, userMode, true); err != nil {
		b.Fatalf("expected allowed record: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := machine.enforceUserModeDMLAccessDeferring("insert", record, userMode, true); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkAccountFieldAccessDenial measures the new single-field denial separately.
func BenchmarkAccountFieldAccessDenial(b *testing.B) {
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "PermissionSetAssignment")
	profile := org.Objects["Profile"]
	profileRow := profile.Records["00e000000000002"]
	profileRow.Fields["Name"] = storage.StringValue("System Administrator")
	profile.Records["00e000000000002"] = profileRow
	org.Objects["Profile"] = profile
	account := org.Objects["Account"]
	for i := 0; i < 30; i++ {
		name := fmt.Sprintf("Field_%02d__c", i)
		account.Definition.Fields[name] = storage.Field{APIName: name, Type: storage.FieldString, DisplayType: "STRING", Length: 255}
	}
	org.Objects["Account"] = account
	perms := org.Objects["FieldPermissions"]
	for i := 0; i < 1000; i++ {
		id := storage.ID(fmt.Sprintf("0FP%012d", i))
		perms.Records[id] = storage.Record{ID: id, Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(storage.ID(fmt.Sprintf("0PS%012d", i))), "SObjectType": storage.StringValue("Account"), "Field": storage.StringValue(fmt.Sprintf("Account.Field_%02d__c", i%30)), "PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true)}}
	}
	org.Objects["FieldPermissions"] = perms
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.currentMethod.APIVersion = "67.0"
	user := Object("User")
	user.Fields["Id"] = String("005000000000998")
	user.Fields["ProfileId"] = String("00e000000000002")
	machine.testContext.CurrentUser = user
	record := Object("Account")
	setExplicitSObjectField(&record, "Name", String("Inventory"))
	for i := 0; i < 1; i++ {
		setExplicitSObjectField(&record, fmt.Sprintf("Field_%02d__c", i), String("v"))
	}
	if err := machine.enforceUserModeDMLAccessDeferring("insert", record, Null, false); err == nil {
		b.Fatal("expected denied record")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := machine.enforceUserModeDMLAccessDeferring("insert", record, Null, false); err == nil {
			b.Fatal("expected denied record")
		}
	}
}
