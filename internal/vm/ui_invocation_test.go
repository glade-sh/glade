package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/trace"
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

func TestInvokeLWCMethodUsesCurrentPageReference(t *testing.T) {
	program, err := CompileAnonymous(`
PageReference page = System.currentPageReference();
page.getParameters().put('wire', recordId);
return page.getUrl() + ':' + page.getParameters().get('mode') + ':' + page.getParameters().get('wire');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	machine.SetCurrentPageURL("/apex/Host?mode=local")
	if err := machine.RegisterMethod(Method{
		Name:       "ItemCtrl.getItems",
		ClassName:  "ItemCtrl",
		ReturnType: "String",
		IsStatic:   true,
		Modifiers:  []string{"AuraEnabled"},
		Params:     []Param{{Name: "recordId", Type: "String"}},
		Program:    program,
	}); err != nil {
		t.Fatal(err)
	}
	result, err := machine.InvokeLWCMethod("ItemCtrl", "getItems", map[string]any{"recordId": "001XX0000000001"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("success = false: %#v", result.Error)
	}
	if result.ReturnValue != "/apex/Host?mode=local&wire=001XX0000000001:local:001XX0000000001" {
		t.Fatalf("returnValue = %#v", result.ReturnValue)
	}
	if got := machine.CurrentPage().Fields["parameters"].Map[mapKey(String("wire"))]; got.Kind != ValueString || got.Text != "001XX0000000001" {
		t.Fatalf("current page params = %#v", machine.CurrentPage().Fields["parameters"])
	}
}

func TestInvokeLWCMethodRejectsOverloadedAuraEnabledMethods(t *testing.T) {
	firstProgram, err := CompileAnonymous(`return 'first';`)
	if err != nil {
		t.Fatal(err)
	}
	secondProgram, err := CompileAnonymous(`return 'second';`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, method := range []Method{
		{
			Name:       "WidgetController.load",
			ClassName:  "WidgetController",
			ReturnType: "String",
			IsStatic:   true,
			Modifiers:  []string{"AuraEnabled"},
			Params:     []Param{{Name: "value", Type: "String"}},
			Program:    firstProgram,
		},
		{
			Name:       "WidgetController.load",
			ClassName:  "WidgetController",
			ReturnType: "String",
			IsStatic:   true,
			Modifiers:  []string{"AuraEnabled"},
			Params:     []Param{{Name: "value", Type: "Object"}},
			Program:    secondProgram,
		},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	result, err := machine.InvokeLWCMethod("WidgetController", "load", map[string]any{"value": "Acme"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Success || result.Error == nil {
		t.Fatalf("result = %#v", result)
	}
	if result.Error.Code != "GLADELWC013" || result.Error.Type != "UnsupportedFeature" || result.Error.Message != "overloaded AuraEnabled method unsupported" {
		t.Fatalf("error = %#v", result.Error)
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

func TestPaginationCursorAuraSerialization(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`return Database.getPaginationCursor('SELECT Id FROM Account');`, CompileOptions{APIVersion: "66.0"})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterMethod(Method{Name: "CursorController.load", ClassName: "CursorController", ReturnType: "Database.PaginationCursor", IsStatic: true, Modifiers: []string{"AuraEnabled"}, APIVersion: "66.0", Program: program}); err != nil {
		t.Fatal(err)
	}
	result, err := machine.InvokeAuraAction("CursorController", "load", nil)
	if err != nil || !result.Success {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	cursor, ok := result.ReturnValue.(map[string]any)
	if !ok || cursor["Query"] != "SELECT Id FROM Account" {
		t.Fatalf("returnValue = %#v", result.ReturnValue)
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
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
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
	for _, event := range []string{
		"apex.visualforce.controller.construct",
		"apex.visualforce.action",
	} {
		span := findTraceEvent(result.Trace, event)
		if span == nil || span.Phase != trace.PhaseComplete || span.Duration <= 0 {
			t.Fatalf("duration trace missing %s: %#v", event, result.Trace)
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

func TestInvokeVisualforceActionResetsPageMessagesPerRequest(t *testing.T) {
	saveProgram, err := CompileAnonymous(`
ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.INFO, 'Saved'));
return null;
`)
	if err != nil {
		t.Fatal(err)
	}
	quietProgram, err := CompileAnonymous(`return null;`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "AccountController",
		Methods: map[string]Method{
			"save": {
				Name:       "AccountController.save",
				ClassName:  "AccountController",
				ReturnType: "PageReference",
				Program:    saveProgram,
			},
			"quiet": {
				Name:       "AccountController.quiet",
				ClassName:  "AccountController",
				ReturnType: "PageReference",
				Program:    quietProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	first, err := machine.InvokeVisualforceAction("AccountController", "save", "/apex/Edit", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Success || len(first.PageMessages) != 1 {
		t.Fatalf("first result = %#v", first)
	}
	second, err := machine.InvokeVisualforceAction("AccountController", "quiet", "/apex/Edit", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Success {
		t.Fatalf("second success = false: %#v", second.Error)
	}
	if len(second.PageMessages) != 0 {
		t.Fatalf("second pageMessages = %#v", second.PageMessages)
	}
}

func TestInvokeVisualforceActionOnControllerMergesPageQueryAndPostedParams(t *testing.T) {
	program, err := CompileAnonymous(`
