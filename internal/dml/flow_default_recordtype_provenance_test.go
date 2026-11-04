package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native Apex rows in testdata/conformance/person_account_followups/review2/org.tsv:
// 62/67 database_null_no_type => REQUIRED_FIELD_MISSING:LastName;
// database_null_business_type => INVALID_FIELD_FOR_INSERT_UPDATE:LastName.
func TestDirectInsertFlowDefaultRecordTypeProvenance(t *testing.T) {
	for _, businessID := range []storage.ID{"012000000000101", "012000000000102"} {
		for _, explicit := range []bool{false, true} {
			name := string(businessID)
			if explicit {
				name += "/explicit"
			} else {
				name += "/default"
			}
			t.Run(name, func(t *testing.T) {
				org := testOrg()
				storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
				storage.EnsureDeterministicPlatformData(&org)
				account := org.Objects["Account"]
				account.Definition.RecordTypes = []storage.RecordTypeInfo{
					{ID: businessID, DeveloperName: "Business", Active: true, Available: true, Default: true},
					{ID: "012000000000103", DeveloperName: "Person", Active: true, Available: true, PersonAccount: true},
				}
				org.Objects["Account"] = account
				fields := map[string]storage.Value{}
				if explicit {
					fields["RecordTypeId"] = storage.IDValue(businessID)
				}
				engine := NewEngine(&org)
				results := engine.Insert([]storage.Record{{Object: "Account", Fields: fields, ExplicitNulls: map[string]bool{"LastName": true}}})
				want := "REQUIRED_FIELD_MISSING"
				if explicit {
					want = "INVALID_FIELD_FOR_INSERT_UPDATE"
				}
				if len(results) != 1 || results[0].Success || results[0].StatusCode != want {
					t.Fatalf("insert = %#v, want %s", results, want)
				}
			})
		}
	}
}

// Preserve the base direct-insert error when a Flow leaves the default unchanged.
// The captured no-Flow controls are in person_account_followups/review2/org.tsv:2-3,86-87.
func TestDirectInsertFlowDefaultProvenanceBoundary(t *testing.T) {
	for _, tc := range []struct {
		name                string
		active              bool
		trigger, mode, want string
		criteria            []storage.WorkflowCriteriaItem
	}{
		{"active-before-noop", true, "RecordBeforeSave", "Create", "REQUIRED_FIELD_MISSING", nil},
		{"active-before-unmatched", true, "RecordBeforeSave", "Create", "REQUIRED_FIELD_MISSING", []storage.WorkflowCriteriaItem{{Field: "Name", Operation: "equals", Value: "not-present"}}},
		{"active-before-update-only", true, "RecordBeforeSave", "Update", "REQUIRED_FIELD_MISSING", nil},
		{"inactive-before", false, "RecordBeforeSave", "Create", "REQUIRED_FIELD_MISSING", nil},
		{"active-after", true, "RecordAfterSave", "Create", "REQUIRED_FIELD_MISSING", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			storage.EnsureDeterministicPlatformData(&org)
			account := org.Objects["Account"]
			account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000101", DeveloperName: "Business", Active: true, Available: true, Default: true}, {ID: "012000000000103", DeveloperName: "Person", Active: true, Available: true, PersonAccount: true}}
			account.Definition.FlowRules = []storage.FlowRule{{Name: "Boundary", Active: tc.active, TriggerType: tc.trigger, RecordTriggerType: tc.mode, Criteria: tc.criteria}}
			org.Objects["Account"] = account
			engine := NewEngine(&org)
			got := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{}, ExplicitNulls: map[string]bool{"LastName": true}}})
			if len(got) != 1 || got[0].Success || got[0].StatusCode != tc.want {
				t.Fatalf("insert=%#v, want %s", got, tc.want)
			}
		})
	}
}
