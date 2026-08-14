//go:build cgo

package apexast

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestParseDeclarations(t *testing.T) {
	tests := []struct {
		name   string
		source string
		check  func(t *testing.T, file File)
	}{
		{
			name: "class",
			source: `public with sharing class Hello {
  private Integer count;
  public Hello() {}
  public static void run() {
    System.debug('hello');
  }
  public String Name { get; set; }
}`,
			check: func(t *testing.T, file File) {
				if file.Kind != FileKindClass {
					t.Fatalf("kind = %q", file.Kind)
				}
				class := file.Declarations[0]
				if class.Kind != DeclarationClass || class.Name != "Hello" {
					t.Fatalf("class = %#v", class)
				}
				if len(class.Modifiers) != 2 || class.Modifiers[1] != "with sharing" {
					t.Fatalf("modifiers = %#v", class.Modifiers)
				}
				if len(class.Members) != 4 {
					t.Fatalf("members = %#v", class.Members)
				}
				if class.Members[2].Kind != DeclarationMethod || class.Members[2].Name != "run" || class.Members[2].Type != "void" {
					t.Fatalf("method = %#v", class.Members[2])
				}
				if class.Members[3].Kind != DeclarationProperty || len(class.Members[3].Accessors) != 2 {
					t.Fatalf("property = %#v", class.Members[3])
				}
			},
		},
		{
			name: "parameters",
			source: `public class Hello {
  public static List<Account> run(final Map<String, Account> accounts, Integer count) {
    return new List<Account>();
  }
}`,
			check: func(t *testing.T, file File) {
				params := file.Declarations[0].Members[0].Parameters
				if len(params) != 2 {
					t.Fatalf("params = %#v", params)
				}
				if params[0].Name != "accounts" || params[0].Type != "Map<String,Account>" || len(params[0].Modifiers) != 1 || params[0].Modifiers[0] != "final" {
					t.Fatalf("first param = %#v", params[0])
				}
				if params[1].Name != "count" || params[1].Type != "Integer" {
					t.Fatalf("second param = %#v", params[1])
				}
			},
		},
		{
			name: "initializers",
			source: `public class Hello {
  static { Count = 1; }
  { Count = Count + 1; }
}`,
			check: func(t *testing.T, file File) {
				members := file.Declarations[0].Members
				if len(members) != 2 {
					t.Fatalf("members = %#v", members)
				}
				if members[0].Kind != DeclarationInitializer || !containsModifier(members[0].Modifiers, "static") {
					t.Fatalf("static initializer = %#v", members[0])
				}
				if members[1].Kind != DeclarationInitializer || containsModifier(members[1].Modifiers, "static") {
					t.Fatalf("instance initializer = %#v", members[1])
				}
			},
		},
		{
			name: "property bodies",
			source: `public class Hello {
  private String backing;
  public String Name {
    get { return backing; }
    private set { backing = value; }
  }
}`,
			check: func(t *testing.T, file File) {
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
			},
		},
		{
			name: "trigger",
			source: `trigger AccountTrigger on Account (before insert, after update) {
  AccountTriggerHandler.run();
}`,
			check: func(t *testing.T, file File) {
				if file.Kind != FileKindTrigger {
					t.Fatalf("kind = %q", file.Kind)
				}
				trigger := file.Declarations[0]
				if trigger.Name != "AccountTrigger" || trigger.ObjectName != "Account" {
					t.Fatalf("trigger = %#v", trigger)
				}
				if len(trigger.Events) != 2 || trigger.Events[0] != "beforeinsert" || trigger.Events[1] != "afterupdate" {
					t.Fatalf("events = %#v", trigger.Events)
				}
			},
		},
		{
			name:   "interface",
			source: `global interface ITemplateData { Object getData(); void setRequest(DataRequest request); }`,
			check: func(t *testing.T, file File) {
				if file.Kind != FileKindInterface || len(file.Declarations[0].Members) != 2 {
					t.Fatalf("interface = %#v", file)
				}
			},
		},
		{
			name:   "enum",
			source: `public enum Color { Red, Green }`,
			check: func(t *testing.T, file File) {
				if file.Kind != FileKindEnum || file.Declarations[0].Name != "Color" {
					t.Fatalf("enum = %#v", file)
				}
				members := file.Declarations[0].Members
				if len(members) != 2 || members[0].Name != "Red" || members[1].Name != "Green" {
					t.Fatalf("enum members = %#v", members)
				}
				if members[0].Kind != DeclarationField || members[0].Type != "Color" {
					t.Fatalf("enum constant = %#v", members[0])
				}
			},
		},
		{
			name:   "user generic class declaration",
			source: `public class Box<T> { public T value; }`,
			check: func(t *testing.T, file File) {
				if got := file.Declarations[0].TypeParameters; len(got) != 1 || got[0] != "T" {
					t.Fatalf("type parameters = %#v", got)
				}
			},
		},
		{
			name: "void identifier",
			source: `public class PaymentGatewayService {
  public PaymentGatewayResponse void(PaymentGatewayRequest request) {
    return getGateway().void(request);
  }
  public List<PaymentGatewayResponse> void(List<PaymentGatewayRequest> requests) {
    return void(requests);
  }
  public static void run() {
    System.debug('void(');
  }
}`,
			check: func(t *testing.T, file File) {
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
				if voidMethods != 2 || runType != "void" {
					t.Fatalf("methods = %#v", methods)
				}
			},
		},
		{
			name:   "nested type",
			source: `public class Container { public class Nested { public void run() {} } }`,
			check: func(t *testing.T, file File) {
				outer := file.Declarations[0]
				if len(outer.Members) != 1 || outer.Members[0].Kind != DeclarationClass || outer.Members[0].Name != "Nested" {
					t.Fatalf("outer = %#v", outer)
				}
			},
		},
		{
			name:   "field variables",
			source: `public class Fields { Integer a, b; }`,
			check: func(t *testing.T, file File) {
				members := file.Declarations[0].Members
				if len(members) != 2 || members[0].Name != "a" || members[1].Name != "b" {
					t.Fatalf("members = %#v", members)
				}
			},
		},
		{
			name: "constructor call with cast argument",
			source: `public class Buttons {
  public class Request {
    public Request(Button2__mdt recordArg) {}
    public Request(Button2__mdt recordArg, CardData dataInstance) {}
    public Request(SObject recordArg, CardData dataInstance) {
      if (recordArg.getSObjectType() == Button2__mdt.SObjectType) {
        this((Button2__mdt)recordArg, dataInstance);
      }
      super((Button2__mdt)recordArg);
    }
  }
}`,
			check: func(t *testing.T, file File) {
				inner := file.Declarations[0].Members[0]
				if inner.Kind != DeclarationClass || inner.Name != "Request" || len(inner.Members) != 3 {
					t.Fatalf("inner class = %#v", inner)
				}
				if inner.Members[2].Kind != DeclarationConstructor || len(inner.Members[2].Parameters) != 2 {
					t.Fatalf("constructor = %#v", inner.Members[2])
				}
			},
		},
		{
			name: "annotation trailing comment is not modifier",
			source: `public class Tests {
  @isTest // covers all forms
  static void run() {}
}`,
			check: func(t *testing.T, file File) {
				method := file.Declarations[0].Members[0]
				if len(method.Modifiers) != 2 || !containsModifier(method.Modifiers, "@isTest") || !containsModifier(method.Modifiers, "static") {
					t.Fatalf("modifiers = %#v", method.Modifiers)
				}
			},
		},
		{
			name: "annotation whitespace canonicalization",
			source: `public class Requests {
  @InvocableVariable( Required=true Label='A Label' )
  public String label;
}`,
			check: func(t *testing.T, file File) {
				field := file.Declarations[0].Members[0]
				if len(field.Modifiers) != 2 || field.Modifiers[0] != "@InvocableVariable(Required=trueLabel='A Label')" || field.Modifiers[1] != "public" {
					t.Fatalf("modifiers = %#v", field.Modifiers)
				}
			},
		},
		{
			name: "trigger context chained call",
			source: `public class Handler {
  public void run() {
    Trigger.oldMap.get(item.Education__c).addError(constants.ERR_CANNOT_DELETE_RECORD + ': ' + item.Id);
  }
}`,
			check: func(t *testing.T, file File) {
				class := file.Declarations[0]
				if class.Kind != DeclarationClass || class.Name != "Handler" || len(class.Members) != 1 {
					t.Fatalf("class = %#v", class)
				}
			},
		},
		{
			name:   "sentinel-like identifiers keep original text",
			source: `public class Tr1gger { public PaymentGatewayResponse v0id(PaymentGatewayRequest request) { return null; } }`,
			check: func(t *testing.T, file File) {
				class := file.Declarations[0]
				if class.Name != "Tr1gger" || len(class.Members) != 1 || class.Members[0].Name != "v0id" {
					t.Fatalf("declarations = %#v", file.Declarations)
				}
			},
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := parser.ParseSource(tt.name+".cls", tt.source)
			if len(file.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
			}
			if len(file.Declarations) != 1 {
				t.Fatalf("declarations = %#v", file.Declarations)
			}
			tt.check(t, file)
		})
	}
}

