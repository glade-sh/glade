package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// Exercise lexical type declarations with compiled synthetic method bodies.
func dispatchProvenanceVM(t *testing.T, api string) *VM {
	t.Helper()
	machine := New(nil)
	for _, class := range []Class{
		{Name: "DispatchContextProbe"},
		{Name: "DispatchContextProbe.Model", IsInterface: true},
		{Name: "DispatchContextProbe.Detail", Interfaces: []string{"DispatchContextProbe.Model"}, Fields: map[string]Field{"count": {Name: "count", Type: "Integer"}}},
		{Name: "DispatchContextProbe.Wrapper", Fields: map[string]Field{"child": {Name: "child", Type: "Holder"}}},
		{Name: "DispatchContextProbe.Holder", Fields: map[string]Field{"value": {Name: "value", Type: "String"}}},
		// Both the top-level and caller-local name shadow the owner's nested type.
		{Name: "ProbeItem"}, {Name: "DispatchContextProbe.ProbeItem"},
		{Name: "ProbeEnvelope", Fields: map[string]Field{"child": {Name: "child", Type: "ProbeItem"}}},
		{Name: "ProbeEnvelope.ProbeItem", Fields: map[string]Field{"value": {Name: "value", Type: "String"}}},
		{Name: "DerivedEnvelope", SuperClass: "ProbeEnvelope"},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	methods := []struct {
		name, returns, body string
		static              bool
		params              []Param
	}{
		{"takeDetail", "String", "return 'detail';", true, []Param{{Name: "value", Type: "Detail"}}},
		{"takeInstanceDetail", "String", "return 'detail';", false, []Param{{Name: "value", Type: "Detail"}}},
		{"takeText", "String", "return value == null ? 'null' : value;", true, []Param{{Name: "value", Type: "String"}}},
		{"makeDetails", "List<Detail>", `Model item = new Detail(); List<Detail> typedValues = new List<Detail>(); List<Model> values = (List<Model>)typedValues; values.add(item); return (List<Detail>)values;`, true, nil},
	}
	for _, m := range methods {
		program, err := CompileAnonymousWithOptions(m.body, CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterMethod(Method{Name: "DispatchContextProbe." + m.name, ClassName: "DispatchContextProbe", IsStatic: m.static, ReturnType: m.returns, Params: m.params, Program: program}); err != nil {
			t.Fatal(err)
		}
	}
	return machine
}

func runDispatchProvenance(t *testing.T, machine *VM, api, body string) error {
	t.Helper()
	program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "DispatchContextProbe.run", ClassName: "DispatchContextProbe", IsStatic: true, Program: program}); err != nil {
		t.Fatal(err)
	}
	entry, err := CompileAnonymousWithOptions("DispatchContextProbe.run();", CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(entry)
	return err
}

func TestDispatchCollectionElementDeclaredType(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, read := range []string{"values[0]", "values.get(0)"} {
			for _, call := range []string{"takeDetail", "new DispatchContextProbe().takeInstanceDetail"} {
				t.Run(api+"/"+read+"/"+call, func(t *testing.T) {
					machine := dispatchProvenanceVM(t, api)
					err := runDispatchProvenance(t, machine, api, `List<Detail> values = makeDetails(); System.assertEquals('detail', `+call+`(`+read+`));`)
					if err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestDispatchCollectionReadViewPreservesAliases(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, kind := range []string{"List", "Map"} {
			for _, read := range []string{"values[0]", "values.get(0)"} {
				t.Run(api+"/"+kind+"/"+read, func(t *testing.T) {
					machine := dispatchProvenanceVM(t, api)
					item := Object("DispatchContextProbe.Detail")
					item.Static = "DispatchContextProbe.Model"
					item.Fields["count"] = Int(1)
					values := typedList("List<DispatchContextProbe.Detail>")
					if kind == "List" {
						values.List = []Value{item}
					} else {
						values = typedMap("Map<Integer,DispatchContextProbe.Detail>")
						values.Map[mapKey(Int(0))] = item
					}
					machine.Globals["values"] = values
					machine.VarTypes["values"] = values.Type
					machine.Globals["alias"] = item
					machine.VarTypes["alias"] = "DispatchContextProbe.Model"
					program, err := CompileAnonymousWithOptions(`System.assertEquals('detail', DispatchContextProbe.takeDetail(`+read+`));`, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = machine.Execute(program)
					// R007 rejects Map brackets before overload dispatch. R008
					// backs the declared read view and alias checks through get().
					if kind == "Map" && read == "values[0]" {
						if err == nil || !strings.Contains(err.Error(), "Expression must be a list type: Map<Integer,DispatchContextProbe.Detail>") {
							t.Fatalf("want native Map bracket rejection, got %v", err)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					receiver := ir.Expr{Kind: ir.ExprVariable, Name: "values"}
					value, err := machine.eval(ir.Expr{Kind: ir.ExprCall, Callee: "get", Left: &receiver, Args: []ir.Expr{{Kind: ir.ExprLiteral, Value: "0"}}}, &Result{})
					if err != nil {
						t.Fatal(err)
					}
					if value.Ref != item.Ref {
						t.Fatal("read changed object identity")
					}

					if machine.Globals["alias"].Static != item.Static {
						t.Fatal("read retyped another alias")
					}
					stored := machine.Globals["values"].List
					storedItem := item
					if kind == "List" {
						storedItem = stored[0]
					} else {
						storedItem = machine.Globals["values"].Map[mapKey(Int(0))]
					}
					if storedItem.Static != item.Static || storedItem.Ref != item.Ref {
						t.Fatal("read retyped stored element")
					}
					value.Fields["count"] = Int(2)
					if storedItem.Fields["count"].Int != 2 || machine.Globals["alias"].Fields["count"].Int != 2 {
						t.Fatal("read detached mutable object contents")
					}
				})
			}
		}
	}
}

func TestDispatchSafeNavigationFinalFieldType(t *testing.T) {
	cases := map[string]string{
		"safeNavigationFinalFieldType":             `Wrapper root = null; System.assertEquals('null', takeText(root?.child?.value));`,
		"externalSafeNavigationShadowedNestedType": `ProbeEnvelope root = null; System.assertEquals('null', takeText(root?.child?.value));`,
		"inheritedOwner":                           `DerivedEnvelope root = null; System.assertEquals('null', takeText(root?.child?.value));`,
		"nullChild":                                `ProbeEnvelope root = new ProbeEnvelope(); System.assertEquals('null', takeText(root?.child?.value));`,
		"nonnullChain":                             `ProbeEnvelope root = new ProbeEnvelope(); root.child = new ProbeEnvelope.ProbeItem(); root.child.value = 'text'; System.assertEquals('text', takeText(root?.child?.value));`,
	}
	for _, api := range []string{"62.0", "67.0"} {
		for name, body := range cases {
			t.Run(api+"/"+name, func(t *testing.T) {
				if err := runDispatchProvenance(t, dispatchProvenanceVM(t, api), api, body); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestDispatchTypeProvenanceRejection(t *testing.T) {
	cases := map[string]string{
		"listIndexBroaderType":         `List<Model> values = new List<Model>{new Detail()}; takeDetail(values[0]);`,
		"listGetBroaderType":           `List<Model> values = new List<Model>{new Detail()}; takeDetail(values.get(0));`,
		"mapGetBroaderType":            `Map<Integer,Model> values = new Map<Integer,Model>{0 => new Detail()}; takeDetail(values.get(0));`,
		"mapIndexBroaderType":          `Map<Integer,Model> values = new Map<Integer,Model>{0 => new Detail()}; takeDetail(values[0]);`,
		"listNullBroaderType":          `List<Model> values = new List<Model>{null}; takeDetail(values[0]);`,
		"mapMissingBroaderType":        `Map<Integer,Model> values = new Map<Integer,Model>(); takeDetail(values.get(0));`,
		"safeNavigationWrongFinalType": `Wrapper root = null; takeDetail(root?.child);`,
		"externalWrongFinalType":       `ProbeEnvelope root = null; takeDetail(root?.child);`,
	}
	for _, api := range []string{"62.0", "67.0"} {
		for name, body := range cases {
			t.Run(api+"/"+name, func(t *testing.T) {
				err := runDispatchProvenance(t, dispatchProvenanceVM(t, api), api, body)
				want := `unsupported call "takeDetail"`
				if name == "mapIndexBroaderType" {
					want = "Expression must be a list type: Map<Integer,DispatchContextProbe.Model>"
				}
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("want inapplicable argument rejection, got %v", err)
				}
			})
		}
	}
}
