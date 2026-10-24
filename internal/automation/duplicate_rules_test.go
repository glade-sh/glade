package automation

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestDuplicateRuleLoadAndApplyDeployProvenMetadataRoute(t *testing.T) {
	root := filepath.Join("..", "apextest", "testdata", "conformance", "setup_metadata", "project")
	assertDuplicateRuleLoad(t, root)
}

func assertDuplicateRuleLoad(t *testing.T, root string) {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatalf("deploy-proven metadata has loader diagnostics: %v", idx.Diagnostics)
	}
	org := storage.OrgState{}
	storage.EnsureStandardObject(&org, "Contact")
	ApplyToOrg(&org, idx)
	ApplyToOrg(&org, idx) // Reapplication replaces the same metadata rather than accumulating rules.
	body, err := json.Marshal(org.Objects["Contact"].Definition)
	if err != nil {
		t.Fatal(err)
	}
	var definition struct {
		Rules []struct {
			Name           string `json:"name"`
			Active         bool   `json:"active"`
			ActionOnInsert string `json:"actionOnInsert"`
			ActionOnUpdate string `json:"actionOnUpdate"`
			SortOrder      int    `json:"sortOrder"`
			MatchRules     []struct {
				Object string `json:"object"`
				Name   string `json:"name"`
				Items  []struct {
					Field         string `json:"field"`
					Method        string `json:"method"`
					BlankBehavior string `json:"blankBehavior"`
				} `json:"items"`
			} `json:"matchRules"`
		} `json:"duplicateRules"`
	}
	if err := json.Unmarshal(body, &definition); err != nil {
		t.Fatal(err)
	}
	if len(definition.Rules) != 1 {
		t.Fatalf("Contact duplicate definitions=%d, want 1 from deployed metadata", len(definition.Rules))
	}
	rule := definition.Rules[0]
	if rule.Name != "Grove_Email_Check" || !rule.Active || rule.ActionOnInsert != "Allow" || rule.ActionOnUpdate != "Allow" || rule.SortOrder != 3 {
		t.Fatalf("duplicate definition lost deployed values: %+v", rule)
	}
	if len(rule.MatchRules) != 1 || rule.MatchRules[0].Object != "Contact" || rule.MatchRules[0].Name != "Grove_Exact_Email" || len(rule.MatchRules[0].Items) != 1 {
		t.Fatalf("matching definition not joined: %+v", rule.MatchRules)
	}
	item := rule.MatchRules[0].Items[0]
	if item.Field != "Email" || item.Method != "Exact" || item.BlankBehavior != "NullNotAllowed" {
		t.Fatalf("matching item lost deployed values: %+v", item)
	}
}

func TestDuplicateRuleLoadSourceMetadataAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"source", "metadata"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				fixture := filepath.Join("..", "apextest", "testdata", "conformance", "setup_metadata", "project")
				err := filepath.WalkDir(fixture, func(path string, entry fs.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					name, err := filepath.Rel(fixture, path)
					if err != nil {
						return err
					}
					if route == "metadata" {
						name = strings.ReplaceAll(name, "Rule-meta.xml", "Rule")
					}
					body, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					writeWorkflowTestFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				assertDuplicateRuleLoad(t, root)
			})
		}
	}
}

func TestDuplicateRuleLoaderMalformedAndMissingMatchingRoutes(t *testing.T) {
	for _, route := range []string{"duplicate_xml", "matching_xml"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			duplicate := `<DuplicateRule><isActive>true</isActive><duplicateRuleMatchRules><matchRuleSObjectType>Contact</matchRuleSObjectType><matchingRule>Grove_Exact_Email</matchingRule></duplicateRuleMatchRules></DuplicateRule>`
			if route == "duplicate_xml" {
				duplicate = `<DuplicateRule>`
			}
			writeWorkflowTestFile(t, filepath.Join(root, "duplicateRules/Contact.Grove_Email_Check.duplicateRule-meta.xml"), duplicate)
			if route == "matching_xml" {
				writeWorkflowTestFile(t, filepath.Join(root, "matchingRules/Contact.matchingRule-meta.xml"), `<MatchingRules>`)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			idx, err := LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			if len(idx.Diagnostics) == 0 {
				t.Fatal("broken duplicate/matching metadata was silently ignored")
			}
			for _, d := range idx.Diagnostics {
				if d.File == "" || d.Code != "GLADEAUTO003" {
					t.Fatalf("missing file-scoped load diagnostic: %+v", d)
				}
			}
		})
	}
}

func TestDuplicateRuleUndeclaredMatchingRuleLoadsWithoutDiagnostics(t *testing.T) {
	for _, name := range []string{"Standard_Contact_Match_Rule_v1_1", "Grove_Exact_Email"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			duplicate := `<DuplicateRule><isActive>true</isActive><duplicateRuleMatchRules><matchRuleSObjectType>Contact</matchRuleSObjectType><matchingRule>` + name + `</matchingRule></duplicateRuleMatchRules></DuplicateRule>`
			writeWorkflowTestFile(t, filepath.Join(root, "duplicateRules/Contact.Grove_Email_Check.duplicateRule-meta.xml"), duplicate)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			idx, err := LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			if len(idx.Diagnostics) != 0 {
				t.Fatalf("undeclared matching rule reference produced diagnostics: %+v", idx.Diagnostics)
			}
			org := storage.OrgState{}
			storage.EnsureStandardObject(&org, "Contact")
			ApplyToOrg(&org, idx)
			rules := org.Objects["Contact"].Definition.DuplicateRules
			if len(rules) != 1 || len(rules[0].MatchRules) != 1 || rules[0].MatchRules[0].Object != "Contact" || rules[0].MatchRules[0].Name != name {
				t.Fatalf("placeholder matching rule not recorded: %+v", rules)
			}
		})
	}
}

func TestDuplicateRuleApplyIndexAndOrgIsolationRoutes(t *testing.T) {
	p, err := project.Load(filepath.Join("..", "apextest", "testdata", "conformance", "setup_metadata", "project"))
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Diagnostics) != 0 {
		t.Fatal(idx.Diagnostics)
	}
	first, second := storage.OrgState{}, storage.OrgState{}
	storage.EnsureStandardObject(&first, "Contact")
	storage.EnsureStandardObject(&second, "Contact")
	ApplyToOrg(&first, idx)
	ApplyToOrg(&second, idx)
	state := first.Objects["Contact"]
	if len(state.Definition.DuplicateRules) != 1 {
		t.Fatalf("rules=%d", len(state.Definition.DuplicateRules))
	}
	state.Definition.DuplicateRules[0].MatchRules[0].Items[0].Field = "LastName"
	state.Definition.DuplicateRules[0].OperationsOnInsert[0] = "Alert"
	other := second.Objects["Contact"].Definition.DuplicateRules[0]
	if other.MatchRules[0].Items[0].Field != "Email" || other.OperationsOnInsert[0] != "Report" {
		t.Fatal("org installations share mutable metadata")
	}
	ApplyToOrg(&first, idx)
	restored := first.Objects["Contact"].Definition.DuplicateRules[0]
	if restored.MatchRules[0].Items[0].Field != "Email" || restored.OperationsOnInsert[0] != "Report" {
		t.Fatal("org mutation changed the reusable loader index")
	}
}
