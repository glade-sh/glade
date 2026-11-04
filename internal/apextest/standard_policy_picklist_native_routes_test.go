package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestStandardPolicyPicklistNamedApexNativeRoutes(t *testing.T) {
	const fixture = "testdata/conformance/standard_policy_picklist_routes"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
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
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			want := map[string]string{}
			anonymousWant := map[string]string{}
			for _, row := range rows[1:] {
				if row[0] == api && row[1] == "anonymous" {
					var value string
					if err := json.Unmarshal([]byte(row[3]), &value); err != nil {
						t.Fatal(err)
					}
					anonymousWant[row[2]] = value
				}
				if row[0] == api && row[1] == "named" {
					var value string
					if err := json.Unmarshal([]byte(row[3]), &value); err != nil {
						t.Fatal(err)
					}
					want[row[2]] = value
				}
			}
			if len(want) != 8 {
				t.Fatalf("native rows=%d", len(want))
			}
			root := t.TempDir()
			base := filepath.Join(root, "force-app/main/default/classes")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			name := "Case251PicklistProbe" + strings.TrimSuffix(api, ".0")
			for _, ext := range []string{".cls", ".cls-meta.xml"} {
				body, err := os.ReadFile(filepath.Join(fixture, name+ext))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(base, name+ext), string(body))
			}
			testName := "Case251PicklistRouteTest"
			writeFile(t, filepath.Join(base, testName+".cls"), fmt.Sprintf(`@IsTest private class %s { @IsTest static void observed() { System.assert(false, 'CASE251TEST|' + new %s().getObserved()); } }`, testName, name))
			writeFile(t, filepath.Join(base, testName+".cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			for key := range want {
				parts := strings.SplitN(key, ".", 2)
				storage.EnsureStandardObject(&org, parts[0])
			}
			for key := range want {
				parts := strings.SplitN(key, ".", 2)
				state := org.Objects[parts[0]]
				field := state.Definition.Fields[parts[1]]
				field.PicklistValues = nil
				field.PicklistValuesConfigured = false
				state.Definition.Fields[parts[1]] = field
				org.Objects[parts[0]] = state
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			program, err := vm.CompileAnonymousWithOptions(fmt.Sprintf(`System.debug('CASE251START' + new %s().getObserved() + 'CASE251END');`, name), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err := runner.newMachine().Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			debug := strings.Join(result.Debug, "\n")
			_, after, ok := strings.Cut(debug, "CASE251START")
			if !ok {
				t.Fatalf("missing marker: %s", debug)
			}
			payload, _, ok := strings.Cut(after, "CASE251END")
			if !ok {
				t.Fatal("missing end marker")
			}
			var got map[string]string
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatalf("payload %q: %v", payload, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Apex=%v native=%v", got, want)
			}
			// Local test-runner route must preserve the already-captured named
			// route values; no separate native @IsTest observation is claimed.
			run := Run(index, Options{SelectedClasses: []string{testName}, SelectedMethod: "observed", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing test observation: %+v", run.Summary())
			}
			_, testJSON, ok := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "CASE251TEST|")
			if !ok {
				t.Fatalf("missing test marker: %+v", run.Suites[0].Cases[0].Problem)
			}
			var testValues map[string]string
			if err := json.Unmarshal([]byte(testJSON), &testValues); err != nil || !reflect.DeepEqual(testValues, got) {
				t.Fatalf("test route=%v named=%v err=%v", testValues, got, err)
			}
			// Contract's native anonymous view lacks permission, whereas the
			// local synthetic org has no matching FLS configuration. Assert the
			// captured boundary and compare the seven grant-visible fields.
			if anonymousWant["Contract.ShippingGeocodeAccuracy"] != "ERROR:System.NullPointerException" {
				t.Fatal("missing native visibility boundary")
			}
			body, err := os.ReadFile(filepath.Join(fixture, "probe.apex"))
			if err != nil {
				t.Fatal(err)
			}
			anonymous, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			anonResult, err := runner.newMachine().Execute(anonymous)
			if err != nil {
				t.Fatal(err)
			}
			_, anonAfter, ok := strings.Cut(strings.Join(anonResult.Debug, "\n"), "CASE251START")
			if !ok {
				t.Fatal("missing anonymous marker")
			}
			anonJSON, _, ok := strings.Cut(anonAfter, "CASE251END")
			if !ok {
				t.Fatal("missing anonymous end")
			}
			var anonValues map[string]string
			if err := json.Unmarshal([]byte(anonJSON), &anonValues); err != nil {
				t.Fatal(err)
			}
			for field, expected := range anonymousWant {
				if field != "Contract.ShippingGeocodeAccuracy" && anonValues[field] != expected {
					t.Errorf("anonymous %s=%q native=%q", field, anonValues[field], expected)
				}
			}
		})
	}
}
