package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Test the captured inserts through the test runner, including both DML forms.
func TestEmailInsertValidationRoutesNativeConformance(t *testing.T) {
	base := filepath.Join("..", "dml", "testdata", "email_insert_validation")
	f, err := os.Open(filepath.Join(base, "org.tsv"))
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
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			metadata, err := os.ReadFile(filepath.Join(base, "force-app/main/default/objects/Contact/fields/ProbeEmail__c.field-meta.xml"))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/Contact/fields/ProbeEmail__c.field-meta.xml"), string(metadata))
			anonymous := make(map[int]string)
			for _, group := range []int{1, 3} {
				var expected []string
				start := group * 6
				for i := start; i < start+6; i++ {
					for k, field := range []string{"Contact.Email", "Lead.Email", "Contact.ProbeEmail__c"} {
						for mode, operation := range []string{"Database.insert(false)", "insert"} {
							// Capture rows are ordered by value; each first repeat is a route twin.
							index := 1 + i*6 + k*2 + mode
							row := rows[index]
							if row[0] != "62.0" || row[1] != "anonymous" || row[2] != "1" || row[4] != field || row[5] != operation {
								t.Fatalf("unexpected native row %d: %v", index, row)
							}
							expected = append(expected, fmt.Sprintf("%d:%d:%d:%s", i, k, mode, row[6]))
						}
					}
				}
				anon, err := os.ReadFile(filepath.Join(base, fmt.Sprintf("probe%d.apex", group)))
				if err != nil {
					t.Fatal(err)
				}
				anonymous[group] = strings.Replace(string(anon), "System.assert(false,'OBSBEGIN'+payload+'OBSEND');", "System.assertEquals("+conformanceApexString(strings.Join(expected, "|"))+",payload);", 1)
				name := fmt.Sprintf("EmailProbe%d", group)
				contents, err := os.ReadFile(filepath.Join(base, "force-app/main/default/classes", name+".cls"))
				if err != nil {
					t.Fatal(err)
				}
				source := strings.Replace(string(contents), "System.assert(false,'OBSBEGIN'+payload+'OBSEND');", "System.assertEquals("+conformanceApexString(strings.Join(expected, "|"))+",payload);", 1)
				if source == string(contents) {
					t.Fatal("missing assertion marker")
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls"), source)
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			}
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, group := range []int{1, 3} {
				source := anonymous[group]
				analysis := sema.AnalyzeAnonymous(index, source, api)
				program, compileErr := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if analysis.HasErrors() || compileErr != nil {
					t.Fatalf("anonymous compilation: %v %v", compileErr, analysis.Diagnostics)
				}
				if _, err := runner.execute(program); err != nil {
					t.Errorf("anonymous group %d: %v", group, err)
				}
			}
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1, SelectedClasses: []string{"EmailProbe1", "EmailProbe3"}})
			summary := run.Summary()
			if summary.Total != 2 || summary.Passed != 2 {
				t.Fatalf("unexpected results: %#v: %s", summary, firstRunProblem(run))
			}
		})
	}
}
