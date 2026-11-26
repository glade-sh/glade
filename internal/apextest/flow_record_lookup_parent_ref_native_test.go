package apextest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE rows: flow_record_lookup_parent_ref/org.tsv and
// flow_parent_reference_id_forms/org.tsv.
// Three record-triggered after-save Flows each run Get Records with a filter
// whose value is $Record.<Lookup>__r.<Field>: a Contact lookup on the parent
// compared with User.ContactId and with a custom Contact lookup, and a lookup
// field of another type. A null parent value matches the null field. At API
// 62.0 and 67.0 Salesforce found no User for a Contact, found the custom row,
// and found a User when the parent value was null.
func TestFlowRecordLookupParentRefNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_record_lookup_parent_ref"
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	rows := strings.Split(strings.TrimSpace(read("org.tsv")), "\n")
	apex := read("run.apex")
	if !strings.Contains(apex, "System.debug('R5|lookup|' + String.join(rows, ','));") {
		t.Fatal("fixture apex changed")
	}
	wantByAPI := map[string]string{}
	for line, row := range rows[1:] {
		cols := strings.Split(row, "\t")
		if len(cols) < 9 || cols[0] != "lookup" || cols[2] != "anonymous" || cols[3] != "True" || cols[4] != "True" || !strings.HasPrefix(cols[8], "cleaned") {
			t.Fatalf("invalid native row %d: %s", line+2, row)
		}
		wantByAPI[cols[1]] = strings.ReplaceAll(cols[5], "&#124;", "|")
	}
	const formsFixture = "testdata/conformance/flow_parent_reference_id_forms"
	formsData, err := os.ReadFile(filepath.Join(formsFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	forms := map[string]string{}
	formRows := strings.Split(strings.TrimSpace(string(formsData)), "\n")
	if len(formRows) != 25 {
		t.Fatalf("native ID-form row count = %d, want 24", len(formRows)-1)
	}
	for _, row := range formRows[1:] {
		cols := strings.Split(row, "\t")
		if len(cols) != 9 {
			t.Fatalf("invalid ID-form row")
		}
		key := cols[1] + "/" + cols[0]
		if _, duplicate := forms[key]; duplicate {
			t.Fatalf("duplicate ID-form row: %s", key)
		}
		forms[key] = cols[5]
	}
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0"} {
		for _, digits := range []int{15, 18} {
			t.Run(fmt.Sprintf("%s/id%d", api, digits), func(t *testing.T) {
				legacy, hasLegacy := wantByAPI[api]
				want, ok := forms[fmt.Sprintf("%s/lookup_%d_istest", api, digits)]
				if !ok {
					t.Fatalf("missing test-route ID-form row")
				}
				wantAnonymous, ok := forms[fmt.Sprintf("%s/lookup_%d_anonymous", api, digits)]
				if !ok || wantAnonymous != want || (hasLegacy && want != legacy) {
					t.Fatalf("ID-form native rows disagree with the original observation")
				}
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [R5L]\n")
				err := filepath.WalkDir(filepath.Join(fixture, "project/force-app"), func(path string, entry fs.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					name, err := filepath.Rel(filepath.Join(fixture, "project"), path)
					if err != nil {
						return err
					}
					writeFile(t, filepath.Join(root, name), strings.ReplaceAll(read(filepath.Join("project", name)), "__API__", api))
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				caseBytes, err := os.ReadFile(filepath.Join(formsFixture, fmt.Sprintf("probe%d.apex", digits)))
				if err != nil {
					t.Fatal(err)
				}
				caseApex := strings.ReplaceAll(string(caseBytes), "System.assert(false, 'R5|lookup|' + String.join(rows, ','));", "System.debug('R5|lookup|' + String.join(rows, ','));")
				if caseApex == string(caseBytes) {
					t.Fatal("ID-form probe assertion missing")
				}
				probe := "@IsTest private class FlowLookupProbe { @IsTest static void observe() {\n" +
					strings.ReplaceAll(caseApex, "System.debug('R5|lookup|' + String.join(rows, ','));", "System.assert(false, 'R5|lookup|' + String.join(rows, ','));") + "\n} }"
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/FlowLookupProbe.cls"), probe)
				index := loadTestIndex(t, root)
				run := Run(index, Options{SelectedClasses: []string{"FlowLookupProbe"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("unexpected test selection: %+v", run.Summary())
				}
				observation := run.Suites[0].Cases[0]
				if observation.Problem == nil {
					t.Fatal("probe did not report")
				}
				got := observation.Problem.Message
				if !strings.Contains(got, want) {
					t.Fatalf("NATIVE flow_record_lookup_parent_ref/org.tsv %s anonymous: want %q; Glade %q", api, want, got)
				}

				// Replay the anonymous route without the reporting assertion so the
				// actual stored lookup spelling can be checked, rather than inferred
				// from the submitted Apex expression.
				org := capturedFixtureOrg(t, index, api)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				machine := runner.newMachine()
				program, err := vm.CompileAnonymousWithOptions(caseApex, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = machine.Execute(program); err != nil {
					t.Fatal(err)
				}
				parents := machine.Org.Objects["Pv__c"].Records
				if len(parents) != 2 {
					t.Fatalf("stored parent records = %d, want 2", len(parents))
				}
				for key := range parents {
					if len(key) != 15 {
						t.Fatalf("stored parent key length = %d, want 15", len(key))
					}
				}
				count := 0
				for _, row := range machine.Org.Objects["Ca__c"].Records {
					value, ok := row.Fields["Pv__c"]
					if !ok || value.Kind == storage.ValueNull {
						continue
					}
					var raw string
					switch value.Kind {
					case storage.ValueID:
						raw = string(value.ID)
					case storage.ValueString:
						raw = value.String
					default:
						t.Fatalf("unexpected lookup value kind: %v", value.Kind)
					}
					if len(raw) != digits {
						t.Fatalf("stored lookup length = %d, want %d", len(raw), digits)
					}
					count++
				}
				if count != 2 {
					t.Fatalf("stored parent references = %d, want 2", count)
				}
				var observations []string
				for _, row := range machine.Org.Objects["FL__c"].Records {
					observations = append(observations, row.Fields["Name"].String+":"+row.Fields["Tag__c"].String)
				}
				sort.Strings(observations)
				if actual := "R5|lookup|" + strings.Join(observations, ","); actual != wantAnonymous {
					t.Fatalf("native observation %q; anonymous Glade %q", wantAnonymous, actual)
				}
			})
		}
	}
}
