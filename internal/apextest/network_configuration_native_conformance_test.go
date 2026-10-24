package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestNetworkConfigurationNativeProjectRoutes(t *testing.T) {
	const fixture = "testdata/conformance/network_configuration"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	reflection, err := os.Open(filepath.Join(fixture, "reflection-org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer reflection.Close()
	reflectionReader := csv.NewReader(reflection)
	reflectionReader.Comma = '\t'
	reflectionRows, err := reflectionReader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	rows = append(rows, reflectionRows[1:]...)
	for _, api := range []string{"62.0", "67.0"} {
		for _, orgSetup := range []string{"minimal", "custom_object"} {
			for _, shape := range []string{"disabled", "enabled", "package_metadata", "extra_metadata", "external_metadata", "scratch_metadata", "disabled_metadata"} {
				t.Run(api+"/"+orgSetup+"/"+shape, func(t *testing.T) {
					root := filepath.Join(t.TempDir(), "project")
					writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
					if orgSetup == "custom_object" {
						writeFile(t, filepath.Join(root, "force-app/main/default/objects/Synthetic_Item__c/Synthetic_Item__c.object-meta.xml"), `<CustomObject><label>Synthetic Item</label><pluralLabel>Synthetic Items</pluralLabel><nameField><type>Text</type><label>Name</label></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
					}
					configuration := "metadata"
					enabled := shape != "disabled" && shape != "disabled_metadata"
					if enabled {
						writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [Communities]\n")
					}
					if shape == "disabled" || shape == "disabled_metadata" {
						configuration = "disabled"
					}
					if shape == "enabled" {
						configuration = "enabled"
					}
					if strings.HasSuffix(shape, "metadata") {
						dir := "force-app"
						if shape == "extra_metadata" {
							dir = "supplemental"
						}
						if shape == "external_metadata" {
							dir = "../shared"
						}
						if dir != "force-app" {
							writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [Communities]\nproject:\n  extraMetadataDirs: ["+dir+", "+dir+"]\n")
						}
						if shape == "scratch_metadata" {
							writeFile(t, filepath.Join(root, "glade.yml"), "")
							writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"settings":{"communitiesSettings":{"enableNetworksEnabled":true}}}`)
						}
						metadata, err := os.ReadFile(filepath.Join(fixture, "Synthetic Portal.network-meta.xml"))
						if err != nil {
							t.Fatal(err)
						}
						writeFile(t, filepath.Join(root, dir, "main/default/networks/Synthetic Portal.network-meta.xml"), string(metadata))
					}
					names := []string{"NetworkObservation", "NetworkObservationTest"}
					observations := []string{"NetworkObservation"}
					if configuration != "metadata" {
						names = append(names, "NetworkReflectionObservation", "NetworkReflectionObservationTest")
						observations = append(observations, "NetworkReflectionObservation")
					}
					if enabled {
						names = append(names, "NetworkLiteralObservation", "NetworkLiteralObservationTest")
						observations = append(observations, "NetworkLiteralObservation")
					}
					for _, name := range names {
						source, err := os.ReadFile(filepath.Join(fixture, name+".cls"))
						if err != nil {
							t.Fatal(err)
						}
						path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
						writeFile(t, path, string(source))
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					}
					index := loadTestIndex(t, root)
					if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
						t.Fatalf("native declarations compile: %v %v", index.Diagnostics, analysis.Diagnostics)
					}
					for _, route := range []string{"anonymous", "istest"} {
						t.Run(route, func(t *testing.T) {
							got := map[string]string{}
							for _, name := range observations {
								if name == "NetworkLiteralObservation" && !enabled {
									continue
								}
								var message string
								if route == "anonymous" {
									org, err := orgFromIndexWithError(index)
									if err != nil {
										t.Fatal(err)
									}
									org.APIVersion = api
									runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
									program, err := vm.CompileAnonymousWithOptions("System.assert(false, 'NETWORK_OBS|' + "+name+".observe());", vm.CompileOptions{APIVersion: api})
									if err != nil {
										t.Fatal(err)
									}
									_, err = runner.newMachine().Execute(program)
									var failure *vm.RuntimeError
									if !errors.As(err, &failure) || !strings.HasSuffix(failure.Type, "AssertException") {
										t.Fatalf("missing anonymous observation: %v", err)
									}
									message = failure.ExceptionMessage()
								} else {
									run := Run(index, Options{SelectedClasses: []string{name + "Test"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
									if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
										t.Fatalf("unexpected test selection: %+v", run.Summary())
									}
									problem := run.Suites[0].Cases[0].Problem
									if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
										t.Fatalf("missing test observation: %+v", problem)
									}
									message = problem.Message
								}
								_, payload, ok := strings.Cut(message, "NETWORK_OBS|")
								var observed map[string]*string
								if !ok || json.Unmarshal([]byte(payload), &observed) != nil {
									t.Fatalf("invalid observation: %s", message)
								}
								for key, value := range observed {
									got[key] = "null"
									if value != nil {
										got[key] = *value
									}
								}
							}
							checked := 0
							for _, row := range rows[1:] {
								if len(row) != 5 {
									t.Fatalf("invalid native row: %v", row)
								}
								if row[0] != configuration || row[1] != api || row[2] != route {
									continue
								}
								checked++
								if value, ok := got[row[3]]; !ok || value != row[4] {
									t.Errorf("%s: native=%q Glade=%q (present=%v)", row[3], row[4], value, ok)
								}
							}
							if checked != len(got) {
								t.Fatalf("native coverage=%d, observed=%d", checked, len(got))
							}
						})
					}
				})
			}
		}
	}
}
