package apexast

import "testing"

func TestParseClass(t *testing.T) {
	src := `
public with sharing class Hello {
  private Integer count;
  public Hello() {}
  public static void run() {
    System.debug('hello');
  }
  public String Name { get; set; }
}
`
	file := NewParser().ParseSource("Hello.cls", src)
	if len(file.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
	}
	if file.Kind != FileKindClass {
		t.Fatalf("kind = %q", file.Kind)
	}
	if len(file.Declarations) != 1 {
		t.Fatalf("declaration count = %d", len(file.Declarations))
	}
	class := file.Declarations[0]
	if class.Kind != DeclarationClass || class.Name != "Hello" {
		t.Fatalf("class declaration = %#v", class)
	}
	if len(class.Modifiers) != 2 || class.Modifiers[1] != "with sharing" {
		t.Fatalf("modifiers = %#v", class.Modifiers)
	}
	if len(class.Members) != 4 {
		t.Fatalf("member count = %d; members=%#v", len(class.Members), class.Members)
	}
	if class.Members[2].Kind != DeclarationMethod || class.Members[2].Name != "run" || class.Members[2].Type != "void" {
		t.Fatalf("method declaration = %#v", class.Members[2])
	}
}

func TestParseMethodAndConstructorParameters(t *testing.T) {
	src := `
public class Hello {
  public Hello(String name) {}
  public static List<Account> run(final Map<String, Account> accounts, Integer count) {
    return new List<Account>();
  }
}
`
	file := NewParser().ParseSource("Hello.cls", src)
	if len(file.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
	}
	members := file.Declarations[0].Members
	if got := members[0].Parameters; len(got) != 1 || got[0].Name != "name" || got[0].Type != "String" {
		t.Fatalf("constructor parameters = %#v", got)
	}
	params := members[1].Parameters
	if len(params) != 2 {
		t.Fatalf("method parameters = %#v", params)
	}
	if params[0].Name != "accounts" || params[0].Type != "Map<String,Account>" || len(params[0].Modifiers) != 1 || params[0].Modifiers[0] != "final" {
		t.Fatalf("first parameter = %#v", params[0])
	}
	if params[1].Name != "count" || params[1].Type != "Integer" {
		t.Fatalf("second parameter = %#v", params[1])
	}
}

func TestParseInitializerBlocks(t *testing.T) {
	src := `
public class Hello {
  static { Count = 1; }
  { Count = Count + 1; }
}
`
	file := NewParser().ParseSource("Hello.cls", src)
	if len(file.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
	}
	members := file.Declarations[0].Members
	if len(members) != 2 {
		t.Fatalf("member count = %d; members=%#v", len(members), members)
	}
	if members[0].Kind != DeclarationInitializer || !containsModifier(members[0].Modifiers, "static") {
		t.Fatalf("static initializer = %#v", members[0])
	}
	if members[1].Kind != DeclarationInitializer || containsModifier(members[1].Modifiers, "static") {
		t.Fatalf("instance initializer = %#v", members[1])
	}
}

func TestParsePropertyAccessors(t *testing.T) {
	src := `
public class Hello {
  private String backing;
  public String Name {
    get { return backing; }
    private set { backing = value; }
  }
}
`
	file := NewParser().ParseSource("Hello.cls", src)
	if len(file.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
	}
	prop := file.Declarations[0].Members[1]
	if prop.Kind != DeclarationProperty || len(prop.Accessors) != 2 {
		t.Fatalf("property = %#v", prop)
	}
	if prop.Accessors[0].Kind != "get" || !prop.Accessors[0].HasBody {
		t.Fatalf("getter = %#v", prop.Accessors[0])
	}
	if prop.Accessors[1].Kind != "set" || !prop.Accessors[1].HasBody || !containsModifier(prop.Accessors[1].Modifiers, "private") {
		t.Fatalf("setter = %#v", prop.Accessors[1])
	}
}

func TestParseTrigger(t *testing.T) {
	src := `trigger AccountTrigger on Account (before insert, after update) {
  AccountTriggerHandler.run();
}`
	file := NewParser().ParseSource("AccountTrigger.trigger", src)
	if len(file.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
	}
	if file.Kind != FileKindTrigger {
		t.Fatalf("kind = %q", file.Kind)
	}
	trigger := file.Declarations[0]
	if trigger.Name != "AccountTrigger" || trigger.ObjectName != "Account" {
		t.Fatalf("trigger declaration = %#v", trigger)
	}
	if len(trigger.Events) != 2 || trigger.Events[0] != "beforeinsert" || trigger.Events[1] != "afterupdate" {
		t.Fatalf("events = %#v", trigger.Events)
	}
}

func TestParseInterface(t *testing.T) {
	src := `global interface ITemplateData {
  Object getData();
  void setRequest(DataRequest request);
}`
	file := NewParser().ParseSource("ITemplateData.cls", src)
	if len(file.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
	}
	if file.Kind != FileKindInterface {
		t.Fatalf("kind = %q", file.Kind)
	}
	if file.Declarations[0].Kind != DeclarationInterface || len(file.Declarations[0].Members) != 2 {
		t.Fatalf("interface declaration = %#v", file.Declarations[0])
	}
}

func TestParseVoidIdentifier(t *testing.T) {
	src := `
public class PaymentGatewayService {
  public PaymentGatewayResponse void(PaymentGatewayRequest request) {
    return getGateway().void(request);
  }
  public List<PaymentGatewayResponse> void(
    List<PaymentGatewayRequest> requests
  ) {
    return void(requests);
  }
  public static void run() {
    System.debug('void(');
  }
}
`
	file := NewParser().ParseSource("PaymentGatewayService.cls", src)
	if len(file.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
	}
	methods := file.Declarations[0].Members
	var voidMethods int
	var runType string
	for _, method := range methods {
		if method.Name == "void" {
			voidMethods++
		}
		if method.Name == "run" {
			runType = method.Type
		}
	}
	if voidMethods != 2 {
		t.Fatalf("void method count = %d; members=%#v", voidMethods, methods)
	}
	if runType != "void" {
		t.Fatalf("run return type = %q", runType)
	}
}

func TestParseSyntaxError(t *testing.T) {
	file := NewParser().ParseSource("Broken.cls", "public class Broken {")
	if len(file.Diagnostics) == 0 {
		t.Fatal("expected syntax diagnostic")
	}
	got := file.Diagnostics[0].Range
	if got.Start.Line == 0 || got.Start.Column == 0 || got.Start.Offset == 0 {
		t.Fatalf("diagnostic missing range: %#v", file.Diagnostics[0])
	}
	if got.End.Offset < got.Start.Offset {
		t.Fatalf("diagnostic range went backwards: %#v", got)
	}
}

func TestWalkFile(t *testing.T) {
	file := NewParser().ParseSource("Hello.cls", "public class Hello { Integer count; void run() {} }")
	var names []string
	WalkFile(file, VisitorFunc(func(decl Declaration) bool {
		names = append(names, decl.Name)
		return true
	}))
	if len(names) != 3 || names[0] != "Hello" || names[2] != "run" {
		t.Fatalf("walked names = %#v", names)
	}
}

func TestLineMapAndFileURI(t *testing.T) {
	lineMap := NewLineMap("one\ntwo\n")
	pos := lineMap.Position(5)
	if pos.Line != 2 || pos.Column != 2 {
		t.Fatalf("position = %#v", pos)
	}
	if uri := FileURI("Hello.cls"); uri[:7] != "file://" {
		t.Fatalf("uri = %q", uri)
	}
}
