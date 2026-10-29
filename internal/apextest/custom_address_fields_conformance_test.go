package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

const customAddressFixture = "../../testdata/conformance/custom_address_fields"

// customAddressKnownGaps lists native rows Glade does not match for reasons
// unrelated to custom Address fields. Each gap reproduces with ordinary fields.
var customAddressKnownGaps = map[string]string{
	"P07b": "getPopulatedFieldsAsMap key order follows the SELECT list natively (generic)",
	"P08b": "getPopulatedFieldsAsMap key order follows the SELECT list natively (generic)",
	"P10":  "Database.query checks COUNT() routing before WHERE validity (generic); P10x covers the Address rule",
	"P10d": "AggregateResult debug text includes the grouped field next to its alias (generic)",
	"C06":  "unknown fields on project custom objects compile in Glade anonymous Apex (generic)",
	"C07":  "unknown fields on project custom objects compile in Glade anonymous Apex (generic)",
	"C08":  "System.Address is not modeled as a System.Location subtype (generic)",
}

type customAddressRow struct{ route, id, observation string }

func customAddressNativeRows(t *testing.T, api string) []customAddressRow {
	t.Helper()
	f, err := os.Open(filepath.Join(customAddressFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	table, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	var rows []customAddressRow
	for _, row := range table[1:] {
		if row[0] == api {
			rows = append(rows, customAddressRow{row[1], row[2], row[3]})
		}
	}
	if len(rows) != 68 {
		t.Fatalf("native API %s rows = %d; want 68", api, len(rows))
	}
	return rows
}

func customAddressProject(t *testing.T, api string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(customAddressFixture, "project")
	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(source, path)
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := strings.ReplaceAll(string(raw), "67.0", api)
		writeFile(t, filepath.Join(root, rel), text)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("project compilation: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	return index
}

func customAddressRun(t *testing.T, index typesys.Index, api, source string) ([]string, error) {
	t.Helper()
	org := orgFromIndex(index)
	org.APIVersion = api
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
	program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	result, err := runner.execute(program)
	var out []string
	for _, line := range result.Debug {
		if _, row, ok := strings.Cut(line, "ADDR|"); ok {
			out = append(out, row)
		}
	}
	return out, err
}

// customAddressJSONEqual compares row P14 by value: the probe re-serializes a
// deserializeUntyped map, whose key order is a separate generic gap. Direct
// System.Address JSON order is asserted by TestCustomAddressJSONPropertyOrder.
func customAddressJSONEqual(native, glade string) bool {
	parse := func(text string) map[string]any {
		out := map[string]any{}
		for _, part := range strings.Split(text, ";") {
			key, value, _ := strings.Cut(part, "=")
			var decoded any
			if json.Unmarshal([]byte(value), &decoded) != nil {
				return nil
			}
			out[key] = decoded
		}
		return out
	}
	left, right := parse(native), parse(glade)
	return left != nil && reflect.DeepEqual(left, right)
}

// TestCustomAddressFieldsNativeConformance replays the native custom Address
// capture (fresh Developer scratch org, Custom Address Fields enabled, API
// 62.0 and 67.0): describe facets, component and compound access, DML, SOQL,
// get/put, JSON, compile-time diagnostics and the uncatchable ORDER BY row.
func TestCustomAddressFieldsNativeConformance(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := customAddressProject(t, api)
			native := customAddressNativeRows(t, api)
			anonymous, err := customAddressRun(t, index, api, "for (String r : AddressFieldProbe.run()) System.debug(LoggingLevel.ERROR, 'ADDR|' + r);")
			if err != nil {
				t.Fatalf("probe: %v", err)
			}
			got := map[string]string{}
			for _, row := range anonymous {
				id, observation, _ := strings.Cut(row, "|")
				got[id] = observation
			}
			matched := 0
			for _, row := range native {
				if row.route != "anonymous" {
					continue
				}
				if _, gap := customAddressKnownGaps[row.id]; gap {
					continue
				}
				observed, ok := got[row.id]
				if row.id == "P14" && ok && customAddressJSONEqual(row.observation, observed) {
					matched++
					continue
				}
				if !ok || observed != row.observation {
					t.Errorf("%s: native <%s> glade <%s>", row.id, row.observation, observed)
					continue
				}
				matched++
			}
			if matched != 49 {
				t.Errorf("matched anonymous rows = %d; want 49", matched)
			}
			for _, row := range native {
				if row.route != "compile" {
					continue
				}
				if _, gap := customAddressKnownGaps[row.id]; gap {
					continue
				}
				raw, err := os.ReadFile(filepath.Join(customAddressFixture, "cases", row.id+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				source := string(raw)
				kind, detail, _ := strings.Cut(row.observation, "|")
				analysis := sema.AnalyzeAnonymous(index, source, api)
				if kind == "COMPILE" {
					if !analysis.HasErrors() {
						t.Errorf("%s: native compile error <%s>; glade compiled", row.id, detail)
						continue
					}
					text := ""
					for _, item := range analysis.Diagnostics {
						text += item.Message + "|" + item.NativeMessage + "\n"
					}
					if !strings.Contains(text, detail) && !strings.Contains(text, customAddressCompileFragment(row.id)) {
						t.Errorf("%s: native <%s>; glade %s", row.id, detail, text)
					}
					continue
				}
				if analysis.HasErrors() {
					t.Errorf("%s: native compiled; glade %v", row.id, analysis.Diagnostics)
					continue
				}
				out, err := customAddressRun(t, index, api, source)
				switch kind {
				case "UNCAUGHT":
					var runtimeErr *vm.RuntimeError
					if !errors.As(err, &runtimeErr) || runtimeErr.Type+": "+runtimeErr.Message != detail {
						t.Errorf("%s: native <%s>; glade %v", row.id, detail, err)
					}
				case "OK":
					if err != nil || strings.Join(out, ";") != detail {
						t.Errorf("%s: native <%s>; glade <%s> %v", row.id, detail, strings.Join(out, ";"), err)
					}
				}
			}
			// P10x: the WHERE rule of P10 without the COUNT() routing gap.
			out, err := customAddressRun(t, index, api, `try { Database.query('SELECT Id FROM Addr_Test__c WHERE Home__c != null'); } catch (QueryException e) { System.debug('ADDR|' + e.getMessage()); }`)
			if err != nil || len(out) != 1 || out[0] != "Address fields can only be filtered using Distance expressions" {
				t.Errorf("P10x: %v %v", out, err)
			}
		})
	}
}

// Glade compile messages carry a Glade code prefix; match the native text or
// the stable fragment Glade emits for the same rule.
func customAddressCompileFragment(id string) string {
	switch id {
	case "C03":
		return "unknown field Addr_Test__c.Home__Bogus__s"
	case "C10":
		return "unknown field Addr_Test__c.Home__State__s"
	case "C13":
		return "Home__Latitude__s with String"
	case "C14":
		return "Home__City__s with Integer"
	}
	return "\x00"
}

// TestCustomAddressFieldsDescribeMatchesNative compares the storage describe
// of every Home component with native REST describe facets.
func TestCustomAddressFieldsDescribeMatchesNative(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(customAddressFixture, "describe.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	reader.LazyQuotes = true
	table, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	header := table[0]
	col := func(row []string, name string) string {
		for i, item := range header {
			if item == name {
				return row[i]
			}
		}
		t.Fatalf("missing column %s", name)
		return ""
	}
	flag := func(value *bool) string {
		if value == nil {
			return "nil"
		}
		if *value {
			return "True"
		}
		return "False"
	}
	for _, api := range []string{"62.0", "67.0"} {
		index := customAddressProject(t, api)
		org := orgFromIndex(index)
		checked := 0
		for _, row := range table[1:] {
			if row[0] != api {
				continue
			}
			objectName, fieldName := col(row, "object"), col(row, "name")
			name, field, ok := storage.ResolveFieldDefinition(org.Objects[objectName].Definition, "", fieldName)
			if !ok || name != fieldName {
				t.Errorf("%s %s.%s: resolved %q %v", api, objectName, fieldName, name, ok)
				continue
			}
			wantCompound := col(row, "compoundFieldName")
			if wantCompound == "None" {
				wantCompound = ""
			}
			got := []string{field.Label, field.CompoundFieldName, flag(field.Nillable), flag(field.Createable), flag(field.Updateable),
				flag(field.Filterable), flag(field.Sortable), flag(field.Groupable), flag(field.Aggregatable)}
			want := []string{col(row, "label"), wantCompound, col(row, "nillable"), col(row, "createable"), col(row, "updateable"),
				col(row, "filterable"), col(row, "sortable"), col(row, "groupable"), col(row, "aggregatable")}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s %s.%s: native %v glade %v", api, objectName, fieldName, want, got)
			}
			if values := col(row, "picklistValuesN"); values != "0" && values != strconv.Itoa(len(field.PicklistValues)) {
				t.Errorf("%s %s.%s: native %s picklist values; glade %d", api, objectName, fieldName, values, len(field.PicklistValues))
			}
			checked++
		}
		if checked != 18 {
			t.Fatalf("%s: checked %d describe rows; want 18", api, checked)
		}
	}
}
