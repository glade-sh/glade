package vm

import (
	"strings"
	"testing"
)

// Native C002 observes the ancestor Id overload for an explicit Id null. R001,
// R004 and R005 preserve local/untyped and typed calls; C001 is the rejection twin.
func TestRuntimeStaticOverloadTypedIdNull(t *testing.T) {
	for _, tc := range []struct {
		name, call, want string
		ambiguous        bool
	}{
		{"typed Id", "Child.pick((Id)null)", "base-id", false},
		{"untyped null", "Child.pick(null)", "child-widget", false},
		{"typed Widget", "Child.pick((Widget)null)", "child-widget", false},
		{"base typed Id", "Base.pick((Id)null)", "base-id", false},
		{"base null rejection twin", "Base.pick(null)", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			for _, class := range []Class{{Name: "Widget"}, {Name: "Base"}, {Name: "Child", SuperClass: "Base"}} {
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			for _, spec := range []struct{ owner, param, value string }{
				{"Base", "Widget", "base-widget"}, {"Base", "Id", "base-id"}, {"Child", "Widget", "child-widget"},
			} {
				body, err := CompileAnonymous("return '" + spec.value + "';")
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterMethod(Method{Name: spec.owner + ".pick", ClassName: spec.owner, IsStatic: true, ReturnType: "String", Params: []Param{{Name: "value", Type: spec.param}}, Program: body}); err != nil {
					t.Fatal(err)
				}
			}
			program, err := CompileAnonymous("System.assert('" + tc.want + "'.equals(" + tc.call + "));")
			if err != nil {
				t.Fatal(err)
			}
			_, err = machine.Execute(program)
			if tc.ambiguous {
				if err == nil || !strings.Contains(err.Error(), "ambiguous") {
					t.Fatalf("rejection twin: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

// These matching controls keep the native all-null user-method rule from
// treating alias/provenance duplicates as overloads or widening its scope.
func TestRuntimeNullOverloadBoundaries(t *testing.T) {
	typedNull := Null
	typedNull.Type = "List<String>"
	for _, tc := range []struct {
		name        string
		left, right string
		args        []Value
		constructor bool
		generated   bool
		ambiguous   bool
	}{
		{name: "bare collection", left: "Object", right: "List<String>", args: []Value{Null}, ambiguous: true},
		{name: "typed collection", left: "Object", right: "List<String>", args: []Value{typedNull}},
		{name: "same types different parameter names", left: "Object", right: "Object", args: []Value{Null}},
		{name: "scalar alias", left: "String", right: "System.String", args: []Value{Null}},
		{name: "nested alias", left: "List<String>", right: "List<System.String>", args: []Value{Null}},
		{name: "map aliases", left: "Map<Id,List<String>>", right: "Map<System.Id,List<System.String>>", args: []Value{Null}},
		{name: "zero arguments"},
		{name: "constructors", left: "Object", right: "String", args: []Value{Null}, constructor: true},
		{name: "platform methods", left: "Object", right: "String", args: []Value{Null}, generated: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			methods := []Method{{Name: "Probe.pick", IsConstructor: tc.constructor}, {Name: "Probe.pick", IsConstructor: tc.constructor}}
			if tc.left != "" {
				methods[0].Params = []Param{{Name: "first", Type: tc.left}}
				methods[1].Params = []Param{{Name: "second", Type: tc.right}}
			}
			if tc.generated {
				for i := range methods {
					methods[i].Modifiers = []string{"passive-generated"}
				}
			}
			_, found, ambiguous := New(nil).matchMethodByArgs(methods, tc.args)
			if ambiguous != tc.ambiguous || found == tc.ambiguous {
				t.Fatalf("found=%v ambiguous=%v, want ambiguity=%v", found, ambiguous, tc.ambiguous)
			}
		})
	}
}

// R001/R003/R004/R005 capture these calls at both API endpoints and routes.
// Exercise dispatch directly so semantic selection cannot hide VM ambiguity.
func TestRuntimeMethodApplicabilityPreservesDeclaredArguments(t *testing.T) {
	machine := New(nil)
	for _, class := range []Class{
		{Name: "BaseValue"},
		{Name: "NarrowValue", SuperClass: "BaseValue"},
		{Name: "LeafValue", SuperClass: "NarrowValue"},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	held := Object("LeafValue")
	held.Static = "BaseValue"
	typedNull := Null
	typedNull.Type = "BaseValue"
	id := String("001000000000001")
	id.Type = "Id"
	choose := []Method{
		{Name: "Probe.choose", Params: []Param{{Type: "NarrowValue"}, {Type: "LeafValue"}}},
		{Name: "Probe.choose", Params: []Param{{Type: "BaseValue"}, {Type: "BaseValue"}}},
	}
	for _, tc := range []struct {
		name       string
		methods    []Method
		args       []Value
		parameters string
	}{
		{name: "exact Id with bare null", methods: []Method{
			{Name: "Probe.bare", Params: []Param{{Type: "Id"}, {Type: "Map<Id,List<String>>"}}},
			{Name: "Probe.bare", Params: []Param{{Type: "String"}, {Type: "NarrowValue"}}},
		}, args: []Value{id, Null}, parameters: "(Id, Map<Id,List<String>>)"},
		{name: "base holding leaf", methods: choose, args: []Value{Object("NarrowValue"), held}, parameters: "(BaseValue, BaseValue)"},
		{name: "typed base null", methods: choose, args: []Value{Object("NarrowValue"), typedNull}, parameters: "(BaseValue, BaseValue)"},
		{name: "explicit leaf", methods: choose, args: []Value{Object("NarrowValue"), Object("LeafValue")}, parameters: "(NarrowValue, LeafValue)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			method, found, ambiguous := machine.matchMethodByArgs(tc.methods, tc.args)
			if !found || ambiguous || methodParamSignature(method) != tc.parameters {
				t.Fatalf("found=%v ambiguous=%v signature=%s, want %s", found, ambiguous, methodParamSignature(method), tc.parameters)
			}
		})
	}
}
