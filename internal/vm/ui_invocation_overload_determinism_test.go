package vm

import (
	"bufio"
	"github.com/glade-sh/glade/internal/ir"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Native Lightning Out observations for equal-arity AuraEnabled overloads of
// API 45 and 54 classes (the overloads do not deploy at API 55 or later):
// aura_overload_determinism_scalar/org.tsv and aura_overload_determinism_ranking/org.tsv.
// Selection did not vary with declaration order, deploy order, class-name
// position, Apex API, component API, repeat or argument value.

func determinismRows(t *testing.T, fixture string) []map[string]string {
	t.Helper()
	file, err := os.Open(filepath.Join("..", "apextest", "testdata", "conformance", fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	var header []string
	var rows []map[string]string
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if header == nil {
			header = fields
			continue
		}
		if len(fields) != len(header) {
			t.Fatalf("%s: row has %d fields, want %d: %q", fixture, len(fields), len(header), scanner.Text())
		}
		row := map[string]string{}
		for i, name := range header {
			row[name] = fields[i]
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}

func overloadPair(t *testing.T, api, className string, types ...string) *VM {
	t.Helper()
	machine := New(nil)
	for _, typeName := range types {
		program, err := CompileAnonymous(`return '` + typeName + `';`)
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterMethod(Method{Name: className + ".pick", ClassName: className, ReturnType: "String", IsStatic: true, APIVersion: api, Modifiers: []string{"AuraEnabled"}, Params: []Param{{Name: "value", Type: typeName}}, Program: program}); err != nil {
			t.Fatal(err)
		}
	}
	return machine
}

func determinismArgs(t *testing.T, name string) map[string]any {
	t.Helper()
	switch name {
	case "missing":
		return map[string]any{}
	case "null":
		return map[string]any{"value": nil}
	case "str":
		return map[string]any{"value": "x"}
	case "num":
		return map[string]any{"value": float64(42)}
	}
	t.Fatalf("unknown case %q", name)
	return nil
}

func TestHistoricalLightningRankedOverloadsNative(t *testing.T) {
	type observation struct{ api, first, second, name, outcome string }
	seen := map[observation]bool{}
	for _, row := range determinismRows(t, "aura_overload_determinism_ranking") {
		o := observation{row["apex_api"], row["first_type"], row["second_type"], row["case"], row["outcome"]}
		decl := []string{o.first, o.second}
		if row["decl"] == "reverse" {
			decl = []string{o.second, o.first}
		}
		o.name = o.name + "/" + strings.Join(decl, ",")
		if seen[o] {
			continue
		}
		seen[o] = true
		caseName := strings.SplitN(o.name, "/", 2)[0]
		t.Run(o.api+"/"+o.name, func(t *testing.T) {
			machine := overloadPair(t, o.api, "RankController", decl...)
			got, err := machine.InvokeLWCMethod("RankController", "pick", determinismArgs(t, caseName))
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(o.outcome, "error:") {
				if got.Success || got.Error == nil {
					t.Fatalf("native observed %s; got %#v", o.outcome, got)
				}
				return
			}
			if !got.Success || got.ReturnValue != o.outcome {
				t.Fatalf("result=%#v error=%#v want %s", got.ReturnValue, got.Error, o.outcome)
			}
		})
	}
	if len(seen) < 400 {
		t.Fatalf("expected at least 400 distinct observations, got %d", len(seen))
	}
}

func TestHistoricalLightningStringIntegerOverloadsNative(t *testing.T) {
	want := map[string]string{"missing": "string:null", "null": "string:null", "numeric": "string:42", "string": "string:bad", "bool": "string:true", "decimal": "string:1.5"}
	args := map[string]map[string]any{
		"missing": {}, "null": {"value": nil}, "numeric": {"value": float64(42)},
		"string": {"value": "bad"}, "bool": {"value": true}, "decimal": {"value": 1.5},
	}
	checked := 0
	seen := map[string]bool{}
	for _, row := range determinismRows(t, "aura_overload_determinism_scalar") {
		if row["decl"] == "control" || row["case"] == "overloaded-declaration" {
			continue
		}
		for _, outcome := range strings.Split(row["outcomes"], "|") {
			if outcome != "ok:"+want[row["case"]] {
				t.Fatalf("fixture row is not uniform: %v", row)
			}
		}
		key := row["apex_api"] + "/" + row["decl"] + "/" + row["case"]
		if seen[key] {
			continue
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			first, second := "String", "Integer"
			if row["decl"] == "integer-first" {
				first, second = second, first
			}
			text, err := CompileAnonymous(`return 'string:' + String.valueOf(value);`)
			if err != nil {
				t.Fatal(err)
			}
			integer, err := CompileAnonymous(`return 'integer:' + String.valueOf(value);`)
			if err != nil {
				t.Fatal(err)
			}
			programs := map[string]ir.Program{"String": text, "Integer": integer}
			machine := New(nil)
			for _, typeName := range []string{first, second} {
				if err := machine.RegisterMethod(Method{Name: "TieController.pick", ClassName: "TieController", ReturnType: "String", IsStatic: true, APIVersion: row["apex_api"], Modifiers: []string{"AuraEnabled"}, Params: []Param{{Name: "value", Type: typeName}}, Program: programs[typeName]}); err != nil {
					t.Fatal(err)
				}
			}
			got, err := machine.InvokeLWCMethod("TieController", "pick", args[row["case"]])
			if err != nil || !got.Success || got.ReturnValue != want[row["case"]] {
				t.Fatalf("result=%#v error=%v want %s", got, err, want[row["case"]])
			}
		})
		checked++
	}
	if checked < 24 {
		t.Fatalf("expected at least 24 distinct native cells, got %d", checked)
	}
}

// Cases the native rows do not cover keep the existing resolver behavior.
func TestHistoricalLightningUnrankedOverloadsKeepUnsupported(t *testing.T) {
	cases := []struct {
		name   string
		api    string
		params [][]Param
	}{
		{"unranked collection", "54.0", [][]Param{{{Name: "value", Type: "String"}}, {{Name: "value", Type: "Map<String,String>"}}}},
		{"two parameters", "54.0", [][]Param{{{Name: "a", Type: "String"}, {Name: "b", Type: "Integer"}}, {{Name: "a", Type: "String"}, {Name: "b", Type: "Boolean"}}}},
		{"current API", "55.0", [][]Param{{{Name: "value", Type: "String"}}, {{Name: "value", Type: "Integer"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(`return 'x';`)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			for _, params := range tc.params {
				if err := machine.RegisterMethod(Method{Name: "OtherController.pick", ClassName: "OtherController", ReturnType: "String", IsStatic: true, APIVersion: tc.api, Modifiers: []string{"AuraEnabled"}, Params: params, Program: program}); err != nil {
					t.Fatal(err)
				}
			}
			got, err := machine.InvokeLWCMethod("OtherController", "pick", map[string]any{})
			if err != nil {
				t.Fatal(err)
			}
			if got.Success || got.Error == nil || got.Error.Code != "GLADELWC013" {
				t.Fatalf("expected the unsupported overload result, got %#v", got)
			}
		})
	}
}
