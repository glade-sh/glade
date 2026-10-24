package storage

import "testing"

func TestDefaultContactDuplicateRuleContext(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	// Native duplicate enforcement ST01: same-object rules are active and the
	// cross-object Contact/Lead rules are inactive.
	names := map[string]bool{}
	for _, row := range org.Objects["DuplicateRule"].Records {
		names[row.Fields["SobjectType"].String+":"+row.Fields["DeveloperName"].String] = row.Fields["IsActive"].Boolean
	}
	for name, active := range map[string]bool{
		"Account:Standard_Account_Duplicate_Rule":                 true,
		"Contact:Standard_Contact_Duplicate_Rule":                 true,
		"Contact:Standard_Rule_for_Contacts_with_Duplicate_Leads": false,
		"Lead:Standard_Lead_Duplicate_Rule":                       true,
		"Lead:Standard_Rule_for_Leads_with_Duplicate_Contacts":    false,
	} {
		if got, ok := names[name]; !ok || got != active {
			t.Errorf("standard rule %s active=%v present=%v; want active=%v", name, got, ok, active)
		}
	}
	matching := map[string]bool{}
	for _, row := range org.Objects["MatchingRule"].Records {
		matching[row.Fields["SobjectType"].String+":"+row.Fields["DeveloperName"].String+":"+row.Fields["RuleStatus"].String] = true
	}
	for _, name := range []string{"Account:Standard_Account_Match_Rule_v1_0:Active", "Contact:Standard_Contact_Match_Rule_v1_1:Active", "Lead:Standard_Lead_Match_Rule_v1_0:Active"} {
		if !matching[name] {
			t.Errorf("missing standard matching rule %s", name)
		}
	}
	// Native MD01: each object's definition carries its default rules.
	for object, want := range map[string]int{"Account": 1, "Contact": 2, "Lead": 2} {
		if got := len(org.Objects[object].Definition.DuplicateRules); got != want {
			t.Errorf("%s default definitions=%d; want %d", object, got, want)
		}
	}
	EnsureDeterministicPlatformData(&org)
	if len(org.Objects["DuplicateRule"].Records) != 5 || len(org.Objects["Contact"].Definition.DuplicateRules) != 2 {
		t.Fatal("default rule context is not idempotent")
	}
}

// Local-only preservation controls, separate from the Salesforce default-context proof.
func TestDefaultDuplicateRulesPreserveSuppliedContext(t *testing.T) {
	for _, populated := range []bool{false, true} {
		t.Run(map[bool]string{false: "importedEmpty", true: "suppliedRows"}[populated], func(t *testing.T) {
			org := NewOrgState()
			EnsureStandardObject(&org, "DuplicateRule")
			if populated {
				state := org.Objects["DuplicateRule"]
				state.Records = map[ID]Record{"0Bm000000000099": {ID: "0Bm000000000099", Object: "DuplicateRule", Fields: map[string]Value{"DeveloperName": StringValue("Supplied"), "IsActive": BooleanValue(false)}}}
				org.Objects["DuplicateRule"] = state
			} else {
				org.OrgID = "00D000000000099"
			}
			EnsureDeterministicPlatformData(&org)
			want := 0
			if populated {
				want = 1
			}
			if populated && org.Objects["DuplicateRule"].Records["0Bm000000000099"].Fields["IsActive"].Boolean {
				t.Fatal("reactivated supplied rule")
			}
			if len(org.Objects["DuplicateRule"].Records) != want {
				t.Fatal("changed supplied duplicate-rule context")
			}
		})
	}
}
