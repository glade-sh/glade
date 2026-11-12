package dml_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func fuzzyContactOrg(t *testing.T, action string, items ...storage.MatchingRuleItem) *storage.OrgState {
	t.Helper()
	org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, "Contact")
	state := org.Objects["Contact"]
	state.Definition.DuplicateRules = append(state.Definition.DuplicateRules, storage.DuplicateRule{
		Name: "Fuzzy", Object: "Contact", Active: true, ActionOnInsert: action, ActionOnUpdate: action, AlertText: "fuzzy duplicate",
		MatchRules: []storage.MatchingRule{{Object: "Contact", Name: "FuzzyFirst", Status: "Active", Items: items}},
	})
	org.Objects["Contact"] = state
	return &org
}

func insertContact(engine dml.Engine, first, last string) dml.Result {
	fields := map[string]storage.Value{"LastName": storage.StringValue(last)}
	if first != "" {
		fields["FirstName"] = storage.StringValue(first)
	}
	return engine.Insert([]storage.Record{{Object: "Contact", Fields: fields}})[0]
}

// A blocking duplicate rule with a fuzzy first-name item is enforced on insert
// and update through the shared DML path: nicknames from the Name Variant
// group match, unrelated names and blank values do not.
func TestFuzzyDuplicateRuleBlocksNicknameDML(t *testing.T) {
	org := fuzzyContactOrg(t, "Block", storage.MatchingRuleItem{Field: "FirstName", Method: "FirstName", BlankBehavior: "NullNotAllowed"})
	engine := dml.NewEngine(org)
	engine.Options.EnforceDuplicateRules = true
	if r := insertContact(engine, "Robert", "One"); !r.Success {
		t.Fatalf("seed: %+v", r)
	}
	if r := insertContact(engine, "Bob", "Two"); r.Success || r.StatusCode != "DUPLICATES_DETECTED" {
		t.Fatalf("nickname must be blocked: %+v", r)
	}
	for _, name := range []string{"Alice", ""} {
		if r := insertContact(engine, name, "Three"); !r.Success {
			t.Fatalf("%q must be allowed: %+v", name, r)
		}
	}
	// A blank value never matches another blank value under NullNotAllowed.
	if r := insertContact(engine, "", "Four"); !r.Success {
		t.Fatalf("blank pair must not match: %+v", r)
	}
	// Update: renaming a record to a nickname of another is blocked.
	seeded := org.Objects["Contact"]
	var alice storage.Record
	for _, record := range seeded.Records {
		if value, ok := record.GetField("FirstName"); ok && value.String == "Alice" {
			alice = record.Clone()
		}
	}
	alice.Fields["FirstName"] = storage.StringValue("Rob")
	if results := engine.Update([]storage.Record{alice}); len(results) != 1 || results[0].Success || results[0].StatusCode != "DUPLICATES_DETECTED" {
		t.Fatalf("update to a nickname must be blocked: %+v", results)
	}
}

// MatchBlanks pairs two blank values for a fuzzy item as it does for an exact
// item (native Account rows with Match Blank Fields).
func TestFuzzyDuplicateRuleMatchBlanks(t *testing.T) {
	org := fuzzyContactOrg(t, "Block", storage.MatchingRuleItem{Field: "FirstName", Method: "FirstName", BlankBehavior: "MatchBlanks"})
	engine := dml.NewEngine(org)
	engine.Options.EnforceDuplicateRules = true
	if r := insertContact(engine, "", "One"); !r.Success {
		t.Fatalf("seed: %+v", r)
	}
	if r := insertContact(engine, "", "Two"); r.Success || r.StatusCode != "DUPLICATES_DETECTED" {
		t.Fatalf("blank pair must match under MatchBlanks: %+v", r)
	}
	if r := insertContact(engine, "Bob", "Three"); !r.Success {
		t.Fatalf("one blank side must not match: %+v", r)
	}
}

// Unknown fuzzy methods stay unenforced, as before fuzzy support.
func TestFuzzyDuplicateRuleUnknownMethodUnenforced(t *testing.T) {
	org := fuzzyContactOrg(t, "Block", storage.MatchingRuleItem{Field: "FirstName", Method: "Soundex", BlankBehavior: "NullNotAllowed"})
	engine := dml.NewEngine(org)
	engine.Options.EnforceDuplicateRules = true
	insertContact(engine, "Robert", "One")
	if r := insertContact(engine, "Robert", "Two"); !r.Success {
		t.Fatalf("unsupported method must not block: %+v", r)
	}
}

// The metadata route: a fuzzy matching rule and its duplicate rule loaded from
// project files are enforced by the same matcher.
func TestFuzzyDuplicateRuleFromProjectMetadata(t *testing.T) {
	root := t.TempDir()
	write := func(path, body string) {
		t.Helper()
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("sfdx-project.json", `{"packageDirectories": [{"path": "force-app", "default": true}], "sourceApiVersion": "67.0"}`)
	write("force-app/main/default/matchingRules/Contact.matchingRule-meta.xml", `<MatchingRules xmlns="http://soap.sforce.com/2006/04/metadata"><matchingRules><fullName>Fuzzy_Phone</fullName><label>Fuzzy Phone</label><matchingRuleItems><blankValueBehavior>NullNotAllowed</blankValueBehavior><fieldName>Phone</fieldName><matchingMethod>Phone</matchingMethod></matchingRuleItems><ruleStatus>Active</ruleStatus></matchingRules></MatchingRules>`)
	write("force-app/main/default/duplicateRules/Contact.Fuzzy_Phone.duplicateRule-meta.xml", `<DuplicateRule xmlns="http://soap.sforce.com/2006/04/metadata"><actionOnInsert>Block</actionOnInsert><actionOnUpdate>Block</actionOnUpdate><alertText>phone duplicate</alertText><duplicateRuleMatchRules><matchRuleSObjectType>Contact</matchRuleSObjectType><matchingRule>Fuzzy_Phone</matchingRule></duplicateRuleMatchRules><isActive>true</isActive><masterLabel>Fuzzy Phone</masterLabel><securityOption>BypassSharingRules</securityOption><sortOrder>1</sortOrder></DuplicateRule>`)
	loaded, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	policies, err := automation.LoadProject(loaded)
	if err != nil || len(policies.Diagnostics) != 0 {
		t.Fatalf("load policies: %v %v", err, policies.Diagnostics)
	}
	org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, "Contact")
	automation.ApplyToOrg(&org, policies)
	engine := dml.NewEngine(&org)
	engine.Options.EnforceDuplicateRules = true
	insertPhone := func(last, phone string) dml.Result {
		return engine.Insert([]storage.Record{{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue(last), "Phone": storage.StringValue(phone)}}})[0]
	}
	if r := insertPhone("One", "(415) 555-1234"); !r.Success {
		t.Fatalf("seed: %+v", r)
	}
	// Same digits in another format, and a differing line number (weighted score 90).
	for _, phone := range []string{"415.555.1234", "415-555-9999"} {
		if r := insertPhone("Two", phone); r.Success || r.StatusCode != "DUPLICATES_DETECTED" {
			t.Fatalf("%s must be blocked: %+v", phone, r)
		}
	}
	// A different area code scores below the threshold, and an extension makes
	// the number unparseable, so neither matches.
	for _, phone := range []string{"425-555-1234", "415-555-1234 x5"} {
		if r := insertPhone("Three", phone); !r.Success {
			t.Fatalf("%s must be allowed: %+v", phone, r)
		}
	}
}
