package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

type personAccountDependencyLayout struct {
	Name      string
	Enabled   bool
	Call      string
	Rejection string
	Files     map[string]string
}

func loadPersonAccountDependencyLayouts(t *testing.T) ([]personAccountDependencyLayout, string) {
	t.Helper()
	const dir = "testdata/conformance/person_account_evidence"
	raw, err := os.ReadFile(filepath.Join(dir, "dependency_layouts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var layouts []personAccountDependencyLayout
	if err := json.Unmarshal(raw, &layouts); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "observe.apex"))
	if err != nil {
		t.Fatal(err)
	}
	return layouts, string(body)
}

// writePersonAccountDependencyLayout writes the workspace and returns the
// consumer root. Every class receives a metadata sidecar at the source API.
func writePersonAccountDependencyLayout(t *testing.T, root string, layout personAccountDependencyLayout, api, body string) string {
	t.Helper()
	for path, source := range layout.Files {
		source = strings.ReplaceAll(strings.ReplaceAll(source, "{{API}}", api), "{{OBSERVE}}", body)
		full := filepath.Join(root, filepath.FromSlash(path))
		writeFile(t, full, source)
		if strings.HasSuffix(path, ".cls") {
			writeFile(t, full+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
		}
	}
	consumer := filepath.Join(root, "consumer")
	// Keep the retrieved consumer's explicit Account schema in every route.
	writeFile(t, filepath.Join(consumer, "force-app/main/default/objects/Account/Account.object-meta.xml"), `<CustomObject><fields><fullName>Name</fullName><type>Text</type></fields></CustomObject>`)
	return consumer
}

// The org shape is the effective project's: the consumer plus every declared
// dependency view deployed into the same org. A dependency that carries Person
// Account evidence (a retrieved PersonAccount record type, the default
// Account.PersonAccount identity, or a PersonAccounts scratch feature) enables
// the native person_account_evidence behaviour for the consumer's routes; with
// no evidence anywhere the person_account_without_feature rejection stands.
func TestPersonAccountDependencyEvidenceProjectRoutes(t *testing.T) {
	layouts, body := loadPersonAccountDependencyLayouts(t)
	const local = "Account local = new Account(FirstName='C',LastName='D');System.assertEquals('D',local.LastName);"
	for _, api := range []string{"62.0", "67.0"} {
		for _, layout := range layouts {
			for _, route := range []string{"check", "anonymous", "named", "test-runner"} {
				t.Run(api+"/"+layout.Name+"/"+route, func(t *testing.T) {
					consumer := writePersonAccountDependencyLayout(t, t.TempDir(), layout, api, body)
					const class = "PersonEvidenceConsumer"
					anonymous := layout.Call + local
					source := "public class " + class + " { public static void sample(){" + anonymous + "} }"
					if route == "test-runner" {
						source = "@IsTest private class " + class + " { @IsTest static void sample(){" + anonymous + "} }"
					}
					if route != "anonymous" {
						path := filepath.Join(consumer, "force-app/main/default/classes", class+".cls")
						writeFile(t, path, source)
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					}
					index := loadTestIndex(t, consumer)
					if index.HasErrors() {
						t.Fatal(index.Diagnostics)
					}
					hasFeature := false
					for _, feature := range index.OrgShapeFeatures {
						hasFeature = hasFeature || feature == "PersonAccounts"
					}
					if hasFeature != layout.Enabled {
						t.Errorf("index features=%v, want PersonAccounts=%t", index.OrgShapeFeatures, layout.Enabled)
					}
					analysis := sema.Analyze(index)
					if route == "anonymous" {
						analysis = sema.AnalyzeAnonymous(index, anonymous, api)
					}
					var rejections []diagnostic.Diagnostic
					for _, d := range analysis.Diagnostics {
						if strings.Contains(d.Message+" "+d.NativeMessage, "FirstName") {
							rejections = append(rejections, d)
						}
					}
					if layout.Enabled && (analysis.HasErrors() || len(rejections) > 0) {
						t.Fatalf("enabled; compilation diagnostics=%v", analysis.Diagnostics)
					}
					if !layout.Enabled {
						// A loaded source-backed dependency downgrades project
						// class semantic errors to warnings (existing check
						// policy); anonymous Apex and projects without one keep
						// the native rejection as an error.
						want := diagnostic.Error
						if layout.Rejection == "warning" && route != "anonymous" {
							want = diagnostic.Warning
						}
						if len(rejections) == 0 || rejections[0].Severity != want {
							t.Fatalf("expected native FirstName rejection as %s, got %v", want, analysis.Diagnostics)
						}
						if route == "test-runner" {
							run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
							if run.Summary().OK() {
								t.Fatalf("missing runner rejection: %+v", run.Summary())
							}
						}
						return
					}
					if route == "check" {
						return
					}
					if route == "test-runner" {
						for _, state := range []string{"cold", "reused"} {
							t.Run(state, func(t *testing.T) {
								run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
								if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Status != testreport.StatusPass {
									t.Fatalf("runner: %+v %+v", run.Summary(), run.Suites)
								}
							})
						}
						return
					}
					programSource := anonymous
					if route == "named" {
						programSource = class + ".sample();"
					}
					program, err := vm.CompileAnonymousWithOptions(programSource, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					if _, err := runner.execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

// Every feature-gated field set follows the effective project, not only
// PersonAccounts: a dependency's scratch definition enables MultiCurrency,
// StateAndCountryPicklist and PersonAccounts fields in the runtime schema.
func TestDependencyScratchFeaturesReachRuntimeSchema(t *testing.T) {
	gated := map[string]string{"CurrencyIsoCode": "MultiCurrency", "BillingStateCode": "StateAndCountryPicklist", "IsPersonAccount": "PersonAccounts"}
	for _, declared := range []bool{false, true} {
		name := "absent"
		if declared {
			name = "dependency"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			consumer := filepath.Join(root, "consumer")
			base := filepath.Join(root, "base")
			writeFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"ShapeBase"}]}]}`)
			writeFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  dependencyPaths: [\"../base\"]\norg:\n  features: []\n")
			writeFile(t, filepath.Join(consumer, "force-app/main/default/classes/ShapeConsumer.cls"), "public class ShapeConsumer {}")
			writeFile(t, filepath.Join(consumer, "force-app/main/default/classes/ShapeConsumer.cls-meta.xml"), "<ApexClass><apiVersion>67.0</apiVersion></ApexClass>")
			writeFile(t, filepath.Join(base, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"base-app","default":true,"package":"ShapeBase"}]}`)
			writeFile(t, filepath.Join(base, "base-app/main/default/classes/ShapeBase.cls"), "public class ShapeBase {}")
			writeFile(t, filepath.Join(base, "base-app/main/default/classes/ShapeBase.cls-meta.xml"), "<ApexClass><apiVersion>67.0</apiVersion></ApexClass>")
			if declared {
				writeFile(t, filepath.Join(base, "config/project-scratch-def.json"), `{"features":["MultiCurrency","StateAndCountryPicklist","PersonAccounts"]}`)
			}
			index := loadTestIndex(t, consumer)
			org := orgFromIndex(index)
			account := org.Objects["Account"].Definition
			for field, feature := range gated {
				hasFeature := false
				for _, candidate := range index.OrgShapeFeatures {
					hasFeature = hasFeature || candidate == feature
				}
				_, hasField := storage.ResolveFieldName(account, "", field)
				if hasFeature != declared || hasField != declared {
					t.Errorf("%s: feature %s=%t field=%t, want %t (features=%v)", field, feature, hasFeature, hasField, declared, index.OrgShapeFeatures)
				}
			}
		})
	}
}