func TestParseMethodConstructorHasBody(t *testing.T) {
	file := NewParser().ParseSource("Shape.cls", `public abstract class Shape {
  public abstract void draw();
  public void paint() { System.debug('paint'); }
  public Shape() {}
}
public interface Drawable { void draw(); }`)
	if len(file.Declarations) != 2 {
		t.Fatalf("declarations = %#v", file.Declarations)
	}
	shape := file.Declarations[0]
	if shape.Members[0].HasBody {
		t.Fatalf("abstract method should not have body: %#v", shape.Members[0])
	}
	if !shape.Members[1].HasBody {
		t.Fatalf("concrete method should have body: %#v", shape.Members[1])
	}
	if !shape.Members[2].HasBody {
		t.Fatalf("constructor should have body: %#v", shape.Members[2])
	}
	iface := file.Declarations[1]
	if iface.Members[0].HasBody {
		t.Fatalf("interface method should not have body: %#v", iface.Members[0])
	}
}

func TestParseInheritanceAndBodyRanges(t *testing.T) {
	source := `public class Child extends /* { superclass comment } */ Base implements One, /* { interface comment } */ Map<String, List<Integer>> {
  static { /* { initializer comment } */ Count = 1; }
  { /* } initializer comment { */ Count++; }
  public String Name {
    get { /* { getter comment } */ return backing; }
    set { /* } setter comment { */ backing = value; }
  }
  public void run() { /* { method comment } */ System.debug('}'); }
}
public interface ChildContract extends BaseContract, Generic<String> {
  void run();
}
trigger ChildTrigger on Account (before insert) {
  System.debug('}');
}`
	file := NewParser().ParseSource("Inheritance.cls", source)
	if len(file.Diagnostics) != 0 || len(file.Declarations) != 3 {
		t.Fatalf("parse = %#v", file)
	}
	bodyText := func(r *Range) string {
		if r == nil {
			return ""
		}
		return source[r.Start.Offset:r.End.Offset]
	}
	child := file.Declarations[0]
	if child.SuperClass != "Base" {
		t.Fatalf("superclass = %q", child.SuperClass)
	}
	if got, want := child.Interfaces, []string{"One", "Map<String, List<Integer>>"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("interfaces = %#v, want %#v", got, want)
	}
	if got := strings.TrimSpace(bodyText(child.BodyRange)); !strings.HasPrefix(got, "{") || !strings.HasSuffix(got, "}") {
		t.Fatalf("class body = %q", got)
	}
	if got := strings.TrimSpace(bodyText(child.Members[0].BodyRange)); got != "{ /* { initializer comment } */ Count = 1; }" {
		t.Fatalf("static initializer body = %q", got)
	}
	if got := strings.TrimSpace(bodyText(child.Members[1].BodyRange)); got != "{ /* } initializer comment { */ Count++; }" {
		t.Fatalf("instance initializer body = %q", got)
	}
	property := child.Members[2]
	if len(property.Accessors) != 2 || strings.TrimSpace(bodyText(property.Accessors[0].BodyRange)) != "{ /* { getter comment } */ return backing; }" || strings.TrimSpace(bodyText(property.Accessors[1].BodyRange)) != "{ /* } setter comment { */ backing = value; }" {
		t.Fatalf("accessor bodies = %#v", property.Accessors)
	}
	if got := strings.TrimSpace(bodyText(child.Members[3].BodyRange)); got != "{ /* { method comment } */ System.debug('}'); }" {
		t.Fatalf("method body = %q", got)
	}
	contract := file.Declarations[1]
	if got, want := contract.Interfaces, []string{"BaseContract", "Generic<String>"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("interface bases = %#v, want %#v", got, want)
	}
	trigger := file.Declarations[2]
	if got := strings.TrimSpace(bodyText(trigger.BodyRange)); got != "{\n  System.debug('}');\n}" {
		t.Fatalf("trigger body = %q", got)
	}
}

