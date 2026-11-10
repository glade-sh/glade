package apextest

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

const standardRestrictedP0 = "testdata/conformance/standard_describe_profiles/P0"

type standardRestrictedNativeField struct {
	Name               string `json:"name"`
	Type               string `json:"type"`
	RestrictedPicklist bool   `json:"restrictedPicklist"`
	DependentPicklist  bool   `json:"dependentPicklist"`
	PicklistValues     []struct {
		Value        string  `json:"value"`
		Label        *string `json:"label"`
		Active       bool    `json:"active"`
		DefaultValue bool    `json:"defaultValue"`
	} `json:"picklistValues"`
}

// standardRestrictedStringTyped names native restricted picklists Glade
// still types STRING, with the reason.
var standardRestrictedStringTyped = map[string]string{
	"WorkloadUnit.MeasureUnit": "the catalog input types it string and no stub overlay covers it",
}

func standardRestrictedNativeDescribes(t *testing.T, api string) map[string][]standardRestrictedNativeField {
	t.Helper()
	f, err := os.Open(filepath.Join(standardRestrictedP0, "api"+strings.TrimSuffix(api, ".0"), "default", "describes.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Describes map[string]struct {
			Fields []standardRestrictedNativeField `json:"fields"`
		} `json:"describes"`
	}
	if err := json.NewDecoder(reader).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	out := make(map[string][]standardRestrictedNativeField, len(doc.Describes))
	for name, describe := range doc.Describes {
		out[name] = describe.Fields
	}
	return out
}

func standardRestrictedAllowlist(t *testing.T) map[string]bool {
	t.Helper()
	f, err := os.Open("../storage/testdata/standard_picklist_values.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var values []string
	if err := json.NewDecoder(reader).Decode(&values); err != nil {
		t.Fatal(err)
	}
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

// standardRestrictedPreSupplementStrings replays the released baseline's
// STRING-typed fields. This keeps the prior value-list assertion unchanged
// for every field to which it applied before the supplement.
func standardRestrictedPreSupplementStrings(t *testing.T) map[string]map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata/conformance/standard_describe_profiles", "pre_supplement_string_fields.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows map[string][]string
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	out := make(map[string]map[string]bool, len(rows))
	for api, specs := range rows {
		out[api] = make(map[string]bool, len(specs))
		for _, spec := range specs {
			out[api][spec] = true
		}
	}
	return out
}

// standardRestrictedValuesWithheld names allowlisted native lists Glade does
// not carry, with the reason.
var standardRestrictedValuesWithheld = map[string]string{
	"PermissionSetEvent.SessionLevel": "native entries have a null label; Glade renders the value as the label",
}

// TestStandardRestrictedPicklistNativeDescribe compares Apex describe with the
// owned P0 REST describes (fresh plain Developer org, default profile, API
// 62.0 and 67.0). Every standard field native reports as a restricted picklist
// and Glade describes as a picklist must report isRestrictedPicklist(). An
// empty Glade value list is accepted only where a native value is outside the
// reviewed public allowlist. For fields previously typed STRING, the
// supplement additionally withholds lists with null labels or defaults,
// dependent values, or differences between the APIs. Fields on the Apex stub path follow the generated P0 supplement;
// a field Glade still types STRING fails unless it is named in
// standardRestrictedStringTyped. The storage test
// TestStandardStubSupplementApplied checks the stub lists themselves.
func TestStandardRestrictedPicklistNativeDescribe(t *testing.T) {
	allowed := standardRestrictedAllowlist(t)
	priorStrings := standardRestrictedPreSupplementStrings(t)
	describes := map[string]map[string][]standardRestrictedNativeField{}
	for _, api := range []string{"62.0", "67.0"} {
		describes[api] = standardRestrictedNativeDescribes(t, api)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			native := describes[api]
			var specs []string
			want := map[string]standardRestrictedNativeField{}
			for object, fields := range native {
				for _, field := range fields {
					if field.RestrictedPicklist {
						spec := object + "." + field.Name
						specs = append(specs, spec)
						want[spec] = field
					}
				}
			}
			sort.Strings(specs)
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeFile(t, filepath.Join(root, "force-app/.keep"), "")
			index := loadTestIndex(t, root)
			quoted := make([]string, len(specs))
			for i, spec := range specs {
				quoted[i] = "'" + spec + "'"
			}
			source := `Map<String, Schema.SObjectType> g = Schema.getGlobalDescribe();
for (String spec : new List<String>{` + strings.Join(quoted, ",") + `}) {
  List<String> p = spec.split('\\.');
  Schema.SObjectType t = g.get(p[0]);
  if (t == null) continue;
  Schema.SObjectField f = t.getDescribe().fields.getMap().get(p[1]);
  if (f == null) continue;
  Schema.DescribeFieldResult d = f.getDescribe();
  System.debug('RP|' + spec + '|' + d.getType() + '|' + d.isRestrictedPicklist() + '|' + d.getPicklistValues().size());
}`
			org := capturedFixtureOrg(t, index, api)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org})
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			checked, stringTyped := 0, 0
			for _, line := range result.Debug {
				_, row, ok := strings.Cut(line, "RP|")
				if !ok {
					continue
				}
				cells := strings.Split(row, "|")
				spec, kind, restricted, count := cells[0], cells[1], cells[2], cells[3]
				if kind != "PICKLIST" && kind != "MULTIPICKLIST" {
					stringTyped++
					if standardRestrictedStringTyped[spec] == "" {
						t.Errorf("%s: native restricted picklist; glade type %s", spec, kind)
					}
					continue
				}
				checked++
				if restricted != "true" {
					t.Errorf("%s: native restricted picklist; glade isRestrictedPicklist()=%s", spec, restricted)
				}
				if count == "0" && len(want[spec].PicklistValues) != 0 {
					// A list is withheld when a value at either API is outside
					// the allowlist: a partial list must not look complete.
					// A list is also withheld when it carries a null label or a
					// default entry, belongs to a dependent picklist, or differs
					// between the APIs (scripts/generate-standard-stub-restricted-picklists.py).
					blocked := standardRestrictedValuesWithheld[spec] != ""
					var lists []string
					supplementOmission := false
					for _, other := range []string{"62.0", "67.0"} {
						for _, field := range describes[other][strings.SplitN(spec, ".", 2)[0]] {
							if field.Name != strings.SplitN(spec, ".", 2)[1] {
								continue
							}
							supplementOmission = supplementOmission || field.DependentPicklist
							var list []string
							for _, value := range field.PicklistValues {
								blocked = blocked || !allowed[value.Value]
								supplementOmission = supplementOmission || value.Label == nil || value.DefaultValue
								list = append(list, fmt.Sprintf("%s=%s/%v", value.Value, standardRestrictedLabel(value.Label), value.Active))
							}
							lists = append(lists, strings.Join(list, ","))
						}
					}
					supplementOmission = supplementOmission || (len(lists) == 2 && lists[0] != lists[1])
					// These omissions apply only to fields that were strings before the supplement.
					blocked = blocked || (priorStrings[api][spec] && supplementOmission)
					if !blocked {
						t.Errorf("%s: native lists %d allowlisted values; glade lists none", spec, len(want[spec].PicklistValues))
					}
				}
			}
			if checked < 1650 {
				t.Errorf("checked %d picklist-typed fields; want at least 1650", checked)
			}
			t.Logf("API %s: %d native restricted fields, %d picklist-typed checked, %d typed STRING by Glade", api, len(specs), checked, stringTyped)
		})
	}
}

func standardRestrictedLabel(label *string) string {
	if label == nil {
		return "<null>"
	}
	return *label
}
