package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Named-only API68 facets come from the P0 named-facet capture packets.
func TestStandardDescribeAPI68NativeNamedOnlyFacets(t *testing.T) {
	root := "testdata/conformance/standard_describe_api68_drift/"
	var workload struct {
		Name, Label, Type          string
		Accessible, Permissionable bool
	}
	data, err := os.ReadFile(root + "P0-named-facets/default.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &workload); err != nil {
		t.Fatal(err)
	}
	var shift []struct {
		Name, Label, Type          string
		Accessible, Permissionable bool
	}
	data, err = os.ReadFile(root + "P0-new-named-facets/default.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &shift); err != nil {
		t.Fatal(err)
	}
	if len(shift) != 2 {
		t.Fatalf("native Shift fields=%d, want 2", len(shift))
	}
	const classSource = `public class V035NativeFacets { public static void capture(String obj, String field) { Schema.DescribeFieldResult d = Schema.getGlobalDescribe().get(obj).getDescribe().fields.getMap().get(field).getDescribe(); System.debug(d.getLabel()); System.debug(String.valueOf(d.getType())); System.debug(d.isAccessible()); System.debug(d.isPermissionable()); } }`
	index := standardDescribeDeclaredBoundaryIndex(t, "68.0", map[string]string{"V035NativeFacets": classSource}, nil)
	var source strings.Builder
	var want []string
	fmt.Fprintf(&source, "V035NativeFacets.capture('WorkloadUnit','%s');\n", workload.Name)
	want = append(want, workload.Label, workload.Type, fmt.Sprint(workload.Accessible), fmt.Sprint(workload.Permissionable))
	for _, field := range shift {
		fmt.Fprintf(&source, "V035NativeFacets.capture('ShiftAllocationRequest','%s');\n", field.Name)
		want = append(want, field.Label, field.Type, fmt.Sprint(field.Accessible), fmt.Sprint(field.Permissionable))
	}
	program, err := vm.CompileAnonymousWithOptions(source.String(), vm.CompileOptions{APIVersion: "68.0"})
	if err != nil {
		t.Fatal(err)
	}
	org := orgFromIndex(index)
	org.APIVersion = "68.0"
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	result, err := runner.execute(program)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(result.Debug, "|") != strings.Join(want, "|") {
		t.Fatalf("native %v; Glade %v", want, result.Debug)
	}
}
