package apextest

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestDuplicateRuleMetadataSnapshotSourceAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, relative := range []string{
			"matchingRules/Contact.matchingRule-meta.xml",
			"matchingRules/PersonAccount.matchingRule-meta.xml",
			"duplicateRules/Contact.Grove_Email_Check.duplicateRule-meta.xml",
			"duplicateRules/PersonAccount.Grove_PersonEmail_Check.duplicateRule-meta.xml",
		} {
			t.Run(api+"/"+relative, func(t *testing.T) {
				root := t.TempDir()
				fixture := filepath.Join("testdata", "conformance", "setup_metadata", "project")
				if err := filepath.WalkDir(fixture, func(path string, entry fs.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					name, err := filepath.Rel(fixture, path)
					if err != nil {
						return err
					}
					body, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				index := loadTestIndex(t, root)
				sources := newSourceCache()
				generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
				if err := sources.captureProjectRuntimeMetadata(index, &generation); err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(root, "force-app/main/default", relative)
				original, err := os.ReadFile(target)
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, target, string(original)+"\n")
				captured, err := sources.readCapturedProjectMetadata(target)
				if err != nil || !bytes.Equal(captured, original) {
					t.Fatalf("rule snapshot lost deployed metadata after disk edit: %v", err)
				}
				if err := sources.validateCapturedSourceGenerationRaw(); err == nil {
					t.Fatal("rule metadata edit did not invalidate the captured generation")
				}
			})
		}
	}
}

func copyDuplicateRuleProject(t *testing.T, api string) string {
	t.Helper()
	root := t.TempDir()
	fixture := filepath.Join("testdata", "conformance", "setup_metadata", "project")
	err := filepath.WalkDir(fixture, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(fixture, path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, relative), strings.ReplaceAll(string(body), "62.0", api))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDuplicateRuleMetadataRuntimeAnonymousAndIsTestAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, mode := range []string{"live", "captured"} {
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(api+"/"+mode+"/"+route, func(t *testing.T) {
					root := copyDuplicateRuleProject(t, api)
					index := loadTestIndex(t, root)
					if result := sema.Analyze(index); index.HasErrors() || result.HasErrors() {
						t.Fatalf("native project diagnostics: %v %v", index.Diagnostics, result.Diagnostics)
					}
					sources := newSourceCache()
					if mode == "captured" {
						generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
						if err := sources.captureProjectRuntimeMetadata(index, &generation); err != nil {
							t.Fatal(err)
						}
						for _, relative := range []string{"matchingRules/Contact.matchingRule-meta.xml", "duplicateRules/Contact.Grove_Email_Check.duplicateRule-meta.xml"} {
							path := filepath.Join(root, "force-app/main/default", relative)
							body, err := os.ReadFile(path)
							if err != nil {
								t.Fatal(err)
							}
							body = bytes.ReplaceAll(body, []byte("<fieldName>Email</fieldName>"), []byte("<fieldName>LastName</fieldName>"))
							body = bytes.ReplaceAll(body, []byte("<isActive>true</isActive>"), []byte("<isActive>false</isActive>"))
							writeFile(t, path, string(body))
						}
					}
					org, err := orgFromIndexWithError(index, sources)
					if err != nil {
						t.Fatal(err)
					}
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					rows, err := os.ReadFile(filepath.Join("testdata/conformance/setup_metadata", map[string]string{"anonymous": "anonymous/org.tsv", "istest": "org.tsv"}[route]))
					if err != nil {
						t.Fatal(err)
					}
					for _, row := range strings.Split(strings.TrimSpace(string(rows)), "\n")[1:] {
						cells := strings.SplitN(row, "\t", 3)
						if cells[0] != api || cells[1] == "businessHours" {
							continue
						}
						method := cells[1]
						var want, got map[string]any
						if err := json.Unmarshal([]byte(cells[2]), &want); err != nil {
							t.Fatal(err)
						}
						var executionError error
						if route == "anonymous" {
							source, err := os.ReadFile(filepath.Join("testdata/conformance/setup_metadata/anonymous", method+".apex"))
							if err != nil {
								t.Fatal(err)
							}
							program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							_, executionError = runner.newMachine().Execute(program)
						} else {
							cases := Discover(index, Options{SelectedClasses: []string{"SetupMetadataTest"}, SelectedMethod: method})
							if len(cases) != 1 {
								t.Fatalf("discovered %d methods", len(cases))
							}
							methods, methodErrors := compileTestMethodsForIndex(&index, cases)
							programs, programErrors := compileTestInvokePrograms(cases)
							key := testCaseKey(cases[0])
							if methodErrors[key] != nil || programErrors[key] != nil {
								t.Fatalf("compile %v %v", methodErrors[key], programErrors[key])
							}
							machine := runner.newMachine()
							machine.EnableTestContext()
							if err := machine.RegisterMethod(methods[key]); err != nil {
								t.Fatal(err)
							}
							_, executionError = machine.ExecuteTestInClass(programs[key], cases[0].ClassName)
						}
						var fault *vm.RuntimeError
						if !errors.As(executionError, &fault) {
							t.Fatalf("missing observation: %v", executionError)
						}
						_, payload, found := strings.Cut(fault.ExceptionMessage(), "OBSERVED|"+method+"|")
						if !found || json.Unmarshal([]byte(payload), &got) != nil || !reflect.DeepEqual(got, want) {
							t.Errorf("%s: native=%v local=%s", method, want, fault.ExceptionMessage())
						}
					}
					for _, object := range []string{"Contact", "Account"} {
						body, err := json.Marshal(org.Objects[object].Definition)
						if err != nil {
							t.Fatal(err)
						}
						var definition struct {
							Rules []struct {
								ID         string
								Name       string
								Active     bool
								MatchRules []struct{ Items []struct{ Field string } }
							} `json:"duplicateRules"`
						}
						if err := json.Unmarshal(body, &definition); err != nil {
							t.Fatal(err)
						}
						// Native duplicate enforcement MD01: the org's default rules
						// remain beside the deployed rule.
						deployed := definition.Rules[:0]
						for _, rule := range definition.Rules {
							if !strings.HasPrefix(rule.Name, "Standard_") {
								deployed = append(deployed, rule)
							}
						}
						definition.Rules = deployed
						if len(definition.Rules) != 1 || !definition.Rules[0].Active || len(definition.Rules[0].MatchRules) != 1 || len(definition.Rules[0].MatchRules[0].Items) != 1 {
							t.Errorf("%s runtime lost deployed duplicate/matching definitions: rules=%d", object, len(definition.Rules))
							continue
						}
						rule := definition.Rules[0]
						record, exists := org.Objects["DuplicateRule"].Records[storage.ID(rule.ID)]
						if !exists || record.Fields["DeveloperName"].String != rule.Name || record.Fields["SobjectType"].String != object {
							t.Errorf("%s definition does not reference its queryable DuplicateRule row", object)
						}
						want := map[string]string{"Contact": "Email", "Account": "PersonEmail"}[object]
						if got := definition.Rules[0].MatchRules[0].Items[0].Field; got != want {
							t.Errorf("%s: runtime matching field=%s want %s", object, got, want)
						}
					}
				})
			}
		}
	}
}
