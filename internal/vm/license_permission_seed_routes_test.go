package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Local compatibility controls, not native observations: before the seed
// correction a license named Chatter External denied object and field access.
// The seeded Customer Community Login license (it carried that name) keeps the
// denial; a license matched only by key does not gain it.
func TestLegacyExternalLicenseObjectAndFieldFallback(t *testing.T) {
	for _, tc := range []struct{ name, key, label string }{
		{"Chatter External", "local-key", "name"},
		{"Customer Community Login", "PID_Customer_Community_Login", "seeded pair"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			machine := builtinLicenseAccountActor(t)
			profiles := machine.Org.Objects["Profile"]
			p := profiles.Records[storage.ID("00e000000000091")]
			p.Fields["Name"] = storage.StringValue("Local test profile")
			p.Fields["UserType"] = storage.StringValue("Standard")
			profiles.Records[p.ID] = p
			licenses := machine.Org.Objects["UserLicense"]
			row := licenses.Records[storage.ID("100000000000091")]
			row.Fields["Name"] = storage.StringValue(tc.name)
			row.Fields["LicenseDefinitionKey"] = storage.StringValue(tc.key)
			licenses.Records[row.ID] = row
			for _, method := range []string{"isAccessible", "isCreateable", "isUpdateable"} {
				if machine.currentUserObjectPermission("Contact", method) {
					t.Errorf("legacy external fallback unexpectedly grants Contact %s", method)
				}
				if machine.currentUserFieldPermission("Contact", "Description", method) {
					t.Errorf("legacy external fallback unexpectedly grants Contact.Description %s", method)
				}
			}
			program, err := CompileAnonymous(`
System.assertEquals(false, Contact.SObjectType.getDescribe().isAccessible());
System.assertEquals(false, Contact.Description.getDescribe().isAccessible());
Boolean deniedInsert = false;
try { insert as user new Contact(LastName='Example'); } catch (Exception e) { deniedInsert = true; }
System.assertEquals(true, deniedInsert);
Boolean deniedQuery = false;
try { Database.query('SELECT Id FROM Contact WITH USER_MODE'); } catch (Exception e) { deniedQuery = true; }
System.assertEquals(true, deniedQuery);
`)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatalf("describe and user-mode routes: %v", err)
			}
			row.Fields["Name"] = storage.StringValue("Local test license")
			licenses.Records[row.ID] = row
			if !machine.currentUserObjectPermission("Contact", "isAccessible") || !machine.currentUserFieldPermission("Contact", "Description", "isAccessible") {
				t.Fatal("unrelated license fallback changed")
			}
		})
	}
}

// A license matched only by key (Chatter External's or Community Login's key
// under another name) keeps the pre-change behavior: no denial.
func TestExternalLicenseKeyAloneDoesNotDeny(t *testing.T) {
	for _, key := range []string{"CSN_External_User", "PID_Customer_Community_Login"} {
		machine := builtinLicenseAccountActor(t)
		profiles := machine.Org.Objects["Profile"]
		p := profiles.Records[storage.ID("00e000000000091")]
		p.Fields["Name"] = storage.StringValue("Local test profile")
		p.Fields["UserType"] = storage.StringValue("Standard")
		profiles.Records[p.ID] = p
		licenses := machine.Org.Objects["UserLicense"]
		row := licenses.Records[storage.ID("100000000000091")]
		row.Fields["Name"] = storage.StringValue("Local test license")
		row.Fields["LicenseDefinitionKey"] = storage.StringValue(key)
		licenses.Records[row.ID] = row
		if !machine.currentUserObjectPermission("Contact", "isAccessible") || !machine.currentUserFieldPermission("Contact", "Description", "isAccessible") {
			t.Errorf("key %s alone changed access", key)
		}
	}
}

// The name-based Salesforce license path remains the preexisting local
// unlisted-Account fallback, and field checks continue to honor that result.
func TestSalesforceLicenseObjectAndFieldFallback(t *testing.T) {
	machine := builtinLicenseAccountActor(t)
	profiles := machine.Org.Objects["Profile"]
	p := profiles.Records[storage.ID("00e000000000091")]
	p.Fields["Name"] = storage.StringValue("Local test profile")
	p.Fields["UserType"] = storage.StringValue("Standard")
	profiles.Records[p.ID] = p
	licenses := machine.Org.Objects["UserLicense"]
	row := licenses.Records[storage.ID("100000000000091")]
	row.Fields["Name"] = storage.StringValue("Salesforce")
	row.Fields["LicenseDefinitionKey"] = storage.StringValue("local-control")
	licenses.Records[row.ID] = row
	setID := storage.ID("0PS000000000091")
	machine.Org.Objects["PermissionSet"].Records[setID] = storage.Record{ID: setID, Object: "PermissionSet", Fields: map[string]storage.Value{"ProfileId": storage.IDValue(p.ID), "IsOwnedByProfile": storage.BooleanValue(true), "IsCustom": storage.BooleanValue(true), "Type": storage.StringValue("Profile")}}
	if !machine.profileHasLicense(string(p.ID), "Salesforce") {
		t.Fatal("name-based license lookup failed")
	}
	if machine.currentUserObjectPermission("Account", "isAccessible") {
		t.Error("unlisted Account became readable")
	}
	if machine.currentUserFieldPermission("Account", "Description", "isAccessible") {
		t.Error("field became readable without object access")
	}
}
