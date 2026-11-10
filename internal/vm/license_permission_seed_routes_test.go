package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Local compatibility controls: before the seed correction both keys used the
// display name Chatter External and followed the same object and field fallback.
// These assertions preserve that behavior; they are not new native observations.
func TestLegacyExternalLicenseObjectAndFieldFallback(t *testing.T) {
	for _, key := range []string{"CSN_External_User", "PID_Customer_Community_Login"} {
		t.Run(key, func(t *testing.T) {
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
			if !machine.profileHasLicenseKey(string(p.ID), key) || machine.profileHasLicense(string(p.ID), "Chatter External") {
				t.Fatal("license lookup did not distinguish key and name")
			}
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
			row.Fields["LicenseDefinitionKey"] = storage.StringValue("local-unrelated-key")
			licenses.Records[row.ID] = row
			if !machine.currentUserObjectPermission("Contact", "isAccessible") || !machine.currentUserFieldPermission("Contact", "Description", "isAccessible") {
				t.Fatal("unrelated license fallback changed")
			}
		})
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
