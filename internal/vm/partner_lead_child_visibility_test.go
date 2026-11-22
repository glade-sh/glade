package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func partnerLeadChildVisibilityMachine(t testing.TB) (*VM, ir.Program) {
	t.Helper()
	org := storage.NewOrgState()
	org.APIVersion = "67.0"
	org.Features = []string{project.PlainDeveloperObjectShape}
	for _, name := range []string{"Case", "Lead", "LeadShare", "User", "Profile", "UserLicense", "FieldPermissions"} {
		storage.EnsureStandardObject(&org, name)
	}
	profiles := org.Objects["Profile"]
	profiles.PutRecord("00e000000000001", storage.Record{ID: "00e000000000001", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("System Administrator")}})
	profiles.PutRecord("00e000000000002", storage.Record{ID: "00e000000000002", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Partner Community User"), "UserLicenseId": storage.IDValue("100000000000001")}})
	profiles.PutRecord("00e000000000003", storage.Record{ID: "00e000000000003", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Standard User")}})
	org.Objects["Profile"] = profiles
	licenses := org.Objects["UserLicense"]
	licenses.PutRecord("100000000000001", storage.Record{ID: "100000000000001", Object: "UserLicense", Fields: map[string]storage.Value{"LicenseDefinitionKey": storage.StringValue("PID_Partner_Community")}})
	org.Objects["UserLicense"] = licenses
	users := org.Objects["User"]
	users.PutRecord("005000000000001", storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000001")}})
	users.PutRecord("005000000000003", storage.Record{ID: "005000000000003", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000003")}})
	org.Objects["User"] = users
	addPartnerLeadReadPermission(&org)
	permissions := org.Objects["ObjectPermissions"]
	permissions.PutRecord("110000000000002", storage.Record{ID: "110000000000002", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("00e000000000002"), "SObjectType": storage.StringValue("Case"), "PermissionsRead": storage.BooleanValue(true)}})
	org.Objects["ObjectPermissions"] = permissions
	fields := org.Objects["FieldPermissions"]
	fields.PutRecord("0FP000000000001", storage.Record{ID: "0FP000000000001", Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("00e000000000002"), "SObjectType": storage.StringValue("Lead"), "Field": storage.StringValue("Lead.CaseLink__c"), "PermissionsRead": storage.BooleanValue(true)}})
	fields.PutRecord("0FP000000000002", storage.Record{ID: "0FP000000000002", Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("00e000000000002"), "SObjectType": storage.StringValue("Lead"), "Field": storage.StringValue("Lead.LastName"), "PermissionsRead": storage.BooleanValue(true)}})
	org.Objects["FieldPermissions"] = fields
	caseID := storage.ID("500000000000001")
	cases := org.Objects["Case"]
	cases.Definition.SharingModel = "ReadWrite"
	cases.PutRecord(caseID, storage.Record{ID: caseID, Object: "Case", System: storage.SystemFields{OwnerID: "005000000000002", CreatedByID: "005000000000002"}})
	org.Objects["Case"] = cases
	leads := org.Objects["Lead"]
	leads.Definition.SharingModel = "ReadWriteTransfer"
	leads.Definition.Fields["CaseLink__c"] = storage.Field{APIName: "CaseLink__c", Type: storage.FieldReference, ReferenceTo: []string{"Case"}, RelationshipName: "CaseLink__r", ChildRelationshipName: "SyntheticLeads__r"}
	leads.Definition.Relations = []storage.Relationship{{Field: "CaseLink__c", ParentObjects: []string{"Case"}, ParentRelationship: "CaseLink__r", ChildRelationship: "SyntheticLeads__r"}}
	shares := org.Objects["LeadShare"]
	for i := 0; i < 200; i++ {
		id := storage.ID(fmt.Sprintf("00Q%012d", i+1))
		leads.PutRecord(id, storage.Record{ID: id, Object: "Lead", System: storage.SystemFields{OwnerID: "005000000000001", CreatedByID: "005000000000003"}, Fields: map[string]storage.Value{"CaseLink__c": storage.IDValue(caseID), "LastName": storage.StringValue("Neighbor")}})
		sid := storage.ID(fmt.Sprintf("00T%012d", i+1))
		shares.PutRecord(sid, storage.Record{ID: sid, Object: "LeadShare", Fields: map[string]storage.Value{"LeadId": storage.IDValue(id), "RowCause": storage.StringValue("Owner")}})
	}
	org.Objects["Lead"], org.Objects["LeadShare"] = leads, shares
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.testContext.CurrentUser = stripInaccessibleTestUser()
	machine.testContext.CurrentUser.Fields["UserType"] = String("PowerPartner")
	machine.testContext.RunAsDepth = 1
	program, err := CompileAnonymousWithOptions("List<Case> rows=[SELECT (SELECT Id FROM SyntheticLeads__r) FROM Case WITH USER_MODE]; System.debug(rows[0].SyntheticLeads__r.size());", CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	return machine, program
}

func TestPartnerLeadSuggestionsPublicNeighbor(t *testing.T) {
	machine, _ := partnerLeadChildVisibilityMachine(t)
	option := Object("Search.SuggestionOption")
	option.Fields["limit"] = Int(200)
	result, err := machine.searchSuggestionRows("Neighbor", "Lead", option, accessLevelValue("USER_MODE"))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.List) != 200 {
		t.Fatalf("visible suggestions = %d, want 200", len(result.List))
	}
}

func TestPartnerLeadChildVisibilityPublicNeighbor(t *testing.T) {
	machine, program := partnerLeadChildVisibilityMachine(t)
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Debug) != 1 || result.Debug[0] != "200" {
		t.Fatalf("visible child records = %v, want 200", result.Debug)
	}
}

func BenchmarkPartnerLeadChildVisibilityOwnerShares(b *testing.B) {
	machine, program := partnerLeadChildVisibilityMachine(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := machine.Execute(program)
		if err != nil {
			b.Fatal(err)
		}
		if len(result.Debug) != 1 || result.Debug[0] != "200" {
			b.Fatalf("visible child records = %v, want 200", result.Debug)
		}
	}
}

func partnerLeadRelationshipProjection(t testing.TB) (*VM, ir.Program) {
	t.Helper()
	machine, _ := partnerLeadChildVisibilityMachine(t)
	source := `List<Lead> rows=[SELECT Id, UserRecordAccess.HasReadAccess FROM Lead WITH USER_MODE]; Integer visible=0; for (Lead row : rows) { if (row.UserRecordAccess.HasReadAccess) visible++; } System.debug(visible);`
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	return machine, program
}

func TestPartnerLeadRelationshipProjectionPublicNeighbor(t *testing.T) {
	machine, program := partnerLeadRelationshipProjection(t)
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Debug) != 1 || result.Debug[0] != "200" {
		t.Fatalf("readable projected records = %v, want 200", result.Debug)
	}
}

func BenchmarkPartnerLeadRelationshipProjectionOwnerShares(b *testing.B) {
	machine, program := partnerLeadRelationshipProjection(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := machine.Execute(program)
		if err != nil {
			b.Fatal(err)
		}
		if len(result.Debug) != 1 || result.Debug[0] != "200" {
			b.Fatalf("readable projected records = %v, want 200", result.Debug)
		}
	}
}

func TestPartnerLeadControlledChildInheritsPublicRead(t *testing.T) {
	machine, _ := partnerLeadChildVisibilityMachine(t)
	machine.currentMethod = Method{APIVersion: "67.0"}
	child := storage.Record{
		ID: "a00000000000001", Object: "Detail__c",
		System: storage.SystemFields{OwnerID: "005000000000001"},
		Fields: map[string]storage.Value{"Master__c": storage.IDValue("00Q000000000001")},
	}
	machine.Org.Objects[child.Object] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: child.Object, SharingModel: "ControlledByParent",
			Fields: map[string]storage.Field{"Master__c": {APIName: "Master__c", Type: storage.FieldReference, MasterDetail: true, ReferenceTo: []string{"Lead"}}}},
		Records: map[storage.ID]storage.Record{child.ID: child},
	}
	user := machine.currentUserID()
	parentRead := machine.controlledParentHasSupportedReadSharing(child.Object, child, user)
	childRead := machine.userModeRecordVisible(child.Object, child, user)
	if !parentRead || !childRead {
		t.Fatalf("inherited public read = %t, child visible = %t", parentRead, childRead)
	}
}
