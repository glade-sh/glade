package vm

import "testing"

func TestInvokeLWCMethodSerializesWrapperReturn(t *testing.T) {
	program, err := CompileAnonymous(`
Wrapper out = new Wrapper();
out.name = name;
out.count = 2;
return out;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Wrapper",
		Fields: map[string]Field{
			"name":  {Name: "name", Type: "String"},
			"count": {Name: "count", Type: "Integer"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:       "WidgetController.getWidget",
		ClassName:  "WidgetController",
		ReturnType: "Wrapper",
		IsStatic:   true,
		Modifiers:  []string{"AuraEnabled"},
		Params:     []Param{{Name: "name", Type: "String"}},
		Program:    program,
	}); err != nil {
		t.Fatal(err)
	}
	result, err := machine.InvokeLWCMethod("WidgetController", "getWidget", map[string]any{"name": "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("success = false: %#v", result.Error)
	}
	fields, ok := result.ReturnValue.(map[string]any)
	if !ok || fields["name"] != "Acme" || fields["count"] != int64(2) {
		t.Fatalf("returnValue = %#v", result.ReturnValue)
	}
	if len(result.Trace) == 0 {
		t.Fatalf("trace missing")
	}
}

func TestInvokeAuraActionReturnsAuraHandledExceptionShape(t *testing.T) {
	program, err := CompileAnonymous(`throw new AuraHandledException('blocked');`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "WidgetController.save",
		ClassName:  "WidgetController",
		ReturnType: "void",
		IsStatic:   true,
		Modifiers:  []string{"AuraEnabled"},
		Program:    program,
	}); err != nil {
		t.Fatal(err)
	}
	result, err := machine.InvokeAuraAction("WidgetController", "save", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success || result.Error == nil || result.Error.Type != "AuraHandledException" || result.Error.Message != "blocked" {
		t.Fatalf("result = %#v", result)
	}
}
