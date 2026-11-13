package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"testing"
)

// The L19, L09, L07, V12 and V14 conformance families are one top-level test per
// row range so the shard planners can spread their rows. This pins each part
// list, so a dropped or renamed part cannot silently drop rows, and the Visualforce
// duration table entries. Internal tests prove that the ranges cover every row once.
func TestCIConformanceRowSplitPartsAreListed(t *testing.T) {
	numbered := func(prefix string, parts int) []string {
		var names []string
		for part := 1; part <= parts; part++ {
			names = append(names, fmt.Sprintf("%sPart%dof%d", prefix, part, parts))
		}
		return names
	}
	for _, family := range []struct {
		source string
		prefix string
		want   []string
	}{
		{"../internal/lwc/compile/l19_conformance_test.go", "TestL19SalesforceConformance", numbered("TestL19SalesforceConformance", 3)},
		{"../internal/lwc/compile/l09_conformance_test.go", "TestL09SalesforceConformance", numbered("TestL09SalesforceConformance", 2)},
		{"../internal/visualforce/v12_conformance_test.go", "TestV12SalesforceConformance", numbered("TestV12SalesforceConformance", 2)},
		{"../internal/visualforce/v14_conformance_test.go", "TestV14SalesforceConformance", numbered("TestV14SalesforceConformance", 2)},
		{"../internal/lwc/compile/l07_conformance_test.go", "TestL07SalesforceConformanceAPI", append(append(
			numbered("TestL07SalesforceConformanceAPI59", 2),
			numbered("TestL07SalesforceConformanceAPI67", 2)...),
			numbered("TestL07SalesforceConformanceAPI68", 2)...)},
	} {
		source, err := os.ReadFile(family.source)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, match := range regexp.MustCompile(`(?m)^func (`+family.prefix+`\w*)\(t \*testing\.T\)`).FindAllStringSubmatch(string(source), -1) {
			got = append(got, match[1])
		}
		if !reflect.DeepEqual(got, family.want) {
			t.Errorf("%s parts = %v, want %v", family.prefix, got, family.want)
		}
	}

	data, err := os.ReadFile("../internal/visualforce/testdata/shard_durations.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Tests []struct {
			Name string `json:"name"`
		} `json:"tests"`
	}
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	listed := map[string]bool{}
	for _, test := range table.Tests {
		listed[test.Name] = true
	}
	for _, name := range append(numbered("TestV12SalesforceConformance", 2), numbered("TestV14SalesforceConformance", 2)...) {
		if !listed[name] {
			t.Errorf("duration table does not list %s", name)
		}
	}
	for _, name := range []string{"TestV12SalesforceConformance", "TestV14SalesforceConformance"} {
		if listed[name] {
			t.Errorf("duration table still lists the unsplit %s", name)
		}
	}
}
