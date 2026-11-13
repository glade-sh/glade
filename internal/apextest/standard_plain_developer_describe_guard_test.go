package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// Local cache isolation and explicit value-set controls; native rows are in
// testdata/standard_plain_developer_describe/org.tsv.
func TestPlainDeveloperDescribeCacheAndConfiguredValues(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "standard_plain_developer_describe", "classes", "SchemaProbe62.cls"))
	if err != nil {
		t.Fatal(err)
	}
	index := standardDescribeDeclaredBoundaryIndex(t, "62.0", map[string]string{"SchemaProbe62": string(src)}, nil)
	for _, configured := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "configured"}[configured], func(t *testing.T) {
			org := orgFromIndex(index)
			org.APIVersion = "62.0"
			if configured {
				storage.EnsureStandardObject(&org, "Account")
				state := org.Objects["Account"]
				field := state.Definition.Fields["Type"]
				field.PicklistValuesConfigured = true
				field.PicklistValues = []storage.PicklistValue{{Value: "Project", Label: "Project", Active: true}}
				state.Definition.Fields["Type"] = field
				org.Objects["Account"] = state
			}
			machine := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org}).newMachine()
			source := `Map<String,Schema.SObjectField> a=Account.SObjectType.getDescribe().fields.getMap(); Boolean before=a.containsKey('Tier'); String helper=SchemaProbe62.names('Account',1); Map<String,Schema.SObjectField> b=Account.SObjectType.getDescribe().fields.getMap(); Boolean after=b.containsKey('Tier'); List<Schema.PicklistEntry> entries=b.get('Type').getDescribe().getPicklistValues(); String first=entries[0].getValue();`
			p, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: "62.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(p); err != nil {
				t.Fatal(err)
			}
			if machine.Globals["before"].Bool || machine.Globals["after"].Bool || !strings.Contains(machine.Globals["helper"].Text, ",tier,") {
				t.Fatalf("direct/helper/direct cache leak: %#v", machine.Globals)
			}
			want := "Prospect"
			if configured {
				want = "Project"
			}
			if got := machine.Globals["first"].Text; got != want {
				t.Fatalf("first value %q want %q", got, want)
			}
		})
	}
}
