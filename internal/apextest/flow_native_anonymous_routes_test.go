package apextest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// The native captures use @IsTest. These twins compare the same observations
// through anonymous execution, without claiming an additional native capture.
func TestFlowNativeAnonymousRouteTwins(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, family := range []string{"flow_decision_criteria_native", "flow_related_fields", "generated_soql_relationships", "flow_subflow_record_lookup"} {
			t.Run(api+"/"+family+"/anonymous", func(t *testing.T) {
				fixture := filepath.Join("testdata/conformance", family)
				source := filepath.Join(fixture, "native", "api"+api[:2])
				class, marker := "OwnedRelationshipProbe"+api[:2], "OBSERVED|flowRelatedFields|"
				invocation := class + ".observe();"
				if family == "flow_decision_criteria_native" {
					source = filepath.Join(fixture, "project")
					class, marker = "CedarDecisionCriteriaProbeTest", "OBSERVED|decisionCriteria|"
					invocation = class + ".observe();"
				} else if family == "generated_soql_relationships" {
					class, marker = "OwnedGeneratedQueryProbe", "OBSERVED|generatedSOQLRelationship|"
					invocation = class + ".observe();"
				} else if family == "flow_subflow_record_lookup" {
					source = filepath.Join(fixture, "api"+api[:2])
					class, marker = "WillowLookupProbe", "WILLOW_RESULT|"
					invocation = "System.assert(false,'WILLOW_RESULT|'+JSON.serialize(WillowLookupProbe.observe()));"
				}
				root := t.TempDir()
				err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if entry.IsDir() {
						if entry.Name() == "native" || strings.HasPrefix(entry.Name(), ".") {
							return filepath.SkipDir
						}
						return nil
					}
					body, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					name, err := filepath.Rel(source, path)
					if err != nil {
						return err
					}
					text := strings.ReplaceAll(string(body), "67.0", api)
					if filepath.Base(path) == class+".cls" && family != "flow_subflow_record_lookup" {
						text = strings.ReplaceAll(text, "@IsTest static void observe()", "public static void observe()")
						text = strings.ReplaceAll(text, "@IsTest", "")
						text = strings.Replace(text, "private class "+class, "public class "+class, 1)
					}
					writeFile(t, filepath.Join(root, name), text)
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				index := loadTestIndex(t, root)
				if analysis := sema.AnalyzeAnonymous(index, invocation, api); analysis.HasErrors() {
					t.Fatal(analysis.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(invocation, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				_, err = runner.execute(program)
				var runtimeErr *vm.RuntimeError
				if !errors.As(err, &runtimeErr) {
					t.Fatalf("missing observation: %v", err)
				}
				_, payload, found := strings.Cut(runtimeErr.ExceptionMessage(), marker)
				got := map[string]any{}
				if !found || json.Unmarshal([]byte(payload), &got) != nil {
					t.Fatalf("invalid observation: %v", err)
				}
				want := map[string]any{}
				table := filepath.Join(fixture, "org.tsv")
				if family == "flow_subflow_record_lookup" {
					table = filepath.Join(source, "native/org.tsv")
				}
				raw, err := os.ReadFile(table)
				if err != nil {
					t.Fatal(err)
				}
				for _, row := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
					columns := strings.Split(row, "\t")
					switch family {
					case "flow_decision_criteria_native":
						if columns[0] == api && columns[2] != "DeleteLookupTitleContains" && columns[2] != "TitleWhitespaceFieldReadback" {
							want[columns[2]] = columns[3]
						}
					case "flow_subflow_record_lookup":
						var value string
						if err := json.Unmarshal([]byte(columns[1]), &value); err != nil {
							t.Fatal(err)
						}
						want[columns[0]] = value
					default:
						if columns[0] == api {
							if err := json.Unmarshal([]byte(columns[2]), &want); err != nil {
								t.Fatal(err)
							}
						}
					}
				}
				if len(want) == 0 {
					t.Fatal("empty native selection")
				}
				for name, expected := range want {
					actual := got[name]
					if family == "flow_subflow_record_lookup" {
						actual = normalizeFlowLookupObservation(fmt.Sprint(actual))
					}
					if !reflect.DeepEqual(actual, expected) {
						t.Errorf("%s: native=%v; anonymous=%v", name, expected, actual)
					}
				}
			})
		}
	}
}
