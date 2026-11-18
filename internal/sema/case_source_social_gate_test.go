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

// NATIVE Case.SourceId compiles with the SocialCustomerService feature alone
// internal/apextest/testdata/conformance/standard_case_source_social/org.tsv:2 anonymous 62.0
// NATIVE Case.SourceId compiles with the SocialCustomerService feature alone
// internal/apextest/testdata/conformance/standard_case_source_social/org.tsv:3 anonymous 67.0
// NATIVE Case.SourceId compiles with LiveAgent enabled
// internal/apextest/testdata/conformance/standard_case_source_evidence/org.tsv:2 anonymous 62.0
// NATIVE Case.SourceId is rejected in an org with no feature that supplies it
// internal/apextest/testdata/conformance/unknown_sobject_field/org.tsv:490 anonymous 62.0
func TestCaseSourceIdFollowsSupplyingFeatures(t *testing.T) {
	cases := []struct {
		name     string
		def      string
		rejected bool
	}{
		{"social customer service", `{"edition":"Developer","features":["SocialCustomerService"]}`, false},
		{"live agent", `{"edition":"Developer","features":["LiveAgent"],"settings":{"liveAgentSettings":{"enableLiveAgent":true}}}`, false},
		{"other features", `{"edition":"Developer","features":["Communities","ServiceCloud"]}`, true},
		{"no features", `{"edition":"Developer"}`, true},
		{"no scratch definition", "", false},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, c := range cases {
			t.Run(api+"/"+c.name, func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "Probe.cls")
				writeSemaFile(t, path, "public class Probe { public static Object run(Case c) { return c.SourceId; } }")
				if c.def != "" {
					if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
						t.Fatal(err)
					}
					writeSemaFile(t, filepath.Join(root, "config/project-scratch-def.json"), c.def)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
				rejected := false
				for _, d := range Analyze(index).Diagnostics {
					if strings.Contains(d.Message, "SourceId") {
						rejected = true
					}
				}
				if rejected != c.rejected {
					t.Fatalf("SourceId rejected=%t, want %t", rejected, c.rejected)
				}
			})
		}
	}
}

// NATIVE User.MiddleName compiles with nameSettings.enableMiddleName on
// internal/apextest/testdata/conformance/standard_user_name_settings/org.tsv:6 anonymous 62.0
// NATIVE User.MiddleName compiles with nameSettings.enableMiddleName on
// internal/apextest/testdata/conformance/standard_user_name_settings/org.tsv:7 anonymous 67.0
// NATIVE User.Suffix and User.UserPermissionsLiveAgentUser compile in that org
// internal/apextest/testdata/conformance/standard_user_name_settings/org.tsv:8-9,12-13 anonymous 62.0 67.0
// NATIVE User.MiddleName is absent in an org without the setting
// internal/apextest/testdata/conformance/standard_industries_schema/org.tsv:7 anonymous 62.0
func TestUserNameSettingFieldsFollowDeclaredSettings(t *testing.T) {
	cases := []struct {
		name         string
		def          string
		settingsFile string
		middleHidden bool
	}{
		{"setting in scratch definition", `{"edition":"Developer","features":["LiveAgent"],"settings":{"nameSettings":{"enableMiddleName":true,"enableNameSuffix":true},"liveAgentSettings":{"enableLiveAgent":true}}}`, "", false},
		{"setting in deployed file", `{"edition":"Developer","features":["Communities"]}`, `<?xml version="1.0" encoding="UTF-8"?><NameSettings xmlns="http://soap.sforce.com/2006/04/metadata"><enableMiddleName>true</enableMiddleName><enableNameSuffix>true</enableNameSuffix></NameSettings>`, false},
		{"feature name only", `{"edition":"Developer","features":["MiddleNameEnabled"]}`, "", false},
		{"other features", `{"edition":"Developer","features":["Communities"]}`, "", true},
		{"no scratch definition", "", "", false},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, c := range cases {
			t.Run(api+"/"+c.name, func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "Probe.cls")
				writeSemaFile(t, path, "public class Probe { public static Object a(User u) { return u.MiddleName; } }")
				if c.def != "" {
					writeSemaNested(t, filepath.Join(root, "config/project-scratch-def.json"), c.def)
				}
				if c.settingsFile != "" {
					writeSemaNested(t, filepath.Join(root, "force-app/main/default/settings/Name.settings-meta.xml"), c.settingsFile)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
				hidden := false
				for _, d := range Analyze(index).Diagnostics {
					if strings.Contains(d.Message, "MiddleName") {
						hidden = true
					}
				}
				if hidden != c.middleHidden {
					t.Fatalf("MiddleName rejected=%t, want %t", hidden, c.middleHidden)
				}
			})
		}
	}
}

func writeSemaNested(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, path, contents)
}
