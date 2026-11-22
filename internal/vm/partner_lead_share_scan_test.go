package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPartnerLeadShareScanTracksMutations(t *testing.T) {
	org := storage.NewOrgState()
	org.APIVersion = "67.0"
	org.Features = []string{project.PlainDeveloperObjectShape}
	for _, object := range []string{"Lead", "LeadShare", "Profile", "UserLicense"} {
		storage.EnsureStandardObject(&org, object)
	}
	lead := org.Objects["Lead"]
	lead.Definition.SharingModel = "ReadWriteTransfer"
	lead.PutRecord("00Q000000000001", storage.Record{ID: "00Q000000000001", Object: "Lead", System: storage.SystemFields{OwnerID: "005000000000001", CreatedByID: "005000000000001"}})
	org.Objects["Lead"] = lead
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
	check := func(want bool) {
		t.Helper()
		if got := machine.partnerLeadPrivateExternalDefault("Lead"); got != want {
			t.Fatalf("share shape: got %v want %v", got, want)
		}
	}
	check(true)
	share := storage.Record{ID: "00T000000000001", Object: "LeadShare", Fields: map[string]storage.Value{"LeadId": storage.IDValue("00Q000000000001"), "RowCause": storage.StringValue("Owner"), "UserOrGroupId": storage.IDValue("005000000000002"), "LeadAccessLevel": storage.StringValue("Read")}}
	put := func() {
		state := org.Objects["LeadShare"]
		state.PutRecord(share.ID, share)
		org.Objects["LeadShare"] = state
	}
	put()
	check(true)
	share.Fields = map[string]storage.Value{"LeadId": storage.IDValue("00Q000000000001"), "RowCause": storage.StringValue("Manual"), "UserOrGroupId": storage.IDValue("005000000000002"), "LeadAccessLevel": storage.StringValue("Read")}
	put()
	check(true)
	share.Fields = map[string]storage.Value{"LeadId": storage.IDValue("00Q000000000001"), "RowCause": storage.StringValue("Manual"), "UserOrGroupId": storage.IDValue("005000000000003"), "LeadAccessLevel": storage.StringValue("Read")}
	put()
	check(false)
	share.System.IsDeleted = true
	put()
	check(true)
	share.System.IsDeleted = false
	put()
	check(false)
	state := org.Objects["LeadShare"]
	state.DeleteRecord(share.ID)
	org.Objects["LeadShare"] = state
	check(true)
}
