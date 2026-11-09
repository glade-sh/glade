package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// capi3_feature_gates/org.tsv, shapes "features=Communities" and
// "settings=communitiesSettings.enableNetworksEnabled": the five networks
// services (NavigationMenu, Personalization, ExternalManagedAccount,
// MarketingIntegration, Sites) are visible only with the networks setting. The
// Communities feature alone, or beside a disabled setting, hides them; a
// project that declares no org shape stays open (unspecified shapes are ungated). The scratch
// definitions carry an edition, as every real one does. The setting counts
// whether it comes from the scratch definition or a deployed settings file.
func TestCapi4NetworksSettingGate(t *testing.T) {
	const settingsFile = `<?xml version="1.0" encoding="UTF-8"?><CommunitiesSettings xmlns="http://soap.sforce.com/2006/04/metadata"><enableNetworksEnabled>true</enableNetworksEnabled></CommunitiesSettings>`
	cases := []struct {
		name     string
		def      string
		settings string
		hidden   bool
	}{
		{"feature alone", `{"edition":"Developer","features":["Communities"]}`, "", true},
		{"feature with the setting off", `{"edition":"Developer","features":["Communities"],"settings":{"communitiesSettings":{"enableNetworksEnabled":false}}}`, "", true},
		{"feature with the setting on", `{"edition":"Developer","features":["Communities"],"settings":{"communitiesSettings":{"enableNetworksEnabled":true}}}`, "", false},
		{"setting alone", `{"edition":"Developer","settings":{"communitiesSettings":{"enableNetworksEnabled":true}}}`, "", false},
		{"feature with a deployed settings file", `{"edition":"Developer","features":["Communities"]}`, settingsFile, false},
		{"other feature", `{"edition":"Developer","features":["ServiceCloud"]}`, "", true},
		{"no scratch definition", "", "", false},
	}
	services := []string{"NavigationMenu", "Personalization", "ExternalManagedAccount", "MarketingIntegration", "Sites"}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, c := range cases {
			t.Run(api+"/"+c.name, func(t *testing.T) {
				root := t.TempDir()
				var b strings.Builder
				b.WriteString("public class Probe { public static void run() {\n")
				for i, service := range services {
					b.WriteString("ConnectApi." + service + " v" + string(rune('a'+i)) + ";\n")
				}
				b.WriteString("} }")
				path := filepath.Join(root, "Probe.cls")
				writeSemaFile(t, path, b.String())
				if c.def != "" {
					if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
						t.Fatal(err)
					}
					writeSemaFile(t, filepath.Join(root, "config/project-scratch-def.json"), c.def)
				}
				if c.settings != "" {
					if err := os.MkdirAll(filepath.Join(root, "force-app/main/default/settings"), 0o755); err != nil {
						t.Fatal(err)
					}
					writeSemaFile(t, filepath.Join(root, "force-app/main/default/settings/Communities.settings-meta.xml"), c.settings)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
				hidden := map[string]bool{}
				for _, d := range Analyze(index).Diagnostics {
					if name, ok := strings.CutPrefix(d.Message, "Type is not visible: ConnectApi."); ok {
						hidden[name] = true
					}
				}
				for _, service := range services {
					if hidden[service] != c.hidden {
						t.Errorf("%s hidden = %v, want %v (features %v)", service, hidden[service], c.hidden, index.OrgShapeFeatures)
					}
				}
			})
		}
	}
}
