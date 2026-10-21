package vm

import (
	"strings"
	"testing"
)

// These are internal candidate assertions pending native observations. They
// exercise only dispatch from an abstract interface to the actual receiver.
func abstractListInterfaceMachine(t *testing.T, api string) *VM {
	t.Helper()
	machine := New(nil)
	for _, class := range []Class{
		{Name: "RecordAction", IsInterface: true},
		{Name: "ActionContext", Fields: map[string]Field{"marker": {Name: "marker", Type: "String"}}},
		{Name: "AccountAction", IsTest: true, Interfaces: []string{"RecordAction"}},
		{Name: "ActionInvoker"},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if err := machine.RegisterMethod(Method{
		Name: "RecordAction.act", ClassName: "RecordAction", ReturnType: "void",
		Modifiers: []string{"abstract"}, Params: []Param{{Name: "records", Type: "List<SObject>"}, {Name: "context", Type: "ActionContext"}},
	}); err != nil {
		t.Fatal(err)
	}
	register := func(owner, name, firstType, body string, static bool) {
		program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterMethod(Method{
			Name: owner + "." + name, ClassName: owner, ReturnType: "void", IsStatic: static,
			Params: []Param{{Name: "records", Type: firstType}, {Name: "context", Type: "ActionContext"}}, Program: program,
		}); err != nil {
			t.Fatal(err)
		}
	}
	register("AccountAction", "act", "List<Account>", "context.marker = records[0].Name;", false)
	program, err := CompileAnonymousWithOptions(`action.act(records, context);`, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name: "ActionInvoker.invoke", ClassName: "ActionInvoker", ReturnType: "void", IsStatic: true,
		Params: []Param{{Name: "action", Type: "RecordAction"}, {Name: "records", Type: "List<SObject>"}, {Name: "context", Type: "ActionContext"}}, Program: program,
	}); err != nil {
		t.Fatal(err)
	}
	return machine
}

