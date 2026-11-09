package project

import (
	"path/filepath"
	"strings"
	"testing"
)

// Native captures (apextest org_settings_source and the Network rows in
// org_object_availability): the Communities feature alone leaves Network and
// NetworkMember absent; communitiesSettings.enableNetworksEnabled adds them.
func suggestedFeatures(t *testing.T, files map[string]string) map[string]OrgSetupSuggestion {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app"}]}`)
	for name, body := range files {
		writeFile(t, filepath.Join(root, name), body)
	}
	out := map[string]OrgSetupSuggestion{}
	for _, s := range SuggestOrgSetup(root) {
		if s.Kind == "feature" {
			out[s.Feature] = s
		}
	}
	return out
}

func TestSuggestOrgSetupNetworkObjectsNameNativeSetting(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"soqlNetwork":       {"force-app/c/classes/A.cls": "class A { Object m() { return [SELECT Id FROM Network]; } }"},
		"soqlNetworkMember": {"force-app/c/classes/A.cls": "class A { Object m() { return [SELECT Id FROM NetworkMember]; } }"},
		"dynamicQuery":      {"force-app/c/classes/A.cls": "class A { Object m() { return Database.query('SELECT Id FROM Network'); } }"},
		"flowObject":        {"force-app/c/flows/F.flow-meta.xml": "<Flow><recordLookups><object>Network</object></recordLookups></Flow>"},
	} {
		t.Run(name, func(t *testing.T) {
			got := suggestedFeatures(t, files)
			s, ok := got["NetworksEnabled"]
			if !ok || len(got) != 1 {
				t.Fatalf("want only NetworksEnabled, got %+v", got)
			}
			if !strings.Contains(s.Reason, "communitiesSettings enableNetworksEnabled") {
				t.Fatalf("reason does not name the native setting: %q", s.Reason)
			}
			if !strings.Contains(s.YAML, "features: [NetworksEnabled]") || !strings.Contains(s.YAML, "enableNetworksEnabled") || strings.Contains(s.YAML, "Communities") {
				t.Fatalf("yaml=%q", s.YAML)
			}
		})
	}
}

func TestSuggestOrgSetupNetworkClassCallStaysCommunities(t *testing.T) {
	got := suggestedFeatures(t, map[string]map[string]string{"x": {"force-app/c/classes/A.cls": "class A { Id m() { return Network.getNetworkId(); } }"}}["x"])
	if _, ok := got["Communities"]; !ok || len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	got = suggestedFeatures(t, map[string]string{"force-app/c/classes/A.cls": "class A { Object m(User u) { return u.IsPortalEnabled; } }"})
	if _, ok := got["Communities"]; !ok || len(got) != 1 {
		t.Fatalf("portal field: got %+v", got)
	}
}

func TestSuggestOrgSetupNetworksDeclaredShape(t *testing.T) {
	source := map[string]string{"force-app/c/classes/A.cls": "class A { Object m() { return [SELECT Id FROM Network]; } }"}
	for feature, wantSuggestion := range map[string]bool{"NetworksEnabled": false, "Communities": true} {
		files := map[string]string{"glade.yml": "org:\n  features: [" + feature + "]\n"}
		for k, v := range source {
			files[k] = v
		}
		got := suggestedFeatures(t, files)
		if _, ok := got["NetworksEnabled"]; ok != wantSuggestion {
			t.Fatalf("declared %s: suggestion=%v, got %+v", feature, ok, got)
		}
	}
}