func TestParseInheritanceSkipsCommentsAndPreservesTypeNodes(t *testing.T) {
	for _, test := range []struct {
		source     string
		superClass string
		interfaces []string
	}{
		{source: "public class Child EXTENDS Base implements One, Two {}", superClass: "Base", interfaces: []string{"One", "Two"}},
		{source: "public interface Contract eXtEnDs BaseContract, Generic<String> {}", interfaces: []string{"BaseContract", "Generic<String>"}},
	} {
		file := NewParser().ParseSource("Inheritance.cls", test.source)
		if len(file.Diagnostics) != 0 || len(file.Declarations) != 1 {
			t.Fatalf("parse %q = %#v", test.source, file)
		}
		decl := file.Declarations[0]
		if decl.SuperClass != test.superClass || !reflect.DeepEqual(decl.Interfaces, test.interfaces) {
			t.Fatalf("facts for %q = superclass %q interfaces %#v", test.source, decl.SuperClass, decl.Interfaces)
		}
	}
}

func TestParseEnumUserMethodSyntax(t *testing.T) {
	file := NewParser().ParseSource("Color.cls", `public enum Color { Red, Green; public void run() {} }`)
	if !file.HasErrors() {
		t.Fatalf("expected enum method syntax error, got %#v", file)
	}
}

