package apexast

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
)

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

func TestParseStructuredAnnotations(t *testing.T) {
	src := `@IsTest(SeeAllData = false)
public class Hello {
  @AuraEnabled(cacheable = true)
  public static void run() {}
}`
	file := NewParser().ParseSource("Hello.cls", src)
	if len(file.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
	}
	class := file.Declarations[0]
	if got := class.Annotations; len(got) != 1 || got[0].Name != "IsTest" || len(got[0].Arguments) != 1 || got[0].Arguments[0].Name != "SeeAllData" || got[0].Arguments[0].Value != "false" {
		t.Fatalf("class annotations = %#v", got)
	}
	method := class.Members[0]
	if got := method.Annotations; len(got) != 1 || got[0].Name != "AuraEnabled" || got[0].Arguments[0].Name != "cacheable" || got[0].Arguments[0].Value != "true" {
		t.Fatalf("method annotations = %#v", got)
	}
	for _, argument := range class.Annotations[0].Arguments {
		if argument.Range.Start.Offset < 0 || argument.Range.End.Offset <= argument.Range.Start.Offset || src[argument.Range.Start.Offset:argument.Range.End.Offset] == "" {
			t.Fatalf("argument range = %#v", argument.Range)
		}
	}
}

func TestParseMultipleAuraEnabledArguments(t *testing.T) {
	file := NewParser().ParseSource("Probe.cls", `public class Probe {
  @AuraEnabled(cacheable=true scope='global')
  public static String run() { return 'ok'; }
}`)
	if len(file.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
	}
	annotations := file.Declarations[0].Members[0].Annotations
	if len(annotations) != 1 || len(annotations[0].Arguments) != 2 {
		t.Fatalf("annotations = %#v", annotations)
	}
	if annotations[0].Arguments[1].Name != "scope" || annotations[0].Arguments[1].Value != "'global'" {
		t.Fatalf("scope argument = %#v", annotations[0].Arguments[1])
	}
}

func TestParseMultilineAnnotationArgumentWithEscapedApostrophe(t *testing.T) {
	src := `public class Probe {
  @InvocableVariable(
    Required=false
    Label='A label'
    Description='This isn\'t positional'
  )
  public String value;
}`
	file := NewParser().ParseSource("Probe.cls", src)
	if len(file.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
	}
	arguments := file.Declarations[0].Members[0].Annotations[0].Arguments
	if len(arguments) != 3 {
		t.Fatalf("arguments = %#v", arguments)
	}
	if got := arguments[2]; got.Name != "Description" || got.Value != "'This isn\\'t positional'" {
		t.Fatalf("description argument = %#v", got)
	}
}

func TestParseSalesforceInvocableVariableWithEscapedApostrophe(t *testing.T) {
	src := `public class Probe {
  @InvocableVariable(
    Required=false
    Label='Email From Org-Wide Id'
    Description='The Salesforce Id of the Organization-Wide email address to use as the "From" in emails. If this isn\'t set, the email address of the user sending the email is used instead.'
  )
  public String value;
}`
	file := NewParser().ParseSource("Probe.cls", src)
	if len(file.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
	}
	arguments := file.Declarations[0].Members[0].Annotations[0].Arguments
	if len(arguments) != 3 || arguments[1].Name != "Label" || arguments[2].Name != "Description" {
		t.Fatalf("arguments = %#v", arguments)
	}
	if got, want := arguments[2].Value, `'The Salesforce Id of the Organization-Wide email address to use as the "From" in emails. If this isn\'t set, the email address of the user sending the email is used instead.'`; got != want {
		t.Fatalf("description = %q, want %q", got, want)
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

func TestParseEnumMembersAndHasBody(t *testing.T) {
	file := NewParser().ParseSource("Color.cls", `public enum Color { Red, Green }`)
	if len(file.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
	}
	members := file.Declarations[0].Members
	if len(members) != 2 || members[0].Name != "Red" || members[1].Name != "Green" {
		t.Fatalf("enum members = %#v", members)
	}
	shape := NewParser().ParseSource("Shape.cls", `public abstract class Shape {
  public abstract void draw();
  public void paint() {}
}`)
	if shape.Declarations[0].Members[0].HasBody || !shape.Declarations[0].Members[1].HasBody {
		t.Fatalf("HasBody = %#v", shape.Declarations[0].Members)
	}
}

func TestParseEnumUserMethodSyntax(t *testing.T) {
	file := NewParser().ParseSource("Color.cls", `public enum Color { Red, Green; public void run() {} }`)
	if len(file.Diagnostics) == 0 {
		t.Fatalf("expected enum method syntax error, got %#v", file)
	}
}

func TestParseUserGenericClassDeclaration(t *testing.T) {
	file := NewParser().ParseSource("Box.cls", `public class Box<T> { public T value; }`)
	if len(file.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
	}
	if got := file.Declarations[0].TypeParameters; len(got) != 1 || got[0] != "T" {
		t.Fatalf("type parameters = %#v", got)
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

func TestParseMultilineStringLiteralPreservesMethodRange(t *testing.T) {
	source := "public class Probe {\n  public String run() { return '''\nhello\n'''; }\n}\n"
	file := NewParser().ParseSource("Probe.cls", source)
	if len(file.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
	}
	if len(file.Declarations) != 1 || len(file.Declarations[0].Members) != 1 {
		t.Fatalf("declarations = %#v", file.Declarations)
	}
	method := file.Declarations[0].Members[0]
	if method.Range.End.Offset <= method.Range.Start.Offset {
		t.Fatalf("method range = %#v", method.Range)
	}
	if !strings.Contains(source[method.Range.Start.Offset:method.Range.End.Offset], "'''") {
		t.Fatalf("method range does not contain multiline literal: %q", source[method.Range.Start.Offset:method.Range.End.Offset])
	}
}

func TestParseInheritanceAndBodyRanges(t *testing.T) {
	source := `public class Child extends Base implements One, Map<String, List<Integer>> {
  public String Name { get { return value; } set { value = value; } }
  public void run() { System.debug('}'); }
}
public interface ChildContract extends BaseContract, Generic<String> { void run(); }
trigger ChildTrigger on Account (before insert) { System.debug('}'); }`
	file := NewParser().ParseSource("Inheritance.cls", source)
	if len(file.Diagnostics) != 0 || len(file.Declarations) != 3 {
		t.Fatalf("parse = %#v", file)
	}
	bodyText := func(r *diagnostic.Range) string {
		if r == nil {
			return ""
		}
		return source[r.Start.Offset:r.End.Offset]
	}
	child := file.Declarations[0]
	if child.SuperClass != "Base" || len(child.Interfaces) != 2 || child.Interfaces[0] != "One" || child.Interfaces[1] != "Map<String,List<Integer>>" {
		t.Fatalf("class facts = %#v", child)
	}
	if bodyText(child.BodyRange) == "" || bodyText(child.Members[0].Accessors[0].BodyRange) != "{ return value; }" || bodyText(child.Members[1].BodyRange) != "{ System.debug('}'); }" {
		t.Fatalf("class body facts = %#v", child)
	}
	contract := file.Declarations[1]
	if len(contract.Interfaces) != 2 || contract.Interfaces[0] != "BaseContract" || contract.Interfaces[1] != "Generic<String>" {
		t.Fatalf("interface facts = %#v", contract)
	}
	if bodyText(file.Declarations[2].BodyRange) != "{ System.debug('}'); }" {
		t.Fatalf("trigger body = %q", bodyText(file.Declarations[2].BodyRange))
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