func TestAbstractListInterfaceConcreteReceiverCandidate(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, testContext := range []bool{false, true} {
			for name, source := range map[string]string{
				"direct":     `RecordAction action = new AccountAction(); List<Account> records = new List<Account>{new Account(Name='marker')}; ActionContext context = new ActionContext(); action.act(records, context); System.assertEquals('marker', context.marker);`,
				"listHelper": `List<RecordAction> actions = new List<RecordAction>{new AccountAction()}; List<Account> records = new List<Account>{new Account(Name='marker')}; ActionContext context = new ActionContext(); ActionInvoker.invoke(actions[0], records, context); System.assertEquals('marker', context.marker);`,
			} {
				t.Run(api+"/"+map[bool]string{false: "anonymous", true: "istest"}[testContext]+"/"+name, func(t *testing.T) {
					machine := abstractListInterfaceMachine(t, api)
					if testContext {
						machine.EnableTestContext()
					}
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestAbstractListInterfaceCandidateScope(t *testing.T) {
	for _, tc := range []struct {
		name, listType, rowType   string
		null, classInstance, want bool
	}{
		{name: "typedAccount", listType: "List<Account>", rowType: "Account", want: true},
		{name: "accountSObjectView", listType: "List<SObject>", rowType: "Account", want: true},
		{name: "emptyTypedAccount", listType: "List<Account>", want: true},
		{name: "emptySObject", listType: "List<SObject>"},
		{name: "wrongContact", listType: "List<Contact>", rowType: "Contact"},
		{name: "wrongContactSObjectView", listType: "List<SObject>", rowType: "Contact"},
		{name: "objectView", listType: "List<Object>", rowType: "Account"},
		{name: "stringList", listType: "List<String>", rowType: "String"},
		{name: "null", null: true},
		{name: "classInstance", listType: "List<Account>", rowType: "Account", classInstance: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := abstractListInterfaceMachine(t, "67.0")
			value := typedList(tc.listType)
			if tc.null {
				value = Null
			} else if tc.rowType != "" {
				if tc.rowType == "String" {
					value.List = []Value{String("marker")}
				} else {
					value.List = []Value{Object(tc.rowType)}
					value.List[0].classInstance = tc.classInstance
				}
			}
			receiver := Object("AccountAction")
			receiver.Static = "RecordAction"
			method, found, ambiguous := machine.resolveValueInstanceMethodForArgs("", receiver, "act", []Value{value, Object("ActionContext")})
			concrete := found && !ambiguous && strings.EqualFold(method.ClassName, "AccountAction")
			if concrete != tc.want {
				t.Fatalf("concrete=%v, want %v: method=%s found=%v ambiguous=%v", concrete, tc.want, method.Name, found, ambiguous)
			}
		})
	}
}

func TestAbstractListInterfaceCandidateExactPrecedesNarrow(t *testing.T) {
	machine := abstractListInterfaceMachine(t, "67.0")
	program, err := CompileAnonymous(`context.marker = 'exact';`)
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "AccountAction.act", ClassName: "AccountAction", ReturnType: "void", Params: []Param{{Name: "records", Type: "List<SObject>"}, {Name: "context", Type: "ActionContext"}}, Program: program}); err != nil {
		t.Fatal(err)
	}
	receiver := Object("AccountAction")
	receiver.Static = "RecordAction"
	value := typedList("List<Account>")
	value.List = []Value{Object("Account")}
	method, found, ambiguous := machine.resolveValueInstanceMethodForArgs("", receiver, "act", []Value{value, Object("ActionContext")})
	if !found || ambiguous || method.Params[0].Type != "List<SObject>" {
		t.Fatalf("exact implementation lost: %+v found=%v ambiguous=%v", method, found, ambiguous)
	}
}

func TestAbstractListInterfaceCandidateRejectsOtherSignatures(t *testing.T) {
	machine := abstractListInterfaceMachine(t, "67.0")
	surface, _ := machine.registeredMethod("RecordAction.act")
	value := typedList("List<Account>")
	value.List = []Value{Object("Account")}
	args := []Value{value, Object("ActionContext")}
	receiver := Object("AccountAction")
	for name, mutate := range map[string]func(*Method){
		"static":          func(candidate *Method) { candidate.IsStatic = true },
		"abstract":        func(candidate *Method) { candidate.Modifiers = []string{"abstract"} },
		"contextMismatch": func(candidate *Method) { candidate.Params[1].Type = "String" },
		"nonList":         func(candidate *Method) { candidate.Params[0].Type = "Account" },
		"returnMismatch":  func(candidate *Method) { candidate.ReturnType = "String" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate, _ := machine.registeredMethod("AccountAction.act")
			candidate.Params = append([]Param(nil), candidate.Params...)
			mutate(&candidate)
			if machine.abstractRecordListParamsMatch(candidate, surface, args) {
				t.Fatal("unrelated signature accepted")
			}
		})
	}
	candidate, _ := machine.registeredMethod("AccountAction.act")
	returnedSurface := surface
	returnedSurface.ReturnType = "String"
	candidate.ReturnType = "String"
	if machine.abstractRecordListParamsMatch(candidate, returnedSurface, args) {
		t.Fatal("uncaptured returned-value method entered fallback")
	}
	surface.Modifiers = nil
	if _, found, ambiguous := machine.abstractRecordListImplementation(receiver, surface, "act", args); found || ambiguous {
		t.Fatal("non-abstract interface surface entered fallback")
	}
	receiver = Object("ActionContext")
	surface.Modifiers = []string{"abstract"}
	if _, found, ambiguous := machine.abstractRecordListImplementation(receiver, surface, "act", args); found || ambiguous {
		t.Fatal("unrelated receiver entered fallback")
	}
}

func TestAbstractListInterfaceCandidateRejectsAmbiguity(t *testing.T) {
	machine := abstractListInterfaceMachine(t, "67.0")
	surface := Method{Name: "RecordAction.choose", ClassName: "RecordAction", ReturnType: "void", Modifiers: []string{"abstract"}, Params: []Param{{Type: "List<SObject>"}, {Type: "List<SObject>"}}}
	for _, params := range [][]Param{
		{{Type: "List<Account>"}, {Type: "List<SObject>"}},
		{{Type: "List<SObject>"}, {Type: "List<Account>"}},
	} {
		if err := machine.RegisterMethod(Method{Name: "AccountAction.choose", ClassName: "AccountAction", ReturnType: "void", Params: params}); err != nil {
			t.Fatal(err)
		}
	}
	value := typedList("List<Account>")
	value.List = []Value{Object("Account")}
	if _, found, ambiguous := machine.abstractRecordListImplementation(Object("AccountAction"), surface, "choose", []Value{value, value}); found || !ambiguous {
		t.Fatalf("want ambiguous implementation: found=%v ambiguous=%v", found, ambiguous)
	}
}
