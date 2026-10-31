package vm

import (
	"errors"
	"fmt"
	"testing"
)

func syntheticJSONClassGraph() (Value, Value, Value) {
	parent := Object("CycleNode")
	parent.classInstance, parent.projectClass = true, true
	parent.Fields["name"] = String("root")
	child := Object("CycleNode")
	child.classInstance, child.projectClass = true, true
	child.Fields["name"] = String("leaf")
	parent.Fields["children"] = List(child)
	nodes := Map()
	nodes.Type = "Map<String,CycleNode>"
	nodes.Map["root"] = parent
	return nodes, parent, child
}

// All guard probes use a finite graph of depth at most six. Seeding an active
// identity tests every descent edge needed by the cyclic graph without giving
// a broken converter a cycle to recurse into. Stop before overload scoring if
// any edge loses the active path or fails to cut a repeated identity.
func assertJSONCycleGuardBounded(t *testing.T) {
	t.Helper()
	nodes, parent, child := syntheticJSONClassGraph()
	ancestor := Object("CycleNode")
	ancestor.Fields["name"] = String("ancestor")
	child.Fields["parent"] = ancestor
	visited := make(map[jsonSerializationIdentity]bool)
	var checkDepth func(Value, int)
	checkDepth = func(value Value, depth int) {
		if depth > 6 {
			t.Fatal("guard probe exceeded its finite depth bound")
		}
		if identity, ok := jsonSerializationToken(value); ok {
			if visited[identity] {
				t.Fatal("guard probe must be acyclic before calling the converter")
			}
			visited[identity] = true
			defer delete(visited, identity)
		}
		for _, item := range value.Fields {
			checkDepth(item, depth+1)
		}
		for _, item := range value.Map {
			checkDepth(item, depth+1)
		}
		for _, item := range value.List {
			checkDepth(item, depth+1)
		}
	}
	checkDepth(nodes, 1)
	for _, value := range []Value{nodes, parent, parent.Fields["children"], child, ancestor} {
		identity, ok := jsonSerializationToken(value)
		if !ok {
			t.Fatal("guard probe needs a composite identity")
		}
		walker := jsonValueWalker{active: map[jsonSerializationIdentity]bool{identity: true}}
		err := func() (err error) {
			defer recoverJSONConversionError(&err)
			walker.convert(nodes, false)
			return nil
		}()
		assertJSONConversionException(t, err)
	}
}

func assertJSONConversionException(t *testing.T, err error) {
	t.Helper()
	var thrown *apexThrowError
	if !errors.As(err, &thrown) || thrown.value.Type != "System.JSONException" {
		t.Fatalf("raw conversion must raise System.JSONException, got %v", err)
	}
	if got := thrown.value.Fields["message"].Text; got != jsonCyclicConversionMessage {
		t.Fatalf("raw conversion exception message = %q", got)
	}
}

func syntheticJSONCyclicShapes() []struct {
	name, serialized string
	value            Value
} {
	nodes, parent, child := syntheticJSONClassGraph()
	child.Fields["parent"] = parent
	// A copied record shares its Fields map. Parent holds the record itself,
	// not its Id; conversion of Parent revisits that same backing map.
	record := Object("Account")
	record.Fields["Name"] = String("cycle")
	record.Fields["Parent"] = record
	return []struct {
		name, serialized string
		value            Value
	}{
		{"class-parent-child", `{"root":{"name":"root","children":[{"parent":null,"name":"leaf"}]}}`, nodes},
		{"record-backing-cycle", `{"attributes":{"type":"Account"},"Name":"cycle","Parent":null}`, record},
	}
}

