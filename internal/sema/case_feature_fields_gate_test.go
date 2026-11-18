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

// NATIVE Case.Language, ConnectionReceivedId and ConnectionSentId do not compile in an org with no feature
// internal/apextest/testdata/conformance/standard_case_field_features_none/org.tsv:26-37,50-73 anonymous 62.0 67.0
// NATIVE the same with LiveAgent, Knowledge, Knowledge with two active languages and a service bundle
// internal/apextest/testdata/conformance/standard_case_field_features_liveagent/org.tsv:26-37,50-73 anonymous 62.0 67.0
// internal/apextest/testdata/conformance/standard_case_field_features_knowledge/org.tsv:26-37,50-73 anonymous 62.0 67.0
// internal/apextest/testdata/conformance/standard_case_field_features_knowledge_languages/org.tsv:26-37,50-73 anonymous 62.0 67.0
// internal/apextest/testdata/conformance/standard_case_field_features_service_bundle/org.tsv:26-37,50-73 anonymous 62.0 67.0
// A project with no scratch definition keeps them open: its org shape is unknown.
func TestCaseUnsuppliedFieldsAreCompileErrorsWithScratchDefinition(t *testing.T) {
	definitions := map[string]string{
		"no features":     `{"edition":"Developer"}`,
		"live agent":      `{"edition":"Developer","features":["LiveAgent"]}`,
		"division":        `{"edition":"Developer","features":["Division"]}`,
		"knowledge":       `{"edition":"Developer","features":["Knowledge","ServiceCloud"],"settings":{"knowledgeSettings":{"enableKnowledge":true}}}`,
		"service bundle":  `{"edition":"Developer","features":["Communities","ServiceCloud","LiveAgent","SocialCustomerService","Knowledge","MultiCurrency","Entitlements"]}`,
		"no scratch file": "",
	}
	statements := map[string]string{
		"Language read":             "return c.Language;",
		"Language assign":           "c.Language = null; return null;",
		"ConnectionReceivedId read": "return c.ConnectionReceivedId;",
		"ConnectionSentId assign":   "c.ConnectionSentId = null; return null;",
		"Language constructor":      "Case d = new Case(Language = null); return d;",
	}
	for _, api := range []string{"62.0", "67.0"} {
		for defName, def := range definitions {
			for name, body := range statements {
				t.Run(api+"/"+defName+"/"+name, func(t *testing.T) {
					root := t.TempDir()
					path := filepath.Join(root, "Probe.cls")
					writeSemaFile(t, path, "public class Probe { public static Object run(Case c) { "+body+" } }")
					if def != "" {
						if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
							t.Fatal(err)
						}
						writeSemaFile(t, filepath.Join(root, "config/project-scratch-def.json"), def)
					}
					index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
					rejected := false
					for _, d := range Analyze(index).Diagnostics {
						if strings.Contains(d.Message, "Language") || strings.Contains(d.Message, "Connection") {
							rejected = true
						}
					}
					if want := def != ""; rejected != want {
						t.Fatalf("rejected=%t, want %t", rejected, want)
					}
				})
			}
		}
	}
}
