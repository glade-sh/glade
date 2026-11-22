package vm

import (
	"maps"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPartnerLeadShareScanTracksMutations(t *testing.T) {
	org := storage.NewOrgState()
	org.APIVersion = "67.0"
	org.Features = []string{project.PlainDeveloperObjectShape}
	for _, object := range []string{"Lead", "LeadShare", "User", "Profile", "UserLicense"} {
		storage.EnsureStandardObject(&org, object)
	}
	lead := org.Objects["Lead"]
	lead.Definition.SharingModel = "ReadWriteTransfer"
	lead.PutRecord("00Q000000000001", storage.Record{ID: "00Q000000000001", Object: "Lead", System: storage.SystemFields{OwnerID: "005000000000001", CreatedByID: "005000000000001"}})
	org.Objects["Lead"] = lead
	profiles := org.Objects["Profile"]
	profiles.PutRecord("00e000000000001", storage.Record{ID: "00e000000000001", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("System Administrator")}})
	profiles.PutRecord("00e000000000002", storage.Record{ID: "00e000000000002", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Partner Community User"), "UserLicenseId": storage.IDValue("100000000000001")}})
	org.Objects["Profile"] = profiles
	users := org.Objects["User"]
	users.PutRecord("005000000000001", storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000001")}})
	org.Objects["User"] = users
	licenses := org.Objects["UserLicense"]
	licenses.PutRecord("100000000000001", storage.Record{ID: "100000000000001", Object: "UserLicense", Fields: map[string]storage.Value{"LicenseDefinitionKey": storage.StringValue("PID_Partner_Community")}})
	org.Objects["UserLicense"] = licenses
	addPartnerLeadReadPermission(&org)
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	machine.testContext.CurrentUser = stripInaccessibleTestUser()
	machine.testContext.CurrentUser.Fields["UserType"] = String("PowerPartner")
	machine.testContext.RunAsDepth = 1
	machine.currentMethod = Method{APIVersion: "67.0"}
	check := func(want bool) {
		t.Helper()
		defer machine.beginPartnerLeadShareScan()()
		if got := machine.partnerLeadPrivateExternalDefault("Lead"); got != want {
			t.Fatalf("share shape: got %v want %v", got, want)
		}
	}
	check(true)
	endPrincipalScan := machine.beginPartnerLeadShareScan()
	if !machine.partnerLeadPrivateExternalDefault("Lead") {
		t.Fatal("captured principal did not match")
	}
	machine.testContext.CurrentUser.Fields["UserType"] = String("Standard")
	if machine.partnerLeadPrivateExternalDefault("Lead") {
		t.Fatal("changed principal matched inside the same pass")
	}
	machine.testContext.CurrentUser.Fields["UserType"] = String("PowerPartner")
	if !machine.partnerLeadPrivateExternalDefault("Lead") {
		t.Fatal("restored principal did not match")
	}
	machine.testContext.CurrentUser.Fields["ProfileId"] = String("00e000000000001")
	if machine.partnerLeadPrivateExternalDefault("Lead") {
		t.Fatal("changed profile matched inside the same pass")
	}
	machine.testContext.CurrentUser.Fields["ProfileId"] = String("00e000000000002")
	machine.testContext.CurrentUser.Fields["Id"] = String("005000000000001")
	if machine.partnerLeadPrivateExternalDefault("Lead") {
		t.Fatal("changed user matched inside the same pass")
	}
	machine.testContext.CurrentUser.Fields["Id"] = String("005000000000002")
	endPrincipalScan()
	profile := profiles.Records["00e000000000002"]
	unnamedProfile := profile
	unnamedProfile.Fields = maps.Clone(profile.Fields)
	delete(unnamedProfile.Fields, "Name")
	profiles.PutRecord(unnamedProfile.ID, unnamedProfile)
	org.Objects["Profile"] = profiles
	projectedProfile := Object("Profile")
	projectedProfile.Fields["Name"] = String("Partner Community User")
	machine.testContext.CurrentUser.Fields["Profile"] = projectedProfile
	endProfileScan := machine.beginPartnerLeadShareScan()
	if !machine.partnerLeadPrivateExternalDefault("Lead") {
		t.Fatal("projected profile did not match")
	}
	projectedProfile.Fields["Name"] = String("Standard User")
	if machine.partnerLeadPrivateExternalDefault("Lead") {
		t.Fatal("changed projected profile matched inside the same pass")
	}
	endProfileScan()
	delete(machine.testContext.CurrentUser.Fields, "Profile")
	profiles.PutRecord(profile.ID, profile)
	org.Objects["Profile"] = profiles
	lead.Definition.SharingModel = "ReadWrite"
	org.Objects["Lead"] = lead
	endActorScan := machine.beginPartnerLeadShareScan()
	if !machine.partnerLeadPrivateExternalDefaultForNamedAccess("Lead", "") || machine.partnerLeadPrivateExternalDefaultForNamedAccess("Lead", "005000000000001") {
		t.Fatal("actor change reused the wrong sharing policy")
	}
	endActorScan()
	lead.Definition.SharingModel = "ReadWriteTransfer"
	org.Objects["Lead"] = lead
	permissions := org.Objects["ObjectPermissions"]
	grant := permissions.Records["110000000000001"]
	originalGrant := grant
	grant.Fields = maps.Clone(grant.Fields)
	grant.Fields["PermissionsEdit"] = storage.BooleanValue(true)
	permissions.PutRecord(grant.ID, grant)
	org.Objects["ObjectPermissions"] = permissions
	check(false)
	permissions.PutRecord(originalGrant.ID, originalGrant)
	org.Objects["ObjectPermissions"] = permissions
	check(true)
	machine.currentMethod.APIVersion = "68.0"
	check(false)
	machine.currentMethod.APIVersion = "67.0"
	check(true)
	license := licenses.Records["100000000000001"]
	wrongLicense := license
	wrongLicense.Fields = maps.Clone(license.Fields)
	wrongLicense.Fields["LicenseDefinitionKey"] = storage.StringValue("unrelated")
	licenses.PutRecord(wrongLicense.ID, wrongLicense)
	org.Objects["UserLicense"] = licenses
	check(false)
	licenses.PutRecord(license.ID, license)
	org.Objects["UserLicense"] = licenses
	check(true)
	share := storage.Record{ID: "00T000000000001", Object: "LeadShare", Fields: map[string]storage.Value{"LeadId": storage.IDValue("00Q000000000001"), "RowCause": storage.StringValue("Owner"), "UserOrGroupId": storage.IDValue("005000000000002"), "LeadAccessLevel": storage.StringValue("Read")}}
	put := func() {
		state := org.Objects["LeadShare"]
		state.PutRecord(share.ID, share)
		org.Objects["LeadShare"] = state
	}
	put()
	endScan := machine.beginPartnerLeadShareScan()
	if !machine.partnerLeadPrivateExternalDefault("Lead") {
		t.Fatal("owner share changed the captured shape")
	}
	if machine.partnerLeadShares == nil || !machine.partnerLeadShares.ready {
		t.Fatal("share pass did not exercise the index")
	}
	endScan()
	if machine.partnerLeadShares != nil {
		t.Fatal("share index survived its record pass")
	}
	share.Fields = map[string]storage.Value{"LeadId": storage.IDValue("00Q000000000001"), "RowCause": storage.StringValue("Manual"), "UserOrGroupId": storage.IDValue("005000000000002"), "LeadAccessLevel": storage.StringValue("Read")}
	put()
	check(true)
	if !machine.currentUserHasDirectLeadManualRead(lead.Records["00Q000000000001"], "005000000000002") {
		t.Fatal("manual share was not indexed")
	}
	changedLead := lead.Records["00Q000000000001"]
	changedLead.System.CreatedByID = "005000000000003"
	lead.PutRecord(changedLead.ID, changedLead)
	org.Objects["Lead"] = lead
	check(false)
	changedLead.System.CreatedByID = changedLead.System.OwnerID
	lead.PutRecord(changedLead.ID, changedLead)
	org.Objects["Lead"] = lead
	check(true)
	for _, mutation := range []struct {
		field string
		value storage.Value
	}{
		{"LeadId", storage.IDValue("00Q000000000002")},
		{"LeadAccessLevel", storage.StringValue("Edit")},
	} {
		original := share.Fields
		share.Fields = maps.Clone(original)
		share.Fields[mutation.field] = mutation.value
		put()
		check(false)
		share.Fields = original
		put()
		check(true)
	}
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
	state = storage.ObjectState{Definition: state.Definition, Records: map[storage.ID]storage.Record{share.ID: share}}
	org.Objects["LeadShare"] = state
	check(false)
	delete(state.Records, share.ID)
	check(true)
}

func TestPartnerLeadShareScanTracksAliasedRecordMutation(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "LeadShare")
	shares := org.Objects["LeadShare"]
	id := storage.ID("00T000000000001")
	shares.PutRecord(id, storage.Record{ID: id, Object: "LeadShare", Fields: map[string]storage.Value{"RowCause": storage.StringValue("Owner")}})
	org.Objects["LeadShare"] = shares
	machine := New(nil)
	machine.SetOrg(&org)
	endScan := machine.beginPartnerLeadShareScan()
	if got := machine.partnerLeadNonOwnerShares(); len(got) != 0 {
		t.Fatalf("initial non-owner shares = %d", len(got))
	}
	endScan()
	before, ok := shares.RecordsToken()
	if !ok {
		t.Fatal("fixture needs a versioned share map")
	}
	clone := storage.ObjectState{}
	clone.SetRecords(shares.Records)
	row := clone.Records[id]
	row.Fields["RowCause"] = storage.StringValue("Manual")
	clone.PutRecord(id, row)
	after, _ := shares.RecordsToken()
	if before != after {
		t.Fatal("mutation unexpectedly advanced original token")
	}
	endScan = machine.beginPartnerLeadShareScan()
	defer endScan()
	if got := machine.partnerLeadNonOwnerShares(); len(got) != 1 {
		t.Fatalf("aliased non-owner shares = %d, want 1", len(got))
	}
}

func TestPartnerLeadNestedShareScanReusesIndex(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "LeadShare")
	machine := New(nil)
	machine.SetOrg(&org)
	endOuter := machine.beginPartnerLeadShareScan()
	defer endOuter()
	machine.partnerLeadNonOwnerShares()
	outer := machine.partnerLeadShares
	endInner := machine.beginPartnerLeadShareScan()
	if machine.partnerLeadShares != outer {
		t.Fatal("nested read-only scan discarded the enclosing index")
	}
	endInner()
	if machine.partnerLeadShares != outer {
		t.Fatal("nested scan did not preserve the enclosing index")
	}
}

func TestPartnerLeadUncapturedCreatorAvoidsShareScan(t *testing.T) {
	machine, _ := partnerLeadChildVisibilityMachine(t)
	machine.currentMethod = Method{APIVersion: "67.0"}
	end := machine.beginPartnerLeadShareScan()
	defer end()
	for _, record := range machine.Org.Objects["Lead"].Records {
		if !machine.userModeRecordVisible("Lead", record, machine.currentUserID()) {
			t.Fatal("uncaptured creator lost visibility")
		}
		if machine.partnerLeadShares.ready {
			t.Fatal("uncaptured creator scanned share rows")
		}
		break
	}
}
