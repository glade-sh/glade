package storage

import "testing"

// Salesforce has exactly one Organization row, whose Id is the org ID. A
// fixture that sets the org ID moves the seeded row instead of adding a second.
func TestApplyFixtureOrgIDKeepsSingleOrganizationRecord(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	if got := len(org.Objects["Organization"].Records); got != 1 {
		t.Fatalf("seeded Organization records = %d, want 1", got)
	}
	fixture := Fixture{Version: FixtureVersion, Org: FixtureOrg{OrgID: "00D000000000001EAA"}}
	if err := ApplyFixture(&org, fixture); err != nil {
		t.Fatal(err)
	}
	EnsureDeterministicPlatformData(&org)
	records := org.Objects["Organization"].Records
	if len(records) != 1 {
		ids := []ID{}
		for id := range records {
			ids = append(ids, id)
		}
		t.Fatalf("Organization records = %v, want only 00D000000000001EAA", ids)
	}
	record, ok := records["00D000000000001EAA"]
	if !ok {
		t.Fatalf("Organization record missing for new org ID: %v", records)
	}
	if name := record.Fields["Name"]; name.String == "" {
		t.Fatalf("Organization Name lost: %#v", record.Fields)
	}
}