func jsonCycleTestMachine(t *testing.T) *VM {
	t.Helper()
	machine := New(nil)
	for _, class := range []Class{
		{Name: "CycleNode", Fields: map[string]Field{
			"name":     {Name: "name", Type: "String"},
			"parent":   {Name: "parent", Type: "CycleNode"},
			"children": {Name: "children", Type: "List<CycleNode>"},
		}},
		{Name: "CycleOverloads"},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	return machine
}

func TestJSONRawCycleExceptionShapes(t *testing.T) {
	assertJSONCycleGuardBounded(t)
	machine := jsonCycleTestMachine(t)
	for _, tc := range syntheticJSONCyclicShapes() {
		t.Run(tc.name, func(t *testing.T) {
			for _, suppress := range []bool{false, true} {
				converted, err := jsonFromValueChecked(tc.value, suppress)
				assertJSONConversionException(t, err)
				if converted != nil {
					t.Fatal("checked conversion returned partial JSON after a cycle")
				}
				err = func() (err error) {
					defer recoverJSONConversionError(&err)
					jsonFromValue(tc.value, suppress)
					return nil
				}()
				assertJSONConversionException(t, err)
				got, err := jsonMarshalNoEscape(machine.jsonFromValueForSerialize(tc.value, suppress))
				if err != nil || string(got) != tc.serialized {
					t.Fatalf("canonical serialize suppress=%v: got=%s err=%v, want=%s", suppress, got, err, tc.serialized)
				}
			}
		})
	}
}

func TestJSONCycleOverloadScoringBounded(t *testing.T) {
	assertJSONCycleGuardBounded(t)
	machine := jsonCycleTestMachine(t)
	nodes, parent, child := syntheticJSONClassGraph()
	child.Fields["parent"] = parent
	candidates := []Method{
		{Name: "CycleOverloads.pick", ClassName: "CycleOverloads", IsStatic: true, Params: []Param{{Name: "node", Type: "CycleNode"}}},
		{Name: "CycleOverloads.pick", ClassName: "CycleOverloads", IsStatic: true, Params: []Param{{Name: "nodes", Type: "Map<String,CycleNode>"}}},
	}
	// The finite guard probes above fail before this entry if cycle protection
	// is lost. E21 C01/C02 chooses the Map overload without converting the
	// cyclic argument to the inapplicable class parameter.
	for _, candidate := range candidates {
		label := "map"
		if candidate.Params[0].Type == "CycleNode" {
			label = "node"
		}
		body, err := CompileAnonymous("return '" + label + "';")
		if err != nil {
			t.Fatal(err)
		}
		candidate.ReturnType, candidate.Program = "String", body
		if err := machine.RegisterMethod(candidate); err != nil {
			t.Fatal(err)
		}
	}
	got, err := machine.CallStatic("CycleOverloads.pick", []Value{nodes})
	if err != nil || got.Kind != ValueString || got.Text != "map" {
		t.Fatalf("E21 cyclic Map overload: result kind=%s text=%q err=%v", got.Kind, got.Text, err)
	}
}

func TestJSONMapOverloadReturnsForCyclicArguments(t *testing.T) {
	assertJSONCycleGuardBounded(t)
	for _, tc := range syntheticJSONCyclicShapes() {
		t.Run(tc.name, func(t *testing.T) {
			machine := jsonCycleTestMachine(t)
			paramType := "CycleNode"
			if tc.value.Kind == ValueObject {
				paramType = "Account"
			}
			mapType := "Map<String," + paramType + ">"
			for _, param := range []string{paramType, mapType} {
				label := "node"
				if param == mapType {
					label = "map"
				}
				body, err := CompileAnonymous("return '" + label + "';")
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterMethod(Method{Name: "CycleOverloads.pick", ClassName: "CycleOverloads", IsStatic: true, ReturnType: "String", Params: []Param{{Name: "value", Type: param}}, Program: body}); err != nil {
					t.Fatal(err)
				}
			}
			payload := tc.value
			if payload.Kind != ValueMap {
				payload = Map()
				payload.Type = mapType
				payload.Map["root"] = tc.value
			}
			machine.Globals["payload"] = payload
			program, err := CompileAnonymous("System.assertEquals('map', CycleOverloads.pick(payload));")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestJSONRawCycleExceptionUIBoundary(t *testing.T) {
	assertJSONCycleGuardBounded(t)
	for _, tc := range syntheticJSONCyclicShapes() {
		t.Run(tc.name, func(t *testing.T) {
			machine := jsonCycleTestMachine(t)
			if err := machine.RegisterClass(Class{Name: "CycleReturn", StaticFields: map[string]Field{
				"payload": {Name: "payload", Type: "Object", Static: true},
			}}); err != nil {
				t.Fatal(err)
			}
			if err := machine.ensureClassInitialized("CycleReturn"); err != nil {
				t.Fatal(err)
			}
			class, _ := machine.lookupClass("CycleReturn")
			machine.writeStaticFieldValue("CycleReturn", "payload", class, class.StaticFields["payload"], tc.value)
			body, err := CompileAnonymous("return CycleReturn.payload;")
			if err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterMethod(Method{Name: "CycleOverloads.emit", ClassName: "CycleOverloads", IsStatic: true, ReturnType: "Object", Modifiers: []string{"AuraEnabled"}, Program: body}); err != nil {
				t.Fatal(err)
			}
			out, err := machine.InvokeAuraAction("CycleOverloads", "emit", nil)
			if err != nil || out.Success || out.ReturnValue != nil || out.Error == nil || out.Error.Type != "System.JSONException" || out.Error.Message != jsonCyclicConversionMessage {
				t.Fatalf("raw conversion UI failure: result=%+v err=%v", out, err)
			}
		})
	}
}

func TestJSONSObjectAcyclicControls(t *testing.T) {
	child := Object("Account")
	child.Fields["Name"] = String("child")
	parent := Object("Account")
	parent.Fields["Name"] = String("parent")
	parent.Fields["Parent"] = child
	shared := Object("Account")
	shared.Fields["Left"] = parent
	shared.Fields["Right"] = parent
	chain := Object("Account")
	chain.Fields["Name"] = String("node-199")
	chainJSON := `{"attributes":{"type":"Account"},"Name":"node-199"}`
	for i := 198; i >= 0; i-- {
		next := Object("Account")
		next.Fields["Name"] = String(fmt.Sprintf("node-%d", i))
		next.Fields["Parent"] = chain
		chain = next
		chainJSON = fmt.Sprintf(`{"attributes":{"type":"Account"},"Name":"node-%d","Parent":%s}`, i, chainJSON)
	}
	childJSON := `{"attributes":{"type":"Account"},"Name":"child"}`
	parentJSON := `{"attributes":{"type":"Account"},"Name":"parent","Parent":` + childJSON + `}`
	for _, tc := range []struct {
		name  string
		value Value
		want  string
	}{
		{"deep-200", chain, chainJSON},
		{"shared-diamond", shared, `{"attributes":{"type":"Account"},"Left":` + parentJSON + `,"Right":` + parentJSON + `}`},
		{"parent-child", parent, parentJSON},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, suppress := range []bool{false, true} {
				for route, convert := range map[string]func(Value, bool) any{
					"raw":       jsonFromValue,
					"serialize": (&VM{}).jsonFromValueForSerialize,
				} {
					got, err := jsonMarshalNoEscape(convert(tc.value, suppress))
					if err != nil || string(got) != tc.want {
						t.Fatalf("%s suppress=%v: err=%v, output matches=%v", route, suppress, err, string(got) == tc.want)
					}
				}
			}
		})
	}
}
