package apextest

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestMetadataNamespaceRelationshipNativeConformance(t *testing.T) {
	testMetadataNamespaceRelationshipRoutes(t, false)
}

// Foreign values preserve the shipping-parent local fallback; the owned
// controls continue to come from the adjacent native capture.
func TestMetadataForeignRelationshipProjectRoutes(t *testing.T) {
	testMetadataNamespaceRelationshipRoutes(t, true)
}

func testMetadataNamespaceRelationshipRoutes(t *testing.T, foreign bool) {
	const capture = "testdata/conformance/metadata_namespace_relationship"
	f, err := os.Open(filepath.Join(capture, "org.tsv"))
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
	expected := map[string]string{}
	for _, row := range rows[1:] {
		expected[strings.Join(row[:3], "/")] = row[3]
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				src := "testdata/conformance/metadata_field_names/fixture"
				err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if entry.IsDir() {
						if strings.HasPrefix(entry.Name(), ".") {
							return filepath.SkipDir
						}
						return nil
					}
					rel, err := filepath.Rel(src, path)
					if err != nil {
						return err
					}
					data, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					if foreign && entry.Name() == "MeadowField.CustomState.md-meta.xml" {
						text := strings.ReplaceAll(string(data), "MeadowEntry__c.State__c", "other__MeadowEntry__c.other__State__c")
						text = strings.ReplaceAll(text, ">MeadowEntry__c<", ">other__MeadowEntry__c<")
						data = []byte(text)
					}
					if strings.HasSuffix(path, ".cls-meta.xml") {
						data = []byte(fmt.Sprintf("<ApexClass><apiVersion>%s</apiVersion><status>Active</status></ApexClass>", api))
					}
					writeFile(t, filepath.Join(root, rel), string(data))
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"namespace":"gladens004","sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				probe, err := os.ReadFile(filepath.Join(capture, "probe.cls"))
				if err != nil {
					t.Fatal(err)
				}
				classDir := filepath.Join(root, "force-app/main/default/classes")
				writeFile(t, filepath.Join(classDir, "Road215NamespaceProbe.cls"), string(probe))
				writeFile(t, filepath.Join(classDir, "Road215NamespaceProbe.cls-meta.xml"), fmt.Sprintf("<ApexClass><apiVersion>%s</apiVersion><status>Active</status></ApexClass>", api))
				var assertions strings.Builder
				assertions.WriteString("Map<String,Object> got = Road215NamespaceProbe.observe();\n")
				for _, id := range []string{"objectDescribe", "fieldDescribe", "fieldRelationship", "objectRelationship", "standardFieldRelationship", "matchingGroups", "missing"} {
					want, ok := expected[api+"/"+route+"/"+id]
					if !ok {
						t.Fatal("missing native row: ", id)
					}
					if foreign {
						switch id {
						case "objectRelationship":
							want = "other__MeadowEntry__c"
						case "fieldRelationship":
							want = "other__State__c"
						case "matchingGroups":
							want = "0"
						}
					}
					fmt.Fprintf(&assertions, "System.assertEquals('%s', String.valueOf(got.get('%s')), '%s');\n", want, id, id)
				}
				if route == "istest" {
					writeFile(t, filepath.Join(classDir, "MeadowNativeTest.cls"), "@IsTest private class MeadowNativeTest { @IsTest static void verifyRows(){"+assertions.String()+"}}")
					run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
					if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
						t.Fatalf("native named assertions: summary=%+v problem=%s", got, firstRunProblem(run))
					}
					return
				}
				index := loadTestIndex(t, root)
				body := assertions.String()
				if result := sema.AnalyzeAnonymous(index, body, api); result.HasErrors() {
					t.Fatalf("compile: %+v", result.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				if _, err := runner.execute(program); err != nil {
					t.Fatalf("native anonymous assertions: %v", err)
				}
			})
		}
	}
}