return ApexPages.currentPage().getParameters().get('id') + ':' + ApexPages.currentPage().getParameters().get('mode');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "EditController",
		Methods: map[string]Method{
			"save": {
				Name:       "EditController.save",
				ClassName:  "EditController",
				ReturnType: "String",
				Program:    program,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	value, _, result, err := machine.InvokeVisualforceActionOnController(Object("EditController"), "EditController", "save", "/apex/Edit?id=001xx000003DGbY", map[string]string{"mode": "quick"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success || result.Error != nil {
		t.Fatalf("result = %#v", result)
	}
	if value.Kind != ValueString || value.Text != "001xx000003DGbY:quick" {
		t.Fatalf("value = %#v", value)
	}
}

func TestInvokeVisualforceActionOnControllerRejectsPrivateAction(t *testing.T) {
	program, err := CompileAnonymous(`return null;`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "EditController",
		Methods: map[string]Method{
			"save": {
				Name:       "EditController.save",
				ClassName:  "EditController",
				ReturnType: "PageReference",
				Access:     "private",
				Program:    program,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	_, _, result, err := machine.InvokeVisualforceActionOnController(Object("EditController"), "EditController", "save", "/apex/Edit", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success || result.Error == nil {
		t.Fatalf("result = %#v", result)
	}
}

func TestInvokeVisualforceActionTracesStandardControllerActions(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'VF Trace');
ApexPages.StandardController controller = new ApexPages.StandardController(account);
PageReference saved = controller.SAVE();
PageReference viewed = controller.VIEW();
PageReference cancelled = controller.CANCEL();
System.assertEquals('/' + account.Id, saved.getUrl());
System.assertEquals('/' + account.Id, viewed.getUrl());
System.assertEquals('/' + account.Id, cancelled.getUrl());
return controller.delete();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "AccountController",
		Methods: map[string]Method{
			"remove": {
				Name:       "AccountController.remove",
				ClassName:  "AccountController",
				ReturnType: "PageReference",
				Program:    program,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	result, err := machine.InvokeVisualforceAction("AccountController", "remove", "/apex/Edit", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("success = false: %#v", result.Error)
	}
	if countTraceEvents(result.Trace, "apex.visualforce.standard_controller.action.start") != 4 {
		t.Fatalf("standard controller start traces = %#v", result.Trace)
	}
	if countTraceEvents(result.Trace, "apex.visualforce.standard_controller.action.complete") != 4 {
		t.Fatalf("standard controller complete traces = %#v", result.Trace)
	}
	deleteComplete := findTraceEventWithArg(result.Trace, "apex.visualforce.standard_controller.action.complete", "method", "delete")
	if deleteComplete == nil {
		t.Fatalf("delete completion trace missing: %#v", result.Trace)
	}
	if deleteComplete.Category != "apex.visualforce.standard_controller" || deleteComplete.Args["objectType"] != "Account" || deleteComplete.Args["dmlOperation"] != "delete" {
		t.Fatalf("delete completion args = %#v", deleteComplete.Args)
	}
	pageRef, ok := deleteComplete.Args["pageReference"].(map[string]any)
	if !ok || pageRef["url"] == "" {
		t.Fatalf("delete pageReference trace = %#v", deleteComplete.Args["pageReference"])
	}
	if findTraceEventWithArg(result.Trace, "apex.visualforce.standard_controller.action.complete", "method", "view") == nil {
		t.Fatalf("view completion trace missing: %#v", result.Trace)
	}
	if findTraceEventWithArg(result.Trace, "apex.visualforce.standard_controller.action.complete", "method", "cancel") == nil {
		t.Fatalf("cancel completion trace missing: %#v", result.Trace)
	}
}

func TestInvokeVisualforceActionOnControllerDispatchesStandardControllerMember(t *testing.T) {
	machine := New(nil)
	record := Object("Account")
	record.Fields["Id"] = String("001000000000001AAA")
	controller := Object("ApexPages.StandardController")
	controller.Fields["record"] = record

	value, updated, result, err := machine.InvokeVisualforceActionOnController(controller, "ApexPages.StandardController", "cancel", "/apex/Edit", nil)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Kind != ValueObject || updated.Type != "ApexPages.StandardController" {
		t.Fatalf("updated controller = %#v", updated)
	}
	if !result.Success || result.Error != nil {
		t.Fatalf("result = %#v", result)
	}
	if value.Kind != ValueObject || value.Type != "PageReference" || value.Fields["url"].Text != "/001000000000001AAA" {
		t.Fatalf("value = %#v", value)
	}
}

func TestInvokeVisualforceActionOnControllerDispatchesStandardSetControllerMember(t *testing.T) {
	machine := New(nil)
	controller := Object("ApexPages.StandardSetController")
	controller.Fields["records"] = List(Object("Account"), Object("Account"), Object("Account"))
	controller.Fields["selected"] = List()
	controller.Fields["pageSize"] = Int(1)
	controller.Fields["pageNumber"] = Int(1)

	_, updated, result, err := machine.InvokeVisualforceActionOnController(controller, "ApexPages.StandardSetController", "next", "/apex/List", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success || result.Error != nil {
		t.Fatalf("result = %#v", result)
	}
	if updated.Fields["pageNumber"].Kind != ValueInt || updated.Fields["pageNumber"].Int != 2 {
		t.Fatalf("updated pageNumber = %#v", updated.Fields["pageNumber"])
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

func TestInvokeVisualforceActionTracesStandardControllerActionError(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Unsaved');
ApexPages.StandardController controller = new ApexPages.StandardController(account);
return controller.delete();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "AccountController",
		Methods: map[string]Method{
			"removeUnsaved": {
				Name:       "AccountController.removeUnsaved",
				ClassName:  "AccountController",
				ReturnType: "PageReference",
				Program:    program,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	result, err := machine.InvokeVisualforceAction("AccountController", "removeUnsaved", "/apex/Edit", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Success || result.Error == nil {
		t.Fatalf("result = %#v", result)
	}
	actionError := findTraceEventWithArg(result.Trace, "apex.visualforce.standard_controller.action.error", "method", "delete")
	if actionError == nil {
		t.Fatalf("standard controller action error trace missing: %#v", result.Trace)
	}
	if actionError.Args["objectType"] != "Account" || actionError.Args["dmlOperation"] != "delete" {
		t.Fatalf("action error args = %#v", actionError.Args)
	}
	if actionError.Args["error"] == "" || actionError.Args["errorType"] == "" {
		t.Fatalf("action error args = %#v", actionError.Args)
	}
	if !traceHas(result.Trace, "apex.visualforce.action.error", "apex.visualforce") {
		t.Fatalf("visualforce action error trace missing: %#v", result.Trace)
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

func findTraceEventWithArg(events []trace.Event, name, key string, value any) *trace.Event {
	for i := range events {
		if events[i].Name == name && events[i].Args[key] == value {
			return &events[i]
		}
	}
	return nil
}

func countTraceEvents(events []trace.Event, name string) int {
	count := 0
	for _, event := range events {
		if event.Name == name {
			count++
		}
	}
	return count
}
