package visualforce

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Form lifecycle r_immediate_true_valid and r_region_inside_valid capture the shared
// lifecycle boundaries. Select controls binding_*_{second,none}_runtime capture selection
// decoding. These local regressions verify that selections use those phases;
// they do not add Salesforce observations to either family's denominator.
func TestV07SelectionLifecycleBoundaries(t *testing.T) {
	for _, tag := range []string{"selectList", "selectRadio", "selectCheckboxes"} {
		t.Run(tag, func(t *testing.T) {
			for _, tc := range []struct {
				name          string
				immediate     bool
				region        bool
				outsidePosted bool
				empty         bool
				calls         int64
			}{
				{name: "ordinary", outsidePosted: true, calls: 2},
				{name: "immediate", immediate: true, outsidePosted: true},
				{name: "region_outside_absent", region: true, calls: 1},
				{name: "region_outside_posted", region: true, outsidePosted: true, calls: 1},
				{name: "region_inside_empty", region: true, empty: true, calls: 1},
			} {
				t.Run(tc.name, func(t *testing.T) {
					const controllerName = "V07LifecycleController"
					typeName := "String"
					value := func(text string) vm.Value { return vm.String(text) }
					if tag == "selectCheckboxes" {
						typeName = "List<String>"
						value = func(text string) vm.Value { return vm.List(vm.String(text)) }
					}
					fields := map[string]vm.Field{
						"calls":    {Name: "calls", Type: "Integer", InitialValue: vm.Int(0)},
						"actions":  {Name: "actions", Type: "Integer", InitialValue: vm.Int(0)},
						"aWasNull": {Name: "aWasNull", Type: "Boolean", InitialValue: vm.Bool(false)},
					}
					for property, initial := range map[string]string{"a": "a", "b": "c"} {
						program, err := vm.CompileAnonymous(`this.calls++; this.` + property + ` = value;`)
						if err != nil {
							t.Fatal(err)
						}
						setter := vm.Method{Name: controllerName + "." + property + ".set", ClassName: controllerName, Params: []vm.Param{{Name: "value", Type: typeName}}, Program: program}
						fields[property] = vm.Field{Name: property, Type: typeName, Property: true, HasGetter: true, HasSetter: true, Setter: &setter, InitialValue: value(initial)}
					}
					action, err := vm.CompileAnonymous(`this.actions++; this.aWasNull = this.a == null; return null;`)
					if err != nil {
						t.Fatal(err)
					}
					machine := testRunner(t)
					if err := machine.RegisterClass(vm.Class{Name: controllerName, Fields: fields, Methods: map[string]vm.Method{
						"submit": {Name: controllerName + ".submit", ClassName: controllerName, ReturnType: "PageReference", Program: action},
					}}); err != nil {
						t.Fatal(err)
					}
					control := func(property string) string {
						return fmt.Sprintf(`<apex:%s id="%s" value="{!%s}"><apex:selectOption itemValue="a" itemLabel="Alpha"/><apex:selectOption itemValue="b" itemLabel="Beta"/><apex:selectOption itemValue="c" itemLabel="Gamma"/></apex:%s>`, tag, property, property, tag)
					}
					inside := control("a") + fmt.Sprintf(`<apex:commandButton action="{!submit}" value="Submit" immediate="%t"/>`, tc.immediate)
					if tc.region {
						inside = `<apex:actionRegion>` + inside + `</apex:actionRegion>`
					}
					p, index := expressionContractPage(t, "V07Lifecycle", `<apex:page controller="`+controllerName+`"><apex:form>`+inside+control("b")+`</apex:form></apex:page>`)
					values := map[string]string{}
					if !tc.empty {
						values["a"] = "b"
					}
					if tc.outsidePosted {
						values["b"] = "a"
					}
					secret := []byte("Select controls local lifecycle regression")
					result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: index, Machine: machine, PageName: "V07Lifecycle", Action: "{!submit}", FormValues: values, ViewStateSecret: secret})
					if err != nil {
						t.Fatal(err)
					}
					payload, err := DecodeViewState(result.ViewState, secret)
					if err != nil {
						t.Fatal(err)
					}
					wantA, wantB := value("b"), value("a")
					if tc.immediate {
						wantA, wantB = value("a"), value("c")
					} else if tc.region {
						wantB = value("c")
						if tc.empty {
							wantA = vm.Null
							if tag == "selectCheckboxes" {
								wantA = vm.List()
							}
						}
					}
					for property, want := range map[string]vm.Value{"a": wantA, "b": wantB} {
						got, present := payload.ControllerValues[property]
						if want.Kind == vm.ValueNull {
							// View-state encoding omits scalar null fields. The action
							// observes the raw value before that projection occurs.
							if present || !payload.ControllerValues["aWasNull"].Bool {
								t.Errorf("%s: expected raw null before encoding and an omitted field, got %v (present %t)", property, got, present)
							}
							continue
						}
						if !present || got.Kind != want.Kind || got.String() != want.String() {
							t.Errorf("%s: got %v (%v), want %v (%v)", property, got, got.Kind, want, want.Kind)
						}
					}
					if got := payload.ControllerValues["calls"].Int; got != tc.calls {
						t.Errorf("setter calls: got %d, want %d", got, tc.calls)
					}
					if got := payload.ControllerValues["actions"].Int; got != 1 {
						t.Errorf("actions: got %d, want 1", got)
					}
				})
			}
		})
	}
}