func TestParseSyntaxError(t *testing.T) {
	file := NewParser().ParseSource("Broken.cls", "public class Broken {")
	if len(file.Diagnostics) == 0 {
		t.Fatal("expected syntax diagnostic")
	}
	if file.Diagnostics[0].Range == nil || file.Diagnostics[0].Range.Start.Line == 0 {
		t.Fatalf("diagnostic = %#v", file.Diagnostics[0])
	}
}

func TestParseRejectsEverySalesforceReservedVariableName(t *testing.T) {
	reserved := strings.Fields(`abstract activate and any array as asc autonomous
		begin bigdecimal blob boolean break bulk by byte case cast catch char class
		collect commit const continue currency date datetime decimal default delete
		desc do double else end enum exception exit export extends false final finally
		float for from global goto group having hint if implements import in inner
		insert instanceof int integer interface into join like limit list long loop
		map merge new not null nulls number object of on or outer override package
		parallel pragma private protected public retrieve return rollback select set
		short sObject sort static string super switch synchronized system testmethod
		then this throw time transaction trigger true try undelete update upsert using
		virtual void webservice when where while`)
	if len(reserved) != 121 || len(salesforceReservedIdentifiers) != 121 {
		t.Fatalf("reserved word counts = test:%d implementation:%d, want 121", len(reserved), len(salesforceReservedIdentifiers))
	}
	parser := NewParser()
	for _, word := range reserved {
		t.Run(word, func(t *testing.T) {
			source := fmt.Sprintf("class Probe { void run() { String %s = 'x'; } }", word)
			if file := parser.ParseSource("Probe.cls", source); !file.HasErrors() {
				t.Fatalf("%q was accepted as a variable name", word)
			}
		})
	}
}

