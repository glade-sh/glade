package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

func TestConstructControllerWithTraceEnabledDoesNotPanicOnNilResult(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:   "PageController",
		Access: "public",
		Constructors: []Method{{
			Name:          "PageController.<init>",
			ClassName:     "PageController",
			IsConstructor: true,
			Access:        "public",
			Program: ir.Program{Instructions: []ir.Instruction{{
				Op: ir.OpReturn,
			}}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.ConstructController("PageController"); err != nil {
		t.Fatalf("ConstructController() err = %v", err)
	}
}

func TestReadInstancePropertyResolvesNoArgGetterMethod(t *testing.T) {
	getRows, err := CompileAnonymous(`return new List<String>{'alpha', 'bravo', 'charlie'};`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "PageController",
		Methods: map[string]Method{
			"getRows": {Name: "PageController.getRows", ClassName: "PageController", ReturnType: "List<String>", Program: getRows},
		},
	}); err != nil {
		t.Fatal(err)
	}
	controller, err := machine.ConstructController("PageController")
	if err != nil {
		t.Fatal(err)
	}

	value, ok, err := machine.ReadInstanceProperty(controller, "rows")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || value.Kind != ValueList || len(value.List) != 3 || value.List[0].Text != "alpha" || value.List[2].Text != "charlie" {
		t.Fatalf("ReadInstanceProperty(rows) = %#v ok=%v", value, ok)
	}
}

func TestReadInstancePropertyRejectsPrivateGetterMethod(t *testing.T) {
	getSecret, err := CompileAnonymous(`return 'hidden';`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "PageController",
		Methods: map[string]Method{
			"getSecret": {
				Name:       "PageController.getSecret",
				ClassName:  "PageController",
				ReturnType: "String",
				Access:     "private",
				Program:    getSecret,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	controller, err := machine.ConstructController("PageController")
	if err != nil {
		t.Fatal(err)
	}

	_, ok, err := machine.ReadInstanceProperty(controller, "secret")
	if !ok || err == nil {
		t.Fatalf("ReadInstanceProperty(secret) ok=%v err=%v, want private access error", ok, err)
	}
}
