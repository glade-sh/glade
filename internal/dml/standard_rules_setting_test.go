package dml_test

import (
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func seededAccountOrg(t *testing.T) *storage.OrgState {
	t.Helper()
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	rules := org.Objects["Account"].Definition.DuplicateRules
	if len(rules) == 0 || !rules[0].Seeded || !rules[0].Active {
		t.Fatalf("the seeded standard Account rule is missing: %+v", rules)
	}
	return &org
}

func insertTestAccounts(engine dml.Engine) (first, second dml.Result) {
	fields := func(name string) map[string]string {
		return map[string]string{"Name": name, "BillingCity": "Springfield"}
	}
	return insertRow(engine, "Account", fields("Test Account 1")), insertRow(engine, "Account", fields("Test Account 2"))
}

// The standard rules are off by default: numbered test accounts in one city
// insert without a duplicate alert.
func TestStandardDuplicateRulesOffByDefault(t *testing.T) {
	t.Setenv("GLADE_STANDARD_DUPLICATE_RULES", "")
	engine := dml.NewEngine(seededAccountOrg(t))
	engine.Options.EnforceDuplicateRules = true
	first, second := insertTestAccounts(engine)
	if !first.Success || !second.Success {
		t.Fatalf("default off: %+v %+v", first, second)
	}
}

// With duplicateRules.standard on, the standard rule is enforced as natively
// (an Allow rule alerts, and allowSave lets the save through).
//
// NATIVE Test Account 1 finds Test Account 2 in one city:
// internal/apextest/testdata/conformance/duplicate_standard_rules/org.tsv Standard.Shapes.Account.Family
// anonymous 67.0.
func TestStandardDuplicateRulesSettingOn(t *testing.T) {
	for name, enable := range map[string]func(*storage.OrgState){
		"setting": func(o *storage.OrgState) { o.StandardDuplicateRules = true },
		"env":     func(*storage.OrgState) { t.Setenv("GLADE_STANDARD_DUPLICATE_RULES", "on") },
	} {
		t.Run(name, func(t *testing.T) {
			org := seededAccountOrg(t)
			enable(org)
			engine := dml.NewEngine(org)
			engine.Options.EnforceDuplicateRules = true
			first, second := insertTestAccounts(engine)
			if !first.Success || !blocked(second) {
				t.Fatalf("setting on: %+v %+v", first, second)
			}
			engine.Options.AllowDuplicateSave = true
			if r := insertRow(engine, "Account", map[string]string{"Name": "Test Account 3", "BillingCity": "Springfield"}); !r.Success {
				t.Fatalf("allowSave must save: %+v", r)
			}
		})
	}
}

// A project's metadata for a standard rule wins over the setting, active or
// inactive, and custom rules are enforced whatever the setting says.
func TestStandardDuplicateRuleMetadataOverridesSetting(t *testing.T) {
	for _, tc := range []struct {
		name    string
		setting bool
		active  bool
		block   bool
	}{
		{"metadata active, setting off", false, true, true},
		{"metadata inactive, setting on", true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := seededAccountOrg(t)
			org.StandardDuplicateRules = tc.setting
			// Project metadata replaces the seeded rule: same name, not seeded.
			state := org.Objects["Account"]
			state.Definition.DuplicateRules = []storage.DuplicateRule{{
				Name: "Standard_Account_Duplicate_Rule", Object: "Account", Active: tc.active, ActionOnInsert: "Block",
				MatchRules: []storage.MatchingRule{{Object: "Account", Name: "Standard_Account_Match_Rule_v1_0", Status: "Active"}},
			}}
			org.Objects["Account"] = state
			engine := dml.NewEngine(org)
			engine.Options.EnforceDuplicateRules = true
			first, second := insertTestAccounts(engine)
			if !first.Success || blocked(second) != tc.block {
				t.Fatalf("%+v %+v", first, second)
			}
		})
	}
}

func TestCustomDuplicateRulesIgnoreStandardSetting(t *testing.T) {
	org := seededAccountOrg(t)
	state := org.Objects["Account"]
	state.Definition.DuplicateRules = append(state.Definition.DuplicateRules, storage.DuplicateRule{
		Name: "Custom", Object: "Account", Active: true, ActionOnInsert: "Block", SortOrder: 9,
		MatchRules: []storage.MatchingRule{{Object: "Account", Name: "FuzzyName", Status: "Active",
			Items: []storage.MatchingRuleItem{{Field: "Name", Method: "CompanyName", BlankBehavior: "NullNotAllowed"}}}},
	})
	org.Objects["Account"] = state
	engine := dml.NewEngine(org)
	engine.Options.EnforceDuplicateRules = true
	if r := insertRow(engine, "Account", map[string]string{"Name": "Acme Widgets"}); !r.Success {
		t.Fatalf("seed: %+v", r)
	}
	if r := insertRow(engine, "Account", map[string]string{"Name": "Acme Widgets"}); !blocked(r) {
		t.Fatalf("the custom rule must be enforced with the setting off: %+v", r)
	}
}
