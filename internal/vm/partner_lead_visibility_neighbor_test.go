package vm

import (
	"fmt"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestPartnerLeadUncapturedCreatorKeepsPublicVisibility(t *testing.T) {
	org := storage.NewOrgState()
	org.APIVersion = "67.0"
	org.Features = []string{project.PlainDeveloperObjectShape}
	storage.EnsureStandardObject(&org, "Lead")
	storage.EnsureStandardObject(&org, "Profile")
	storage.EnsureStandardObject(&org, "User")
	storage.EnsureStandardObject(&org, "UserLicense")
	licenses := org.Objects["UserLicense"]
	licenses.Records = map[storage.ID]storage.Record{"100000000000001": {ID: "100000000000001", Object: "UserLicense", Fields: map[string]storage.Value{"LicenseDefinitionKey": storage.StringValue("PID_Partner_Community")}}}
	org.Objects["UserLicense"] = licenses
	users := org.Objects["User"]
	users.Records = map[storage.ID]storage.Record{"005000000000001": {ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000001")}}, "005000000000003": {ID: "005000000000003", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000001")}}}
	org.Objects["User"] = users
	lead := org.Objects["Lead"]
	lead.Definition.SharingModel = "ReadWrite"
	record := storage.Record{ID: "00Q000000000001", Object: "Lead", System: storage.SystemFields{OwnerID: "005000000000001", CreatedByID: "005000000000003"}}
	lead.Records = map[storage.ID]storage.Record{record.ID: record}
	org.Objects["Lead"] = lead
	if !storage.StandardObjectShapeDeclared(lead.Definition) {
		t.Fatal("fixture lacks declared shape")
	}
	profiles := org.Objects["Profile"]
	profiles.Records = map[storage.ID]storage.Record{"00e000000000002": {ID: "00e000000000002", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Partner Community User"), "UserLicenseId": storage.IDValue("100000000000001")}}}
	profiles.Records["00e000000000001"] = storage.Record{ID: "00e000000000001", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Standard User")}}
	org.Objects["Profile"] = profiles
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.testContext.CurrentUser = stripInaccessibleTestUser()
	machine.testContext.CurrentUser.Fields["UserType"] = String("PowerPartner")
	machine.testContext.RunAsDepth = 1
	machine.currentMethod = Method{APIVersion: "67.0"}
	if !machine.userModeRecordVisible("Lead", record, machine.currentUserID()) {
		t.Fatal("uncaptured Lead with separate creator lost public-model visibility")
	}
}

func BenchmarkPartnerLeadVisibilityOwnerShares(b *testing.B) {
	org := storage.NewOrgState()
	org.APIVersion = "67.0"
	org.Features = []string{project.PlainDeveloperObjectShape}
	for _, object := range []string{"Lead", "LeadShare", "Profile", "UserLicense"} {
		storage.EnsureStandardObject(&org, object)
	}
	lead := org.Objects["Lead"]
	lead.Definition.SharingModel = "ReadWriteTransfer"
	shares := org.Objects["LeadShare"]
	for i := 0; i < 200; i++ {
		id := storage.ID(fmt.Sprintf("00Q%012d", i+1))
		lead.PutRecord(id, storage.Record{ID: id, Object: "Lead", System: storage.SystemFields{OwnerID: "005000000000002", CreatedByID: "005000000000002"}})
		sid := storage.ID(fmt.Sprintf("00T%012d", i+1))
		shares.PutRecord(sid, storage.Record{ID: sid, Fields: map[string]storage.Value{"LeadId": storage.IDValue(id), "RowCause": storage.StringValue("Owner")}})
	}
	org.Objects["Lead"], org.Objects["LeadShare"] = lead, shares
	profiles := org.Objects["Profile"]
	profiles.PutRecord("00e000000000002", storage.Record{ID: "00e000000000002", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Partner Community User"), "UserLicenseId": storage.IDValue("100000000000001")}})
	org.Objects["Profile"] = profiles
	licenses := org.Objects["UserLicense"]
	licenses.PutRecord("100000000000001", storage.Record{ID: "100000000000001", Object: "UserLicense", Fields: map[string]storage.Value{"LicenseDefinitionKey": storage.StringValue("PID_Partner_Community")}})
	org.Objects["UserLicense"] = licenses
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.testContext.CurrentUser = stripInaccessibleTestUser()
	machine.testContext.RunAsDepth = 1
	machine.currentMethod = Method{APIVersion: "67.0"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		endScan := machine.beginPartnerLeadShareScan()
		visible := 0
		for _, record := range lead.Records {
			if machine.userModeRecordVisible("Lead", record, machine.currentUserID()) {
				visible++
			}
		}
		endScan()
		if visible != 200 {
			b.Fatalf("visible = %d, want 200", visible)
		}
	}
}

func TestPartnerLeadUncapturedLicenseKeepsPublicVisibility(t *testing.T) {
	org := storage.NewOrgState()
	org.APIVersion = "67.0"
	org.Features = []string{project.PlainDeveloperObjectShape}
	for _, object := range []string{"Lead", "Profile", "User", "UserLicense"} {
		storage.EnsureStandardObject(&org, object)
	}
	lead := org.Objects["Lead"]
	lead.Definition.SharingModel = "ReadWrite"
	record := storage.Record{ID: "00Q000000000001", Object: "Lead", System: storage.SystemFields{OwnerID: "005000000000001", CreatedByID: "005000000000001"}}
	lead.Records = map[storage.ID]storage.Record{record.ID: record}
	org.Objects["Lead"] = lead
	profiles := org.Objects["Profile"]
	profiles.Records = map[storage.ID]storage.Record{
		"00e000000000001": {ID: "00e000000000001", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("System Administrator")}},
		"00e000000000002": {ID: "00e000000000002", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Partner Community User"), "UserLicenseId": storage.IDValue("100000000000001")}},
	}
	org.Objects["Profile"] = profiles
	licenses := org.Objects["UserLicense"]
	licenses.Records = map[storage.ID]storage.Record{"100000000000001": {ID: "100000000000001", Object: "UserLicense", Fields: map[string]storage.Value{"LicenseDefinitionKey": storage.StringValue("unrelated")}}}
	org.Objects["UserLicense"] = licenses
	users := org.Objects["User"]
	users.Records = map[storage.ID]storage.Record{"005000000000001": {ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000001")}}}
	org.Objects["User"] = users
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.testContext.CurrentUser = stripInaccessibleTestUser()
	machine.testContext.RunAsDepth = 1
	machine.currentMethod = Method{APIVersion: "67.0"}
	if !machine.userModeRecordVisible("Lead", record, machine.currentUserID()) {
		t.Fatal("uncaptured license lost public-model visibility")
	}
}

func TestPartnerLeadUncapturedReaderKeepsPublicVisibility(t *testing.T) {
	org := storage.NewOrgState()
	org.APIVersion = "67.0"
	org.Features = []string{project.PlainDeveloperObjectShape}
	for _, object := range []string{"Lead", "Profile", "User", "UserLicense"} {
		storage.EnsureStandardObject(&org, object)
	}
	lead := org.Objects["Lead"]
	lead.Definition.SharingModel = "ReadWrite"
	record := storage.Record{ID: "00Q000000000001", Object: "Lead", System: storage.SystemFields{OwnerID: "005000000000001", CreatedByID: "005000000000001"}}
	lead.Records = map[storage.ID]storage.Record{record.ID: record}
	org.Objects["Lead"] = lead
	profiles := org.Objects["Profile"]
	profiles.Records = map[storage.ID]storage.Record{
		"00e000000000001": {ID: "00e000000000001", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("System Administrator")}},
		"00e000000000002": {ID: "00e000000000002", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Partner Community User"), "UserLicenseId": storage.IDValue("100000000000001")}},
	}
	org.Objects["Profile"] = profiles
	licenses := org.Objects["UserLicense"]
	licenses.Records = map[storage.ID]storage.Record{"100000000000001": {ID: "100000000000001", Object: "UserLicense", Fields: map[string]storage.Value{"LicenseDefinitionKey": storage.StringValue("PID_Partner_Community")}}}
	org.Objects["UserLicense"] = licenses
	users := org.Objects["User"]
	users.Records = map[storage.ID]storage.Record{"005000000000001": {ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000001")}}}
	org.Objects["User"] = users
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.testContext.CurrentUser = stripInaccessibleTestUser()
	machine.testContext.RunAsDepth = 1
	machine.currentMethod = Method{APIVersion: "67.0"}
	if !machine.userModeRecordVisible("Lead", record, "005000000000003") {
		t.Fatal("uncaptured reader lost public-model visibility")
	}
}
