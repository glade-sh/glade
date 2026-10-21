package storage

import "strings"

// Fresh Developer org observations expose this license and its standard profile
// independently of Experience Cloud site metadata. Seed their queryable identity;
// provisioning users and granting permissions remain separate operations.
func ensureDefaultCommunityLicense(org *OrgState) {
	licenseID := putDefaultSetupRecord(org, "UserLicense", "LicenseDefinitionKey", 6, map[string]Value{
		"Name": StringValue("Customer Community"), "LicenseDefinitionKey": StringValue("PID_Customer_Community"),
		"Status": StringValue("Active"), "TotalLicenses": IntegerValue(5),
	})
	putDefaultSetupRecord(org, "Profile", "Name", 12, map[string]Value{
		"Name": StringValue("Customer Community User"), "UserType": StringValue("CspLitePortal"), "UserLicenseId": IDValue(licenseID),
	})
}

// Developer orgs expose these entitlement records without an installed package.
// Their presence supplies setup query data, not package classes or execution.
func ensureDefaultPackageLicenses(org *OrgState) {
	for _, seed := range []struct{ name, label string }{
		{"SalesforceCPQ_CPQStandardPerm", "Salesforce CPQ License"},
		{"SalesforceCPQ_CPQAAPerm", "Salesforce CPQ AA License"},
	} {
		putDefaultSetupRecord(org, "PermissionSetLicense", "DeveloperName", 1, map[string]Value{
			"DeveloperName": StringValue(seed.name), "MasterLabel": StringValue(seed.label),
			"Status": StringValue("Active"), "TotalLicenses": IntegerValue(2),
		})
	}
}

// Imported setup records take precedence over defaults, including their identity,
// status and capacity. Allocate only absent natural keys and never reuse an ID
// occupied by an unrelated record.
func putDefaultSetupRecord(org *OrgState, objectName, keyField string, firstSequence uint64, fields map[string]Value) ID {
	EnsureStandardObject(org, objectName)
	state := org.Objects[objectName]
	for id, record := range state.Records {
		if strings.EqualFold(record.Fields[keyField].String, fields[keyField].String) {
			putSeedRecord(org, objectName, Record{ID: id, Object: objectName, Fields: fields})
			return id
		}
	}
	if org.IDSequences == nil {
		org.IDSequences = make(map[string]uint64)
	}
	for sequence := firstSequence; ; sequence++ {
		id := ID(state.Definition.KeyPrefix + leftPadBase36(sequence, 12))
		if _, _, used := LookupRecordByID(state.Records, id); used {
			continue
		}
		putSeedRecord(org, objectName, Record{ID: id, Object: objectName, Fields: fields})
		if org.IDSequences[objectName] < sequence {
			org.IDSequences[objectName] = sequence
		}
		return id
	}
}
