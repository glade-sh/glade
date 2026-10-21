package storage

import "testing"

func TestDefaultSetupLicensesPreserveImportedRecords(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "UserLicense")
	EnsureStandardObject(&org, "PermissionSetLicense")
	userLicense := Record{ID: "100000000000999", Object: "UserLicense", Fields: map[string]Value{
		"Name": StringValue("Imported customer license"), "LicenseDefinitionKey": StringValue("PID_Customer_Community"),
		"Status": StringValue("Inactive"), "TotalLicenses": IntegerValue(17),
	}}
	packageLicense := Record{ID: "0PL000000000999", Object: "PermissionSetLicense", Fields: map[string]Value{
		"DeveloperName": StringValue("SalesforceCPQ_CPQStandardPerm"), "MasterLabel": StringValue("Imported entitlement"),
		"Status": StringValue("Inactive"), "TotalLicenses": IntegerValue(23),
	}}
	org.Objects["UserLicense"].Records[userLicense.ID] = userLicense
	org.Objects["PermissionSetLicense"].Records[packageLicense.ID] = packageLicense
	for range 2 {
		EnsureDeterministicPlatformData(&org)
	}
	for _, record := range []Record{userLicense, packageLicense} {
		got := org.Objects[record.Object].Records[record.ID]
		for field, want := range record.Fields {
			if !ValuesEqual(got.Fields[field], want) {
				t.Errorf("imported %s.%s=%v; want %v", record.Object, field, got.Fields[field], want)
			}
		}
	}
	for _, key := range []struct{ object, field, value string }{
		{"UserLicense", "LicenseDefinitionKey", "PID_Customer_Community"},
		{"PermissionSetLicense", "DeveloperName", "SalesforceCPQ_CPQStandardPerm"},
		{"PermissionSetLicense", "DeveloperName", "SalesforceCPQ_CPQAAPerm"},
	} {
		count := 0
		for _, record := range org.Objects[key.object].Records {
			if record.Fields[key.field].String == key.value {
				count++
			}
		}
		if count != 1 {
			t.Errorf("%s %s count=%d; want 1", key.object, key.value, count)
		}
	}
	profile, ok := findRecordByStringField(org.Objects["Profile"].Records, "Name", "Customer Community User")
	if !ok || profile.Fields["UserLicenseId"].ID != userLicense.ID {
		t.Errorf("customer profile must link to imported license: %+v", profile)
	}
	ResetNonPlatformData(&org)
	for _, record := range []Record{userLicense, packageLicense} {
		if _, ok := org.Objects[record.Object].Records[record.ID]; !ok {
			t.Errorf("non-platform reset discarded %s", record.Object)
		}
	}
}