func TestParseRejectsReservedIdentifiersCaseInsensitively(t *testing.T) {
	source := "class Probe { void run() { String Currency = 'USD'; } }"
	file := NewParser().ParseSource("Probe.cls", source)
	for _, diag := range file.Diagnostics {
		if diag.Code == "APEXPARSE002" && diag.Message == "Identifier name is reserved: Currency" {
			return
		}
	}
	t.Fatalf("missing mixed-case reserved identifier diagnostic: %#v", file.Diagnostics)
}

func TestParseRejectsReservedIdentifiersInDeclarationContexts(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "field",
			source: "class Probe { String currency; }",
		},
		{
			name:   "local",
			source: "class Probe { void run() { String currency = 'USD'; } }",
		},
		{
			name:   "parameter",
			source: "class Probe { void run(String currency) {} }",
		},
		{
			name:   "enhanced for",
			source: "class Probe { void run() { for (String currency : new List<String>()) {} } }",
		},
		{
			name:   "catch",
			source: "class Probe { void run() { try {} catch (Exception currency) {} } }",
		},
		{
			name:   "enum constant",
			source: "enum Probe { currency }",
		},
	}
	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := parser.ParseSource("Probe.cls", tt.source)
			var reserved *Diagnostic
			for i := range file.Diagnostics {
				if file.Diagnostics[i].Code == "APEXPARSE002" {
					reserved = &file.Diagnostics[i]
					break
				}
			}
			if reserved == nil {
				t.Fatalf("missing reserved identifier diagnostic: %#v", file.Diagnostics)
			}
			if reserved.Message != "Identifier name is reserved: currency" {
				t.Fatalf("message = %q", reserved.Message)
			}
			if reserved.Range == nil {
				t.Fatalf("diagnostic range = %#v", reserved)
			}
			start, end := reserved.Range.Start.Offset, reserved.Range.End.Offset
			if start < 0 || end > len(tt.source) || start >= end || tt.source[start:end] != "currency" {
				t.Fatalf("range = %#v, source slice %q", reserved.Range, tt.source[start:end])
			}
		})
	}
}

func TestParsePreservesSalesforceContextualIdentifiersAndMethodNames(t *testing.T) {
	permitted := []string{
		"after", "before", "count", "excludes", "first", "includes",
		"last", "order", "sharing", "with", "id",
	}
	parser := NewParser()
	for _, word := range permitted {
		t.Run("variable "+word, func(t *testing.T) {
			source := fmt.Sprintf("class Probe { void run() { String %s = 'x'; } }", word)
			if file := parser.ParseSource("Probe.cls", source); file.HasErrors() {
				t.Fatalf("%q should remain a valid variable name: %#v", word, file.Diagnostics)
			}
		})
	}
	for _, word := range []string{"currency", "void"} {
		t.Run("method "+word, func(t *testing.T) {
			source := fmt.Sprintf("class Probe { String %s() { return 'x'; } }", word)
			if file := parser.ParseSource("Probe.cls", source); file.HasErrors() {
				t.Fatalf("%q should remain a valid method name: %#v", word, file.Diagnostics)
			}
		})
	}
	if file := parser.ParseSource("Probe.cls", "class Probe { String trigger() { return 'x'; } }"); !file.HasErrors() {
		t.Fatal(`"trigger" should remain reserved as a method name`)
	}
}

