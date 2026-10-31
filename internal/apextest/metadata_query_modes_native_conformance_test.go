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

func TestMetadataQueryModesNativeConformance(t *testing.T) {
	const evidence = "testdata/conformance/metadata_query_modes"
	f, err := os.Open(filepath.Join(evidence, "org.tsv"))
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
	if len(rows) != 21 {
		t.Fatalf("native rows: %d", len(rows))
	}
	expected := map[string]string{}
	for _, row := range rows[1:] {
		if len(row) != 4 {
			t.Fatalf("invalid row: %v", row)
		}
		key := strings.Join(row[:3], "/")
		if _, exists := expected[key]; exists {
			t.Fatalf("duplicate: %s", key)
		}
		expected[key] = row[3]
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			sourceRoot := "testdata/conformance/metadata_field_names/fixture"
			err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					return nil
				}
				rel, err := filepath.Rel(sourceRoot, path)
				if err != nil {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				text := string(data)
				if strings.HasSuffix(path, ".cls-meta.xml") {
					text = fmt.Sprintf("<ApexClass><apiVersion>%s</apiVersion><status>Active</status></ApexClass>", api)
				}
				writeFile(t, filepath.Join(root, rel), text)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					get := func(id string) string {
						v, ok := expected[api+"/"+route+"/"+id]
						if !ok {
							t.Fatalf("missing row %s", id)
						}
						return v
					}
					body := fmt.Sprintf(`String n = null; String b = '';
System.assertEquals(%s, [SELECT count() FROM MeadowGroup__mdt WHERE NamespacePrefix = :n], 'null-filter');
System.assertEquals(%s, [SELECT count() FROM MeadowGroup__mdt WHERE NamespacePrefix = :b], 'blank-filter');
System.assertEquals(%s, [SELECT count() FROM MeadowGroup__mdt WHERE DeveloperName = 'Missing'], 'missing-filter');
List<MeadowGroup__mdt> rows = [SELECT DeveloperName, Descriptor__r.Column__r.QualifiedApiName, Descriptor__r.Entity__r.QualifiedApiName, (SELECT Value__c FROM Choices__r) FROM MeadowGroup__mdt WHERE Descriptor__r.Entity__r.QualifiedApiName = 'MeadowEntry__c' WITH USER_MODE];
System.assertEquals('%s', rows.size() + ':' + rows[0].Descriptor__r.Column__r.QualifiedApiName + ':' + rows[0].Choices__r.size(), 'user-mode');
System.assertEquals(%s, [SELECT count() FROM MeadowGroup__mdt WHERE DeveloperName = 'Missing' WITH USER_MODE], 'user-mode-missing');`, get("null-filter"), get("blank-filter"), get("missing-filter"), get("user-mode"), get("user-mode-missing"))
					if route == "istest" {
						writeFile(t, filepath.Join(root, "force-app/main/default/classes/MeadowNativeTest.cls"), "@IsTest private class MeadowNativeTest { @IsTest static void verifyRows(){"+body+"}}")
						run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
						if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
							t.Fatalf("native named assertions: summary=%+v problem=%s", got, firstRunProblem(run))
						}
						return
					}
					index := loadTestIndex(t, root)
					if result := sema.AnalyzeAnonymous(index, body, api); result.HasErrors() {
						t.Fatalf("native anonymous accepted: %+v", result.Diagnostics)
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
		})
	}
}
