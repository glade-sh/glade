package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Uncaptured queue-owned and reader-created Leads retain the prior public model.
func TestPartnerLeadExternalDefaultUncapturedOwners(t *testing.T) {
	for _, tc := range []struct {
		name           string
		owner, creator storage.ID
	}{
		{"queue", "00G000000000001", "005000000000001"},
		{"reader-created", "005000000000001", "005000000000002"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.APIVersion = "67.0"
			storage.EnsureStandardObject(&org, "Lead")
			storage.EnsureStandardObject(&org, "Profile")
			lead := org.Objects["Lead"]
			lead.Definition.SharingModel = "ReadWrite"
			lead.Records = map[storage.ID]storage.Record{"00Q000000000001": {ID: "00Q000000000001", Object: "Lead", System: storage.SystemFields{OwnerID: tc.owner, CreatedByID: tc.creator}}}
			org.Objects["Lead"] = lead
			profiles := org.Objects["Profile"]
			profiles.Records = map[storage.ID]storage.Record{"00e000000000002": {ID: "00e000000000002", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Partner Community User")}}}
			org.Objects["Profile"] = profiles
			machine := New(nil)
			machine.SetOrg(&org)
			machine.EnableTestContext()
			machine.testContext.CurrentUser = stripInaccessibleTestUser()
			machine.testContext.RunAsDepth = 1
			machine.currentMethod = Method{APIVersion: "67.0"}
			if !machine.userModeRecordVisible("Lead", lead.Records["00Q000000000001"], machine.currentUserID()) {
				t.Fatal("uncaptured owner shape lost record visibility")
			}
		})
	}
}

// Self and relationship projections have no matching Partner capture.
func TestPartnerLeadExternalDefaultUncapturedProjection(t *testing.T) {
	org := storage.NewOrgState()
	org.APIVersion = "67.0"
	storage.EnsureStandardObject(&org, "Lead")
	storage.EnsureStandardObject(&org, "Profile")
	lead := org.Objects["Lead"]
	lead.Definition.SharingModel = "ReadWrite"
	record := storage.Record{ID: "00Q000000000001", Object: "Lead", System: storage.SystemFields{OwnerID: "005000000000001", CreatedByID: "005000000000001"}}
	lead.Records = map[storage.ID]storage.Record{record.ID: record}
	org.Objects["Lead"] = lead
	profiles := org.Objects["Profile"]
	profiles.Records = map[storage.ID]storage.Record{"00e000000000002": {ID: "00e000000000002", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Partner Community User")}}}
	org.Objects["Profile"] = profiles
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.testContext.CurrentUser = stripInaccessibleTestUser()
	machine.testContext.RunAsDepth = 1
	machine.currentMethod = Method{APIVersion: "67.0"}
	for _, tc := range []struct {
		name   string
		access storage.Record
	}{{"self", machine.userRecordAccessDirect("Lead", record, false)}, {"relationship", machine.userRecordAccessForRecord("Lead", record)}} {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.access.Fields["HasReadAccess"].Boolean || !tc.access.Fields["HasEditAccess"].Boolean {
				t.Fatalf("uncaptured projection changed: %v", tc.access.Fields)
			}
		})
	}
}