func TestParseRejectsInvalidIdentifierShapesAndLengths(t *testing.T) {
	longOK := "A" + strings.Repeat("a", 254)
	longBad := "A" + strings.Repeat("a", 255)
	if len(longOK) != 255 || len(longBad) != 256 {
		t.Fatalf("fixture lengths = %d/%d, want 255/256", len(longOK), len(longBad))
	}
	tests := []struct {
		name   string
		ident  string
		source string
	}{
		{name: "leading underscore", ident: "_value", source: "class Probe { void run() { String _value = 'x'; } }"},
		{name: "trailing underscore", ident: "value_", source: "class Probe { void run() { String value_ = 'x'; } }"},
		{name: "consecutive underscores", ident: "value__name", source: "class Probe { void run() { String value__name = 'x'; } }"},
		{name: "256 characters", ident: longBad, source: fmt.Sprintf("class Probe { void run() { String %s = 'x'; } }", longBad)},
	}
	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := parser.ParseSource("Probe.cls", tt.source)
			var invalid *Diagnostic
			for i := range file.Diagnostics {
				if file.Diagnostics[i].Code == "APEXPARSE003" {
					invalid = &file.Diagnostics[i]
					break
				}
			}
			if invalid == nil {
				t.Fatalf("missing APEXPARSE003 for %q: %#v", tt.ident, file.Diagnostics)
			}
			wantMsg := "Invalid identifier: " + tt.ident
			if invalid.Message != wantMsg {
				t.Fatalf("message = %q, want %q", invalid.Message, wantMsg)
			}
			if invalid.Range == nil {
				t.Fatalf("diagnostic range = %#v", invalid)
			}
			start, end := invalid.Range.Start.Offset, invalid.Range.End.Offset
			if start < 0 || end > len(tt.source) || start >= end || tt.source[start:end] != tt.ident {
				t.Fatalf("range = %#v, source slice %q", invalid.Range, tt.source[start:end])
			}
		})
	}
	if file := parser.ParseSource("Probe.cls", fmt.Sprintf("class Probe { void run() { String %s = 'x'; } }", longOK)); file.HasErrors() {
		t.Fatalf("255-character identifier should remain valid: %#v", file.Diagnostics)
	}
}

func TestParseRejectsInvalidIdentifiersInDeclarationContexts(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "field", source: "class Probe { String value__name; }"},
		{name: "local", source: "class Probe { void run() { String value__name = 'x'; } }"},
		{name: "parameter", source: "class Probe { void run(String value__name) {} }"},
		{name: "enhanced for", source: "class Probe { void run() { for (String value__name : new List<String>()) {} } }"},
		{name: "catch", source: "class Probe { void run() { try {} catch (Exception value__name) {} } }"},
		{name: "method", source: "class Probe { void value__name() {} }"},
		{name: "class", source: "class value__name {}"},
		{name: "enum constant", source: "enum Probe { value__name }"},
	}
	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := parser.ParseSource("Probe.cls", tt.source)
			found := false
			for _, diag := range file.Diagnostics {
				if diag.Code == "APEXPARSE003" && diag.Message == "Invalid identifier: value__name" {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("missing APEXPARSE003 in %s context: %#v", tt.name, file.Diagnostics)
			}
		})
	}
}

func TestParseDoesNotApplySourceIdentifierRulesToSchemaTypeReferences(t *testing.T) {
	source := "class Probe { void run() { Account__c row; Custom_Field__c value; } }"
	file := NewParser().ParseSource("Probe.cls", source)
	for _, diag := range file.Diagnostics {
		if diag.Code == "APEXPARSE003" {
			t.Fatalf("schema/API type references must not use source-identifier rules: %#v", file.Diagnostics)
		}
	}
}

func TestParseRanges(t *testing.T) {
	source := "public class Hello {\n  public static void run() {}\n}\n"
	file := NewParser().ParseSource("Hello.cls", source)
	if len(file.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
	}
	class := file.Declarations[0]
	if class.Range.Start.Line != 1 || class.Range.Start.Column != 1 || class.Range.Start.Offset != 0 {
		t.Fatalf("class start = %#v", class.Range.Start)
	}
	if class.Range.End.Line != 3 || class.Range.End.Column != 2 {
		t.Fatalf("class end = %#v", class.Range.End)
	}
	method := class.Members[0]
	if method.Range.Start.Line != 2 || method.Range.Start.Column != 3 {
		t.Fatalf("method start = %#v", method.Range.Start)
	}
	if method.Range.End.Line != 2 || method.Range.End.Column != 30 {
		t.Fatalf("method end = %#v", method.Range.End)
	}
}

