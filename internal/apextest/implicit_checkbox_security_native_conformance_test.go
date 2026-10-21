package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Native rows distinguish omitted defaults from explicit and queried checkbox
// values. Namespace variants are local alias controls, not native package proof.
func TestImplicitCheckboxSecurityNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/ordinary_custom_permissions"
	for _, api := range []string{"62.0", "67.0"} {
		raw, err := os.Open(filepath.Join(fixture, "api"+strings.Split(api, ".")[0], "defaults.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(raw)
		reader.Comma = '\t'
		rows, err := reader.ReadAll()
		raw.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 10 {
			t.Fatalf("API %s: expected nine native rows", api)
		}
		for _, namespace := range []string{"", "parcel"} {
			label := namespace
			if label == "" {
				label = "unqualified"
			}
			for ordinal, row := range rows[1:] {
				if len(row) != 6 || row[0] != api || row[1] != "istest" {
					t.Fatalf("invalid native row: %v", row)
				}
				t.Run(api+"/"+label+"/"+row[2], func(t *testing.T) {
					root := t.TempDir()
					err := filepath.WalkDir(filepath.Join(fixture, "force-app"), func(path string, entry os.DirEntry, err error) error {
						if err != nil {
							return err
						}
						if entry.IsDir() {
							return nil
						}
						relative, err := filepath.Rel(fixture, path)
						if err != nil {
							return err
						}
						data, err := os.ReadFile(path)
						if err != nil {
							return err
						}
						if strings.HasSuffix(path, ".cls-meta.xml") {
							data = []byte("<ApexClass><apiVersion>" + api + "</apiVersion></ApexClass>")
						}
						if strings.HasSuffix(path, "ParcelPermissionObservation.cls") {
							terminal := "System.assert(false,'DEFAULT_ROWS|'+String.join(rows,';')+'|END');"
							if strings.Count(string(data), terminal) != 1 {
								t.Fatal("missing native observation transport")
							}
							want := row[2] + ":" + strings.Join(row[3:], ",")
							assertion := fmt.Sprintf("System.assertEquals('%s',rows[%d]);", want, ordinal)
							data = []byte(strings.Replace(string(data), terminal, assertion, 1))
							if namespace != "" {
								data = []byte(strings.ReplaceAll(string(data), "fields.contains('Enabled__c')", "fields.contains('"+namespace+"__Enabled__c')"))
							}
						}
						writeFile(t, filepath.Join(root, relative), string(data))
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
					project, _ := json.Marshal(map[string]any{"sourceApiVersion": api, "namespace": namespace, "packageDirectories": []map[string]any{{"path": "force-app", "default": true}}})
					writeFile(t, filepath.Join(root, "sfdx-project.json"), string(project))
					run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"ParcelPermissionObservation"}, SelectedMethod: "observeDefaults", NoDiskCache: true})
					if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
						data, _ := json.Marshal(run)
						t.Fatalf("native checkbox security mismatch: %s", data)
					}
				})
			}
		}
	}
}
