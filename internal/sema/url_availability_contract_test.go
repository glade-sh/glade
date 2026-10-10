package sema

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// Source: apex_methods_system_url.md, lines 470-488, SHA256
// 4603656c0a4296f10e64939ee225286de77635bf06d8692348e40568e7b43b6b.
// getSalesforceBaseUrl is versioned out in API 59 and later. These cases
// exercise current supported source profiles, not native qualification.
func TestURLMethodAvailabilitySupportedProfiles(t *testing.T) {
	for _, tc := range []struct {
		name      string
		version   string
		anonymous bool
		body      string
		shadow    bool
		removed   bool
	}{
		{"named62", "62.0", false, `System.URL value = URL.getSalesforceBaseUrl();`, false, true},
		{"named65Qualified", "65.0", false, `System.URL value = System.URL.getSalesforceBaseUrl();`, false, true},
		{"named67MixedCase", "67.0", false, `System.URL value = system.url.getSalesforceBaseURL();`, false, true},
		{"anonymous65", "65.0", true, `System.URL value = URL.getSalesforceBaseUrl();`, false, true},
		{"anonymous66Qualified", "66.0", true, `System.URL value = System.URL.getSalesforceBaseUrl();`, false, true},
		{"anonymous67MixedCase", "67.0", true, `System.URL value = system.url.getSalesforceBaseURL();`, false, true},
		{"named62Replacements", "62.0", false, `System.URL org = URL.getOrgDomainUrl(); System.URL request = URL.getCurrentRequestUrl();`, false, false},
		{"named67Replacements", "67.0", false, `System.URL org = System.URL.getOrgDomainURL(); System.URL request = System.URL.getCurrentRequestURL();`, false, false},
		{"anonymous67Replacements", "67.0", true, `System.URL org = URL.getOrgDomainUrl(); System.URL request = System.URL.getCurrentRequestUrl();`, false, false},
		{"named67ProjectURLShadow", "67.0", false, `Integer value = URL.getSalesforceBaseUrl();`, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var result Result
			if tc.anonymous {
				result = AnalyzeAnonymous(typesys.Index{}, tc.body, tc.version)
			} else {
				files := map[string]string{"Probe.cls": `public class Probe { public static void run() { ` + tc.body + ` } }`}
				if tc.shadow {
					files["URL.cls"] = `public class URL { public static Integer getSalesforceBaseUrl() { return 7; } }`
				}
				result = analyzeDeclarationProjectWithAPIVersion(t, files, tc.version)
				if result.Project.SourceAPIVersion != tc.version {
					t.Fatalf("URL source profile changed [%s]: got %q want %q", tc.name, result.Project.SourceAPIVersion, tc.version)
				}
			}
			if tc.removed {
				if len(result.Diagnostics) == 0 {
					t.Fatalf("URL removed call was accepted [%s]: %#v", tc.name, result.Diagnostics)
				}
				if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "GLADESEMA028" || !strings.Contains(strings.ToLower(result.Diagnostics[0].Message), "getsalesforcebaseurl") {
					t.Fatalf("URL removed call had unrelated diagnostics [%s]: %#v", tc.name, result.Diagnostics)
				}
			} else if result.HasErrors() {
				t.Fatalf("URL allowed control was rejected [%s]: %#v", tc.name, result.Diagnostics)
			}
		})
	}
}
