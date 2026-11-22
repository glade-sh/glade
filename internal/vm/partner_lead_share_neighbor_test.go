package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPartnerLeadUncapturedSharesKeepPublicVisibility(t *testing.T) {
	for _, tc := range []struct {
		name, model string
		parents     []storage.ID
	}{
		{"other-owner", "ReadWriteTransfer", []storage.ID{"00Q000000000002"}},
		{"multiple-shares", "ReadWrite", []storage.ID{"00Q000000000003", "00Q000000000004"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.APIVersion = "67.0"
			org.Features = []string{project.PlainDeveloperObjectShape}
			for _, name := range []string{"Lead", "LeadShare", "User", "Profile", "UserLicense"} {
				storage.EnsureStandardObject(&org, name)
			}
			license := org.Objects["UserLicense"]
			license.PutRecord("100000000000001", storage.Record{ID: "100000000000001", Object: "UserLicense", Fields: map[string]storage.Value{"LicenseDefinitionKey": storage.StringValue("PID_Partner_Community")}})
			org.Objects["UserLicense"] = license
			profiles := org.Objects["Profile"]
			profiles.PutRecord("00e000000000001", storage.Record{ID: "00e000000000001", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("System Administrator")}})
			profiles.PutRecord("00e000000000002", storage.Record{ID: "00e000000000002", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Partner Community User"), "UserLicenseId": storage.IDValue("100000000000001")}})
			profiles.PutRecord("00e000000000003", storage.Record{ID: "00e000000000003", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Standard User")}})
			org.Objects["Profile"] = profiles
			users := org.Objects["User"]
			users.PutRecord("005000000000001", storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000001")}})
			users.PutRecord("005000000000003", storage.Record{ID: "005000000000003", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000003")}})
			org.Objects["User"] = users
			lead := org.Objects["Lead"]
			lead.Definition.SharingModel = tc.model
			for _, id := range []storage.ID{"00Q000000000001", "00Q000000000002", "00Q000000000003", "00Q000000000004"} {
				owner := storage.ID("005000000000001")
				if id == "00Q000000000002" {
					owner = "005000000000003"
				}
				lead.PutRecord(id, storage.Record{ID: id, Object: "Lead", System: storage.SystemFields{OwnerID: owner, CreatedByID: owner}})
			}
			org.Objects["Lead"] = lead
			shares := org.Objects["LeadShare"]
			for i, parent := range tc.parents {
				id := storage.ID("00T000000000001")
				if i == 1 {
					id = "00T000000000002"
				}
				shares.PutRecord(id, storage.Record{ID: id, Object: "LeadShare", Fields: map[string]storage.Value{"LeadId": storage.IDValue(parent), "RowCause": storage.StringValue("Manual"), "UserOrGroupId": storage.IDValue("005000000000002"), "LeadAccessLevel": storage.StringValue("Read")}})
			}
			org.Objects["LeadShare"] = shares
			machine := New(nil)
			machine.SetOrg(&org)
			machine.EnableTestContext()
			machine.testContext.CurrentUser = stripInaccessibleTestUser()
			machine.testContext.CurrentUser.Fields["UserType"] = String("PowerPartner")
			machine.testContext.RunAsDepth = 1
			machine.currentMethod = Method{APIVersion: "67.0"}
			if !machine.userModeRecordVisible("Lead", lead.Records["00Q000000000001"], machine.currentUserID()) {
				t.Fatal("uncaptured share shape lost public-model visibility")
			}
		})
	}
}
