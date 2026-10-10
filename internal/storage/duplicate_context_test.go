package storage

import "testing"

func TestDefaultContactDuplicateRuleContext(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	names := map[string]bool{}
	for _, row := range org.Objects["DuplicateRule"].Records {
		if row.Fields["SobjectType"].String == "Contact" {
			names[row.Fields["DeveloperName"].String] = true
		}
	}
	for _, name := range []string{"Standard_Contact_Duplicate_Rule", "Standard_Rule_for_Contacts_with_Duplicate_Leads"} {
		if !names[name] {
			t.Errorf("missing standard Contact rule %s", name)
		}
	}
	EnsureDeterministicPlatformData(&org)
	if len(org.Objects["DuplicateRule"].Records) != 2 {
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
