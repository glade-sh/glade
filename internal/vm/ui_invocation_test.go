package vm

import (
	"testing"

	"github.com/open-aer/oaer/internal/trace"
)

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
	for _, event := range []string{
		"apex.visualforce.current_page",
		"apex.visualforce.controller.construct.start",
		"apex.visualforce.controller.construct.complete",
		"apex.visualforce.action.invoke",
		"apex.visualforce.action.complete",
	} {
		if !traceHas(result.Trace, event, "apex.visualforce") {
			t.Fatalf("trace missing %s: %#v", event, result.Trace)
		}
	}
	complete := findTraceEvent(result.Trace, "apex.visualforce.action.complete")
	if complete == nil {
		t.Fatalf("completion trace missing: %#v", result.Trace)
	}
	if complete.Args["className"] != "AccountController" || complete.Args["methodName"] != "save" {
		t.Fatalf("completion args = %#v", complete.Args)
	}
	if complete.Args["pageMessageCount"] != 1 {
		t.Fatalf("completion args = %#v", complete.Args)
	}
	pageRef, ok := complete.Args["pageReference"].(map[string]any)
	if !ok || pageRef["url"] != "/apex/Done?mode=quick" || pageRef["redirect"] != true {
		t.Fatalf("pageReference trace = %#v", complete.Args["pageReference"])
	}
	current := findTraceEvent(result.Trace, "apex.visualforce.current_page")
	if current == nil {
		t.Fatalf("current-page trace missing: %#v", result.Trace)
	}
	currentPage, ok := current.Args["page"].(map[string]any)
	if !ok || currentPage["url"] != "/apex/Edit" {
		t.Fatalf("current-page trace = %#v", current.Args["page"])
	}
	currentParams, ok := currentPage["parameters"].(map[string]any)
	if !ok || currentParams["mode"] != "quick" {
		t.Fatalf("current-page params = %#v", currentPage["parameters"])
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

func TestInvokeVisualforceActionTracesRuntimeError(t *testing.T) {
	program, err := CompileAnonymous(`
ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.ERROR, 'Before failure'));
throw new VisualforceException('blocked');
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
	result, err := machine.InvokeVisualforceAction("AccountController", "save", "/apex/Edit", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success || result.Error == nil || result.Error.Type != "VisualforceException" || result.Error.Message != "blocked" {
		t.Fatalf("result = %#v", result)
	}
	if len(result.PageMessages) != 1 {
		t.Fatalf("pageMessages = %#v", result.PageMessages)
	}
	if !traceHas(result.Trace, "apex.visualforce.action.error", "apex.visualforce") {
		t.Fatalf("trace missing action error: %#v", result.Trace)
	}
	errorEvent := findTraceEvent(result.Trace, "apex.visualforce.action.error")
	if errorEvent == nil {
		t.Fatalf("error trace missing: %#v", result.Trace)
	}
	if errorEvent.Args["errorType"] != "VisualforceException" || errorEvent.Args["error"] != "blocked" {
		t.Fatalf("error args = %#v", errorEvent.Args)
	}
	if errorEvent.Args["pageMessageCount"] != 1 {
		t.Fatalf("error args = %#v", errorEvent.Args)
	}
}

func findTraceEvent(events []trace.Event, name string) *trace.Event {
	for i := range events {
		if events[i].Name == name {
			return &events[i]
		}
	}
	return nil
}
