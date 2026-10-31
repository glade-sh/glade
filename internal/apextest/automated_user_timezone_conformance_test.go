package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Both API floors and routes use the first captured automated-user zone.
func TestAutomatedUserTimezoneNativeRoutes(t *testing.T) {
	f, err := os.Open("../storage/testdata/automated_user_timezone/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if row[2] != "automated1" {
			continue
		}
		var observed []string
		if err := json.Unmarshal([]byte(row[3]), &observed); err != nil || len(observed) != 2 {
			t.Fatalf("bad native row %v: %v", row, err)
		}
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, row[0]))
			body := "List<User> u=[SELECT TimeZoneSidKey FROM User WHERE UserType='AutomatedProcess']; System.assertEquals(1,u.size()); System.assertEquals(" + conformanceApexString(observed[1]) + ",u[0].TimeZoneSidKey);"
			classPath := filepath.Join(root, "force-app/main/default/classes/AutomatedZoneTest.cls")
			writeFile(t, classPath, "@IsTest(SeeAllData=true) private class AutomatedZoneTest { @IsTest static void check(){"+body+"} }")
			writeFile(t, classPath+"-meta.xml", "<ApexClass><apiVersion>"+row[0]+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			if row[1] == "istest" {
				result := Run(index, Options{SelectedClasses: []string{"AutomatedZoneTest"}, NoDiskCache: true})
				if s := result.Summary(); s.Total != 1 || s.Passed != 1 {
					data, _ := json.Marshal(result)
					t.Fatalf("test result: %s", data)
				}
				return
			}
			program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = row[0]
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if _, err := runner.execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
