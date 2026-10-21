package vm

import "testing"

func staticInterfaceResultScopeFixture(t *testing.T, api string) (*VM, Value, Method, Method) {
	t.Helper()
	machine := staticInterfaceNative52Machine(t)
	machine.currentMethod.APIVersion = api
	for _, name := range []string{"OwnedAction", "OwnedStaticAction"} {
		class := machine.Classes[name]
		class.APIVersion = api
		machine.Classes[name] = class
	}
	surface, _ := machine.registeredMethod("OwnedAction.run")
	candidate, _ := machine.registeredMethod("OwnedStaticAction.run")
	for _, method := range []*Method{&surface, &candidate} {
		method.APIVersion, method.ReturnType = api, "Object"
		method.Params = []Param{{Name: "term", Type: "Object"}, {Name: "options", Type: "Map<String,Object>"}}
	}
	return machine, Object("OwnedStaticAction"), surface, candidate
}

// The owned result fixture observes precisely this formal signature at each
// matched source API. Values must not change which implementation is selected.
func TestStaticInterfaceResultObservedSignature(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			machine, receiver, surface, candidate := staticInterfaceResultScopeFixture(t, api)
			if !machine.observedStaticInterfaceImplementation(receiver, surface, candidate) {
				t.Fatal("captured static result signature excluded")
			}
		})
	}
}

// Retain unobserved adjacent signatures and source versions on their existing
// resolution paths; these controls do not claim native results for them.
func TestStaticInterfaceResultUnobservedSignatureControls(t *testing.T) {
	for _, tc := range []struct {
		name   string
		modify func(*VM, *Method, *Method)
	}{
		{"String result", func(_ *VM, s, c *Method) { s.ReturnType, c.ReturnType = "String", "String" }},
		{"void result", func(_ *VM, s, c *Method) { s.ReturnType, c.ReturnType = "void", "void" }},
		{"String parameter", func(_ *VM, s, c *Method) { s.Params[0].Type, c.Params[0].Type = "String", "String" }},
		{"different map", func(_ *VM, s, c *Method) {
			s.Params[1].Type, c.Params[1].Type = "Map<String,String>", "Map<String,String>"
		}},
		{"zero parameters", func(_ *VM, s, c *Method) { s.Params, c.Params = nil, nil }},
		{"one parameter", func(_ *VM, s, c *Method) { s.Params, c.Params = s.Params[:1], c.Params[:1] }},
		{"extra parameter", func(_ *VM, s, c *Method) {
			s.Params = append(s.Params, Param{Type: "Object"})
			c.Params = append(c.Params, Param{Type: "Object"})
		}},
		{"source52", func(m *VM, _, _ *Method) { m.currentMethod.APIVersion = "52.0" }},
		{"source64", func(m *VM, _, _ *Method) { m.currentMethod.APIVersion = "64.0" }},
		{"mixed candidate", func(_ *VM, _, c *Method) { c.APIVersion = "62.0" }},
		{"mixed contract", func(m *VM, _, _ *Method) {
			class := m.Classes["OwnedAction"]
			class.APIVersion = "62.0"
			m.Classes["OwnedAction"] = class
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine, receiver, surface, candidate := staticInterfaceResultScopeFixture(t, "67.0")
			tc.modify(machine, &surface, &candidate)
			if machine.observedStaticInterfaceImplementation(receiver, surface, candidate) {
				t.Fatal("unobserved signature crossed the static implementation boundary")
			}
		})
	}
}
