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

// Native rows cover ordinary custom-object updates. Namespace variants are
// local alias controls, not native namespaced-package proof.
func TestCheckboxUpdateLifecycleNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/checkbox_update_lifecycle"
	for _, api := range []string{"62.0", "67.0"} {
		file, err := os.Open(filepath.Join(fixture, "api"+strings.Split(api, ".")[0], "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(file)
		reader.Comma = '\t'
		rows, err := reader.ReadAll()
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 21 {
			t.Fatalf("expected twenty native rows at %s", api)
		}
		grouped := map[string][]string{}
		for _, row := range rows[1:] {
			if len(row) != 7 || row[0] != api || row[1] != "istest" {
				t.Fatalf("invalid native row: %v", row)
			}
			grouped[row[2]] = append(grouped[row[2]], row[3]+":"+strings.Join(row[4:], ","))
		}
		for _, ns := range []string{"", "parcel"} {
			scope := ns
			if scope == "" {
				scope = "unqualified"
			}
			for _, actor := range []string{"runningFull", "identityFull", "standardObject", "standardFull"} {
				if len(grouped[actor]) != 5 {
					t.Fatalf("missing native actor %s", actor)
				}
				t.Run(api+"/"+scope+"/"+actor, func(t *testing.T) {
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
						raw, err := os.ReadFile(path)
						if err != nil {
							return err
						}
						source := string(raw)
						if strings.HasSuffix(path, ".cls-meta.xml") {
							source = "<ApexClass><apiVersion>" + api + "</apiVersion></ApexClass>"
						}
						if strings.HasSuffix(path, "Admin.profile-meta.xml") {
							// Native metadata fullName Admin is the data profile System Administrator.
							relative = filepath.Join("force-app/main/default/profiles", "System Administrator.profile-meta.xml")
						}
						if strings.HasSuffix(path, "CheckboxLifecycleObservation.cls") {
							terminal := "System.assert(false,'LIFECYCLE|'+actor+'|'+String.join(rows,';')+'|END');"
							if strings.Count(source, terminal) != 1 {
								t.Fatal("missing native observation transport")
							}
							assertion := fmt.Sprintf("System.assertEquals(%s,String.join(rows,';'));", conformanceApexString(strings.Join(grouped[actor], ";")))
							source = strings.Replace(source, terminal, assertion, 1)
							if ns != "" {
								source = strings.ReplaceAll(source, ".get('LifecycleParcel__c')", ".get('"+ns+"__LifecycleParcel__c')")
								source = strings.ReplaceAll(source, "removed.contains('Enabled__c')", "removed.contains('"+ns+"__Enabled__c')")
							}
						}
						writeFile(t, filepath.Join(root, relative), source)
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"namespace":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api, ns))
					result := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"CheckboxLifecycleObservation"}, SelectedMethod: actor, NoDiskCache: true})
					if summary := result.Summary(); summary.Total != 1 || summary.Passed != 1 {
						raw, _ := json.Marshal(result)
						t.Fatalf("native checkbox update mismatch: %s", raw)
					}
				})
			}
		}
	}
}
