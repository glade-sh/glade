package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Non-profile row values are stable across the captured API and route matrix.
func TestPermissionSetInventoryNativeSOQLRoutes(t *testing.T) {
	f, err := os.Open("testdata/conformance/permission_set_inventory/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma, r.LazyQuotes = '\t', true
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	var body strings.Builder
	body.WriteString(`Map<String,PermissionSet> sets=new Map<String,PermissionSet>();
for(PermissionSet p:[SELECT Name,Label,Type,NamespacePrefix,IsCustom,IsOwnedByProfile,LicenseId,PermissionsApiUserOnly,PermissionsModifyAllData,PermissionsViewAllData FROM PermissionSet WHERE IsOwnedByProfile=false]) sets.put(p.Name,p);
System.assertEquals(50,sets.size());`)
	count := 0
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "\\'") + "'" }
	for _, row := range rows[1:] {
		if row[2] != "62.0" || row[3] != "anonymous" || row[5] != "queryRow" {
			continue
		}
		var native struct {
			Query  string `json:"queryId"`
			Record struct {
				Name, Label, Type string
				Namespace         *string `json:"NamespacePrefix"`
				Owned             bool    `json:"IsOwnedByProfile"`
				Custom            bool    `json:"IsCustom"`
				API               bool    `json:"PermissionsApiUserOnly"`
				Modify            bool    `json:"PermissionsModifyAllData"`
				View              bool    `json:"PermissionsViewAllData"`
			} `json:"record"`
		}
		if err := json.Unmarshal([]byte(row[8]), &native); err != nil {
			t.Fatal(err)
		}
		v := native.Record
		if native.Query != "all" || v.Owned {
			continue
		}
		count++
		ref := "sets.get(" + quote(v.Name) + ")"
		fmt.Fprintf(&body, "System.assertEquals(%s,%s.Label);System.assertEquals(%s,%s.Type);System.assertEquals(%t,%s.IsCustom);System.assertEquals(%t,%s.PermissionsApiUserOnly);System.assertEquals(%t,%s.PermissionsModifyAllData);System.assertEquals(%t,%s.PermissionsViewAllData);System.assertNotEquals(null,%s.LicenseId);", quote(v.Label), ref, quote(v.Type), ref, v.Custom, ref, v.API, ref, v.Modify, ref, v.View, ref, ref)
		namespace := "null"
		if v.Namespace != nil {
			namespace = quote(*v.Namespace)
		}
		fmt.Fprintf(&body, "System.assertEquals(%s,%s.NamespacePrefix);", namespace, ref)
	}
	if count != 50 {
		t.Fatalf("native non-profile rows=%d", count)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			classes := filepath.Join(root, "force-app/main/default/classes")
			writeFile(t, filepath.Join(classes, "InventoryNativeTest.cls"), "@IsTest private class InventoryNativeTest { @IsTest static void queryRows(){"+body.String()+"} }")
			writeFile(t, filepath.Join(classes, "InventoryNativeTest.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api
			program, err := vm.CompileAnonymousWithOptions(body.String(), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if _, err := runner.execute(program); err != nil {
				t.Errorf("anonymous: %v", err)
			}
			for _, phase := range []string{"cold", "warm"} {
				run := Run(index, Options{SelectedClasses: []string{"InventoryNativeTest"}, Parallelism: 1})
				if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
					t.Errorf("%s test route: %+v suites=%+v", phase, summary, run.Suites)
				}
			}
		})
	}
}
