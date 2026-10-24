package project

import (
	"path/filepath"
	"strings"
	"testing"
)

func suggestionFor(t *testing.T, got []OrgSetupSuggestion, kind string) OrgSetupSuggestion {
	t.Helper()
	for _, s := range got {
		if s.Kind == kind {
			return s
		}
	}
	t.Fatalf("no %s suggestion in %+v", kind, got)
	return OrgSetupSuggestion{}
}

func TestSuggestOrgSetupPersonAccounts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/AcctTest.cls"),
		"@IsTest\nprivate class AcctTest {\n  @IsTest static void t() {\n    Account a = [SELECT Id FROM Account WHERE IsPersonAccount = true LIMIT 1];\n  }\n}\n")
	got := SuggestOrgSetup(root)
	s := suggestionFor(t, got, "feature")
	if s.Feature != "PersonAccounts" || len(s.Files) != 1 || !strings.HasSuffix(s.Files[0], "AcctTest.cls") {
		t.Fatalf("suggestion=%+v", s)
	}
	if want := "org:\n  features: [PersonAccounts]"; s.YAML != want {
		t.Fatalf("yaml=%q", s.YAML)
	}
}

func TestSuggestOrgSetupSkipsDeclaredFeature(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"glade.yml":  {"glade.yml": "org:\n  features: [PersonAccounts]\n"},
		"scratchdef": {"config/project-scratch-def.json": `{"features":["PersonAccounts"]}`},
	} {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app"}]}`)
		writeFile(t, filepath.Join(root, "force-app/c/classes/A.cls"), "class A { Boolean b = Account.PersonEmail != null; }")
		for p, c := range files {
			writeFile(t, filepath.Join(root, p), c)
		}
		got := SuggestOrgSetup(root)
		for _, suggestion := range got {
			if suggestion.Kind == "feature" {
				t.Fatalf("%s: declared feature was re-suggested: %+v", name, suggestion)
			}
		}
		if name == "scratchdef" && (len(got) != 1 || got[0].Kind != "scratchFeature") {
			t.Fatalf("%s: scratch feature mapping missing: %+v", name, got)
		}
		if name == "glade.yml" && len(got) != 0 {
			t.Fatalf("%s: unexpected %+v", name, got)
		}
	}
}

func TestSuggestOrgSetupIgnoresLookalikeNames(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app"}]}`)
	// A class named after person accounts is not evidence of a person-account field.
	writeFile(t, filepath.Join(root, "force-app/c/classes/PersonAccountSearch.cls"), "public class PersonAccountSearch { public String PersonalNote; }")
	writeFile(t, filepath.Join(root, "force-app/c/classes/Net.cls"), "public class Net { String s = 'FROM NetworkHelper__c'; Object o = NetworkTools.run(); }")
	if got := SuggestOrgSetup(root); len(got) != 0 {
		t.Fatalf("unexpected %+v", got)
	}
}

func TestSuggestOrgSetupCustomPersonFieldsAndMerge(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app"}]}`)
	writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [MultiCurrency]\n")
	writeFile(t, filepath.Join(root, "force-app/c/classes/B.cls"), "class B { Object o = new Account().get('Loyalty__pc'); }")
	writeFile(t, filepath.Join(root, "force-app/c/classes/C.cls"), "class C { void m() { List<Network> n = [SELECT Id FROM Network]; Id x = Network.getNetworkId(); } }")
	got := SuggestOrgSetup(root)
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	p := suggestionFor(t, got, "feature")
	// Existing glade.yml entries are kept in the printed list.
	if !strings.Contains(p.YAML, "MultiCurrency") || !strings.Contains(p.YAML, "PersonAccounts") || !strings.Contains(p.YAML, "Communities") {
		t.Fatalf("yaml=%q", p.YAML)
	}
}

func TestSuggestOrgSetupMultiCurrencyAndSites(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app"}]}`)
	writeFile(t, filepath.Join(root, "force-app/c/classes/D.cls"), "class D { void m(Opportunity o) { o.CurrencyIsoCode = 'EUR'; String u = Site.getBaseUrl(); } }")
	got := SuggestOrgSetup(root)
	feats := map[string]bool{}
	for _, s := range got {
		feats[s.Feature] = true
	}
	if !feats["MultiCurrency"] || !feats["Sites"] || len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestSuggestOrgSetupPermissionSets(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app"}]}`)
	writeFile(t, filepath.Join(root, "force-app/c/permissionsets/Synthetic_Access.permissionset-meta.xml"), "<PermissionSet/>")
	writeFile(t, filepath.Join(root, "force-app/c/permissionsets/Other.permissionset-meta.xml"), "<PermissionSet/>")
	writeFile(t, filepath.Join(root, "force-app/c/classes/PTest.cls"),
		"@IsTest class PTest { static void s(Id u) { PermissionSet ps = [SELECT Id FROM PermissionSet WHERE Name = 'Synthetic_Access']; insert new PermissionSetAssignment(AssigneeId = u, PermissionSetId = ps.Id); } }")
	got := SuggestOrgSetup(root)
	s := suggestionFor(t, got, "permissionSet")
	if want := "test:\n  permissionSets: [Synthetic_Access]"; s.YAML != want || len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [Synthetic_Access]\n")
	if got := SuggestOrgSetup(root); len(got) != 0 {
		t.Fatalf("declared: %+v", got)
	}
}

func TestSuggestOrgSetupSkipsNodeModules(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"."}]}`)
	writeFile(t, filepath.Join(root, "node_modules/x/a.js"), "Account.IsPersonAccount")
	if got := SuggestOrgSetup(root); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}
