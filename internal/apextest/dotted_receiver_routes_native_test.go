package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDottedReceiverProjectCompileGateNamedTestAndTriggerRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/type_name_resolution_pins"
	deferred := namespaceAbsenceDeferrals(t, filepath.Join(fixture, "receiver_routes"))
	cases, err := readTypeNamePinRowsForApextest(filepath.Join(fixture, "receiver_routes/cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, row := range cases[1:] {
		sources[row[1]+"/"+row[0]] = row[2]
	}
	rows, err := readTypeNamePinRowsForApextest(filepath.Join(fixture, "receiver_routes/org.tsv"))
	if err != nil || len(rows) != 25 {
		t.Fatalf("native rows: %d, %v", len(rows), err)
	}
	for _, row := range rows[1:] {
		if row[1] == "anonymous" {
			continue
		}
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			if row[3] != "false" {
				t.Fatalf("expected native rejection: %v", row)
			}
			source := sources[row[1]+"/"+row[2]]
			if _, err := os.Stat(filepath.Join(fixture, source)); err != nil {
				t.Fatal(err)
			}
			root := copyTypeNamePinProject(t, fixture, row[0], source)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/DottedReceiverControlTest.cls"), `@IsTest private class DottedReceiverControlTest { @IsTest static void observe() { System.assert(true); } }`)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/DottedReceiverControlTest.cls-meta.xml"), "<ApexClass><apiVersion>"+row[0]+"</apiVersion><status>Active</status></ApexClass>")
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatalf("index errors: %v", index.Diagnostics)
			}
			options := Options{NoDiskCache: true, Parallelism: 1}
			if deferred[row[2]] {
				options.SelectedClasses = []string{"DottedReceiverControlTest"}
			}
			run := Run(index, options)
			if deferred[row[2]] {
				if len(run.ExcludedClasses) != 0 || run.Summary().Passed != 1 {
					t.Fatalf("offline deferred gate: %+v", run)
				}
				return
			}
			name := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
			for _, entry := range run.ExcludedClasses {
				if entry.ClassName == name && strings.Contains(entry.Message, row[4]) {
					if run.Summary().Passed != 1 {
						t.Fatalf("unrelated control did not run: %v", run.Summary())
					}
					return
				}
			}
			t.Fatalf("native rejected %s; project gate did not exclude it: summary=%v excluded=%v", name, run.Summary(), run.ExcludedClasses)
		})
	}
}
