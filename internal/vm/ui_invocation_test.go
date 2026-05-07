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

func TestInvokeVisualforceActionUsesCurrentPageAndMessages(t *testing.T) {
	program, err := CompileAnonymous(`
String mode = ApexPages.currentPage().getParameters().get('mode');
ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.INFO, 'Saved ' + mode, 'detail ' + mode));
PageReference next = new PageReference('/apex/Done?mode=' + mode);
next.setRedirect(true);
return next;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "AccountController",
		Methods: map[string]Method{
			"save": {
				Name:       "AccountController.save",
				ClassName:  "AccountController",
				ReturnType: "PageReference",
				Program:    program,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	result, err := machine.InvokeVisualforceAction("AccountController", "save", "/apex/Edit", map[string]string{"mode": "quick"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("success = false: %#v", result.Error)
	}
	page, ok := result.ReturnValue.(map[string]any)
	if !ok || page["url"] != "/apex/Done?mode=quick" || page["redirect"] != true {
		t.Fatalf("returnValue = %#v", result.ReturnValue)
	}
	if len(result.PageMessages) != 1 {
		t.Fatalf("pageMessages = %#v", result.PageMessages)
	}
	message, ok := result.PageMessages[0].(map[string]any)
	if !ok || message["summary"] != "Saved quick" || message["detail"] != "detail quick" {
		t.Fatalf("pageMessages = %#v", result.PageMessages)
	}
	if len(result.Trace) == 0 {
		t.Fatalf("trace missing")
	}
}

func TestInvokeVisualforceActionReportsMissingInstanceAction(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "AccountController"}); err != nil {
		t.Fatal(err)
	}
	result, err := machine.InvokeVisualforceAction("AccountController", "save", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success || result.Error == nil || result.Error.Type != "UnsupportedFeature" {
		t.Fatalf("result = %#v", result)
	}
}
