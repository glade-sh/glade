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
