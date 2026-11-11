package vm

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Native observations of how Lightning converts the JSON arguments of an
// @AuraEnabled call into Apex parameters:
// lwc_param_conversion/org.tsv. Two fresh scratch orgs called single-parameter
// static methods (one per parameter type) with 35 argument shapes from
// Lightning web components at API 59.0, 62.0 and 67.0 against Apex classes at API
// 54.0, 62.0 and 67.0, and from Aura components at 59.0 and 67.0. The imperative,
// cacheable imperative and Aura routes agreed in every cell, and no cell varied
// with the Apex or component API. Cells where the two orgs differ (Aura batches
// that failed with a transport error, string arguments for SObject parameters)
// assert only what every observation shares.

const lwcParamConversionFixture = "lwc_param_conversion"

type lwcParamCell struct {
	typeName string
	argName  string
	outcomes map[string]bool
}

func lwcParamConversionCells(t *testing.T) []*lwcParamCell {
	t.Helper()
	file, err := os.Open(filepath.Join("..", "apextest", "testdata", "conformance", lwcParamConversionFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	byKey := map[string]*lwcParamCell{}
	var header []string
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if header == nil {
			header = fields
			continue
		}
		if len(fields) != len(header) {
			t.Fatalf("row has %d fields, want %d: %q", len(fields), len(header), scanner.Text())
		}
		row := map[string]string{}
		for i, name := range header {
			row[name] = fields[i]
		}
		// A transport failure of the whole Aura batch is not a conversion answer.
		if strings.Contains(row["outcome"], "aura:systemError") {
			continue
		}
		key := row["type"] + "\x00" + row["case"]
		cell := byKey[key]
		if cell == nil {
			cell = &lwcParamCell{typeName: row["type"], argName: row["case"], outcomes: map[string]bool{}}
			byKey[key] = cell
		}
		cell.outcomes[row["outcome"]] = true
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	cells := make([]*lwcParamCell, 0, len(byKey))
	for _, cell := range byKey {
		cells = append(cells, cell)
	}
	sort.Slice(cells, func(i, j int) bool {
		if cells[i].typeName != cells[j].typeName {
			return cells[i].typeName < cells[j].typeName
		}
		return cells[i].argName < cells[j].argName
	})
	return cells
}

// Native answers that Glade does not reproduce yet. A Date parameter that
// receives a UTC Datetime string keeps its time of day, which the Date value
// model cannot represent.
var lwcParamConversionParked = map[string]bool{
	"Date/str_datetime_z": true,
}

func lwcParamConversionArgs(t *testing.T, name string) (map[string]any, bool) {
	t.Helper()
	values := map[string]any{
		"null": nil, "str_x": "x", "num_42": float64(42), "num_0": float64(0), "num_1": float64(1),
		"num_frac": 4.5, "num_big": float64(3000000000), "num_neg": float64(-7),
		"bool_true": true, "bool_false": false,
		"str_true": "true", "str_false": "false", "str_TRUE": "TRUE", "str_empty": "",
		"str_42": "42", "str_frac": "4.5", "str_date": "2024-01-02",
		"str_datetime_z": "2024-01-02T03:04:05.000Z", "str_datetime_noz": "2024-01-02T03:04:05.000",
		"str_datetime_offset": "2024-01-02T03:04:05.000+0100", "num_epoch": float64(1700000000000),
		"str_id18": "001000000000000AAA", "str_id15": "001000000000000",
		"obj_empty": map[string]any{}, "obj_name": map[string]any{"Name": "n"},
		"obj_sobj":       map[string]any{"sobjectType": "Account", "Name": "n"},
		"obj_sobj_wrong": map[string]any{"sobjectType": "Contact", "LastName": "n"},
		"arr_empty":      []any{}, "arr_str": []any{"a", "b"}, "arr_num": []any{float64(1), float64(2)},
		"arr_bool": []any{true}, "arr_mixed": []any{"a", float64(1)}, "arr_null": []any{"a", nil},
		"str_json_arr": `["a"]`,
	}
	if name == "missing" {
		return map[string]any{}, true
	}
	value, ok := values[name]
	if !ok {
		return nil, false
	}
	return map[string]any{"value": value}, true
}

// The method bodies mirror the capture classes: each returns the parameter
// type and the value the platform handed to the method.
func lwcParamConversionBody(typeName string) string {
	switch typeName {
	case "List<String>":
		return `return 'List<String>:' + (value == null ? 'NULL' : 'n=' + value.size() + ':' + String.join(value, '|'));`
	case "Object":
		return `String kind = 'Other';
if (value == null) { kind = 'null'; }
else if (value instanceof String) { kind = 'String'; }
else if (value instanceof Integer) { kind = 'Integer'; }
else if (value instanceof Long) { kind = 'Long'; }
else if (value instanceof Double) { kind = 'Double'; }
else if (value instanceof Decimal) { kind = 'Decimal'; }
else if (value instanceof Boolean) { kind = 'Boolean'; }
else if (value instanceof List<Object>) { kind = 'List'; }
return 'Object:' + (value == null ? 'NULL' : String.valueOf(value)) + '#' + kind + '#' + (value == null ? 'NULL' : JSON.serialize(value));`
	}
	// The capture classes format through an Object parameter, which prints a
	// Date with a time component.
	return `Object boxed = value;
return '` + typeName + `:' + (boxed == null ? 'NULL' : String.valueOf(boxed));`
}

func lwcParamConversionMachine(t *testing.T, typeName, apiVersion string) *VM {
	t.Helper()
	program, err := CompileAnonymousWithOptions(lwcParamConversionBody(typeName), CompileOptions{APIVersion: apiVersion})
	if err != nil {
		t.Fatalf("%s: %v", typeName, err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{Name: "Convert.run", ClassName: "Convert", ReturnType: "String", IsStatic: true, APIVersion: apiVersion, Modifiers: []string{"AuraEnabled"}, Params: []Param{{Name: "value", Type: typeName}}, Program: program}); err != nil {
		t.Fatal(err)
	}
	return machine
}

// Every cell where the observations agree is asserted exactly. Where they
// differ, only the shared kind (all errors) is asserted.
func lwcParamConversionExpectation(cell *lwcParamCell) (want string, exact bool, mustFail bool, skip bool) {
	if len(cell.outcomes) == 1 {
		for outcome := range cell.outcomes {
			return outcome, true, strings.HasPrefix(outcome, "error:"), false
		}
	}
	for outcome := range cell.outcomes {
		if !strings.HasPrefix(outcome, "error:") {
			return "", false, false, true
		}
	}
	return "", false, true, false
}

func TestLWCParameterConversionNative(t *testing.T) {
	cells := lwcParamConversionCells(t)
	if len(cells) < 450 {
		t.Fatalf("expected at least 450 native cells, got %d", len(cells))
	}
	for _, apiVersion := range []string{"54.0", "62.0", "67.0"} {
		for _, cell := range cells {
			cell := cell
			args, ok := lwcParamConversionArgs(t, cell.argName)
			if !ok {
				t.Fatalf("no argument for case %q", cell.argName)
			}
			want, exact, mustFail, skip := lwcParamConversionExpectation(cell)
			if skip || lwcParamConversionParked[cell.typeName+"/"+cell.argName] {
				continue
			}
			t.Run(fmt.Sprintf("%s/%s/%s", apiVersion, cell.typeName, cell.argName), func(t *testing.T) {
				machine := lwcParamConversionMachine(t, cell.typeName, apiVersion)
				got, err := machine.InvokeLWCMethod("Convert", "run", args)
				if err != nil {
					t.Fatal(err)
				}
				if mustFail {
					if got.Success || got.Error == nil {
						t.Fatalf("native rejected %v; got %#v", args["value"], got.ReturnValue)
					}
					message := strings.TrimPrefix(want, "error:")
					// Parser positions and hosted entity identifiers (U#...) in the
					// other native messages belong to the hosted platform; only
					// the action parameter and SObject read errors are asserted.
					if exact && (strings.HasPrefix(message, "Value provided is invalid for action parameter") || message == "Unable to read SObject") {
						if got.Error.Type != "InvalidActionParameter" || got.Error.Message != message {
							t.Fatalf("error = %s: %s, want InvalidActionParameter: %s", got.Error.Type, got.Error.Message, message)
						}
					}
					return
				}
				if !got.Success {
					t.Fatalf("native accepted %v as %s; got error %v", args["value"], want, got.Error)
				}
				if got.ReturnValue != strings.TrimPrefix(want, "ok:") {
					t.Fatalf("result = %#v, want %s", got.ReturnValue, strings.TrimPrefix(want, "ok:"))
				}
			})
		}
	}
}
