package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Replays the anonymous Case field cases against the scratch definition that
// each capture used. Row lines are cited per behavior in the tests below.
//
// NATIVE Case.Language, ConnectionReceivedId and ConnectionSentId: compile errors, run-time errors and describe absence
// testdata/conformance/standard_case_field_features_none/org.tsv:26-37,50-73 anonymous 62.0 67.0
// testdata/conformance/standard_case_field_features_liveagent/org.tsv:26-37,50-73 anonymous 62.0 67.0
// testdata/conformance/standard_case_field_features_knowledge/org.tsv:26-37,50-73 anonymous 62.0 67.0
// testdata/conformance/standard_case_field_features_knowledge_languages/org.tsv:26-37,50-73 anonymous 62.0 67.0
// testdata/conformance/standard_case_field_features_service_bundle/org.tsv:26-37,50-73 anonymous 62.0 67.0
// NATIVE Case.Division with the Division feature: read, SOQL, dynamic query and describe succeed; assignment does not compile; put throws
// testdata/conformance/standard_case_field_features_division/org.tsv:38-49 anonymous 62.0 67.0
// NATIVE Case.Division without the feature is absent
// testdata/conformance/standard_case_field_features_none/org.tsv:38-49 anonymous 62.0 67.0
func TestCaseFeatureFieldsNativeConformance(t *testing.T) {
	const base = "testdata/conformance/"
	captures := []string{
		"standard_case_field_features_none",
		"standard_case_field_features_liveagent",
		"standard_case_field_features_division",
		"standard_case_field_features_knowledge",
		"standard_case_field_features_knowledge_languages",
		"standard_case_field_features_service_bundle",
	}
	for _, capture := range captures {
		rows := readNativeTSV(t, filepath.Join(base, capture, "org.tsv"))
		for _, api := range []string{"62.0", "67.0"} {
			capture, api := capture, api
			runConformanceAPIVersion(t, capture+"/"+api, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				copyTree(t, filepath.Join(base, capture, "project-scratch-def.json"), filepath.Join(root, "config/project-scratch-def.json"), api)
				index := loadTestIndex(t, root)
				org := orgFromIndex(index)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				for _, row := range rows {
					id := row["id"]
					if row["api"] != api || !(strings.HasPrefix(id, "Language_") || strings.HasPrefix(id, "Connection") || strings.HasPrefix(id, "Division_")) {
						continue
					}
					raw, err := os.ReadFile(filepath.Join(base, capture, "cases", id+".apex"))
					if err != nil {
						t.Fatal(err)
					}
					body := string(raw)
					outcome := row["outcome"]
					analysis := sema.AnalyzeAnonymous(index, body, api)
					switch {
					case strings.HasPrefix(outcome, "CE|"):
						if !analysis.HasErrors() {
							t.Errorf("%s: native compile error %q; Glade compiled", id, outcome)
						}
						continue
					case analysis.HasErrors():
						t.Errorf("%s: native outcome %q; Glade compile errors %v", id, outcome, analysis.Diagnostics)
						continue
					}
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Errorf("%s: native outcome %q; Glade compile %v", id, outcome, err)
						continue
					}
					result, runErr := runner.execute(program)
					if strings.HasPrefix(outcome, "EX|") {
						want := strings.TrimPrefix(strings.SplitN(outcome, "\n", 2)[0], "EX|")
						want = strings.TrimPrefix(want, "System.")
						if runErr == nil || !strings.Contains(runErr.Error(), strings.SplitN(want, ".", 2)[0][:min(len(want), 40)]) {
							t.Errorf("%s: native %q; Glade error %v", id, outcome, runErr)
						}
						continue
					}
					if runErr != nil {
						t.Errorf("%s: native %q; Glade error %v", id, outcome, runErr)
						continue
					}
					if !strings.Contains(strings.Join(result.Debug, "\n"), strings.TrimPrefix(outcome, "")) {
						t.Errorf("%s: native %q; Glade debug %v", id, outcome, result.Debug)
					}
				}
			})
		}
	}
}