func TestParseMultilineStringLiteralPreservesMethodRange(t *testing.T) {
	source := "public class Probe {\n  public String run() { return '''\nhello\n'''; }\n}\n"
	file := NewParser().ParseSource("Probe.cls", source)
	if file.HasErrors() {
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

func TestParserConcurrentUse(t *testing.T) {
	parser := NewParser()
	source := benchmarkApexClass(5)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 25; j++ {
				file := parser.ParseSource("Concurrent.cls", source)
				if len(file.Diagnostics) != 0 {
					t.Errorf("unexpected diagnostics: %#v", file.Diagnostics)
					return
				}
				if file.Kind != FileKindClass || len(file.Declarations) != 1 {
					t.Errorf("file = %#v", file)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestResultHasErrors(t *testing.T) {
	ok := Result{Files: []File{{Kind: FileKindClass}}}
	if ok.HasErrors() {
		t.Fatal("unexpected errors")
	}
	bad := Result{Files: []File{{Diagnostics: []Diagnostic{{Severity: Error, Message: "broken"}}}}}
	if !bad.HasErrors() {
		t.Fatal("expected file diagnostic error")
	}
	reportBad := Result{Diagnostics: []Diagnostic{{Severity: Error, Message: "broken"}}}
	if !reportBad.HasErrors() {
		t.Fatal("expected report diagnostic error")
	}
}

func TestParseStructuredAnnotations(t *testing.T) {
	file := NewParser().ParseSource("Probe.cls", `
@IsTest(SeeAllData = false)
public class Probe {
  @AuraEnabled(cacheable = true)
  public static String run() { return 'ok'; }
}`)
	if len(file.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
	}
	decl := file.Declarations[0]
	if len(decl.Annotations) != 1 || decl.Annotations[0].Name != "IsTest" || len(decl.Annotations[0].Arguments) != 1 {
		t.Fatalf("class annotations = %#v", decl.Annotations)
	}
	method := decl.Members[0]
	if len(method.Annotations) != 1 || method.Annotations[0].Name != "AuraEnabled" || method.Annotations[0].Arguments[0].Name != "cacheable" {
		t.Fatalf("method annotations = %#v", method.Annotations)
	}
}

func TestParseStructuredAnnotationArgumentsPreserveStringsAndRanges(t *testing.T) {
	source := `@InvocableMethod(label = 'a = b', description = 'hello world') public class Probe {}`
	file := NewParser().ParseSource("Probe.cls", source)
	annotation := file.Declarations[0].Annotations[0]
	if len(annotation.Arguments) != 2 || annotation.Arguments[0].Name != "label" || annotation.Arguments[0].Value != "'a = b'" {
		t.Fatalf("arguments = %#v", annotation.Arguments)
	}
	for _, argument := range annotation.Arguments {
		if argument.Range.Start.Offset < 0 || argument.Range.End.Offset <= argument.Range.Start.Offset || source[argument.Range.Start.Offset:argument.Range.End.Offset] == "" {
			t.Fatalf("argument range = %#v", argument.Range)
		}
	}
}

func TestParseStructuredAnnotationArgumentsSupportWhitespaceSeparation(t *testing.T) {
	source := `public class Probe { @AuraEnabled(cacheable=true scope='global') public static String run() { return 'ok'; } }`
	file := NewParser().ParseSource("Probe.cls", source)
	annotation := file.Declarations[0].Members[0].Annotations[0]
	if len(annotation.Arguments) != 2 || annotation.Arguments[0].Name != "cacheable" || annotation.Arguments[0].Value != "true" || annotation.Arguments[1].Name != "scope" || annotation.Arguments[1].Value != "'global'" {
		t.Fatalf("arguments = %#v", annotation.Arguments)
	}
}

func TestParseAnnotationModifierPreservesBackslashEscapedApostrophe(t *testing.T) {
	for name, tc := range map[string]struct {
		source string
		want   string
	}{
		"one preceding backslash": {
			source: `Description='This isn\'t positional'`,
			want:   `@InvocableVariable(Description='This isn\'t positional')`,
		},
		"two preceding backslashes": {
			source: `Description='ends with two backslashes\\' Label='after'`,
			want:   `@InvocableVariable(Description='ends with two backslashes\\'Label='after')`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			source := "public class Probe {\n  @InvocableVariable(" + tc.source + ")\n  public String value;\n}"
			file := NewParser().ParseSource("Probe.cls", source)
			if len(file.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
			}
			modifiers := file.Declarations[0].Members[0].Modifiers
			if len(modifiers) == 0 || modifiers[0] != tc.want {
				t.Fatalf("modifiers = %#v", modifiers)
			}
		})
	}
}

func TestAnnotationStringScannerHandlesDoubledApostrophe(t *testing.T) {
	arguments := splitAnnotationArguments(`Description='it''s doubled' Label='after'`)
	if len(arguments) != 2 || strings.TrimSpace(arguments[0].text) != `Description='it''s doubled'` || strings.TrimSpace(arguments[1].text) != `Label='after'` {
		t.Fatalf("arguments = %#v", arguments)
	}
	if got, want := normalizeAnnotationText(`@InvocableVariable(Description='it''s doubled' Label='after')`), `@InvocableVariable(Description='it''s doubled'Label='after')`; got != want {
		t.Fatalf("normalized annotation = %q, want %q", got, want)
	}
}

func TestHasOddPrecedingBackslashes(t *testing.T) {
	for name, tc := range map[string]struct {
		source string
		index  int
		want   bool
	}{
		"one":     {source: `'one\'two'`, index: len(`'one\`), want: true},
		"two":     {source: `'two\\'`, index: len(`'two\\`), want: false},
		"doubled": {source: `'it''s'`, index: len(`'it`), want: false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := hasOddPrecedingBackslashes(tc.source, tc.index); got != tc.want {
				t.Fatalf("hasOddPrecedingBackslashes(%q, %d) = %t, want %t", tc.source, tc.index, got, tc.want)
			}
		})
	}
}

func TestParseSalesforceInvocableVariableWithEscapedApostrophe(t *testing.T) {
	source := `public class Probe {
  @InvocableVariable(
    Required=false
    Label='Email From Org-Wide Id'
    Description='The Salesforce Id of the Organization-Wide email address to use as the "From" in emails. If this isn\'t set, the email address of the user sending the email is used instead.'
  )
  public String value;
}`
	file := NewParser().ParseSource("Probe.cls", source)
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

func TestParseRejectsUnterminatedAnnotationStringAfterEscapedApostrophe(t *testing.T) {
	source := `public class Probe {
  @InvocableVariable(Description='This isn\'t)
  public String value;
}`
	file := NewParser().ParseSource("Probe.cls", source)
	if !file.HasErrors() {
		t.Fatalf("unterminated annotation string was accepted: %#v", file)
	}
}

func TestParseRejectsMultipleSuppressWarningsArguments(t *testing.T) {
	file := NewParser().ParseSource("Probe.cls", `@SuppressWarnings('one', 'two') public class Probe {}`)
	if !file.HasErrors() {
		t.Fatalf("multiple SuppressWarnings arguments were accepted: %#v", file)
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

func BenchmarkParseClass(b *testing.B) {
	src := benchmarkApexClass(40)
	parser := NewParser()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		file := parser.ParseSource("Benchmark.cls", src)
		if len(file.Diagnostics) != 0 {
			b.Fatalf("unexpected diagnostics: %#v", file.Diagnostics)
		}
	}
}

func benchmarkApexClass(methods int) string {
	var src strings.Builder
	src.WriteString("public with sharing class Benchmark {\n")
	src.WriteString("  public Integer seed { get; set; }\n")
	for i := 0; i < methods; i++ {
		fmt.Fprintf(&src, "  public static Integer method%d(Integer value) {\n", i)
		src.WriteString("    Integer total = 0;\n")
		src.WriteString("    for (Integer i = 0; i < value; i++) {\n")
		src.WriteString("      total = total + i;\n")
		src.WriteString("    }\n")
		src.WriteString("    return total;\n")
		src.WriteString("  }\n")
	}
	src.WriteString("}\n")
	return src.String()
}

func containsModifier(mods []string, expected string) bool {
	for _, mod := range mods {
		if strings.EqualFold(mod, expected) {
			return true
		}
	}
	return false
}
