package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestVMRecordFieldPathPreservesMissingNestedParentRelationshipNull(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Line__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Line__c",
			Relations: []storage.Relationship{{
				Field:              "Product__c",
				ParentObjects:      []string{"Product__c"},
				ParentRelationship: "Product__r",
			}},
		},
	}
	org.Objects["Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Product__c",
			Relations: []storage.Relationship{{
				Field:              "Event__c",
				ParentObjects:      []string{"Event__c"},
				ParentRelationship: "Event__r",
			}},
		},
	}
	org.Objects["Event__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Event__c"}}

	machine := New(nil)
	machine.SetOrg(&org)
	value := machine.vmValueFromRecord(storage.Record{
		Object: "Line__c",
		Fields: map[string]storage.Value{
			"Product__r.Event__r": storage.NullValue(),
			"Product__r.event__r": storage.StringValue("wrong-shell"),
		},
	})

	event, err := machine.lookupPath(value, []string{"Product__r", "Event__r"})
	if err != nil {
		t.Fatal(err)
	}
	if event.Kind != ValueNull || event.Type != "Event__c" || !isRelationshipNull(event) {
		t.Fatalf("event relationship = %#v", event)
	}
}

func TestExecSObjectPropertyGetterNullSupportsFieldAccess(t *testing.T) {
	getterProgram, err := CompileAnonymous(`return null;`)
	if err != nil {
		t.Fatal(err)
	}
	methodProgram, err := CompileAnonymous(`return null;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Harness h = new Harness();
System.assertEquals(null, h.AccountValue);
System.assertEquals(null, h.AccountValue.Name);
try {
    String name = h.getAccountValue().Name;
    System.assert(false, 'method null SObject dereference should throw');
} catch (System.NullPointerException e) {
    System.assert(e.getMessage().contains('Attempt to de-reference a null object'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Harness",
		Fields: map[string]Field{
			"AccountValue": {Name: "AccountValue", Type: "Account", Property: true, Getter: &Method{Name: "Harness.AccountValue.get", ClassName: "Harness", ReturnType: "Account", Program: getterProgram}},
		},
		Methods: map[string]Method{
			"getAccountValue": {Name: "Harness.getAccountValue", ClassName: "Harness", ReturnType: "Account", Program: methodProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCoerceAssignableAcceptsNamespaceAliasWhenNamespaceMatchesCurrentClass(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "verifiable", Namespace: "verifiable"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "responseData", Namespace: "verifiable"}); err != nil {
		t.Fatal(err)
	}
	machine.currentClass = "verifiable"
	value := Object("verifiable.responseData")
	if _, err := machine.coerceAssignable("responseData", value); err != nil {
		t.Fatal(err)
	}
}

func TestExecAssignStringSplitToListIDDoesNotValidateElementsEagerly(t *testing.T) {
	program, err := CompileAnonymous(`
List<Id> ids = ''.split(',');
System.assertEquals(1, ids.size());
System.assertEquals('[""]', JSON.serialize(ids));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRegisteredStaticMethod(t *testing.T) {
	methodProgram, err := CompileAnonymous("return a + b;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("System.assertEquals(3, MathUtil.add(1, 2));")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "MathUtil.add",
		ReturnType: "Integer",
		Params: []Param{
			{Name: "a", Type: "Integer"},
			{Name: "b", Type: "Integer"},
		},
		Program: methodProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringValueOfNestedClassUsesLocalName(t *testing.T) {
	program, err := CompileAnonymous(`
Outer.Inner inner = new Outer.Inner();
System.assertEquals('Inner:{}', String.valueOf(inner));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Outer"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Outer.Inner"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNestedSObjectFieldAssignmentThroughObjectPropertyPersists(t *testing.T) {
	program, err := CompileAnonymous(`
Holder holder = new Holder();
holder.Cart = new Cart__c(Batch__c = 'a00000000000001AAA');
holder.Cart.putSObject('Batch__r', new Batch__c(Id = 'a00000000000001AAA'));
holder.Cart.Batch__c = null;
System.assertEquals(null, holder.Cart.Batch__c);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "NU"
	org.Objects["NU__Cart__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "NU__Cart__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"NU__Batch__c": {APIName: "NU__Batch__c", Type: storage.FieldReference, ReferenceTo: []string{"NU__Batch__c"}},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["NU__Batch__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "NU__Batch__c",
			KeyPrefix: "a00",
			Fields: map[string]storage.Field{
				"Id": {APIName: "Id", Type: storage.FieldID},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "Holder",
		Fields: map[string]Field{
			"Cart": {Name: "Cart", Type: "Cart__c"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecIntegerPropertyAssignmentThroughIndexedListUsesIntegerLiteral(t *testing.T) {
	program, err := CompileAnonymous(`
List<Item> items = new List<Item>{ new Item() };
items[0].PaymentSortOrder = 200;
System.assertEquals(200, items[0].PaymentSortOrder);
Holder holder = new Holder();
holder.Items = items;
holder.getItems()[0].PaymentSortOrder = 100;
System.assertEquals(100, holder.getItems()[0].PaymentSortOrder);
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Item",
		Fields: map[string]Field{
			"PaymentSortOrder": {Name: "PaymentSortOrder", Type: "Integer", Property: true},
		},
	}); err != nil {
		t.Fatal(err)
	}
	methodProgram, err := CompileAnonymous("return this.Items;")
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Holder",
		Fields: map[string]Field{
			"Items": {Name: "Items", Type: "List<Item>", Property: true},
		},
		Methods: map[string]Method{
			"getItems": {
				Name:       "Holder.getItems",
				ClassName:  "Holder",
				ReturnType: "List<Item>",
				Program:    methodProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCoerceUntypedIntegralDecimalToInteger(t *testing.T) {
	machine := New(nil)
	decimal := Decimal(200)
	decimal.Text = "200.0"
	coerced, err := machine.coerceAssignable("Integer", decimal)
	if err != nil {
		t.Fatal(err)
	}
	if coerced.Kind != ValueInt || coerced.Int != 200 {
		t.Fatalf("coerced = %#v, want integer 200", coerced)
	}
	typedDecimal := decimal
	typedDecimal.Static = "Decimal"
	if _, err := machine.coerceAssignable("Integer", typedDecimal); err == nil {
		t.Fatal("typed Decimal assigned to Integer without error")
	}
}

func TestExecRegisteredStaticMethodMatchesNestedSObjectCollectionMap(t *testing.T) {
	methodProgram, err := CompileAnonymous("return values.size();")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Map<Id, List<sObject>> values = new Map<Id, List<sObject>>();
values.put('001000000000001AAA', new List<sObject>());
System.assertEquals(1, CouponReApplier.reapplyCartCoupons(values));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	machine := New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterMethod(Method{
		Name:       "CouponReApplier.reApplyCartCoupons",
		ClassName:  "CouponReApplier",
		IsStatic:   true,
		ReturnType: "Integer",
		Params:     []Param{{Name: "values", Type: "Map<Id, List<CartItemLine__c>>"}},
		Program:    methodProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRegisteredMethodDoesNotLeakParams(t *testing.T) {
	methodProgram, err := CompileAnonymous("return a;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("Integer a = 9; System.assertEquals(1, Util.id(1)); System.assertEquals(9, a);")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "Util.id",
		ReturnType: "Integer",
		Params:     []Param{{Name: "a", Type: "Integer"}},
		Program:    methodProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRecursiveMethodReturnsRuntimeError(t *testing.T) {
	methodProgram, err := CompileAnonymous("return Loop.run();")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("Loop.run();")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{Name: "Loop.run", ClassName: "Loop", ReturnType: "Object", Program: methodProgram}); err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "maximum Apex call stack depth exceeded") {
		t.Fatalf("expected call depth error, got %v", err)
	}
}

func TestExecRegisteredMethodCoercesParamsAndReturns(t *testing.T) {
	methodProgram, err := CompileAnonymous("return value;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("System.assertEquals(1.5, Util.id(1) + 0.5);")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "Util.id",
		ReturnType: "Decimal",
		Params:     []Param{{Name: "value", Type: "Decimal"}},
		Program:    methodProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRegisteredMethodAcceptsMessagingEmailBaseType(t *testing.T) {
	methodProgram, err := CompileAnonymous("return 1;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
EmailSink sink = new EmailSink();
Messaging.SingleEmailMessage email = new Messaging.SingleEmailMessage();
System.assertEquals(1, sink.registerEmail(email));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "EmailSink",
		Methods: map[string]Method{
			"registerEmail": {
				Name:       "EmailSink.registerEmail",
				ClassName:  "EmailSink",
				ReturnType: "Integer",
				Params:     []Param{{Name: "email", Type: "Messaging.Email"}},
				Program:    methodProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRegisteredStaticMethodWithBranchingReturn(t *testing.T) {
	methodProgram, err := CompileAnonymous(`
if (a > b) {
	return a;
} else {
	return b;
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals(5, MathUtil.max(5, 2));
System.assertEquals(7, MathUtil.max(3, 7));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "MathUtil.max",
		ReturnType: "Integer",
		Params: []Param{
			{Name: "a", Type: "Integer"},
			{Name: "b", Type: "Integer"},
		},
		Program: methodProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRegisteredStaticMethodWithWhileLoop(t *testing.T) {
	methodProgram, err := CompileAnonymous(`
Integer total = 0;
Integer i = 1;
while (i <= n) {
	total = total + i;
	i = i + 1;
}
return total;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("System.assertEquals(15, MathUtil.sumTo(5));")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "MathUtil.sumTo",
		ReturnType: "Integer",
		Params:     []Param{{Name: "n", Type: "Integer"}},
		Program:    methodProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRegisteredInstanceMethod(t *testing.T) {
	methodProgram, err := CompileAnonymous("return a + b;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Calculator calc = new Calculator();
System.assertEquals(7, calc.add(3, 4));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "Calculator.add",
		ReturnType: "Integer",
		Params: []Param{
			{Name: "a", Type: "Integer"},
			{Name: "b", Type: "Integer"},
		},
		Program: methodProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRegisteredClassFieldsConstructorAndInstanceState(t *testing.T) {
	ctorProgram, err := CompileAnonymous("value = seed;")
	if err != nil {
		t.Fatal(err)
	}
	incProgram, err := CompileAnonymous("value = value + amount; return value;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Counter c = new Counter(2);
System.assertEquals(5, c.inc(3));
System.assertEquals(6, c.inc(1));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Counter",
		Fields: map[string]Field{
			"value": {Name: "value", Type: "Integer"},
		},
		Constructors: []Method{{
			Name:          "Counter.<init>",
			ClassName:     "Counter",
			Params:        []Param{{Name: "seed", Type: "Integer"}},
			Program:       ctorProgram,
			IsConstructor: true,
		}},
		Methods: map[string]Method{
			"inc": {
				Name:       "Counter.inc",
				ClassName:  "Counter",
				ReturnType: "Integer",
				Params:     []Param{{Name: "amount", Type: "Integer"}},
				Program:    incProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamespaceQualifiedClassName(t *testing.T) {
	methodProgram, err := CompileAnonymous("return value;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
pkg.Counter c = new pkg.Counter();
c.value = 7;
System.assertEquals(7, c.score());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "pkg.Counter",
		Fields: map[string]Field{
			"value": {Name: "value", Type: "Integer"},
		},
		Methods: map[string]Method{
			"score": {Name: "pkg.Counter.score", ClassName: "pkg.Counter", ReturnType: "Integer", Program: methodProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStaticAndInstanceInitializers(t *testing.T) {
	staticInit, err := CompileAnonymous("seed = 4;")
	if err != nil {
		t.Fatal(err)
	}
	instanceInit, err := CompileAnonymous("value = seed + 1;")
	if err != nil {
		t.Fatal(err)
	}
	scoreProgram, err := CompileAnonymous("return value;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals(4, Counter.seed);
Counter c = new Counter();
System.assertEquals(5, c.score());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Counter",
		StaticFields: map[string]Field{
			"seed": {Name: "seed", Type: "Integer", Static: true},
		},
		Fields: map[string]Field{
			"value": {Name: "value", Type: "Integer"},
		},
		StaticInitializers: []Method{{
			Name:      "Counter.<static_init>",
			ClassName: "Counter",
			Program:   staticInit,
			IsStatic:  true,
		}},
		InstanceInitializers: []Method{{
			Name:      "Counter.<init_block>",
			ClassName: "Counter",
			Program:   instanceInit,
		}},
		Methods: map[string]Method{
			"score": {Name: "Counter.score", ClassName: "Counter", ReturnType: "Integer", Program: scoreProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInstanceFieldsAreCaseInsensitiveAcrossSuperclass(t *testing.T) {
	getProgram, err := CompileAnonymous("return this.enforceCRUD;")
	if err != nil {
		t.Fatal(err)
	}
	setProgram, err := CompileAnonymous("this.enforceFLS = enforce; return this.enforceFls;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
ChildSelector selector = new ChildSelector();
System.assertEquals(true, selector.isCrud());
System.assertEquals(false, selector.setFLS(false));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "BaseSelector",
		Fields: map[string]Field{
			"enforceCrud": {Name: "enforceCrud", Type: "Boolean", InitialValue: Bool(true)},
			"enforceFls":  {Name: "enforceFls", Type: "Boolean", InitialValue: Bool(true)},
		},
		Methods: map[string]Method{
			"isCrud": {Name: "BaseSelector.isCrud", ClassName: "BaseSelector", ReturnType: "Boolean", Program: getProgram},
			"setFLS": {
				Name:       "BaseSelector.setFLS",
				ClassName:  "BaseSelector",
				ReturnType: "Boolean",
				Params:     []Param{{Name: "enforce", Type: "Boolean"}},
				Program:    setProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "ChildSelector",
		SuperClass: "BaseSelector",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStaticInitializersRunLazilyOnFirstClassUse(t *testing.T) {
	staticInit, err := CompileAnonymous("seed = 4;")
	if err != nil {
		t.Fatal(err)
	}
	getProgram, err := CompileAnonymous("return seed;")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Counter",
		StaticFields: map[string]Field{
			"seed": {Name: "seed", Type: "Integer", Static: true},
		},
		StaticInitializers: []Method{{
			Name:      "Counter.<static_init>",
			ClassName: "Counter",
			Program:   staticInit,
			IsStatic:  true,
		}},
		Methods: map[string]Method{
			"get": {Name: "Counter.get", ClassName: "Counter", ReturnType: "Integer", IsStatic: true, Program: getProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if got := machine.Classes["Counter"].StaticFields["seed"].Value.Int; got != 0 {
		t.Fatalf("seed initialized eagerly = %d", got)
	}
	value, err := machine.CallStatic("Counter.get", nil)
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueInt || value.Int != 4 {
		t.Fatalf("Counter.get = %#v, want 4", value)
	}
}

func TestDuplicateClassStaticInitializersPreferProjectLast(t *testing.T) {
	projectInit, err := CompileAnonymous("selector = 'project';")
	if err != nil {
		t.Fatal(err)
	}
	dependencyInit, err := CompileAnonymous("selector = 'dependency';")
	if err != nil {
		t.Fatal(err)
	}
	getProgram, err := CompileAnonymous("return selector;")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:      "AccountSelectorTest",
		Namespace: "znu",
		StaticFields: map[string]Field{
			"selector": {Name: "selector", Type: "String", Static: true},
		},
		StaticInitializers: []Method{{
			Name:      "AccountSelectorTest.<static_project>",
			ClassName: "AccountSelectorTest",
			Program:   projectInit,
			IsStatic:  true,
		}},
		Methods: map[string]Method{
			"get": {Name: "AccountSelectorTest.get", ClassName: "AccountSelectorTest", ReturnType: "String", IsStatic: true, Program: getProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "AccountSelectorTest",
		Namespace:  "znu",
		Dependency: true,
		StaticFields: map[string]Field{
			"selector": {Name: "selector", Type: "String", Static: true},
		},
		StaticInitializers: []Method{{
			Name:      "AccountSelectorTest.<static_dependency>",
			ClassName: "AccountSelectorTest",
			Program:   dependencyInit,
			IsStatic:  true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	value, err := machine.CallStatic("AccountSelectorTest.get", nil)
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueString || value.Text != "project" {
		t.Fatalf("selector = %#v, want project", value)
	}
}

func TestDuplicateClassNullOverloadPrefersProjectMethod(t *testing.T) {
	projectSelect, err := CompileAnonymous("return 'project';")
	if err != nil {
		t.Fatal(err)
	}
	dependencySelect, err := CompileAnonymous("return 'dependency';")
	if err != nil {
		t.Fatal(err)
	}
	caller, err := CompileAnonymous("return new AdditionalSettingSelector().selectById(null);")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:      "AdditionalSettingSelector",
		Namespace: "znu",
		Methods: map[string]Method{
			"selectById": {
				Name:       "AdditionalSettingSelector.selectById",
				ClassName:  "AdditionalSettingSelector",
				ReturnType: "String",
				Params:     []Param{{Name: "idParam", Type: "Id"}},
				Program:    projectSelect,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "AdditionalSettingSelector",
		Namespace:  "znu",
		Dependency: true,
		Methods: map[string]Method{
			"selectById": {
				Name:       "AdditionalSettingSelector.selectById",
				ClassName:  "AdditionalSettingSelector",
				ReturnType: "String",
				Params:     []Param{{Name: "idSet", Type: "Set<Id>"}},
				Program:    dependencySelect,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:      "Caller",
		Namespace: "znu",
		Methods: map[string]Method{
			"run": {Name: "Caller.run", ClassName: "Caller", ReturnType: "String", IsStatic: true, Program: caller},
		},
	}); err != nil {
		t.Fatal(err)
	}
	value, err := machine.CallStatic("Caller.run", nil)
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueString || value.Text != "project" {
		t.Fatalf("selectById(null) = %#v, want project", value)
	}
}

func TestDuplicateSuperclassImplicitConstructorPrefersSubclassProvenance(t *testing.T) {
	dependencyCtor, err := CompileAnonymous("ObserversReady = true; PageNumber = 1;")
	if err != nil {
		t.Fatal(err)
	}
	projectCtor, err := CompileAnonymous("PageSize = 10; PageNumber = 1;")
	if err != nil {
		t.Fatal(err)
	}
	subclassCtor, err := CompileAnonymous("super();")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
StorePagination pagination = new StorePagination();
System.assertEquals(true, pagination.ObserversReady);
System.assertEquals(10, pagination.PageSize);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "Pageable",
		Dependency: true,
		Fields: map[string]Field{
			"PageSize":   {Name: "PageSize", Type: "Integer", Property: true},
			"PageNumber": {Name: "PageNumber", Type: "Integer", Property: true},
			"ObserversReady": {
				Name:     "ObserversReady",
				Type:     "Boolean",
				Property: true,
			},
		},
		Constructors: []Method{{
			Name:          "Pageable.<init>",
			ClassName:     "Pageable",
			IsConstructor: true,
			Program:       dependencyCtor,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Pageable",
		Fields: map[string]Field{
			"PageSize":   {Name: "PageSize", Type: "Integer", Property: true},
			"PageNumber": {Name: "PageNumber", Type: "Integer", Property: true},
			"ObserversReady": {
				Name:     "ObserversReady",
				Type:     "Boolean",
				Property: true,
			},
		},
		Constructors: []Method{{
			Name:          "Pageable.<init>",
			ClassName:     "Pageable",
			IsConstructor: true,
			Program:       projectCtor,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "StorePagination",
		SuperClass: "Pageable",
		Constructors: []Method{{
			Name:          "StorePagination.<init>",
			ClassName:     "StorePagination",
			IsConstructor: true,
			Program:       subclassCtor,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecIntegerPropertyGetterArithmetic(t *testing.T) {
	program, err := CompileAnonymous(`
Pageable pager = new Pageable();
System.assertEquals(10, pager.PageSize);
System.assertEquals(0, pager.Offset);
`)
	if err != nil {
		t.Fatal(err)
	}
	pageIndex, err := CompileAnonymous("return PageNumber - 1;")
	if err != nil {
		t.Fatal(err)
	}
	offset, err := CompileAnonymous("return PageSize * PageIndex;")
	if err != nil {
		t.Fatal(err)
	}
	init, err := CompileAnonymous("PageSize = 10; PageNumber = 1;")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Pageable",
		Fields: map[string]Field{
			"PageSize":   {Name: "PageSize", Type: "Integer"},
			"PageNumber": {Name: "PageNumber", Type: "Integer"},
			"PageIndex": {
				Name:   "PageIndex",
				Type:   "Integer",
				Getter: &Method{Name: "Pageable.PageIndex.get", ClassName: "Pageable", ReturnType: "Integer", Program: pageIndex},
			},
			"Offset": {
				Name:   "Offset",
				Type:   "Integer",
				Getter: &Method{Name: "Pageable.Offset.get", ClassName: "Pageable", ReturnType: "Integer", Program: offset},
			},
		},
		Constructors: []Method{{
			Name:          "Pageable.<init>",
			ClassName:     "Pageable",
			IsConstructor: true,
			Program:       init,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCloneRuntimeKeepsStaticStateIsolated(t *testing.T) {
	getProgram, err := CompileAnonymous("return seed;")
	if err != nil {
		t.Fatal(err)
	}
	template := New(nil)
	if err := template.RegisterClass(Class{
		Name: "Counter",
		StaticFields: map[string]Field{
			"seed": {Name: "seed", Type: "Integer", Static: true},
		},
		Methods: map[string]Method{
			"get": {Name: "Counter.get", ClassName: "Counter", ReturnType: "Integer", IsStatic: true, Program: getProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}

	first := template.CloneRuntime(nil)
	class := first.Classes["Counter"]
	field := class.StaticFields["seed"]
	field.Value = Int(7)
	class.StaticFields["seed"] = field
	first.Classes["Counter"] = class

	second := template.CloneRuntime(nil)
	value, err := second.CallStatic("Counter.get", nil)
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueNull {
		t.Fatalf("Counter.get from second clone = %#v, want null static default", value)
	}
}

func TestResetStaticsClonesExplicitCollectionInitializers(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Registry",
		StaticFields: map[string]Field{
			"values": {Name: "values", Type: "List<String>", Static: true, InitialValue: List()},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.ResetStatics(); err != nil {
		t.Fatal(err)
	}
	class := machine.Classes["Registry"]
	first := class.StaticFields["values"].Value
	first.List = append(first.List, String("leaked"))
	field := class.StaticFields["values"]
	field.Value = first
	class.StaticFields["values"] = field
	machine.Classes["Registry"] = class

	if err := machine.ResetStatics(); err != nil {
		t.Fatal(err)
	}
	reset := machine.Classes["Registry"].StaticFields["values"].Value
	if reset.Kind != ValueList || len(reset.List) != 0 {
		t.Fatalf("reset static list = %#v, want empty cloned initializer", reset)
	}
	if reset.Ref == first.Ref {
		t.Fatalf("reset static list reused ref %d", reset.Ref)
	}
}

func TestStaticValueRefCacheExtendsOnStaticAssignment(t *testing.T) {
	machine := New(nil)
	oldValue := List(String("old"))
	newValue := List(String("new"))
	machine.staticValueRefs = map[uint64]bool{oldValue.Ref: true}

	machine.invalidateStaticValueRefsForChange(oldValue, newValue)

	if machine.staticValueRefs == nil {
		t.Fatalf("static ref cache was discarded")
	}
	if !machine.staticValueRefs[newValue.Ref] {
		t.Fatalf("new static ref %d was not remembered: %#v", newValue.Ref, machine.staticValueRefs)
	}
}

func TestFrameworkMethodCountRecorderRollbackCapturesOnFirstWrite(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "framework_MethodCountRecorder",
		StaticFields: map[string]Field{
			"orderedMethodCalls": {Name: "orderedMethodCalls", Type: "List<framework_InvocationOnMock>", Static: true, Value: List(String("old"))},
		},
	}); err != nil {
		t.Fatal(err)
	}

	rollback := machine.beginFrameworkMethodCountRecorderRollback()
	if len(rollback.values) != 0 {
		t.Fatalf("rollback captured before write: %#v", rollback.values)
	}
	updated, ok := machine.frameworkMethodCountRecorderStatic("orderedMethodCalls")
	if !ok {
		t.Fatalf("missing recorder static")
	}
	updated.List = append(updated.List, String("new"))
	machine.setFrameworkMethodCountRecorderStatic("orderedMethodCalls", updated)
	if len(rollback.values) != 1 {
		t.Fatalf("rollback did not capture first write: %#v", rollback.values)
	}
	machine.endFrameworkMethodCountRecorderRollback(rollback, true)

	restored, ok := machine.frameworkMethodCountRecorderStatic("orderedMethodCalls")
	if !ok {
		t.Fatalf("missing restored recorder static")
	}
	if len(restored.List) != 1 || restored.List[0].Text != "old" {
		t.Fatalf("restored recorder = %#v", restored)
	}
}

func TestCustomObjectMapKeyUsesStableFieldValues(t *testing.T) {
	firstMock := Object("Mocked")
	secondMock := Object("Mocked")
	first := Object("QualifiedMethod")
	first.Fields["typeName"] = String("Selector")
	first.Fields["methodName"] = String("selectById")
	first.Fields["methodArgTypes"] = List(platformScalar("Type", "Id"))
	first.Fields["mockInstance"] = firstMock
	same := Object("QualifiedMethod")
	same.Fields["typeName"] = String("Selector")
	same.Fields["methodName"] = String("selectById")
	same.Fields["methodArgTypes"] = List(platformScalar("Type", "Id"))
	same.Fields["mockInstance"] = firstMock
	differentArgs := Object("QualifiedMethod")
	differentArgs.Fields["typeName"] = String("Selector")
	differentArgs.Fields["methodName"] = String("selectById")
	differentArgs.Fields["methodArgTypes"] = List(platformScalar("Type", "Set<Id>"))
	differentArgs.Fields["mockInstance"] = firstMock
	differentMock := Object("QualifiedMethod")
	differentMock.Fields["typeName"] = String("Selector")
	differentMock.Fields["methodName"] = String("selectById")
	differentMock.Fields["methodArgTypes"] = List(platformScalar("Type", "Id"))
	differentMock.Fields["mockInstance"] = secondMock

	if mapKey(first) != mapKey(same) {
		t.Fatalf("same value-object key mismatch: %q != %q", mapKey(first), mapKey(same))
	}
	if mapKey(first) == mapKey(differentArgs) {
		t.Fatalf("value-object key ignored argument types: %q", mapKey(first))
	}
	if mapKey(first) != mapKey(differentMock) {
		t.Fatalf("value-object key should ignore arbitrary object references: %q != %q", mapKey(first), mapKey(differentMock))
	}
}

func TestExecMapKeyUsesCustomHashCode(t *testing.T) {
	equalsProgram, err := CompileAnonymous(`
Key otherKey = other instanceof Key ? (Key)other : null;
return otherKey != null && Code == otherKey.Code;
`)
	if err != nil {
		t.Fatal(err)
	}
	hashProgram, err := CompileAnonymous("return Code == null ? 0 : Code.hashCode();")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Key left = new Key('same');
Key right = new Key('same');
Map<Key, String> values = new Map<Key, String>();
values.put(left, 'matched');
System.assertEquals('matched', values.get(right));
`)
	if err != nil {
		t.Fatal(err)
	}
	ctorProgram, err := CompileAnonymous("Code = code;")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Key",
		Fields: map[string]Field{
			"Code": {Name: "Code", Type: "String"},
		},
		Constructors: []Method{{Name: "Key.<init>", ClassName: "Key", Params: []Param{{Name: "code", Type: "String"}}, Program: ctorProgram}},
		Methods: map[string]Method{
			"equals":   {Name: "Key.equals", ClassName: "Key", ReturnType: "Boolean", Params: []Param{{Name: "other", Type: "Object"}}, Program: equalsProgram},
			"hashCode": {Name: "Key.hashCode", ClassName: "Key", ReturnType: "Integer", Program: hashProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMapKeyUsesObjectIdentityWithoutCustomHashCode(t *testing.T) {
	program, err := CompileAnonymous(`
Key left = new Key('same');
Key right = new Key('same');
Map<Key, String> values = new Map<Key, String>{
	left => 'left',
	right => 'right'
};
System.assertEquals(2, values.size());
System.assertEquals('left', values.get(left));
System.assertEquals('right', values.get(right));
`)
	if err != nil {
		t.Fatal(err)
	}
	ctorProgram, err := CompileAnonymous("Code = code;")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Key",
		Fields: map[string]Field{
			"Code": {Name: "Code", Type: "String"},
		},
		Constructors: []Method{{Name: "Key.<init>", ClassName: "Key", Params: []Param{{Name: "code", Type: "String"}}, Program: ctorProgram}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCastsDatetimeToDate(t *testing.T) {
	program, err := CompileAnonymous(`
Datetime source = Datetime.newInstance(2026, 5, 2, 9, 30, 0);
Date actual = (Date)source;
System.assertEquals(Date.newInstance(2026, 5, 2), actual);
Date normalized = (Date)Date.valueOf('2026-05-02 00:00:00');
System.assertEquals(Date.newInstance(2026, 5, 2), normalized);
System.assertEquals(Date.valueOf('2026-05-02 00:00:00'), Date.newInstance(2026, 5, 2));
System.assertEquals(DateTime.newInstance(2026, 5, 2, 0, 0, 0), Date.newInstance(2026, 5, 2));
System.assertEquals(Date.valueOf('2026-05-02 00:00:00'), '2026-05-02');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeOverloadResolutionUsesTypedNullCasts(t *testing.T) {
	program, err := CompileAnonymous(`
Selector selector = new Selector();
Object placeholder = null;
System.assertEquals('fieldsets', selector.selectById((Set<Id>)placeholder, (List<Schema.FieldSet>)placeholder));
System.assertEquals('fieldsets', selector.selectById((Set<Id>)Matcher.anyObject(), (List<Schema.FieldSet>)Matcher.anyList()));
System.assertEquals('fieldsets', selector.selectById((Set<Id>)Matcher.anyObject(), (List<FieldSet>)Matcher.anyList()));
`)
	if err != nil {
		t.Fatal(err)
	}
	fieldsetsProgram, err := CompileAnonymous("return 'fieldsets';")
	if err != nil {
		t.Fatal(err)
	}
	stringsProgram, err := CompileAnonymous("return 'strings';")
	if err != nil {
		t.Fatal(err)
	}
	anyObjectProgram, err := CompileAnonymous("return null;")
	if err != nil {
		t.Fatal(err)
	}
	anyListProgram, err := CompileAnonymous("return (List<Object>)null;")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Selector"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Matcher"}); err != nil {
		t.Fatal(err)
	}
	for _, method := range []Method{
		{Name: "Selector.selectById", ClassName: "Selector", ReturnType: "String", Params: []Param{{Name: "ids", Type: "Set<Id>"}, {Name: "fieldSets", Type: "List<Schema.FieldSet>"}}, Program: fieldsetsProgram},
		{Name: "Selector.selectById", ClassName: "Selector", ReturnType: "String", Params: []Param{{Name: "ids", Type: "Set<Id>"}, {Name: "fields", Type: "Set<String>"}}, Program: stringsProgram},
		{Name: "Matcher.anyObject", ClassName: "Matcher", ReturnType: "Object", IsStatic: true, Program: anyObjectProgram},
		{Name: "Matcher.anyList", ClassName: "Matcher", ReturnType: "List<Object>", IsStatic: true, Program: anyListProgram},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestResolveClassNamePrefersLexicalNestedType(t *testing.T) {
	machine := New(nil)
	machine.currentClass = "Outer"
	if err := machine.RegisterClass(Class{Name: "Nested"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Outer"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Outer.Nested", SuperClass: "Base"}); err != nil {
		t.Fatal(err)
	}
	resolved, ok := machine.resolveClassName("Nested")
	if !ok || resolved != "Outer.Nested" {
		t.Fatalf("resolved Nested = %q, %v; want Outer.Nested", resolved, ok)
	}
}

func TestExecTestCreateStubUsesArityMetadataForTypedNullOverload(t *testing.T) {
	providerProgram, err := CompileAnonymous("return 'handled';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Selector proxy = (Selector)Test.createStub(Selector.class, new Provider());
Object placeholder = null;
System.assertEquals('handled', proxy.selectById((Set<Id>)placeholder, (List<Schema.FieldSet>)placeholder));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{Name: "Selector"}); err != nil {
		t.Fatal(err)
	}
	machine.MethodOverloads["Selector.selectById"] = []Method{
		{Name: "Selector.selectById", ClassName: "Selector", ReturnType: "String", Params: []Param{{Name: "ids", Type: "Set<Id>"}, {Name: "fieldSets", Type: "List<Schema.FieldSet>"}}},
		{Name: "Selector.selectById", ClassName: "Selector", ReturnType: "String", Params: []Param{{Name: "ids", Type: "Set<Id>"}, {Name: "fields", Type: "Set<String>"}}},
	}
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"StubProvider"},
		Methods: map[string]Method{
			"handleMethodCall": {
				Name:       "Provider.handleMethodCall",
				ClassName:  "Provider",
				ReturnType: "Object",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateStubReturnsReceiverForUnstubbedFluentSelfMethod(t *testing.T) {
	providerProgram, err := CompileAnonymous("return null;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Fluent proxy = (Fluent)Test.createStub(Fluent.class, new Provider());
System.assertEquals(proxy, proxy.disableSecurity().allOrNothing(false));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "Fluent",
		Methods: map[string]Method{
			"disableSecurity": {Name: "Fluent.disableSecurity", ClassName: "Fluent", ReturnType: "Fluent"},
			"allOrNothing": {
				Name:       "Fluent.allOrNothing",
				ClassName:  "Fluent",
				ReturnType: "Fluent",
				Params:     []Param{{Name: "all", Type: "Boolean"}},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"StubProvider"},
		Methods: map[string]Method{
			"handleMethodCall": {
				Name:       "Provider.handleMethodCall",
				ClassName:  "Provider",
				ReturnType: "Object",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateStubReturnsNullForUnstubbedCollectionMethod(t *testing.T) {
	providerProgram, err := CompileAnonymous("return null;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Collector proxy = (Collector)Test.createStub(Collector.class, new Provider());
System.assertEquals(null, proxy.results());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "Collector",
		Methods: map[string]Method{
			"results": {Name: "Collector.results", ClassName: "Collector", ReturnType: "List<String>"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"StubProvider"},
		Methods: map[string]Method{
			"handleMethodCall": {
				Name:       "Provider.handleMethodCall",
				ClassName:  "Provider",
				ReturnType: "Object",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateStubReportsResolvedMethodNameCasing(t *testing.T) {
	providerProgram, err := CompileAnonymous(`
System.assertEquals('getPaymentLinesByIds', stubbedMethodName);
return 'handled';
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Gateway proxy = (Gateway)Test.createStub(Gateway.class, new Provider());
System.assertEquals('handled', proxy.getPaymentLinesByIDs(new Set<Id>()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "Gateway",
		Methods: map[string]Method{
			"getPaymentLinesByIds": {
				Name:       "Gateway.getPaymentLinesByIds",
				ClassName:  "Gateway",
				ReturnType: "String",
				Params:     []Param{{Name: "ids", Type: "Set<Id>"}},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"StubProvider"},
		Methods: map[string]Method{
			"handleMethodCall": {
				Name:       "Provider.handleMethodCall",
				ClassName:  "Provider",
				ReturnType: "Object",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateStubDispatchesThroughStaticField(t *testing.T) {
	providerProgram, err := CompileAnonymous(`
Count = Count + 1;
System.assertEquals('fetch', stubbedMethodName);
return 'handled';
`)
	if err != nil {
		t.Fatal(err)
	}
	controllerProgram, err := CompileAnonymous(`return service.fetch(recordId);`)
	if err != nil {
		t.Fatal(err)
	}
	realProgram, err := CompileAnonymous(`return 'real';`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Provider provider = new Provider();
Controller.service = (Service)Test.createStub(Service.class, provider);
System.assertEquals('handled', Controller.fetch('003000000000001'));
System.assertEquals(1, provider.Count);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "Service",
		Methods: map[string]Method{
			"fetch": {
				Name:       "Service.fetch",
				ClassName:  "Service",
				ReturnType: "String",
				Params:     []Param{{Name: "recordId", Type: "Id"}},
				Program:    realProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Controller",
		StaticFields: map[string]Field{
			"service": {Name: "service", Type: "Service", Static: true},
		},
		Methods: map[string]Method{
			"fetch": {
				Name:       "Controller.fetch",
				ClassName:  "Controller",
				ReturnType: "String",
				IsStatic:   true,
				Params:     []Param{{Name: "recordId", Type: "Id"}},
				Program:    controllerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"StubProvider"},
		Fields: map[string]Field{
			"Count": {Name: "Count", Type: "Integer", InitialValue: Int(0)},
		},
		Methods: map[string]Method{
			"handleMethodCall": {
				Name:       "Provider.handleMethodCall",
				ClassName:  "Provider",
				ReturnType: "Object",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateStubDispatchesThroughStaticFieldFromWrapperProvider(t *testing.T) {
	constructorProgram, err := CompileAnonymous(`this.stub = Test.createStub(classType, this);`)
	if err != nil {
		t.Fatal(err)
	}
	forTypeProgram, err := CompileAnonymous(`return new Mock(type);`)
	if err != nil {
		t.Fatal(err)
	}
	providerProgram, err := CompileAnonymous(`
Count = Count + 1;
return 'handled';
`)
	if err != nil {
		t.Fatal(err)
	}
	controllerProgram, err := CompileAnonymous(`return service.fetch(recordId);`)
	if err != nil {
		t.Fatal(err)
	}
	realProgram, err := CompileAnonymous(`return 'real';`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Mock mock = Mock.forType(Service.class);
Controller.service = (Service)mock.stub;
System.assertEquals('handled', Controller.fetch('003000000000001'));
System.assertEquals(1, mock.Count);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "Service",
		Methods: map[string]Method{
			"fetch": {
				Name:       "Service.fetch",
				ClassName:  "Service",
				ReturnType: "String",
				Params:     []Param{{Name: "recordId", Type: "Id"}},
				Program:    realProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Controller",
		StaticFields: map[string]Field{
			"service": {Name: "service", Type: "Service", Static: true},
		},
		Methods: map[string]Method{
			"fetch": {
				Name:       "Controller.fetch",
				ClassName:  "Controller",
				ReturnType: "String",
				IsStatic:   true,
				Params:     []Param{{Name: "recordId", Type: "Id"}},
				Program:    controllerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Mock",
		Interfaces: []string{"StubProvider"},
		Constructors: []Method{{
			Name:          "Mock.<init>",
			ClassName:     "Mock",
			Params:        []Param{{Name: "classType", Type: "Type"}},
			Program:       constructorProgram,
			IsConstructor: true,
		}},
		Fields: map[string]Field{
			"stub":  {Name: "stub", Type: "Object"},
			"Count": {Name: "Count", Type: "Integer", InitialValue: Int(0)},
		},
		Methods: map[string]Method{
			"forType": {
				Name:       "Mock.forType",
				ClassName:  "Mock",
				ReturnType: "Mock",
				IsStatic:   true,
				Params:     []Param{{Name: "type", Type: "Type"}},
				Program:    forTypeProgram,
			},
			"handleMethodCall": {
				Name:       "Mock.handleMethodCall",
				ClassName:  "Mock",
				ReturnType: "Object",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecExactTypeStaticCallWinsOverCaseFoldedLocal(t *testing.T) {
	createProgram, err := CompileAnonymous(`return 'made';`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Service service = null;
System.assertEquals('made', Service.create());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Service",
		Methods: map[string]Method{
			"create": {
				Name:       "Service.create",
				ClassName:  "Service",
				ReturnType: "String",
				IsStatic:   true,
				Program:    createProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUnqualifiedStaticMethodWinsBeforeInheritedArityFallback(t *testing.T) {
	runProgram, err := CompileAnonymous(`
Set<Id> ids = new Set<Id>{ '001000000000001AAA' };
execute('001000000000002AAA', ids);
`)
	if err != nil {
		t.Fatal(err)
	}
	staticProgram, err := CompileAnonymous(`Called = 'static';`)
	if err != nil {
		t.Fatal(err)
	}
	inheritedProgram, err := CompileAnonymous(`Called = 'inherited';`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
new Child().run();
System.assertEquals('static', Child.Called);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Base",
		Methods: map[string]Method{
			"execute": {
				Name:      "Base.execute",
				ClassName: "Base",
				Params: []Param{
					{Name: "context", Type: "Database.BatchableContext"},
					{Name: "scope", Type: "List<Object>"},
				},
				Program: inheritedProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Child",
		SuperClass: "Base",
		StaticFields: map[string]Field{
			"Called": {Name: "Called", Type: "String", Static: true},
		},
		Methods: map[string]Method{
			"run": {
				Name:      "Child.run",
				ClassName: "Child",
				Program:   runProgram,
			},
			"execute": {
				Name:      "Child.execute",
				ClassName: "Child",
				IsStatic:  true,
				Params: []Param{
					{Name: "recordId", Type: "Id"},
					{Name: "ids", Type: "Set<Id>"},
				},
				Program: staticProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateStubSObjectGetFallsThroughWithoutMethodMetadata(t *testing.T) {
	program, err := CompileAnonymous(`
Order orderStub = (Order)Test.createStub(Order.class, new Provider());
orderStub.put('Status', 'Draft');
System.assertEquals('Draft', orderStub.get('Status'));
`)
	if err != nil {
		t.Fatal(err)
	}
	providerProgram, err := CompileAnonymous(`return null;`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Order")
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{Name: "Order"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"StubProvider"},
		Methods: map[string]Method{
			"handleMethodCall": {
				Name:      "Provider.handleMethodCall",
				ClassName: "Provider",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				ReturnType: "Object",
				Program:    providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateStubDynamicMethodWithoutMetadataCallsProvider(t *testing.T) {
	program, err := CompileAnonymous(`
Order orderStub = (Order)Test.createStub(Order.class, new Provider());
System.assertEquals('handled', orderStub.get());
`)
	if err != nil {
		t.Fatal(err)
	}
	providerProgram, err := CompileAnonymous(`return 'handled';`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Order")
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{Name: "Order"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"StubProvider"},
		Methods: map[string]Method{
			"handleMethodCall": {
				Name:      "Provider.handleMethodCall",
				ClassName: "Provider",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				ReturnType: "Object",
				Program:    providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateStubVerifyModeUsesCurrentProviderState(t *testing.T) {
	providerProgram, err := CompileAnonymous(`
if (Verifying) {
  return null;
}
Count = Count + 1;
return null;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Provider mocks = new Provider();
Service proxy = (Service)Test.createStub(Service.class, mocks);
proxy.send();
mocks.Verifying = true;
((IService)proxy).send();
mocks.Verifying = false;
proxy.send();
System.assertEquals(2, mocks.Count);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name:       "IService",
		Interfaces: []string{},
		Methods: map[string]Method{
			"send": {Name: "IService.send", ClassName: "IService", ReturnType: "void"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Service",
		Interfaces: []string{"IService"},
		Methods: map[string]Method{
			"send": {Name: "Service.send", ClassName: "Service", ReturnType: "void"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"StubProvider"},
		Fields: map[string]Field{
			"Count":     {Name: "Count", Type: "Integer", InitialValue: Int(0)},
			"Verifying": {Name: "Verifying", Type: "Boolean", InitialValue: Bool(false)},
		},
		Methods: map[string]Method{
			"handleMethodCall": {
				Name:       "Provider.handleMethodCall",
				ClassName:  "Provider",
				ReturnType: "Object",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentStubProviderPrefersActiveFrameworkApexMocksProvider(t *testing.T) {
	machine := New(nil)
	attached := Object("framework_ApexMocks")
	attached.Fields["methodReturnValueRecorder"] = Object("framework_MethodReturnValueRecorder")
	proxy := Object("IService__sfdc_ApexStub")
	proxy.Fields["__gladeStubProvider"] = attached

	live := Object("framework_ApexMocks")
	live.Fields["verifying"] = Bool(true)
	live.Fields["methodReturnValueRecorder"] = Object("framework_MethodReturnValueRecorder")
	machine.Globals["mocks"] = live

	provider := machine.currentStubProvider(proxy)
	if provider.Ref != live.Ref {
		t.Fatalf("expected active framework_ApexMocks provider ref %d, got %d", live.Ref, provider.Ref)
	}
}

func TestCurrentStubProviderKeepsReceiverProviderWhenAnotherApexMocksProviderIsVerifying(t *testing.T) {
	machine := New(nil)
	lineProvider := Object("fflib_ApexMocks")
	lineProvider.Fields["methodReturnValueRecorder"] = Object("fflib_MethodReturnValueRecorder")
	lineProxy := Object("PaymentLineBase__sfdc_ApexStub")
	lineProxy.Fields["__gladeStubProvider"] = lineProvider
	machine.Globals["line"] = lineProxy

	verifyingProvider := Object("fflib_ApexMocks")
	verifyingProvider.Fields["Verifying"] = Bool(true)
	verifyingProvider.Fields["methodReturnValueRecorder"] = Object("fflib_MethodReturnValueRecorder")
	machine.Globals["mocks"] = verifyingProvider

	provider := machine.currentStubProvider(lineProxy)
	if provider.Ref != lineProvider.Ref {
		t.Fatalf("expected receiver ApexMocks provider ref %d, got %d", lineProvider.Ref, provider.Ref)
	}
}

func TestCurrentStubProviderPrefersUniqueLiveFrameworkApexMocksProviderOverSnapshot(t *testing.T) {
	machine := New(nil)
	attached := Object("framework_ApexMocks")
	attachedRecorder := Object("framework_MethodReturnValueRecorder")
	attachedRecorder.Fields["Stubbing"] = Bool(true)
	attached.Fields["methodReturnValueRecorder"] = attachedRecorder
	proxy := Object("IService__sfdc_ApexStub")
	proxy.Fields["__gladeStubProvider"] = attached

	live := Object("framework_ApexMocks")
	liveRecorder := Object("framework_MethodReturnValueRecorder")
	liveRecorder.Fields["Stubbing"] = Bool(false)
	live.Fields["methodReturnValueRecorder"] = liveRecorder
	machine.Globals["mocks"] = live

	provider := machine.currentStubProvider(proxy)
	if provider.Ref != live.Ref {
		t.Fatalf("expected live framework_ApexMocks provider ref %d, got %d", live.Ref, provider.Ref)
	}
}

func TestPropagateUpdatedValueAliasesKeepsCompleteApexMocksProvider(t *testing.T) {
	machine := New(nil)
	live := Object("fflib_ApexMocks")
	live.Fields["methodVerifier"] = Object("fflib_AnyOrder")
	live.Fields["methodReturnValueRecorder"] = Object("fflib_MethodReturnValueRecorder")
	machine.Globals["mocks"] = live

	snapshot := Object("fflib_ApexMocks")
	snapshot.Ref = live.Ref
	proxy := Object("ExchangeRatesApiV1Service")
	proxy.Fields["__gladeStubProvider"] = snapshot
	updated := typedList("List<Object>")
	updated.List = append(updated.List, proxy)

	machine.propagateUpdatedValueAliases(machine.Globals, updated)
	mocks := machine.Globals["mocks"]
	if _, verifier, ok := objectFieldValue(mocks, "methodVerifier"); !ok || verifier.Kind != ValueObject {
		t.Fatalf("methodVerifier lost: %#v", mocks)
	}
	if _, recorder, ok := objectFieldValue(mocks, "methodReturnValueRecorder"); !ok || recorder.Kind != ValueObject {
		t.Fatalf("methodReturnValueRecorder lost: %#v", mocks)
	}
}

func TestFrameworkQualifiedMethodMapKeyNormalizesFflibTypes(t *testing.T) {
	machine := New(nil)
	left := Object("fflib_QualifiedMethod")
	left.Fields["typeName"] = String("PricingManagerFacade__sfdc_ApexStub")
	left.Fields["methodName"] = String("price")
	left.Fields["methodArgTypes"] = typedList("List<Type>")

	right := Object("framework_QualifiedMethod")
	right.Fields["typeName"] = String("PricingManagerFacade__sfdc_ApexStub")
	right.Fields["methodName"] = String("price")
	right.Fields["methodArgTypes"] = typedList("List<Type>")

	if machine.mapKey(left) != machine.mapKey(right) {
		t.Fatalf("expected fflib and framework qualified method keys to match")
	}
	equal, err := machine.mapKeysEqual(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if !equal {
		t.Fatalf("expected fflib and framework qualified method keys to compare equal")
	}
}

func TestManagedSuperclassConstructorCallIsPassiveWhenSuperclassIsExternal(t *testing.T) {
	ctorProgram, err := CompileAnonymous("super('related');")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("new Plugin();")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "Plugin",
		SuperClass: "pkg.ExternalBase",
		Constructors: []Method{{
			Name:          "Plugin.<init>",
			ClassName:     "Plugin",
			ReturnType:    "void",
			IsConstructor: true,
			Program:       ctorProgram,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestFrameworkMatcherFastPathMatchesCommonMatchers(t *testing.T) {
	machine := New(nil)
	methodArg := Object("framework_MethodArgValues")
	methodArg.Fields["argValues"] = List(String("Ada"), Null, Object("Account"))
	eq := Object("framework_MatcherDefinitions.Eq")
	eq.Fields["toMatch"] = String("Ada")
	isNull := Object("framework_MatcherDefinitions.IsNull")
	anySObject := Object("framework_MatcherDefinitions.AnySObject")

	matched, handled, err := machine.frameworkMatchesAllArgs(methodArg, List(eq, isNull, anySObject))
	if err != nil {
		t.Fatal(err)
	}
	if !handled || !matched {
		t.Fatalf("matched=%v handled=%v, want true true", matched, handled)
	}

	anyString := Object("framework_MatcherDefinitions.AnyString")
	matched, handled, err = machine.frameworkMatchesAllArgs(methodArg, List(anyString, isNull, anySObject))
	if err != nil {
		t.Fatal(err)
	}
	if !handled || !matched {
		t.Fatalf("matched=%v handled=%v for AnyString, want true true", matched, handled)
	}

	methodArg.Fields["argValues"] = List(platformScalar("Id", "a5B000000000001AAA"))
	matched, handled, err = machine.frameworkMatchesAllArgs(methodArg, List(anyString))
	if err != nil {
		t.Fatal(err)
	}
	if !handled || !matched {
		t.Fatalf("matched=%v handled=%v for AnyString Id argument, want true true", matched, handled)
	}

	expectedAccount := Object("Account")
	expectedAccount.Fields["Name"] = String("Acme")
	actualAccount := expectedAccount
	actualAccount.Ref = newValueRef()
	eqSObject := Object("framework_MatcherDefinitions.Eq")
	eqSObject.Fields["toMatch"] = expectedAccount
	methodArg.Fields["argValues"] = List(actualAccount)
	matched, handled, err = machine.frameworkMatchesAllArgs(methodArg, List(eqSObject))
	if err != nil {
		t.Fatal(err)
	}
	if !handled || !matched {
		t.Fatalf("matched=%v handled=%v for SObject Eq, want true true", matched, handled)
	}

	expectedDTO := Object("CredentialingObjectMapping.CredentialingItemType")
	expectedDTO.Fields["internalName"] = String("Custom")
	actualDTO := Object("CredentialingObjectMapping.CredentialingItemType")
	actualDTO.Fields["internalName"] = String("Custom")
	eqDTO := Object("framework_MatcherDefinitions.Eq")
	eqDTO.Fields["toMatch"] = expectedDTO
	methodArg.Fields["argValues"] = List(actualDTO)
	matched, handled, err = machine.frameworkMatchesAllArgs(methodArg, List(eqDTO))
	if err != nil {
		t.Fatal(err)
	}
	if !handled || !matched {
		t.Fatalf("matched=%v handled=%v for DTO Eq, want true true", matched, handled)
	}
}

func TestFrameworkRecordMethodInvocationInitializesNilStaticMap(t *testing.T) {
	machine := New(nil)
	machine.Classes["fflib_MethodCountRecorder"] = Class{
		Name: "fflib_MethodCountRecorder",
		StaticFields: map[string]Field{
			"methodArgumentsByTypeName": {
				Name:  "methodArgumentsByTypeName",
				Type:  "Map<fflib_QualifiedMethod,List<fflib_MethodArgValues>>",
				Value: Value{Kind: ValueMap, Type: "Map<fflib_QualifiedMethod,List<fflib_MethodArgValues>>"},
			},
		},
	}
	invocation := Object("fflib_InvocationOnMock")
	qm := Object("fflib_QualifiedMethod")
	qm.Fields["typeName"] = String("ExchangeRatesApiV1Service")
	qm.Fields["methodName"] = String("updateExchangeRates")
	qm.Fields["methodArgTypes"] = List()
	invocation.Fields["qm"] = qm
	invocation.Fields["methodArg"] = Object("fflib_MethodArgValues")
	if err := machine.frameworkRecordMethodInvocation(invocation); err != nil {
		t.Fatal(err)
	}
	value, ok := machine.frameworkMethodCountRecorderStatic("methodArgumentsByTypeName")
	if !ok || value.Kind != ValueMap || len(value.Map) != 1 {
		t.Fatalf("methodArgumentsByTypeName = %#v", value)
	}
}

func TestAssignInstanceFieldInitializesNilFieldsMap(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:   "Thing",
		Fields: map[string]Field{"Name": {Name: "Name", Type: "String"}},
	}); err != nil {
		t.Fatal(err)
	}
	machine.Globals["this"] = Value{Kind: ValueObject, Type: "Thing"}
	if err := machine.assign("Name", String("Acme")); err != nil {
		t.Fatal(err)
	}
	this := machine.Globals["this"]
	if this.Fields == nil || this.Fields["Name"].Text != "Acme" {
		t.Fatalf("this fields = %#v", this.Fields)
	}
}

func TestExecNamespacedConstructorAssignsSubclassToSuperField(t *testing.T) {
	constructorProgram, err := CompileAnonymous(`this.methodVerifier = new fflib_AnyOrder();`)
	if err != nil {
		t.Fatal(err)
	}
	checkProgram, err := CompileAnonymous(`System.assert(this.methodVerifier != null);`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
fflib_ApexMocks mocks = new fflib_ApexMocks();
mocks.check();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Namespace = "NU"
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name:       "fflib_MethodVerifier",
		Namespace:  "NU",
		IsAbstract: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "fflib_AnyOrder",
		Namespace:  "NU",
		SuperClass: "fflib_MethodVerifier",
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:      "fflib_ApexMocks",
		Namespace: "NU",
		Access:    "global",
		Fields: map[string]Field{
			"methodVerifier": {Name: "methodVerifier", Type: "fflib_MethodVerifier"},
		},
		Constructors: []Method{{Program: constructorProgram}},
		Methods: map[string]Method{
			"check": {ReturnType: "void", Program: checkProgram, Access: "global"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestFrameworkArgumentCaptorMatcherStoresMatchedArgument(t *testing.T) {
	machine := New(nil)
	record := Object("Credentialing_Item__c")
	record.Fields["Id"] = platformScalar("Id", "a5B000000000001")
	records := List(record)
	records.Type = "List<SObject>"

	methodArg := Object("framework_MethodArgValues")
	methodArg.Fields["argValues"] = List(records)
	captorMatcher := Object("framework_ArgumentCaptor.AnyObject")

	matched, handled, err := machine.frameworkMatchesAllArgs(methodArg, List(captorMatcher))
	if err != nil {
		t.Fatal(err)
	}
	if !handled || !matched {
		t.Fatalf("matched=%v handled=%v, want true true", matched, handled)
	}
	captured := captorMatcher.Fields["value"]
	if captured.Kind != ValueList || len(captured.List) != 1 {
		t.Fatalf("captured = %#v, want single-record list", captured)
	}
	if got := sObjectIDValue(captured.List[0]); got.String() != "a5B000000000001EAA" {
		t.Fatalf("captured record id = %v, want a5B000000000001EAA", got)
	}
}

func TestFrameworkSObjectUnitOfWorkHandleRegisterTypeFastPath(t *testing.T) {
	machine := New(nil)
	uow := Object("framework_SObjectUnitOfWork")
	for _, field := range []string{
		"m_newListByType",
		"m_dirtyMapByType",
		"m_deletedMapByType",
		"m_emptyRecycleBinMapByType",
		"m_relationships",
		"m_publishBeforeListByType",
		"m_publishAfterSuccessListByType",
		"m_publishAfterFailureListByType",
	} {
		uow.Fields[field] = Map()
	}

	_, handled, err := machine.callFrameworkSObjectUnitOfWorkMember(uow, "handleRegisterType", []Value{sObjectTypeToken("Account")}, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("handleRegisterType was not handled")
	}
	if got := uow.Fields["m_newListByType"].Map[mapKey(String("Account"))]; got.Kind != ValueList {
		t.Fatalf("new list kind = %v, want list", got.Kind)
	}
	if got := uow.Fields["m_dirtyMapByType"].Map[mapKey(String("Account"))]; got.Kind != ValueMap {
		t.Fatalf("dirty map kind = %v, want map", got.Kind)
	}
	if got := uow.Fields["m_relationships"].Map[mapKey(String("Account"))]; got.Type != "framework_SObjectUnitOfWork.Relationships" {
		t.Fatalf("relationships type = %q", got.Type)
	}
}

func TestExecUserRelationshipsClassDispatchesAddMethod(t *testing.T) {
	addProgram, err := CompileAnonymous("this.touched = true;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
UnitOfWork.Relationships relations = new UnitOfWork.Relationships();
relations.add(new Account());
System.assertEquals(true, relations.touched);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "UnitOfWork"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "UnitOfWork.Relationships",
		Fields: map[string]Field{
			"touched": {Name: "touched", Type: "Boolean", InitialValue: Bool(false)},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:      "UnitOfWork.Relationships.add",
		ClassName: "UnitOfWork.Relationships",
		Params:    []Param{{Name: "record", Type: "Object"}},
		Program:   addProgram,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestFrameworkSObjectUnitOfWorkSendEmailWorkFastPath(t *testing.T) {
	machine := New(nil)
	emailWork := Object("framework_SObjectUnitOfWork.SendEmailWork")
	emailWork.Fields["emails"] = typedList("List<Messaging.Email>")
	email := Object("Messaging.SingleEmailMessage")
	email.Fields["plainTextBody"] = String("body")

	if _, handled, err := machine.callFrameworkSObjectUnitOfWorkMember(emailWork, "registerEmail", []Value{email}, &Result{}); err != nil {
		t.Fatal(err)
	} else if !handled {
		t.Fatal("registerEmail was not handled")
	}
	if got := emailWork.Fields["emails"]; got.Kind != ValueList || len(got.List) != 1 {
		t.Fatalf("emails = %#v, want one registered email", got)
	}
	if _, handled, err := machine.callFrameworkSObjectUnitOfWorkMember(emailWork, "doWork", nil, &Result{}); err != nil {
		t.Fatal(err)
	} else if !handled {
		t.Fatal("doWork was not handled")
	}
	if machine.limits.EmailInvokes != 1 {
		t.Fatalf("email invocations = %d, want 1", machine.limits.EmailInvokes)
	}
}

func TestManagedTriggerHandlerManagerStaticTogglesAreNoops(t *testing.T) {
	machine := New(nil)
	for _, method := range []string{"disableTriggerStep", "enableTriggerStep", "reenableTriggerForThisRequest"} {
		if _, handled, err := machine.callFrameworkStaticMember("znu.TriggerHandlerManager", method, []Value{String("StepName")}); err != nil {
			t.Fatal(err)
		} else if !handled {
			t.Fatalf("%s was not handled", method)
		}
	}

	if err := machine.RegisterClass(Class{Name: "TriggerHandlerManager"}); err != nil {
		t.Fatal(err)
	}
	if _, handled, err := machine.callFrameworkStaticMember("TriggerHandlerManager", "disableTriggerForThisRequest", []Value{String("StepName")}); err != nil {
		t.Fatal(err)
	} else if handled {
		t.Fatal("real TriggerHandlerManager class should not be handled by managed fallback")
	}
}

func TestFrameworkQueryFactoryOrderingToSOQLFastPath(t *testing.T) {
	machine := New(nil)
	ordering := Object("framework_QueryFactory.Ordering")
	ordering.Fields["field"] = String("Name")
	ordering.Fields["direction"] = Value{Kind: ValueObject, Type: "framework_QueryFactory.SortOrder", Text: "ASCENDING"}
	ordering.Fields["nullsLast"] = Bool(true)

	value, handled, err := machine.callFrameworkQueryFactoryMember(ordering, "toSOQL", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("toSOQL was not handled")
	}
	if value.String() != "Name ASC NULLS LAST " {
		t.Fatalf("toSOQL = %q", value.String())
	}
}

func TestRuntimeStaticLookupIgnoresAmbiguousInstanceOverloads(t *testing.T) {
	program, err := CompileAnonymous(`
Selector selector = new Selector();
Object placeholder = null;
System.assertEquals('fieldsets', selector.selectById((Set<Id>)placeholder, (List<Schema.FieldSet>)placeholder));
`)
	if err != nil {
		t.Fatal(err)
	}
	fieldsetsProgram, err := CompileAnonymous("return 'fieldsets';")
	if err != nil {
		t.Fatal(err)
	}
	stringsProgram, err := CompileAnonymous("return 'strings';")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Selector"}); err != nil {
		t.Fatal(err)
	}
	machine.MethodOverloads["Selector.selectById"] = []Method{
		{Name: "Selector.selectById", ClassName: "Selector", ReturnType: "String", Params: []Param{{Name: "ids", Type: "Set<Id>"}, {Name: "fieldSets", Type: "List<Schema.FieldSet>"}}, Program: fieldsetsProgram},
		{Name: "Selector.selectById", ClassName: "Selector", ReturnType: "String", Params: []Param{{Name: "ids", Type: "Set<Id>"}, {Name: "fields", Type: "Set<String>"}}, Program: stringsProgram},
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDottedStaticMethodRunsStaticInitializer(t *testing.T) {
	staticInit, err := CompileAnonymous("seed = 4;")
	if err != nil {
		t.Fatal(err)
	}
	getProgram, err := CompileAnonymous("return seed;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals(4, Counter.get());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Counter",
		StaticFields: map[string]Field{
			"seed": {Name: "seed", Type: "Integer", Static: true},
		},
		StaticInitializers: []Method{{
			Name:      "Counter.<static_init>",
			ClassName: "Counter",
			Program:   staticInit,
			IsStatic:  true,
		}},
		Methods: map[string]Method{
			"get": {Name: "Counter.get", ClassName: "Counter", ReturnType: "Integer", IsStatic: true, Program: getProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecConstructorThisChaining(t *testing.T) {
	defaultCtor, err := CompileAnonymous("this(2);")
	if err != nil {
		t.Fatal(err)
	}
	seedCtor, err := CompileAnonymous("value = seed;")
	if err != nil {
		t.Fatal(err)
	}
	scoreProgram, err := CompileAnonymous("return value;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Counter c = new Counter();
System.assertEquals(2, c.score());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Counter",
		Fields: map[string]Field{
			"value": {Name: "value", Type: "Integer"},
		},
		Constructors: []Method{
			{Name: "Counter.<init>", ClassName: "Counter", Program: defaultCtor, IsConstructor: true},
			{Name: "Counter.<init>", ClassName: "Counter", Params: []Param{{Name: "seed", Type: "Integer"}}, Program: seedCtor, IsConstructor: true},
		},
		Methods: map[string]Method{
			"score": {Name: "Counter.score", ClassName: "Counter", ReturnType: "Integer", Program: scoreProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecConstructorSuperChaining(t *testing.T) {
	parentCtor, err := CompileAnonymous("base = seed;")
	if err != nil {
		t.Fatal(err)
	}
	childCtor, err := CompileAnonymous("super(3); bonus = 4;")
	if err != nil {
		t.Fatal(err)
	}
	scoreProgram, err := CompileAnonymous("return base + bonus;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Child c = new Child();
System.assertEquals(7, c.score());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Parent",
		Fields: map[string]Field{
			"base": {Name: "base", Type: "Integer"},
		},
		Constructors: []Method{{
			Name:          "Parent.<init>",
			ClassName:     "Parent",
			Params:        []Param{{Name: "seed", Type: "Integer"}},
			Program:       parentCtor,
			IsConstructor: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Child",
		SuperClass: "Parent",
		Fields: map[string]Field{
			"bonus": {Name: "bonus", Type: "Integer"},
		},
		Constructors: []Method{{
			Name:          "Child.<init>",
			ClassName:     "Child",
			Program:       childCtor,
			IsConstructor: true,
		}},
		Methods: map[string]Method{
			"score": {Name: "Child.score", ClassName: "Child", ReturnType: "Integer", Program: scoreProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSuperConstructorOverloadUsesDeclaredArgumentType(t *testing.T) {
	objectCtor, err := CompileAnonymous("kind = 'object';")
	if err != nil {
		t.Fatal(err)
	}
	stringCtor, err := CompileAnonymous("kind = 'string';")
	if err != nil {
		t.Fatal(err)
	}
	childCtor, err := CompileAnonymous("super(parameter);")
	if err != nil {
		t.Fatal(err)
	}
	kindProgram, err := CompileAnonymous("return kind;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
ChildMessage message = new ChildMessage('Pay Later - Bill Me');
System.assertEquals('object', message.kindValue());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "BaseMessage",
		Fields: map[string]Field{
			"kind": {Name: "kind", Type: "String"},
		},
		Constructors: []Method{
			{Name: "BaseMessage.<init>", ClassName: "BaseMessage", Params: []Param{{Name: "parameter", Type: "Object"}}, Program: objectCtor, IsConstructor: true},
			{Name: "BaseMessage.<init>", ClassName: "BaseMessage", Params: []Param{{Name: "eventName", Type: "String"}}, Program: stringCtor, IsConstructor: true},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "ChildMessage",
		SuperClass: "BaseMessage",
		Constructors: []Method{{
			Name:          "ChildMessage.<init>",
			ClassName:     "ChildMessage",
			Params:        []Param{{Name: "parameter", Type: "Object"}},
			Program:       childCtor,
			IsConstructor: true,
		}},
		Methods: map[string]Method{
			"kindvalue": {Name: "ChildMessage.kindValue", ClassName: "ChildMessage", ReturnType: "String", Program: kindProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecConstructorCanReadInheritedFieldAfterSuper(t *testing.T) {
	parentCtor, err := CompileAnonymous("base = seed;")
	if err != nil {
		t.Fatal(err)
	}
	childCtor, err := CompileAnonymous("super(3); bonus = this.base + 4;")
	if err != nil {
		t.Fatal(err)
	}
	scoreProgram, err := CompileAnonymous("return bonus;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Child c = new Child();
System.assertEquals(7, c.score());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Parent",
		Fields: map[string]Field{
			"base": {Name: "base", Type: "Integer"},
		},
		Constructors: []Method{{
			Name:          "Parent.<init>",
			ClassName:     "Parent",
			Params:        []Param{{Name: "seed", Type: "Integer"}},
			Program:       parentCtor,
			IsConstructor: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Child",
		SuperClass: "Parent",
		Fields: map[string]Field{
			"bonus": {Name: "bonus", Type: "Integer"},
		},
		Constructors: []Method{{
			Name:          "Child.<init>",
			ClassName:     "Child",
			Program:       childCtor,
			IsConstructor: true,
		}},
		Methods: map[string]Method{
			"score": {Name: "Child.score", ClassName: "Child", ReturnType: "Integer", Program: scoreProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecConstructorCanReadInheritedPropertyAfterSuper(t *testing.T) {
	parentCtor, err := CompileAnonymous("base = seed;")
	if err != nil {
		t.Fatal(err)
	}
	childCtor, err := CompileAnonymous("super(3); bonus = this.base + 4;")
	if err != nil {
		t.Fatal(err)
	}
	scoreProgram, err := CompileAnonymous("return bonus;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Child c = new Child();
System.assertEquals(7, c.score());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Parent",
		Fields: map[string]Field{
			"base": {Name: "base", Type: "Integer", Property: true},
		},
		Constructors: []Method{{
			Name:          "Parent.<init>",
			ClassName:     "Parent",
			Params:        []Param{{Name: "seed", Type: "Integer"}},
			Program:       parentCtor,
			IsConstructor: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Child",
		SuperClass: "Parent",
		Fields: map[string]Field{
			"bonus": {Name: "bonus", Type: "Integer"},
		},
		Constructors: []Method{{
			Name:          "Child.<init>",
			ClassName:     "Child",
			Program:       childCtor,
			IsConstructor: true,
		}},
		Methods: map[string]Method{
			"score": {Name: "Child.score", ClassName: "Child", ReturnType: "Integer", Program: scoreProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecExplicitEmptyConstructorCallsImplicitSuper(t *testing.T) {
	parentCtor, err := CompileAnonymous("values = new Map<String, Object>();")
	if err != nil {
		t.Fatal(err)
	}
	childCtor, err := CompileAnonymous("")
	if err != nil {
		t.Fatal(err)
	}
	checkProgram, err := CompileAnonymous(`
Child c = new Child();
c.add('name', 'Ada');
System.assertEquals('Ada', c.get('name'));
`)
	if err != nil {
		t.Fatal(err)
	}
	addProgram, err := CompileAnonymous("values.put(key, value);")
	if err != nil {
		t.Fatal(err)
	}
	getProgram, err := CompileAnonymous("return values.get(key);")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Parent",
		Fields: map[string]Field{
			"values": {Name: "values", Type: "Map<String,Object>"},
		},
		Constructors: []Method{{
			Name:          "Parent.<init>",
			ClassName:     "Parent",
			Program:       parentCtor,
			IsConstructor: true,
		}},
		Methods: map[string]Method{
			"add": {Name: "Parent.add", ClassName: "Parent", Params: []Param{{Name: "key", Type: "String"}, {Name: "value", Type: "Object"}}, Program: addProgram},
			"get": {Name: "Parent.get", ClassName: "Parent", Params: []Param{{Name: "key", Type: "String"}}, ReturnType: "Object", Program: getProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Child",
		SuperClass: "Parent",
		Constructors: []Method{{
			Name:          "Child.<init>",
			ClassName:     "Child",
			Program:       childCtor,
			IsConstructor: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(checkProgram); err != nil {
		t.Fatal(err)
	}
}

func TestExecConstructorThisFieldAssignmentStillCallsImplicitSuper(t *testing.T) {
	parentCtor, err := CompileAnonymous("values = new Map<String, Object>();")
	if err != nil {
		t.Fatal(err)
	}
	childCtor, err := CompileAnonymous("this.label = 'Ada'; add('name', this.label);")
	if err != nil {
		t.Fatal(err)
	}
	checkProgram, err := CompileAnonymous(`
Child c = new Child();
System.assertEquals('Ada', c.get('name'));
`)
	if err != nil {
		t.Fatal(err)
	}
	addProgram, err := CompileAnonymous("values.put(key, value);")
	if err != nil {
		t.Fatal(err)
	}
	getProgram, err := CompileAnonymous("return values.get(key);")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Parent",
		Fields: map[string]Field{
			"values": {Name: "values", Type: "Map<String,Object>"},
		},
		Constructors: []Method{{
			Name:          "Parent.<init>",
			ClassName:     "Parent",
			Program:       parentCtor,
			IsConstructor: true,
		}},
		Methods: map[string]Method{
			"add": {Name: "Parent.add", ClassName: "Parent", Params: []Param{{Name: "key", Type: "String"}, {Name: "value", Type: "Object"}}, Program: addProgram},
			"get": {Name: "Parent.get", ClassName: "Parent", Params: []Param{{Name: "key", Type: "String"}}, ReturnType: "Object", Program: getProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Child",
		SuperClass: "Parent",
		Fields: map[string]Field{
			"label": {Name: "label", Type: "String"},
		},
		Constructors: []Method{{
			Name:          "Child.<init>",
			ClassName:     "Child",
			Program:       childCtor,
			IsConstructor: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(checkProgram); err != nil {
		t.Fatal(err)
	}
}

func TestExecConstructorThisPropertySetterStillCallsImplicitSuper(t *testing.T) {
	parentCtor, err := CompileAnonymous("values = new Map<String, Object>();")
	if err != nil {
		t.Fatal(err)
	}
	childCtor, err := CompileAnonymous("this.Name = 'Ada';")
	if err != nil {
		t.Fatal(err)
	}
	checkProgram, err := CompileAnonymous(`
Child c = new Child();
System.assertEquals('Ada', c.get('name'));
`)
	if err != nil {
		t.Fatal(err)
	}
	addProgram, err := CompileAnonymous("values.put(key, value);")
	if err != nil {
		t.Fatal(err)
	}
	getProgram, err := CompileAnonymous("return values.get(key);")
	if err != nil {
		t.Fatal(err)
	}
	setterProgram, err := CompileAnonymous("add('name', value);")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Parent",
		Fields: map[string]Field{
			"values": {Name: "values", Type: "Map<String,Object>"},
		},
		Constructors: []Method{{
			Name:          "Parent.<init>",
			ClassName:     "Parent",
			Program:       parentCtor,
			IsConstructor: true,
		}},
		Methods: map[string]Method{
			"add": {Name: "Parent.add", ClassName: "Parent", Params: []Param{{Name: "key", Type: "String"}, {Name: "value", Type: "Object"}}, Program: addProgram},
			"get": {Name: "Parent.get", ClassName: "Parent", Params: []Param{{Name: "key", Type: "String"}}, ReturnType: "Object", Program: getProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Child",
		SuperClass: "Parent",
		Fields: map[string]Field{
			"Name": {
				Name:   "Name",
				Type:   "String",
				Setter: &Method{Name: "Child.Name.set", ClassName: "Child", Params: []Param{{Name: "value", Type: "String"}}, Program: setterProgram},
			},
		},
		Constructors: []Method{{
			Name:          "Child.<init>",
			ClassName:     "Child",
			Program:       childCtor,
			IsConstructor: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(checkProgram); err != nil {
		t.Fatal(err)
	}
}

func TestExecImplicitSuperConstructorRunsFullAncestorChain(t *testing.T) {
	grandparentCtor, err := CompileAnonymous("values = new Map<String, Object>();")
	if err != nil {
		t.Fatal(err)
	}
	parentCtor, err := CompileAnonymous("")
	if err != nil {
		t.Fatal(err)
	}
	childCtor, err := CompileAnonymous("label = seed;")
	if err != nil {
		t.Fatal(err)
	}
	checkProgram, err := CompileAnonymous(`
Child c = new Child('Ada');
c.add('name', c.label);
System.assertEquals('Ada', c.get('name'));
`)
	if err != nil {
		t.Fatal(err)
	}
	addProgram, err := CompileAnonymous("values.put(key, value);")
	if err != nil {
		t.Fatal(err)
	}
	getProgram, err := CompileAnonymous("return values.get(key);")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Grandparent",
		Fields: map[string]Field{
			"values": {Name: "values", Type: "Map<String,Object>"},
		},
		Constructors: []Method{{
			Name:          "Grandparent.<init>",
			ClassName:     "Grandparent",
			Program:       grandparentCtor,
			IsConstructor: true,
		}},
		Methods: map[string]Method{
			"add": {Name: "Grandparent.add", ClassName: "Grandparent", Params: []Param{{Name: "key", Type: "String"}, {Name: "value", Type: "Object"}}, Program: addProgram},
			"get": {Name: "Grandparent.get", ClassName: "Grandparent", Params: []Param{{Name: "key", Type: "String"}}, ReturnType: "Object", Program: getProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Parent",
		SuperClass: "Grandparent",
		Constructors: []Method{{
			Name:          "Parent.<init>",
			ClassName:     "Parent",
			Program:       parentCtor,
			IsConstructor: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Child",
		SuperClass: "Parent",
		Fields: map[string]Field{
			"label": {Name: "label", Type: "String"},
		},
		Constructors: []Method{{
			Name:          "Child.<init>",
			ClassName:     "Child",
			Params:        []Param{{Name: "seed", Type: "String"}},
			Program:       childCtor,
			IsConstructor: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(checkProgram); err != nil {
		t.Fatal(err)
	}
}

func TestExecExplicitSuperConstructorRunsAncestorDefaultConstructor(t *testing.T) {
	grandparentCtor, err := CompileAnonymous("base = 2;")
	if err != nil {
		t.Fatal(err)
	}
	parentCtor, err := CompileAnonymous("middle = seed;")
	if err != nil {
		t.Fatal(err)
	}
	childCtor, err := CompileAnonymous("super(3); bonus = 4;")
	if err != nil {
		t.Fatal(err)
	}
	scoreProgram, err := CompileAnonymous("return base + middle + bonus;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Child c = new Child();
System.assertEquals(9, c.score());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Grandparent",
		Fields: map[string]Field{
			"base": {Name: "base", Type: "Integer"},
		},
		Constructors: []Method{{
			Name:          "Grandparent.<init>",
			ClassName:     "Grandparent",
			Program:       grandparentCtor,
			IsConstructor: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Parent",
		SuperClass: "Grandparent",
		Fields: map[string]Field{
			"middle": {Name: "middle", Type: "Integer"},
		},
		Constructors: []Method{{
			Name:          "Parent.<init>",
			ClassName:     "Parent",
			Params:        []Param{{Name: "seed", Type: "Integer"}},
			Program:       parentCtor,
			IsConstructor: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Child",
		SuperClass: "Parent",
		Fields: map[string]Field{
			"bonus": {Name: "bonus", Type: "Integer"},
		},
		Constructors: []Method{{
			Name:          "Child.<init>",
			ClassName:     "Child",
			Program:       childCtor,
			IsConstructor: true,
		}},
		Methods: map[string]Method{
			"score": {Name: "Child.score", ClassName: "Child", ReturnType: "Integer", Program: scoreProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCastControlsOverloadResolutionButPreservesRuntimeDispatch(t *testing.T) {
	childProgram, err := CompileAnonymous("return choose((Base)arg);")
	if err != nil {
		t.Fatal(err)
	}
	baseProgram, err := CompileAnonymous("return arg.kind();")
	if err != nil {
		t.Fatal(err)
	}
	kindProgram, err := CompileAnonymous("return 'child';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Chooser c = new Chooser();
System.assertEquals('child', c.choose(new Child()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Base",
		Methods: map[string]Method{
			"kind": {Name: "Base.kind", ClassName: "Base", ReturnType: "String", Modifiers: []string{"abstract"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Child",
		SuperClass: "Base",
		Methods: map[string]Method{
			"kind": {Name: "Child.kind", ClassName: "Child", ReturnType: "String", Program: kindProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Chooser",
		Methods: map[string]Method{
			"choose": {Name: "Chooser.choose", ClassName: "Chooser", Params: []Param{{Name: "arg", Type: "Child"}}, ReturnType: "String", Program: childProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	machine.MethodOverloads["Chooser.choose"] = []Method{
		{Name: "Chooser.choose", ClassName: "Chooser", Params: []Param{{Name: "arg", Type: "Child"}}, ReturnType: "String", Program: childProgram},
		{Name: "Chooser.choose", ClassName: "Chooser", Params: []Param{{Name: "arg", Type: "Base"}}, ReturnType: "String", Program: baseProgram},
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCastStaticTypeParticipatesInOverloadCacheKey(t *testing.T) {
	childProgram, err := CompileAnonymous("this.register((Base)left, (Base)right);")
	if err != nil {
		t.Fatal(err)
	}
	baseProgram, err := CompileAnonymous("called = 'base';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Registrar registrar = new Registrar();
Child child = new Child();
registrar.register(child, child);
System.assertEquals('base', registrar.called);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Base"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Child", SuperClass: "Base"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Registrar",
		Fields: map[string]Field{
			"called": {Name: "called", Type: "String"},
		},
		Methods: map[string]Method{
			"register": {Name: "Registrar.register", ClassName: "Registrar", Params: []Param{{Name: "left", Type: "Child"}, {Name: "right", Type: "Child"}}, Program: childProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	machine.MethodOverloads["Registrar.register"] = []Method{
		{Name: "Registrar.register", ClassName: "Registrar", Params: []Param{{Name: "left", Type: "Child"}, {Name: "right", Type: "Child"}}, Program: childProgram},
		{Name: "Registrar.register", ClassName: "Registrar", Params: []Param{{Name: "left", Type: "Base"}, {Name: "right", Type: "Base"}}, Program: baseProgram},
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateStubFallsThroughExplicitPropertyGetterWhenProviderReturnsNull(t *testing.T) {
	getterProgram, err := CompileAnonymous("return value();")
	if err != nil {
		t.Fatal(err)
	}
	valueProgram, err := CompileAnonymous("return 'mock';")
	if err != nil {
		t.Fatal(err)
	}
	providerProgram, err := CompileAnonymous(`
if (stubbedMethodName == 'value') {
	return 'mocked method';
}
return null;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Base proxy = (Base)Test.createStub(Base.class, new Provider());
System.assertEquals('mocked method', proxy.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "Base",
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String", Getter: &Method{Name: "Base.Name.get", ClassName: "Base", ReturnType: "String", Program: getterProgram}},
		},
		Methods: map[string]Method{
			"value": {Name: "Base.value", ClassName: "Base", ReturnType: "String", Program: valueProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"StubProvider"},
		Methods: map[string]Method{
			"handleMethodCall": {
				Name:       "Provider.handleMethodCall",
				ClassName:  "Provider",
				ReturnType: "Object",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateStubInterceptsPropertyGetterWhenProviderReturnsValue(t *testing.T) {
	getterProgram, err := CompileAnonymous("return missing.value;")
	if err != nil {
		t.Fatal(err)
	}
	providerProgram, err := CompileAnonymous("return 'mocked property';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Base proxy = (Base)Test.createStub(Base.class, new Provider());
System.assertEquals('mocked property', proxy.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "Base",
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String", Getter: &Method{Name: "Base.Name.get", ClassName: "Base", ReturnType: "String", Program: getterProgram}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"StubProvider"},
		Methods: map[string]Method{
			"handleMethodCall": {
				Name:       "Provider.handleMethodCall",
				ClassName:  "Provider",
				ReturnType: "Object",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateStubPropertyGetterRunsPrivateHelper(t *testing.T) {
	getterProgram, err := CompileAnonymous("return helper();")
	if err != nil {
		t.Fatal(err)
	}
	helperProgram, err := CompileAnonymous("return value();")
	if err != nil {
		t.Fatal(err)
	}
	valueProgram, err := CompileAnonymous("return 'real';")
	if err != nil {
		t.Fatal(err)
	}
	providerProgram, err := CompileAnonymous(`
if (stubbedMethodName == 'value') {
	return 'mocked';
}
return null;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Base proxy = (Base)Test.createStub(Base.class, new Provider());
System.assertEquals('mocked', proxy.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "Base",
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String", Getter: &Method{Name: "Base.Name.get", ClassName: "Base", ReturnType: "String", Program: getterProgram}},
		},
		Methods: map[string]Method{
			"helper": {Name: "Base.helper", ClassName: "Base", ReturnType: "String", Access: "private", Program: helperProgram},
			"value":  {Name: "Base.value", ClassName: "Base", ReturnType: "String", Access: "public", Program: valueProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"StubProvider"},
		Methods: map[string]Method{
			"handleMethodCall": {
				Name:       "Provider.handleMethodCall",
				ClassName:  "Provider",
				ReturnType: "Object",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateStubCollectionGetterCanInitializeBackingProperty(t *testing.T) {
	getterProgram, err := CompileAnonymous(`
if (Items == null) {
	Items = new List<String>();
}
return Items;
`)
	if err != nil {
		t.Fatal(err)
	}
	providerProgram, err := CompileAnonymous("return null;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Base proxy = (Base)Test.createStub(Base.class, new Provider());
proxy.Items.add('Acme');
System.assertEquals(1, proxy.Items.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "Base",
		Fields: map[string]Field{
			"Items": {Name: "Items", Type: "List<String>", Getter: &Method{Name: "Base.Items.get", ClassName: "Base", ReturnType: "List<String>", Program: getterProgram}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"StubProvider"},
		Methods: map[string]Method{
			"handleMethodCall": {
				Name:       "Provider.handleMethodCall",
				ClassName:  "Provider",
				ReturnType: "Object",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateStubFallsBackForObjectToString(t *testing.T) {
	providerProgram, err := CompileAnonymous("return null;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
IService proxy = (IService)Test.createStub(IService.class, new Provider());
System.assert(proxy.toString().contains('__sfdc_ApexStub'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{Name: "IService", IsInterface: true}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"StubProvider"},
		Methods: map[string]Method{
			"handleMethodCall": {
				Name:       "Provider.handleMethodCall",
				ClassName:  "Provider",
				ReturnType: "Object",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateStubInterceptsVoidMethodFromStaticList(t *testing.T) {
	flushProgram, err := CompileAnonymous(`
for (Logger.ILogger logger : loggers) {
    logger.flush();
}
`)
	if err != nil {
		t.Fatal(err)
	}
	providerProgram, err := CompileAnonymous("return null;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Logger.loggers = new List<Logger.ILogger>{ (Logger.ILogger)Test.createStub(SystemLog.class, new Provider()) };
Logger.flush();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "Logger",
		StaticFields: map[string]Field{
			"loggers": {Name: "loggers", Type: "List<Logger.ILogger>", Static: true},
		},
		Methods: map[string]Method{
			"flush": {Name: "Logger.flush", ClassName: "Logger", ReturnType: "void", IsStatic: true, Program: flushProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:        "Logger.ILogger",
		IsInterface: true,
		Methods: map[string]Method{
			"flush": {Name: "Logger.ILogger.flush", ClassName: "Logger.ILogger", ReturnType: "void"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "SystemLog",
		Interfaces: []string{"Logger.ILogger"},
		Methods: map[string]Method{
			"flush": {Name: "SystemLog.flush", ClassName: "SystemLog", ReturnType: "void"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"StubProvider"},
		Methods: map[string]Method{
			"handleMethodCall": {
				Name:       "Provider.handleMethodCall",
				ClassName:  "Provider",
				ReturnType: "Object",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecConstructorSuperChainingPassesMapArgument(t *testing.T) {
	parentCtor, err := CompileAnonymous("values = source;")
	if err != nil {
		t.Fatal(err)
	}
	childCtor, err := CompileAnonymous("super(source);")
	if err != nil {
		t.Fatal(err)
	}
	getProgram, err := CompileAnonymous("return values.get(name);")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Map<String,Object> source = new Map<String,Object>{'id' => 'value'};
Child c = new Child(source);
System.assertEquals('value', c.get('id'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Parent",
		Fields: map[string]Field{
			"values": {Name: "values", Type: "Map<String,Object>"},
		},
		Constructors: []Method{{
			Name:          "Parent.<init>",
			ClassName:     "Parent",
			Params:        []Param{{Name: "source", Type: "Map<String,Object>"}},
			Program:       parentCtor,
			IsConstructor: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Child",
		SuperClass: "Parent",
		Constructors: []Method{{
			Name:          "Child.<init>",
			ClassName:     "Child",
			Params:        []Param{{Name: "source", Type: "Map<String,Object>"}},
			Program:       childCtor,
			IsConstructor: true,
		}},
		Methods: map[string]Method{
			"get": {Name: "Child.get", ClassName: "Child", ReturnType: "Object", Params: []Param{{Name: "name", Type: "String"}}, Program: getProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecConstructorChainPreservesConcreteSObjectListType(t *testing.T) {
	sobjectsCtor, err := CompileAnonymous("describeName = sObjectType.getDescribe().getName();")
	if err != nil {
		t.Fatal(err)
	}
	domainCtor, err := CompileAnonymous("this(records, records.getSObjectType());")
	if err != nil {
		t.Fatal(err)
	}
	domainTypedCtor, err := CompileAnonymous("super(records, sObjectType);")
	if err != nil {
		t.Fatal(err)
	}
	actionCtor, err := CompileAnonymous("super(records);")
	if err != nil {
		t.Fatal(err)
	}
	nameProgram, err := CompileAnonymous("return describeName;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<Account> records = new List<Account>{ new Account(Name = 'Acme') };
ActionDomain domain = new ActionDomain(records);
System.assertEquals('Account', domain.name());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "SObjects",
		Fields: map[string]Field{
			"describeName": {Name: "describeName", Type: "String"},
		},
		Constructors: []Method{{
			Name:          "SObjects.<init>",
			ClassName:     "SObjects",
			Params:        []Param{{Name: "records", Type: "List<SObject>"}, {Name: "sObjectType", Type: "Schema.SObjectType"}},
			Program:       sobjectsCtor,
			IsConstructor: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Domain",
		SuperClass: "SObjects",
		Constructors: []Method{
			{
				Name:          "Domain.<init>",
				ClassName:     "Domain",
				Params:        []Param{{Name: "records", Type: "List<SObject>"}},
				Program:       domainCtor,
				IsConstructor: true,
			},
			{
				Name:          "Domain.<init>",
				ClassName:     "Domain",
				Params:        []Param{{Name: "records", Type: "List<SObject>"}, {Name: "sObjectType", Type: "Schema.SObjectType"}},
				Program:       domainTypedCtor,
				IsConstructor: true,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "ActionDomain",
		SuperClass: "Domain",
		Constructors: []Method{{
			Name:          "ActionDomain.<init>",
			ClassName:     "ActionDomain",
			Params:        []Param{{Name: "records", Type: "List<Account>"}},
			Program:       actionCtor,
			IsConstructor: true,
		}},
		Methods: map[string]Method{
			"name": {Name: "ActionDomain.name", ClassName: "ActionDomain", ReturnType: "String", Program: nameProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMutatingInstanceCollectionFieldPersists(t *testing.T) {
	addProgram, err := CompileAnonymous("names.add(name);")
	if err != nil {
		t.Fatal(err)
	}
	sizeProgram, err := CompileAnonymous("return names.size();")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Box b = new Box();
b.add('Ada');
b.add('Grace');
System.assertEquals(2, b.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Box",
		Fields: map[string]Field{
			"names": {Name: "names", Type: "Set<String>", InitialValue: Set()},
		},
		Methods: map[string]Method{
			"add":  {Name: "Box.add", ClassName: "Box", Params: []Param{{Name: "name", Type: "String"}}, Program: addProgram},
			"size": {Name: "Box.size", ClassName: "Box", ReturnType: "Integer", Program: sizeProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPropertyAccessorMethods(t *testing.T) {
	getter, err := CompileAnonymous("return backing + '!';")
	if err != nil {
		t.Fatal(err)
	}
	setter, err := CompileAnonymous("backing = value.toUpperCase();")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
	Box b = new Box();
	b.Name = 'acme';
	System.assertEquals('ACME!', b.Name);
	System.assertEquals('ACME!', b.name);
	`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Box",
		Fields: map[string]Field{
			"backing": {Name: "backing", Type: "String"},
			"Name": {
				Name:     "Name",
				Type:     "String",
				Property: true,
				Getter:   &Method{Name: "Box.Name.get", ClassName: "Box", ReturnType: "String", Program: getter},
				Setter:   &Method{Name: "Box.Name.set", ClassName: "Box", Params: []Param{{Name: "value", Type: "String"}}, Program: setter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecIndexedPropertyAssignmentCallsSetter(t *testing.T) {
	getter, err := CompileAnonymous("return backing;")
	if err != nil {
		t.Fatal(err)
	}
	setter, err := CompileAnonymous(`
backing = value;
Touched = true;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<Box> boxes = new List<Box>{ new Box() };
boxes[0].Name = 'acme';
System.assertEquals('acme', boxes[0].Name);
System.assertEquals(true, boxes[0].Touched);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Box",
		Fields: map[string]Field{
			"backing": {Name: "backing", Type: "String"},
			"Touched": {Name: "Touched", Type: "Boolean"},
			"Name": {
				Name:     "Name",
				Type:     "String",
				Property: true,
				Getter:   &Method{Name: "Box.Name.get", ClassName: "Box", ReturnType: "String", Program: getter},
				Setter:   &Method{Name: "Box.Name.set", ClassName: "Box", Params: []Param{{Name: "value", Type: "String"}}, Program: setter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMethodResultIndexedPropertyAssignmentCallsSetter(t *testing.T) {
	getter, err := CompileAnonymous("return backing;")
	if err != nil {
		t.Fatal(err)
	}
	setter, err := CompileAnonymous(`
backing = value;
Touched = true;
`)
	if err != nil {
		t.Fatal(err)
	}
	rowsGetter, err := CompileAnonymous("return rows;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Holder holder = new Holder();
holder.rows = new List<Box>();
holder.rows.add(new Box());
holder.getRows()[0].Name = 'acme';
System.assertEquals('acme', holder.getRows()[0].Name);
System.assertEquals(true, holder.getRows()[0].Touched);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Box",
		Fields: map[string]Field{
			"backing": {Name: "backing", Type: "String"},
			"Touched": {Name: "Touched", Type: "Boolean"},
			"Name": {
				Name:     "Name",
				Type:     "String",
				Property: true,
				Getter:   &Method{Name: "Box.Name.get", ClassName: "Box", ReturnType: "String", Program: getter},
				Setter:   &Method{Name: "Box.Name.set", ClassName: "Box", Params: []Param{{Name: "value", Type: "String"}}, Program: setter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Holder",
		Fields: map[string]Field{
			"rows": {Name: "rows", Type: "List<Box>"},
		},
		Methods: map[string]Method{
			"getRows": {Name: "Holder.getRows", ClassName: "Holder", ReturnType: "List<Box>", Program: rowsGetter},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMethodResultIndexedPropertySetterNotifiesObserver(t *testing.T) {
	setter, err := CompileAnonymous(`
backing = value;
for (Counter observer : observers) {
	if (observer.Count == null) {
		observer.Count = 0;
	}
	observer.Count++;
}
`)
	if err != nil {
		t.Fatal(err)
	}
	rowsGetter, err := CompileAnonymous("return rows;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Holder holder = new Holder();
holder.rows = new List<Box>();
holder.rows.add(new Box());
Counter counter = new Counter();
holder.getRows()[0].observers = new List<Counter>();
holder.getRows()[0].observers.add(counter);
holder.getRows()[0].Name = 'acme';
System.assertEquals(1, counter.Count);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Counter",
		Fields: map[string]Field{
			"Count": {Name: "Count", Type: "Integer"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Box",
		Fields: map[string]Field{
			"backing":   {Name: "backing", Type: "String"},
			"observers": {Name: "observers", Type: "List<Counter>"},
			"Name": {
				Name:     "Name",
				Type:     "String",
				Property: true,
				Setter:   &Method{Name: "Box.Name.set", ClassName: "Box", Params: []Param{{Name: "value", Type: "String"}}, Program: setter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Holder",
		Fields: map[string]Field{
			"rows": {Name: "rows", Type: "List<Box>"},
		},
		Methods: map[string]Method{
			"getRows": {Name: "Holder.getRows", ClassName: "Holder", ReturnType: "List<Box>", Program: rowsGetter},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNestedPropertySetterOnDifferentInstanceCallsSetter(t *testing.T) {
	getter, err := CompileAnonymous("return backing;")
	if err != nil {
		t.Fatal(err)
	}
	setter, err := CompileAnonymous(`
backing = value;
if (Other != null && value == 'outer') {
	Other.Name = 'nested';
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Box first = new Box();
Box second = new Box();
first.Other = second;
first.Name = 'outer';
System.assertEquals('outer', first.Name);
System.assertEquals('nested', second.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Box",
		Fields: map[string]Field{
			"backing": {Name: "backing", Type: "String"},
			"Other":   {Name: "Other", Type: "Box"},
			"Name": {
				Name:     "Name",
				Type:     "String",
				Property: true,
				Getter:   &Method{Name: "Box.Name.get", ClassName: "Box", ReturnType: "String", Program: getter},
				Setter:   &Method{Name: "Box.Name.set", ClassName: "Box", Params: []Param{{Name: "value", Type: "String"}}, Program: setter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterClassMergesDuplicateNamespaceClassMembers(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "Shared",
		Namespace:  "znu",
		IsAbstract: true,
		Dependency: true,
		Fields: map[string]Field{
			"DepOnly": {Name: "DepOnly", Type: "String"},
			"Shared":  {Name: "Shared", Type: "DependencyType"},
		},
		Methods: map[string]Method{
			"depOnly#": {Name: "Shared.depOnly", ClassName: "Shared", ReturnType: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:      "Shared",
		Namespace: "znu",
		Fields: map[string]Field{
			"LocalOnly": {Name: "LocalOnly", Type: "String"},
			"Shared":    {Name: "Shared", Type: "LocalType"},
		},
		Methods: map[string]Method{
			"localOnly#": {Name: "Shared.localOnly", ClassName: "Shared", ReturnType: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	class, ok := machine.lookupClass("Shared")
	if !ok {
		t.Fatal("Shared class not registered")
	}
	if _, ok := class.Fields["DepOnly"]; !ok {
		t.Fatalf("dependency field missing after duplicate registration: %#v", class.Fields)
	}
	if _, ok := class.Fields["LocalOnly"]; !ok {
		t.Fatalf("local field missing after duplicate registration: %#v", class.Fields)
	}
	if _, ok := class.Methods["depOnly#"]; !ok {
		t.Fatalf("dependency method missing after duplicate registration: %#v", class.Methods)
	}
	if _, ok := class.Methods["localOnly#"]; !ok {
		t.Fatalf("local method missing after duplicate registration: %#v", class.Methods)
	}
	if class.IsAbstract {
		t.Fatal("incoming concrete duplicate should override dependency abstract flag")
	}
	if class.Fields["Shared"].Type != "LocalType" {
		t.Fatalf("project field should override dependency field, got %#v", class.Fields["Shared"])
	}
}

func TestRegisterClassKeepsProjectMemberWhenDependencyRegistersLater(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:      "Shared",
		Namespace: "znu",
		Fields: map[string]Field{
			"Shared": {Name: "Shared", Type: "LocalType"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Shared",
		Namespace:  "znu",
		Dependency: true,
		Fields: map[string]Field{
			"Shared": {Name: "Shared", Type: "DependencyType"},
			"Dep":    {Name: "Dep", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	class, ok := machine.lookupClass("Shared")
	if !ok {
		t.Fatal("Shared class not registered")
	}
	if class.Fields["Shared"].Type != "LocalType" {
		t.Fatalf("project field should survive later dependency duplicate, got %#v", class.Fields["Shared"])
	}
	if _, ok := class.Fields["Dep"]; !ok {
		t.Fatalf("dependency-only field should be preserved: %#v", class.Fields)
	}
}

func TestRegisterClassDuplicateMethodKeepsWidestVisibleSurface(t *testing.T) {
	getLocal, err := CompileAnonymous("return 'Account';")
	if err != nil {
		t.Fatal(err)
	}
	run, err := CompileAnonymous(`
SObjectTestData data = new AccountTestData();
return data.getSObjectType();
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("System.assertEquals('Account', Runner.run());")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:      "SObjectTestData",
		Namespace: "znu",
		Methods: map[string]Method{
			"getSObjectType": {Name: "SObjectTestData.getSObjectType", ClassName: "SObjectTestData", ReturnType: "String", Access: "protected", Modifiers: []string{"abstract"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "SObjectTestData",
		Namespace:  "znu",
		Dependency: true,
		Methods: map[string]Method{
			"getSObjectType": {Name: "SObjectTestData.getSObjectType", ClassName: "SObjectTestData", ReturnType: "String", Access: "public", Modifiers: []string{"abstract"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "AccountTestData",
		Namespace:  "znu",
		SuperClass: "SObjectTestData",
		Methods: map[string]Method{
			"getSObjectType": {Name: "AccountTestData.getSObjectType", ClassName: "AccountTestData", ReturnType: "String", Access: "protected", Program: getLocal},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:      "Runner",
		Namespace: "znu",
		Access:    "global",
		Methods: map[string]Method{
			"run": {Name: "Runner.run", ClassName: "Runner", ReturnType: "String", IsStatic: true, Access: "global", Program: run},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPassiveGeneratedPropertySetterMutatesBackingField(t *testing.T) {
	empty, err := CompileAnonymous("")
	if err != nil {
		t.Fatal(err)
	}
	enableProgram, err := CompileAnonymous("this.Stubbing = true;")
	if err != nil {
		t.Fatal(err)
	}
	checkProgram, err := CompileAnonymous("return Stubbing;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Box b = new Box();
b.Stubbing = true;
System.assertEquals(true, b.Stubbing);
b.Stubbing = false;
System.assertEquals(false, b.Stubbing);
b.enable();
System.assertEquals(true, b.Stubbing);
System.assertEquals(true, b.check());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Box",
		Fields: map[string]Field{
			"Stubbing": {
				Name:     "Stubbing",
				Type:     "Boolean",
				Property: true,
				Getter: &Method{
					Name:       "Box.Stubbing.get",
					ClassName:  "Box",
					ReturnType: "Boolean",
					Program:    empty,
					Modifiers:  []string{"passive-generated"},
				},
				Setter: &Method{
					Name:      "Box.Stubbing.set",
					ClassName: "Box",
					Params:    []Param{{Name: "value", Type: "Boolean"}},
					Program:   empty,
					Modifiers: []string{"passive-generated"},
				},
			},
		},
		Methods: map[string]Method{
			"enable": {Name: "Box.enable", ClassName: "Box", Program: enableProgram},
			"check":  {Name: "Box.check", ClassName: "Box", ReturnType: "Boolean", Program: checkProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNestedPassiveGeneratedPropertySetterPropagatesToParent(t *testing.T) {
	empty, err := CompileAnonymous("")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Parent p = new Parent();
p.Recorder.Stubbing = true;
System.assertEquals(true, p.Recorder.Stubbing);
p.Recorder.Stubbing = false;
System.assertEquals(false, p.Recorder.Stubbing);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Recorder",
		Fields: map[string]Field{
			"Stubbing": {
				Name:     "Stubbing",
				Type:     "Boolean",
				Property: true,
				Getter: &Method{
					Name:       "Recorder.Stubbing.get",
					ClassName:  "Recorder",
					ReturnType: "Boolean",
					Program:    empty,
					Modifiers:  []string{"passive-generated"},
				},
				Setter: &Method{
					Name:      "Recorder.Stubbing.set",
					ClassName: "Recorder",
					Params:    []Param{{Name: "value", Type: "Boolean"}},
					Program:   empty,
					Modifiers: []string{"passive-generated"},
				},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Parent",
		Fields: map[string]Field{
			"Recorder": {Name: "Recorder", Type: "Recorder", InitialValue: Object("Recorder")},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDottedAssignmentUsesIntermediatePropertyGetter(t *testing.T) {
	getter, err := CompileAnonymous(`
if (child == null) {
    child = new Child();
}
return child;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Parent p = new Parent();
p.Child.Name = 'Acme';
System.assertEquals('Acme', p.Child.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Child",
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Parent",
		Fields: map[string]Field{
			"child": {Name: "child", Type: "Child", Access: "private"},
			"Child": {
				Name:     "Child",
				Type:     "Child",
				Property: true,
				Getter:   &Method{Name: "Parent.Child.get", ClassName: "Parent", ReturnType: "Child", Program: getter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPropertyGetterSelfReferenceUsesBackingField(t *testing.T) {
	getter, err := CompileAnonymous("return this.Name;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Box b = new Box();
b.Name = 'Acme';
System.assertEquals('Acme', b.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Box",
		Fields: map[string]Field{
			"Name": {
				Name:     "Name",
				Type:     "String",
				Property: true,
				Getter:   &Method{Name: "Box.Name.get", ClassName: "Box", ReturnType: "String", Program: getter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPropertySetterSelfReferenceUsesBackingField(t *testing.T) {
	setter, err := CompileAnonymous(`
Seen = value;
this.Name = value;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Box b = new Box();
b.Name = 'Acme';
System.assertEquals('Acme', b.Name);
System.assertEquals('Acme', b.Seen);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Box",
		Fields: map[string]Field{
			"Seen": {Name: "Seen", Type: "String"},
			"Name": {
				Name:     "Name",
				Type:     "String",
				Property: true,
				Setter:   &Method{Name: "Box.Name.set", ClassName: "Box", Params: []Param{{Name: "value", Type: "String"}}, Program: setter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringConcatWithTypedNullStringOperand(t *testing.T) {
	program, err := CompileAnonymous(`
Id missing;
Id present = '001000000000001';
String out = (String)missing + present;
System.assertEquals('null001000000000001AAA', out);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMethodParameterNormalizesTypedDynamicDateLiteral(t *testing.T) {
	methodProgram, err := CompileAnonymous(`
System.assertEquals(Date.today(), transactionDate);
return transactionDate;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Date raw = (Date)'TODAY()';
Date normalized = Probe.accept(raw);
System.assertEquals(Date.today(), normalized);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "Probe.accept",
		ClassName:  "Probe",
		IsStatic:   true,
		ReturnType: "Date",
		Params:     []Param{{Name: "transactionDate", Type: "Date"}},
		Program:    methodProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMethodParameterNormalizesDynamicDateLiteralToDatetime(t *testing.T) {
	methodProgram, err := CompileAnonymous(`
System.assertEquals(Datetime.newInstance(Date.today(), Time.newInstance(0, 0, 0, 0)), transactionDate);
return transactionDate;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Date raw = (Date)'TODAY()';
Datetime normalized = Probe.acceptDatetime(raw);
System.assertEquals(Date.today(), normalized.date());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "Probe.acceptDatetime",
		ClassName:  "Probe",
		IsStatic:   true,
		ReturnType: "Datetime",
		Params:     []Param{{Name: "transactionDate", Type: "Datetime"}},
		Program:    methodProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecConstructorAcceptsTypedNullArguments(t *testing.T) {
	program, err := CompileAnonymous(`
String missing;
Box box = new Box(missing);
System.assertEquals(null, box.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	ctor, err := CompileAnonymous(`this.Name = name;`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Box",
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String"},
		},
		Constructors: []Method{{
			Name:          "Box",
			ClassName:     "Box",
			IsConstructor: true,
			Params:        []Param{{Name: "name", Type: "String"}},
			Program:       ctor,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPropertyGetterCanInitializeAutoProperty(t *testing.T) {
	getter, err := CompileAnonymous(`
if (Items == null) {
	Items = new List<String>();
}
return Items;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Box b = new Box();
b.Items.add('Acme');
System.assertEquals(1, b.Items.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Box",
		Fields: map[string]Field{
			"Items": {
				Name:     "Items",
				Type:     "List<String>",
				Property: true,
				Getter:   &Method{Name: "Box.Items.get", ClassName: "Box", ReturnType: "List<String>", Program: getter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPropertyGetterSelfAssignmentWithSetterUpdatesBackingValue(t *testing.T) {
	getter, err := CompileAnonymous(`
if (Name == null) {
	Name = 'Acme';
}
return Name;
`)
	if err != nil {
		t.Fatal(err)
	}
	setter, err := CompileAnonymous(`return;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Box b = new Box();
System.assertEquals('Acme', b.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Box",
		Fields: map[string]Field{
			"Name": {
				Name:     "Name",
				Type:     "String",
				Property: true,
				Getter:   &Method{Name: "Box.Name.get", ClassName: "Box", ReturnType: "String", Program: getter},
				Setter:   &Method{Name: "Box.Name.set", ClassName: "Box", Params: []Param{{Name: "value", Type: "String"}}, Program: setter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPropertyGetterSelfAssignmentCachesBackingValueAcrossCalls(t *testing.T) {
	getter, err := CompileAnonymous(`
if (DefaultRecord == null) {
	DefaultRecord = new Box();
	DefaultRecord.Name = 'Acme';
}
return DefaultRecord;
`)
	if err != nil {
		t.Fatal(err)
	}
	setter, err := CompileAnonymous(`return;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Cache cache = new Cache();
Box first = cache.DefaultRecord;
first.Name = 'Changed';
Box second = cache.DefaultRecord;
System.assertEquals('Changed', second.Name);
System.assertEquals(first, second);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Box",
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Cache",
		Fields: map[string]Field{
			"DefaultRecord": {
				Name:     "DefaultRecord",
				Type:     "Box",
				Property: true,
				Getter:   &Method{Name: "Cache.DefaultRecord.get", ClassName: "Cache", ReturnType: "Box", Program: getter},
				Setter:   &Method{Name: "Cache.DefaultRecord.set", ClassName: "Cache", Params: []Param{{Name: "value", Type: "Box"}}, Program: setter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamespacedPropertyGetterSelfAssignmentCachesBackingValueAcrossCalls(t *testing.T) {
	getter, err := CompileAnonymous(`
if (DefaultRecord == null) {
	DefaultRecord = new znu.Box();
	DefaultRecord.Name = 'Acme';
}
return DefaultRecord;
`)
	if err != nil {
		t.Fatal(err)
	}
	setter, err := CompileAnonymous(`return;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
znu.Cache cache = new znu.Cache();
znu.Box first = cache.DefaultRecord;
first.Name = 'Changed';
znu.Box second = cache.DefaultRecord;
System.assertEquals('Changed', second.Name);
System.assertEquals(first, second);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "znu.Box",
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "znu.Cache",
		Fields: map[string]Field{
			"DefaultRecord": {
				Name:     "DefaultRecord",
				Type:     "znu.Box",
				Property: true,
				Getter:   &Method{Name: "znu.Cache.DefaultRecord.get", ClassName: "znu.Cache", ReturnType: "znu.Box", Program: getter},
				Setter:   &Method{Name: "znu.Cache.DefaultRecord.set", ClassName: "znu.Cache", Params: []Param{{Name: "value", Type: "znu.Box"}}, Program: setter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStaticPropertyGetterSelfAssignmentWithSetterUpdatesBackingValue(t *testing.T) {
	getter, err := CompileAnonymous(`
if (Instance == null) {
	Instance = new Box();
}
return Instance;
`)
	if err != nil {
		t.Fatal(err)
	}
	setter, err := CompileAnonymous(`return;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertNotEquals(null, BoxService.Instance);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Box"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "BoxService",
		StaticFields: map[string]Field{
			"Instance": {
				Name:     "Instance",
				Type:     "Box",
				Static:   true,
				Property: true,
				Getter:   &Method{Name: "BoxService.Instance.get", ClassName: "BoxService", ReturnType: "Box", Program: getter},
				Setter:   &Method{Name: "BoxService.Instance.set", ClassName: "BoxService", Params: []Param{{Name: "value", Type: "Box"}}, Program: setter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStaticPropertyGetterSelfAssignmentCachesBackingValueAcrossCalls(t *testing.T) {
	getter, err := CompileAnonymous(`
if (Instance == null) {
	Instance = new Box();
}
return Instance;
`)
	if err != nil {
		t.Fatal(err)
	}
	setter, err := CompileAnonymous(`return;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Box first = BoxService.Instance;
first.Name = 'Changed';
Box second = BoxService.Instance;
System.assertEquals('Changed', second.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Box",
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "BoxService",
		StaticFields: map[string]Field{
			"Instance": {
				Name:     "Instance",
				Type:     "Box",
				Static:   true,
				Property: true,
				Getter:   &Method{Name: "BoxService.Instance.get", ClassName: "BoxService", ReturnType: "Box", Program: getter},
				Setter:   &Method{Name: "BoxService.Instance.set", ClassName: "BoxService", Params: []Param{{Name: "value", Type: "Box"}}, Program: setter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamespacedStaticPropertyGetterSelfAssignmentCachesBackingValueAcrossCalls(t *testing.T) {
	getter, err := CompileAnonymous(`
if (Instance == null) {
	Instance = new znu.Box();
}
return Instance;
`)
	if err != nil {
		t.Fatal(err)
	}
	setter, err := CompileAnonymous(`return;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
znu.Box first = znu.BoxService.Instance;
first.Name = 'Changed';
znu.Box second = znu.BoxService.Instance;
System.assertEquals('Changed', second.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "znu.Box",
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "znu.BoxService",
		StaticFields: map[string]Field{
			"Instance": {
				Name:     "Instance",
				Type:     "znu.Box",
				Static:   true,
				Property: true,
				Getter:   &Method{Name: "znu.BoxService.Instance.get", ClassName: "znu.BoxService", ReturnType: "znu.Box", Program: getter},
				Setter:   &Method{Name: "znu.BoxService.Instance.set", ClassName: "znu.BoxService", Params: []Param{{Name: "value", Type: "znu.Box"}}, Program: setter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStaticSingletonInstancePropertySelfAssignmentPersistsOnSingleton(t *testing.T) {
	instanceGetter, err := CompileAnonymous(`
if (Instance == null) {
	Instance = new Cache();
}
return Instance;
`)
	if err != nil {
		t.Fatal(err)
	}
	defaultGetter, err := CompileAnonymous(`
if (DefaultRecord == null) {
	DefaultRecord = new Box();
	DefaultRecord.Name = 'Acme';
}
return DefaultRecord;
`)
	if err != nil {
		t.Fatal(err)
	}
	setter, err := CompileAnonymous(`return;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Box first = Cache.Instance.DefaultRecord;
Box second = Cache.Instance.DefaultRecord;
System.assertEquals(first.Name, second.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Box",
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Cache",
		StaticFields: map[string]Field{
			"Instance": {
				Name:     "Instance",
				Type:     "Cache",
				Static:   true,
				Property: true,
				Getter:   &Method{Name: "Cache.Instance.get", ClassName: "Cache", ReturnType: "Cache", Program: instanceGetter},
				Setter:   &Method{Name: "Cache.Instance.set", ClassName: "Cache", Params: []Param{{Name: "value", Type: "Cache"}}, Program: setter},
			},
		},
		Fields: map[string]Field{
			"DefaultRecord": {
				Name:     "DefaultRecord",
				Type:     "Box",
				Property: true,
				Getter:   &Method{Name: "Cache.DefaultRecord.get", ClassName: "Cache", ReturnType: "Box", Program: defaultGetter},
				Setter:   &Method{Name: "Cache.DefaultRecord.set", ClassName: "Cache", Params: []Param{{Name: "value", Type: "Box"}}, Program: setter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamespacedStaticPropertyGetterSelfAssignmentDispatchesInstanceMethod(t *testing.T) {
	getter, err := CompileAnonymous(`
if (Instance == null) {
	Instance = new znu.Box();
}
return Instance;
`)
	if err != nil {
		t.Fatal(err)
	}
	setter, err := CompileAnonymous(`return;`)
	if err != nil {
		t.Fatal(err)
	}
	touch, err := CompileAnonymous(`return 'touched';`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals('touched', znu.BoxService.Instance.touch());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "znu.Box",
		Methods: map[string]Method{
			"touch": {Name: "znu.Box.touch", ClassName: "znu.Box", ReturnType: "String", Program: touch},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "znu.BoxService",
		StaticFields: map[string]Field{
			"Instance": {
				Name:     "Instance",
				Type:     "znu.Box",
				Static:   true,
				Property: true,
				Getter:   &Method{Name: "znu.BoxService.Instance.get", ClassName: "znu.BoxService", ReturnType: "znu.Box", Program: getter},
				Setter:   &Method{Name: "znu.BoxService.Instance.set", ClassName: "znu.BoxService", Params: []Param{{Name: "value", Type: "znu.Box"}}, Program: setter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNamespacedClassNewExpressionPrefersSameNamespace(t *testing.T) {
	getter, err := CompileAnonymous(`
if (Instance == null) {
	Instance = new Box();
}
return Instance;
`)
	if err != nil {
		t.Fatal(err)
	}
	setter, err := CompileAnonymous(`return;`)
	if err != nil {
		t.Fatal(err)
	}
	namespacedTouch, err := CompileAnonymous(`return 'namespaced';`)
	if err != nil {
		t.Fatal(err)
	}
	localTouch, err := CompileAnonymous(`return 'local';`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals('namespaced', znu.BoxService.Instance.touch());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Box",
		Methods: map[string]Method{
			"touch": {Name: "Box.touch", ClassName: "Box", ReturnType: "String", Program: localTouch},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "znu.Box",
		Methods: map[string]Method{
			"touch": {Name: "znu.Box.touch", ClassName: "znu.Box", ReturnType: "String", Program: namespacedTouch},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "znu.BoxService",
		StaticFields: map[string]Field{
			"Instance": {
				Name:     "Instance",
				Type:     "znu.Box",
				Static:   true,
				Property: true,
				Getter:   &Method{Name: "znu.BoxService.Instance.get", ClassName: "znu.BoxService", ReturnType: "znu.Box", Program: getter},
				Setter:   &Method{Name: "znu.BoxService.Instance.set", ClassName: "znu.BoxService", Params: []Param{{Name: "value", Type: "znu.Box"}}, Program: setter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDependencySingletonDuplicateClassKeepsDependencyMethod(t *testing.T) {
	getter, err := CompileAnonymous(`
if (Instance == null) {
	Instance = new PaymentOptions();
}
return Instance;
`)
	if err != nil {
		t.Fatal(err)
	}
	setter, err := CompileAnonymous(`return;`)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := CompileAnonymous(`return 'selected';`)
	if err != nil {
		t.Fatal(err)
	}
	run, err := CompileAnonymous(`
return znu.PaymentOptionsService.Instance.getSelectedPaymentOption(new znu.PaymentOptionsService.Request());
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals('selected', znu.Runner.run());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:      "PaymentOptions",
		Namespace: "znu",
		Methods: map[string]Method{
			"localOnly": {Name: "PaymentOptions.localOnly", ClassName: "PaymentOptions", ReturnType: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "PaymentOptions",
		Namespace:  "znu",
		Access:     "global",
		Dependency: true,
		Methods: map[string]Method{
			"getSelectedPaymentOption": {
				Name:       "PaymentOptions.getSelectedPaymentOption",
				ClassName:  "PaymentOptions",
				ReturnType: "String",
				Access:     "global",
				Params:     []Param{{Name: "request", Type: "PaymentOptionsService.Request"}},
				Program:    selected,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:      "PaymentOptionsService",
		Namespace: "znu",
		StaticFields: map[string]Field{
			"Instance": {
				Name:     "Instance",
				Type:     "PaymentOptionsService",
				Static:   true,
				Property: true,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "PaymentOptionsService",
		Namespace:  "znu",
		Access:     "global",
		Dependency: true,
		StaticFields: map[string]Field{
			"Instance": {
				Name:     "Instance",
				Type:     "PaymentOptions",
				Access:   "global",
				Static:   true,
				Property: true,
				Getter:   &Method{Name: "PaymentOptionsService.Instance.get", ClassName: "PaymentOptionsService", ReturnType: "PaymentOptions", Access: "global", Program: getter},
				Setter:   &Method{Name: "PaymentOptionsService.Instance.set", ClassName: "PaymentOptionsService", Params: []Param{{Name: "value", Type: "PaymentOptions"}}, Access: "private", Program: setter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "PaymentOptionsService.Request",
		Namespace:  "znu",
		Access:     "global",
		Dependency: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:      "Runner",
		Namespace: "znu",
		Access:    "global",
		Methods: map[string]Method{
			"run": {Name: "Runner.run", ClassName: "Runner", ReturnType: "String", IsStatic: true, Access: "global", Program: run},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestLookupDuplicateStaticFieldKeepsSelectedStorageSlot(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:      "PaymentOptionsService",
		Namespace: "znu",
		StaticFields: map[string]Field{
			"Instance": {
				Name:   "Instance",
				Type:   "PaymentOptionsService",
				Static: true,
				File:   "consumer/PaymentOptionsService.cls",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "PaymentOptionsService",
		Namespace:  "znu",
		Dependency: true,
		StaticFields: map[string]Field{
			"Instance": {
				Name:   "Instance",
				Type:   "PaymentOptions",
				Static: true,
				File:   "dependency/PaymentOptionsService.cls",
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	field, _, ok := machine.lookupStaticFieldForReceiver("znu.PaymentOptionsService", "Instance", true)
	if !ok {
		t.Fatal("dependency static field was not found")
	}
	if got := staticFieldStorageName("Instance", field); got == "Instance" {
		t.Fatalf("duplicate dependency field storage key collapsed to %q", got)
	}
}

func TestExecNamespacedStaticSingletonAliasesShareInstanceState(t *testing.T) {
	instanceGetter, err := CompileAnonymous(`
if (Instance == null) {
	Instance = new EntityTestData();
}
return Instance;
`)
	if err != nil {
		t.Fatal(err)
	}
	entityGetter, err := CompileAnonymous(`
if (DefaultEntity == null) {
	DefaultEntity = new Account(Name = 'default');
}
return DefaultEntity;
`)
	if err != nil {
		t.Fatal(err)
	}
	setter, err := CompileAnonymous(`return;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
EntityTestData data = znu.EntityTestData.Instance;
Account first = data.DefaultEntity;
first.Name = 'changed';
Account second = EntityTestData.Instance.DefaultEntity;
System.assertEquals('changed', second.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:      "EntityTestData",
		Namespace: "znu",
		Access:    "global",
		StaticFields: map[string]Field{
			"Instance": {
				Name:     "Instance",
				Type:     "EntityTestData",
				Access:   "global",
				Static:   true,
				Property: true,
				Getter:   &Method{Name: "EntityTestData.Instance.get", ClassName: "EntityTestData", ReturnType: "EntityTestData", Access: "global", Program: instanceGetter},
				Setter:   &Method{Name: "EntityTestData.Instance.set", ClassName: "EntityTestData", Params: []Param{{Name: "value", Type: "EntityTestData"}}, Access: "global", Program: setter},
			},
		},
		Fields: map[string]Field{
			"DefaultEntity": {
				Name:     "DefaultEntity",
				Type:     "Account",
				Access:   "global",
				Property: true,
				Getter:   &Method{Name: "EntityTestData.DefaultEntity.get", ClassName: "EntityTestData", ReturnType: "Account", Access: "global", Program: entityGetter},
				Setter:   &Method{Name: "EntityTestData.DefaultEntity.set", ClassName: "EntityTestData", Params: []Param{{Name: "value", Type: "Account"}}, Access: "global", Program: setter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestConversionScoreAllowsNamespacedNestedTypeForUnqualifiedParameter(t *testing.T) {
	machine := New(nil)
	if got := machine.conversionScore("PaymentOptionsService.Request", Object("znu.PaymentOptionsService.Request")); got < 0 {
		t.Fatalf("conversionScore = %d, want assignable", got)
	}
}

func TestExecConstructorStoresPropertyBackingValueForSelfReferentialGetter(t *testing.T) {
	getter, err := CompileAnonymous(`
if (Result == null) {
	Result = new OperationResult();
}
return Result;
`)
	if err != nil {
		t.Fatal(err)
	}
	ctor, err := CompileAnonymous(`this.Result = opResult;`)
	if err != nil {
		t.Fatal(err)
	}
	isNotSuccessful, err := CompileAnonymous(`return !Success;`)
	if err != nil {
		t.Fatal(err)
	}
	resultCtor, err := CompileAnonymous(`Success = true;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
OperationResult failed = new OperationResult();
failed.Success = false;
Response response = new Response(failed);
System.assert(response.Result.isNotSuccessful(), 'expected constructor-assigned result to remain failed');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "OperationResult",
		Fields: map[string]Field{
			"Success": {Name: "Success", Type: "Boolean", Property: true},
		},
		Constructors: []Method{{
			Name:          "OperationResult",
			ClassName:     "OperationResult",
			IsConstructor: true,
			Program:       resultCtor,
		}},
		Methods: map[string]Method{
			"isNotSuccessful": {Name: "OperationResult.isNotSuccessful", ClassName: "OperationResult", ReturnType: "Boolean", Program: isNotSuccessful},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Response",
		Fields: map[string]Field{
			"Result": {
				Name:     "Result",
				Type:     "OperationResult",
				Property: true,
				Getter:   &Method{Name: "Response.Result.get", ClassName: "Response", ReturnType: "OperationResult", Program: getter},
			},
		},
		Constructors: []Method{{
			Name:          "Response",
			ClassName:     "Response",
			IsConstructor: true,
			Params:        []Param{{Name: "opResult", Type: "OperationResult"}},
			Program:       ctor,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInheritedWrapperGetterReadsSObjectFieldToken(t *testing.T) {
	getValueFromField, err := CompileAnonymous(`
if (this.record == null) {
	return null;
}
return this.record.get(field);
`)
	if err != nil {
		t.Fatal(err)
	}
	transactionDateGetter, err := CompileAnonymous(`
if (TransactionDate == null) {
	TransactionDate = (Date)getValueFromField(OrderItem__c.TransactionDate__c);
}
return TransactionDate;
`)
	if err != nil {
		t.Fatal(err)
	}
	ctor, err := CompileAnonymous(`this.record = row;`)
	if err != nil {
		t.Fatal(err)
	}
	newInstance, err := CompileAnonymous(`return new AdjustmentOrderItem(row);`)
	if err != nil {
		t.Fatal(err)
	}
	orderItemNewInstance, err := CompileAnonymous(`return AdjustmentOrderItem.newInstance(row);`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
OrderItem__C row = (OrderItem__c)JSON.deserialize('{"TransactionDate__c":"2026-05-02"}', OrderItem__c.class);
OrderItem wrapper = OrderItem.newInstance(row);
System.assertEquals(Date.newInstance(2026, 5, 2), wrapper.TransactionDate);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["OrderItem__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "OrderItem__c",
			KeyPrefix: "a01",
			Fields: map[string]storage.Field{
				"Id":                 {APIName: "Id", Type: storage.FieldID},
				"TransactionDate__c": {APIName: "TransactionDate__c", Type: storage.FieldDate},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "SObjectWrapper",
		Fields: map[string]Field{
			"record": {Name: "record", Type: "SObject", Access: "protected"},
		},
		Methods: map[string]Method{
			"getValueFromField": {
				Name:       "SObjectWrapper.getValueFromField",
				ClassName:  "SObjectWrapper",
				ReturnType: "Object",
				Params:     []Param{{Name: "field", Type: "Schema.SObjectField"}},
				Program:    getValueFromField,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "OrderItem",
		SuperClass: "SObjectWrapper",
		Fields: map[string]Field{
			"TransactionDate": {
				Name:     "TransactionDate",
				Type:     "Date",
				Property: true,
				Getter:   &Method{Name: "OrderItem.TransactionDate.get", ClassName: "OrderItem", ReturnType: "Date", Program: transactionDateGetter},
			},
		},
		Methods: map[string]Method{
			"newInstance": {
				Name:       "OrderItem.newInstance",
				ClassName:  "OrderItem",
				IsStatic:   true,
				ReturnType: "OrderItem",
				Params:     []Param{{Name: "row", Type: "OrderItem__c"}},
				Program:    orderItemNewInstance,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "AdjustmentOrderItem",
		SuperClass: "OrderItem",
		Constructors: []Method{{
			Name:          "AdjustmentOrderItem",
			ClassName:     "AdjustmentOrderItem",
			IsConstructor: true,
			Params:        []Param{{Name: "row", Type: "OrderItem__c"}},
			Program:       ctor,
		}},
		Methods: map[string]Method{
			"newInstance": {
				Name:       "AdjustmentOrderItem.newInstance",
				ClassName:  "AdjustmentOrderItem",
				IsStatic:   true,
				ReturnType: "AdjustmentOrderItem",
				Params:     []Param{{Name: "row", Type: "OrderItem__c"}},
				Program:    newInstance,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecListLiteralKeepsNestedListElement(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> names = new List<String>{ 'Ada' };
List<Object> values = new List<Object>{ names };
System.assertEquals(1, values.size());
System.assert(values[0] instanceof List<String>);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestApproxValueSizeHandlesCyclicObjects(t *testing.T) {
	account := Object("Account")
	account.Fields["Name"] = String("Acme")
	account.Fields["Parent"] = account
	if got := approxValueSize(account); got <= 0 {
		t.Fatalf("approxValueSize(account) = %d, want positive size", got)
	}
}

func TestRegisterClassPreservesFieldOrder(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Ordered",
		Fields: map[string]Field{
			"second": {Name: "second", Type: "Integer"},
			"first":  {Name: "first", Type: "Integer"},
		},
		FieldOrder: []string{"first", "second"},
		StaticFields: map[string]Field{
			"beta":  {Name: "beta", Type: "Integer"},
			"alpha": {Name: "alpha", Type: "Integer"},
		},
		StaticFieldOrder: []string{"alpha", "beta"},
	}); err != nil {
		t.Fatal(err)
	}
	class := machine.Classes["Ordered"]
	if got, want := strings.Join(class.FieldOrder, ","), "first,second"; got != want {
		t.Fatalf("FieldOrder = %s, want %s", got, want)
	}
	if got, want := strings.Join(class.StaticFieldOrder, ","), "alpha,beta"; got != want {
		t.Fatalf("StaticFieldOrder = %s, want %s", got, want)
	}
}

func TestRuntimeRejectsPrivateMemberAccessAcrossClasses(t *testing.T) {
	hidden, err := CompileAnonymous("return 'ok';")
	if err != nil {
		t.Fatal(err)
	}
	reveal, err := CompileAnonymous("return hidden();")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Secret s = new Secret();
System.assertEquals('ok', s.reveal());
s.hidden();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Secret",
		Methods: map[string]Method{
			"hidden": {Name: "Secret.hidden", ClassName: "Secret", ReturnType: "String", Access: "private", Program: hidden},
			"reveal": {Name: "Secret.reveal", ClassName: "Secret", ReturnType: "String", Access: "public", Program: reveal},
		},
	}); err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "Secret.hidden is private") {
		t.Fatalf("err = %v, want private method visibility error", err)
	}
}

func TestRuntimeResolvesUnqualifiedStaticMethodInCurrentClass(t *testing.T) {
	helper, err := CompileAnonymous("return values[0] + '-static';")
	if err != nil {
		t.Fatal(err)
	}
	check, err := CompileAnonymous("return values.isEmpty() ? null : values[0].Name;")
	if err != nil {
		t.Fatal(err)
	}
	reveal, err := CompileAnonymous("return helper((List<String>)values);")
	if err != nil {
		t.Fatal(err)
	}
	revealAccount, err := CompileAnonymous("return checkAccounts(records);")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
StaticHelper h = new StaticHelper();
System.assertEquals('spruce-static', h.reveal(new List<Object>{'spruce'}));
Account a = new Account(Name = 'Acme');
List<sObject> records = new List<sObject>{a};
System.assertEquals('Acme', h.revealAccount(records));
System.assertEquals(null, h.revealAccount(new List<sObject>()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "StaticHelper",
		Methods: map[string]Method{
			"helper":        {Name: "StaticHelper.helper", ClassName: "StaticHelper", ReturnType: "String", Params: []Param{{Name: "values", Type: "List<String>"}}, Access: "private", IsStatic: true, Program: helper},
			"checkAccounts": {Name: "StaticHelper.checkAccounts", ClassName: "StaticHelper", ReturnType: "String", Params: []Param{{Name: "values", Type: "List<Account>"}}, Access: "private", IsStatic: true, Program: check},
			"reveal":        {Name: "StaticHelper.reveal", ClassName: "StaticHelper", ReturnType: "String", Params: []Param{{Name: "values", Type: "List<Object>"}}, Access: "public", Program: reveal},
			"revealAccount": {Name: "StaticHelper.revealAccount", ClassName: "StaticHelper", ReturnType: "String", Params: []Param{{Name: "records", Type: "List<sObject>"}}, Access: "public", Program: revealAccount},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeResolvesDottedStaticPropertyInCurrentClass(t *testing.T) {
	settingsGetter, err := CompileAnonymous("return new Account(Name = 'Acme');")
	if err != nil {
		t.Fatal(err)
	}
	reveal, err := CompileAnonymous("return settings.Name;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
SettingsHolder h = new SettingsHolder();
System.assertEquals('Acme', h.reveal());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "SettingsHolder",
		StaticFields: map[string]Field{
			"settings": {Name: "settings", Type: "Account", Static: true, Access: "private", Getter: &Method{Name: "SettingsHolder.getSettings", ClassName: "SettingsHolder", ReturnType: "Account", IsStatic: true, Access: "private", Program: settingsGetter}},
		},
		StaticFieldOrder: []string{"settings"},
		Methods: map[string]Method{
			"reveal": {Name: "SettingsHolder.reveal", ClassName: "SettingsHolder", ReturnType: "String", Access: "public", Program: reveal},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeCallsMethodOnDottedStaticProperty(t *testing.T) {
	instanceGetter, err := CompileAnonymous("return new DottedWorker();")
	if err != nil {
		t.Fatal(err)
	}
	work, err := CompileAnonymous("return 'bank-vault';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals('bank-vault', DottedManager.Instance.work());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "DottedManager",
		StaticFields: map[string]Field{
			"Instance": {Name: "Instance", Type: "DottedWorker", Static: true, Access: "public", Getter: &Method{Name: "DottedManager.getInstance", ClassName: "DottedManager", ReturnType: "DottedWorker", IsStatic: true, Access: "public", Program: instanceGetter}},
		},
		StaticFieldOrder: []string{"Instance"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "DottedWorker",
		Methods: map[string]Method{
			"work": {Name: "DottedWorker.work", ClassName: "DottedWorker", ReturnType: "String", Access: "public", Program: work},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeCallsMethodOnUnqualifiedStaticPropertyReceiver(t *testing.T) {
	settingsGetter, err := CompileAnonymous("return 'billing';")
	if err != nil {
		t.Fatal(err)
	}
	check, err := CompileAnonymous("return settings.contains('bill');")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals(true, StaticSettingsHolder.check());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "StaticSettingsHolder",
		StaticFields: map[string]Field{
			"settings": {Name: "settings", Type: "String", Static: true, Access: "private", Getter: &Method{Name: "StaticSettingsHolder.getSettings", ClassName: "StaticSettingsHolder", ReturnType: "String", IsStatic: true, Access: "private", Program: settingsGetter}},
		},
		StaticFieldOrder: []string{"settings"},
		Methods: map[string]Method{
			"check": {Name: "StaticSettingsHolder.check", ClassName: "StaticSettingsHolder", ReturnType: "Boolean", IsStatic: true, Access: "public", Program: check},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestManagedAPIStaticPropertyChainUsesPassiveExternalObject(t *testing.T) {
	program, err := CompileAnonymous(`
znu.ProductsApiV1Retriever retriever = znu.ProductsApi.v1.retriever();
znu.ProductsApiRetriever baseRetriever = znu.ProductsApi.v1.retriever();
System.assertNotEquals(null, retriever);
System.assertNotEquals(null, baseRetriever);
System.assertNotEquals(null, retriever.with(new znu.QPlugin.Fields(new Set<String>{ 'Name' })));
System.assertEquals(0, retriever.getById(new Set<Id>{ '001000000000001AAA' }).size());
znu.BulkPriceClassRequest request = new znu.BulkPriceClassRequest();
request.Requests.addAll(new List<Object>());
System.assertEquals(0, request.Requests.size());
System.assertNotEquals(null, znu.ProductPricingInfo.newInstance());
System.assertEquals(false, znu.CurrenciesApi.v1.isMultiCurrencyEnabled());
System.assertNotEquals(null, znu.NimbleAmsSettingsService.Instance.getDefaultEntityId());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeUnqualifiedInstanceCallIgnoresSubclassStaticMethod(t *testing.T) {
	create, err := CompileAnonymous(`
SObject recordToInsert = build();
return recordToInsert;
`)
	if err != nil {
		t.Fatal(err)
	}
	instanceBuild, err := CompileAnonymous("return new Account(Name = 'Acme');")
	if err != nil {
		t.Fatal(err)
	}
	staticBuild, err := CompileAnonymous("return new ChildBuilder();")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
BaseBuilder builder = new ChildBuilder();
SObject record = builder.create();
System.assertEquals('Account', record.getSObjectType().getDescribe().getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "BaseBuilder",
		Methods: map[string]Method{
			"create": {Name: "BaseBuilder.create", ClassName: "BaseBuilder", ReturnType: "SObject", Access: "public", Program: create},
			"build":  {Name: "BaseBuilder.build", ClassName: "BaseBuilder", ReturnType: "SObject", Access: "private", Program: instanceBuild},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "ChildBuilder",
		SuperClass: "BaseBuilder",
		Methods: map[string]Method{
			"build": {Name: "ChildBuilder.build", ClassName: "ChildBuilder", ReturnType: "ChildBuilder", IsStatic: true, Access: "public", Program: staticBuild},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeResolvesUnqualifiedSchemaDescribeAliases(t *testing.T) {
	check, err := CompileAnonymous("return 'readable';")
	if err != nil {
		t.Fatal(err)
	}
	run, err := CompileAnonymous("return checkFieldIsReadable(Account.SObjectType, Account.Name.getDescribe());")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals('readable', SecurityProbe.run());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "SecurityProbe",
		Methods: map[string]Method{
			"run": {Name: "SecurityProbe.run", ClassName: "SecurityProbe", ReturnType: "String", IsStatic: true, Access: "public", Program: run},
			"checkFieldIsReadable": {
				Name:       "SecurityProbe.checkFieldIsReadable",
				ClassName:  "SecurityProbe",
				ReturnType: "String",
				IsStatic:   true,
				Access:     "public",
				Params: []Param{
					{Name: "objType", Type: "SObjectType"},
					{Name: "fieldDescribe", Type: "DescribeFieldResult"},
				},
				Program: check,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeCallsInheritedMethodOnNestedStaticProperty(t *testing.T) {
	instanceGetter, err := CompileAnonymous("return new Child();")
	if err != nil {
		t.Fatal(err)
	}
	find, err := CompileAnonymous("return source;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals('SS', NestedParent.Instance.FindBatch('001000000000001AAA', 'SS', Date.newInstance(2026, 1, 1)));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "NestedParent",
		StaticFields: map[string]Field{
			"Instance": {Name: "Instance", Type: "NestedParent", Static: true, Access: "public", Getter: &Method{Name: "NestedParent.getInstance", ClassName: "NestedParent", ReturnType: "NestedParent", IsStatic: true, Access: "public", Program: instanceGetter}},
		},
		StaticFieldOrder: []string{"Instance"},
		Methods: map[string]Method{
			"FindBatch": {Name: "NestedParent.FindBatch", ClassName: "NestedParent", ReturnType: "String", Params: []Param{{Name: "entity", Type: "Id"}, {Name: "source", Type: "String"}, {Name: "transactionDate", Type: "Date"}}, Access: "public", Program: find},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "NestedParent.Child",
		SuperClass: "NestedParent",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeResolvesUnqualifiedNestedParameterTypes(t *testing.T) {
	validate, err := CompileAnonymous("return request.Name;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
NestedValidator validator = new NestedValidator();
NestedValidator.Request request = new NestedValidator.Request();
request.Name = 'Acme';
System.assertEquals('Acme', validator.validate(request));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "NestedValidator",
		Methods: map[string]Method{
			"validate": {Name: "NestedValidator.validate", ClassName: "NestedValidator", ReturnType: "String", Params: []Param{{Name: "request", Type: "Request"}}, Access: "public", Program: validate},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "NestedValidator.Request",
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSummaryFieldStoredStringPassesToDecimalParameter(t *testing.T) {
	accept, err := CompileAnonymous("return value;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`System.assertEquals(7, Harness.accept(orderRecord.GrandTotal__c));`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "NU"
	org.Objects["NU__Order__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "NU__Order__c",
			Fields: map[string]storage.Field{
				"NU__GrandTotal__c": {APIName: "NU__GrandTotal__c", Type: storage.FieldSummary, DisplayType: "SUMMARY"},
			},
		},
	}
	order := Object("Order__c")
	setExplicitSObjectField(&order, "GrandTotal__c", String("7.0"))
	machine := New(nil)
	machine.SetOrg(&org)
	machine.Globals["orderRecord"] = order
	machine.VarTypes["orderRecord"] = "Order__c"
	if err := machine.RegisterClass(Class{Name: "Harness"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:       "Harness.accept",
		ClassName:  "Harness",
		IsStatic:   true,
		ReturnType: "Decimal",
		Params:     []Param{{Name: "value", Type: "Decimal"}},
		Access:     "public",
		Program:    accept,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecBaseGetterIgnoresIncompatibleSubclassShadowField(t *testing.T) {
	getter, err := CompileAnonymous(`
if (mapping == null) {
    mapping = getMapping();
}
return mapping;
`)
	if err != nil {
		t.Fatal(err)
	}
	getMapping, err := CompileAnonymous(`
Map<Schema.SObjectField, Schema.SObjectField> out = new Map<Schema.SObjectField, Schema.SObjectField>();
for (String sourceField : this.mapping.keySet()) {
    out.put(Account.SObjectType.getDescribe().fields.getMap().get(sourceField),
            Account.SObjectType.getDescribe().fields.getMap().get(this.mapping.get(sourceField)));
}
return out;
`)
	if err != nil {
		t.Fatal(err)
	}
	ctor, err := CompileAnonymous(`this.mapping = new Map<String,String>{'Name' => 'Name'};`)
	if err != nil {
		t.Fatal(err)
	}
	resolve, err := CompileAnonymous(`
Map<Schema.SObjectField, Schema.SObjectField> resolved = mapping;
return resolved.get(Account.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
BaseShadow value = new ChildShadow();
System.assertEquals(Account.Name, value.resolve());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "BaseShadow",
		Fields: map[string]Field{
			"mapping": {Name: "mapping", Type: "Map<Schema.SObjectField,Schema.SObjectField>", Getter: &Method{Name: "BaseShadow.mapping.get", ClassName: "BaseShadow", ReturnType: "Map<Schema.SObjectField,Schema.SObjectField>", Access: "private", Program: getter}},
		},
		Methods: map[string]Method{
			"getMapping": {Name: "BaseShadow.getMapping", ClassName: "BaseShadow", ReturnType: "Map<Schema.SObjectField,Schema.SObjectField>", Access: "protected", Modifiers: []string{"abstract"}},
			"resolve":    {Name: "BaseShadow.resolve", ClassName: "BaseShadow", ReturnType: "Schema.SObjectField", Access: "public", Program: resolve},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "ChildShadow",
		SuperClass: "BaseShadow",
		Fields: map[string]Field{
			"mapping": {Name: "mapping", Type: "Map<String,String>", Access: "private"},
		},
		Constructors: []Method{{
			Name:      "ChildShadow.<init>",
			ClassName: "ChildShadow",
			Access:    "public",
			Program:   ctor,
		}},
		Methods: map[string]Method{
			"getMapping": {Name: "ChildShadow.getMapping", ClassName: "ChildShadow", ReturnType: "Map<Schema.SObjectField,Schema.SObjectField>", Access: "protected", Program: getMapping},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeNestedClassReadsOuterPrivateStaticField(t *testing.T) {
	read, err := CompileAnonymous("return TOKEN;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Outer.Inner inner = new Outer.Inner();
System.assertEquals('spruce', inner.read());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Outer",
		StaticFields: map[string]Field{
			"TOKEN": {Name: "TOKEN", Type: "String", Static: true, Access: "private", InitialValue: String("spruce")},
		},
		StaticFieldOrder: []string{"TOKEN"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Outer.Inner",
		Methods: map[string]Method{
			"read": {Name: "Outer.Inner.read", ClassName: "Outer.Inner", ReturnType: "String", Access: "public", Program: read},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeNestedClassCallsOuterPrivateStaticMethod(t *testing.T) {
	require, err := CompileAnonymous("return value;")
	if err != nil {
		t.Fatal(err)
	}
	run, err := CompileAnonymous("return require(value);")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Outer.Inner inner = new Outer.Inner();
System.assertEquals('spruce', inner.run('spruce'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Outer",
		Methods: map[string]Method{
			"require": {Name: "Outer.require", ClassName: "Outer", ReturnType: "Object", Params: []Param{{Name: "value", Type: "Object"}}, Access: "private", IsStatic: true, Program: require},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Outer.Inner",
		Methods: map[string]Method{
			"run": {Name: "Outer.Inner.run", ClassName: "Outer.Inner", ReturnType: "Object", Params: []Param{{Name: "value", Type: "Object"}}, Access: "public", Program: run},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeSiblingNestedClassReadsPrivateField(t *testing.T) {
	read, err := CompileAnonymous("return left.secret;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Outer.Left left = new Outer.Left();
Outer.Right right = new Outer.Right(left);
System.assertEquals('spruce', right.read());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	ctor, err := CompileAnonymous("this.left = left;")
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Outer"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Outer.Left",
		Fields:     map[string]Field{"secret": {Name: "secret", Type: "String", Access: "private", InitialValue: String("spruce")}},
		FieldOrder: []string{"secret"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Outer.Right",
		Fields: map[string]Field{
			"left": {Name: "left", Type: "Outer.Left", Access: "private"},
		},
		FieldOrder: []string{"left"},
		Methods: map[string]Method{
			"read": {Name: "Outer.Right.read", ClassName: "Outer.Right", ReturnType: "String", Access: "public", Program: read},
		},
		Constructors: []Method{{
			Name:      "Outer.Right",
			ClassName: "Outer.Right",
			Params:    []Param{{Name: "left", Type: "Outer.Left"}},
			Program:   ctor,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeAssignsThroughStaticFieldPath(t *testing.T) {
	run, err := CompileAnonymous(`
Api.v1.service = 'spruce';
System.assertEquals('spruce', Api.v1.service);
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("ApiTest.run();")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Api",
		StaticFields: map[string]Field{
			"v1": {Name: "v1", Type: "ApiV1", Static: true, Access: "global", InitialValue: Object("ApiV1")},
		},
		StaticFieldOrder: []string{"v1"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "ApiV1",
		Fields: map[string]Field{
			"service": {Name: "service", Type: "String", Access: "private", Modifiers: []string{"@TestVisible", "private"}},
		},
		FieldOrder: []string{"service"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:   "ApiTest",
		IsTest: true,
		Methods: map[string]Method{
			"run": {Name: "ApiTest.run", ClassName: "ApiTest", ReturnType: "void", IsStatic: true, Program: run},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeDispatchesUniqueConcreteOverrideForAbstractBaseValue(t *testing.T) {
	implProgram, err := CompileAnonymous("return 3;")
	if err != nil {
		t.Fatal(err)
	}
	run, err := CompileAnonymous(`
Base value = new Impl();
System.assertEquals(3, value.count());
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("PolyTest.run();")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "Base",
		IsAbstract: true,
		Methods: map[string]Method{
			"count": {Name: "Base.count", ClassName: "Base", ReturnType: "Integer", Modifiers: []string{"abstract"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Impl",
		SuperClass: "Base",
		Methods: map[string]Method{
			"count": {Name: "Impl.count", ClassName: "Impl", ReturnType: "Integer", Program: implProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "PolyTest",
		Methods: map[string]Method{
			"run": {Name: "PolyTest.run", ClassName: "PolyTest", ReturnType: "void", IsStatic: true, Program: run},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeDispatchesAbstractBaseValueByConcreteFields(t *testing.T) {
	firstProgram, err := CompileAnonymous("return 'first';")
	if err != nil {
		t.Fatal(err)
	}
	secondProgram, err := CompileAnonymous("return 'second';")
	if err != nil {
		t.Fatal(err)
	}
	run, err := CompileAnonymous(`
Base value = new SecondImpl();
System.assertEquals('second', value.name());
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("PolyTest.run();")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "Base",
		IsAbstract: true,
		Methods: map[string]Method{
			"name": {Name: "Base.name", ClassName: "Base", ReturnType: "String", Modifiers: []string{"abstract"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "FirstImpl",
		SuperClass: "Base",
		Fields:     map[string]Field{"firstField": {Name: "firstField", Type: "String", InitialValue: String("x")}},
		FieldOrder: []string{"firstField"},
		Methods: map[string]Method{
			"name": {Name: "FirstImpl.name", ClassName: "FirstImpl", ReturnType: "String", Program: firstProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "SecondImpl",
		SuperClass: "Base",
		Fields:     map[string]Field{"secondField": {Name: "secondField", Type: "String", InitialValue: String("x")}},
		FieldOrder: []string{"secondField"},
		Methods: map[string]Method{
			"name": {Name: "SecondImpl.name", ClassName: "SecondImpl", ReturnType: "String", Program: secondProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "PolyTest",
		Methods: map[string]Method{
			"run": {Name: "PolyTest.run", ClassName: "PolyTest", ReturnType: "void", IsStatic: true, Program: run},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeConstructorPrefersLexicalInnerClass(t *testing.T) {
	innerCtor, err := CompileAnonymous("this.value = value;")
	if err != nil {
		t.Fatal(err)
	}
	innerValue, err := CompileAnonymous("return value;")
	if err != nil {
		t.Fatal(err)
	}
	run, err := CompileAnonymous(`
ExchangeRateTest rate = new ExchangeRateTest(50);
System.assertEquals(50, rate.value());
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("ExchangeRatesApiV1ServiceImplTest.run();")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "ExchangeRateTest",
		Constructors: []Method{{
			Name:      "ExchangeRateTest",
			ClassName: "ExchangeRateTest",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "ExchangeRatesApiV1ServiceImplTest",
		Methods: map[string]Method{
			"run": {Name: "ExchangeRatesApiV1ServiceImplTest.run", ClassName: "ExchangeRatesApiV1ServiceImplTest", ReturnType: "void", IsStatic: true, Program: run},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "ExchangeRatesApiV1ServiceImplTest.ExchangeRateTest",
		Fields:     map[string]Field{"value": {Name: "value", Type: "Integer"}},
		FieldOrder: []string{"value"},
		Methods: map[string]Method{
			"value": {Name: "ExchangeRatesApiV1ServiceImplTest.ExchangeRateTest.value", ClassName: "ExchangeRatesApiV1ServiceImplTest.ExchangeRateTest", ReturnType: "Integer", Program: innerValue},
		},
		Constructors: []Method{{
			Name:      "ExchangeRatesApiV1ServiceImplTest.ExchangeRateTest",
			ClassName: "ExchangeRatesApiV1ServiceImplTest.ExchangeRateTest",
			Params:    []Param{{Name: "value", Type: "Integer"}},
			Program:   innerCtor,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateStubInterceptsStaticFactoryChain(t *testing.T) {
	cancelProgram, err := CompileAnonymous("return 'real';")
	if err != nil {
		t.Fatal(err)
	}
	factoryProgram, err := CompileAnonymous(`
if (mockInstance != null && Test.isRunningTest()) {
	return mockInstance;
}
return new Gateway();
`)
	if err != nil {
		t.Fatal(err)
	}
	providerProgram, err := CompileAnonymous("return 'mock';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Gateway.mockInstance = (Gateway)Test.createStub(Gateway.class, new Provider());
System.assertEquals('mock', Gateway.newInstance().cancel());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "Gateway",
		StaticFields: map[string]Field{
			"mockInstance": {Name: "mockInstance", Type: "Gateway", Static: true},
		},
		Methods: map[string]Method{
			"newInstance": {Name: "Gateway.newInstance", ClassName: "Gateway", ReturnType: "Gateway", IsStatic: true, Program: factoryProgram},
			"cancel":      {Name: "Gateway.cancel", ClassName: "Gateway", ReturnType: "String", Program: cancelProgram},
		},
		Constructors: []Method{{Name: "Gateway.<init>", ClassName: "Gateway", ReturnType: "void", IsConstructor: true}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"StubProvider"},
		Methods: map[string]Method{
			"handleMethodCall": {
				Name:       "Provider.handleMethodCall",
				ClassName:  "Provider",
				ReturnType: "Object",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateStubStaticFactorySurvivesStopTestAsync(t *testing.T) {
	cancelProgram, err := CompileAnonymous("return 'real';")
	if err != nil {
		t.Fatal(err)
	}
	factoryProgram, err := CompileAnonymous(`
if (mockInstance != null && Test.isRunningTest()) {
	return mockInstance;
}
return new Gateway();
`)
	if err != nil {
		t.Fatal(err)
	}
	providerProgram, err := CompileAnonymous("return 'mock';")
	if err != nil {
		t.Fatal(err)
	}
	jobProgram, err := CompileAnonymous(`
insert new Account(Name = Gateway.newInstance().cancel());
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Gateway.mockInstance = (Gateway)Test.createStub(Gateway.class, new Provider());
Test.startTest();
System.enqueueJob(new GatewayJob());
Test.stopTest();
List<Account> accounts = [SELECT Id, Name FROM Account];
System.assertEquals(1, accounts.size());
System.assertEquals('mock', accounts.get(0).Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "Gateway",
		StaticFields: map[string]Field{
			"mockInstance": {Name: "mockInstance", Type: "Gateway", Static: true},
		},
		Methods: map[string]Method{
			"newInstance": {Name: "Gateway.newInstance", ClassName: "Gateway", ReturnType: "Gateway", IsStatic: true, Program: factoryProgram},
			"cancel":      {Name: "Gateway.cancel", ClassName: "Gateway", ReturnType: "String", Program: cancelProgram},
		},
		Constructors: []Method{{Name: "Gateway.<init>", ClassName: "Gateway", ReturnType: "void", IsConstructor: true}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "GatewayJob",
		Interfaces: []string{"Queueable"},
		Methods: map[string]Method{
			"execute": {
				Name:       "GatewayJob.execute",
				ClassName:  "GatewayJob",
				ReturnType: "void",
				Params:     []Param{{Name: "context", Type: "QueueableContext"}},
				Program:    jobProgram,
			},
		},
		Constructors: []Method{{Name: "GatewayJob.<init>", ClassName: "GatewayJob", ReturnType: "void", IsConstructor: true}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"StubProvider"},
		Methods: map[string]Method{
			"handleMethodCall": {
				Name:       "Provider.handleMethodCall",
				ClassName:  "Provider",
				ReturnType: "Object",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTestCreateStubStaticFactoryInterceptsMapValuesListSObject(t *testing.T) {
	instanceGetter, err := CompileAnonymous(`
if (mockInstance != null && Test.isRunningTest()) {
	return mockInstance;
}
return new DML();
`)
	if err != nil {
		t.Fatal(err)
	}
	updateProgram, err := CompileAnonymous("return null;")
	if err != nil {
		t.Fatal(err)
	}
	providerProgram, err := CompileAnonymous(`
if (calls == null) {
	calls = 0;
}
calls = calls + 1;
paramType = String.valueOf(listOfParamTypes[0]);
return new List<Database.SaveResult>();
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
DML.mockInstance = (DML)Test.createStub(DML.class, new Provider());
Map<Id, Account> accountsById = new Map<Id, Account>();
accountsById.put('001000000000001', new Account(Id = '001000000000001'));
DML.Instance.updateRecords(accountsById.values());
System.assertEquals(1, Provider.calls);
System.assertEquals('List<SObject>', Provider.paramType);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "DML",
		StaticFields: map[string]Field{
			"mockInstance": {Name: "mockInstance", Type: "DML", Static: true},
			"Instance":     {Name: "Instance", Type: "DML", Static: true, Getter: &Method{Name: "DML.Instance", ClassName: "DML", ReturnType: "DML", IsStatic: true, Program: instanceGetter}},
		},
		Methods: map[string]Method{
			"updateRecords": {Name: "DML.updateRecords", ClassName: "DML", ReturnType: "List<Database.SaveResult>", Params: []Param{{Name: "records", Type: "List<SObject>"}}, Program: updateProgram},
		},
		Constructors: []Method{{Name: "DML.<init>", ClassName: "DML", ReturnType: "void", IsConstructor: true}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"StubProvider"},
		StaticFields: map[string]Field{
			"calls":     {Name: "calls", Type: "Integer", Static: true},
			"paramType": {Name: "paramType", Type: "String", Static: true},
		},
		Methods: map[string]Method{
			"handleMethodCall": {
				Name:       "Provider.handleMethodCall",
				ClassName:  "Provider",
				ReturnType: "Object",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUserMethodDatabaseInsertPropagatesSObjectIDToCaller(t *testing.T) {
	insertProgram, err := CompileAnonymous(`return Database.insert(record);`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
new DMLWrapper().insertRecords(account);
System.assertNotEquals(null, account.Id);
Account stored = [SELECT Id FROM Account WHERE Id = :account.Id];
System.assertEquals(account.Id, stored.Id);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "DMLWrapper",
		Methods: map[string]Method{
			"insertRecords": {
				Name:       "DMLWrapper.insertRecords",
				ClassName:  "DMLWrapper",
				ReturnType: "Database.SaveResult",
				Params:     []Param{{Name: "record", Type: "SObject"}},
				Program:    insertProgram,
			},
		},
		Constructors: []Method{{Name: "DMLWrapper.<init>", ClassName: "DMLWrapper", ReturnType: "void", IsConstructor: true}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMapGetUsesCustomEqualsForObjectKeys(t *testing.T) {
	keyCtor, err := CompileAnonymous(`
this.typeName = typeName;
this.methodName = methodName;
this.methodArgTypes = methodArgTypes;
this.mockInstance = mockInstance;
`)
	if err != nil {
		t.Fatal(err)
	}
	equalsProgram, err := CompileAnonymous(`
if (this === other) {
	return true;
}
Key that = other instanceof Key ? (Key)other : null;
return that != null
	&& this.mockInstance === that.mockInstance
	&& this.typeName == that.typeName
	&& this.methodName == that.methodName
	&& this.methodArgTypes == that.methodArgTypes;
`)
	if err != nil {
		t.Fatal(err)
	}
	hashProgram, err := CompileAnonymous(`
Integer prime = 31;
Integer result = 1;
result = prime * result + ((mockInstance == null) ? 0 : mockInstance.hashCode());
result = prime * result + ((methodArgTypes == null) ? 0 : methodArgTypes.hashCode());
result = prime * result + ((methodName == null) ? 0 : methodName.hashCode());
result = prime * result + ((typeName == null) ? 0 : typeName.hashCode());
return result;
`)
	if err != nil {
		t.Fatal(err)
	}
	providerProgram, err := CompileAnonymous("return null;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Gateway mock = (Gateway)Test.createStub(Gateway.class, new Provider());
List<Type> firstTypes = new List<Type>{Account.class};
List<Type> secondTypes = new List<Type>{Account.class};
Key first = new Key('Gateway', 'updateRecords', firstTypes, mock);
Key second = new Key('Gateway', 'updateRecords', secondTypes, mock);
Map<Key, String> values = new Map<Key, String>();
values.put(first, 'hit');
System.assertEquals('hit', values.get(second));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "Gateway",
		Methods: map[string]Method{
			"updateRecords": {Name: "Gateway.updateRecords", ClassName: "Gateway", ReturnType: "void", Params: []Param{{Name: "records", Type: "List<SObject>"}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Key",
		Fields: map[string]Field{
			"typeName":       {Name: "typeName", Type: "String"},
			"methodName":     {Name: "methodName", Type: "String"},
			"methodArgTypes": {Name: "methodArgTypes", Type: "List<Type>"},
			"mockInstance":   {Name: "mockInstance", Type: "Object"},
		},
		Constructors: []Method{{
			Name:      "Key.<init>",
			ClassName: "Key",
			Params: []Param{
				{Name: "typeName", Type: "String"},
				{Name: "methodName", Type: "String"},
				{Name: "methodArgTypes", Type: "List<Type>"},
				{Name: "mockInstance", Type: "Object"},
			},
			Program: keyCtor,
		}},
		Methods: map[string]Method{
			"equals":   {Name: "Key.equals", ClassName: "Key", ReturnType: "Boolean", Params: []Param{{Name: "other", Type: "Object"}}, Program: equalsProgram},
			"hashCode": {Name: "Key.hashCode", ClassName: "Key", ReturnType: "Integer", Program: hashProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Provider",
		Interfaces: []string{"StubProvider"},
		Methods: map[string]Method{
			"handleMethodCall": {
				Name:       "Provider.handleMethodCall",
				ClassName:  "Provider",
				ReturnType: "Object",
				Params: []Param{
					{Name: "stubbedObject", Type: "Object"},
					{Name: "stubbedMethodName", Type: "String"},
					{Name: "returnType", Type: "Type"},
					{Name: "listOfParamTypes", Type: "List<Type>"},
					{Name: "listOfParamNames", Type: "List<String>"},
					{Name: "listOfArgs", Type: "List<Object>"},
				},
				Program: providerProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAsyncDrainRestoresStaticInitializationState(t *testing.T) {
	initProgram, err := CompileAnonymous("AsyncConstants.Query = 'SELECT Id FROM Account';")
	if err != nil {
		t.Fatal(err)
	}
	getProgram, err := CompileAnonymous("return Query;")
	if err != nil {
		t.Fatal(err)
	}
	jobProgram, err := CompileAnonymous("String query = AsyncConstants.getQuery();")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Test.startTest();
System.enqueueJob(new StaticInitJob());
Test.stopTest();
System.assertEquals('SELECT Id FROM Account', AsyncConstants.getQuery());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "AsyncConstants",
		StaticFields: map[string]Field{
			"Query": {Name: "Query", Type: "String", Static: true},
		},
		StaticFieldOrder: []string{"Query"},
		StaticInitializers: []Method{{
			Name:      "AsyncConstants.<static_init>",
			ClassName: "AsyncConstants",
			IsStatic:  true,
			Program:   initProgram,
		}},
		Methods: map[string]Method{
			"getQuery": {Name: "AsyncConstants.getQuery", ClassName: "AsyncConstants", ReturnType: "String", IsStatic: true, Program: getProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "StaticInitJob",
		Interfaces: []string{"Queueable"},
		Methods: map[string]Method{
			"execute": {
				Name:       "StaticInitJob.execute",
				ClassName:  "StaticInitJob",
				ReturnType: "void",
				Params:     []Param{{Name: "context", Type: "QueueableContext"}},
				Program:    jobProgram,
			},
		},
		Constructors: []Method{{Name: "StaticInitJob.<init>", ClassName: "StaticInitJob", ReturnType: "void", IsConstructor: true}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAsyncDrainRestoresStaticCollectionInitializationState(t *testing.T) {
	initProgram, err := CompileAnonymous("AsyncRegistry.Ids = new Set<String>{'a', 'b'};")
	if err != nil {
		t.Fatal(err)
	}
	sizeProgram, err := CompileAnonymous("return Ids.size();")
	if err != nil {
		t.Fatal(err)
	}
	jobProgram, err := CompileAnonymous("String marker = 'noop';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals(2, AsyncRegistry.size());
Test.startTest();
System.enqueueJob(new StaticCollectionJob());
Test.stopTest();
System.assertEquals(2, AsyncRegistry.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "AsyncRegistry",
		StaticFields: map[string]Field{
			"Ids": {Name: "Ids", Type: "Set<String>", Static: true},
		},
		StaticFieldOrder: []string{"Ids"},
		StaticInitializers: []Method{{
			Name:      "AsyncRegistry.<static_init>",
			ClassName: "AsyncRegistry",
			IsStatic:  true,
			Program:   initProgram,
		}},
		Methods: map[string]Method{
			"size": {Name: "AsyncRegistry.size", ClassName: "AsyncRegistry", ReturnType: "Integer", IsStatic: true, Program: sizeProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "StaticCollectionJob",
		Interfaces: []string{"Queueable"},
		Methods: map[string]Method{
			"execute": {
				Name:       "StaticCollectionJob.execute",
				ClassName:  "StaticCollectionJob",
				ReturnType: "void",
				Params:     []Param{{Name: "context", Type: "QueueableContext"}},
				Program:    jobProgram,
			},
		},
		Constructors: []Method{{Name: "StaticCollectionJob.<init>", ClassName: "StaticCollectionJob", ReturnType: "void", IsConstructor: true}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeConstructorResolvesInheritedInnerClass(t *testing.T) {
	innerCtor, err := CompileAnonymous("this.value = value;")
	if err != nil {
		t.Fatal(err)
	}
	innerValue, err := CompileAnonymous("return value;")
	if err != nil {
		t.Fatal(err)
	}
	run, err := CompileAnonymous("return new Relationship(7).value();")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("System.assertEquals(7, ChildDefinition.run());")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "BaseDefinition"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "BaseDefinition.Relationship",
		Fields:     map[string]Field{"value": {Name: "value", Type: "Integer"}},
		FieldOrder: []string{"value"},
		Methods: map[string]Method{
			"value": {Name: "BaseDefinition.Relationship.value", ClassName: "BaseDefinition.Relationship", ReturnType: "Integer", Program: innerValue},
		},
		Constructors: []Method{{
			Name:      "BaseDefinition.Relationship",
			ClassName: "BaseDefinition.Relationship",
			Params:    []Param{{Name: "value", Type: "Integer"}},
			Program:   innerCtor,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "ChildDefinition",
		SuperClass: "BaseDefinition",
		Methods: map[string]Method{
			"run": {Name: "ChildDefinition.run", ClassName: "ChildDefinition", ReturnType: "Integer", IsStatic: true, Program: run},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeRejectsRecursiveChainedConstructor(t *testing.T) {
	ctor, err := CompileAnonymous("this(value);")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("Loopy l = new Loopy('spruce');")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Loopy",
		Constructors: []Method{{
			Name:          "Loopy.<init>",
			ClassName:     "Loopy",
			IsConstructor: true,
			Params:        []Param{{Name: "value", Type: "String"}},
			Program:       ctor,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "recursive constructor invocation Loopy.<init>") {
		t.Fatalf("err = %v, want recursive constructor invocation", err)
	}
}

func TestRuntimeReturnsAlreadyTypedMapWithObjectKeys(t *testing.T) {
	record, err := CompileAnonymous("store.put(key, new List<String>{'spruce'});")
	if err != nil {
		t.Fatal(err)
	}
	get, err := CompileAnonymous("return store;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Key key = new Key();
Holder.record(key);
Map<Key, List<String>> store = Holder.getStore();
System.assertEquals('spruce', store.get(key).get(0));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Key"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Holder",
		StaticFields: map[string]Field{
			"store": {Name: "store", Type: "Map<Key,List<String>>", Static: true, Access: "private", InitialValue: typedMap("Map<Key,List<String>>")},
		},
		StaticFieldOrder: []string{"store"},
		Methods: map[string]Method{
			"record":   {Name: "Holder.record", ClassName: "Holder", Params: []Param{{Name: "key", Type: "Key"}}, Access: "public", IsStatic: true, Program: record},
			"getStore": {Name: "Holder.getStore", ClassName: "Holder", ReturnType: "Map<Key,List<String>>", Access: "public", IsStatic: true, Program: get},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeRejectsPrivateFieldAccessAcrossClasses(t *testing.T) {
	program, err := CompileAnonymous(`
Secret s = new Secret();
String value = s.code;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "Secret",
		Fields:     map[string]Field{"code": {Name: "code", Type: "String", Access: "private", InitialValue: String("x")}},
		FieldOrder: []string{"code"},
	}); err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "Secret.code is private") {
		t.Fatalf("err = %v, want private field visibility error", err)
	}
}

func TestRuntimeAllowsProtectedAccessThroughInheritanceChain(t *testing.T) {
	guarded, err := CompileAnonymous("return 'guarded';")
	if err != nil {
		t.Fatal(err)
	}
	run, err := CompileAnonymous("return guarded();")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Leaf leaf = new Leaf();
System.assertEquals('guarded', leaf.run());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Root",
		Methods: map[string]Method{
			"guarded": {Name: "Root.guarded", ClassName: "Root", ReturnType: "String", Access: "protected", Program: guarded},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Middle", SuperClass: "Root"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Leaf",
		SuperClass: "Middle",
		Methods: map[string]Method{
			"run": {Name: "Leaf.run", ClassName: "Leaf", ReturnType: "String", Program: run},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeAllowsProtectedAccessBetweenNestedSiblingClasses(t *testing.T) {
	guarded, err := CompileAnonymous("return 'guarded';")
	if err != nil {
		t.Fatal(err)
	}
	run, err := CompileAnonymous("return item.guarded();")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Outer.Runner runner = new Outer.Runner();
System.assertEquals('guarded', runner.run(new Outer.Item()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Outer"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Outer.Item",
		Methods: map[string]Method{
			"guarded": {Name: "Outer.Item.guarded", ClassName: "Outer.Item", ReturnType: "String", Access: "protected", Program: guarded},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Outer.Runner",
		Methods: map[string]Method{
			"run": {Name: "Outer.Runner.run", ClassName: "Outer.Runner", Params: []Param{{Name: "item", Type: "Outer.Item"}}, ReturnType: "String", Program: run},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeAllowsSuperclassMethodToDispatchProtectedOverride(t *testing.T) {
	run, err := CompileAnonymous("return token();")
	if err != nil {
		t.Fatal(err)
	}
	token, err := CompileAnonymous("return 'child';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Child child = new Child();
System.assertEquals('child', child.run());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Base",
		Methods: map[string]Method{
			"run": {Name: "Base.run", ClassName: "Base", ReturnType: "String", Program: run},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Child",
		SuperClass: "Base",
		Methods: map[string]Method{
			"token": {Name: "Child.token", ClassName: "Child", ReturnType: "String", Access: "protected", Program: token},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeChecksVirtualDispatchAccessAtVisibleSurface(t *testing.T) {
	run, err := CompileAnonymous("return hook();")
	if err != nil {
		t.Fatal(err)
	}
	hook, err := CompileAnonymous("return 'child';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Child child = new Child();
Base base = child;
System.assertEquals('child', base.run());
System.assertEquals('child', child.run());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Base",
		Methods: map[string]Method{
			"run":  {Name: "Base.run", ClassName: "Base", ReturnType: "String", Program: run},
			"hook": {Name: "Base.hook", ClassName: "Base", ReturnType: "String", Modifiers: []string{"abstract"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Child",
		SuperClass: "Base",
		Methods: map[string]Method{
			"hook": {Name: "Child.hook", ClassName: "Child", ReturnType: "String", Access: "private", Program: hook},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeMatchesMapOverloadByEntries(t *testing.T) {
	accept, err := CompileAnonymous("return values.get('name');")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Map<String,Object> values = new Map<String,Object>();
values.put('name', 'trail');
System.assertEquals('trail', Accept.take(values));
System.assertEquals(null, Accept.take(new Map<String,Object>()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Accept",
		Methods: map[string]Method{
			"take": {
				Name:       "Accept.take",
				ClassName:  "Accept",
				IsStatic:   true,
				ReturnType: "String",
				Params:     []Param{{Name: "values", Type: "Map<String,String>"}},
				Program:    accept,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeAllowsTestVisiblePrivateMethodFromTestClass(t *testing.T) {
	visible, err := CompileAnonymous("return 'visible';")
	if err != nil {
		t.Fatal(err)
	}
	run, err := CompileAnonymous("return Secret.visibleForTests();")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("System.assertEquals('visible', SecretTest.run());")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Secret",
		Methods: map[string]Method{
			"visibleForTests": {
				Name:       "Secret.visibleForTests",
				ClassName:  "Secret",
				ReturnType: "String",
				Access:     "private",
				Modifiers:  []string{"@TestVisible", "private", "static"},
				IsStatic:   true,
				Program:    visible,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:   "SecretTest",
		IsTest: true,
		Methods: map[string]Method{
			"run": {Name: "SecretTest.run", ClassName: "SecretTest", ReturnType: "String", IsStatic: true, Program: run},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeNamespaceRequiresGlobalAcrossBoundary(t *testing.T) {
	publicProgram, err := CompileAnonymous("return 'public';")
	if err != nil {
		t.Fatal(err)
	}
	globalProgram, err := CompileAnonymous("return 'global';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals('global', pkg.Secret.glob());
String value = pkg.Secret.pub();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:      "Secret",
		Namespace: "pkg",
		Access:    "global",
		Methods: map[string]Method{
			"pub":  {Name: "Secret.pub", ClassName: "Secret", ReturnType: "String", Access: "public", IsStatic: true, Program: publicProgram},
			"glob": {Name: "Secret.glob", ClassName: "Secret", ReturnType: "String", Access: "global", IsStatic: true, Program: globalProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "not global") {
		t.Fatalf("err = %v, want namespace visibility error", err)
	}
}

func TestRuntimeAllowsPublicAccessInsideCurrentNamespace(t *testing.T) {
	helperProgram, err := CompileAnonymous("return 'public';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("System.assertEquals('public', Secret.pub());")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:      "Secret",
		Namespace: "pkg",
		Access:    "public",
		Methods: map[string]Method{
			"pub": {Name: "Secret.pub", ClassName: "Secret", ReturnType: "String", Access: "public", IsStatic: true, Program: helperProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:      "SecretTest",
		Namespace: "pkg",
		Access:    "public",
		IsTest:    true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.ExecuteInClass(program, "SecretTest"); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeConstructsNamespaceQualifiedClass(t *testing.T) {
	program, err := CompileAnonymous(`
pkg.Box box = new pkg.Box();
System.assertEquals('Box', box.kind);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "Box",
		Namespace:  "pkg",
		Access:     "global",
		Fields:     map[string]Field{"kind": {Name: "kind", Type: "String", Access: "global", InitialValue: String("Box")}},
		FieldOrder: []string{"kind"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeChoosesOverloadByArgumentTypes(t *testing.T) {
	intProgram, err := CompileAnonymous("return 'int';")
	if err != nil {
		t.Fatal(err)
	}
	stringProgram, err := CompileAnonymous("return 'string';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals('int', Util.pick(1));
System.assertEquals('string', Util.pick('one'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "Util.pick",
		ReturnType: "String",
		Params:     []Param{{Name: "value", Type: "Integer"}},
		Program:    intProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:       "Util.pick",
		ReturnType: "String",
		Params:     []Param{{Name: "value", Type: "String"}},
		Program:    stringProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeStringDoesNotCoerceToArbitraryClass(t *testing.T) {
	machine := New(nil)
	if _, err := machine.coerceAssignable("CurrencyBase", String("USD")); err == nil {
		t.Fatal("expected String to CurrencyBase coercion to fail")
	}
}

func TestRuntimeNamespaceRequiresGlobalClassAcrossBoundary(t *testing.T) {
	globalProgram, err := CompileAnonymous("return 'global';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("String value = pkg.Secret.glob();")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:      "Secret",
		Namespace: "pkg",
		Access:    "public",
		Methods: map[string]Method{
			"glob": {Name: "Secret.glob", ClassName: "Secret", ReturnType: "String", Access: "global", IsStatic: true, Program: globalProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "not global") {
		t.Fatalf("err = %v, want namespace class visibility error", err)
	}
}

func TestRuntimeNamespaceRequiresGlobalClassForConstruction(t *testing.T) {
	program, err := CompileAnonymous("pkg.Box box = new pkg.Box();")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:      "Box",
		Namespace: "pkg",
		Access:    "public",
	}); err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "not global") {
		t.Fatalf("err = %v, want namespace constructor visibility error", err)
	}
}

func TestRuntimeNumericOverloadSpecificityBaseline(t *testing.T) {
	intProgram, err := CompileAnonymous("return 'integer';")
	if err != nil {
		t.Fatal(err)
	}
	decimalProgram, err := CompileAnonymous("return 'decimal';")
	if err != nil {
		t.Fatal(err)
	}
	objectProgram, err := CompileAnonymous("return 'object';")
	if err != nil {
		t.Fatal(err)
	}
	longProgram, err := CompileAnonymous("return 'long';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals('integer', Util.pick(1));
System.assertEquals('decimal', Util.pick(1.5));
System.assertEquals('long', Util.onlyLong(1));
System.assertEquals('object', Util.pick(true));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, method := range []Method{
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "value", Type: "Object"}}, Program: objectProgram},
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "value", Type: "Decimal"}}, Program: decimalProgram},
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "value", Type: "Integer"}}, Program: intProgram},
		{Name: "Util.onlyLong", ReturnType: "String", Params: []Param{{Name: "value", Type: "Long"}}, Program: longProgram},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeTypedNullFieldGuidesOverloadResolution(t *testing.T) {
	listProgram, err := CompileAnonymous("return 'list';")
	if err != nil {
		t.Fatal(err)
	}
	objectProgram, err := CompileAnonymous("return 'object';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Holder holder = new Holder();
System.assertEquals('list', Util.pick(holder.records));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Holder", Fields: map[string]Field{
		"records": {Name: "records", Type: "List<Account>"},
	}}); err != nil {
		t.Fatal(err)
	}
	for _, method := range []Method{
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "value", Type: "Object"}}, Program: objectProgram},
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "value", Type: "List<SObject>"}}, Program: listProgram},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeTypedNullCollectionDowncastGuidesOverloadResolution(t *testing.T) {
	concreteProgram, err := CompileAnonymous("return 'concrete';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<SObject> records = null;
System.assertEquals('concrete', Util.pick(records));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, method := range []Method{
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "value", Type: "List<Account>"}}, Program: concreteProgram},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeNumericOverloadChoosesNarrowestWidening(t *testing.T) {
	longProgram, err := CompileAnonymous("return 'long';")
	if err != nil {
		t.Fatal(err)
	}
	decimalProgram, err := CompileAnonymous("return 'decimal';")
	if err != nil {
		t.Fatal(err)
	}
	doubleProgram, err := CompileAnonymous("return 'double';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals('long', Util.pick(1));
System.assertEquals('decimal', Util.pickDecimal(1));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, method := range []Method{
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "value", Type: "Double"}}, Program: doubleProgram},
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "value", Type: "Decimal"}}, Program: decimalProgram},
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "value", Type: "Long"}}, Program: longProgram},
		{Name: "Util.pickDecimal", ReturnType: "String", Params: []Param{{Name: "value", Type: "Double"}}, Program: doubleProgram},
		{Name: "Util.pickDecimal", ReturnType: "String", Params: []Param{{Name: "value", Type: "Decimal"}}, Program: decimalProgram},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeObjectOverloadChoosesNearestAncestor(t *testing.T) {
	parentProgram, err := CompileAnonymous("return 'parent';")
	if err != nil {
		t.Fatal(err)
	}
	rootProgram, err := CompileAnonymous("return 'root';")
	if err != nil {
		t.Fatal(err)
	}
	objectProgram, err := CompileAnonymous("return 'object';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("System.assertEquals('parent', Util.pick(new Child()));")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, class := range []Class{
		{Name: "Root"},
		{Name: "Parent", SuperClass: "Root"},
		{Name: "Child", SuperClass: "Parent"},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	for _, method := range []Method{
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "value", Type: "Object"}}, Program: objectProgram},
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "value", Type: "Root"}}, Program: rootProgram},
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "value", Type: "Parent"}}, Program: parentProgram},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeOverloadUsesPairwiseSpecificity(t *testing.T) {
	firstProgram, err := CompileAnonymous("return 'first';")
	if err != nil {
		t.Fatal(err)
	}
	secondProgram, err := CompileAnonymous("return 'second';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("Util.pick(1, 'one');")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, method := range []Method{
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "count", Type: "Integer"}, {Name: "label", Type: "Object"}}, Program: firstProgram},
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "count", Type: "Long"}, {Name: "label", Type: "String"}}, Program: secondProgram},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "ambiguous overload") {
		t.Fatalf("expected ambiguous overload error, got %v", err)
	}
	for _, want := range []string{
		"argument types: 1:Integer, 2:String",
		"Util.pick(count Integer, label Object)",
		"Util.pick(count Long, label String)",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ambiguous overload error = %q, want %q", err.Error(), want)
		}
	}
}

func TestRuntimeNullOverloadChoosesMostSpecificType(t *testing.T) {
	stringProgram, err := CompileAnonymous("return 'string';")
	if err != nil {
		t.Fatal(err)
	}
	objectProgram, err := CompileAnonymous("return 'object';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("System.assertEquals('string', Util.pick(null));")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, method := range []Method{
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "value", Type: "Object"}}, Program: objectProgram},
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "value", Type: "String"}}, Program: stringProgram},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeTypedRelationshipNullSelectsSObjectOverload(t *testing.T) {
	accountProgram, err := CompileAnonymous("return 'account';")
	if err != nil {
		t.Fatal(err)
	}
	decimalProgram, err := CompileAnonymous("return 'decimal';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Child__c child = new Child__c();
System.assertEquals('account', Util.pick(child.Parent__r));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Child__c": {
			Definition: storage.ObjectDefinition{
				APIName: "Child__c",
				Fields: map[string]storage.Field{
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Parent__r"},
				},
				Relations: []storage.Relationship{{
					Field:              "Parent__c",
					ParentObjects:      []string{"Account"},
					ParentRelationship: "Parent__r",
				}},
			},
		},
		"Account": {
			Definition: storage.ObjectDefinition{
				APIName: "Account",
				Fields:  map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}},
			},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	for _, method := range []Method{
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "value", Type: "Account"}}, Program: accountProgram},
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "value", Type: "Decimal"}}, Program: decimalProgram},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeRejectsNonVoidMethodFallthrough(t *testing.T) {
	noReturnProgram, err := CompileAnonymous("Integer value = 1;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("Missing.value();")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "Missing.value",
		ReturnType: "Integer",
		Program:    noReturnProgram,
	}); err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "Missing.value must return Integer") {
		t.Fatalf("error = %v", err)
	}
}

func TestExecStaticFieldsAndInheritanceDispatch(t *testing.T) {
	parentProgram, err := CompileAnonymous("return 1;")
	if err != nil {
		t.Fatal(err)
	}
	childProgram, err := CompileAnonymous("return super.score() + bonus;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Child.bonus = 4;
Child c = new Child();
System.assertEquals(5, c.score());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Parent",
		Methods: map[string]Method{
			"score": {Name: "Parent.score", ClassName: "Parent", ReturnType: "Integer", Program: parentProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Child",
		SuperClass: "Parent",
		StaticFields: map[string]Field{
			"bonus": {Name: "bonus", Type: "Integer", Static: true, Value: Int(0)},
		},
		Methods: map[string]Method{
			"score": {Name: "Child.score", ClassName: "Child", ReturnType: "Integer", Program: childProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStaticFieldAssignmentUsesCanonicalCaseInsensitiveName(t *testing.T) {
	program, err := CompileAnonymous(`
Config.MockService = 'member';
System.assertEquals('member', Config.mockService);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Config",
		StaticFields: map[string]Field{
			"mockService": {Name: "mockService", Type: "String", Static: true},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	class := machine.Classes["Config"]
	if _, ok := class.StaticFields["MockService"]; ok {
		t.Fatalf("static assignment created duplicate case-variant field: %#v", class.StaticFields)
	}
}

func TestExecStaticPropertySingletonDispatchesInheritedMethod(t *testing.T) {
	getterProgram, err := CompileAnonymous(`
if (Instance == null) {
    Instance = new ConcreteManager();
}
return Instance;
`)
	if err != nil {
		t.Fatal(err)
	}
	idProgram, err := CompileAnonymous("return 'ok';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals('ok', Manager.Instance.identifier());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "Manager",
		IsAbstract: true,
		StaticFields: map[string]Field{
			"Instance": {
				Name:     "Instance",
				Type:     "Manager",
				Static:   true,
				Property: true,
				Getter:   &Method{Name: "Manager.Instance.get", ClassName: "Manager", ReturnType: "Manager", IsStatic: true, Program: getterProgram},
			},
		},
		Methods: map[string]Method{
			"identifier": {Name: "Manager.identifier", ClassName: "Manager", ReturnType: "String", Program: idProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Manager.ConcreteManager", SuperClass: "Manager"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStaticPropertySingletonDispatchesMethodAfterGetterInitialization(t *testing.T) {
	getterProgram, err := CompileAnonymous(`
if (Instance == null) {
    Instance = new ConcreteManager();
}
String ignored = Instance.identifier();
return Instance;
`)
	if err != nil {
		t.Fatal(err)
	}
	idProgram, err := CompileAnonymous("return 'ok';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals('ok', Manager.Instance.identifier());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "Manager",
		IsAbstract: true,
		StaticFields: map[string]Field{
			"Instance": {
				Name:     "Instance",
				Type:     "Manager",
				Static:   true,
				Property: true,
				Getter:   &Method{Name: "Manager.Instance.get", ClassName: "Manager", ReturnType: "Manager", IsStatic: true, Program: getterProgram},
			},
		},
		Methods: map[string]Method{
			"identifier": {Name: "Manager.identifier", ClassName: "Manager", ReturnType: "String", Program: idProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Manager.ConcreteManager", SuperClass: "Manager"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInheritedMethodPreservesStaticListCacheWithFreshFactoryInstance(t *testing.T) {
	getterProgram, err := CompileAnonymous(`
if (Instance == null || Test.isRunningTest()) {
    Instance = new Manager.Child();
}
return Instance;
`)
	if err != nil {
		t.Fatal(err)
	}
	rowsProgram, err := CompileAnonymous(`
if (cachedRows == null) {
    cachedRows = [SELECT Id FROM Account];
}
return cachedRows;
`)
	if err != nil {
		t.Fatal(err)
	}
	childRowsProgram, err := CompileAnonymous("return super.rows();")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Manager.Instance.rows();
Manager.Instance.rows();
System.assertEquals(1, Limits.getQueries());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name:       "Manager",
		IsAbstract: true,
		StaticFields: map[string]Field{
			"Instance": {
				Name:     "Instance",
				Type:     "Manager",
				Static:   true,
				Property: true,
				Getter:   &Method{Name: "Manager.Instance.get", ClassName: "Manager", ReturnType: "Manager", IsStatic: true, Program: getterProgram},
			},
			"cachedRows": {Name: "cachedRows", Type: "List<Account>", Static: true},
		},
		Methods: map[string]Method{
			"rows": {Name: "Manager.rows", ClassName: "Manager", ReturnType: "List<Account>", Program: rowsProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Manager.Child",
		SuperClass: "Manager",
		Methods: map[string]Method{
			"rows": {Name: "Manager.Child.rows", ClassName: "Manager.Child", ReturnType: "List<Account>", Program: childRowsProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLIDSetDoesNotReturnSiblingObjectRows(t *testing.T) {
	program, err := CompileAnonymous(`
Set<Id> eventIds = new Set<Id>{'a00000000000001AAA'};
System.assertEquals(0, [SELECT Id FROM Account WHERE Id IN :eventIds].size());
List<Account> dynamicRows = Database.query('SELECT Id FROM Account WHERE Id IN :eventIds');
System.assertEquals(0, dynamicRows.size());
System.assertEquals(1, [SELECT Id FROM Event__c WHERE Id IN :eventIds].size());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Id": {APIName: "Id", Type: storage.FieldID},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["Event__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Event__c",
			Fields: map[string]storage.Field{
				"Id": {APIName: "Id", Type: storage.FieldID},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {ID: "a00000000000001", Object: "Event__c"},
		},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSiblingStaticPropertySingletonsKeepInheritedDispatchSeparate(t *testing.T) {
	baseRowsProgram, err := CompileAnonymous("return queryRows(ids);")
	if err != nil {
		t.Fatal(err)
	}
	baseQueryProgram, err := CompileAnonymous("return new List<SObject>();")
	if err != nil {
		t.Fatal(err)
	}
	accountGetterProgram, err := CompileAnonymous(`
if (Instance == null || Test.isRunningTest()) {
    Instance = WithSharingInstance;
}
return Instance;
`)
	if err != nil {
		t.Fatal(err)
	}
	accountSharingGetterProgram, err := CompileAnonymous(`
if (WithSharingInstance == null) {
    WithSharingInstance = new AccountManager.WithSharing();
}
return WithSharingInstance;
`)
	if err != nil {
		t.Fatal(err)
	}
	accountRowsProgram, err := CompileAnonymous("return [SELECT Id FROM Account WHERE Id IN :ids];")
	if err != nil {
		t.Fatal(err)
	}
	accountRowsTypedProgram, err := CompileAnonymous("return (List<Account>) rows(ids);")
	if err != nil {
		t.Fatal(err)
	}
	accountWhoProgram, err := CompileAnonymous("return 'account';")
	if err != nil {
		t.Fatal(err)
	}
	accountChildRowsProgram, err := CompileAnonymous("return super.queryRows(ids);")
	if err != nil {
		t.Fatal(err)
	}
	accountChildTypedProgram, err := CompileAnonymous("return super.accounts(ids);")
	if err != nil {
		t.Fatal(err)
	}
	accountChildWhoProgram, err := CompileAnonymous("return super.who();")
	if err != nil {
		t.Fatal(err)
	}
	eventGetterProgram, err := CompileAnonymous(`
if (Instance == null || Test.isRunningTest()) {
    Instance = new EventManager.WithSharing();
}
return Instance;
`)
	if err != nil {
		t.Fatal(err)
	}
	eventRowsProgram, err := CompileAnonymous("return [SELECT Id FROM Event__c WHERE Id IN :ids];")
	if err != nil {
		t.Fatal(err)
	}
	eventRowsTypedProgram, err := CompileAnonymous("return (List<Event__c>) rows(ids);")
	if err != nil {
		t.Fatal(err)
	}
	eventWhoProgram, err := CompileAnonymous("return 'event';")
	if err != nil {
		t.Fatal(err)
	}
	eventChildRowsProgram, err := CompileAnonymous("return super.queryRows(ids);")
	if err != nil {
		t.Fatal(err)
	}
	eventChildTypedProgram, err := CompileAnonymous("return super.events(ids);")
	if err != nil {
		t.Fatal(err)
	}
	eventChildWhoProgram, err := CompileAnonymous("return super.who();")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Set<Id> eventIds = new Set<Id>{'a00000000000001AAA'};
System.assertEquals(0, [SELECT Id FROM Account WHERE Id IN :eventIds].size());
System.assertEquals(1, [SELECT Id FROM Event__c WHERE Id IN :eventIds].size());
System.assertEquals(1, EventManager.Instance.events(eventIds).size());
System.assertEquals('account', AccountManager.Instance.who());
List<Account> accounts = AccountManager.Instance.accounts(eventIds);
System.assertEquals(0, accounts.size());
`)
	if err != nil {
		t.Fatal(err)
	}

	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Id": {APIName: "Id", Type: storage.FieldID},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	org.Objects["Event__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Event__c",
			Fields: map[string]storage.Field{
				"Id": {APIName: "Id", Type: storage.FieldID},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {ID: "a00000000000001", Object: "Event__c"},
		},
	}

	machine := New(nil)
	machine.EnableTestContext()
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name:       "ManagerBase",
		IsAbstract: true,
		Methods: map[string]Method{
			"rows":      {Name: "ManagerBase.rows", ClassName: "ManagerBase", ReturnType: "List<SObject>", Params: []Param{{Name: "ids", Type: "Set<Id>"}}, Program: baseRowsProgram},
			"queryRows": {Name: "ManagerBase.queryRows", ClassName: "ManagerBase", ReturnType: "List<SObject>", Params: []Param{{Name: "ids", Type: "Set<Id>"}}, Program: baseQueryProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "AccountManager",
		SuperClass: "ManagerBase",
		StaticFields: map[string]Field{
			"Instance": {
				Name:     "Instance",
				Type:     "AccountManager",
				Static:   true,
				Property: true,
				Getter:   &Method{Name: "AccountManager.Instance.get", ClassName: "AccountManager", ReturnType: "AccountManager", IsStatic: true, Program: accountGetterProgram},
			},
			"WithSharingInstance": {
				Name:     "WithSharingInstance",
				Type:     "AccountManager",
				Static:   true,
				Property: true,
				Getter:   &Method{Name: "AccountManager.WithSharingInstance.get", ClassName: "AccountManager", ReturnType: "AccountManager", IsStatic: true, Program: accountSharingGetterProgram},
			},
		},
		Methods: map[string]Method{
			"queryRows": {Name: "AccountManager.queryRows", ClassName: "AccountManager", ReturnType: "List<SObject>", Params: []Param{{Name: "ids", Type: "Set<Id>"}}, Program: accountRowsProgram},
			"accounts":  {Name: "AccountManager.accounts", ClassName: "AccountManager", ReturnType: "List<Account>", Params: []Param{{Name: "ids", Type: "Set<Id>"}}, Program: accountRowsTypedProgram},
			"who":       {Name: "AccountManager.who", ClassName: "AccountManager", ReturnType: "String", Program: accountWhoProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "AccountManager.WithSharing",
		SuperClass: "AccountManager",
		Methods: map[string]Method{
			"queryRows": {Name: "AccountManager.WithSharing.queryRows", ClassName: "AccountManager.WithSharing", ReturnType: "List<SObject>", Params: []Param{{Name: "ids", Type: "Set<Id>"}}, Program: accountChildRowsProgram},
			"accounts":  {Name: "AccountManager.WithSharing.accounts", ClassName: "AccountManager.WithSharing", ReturnType: "List<Account>", Params: []Param{{Name: "ids", Type: "Set<Id>"}}, Program: accountChildTypedProgram},
			"who":       {Name: "AccountManager.WithSharing.who", ClassName: "AccountManager.WithSharing", ReturnType: "String", Program: accountChildWhoProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "EventManager",
		SuperClass: "ManagerBase",
		StaticFields: map[string]Field{
			"Instance": {
				Name:     "Instance",
				Type:     "EventManager",
				Static:   true,
				Property: true,
				Getter:   &Method{Name: "EventManager.Instance.get", ClassName: "EventManager", ReturnType: "EventManager", IsStatic: true, Program: eventGetterProgram},
			},
		},
		Methods: map[string]Method{
			"queryRows": {Name: "EventManager.queryRows", ClassName: "EventManager", ReturnType: "List<SObject>", Params: []Param{{Name: "ids", Type: "Set<Id>"}}, Program: eventRowsProgram},
			"events":    {Name: "EventManager.events", ClassName: "EventManager", ReturnType: "List<Event__c>", Params: []Param{{Name: "ids", Type: "Set<Id>"}}, Program: eventRowsTypedProgram},
			"who":       {Name: "EventManager.who", ClassName: "EventManager", ReturnType: "String", Program: eventWhoProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "EventManager.WithSharing",
		SuperClass: "EventManager",
		Methods: map[string]Method{
			"queryRows": {Name: "EventManager.WithSharing.queryRows", ClassName: "EventManager.WithSharing", ReturnType: "List<SObject>", Params: []Param{{Name: "ids", Type: "Set<Id>"}}, Program: eventChildRowsProgram},
			"events":    {Name: "EventManager.WithSharing.events", ClassName: "EventManager.WithSharing", ReturnType: "List<Event__c>", Params: []Param{{Name: "ids", Type: "Set<Id>"}}, Program: eventChildTypedProgram},
			"who":       {Name: "EventManager.WithSharing.who", ClassName: "EventManager.WithSharing", ReturnType: "String", Program: eventChildWhoProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectDomainSuperConstructorStoresRecords(t *testing.T) {
	constructorProgram, err := CompileAnonymous("super(records);")
	if err != nil {
		t.Fatal(err)
	}
	sizeProgram, err := CompileAnonymous("return Records.size();")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<SObject> records = new List<SObject>{new Account(Name = 'Acme')};
AccountDomain domain = new AccountDomain(records);
System.assertEquals(1, domain.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{
		Name: "SObjectDomain",
		Fields: map[string]Field{
			"Records":         {Name: "Records", Type: "List<SObject>"},
			"SObjectDescribe": {Name: "SObjectDescribe", Type: "Object"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "AccountDomain",
		SuperClass: "SObjectDomain",
		Constructors: []Method{{
			Name:          "AccountDomain.<init>",
			ClassName:     "AccountDomain",
			ReturnType:    "void",
			IsConstructor: true,
			Params:        []Param{{Name: "records", Type: "List<SObject>"}},
			Program:       constructorProgram,
		}},
		Methods: map[string]Method{
			"size": {Name: "AccountDomain.size", ClassName: "AccountDomain", ReturnType: "Integer", Program: sizeProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInterfaceMethodLookupFallback(t *testing.T) {
	methodProgram, err := CompileAnonymous("return 'iface';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
ImplementsWorker worker = new ImplementsWorker();
System.assertEquals('iface', worker.work());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Worker",
		Methods: map[string]Method{
			"work": {Name: "Worker.work", ClassName: "Worker", ReturnType: "String", Program: methodProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "ImplementsWorker",
		Interfaces: []string{"Worker"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecVirtualDispatchThroughBaseAndInterfaceReferences(t *testing.T) {
	baseProgram, err := CompileAnonymous("return 'base';")
	if err != nil {
		t.Fatal(err)
	}
	childProgram, err := CompileAnonymous("return 'child';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Base base = new Child();
Worker worker = new Child();
System.assertEquals('child', base.work());
System.assertEquals('child', worker.work());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Base",
		Methods: map[string]Method{
			"work": {Name: "Base.work", ClassName: "Base", ReturnType: "String", Program: baseProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:        "Worker",
		IsInterface: true,
		Methods: map[string]Method{
			"work": {Name: "Worker.work", ClassName: "Worker", ReturnType: "String", Program: baseProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Child",
		SuperClass: "Base",
		Interfaces: []string{"Worker"},
		Methods: map[string]Method{
			"work": {Name: "Child.work", ClassName: "Child", ReturnType: "String", Program: childProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSuperDispatchUsesDeclaringClass(t *testing.T) {
	parentProgram, err := CompileAnonymous("return 1;")
	if err != nil {
		t.Fatal(err)
	}
	childProgram, err := CompileAnonymous("return super.score() + 1;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
GrandChild value = new GrandChild();
System.assertEquals(2, value.score());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Parent",
		Methods: map[string]Method{
			"score": {Name: "Parent.score", ClassName: "Parent", ReturnType: "Integer", Program: parentProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Child",
		SuperClass: "Parent",
		Methods: map[string]Method{
			"score": {Name: "Child.score", ClassName: "Child", ReturnType: "Integer", Program: childProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "GrandChild", SuperClass: "Child"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInheritedConcreteMethodBeatsInterfaceFallback(t *testing.T) {
	parentProgram, err := CompileAnonymous("return 'parent';")
	if err != nil {
		t.Fatal(err)
	}
	interfaceProgram, err := CompileAnonymous("return 'interface';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Worker worker = new Child();
System.assertEquals('parent', worker.work());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Parent",
		Methods: map[string]Method{
			"work": {Name: "Parent.work", ClassName: "Parent", ReturnType: "String", Program: parentProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:        "Worker",
		IsInterface: true,
		Methods: map[string]Method{
			"work": {Name: "Worker.work", ClassName: "Worker", ReturnType: "String", Program: interfaceProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Child", SuperClass: "Parent", Interfaces: []string{"Worker"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMapInterfaceValueDispatchesConcreteMethod(t *testing.T) {
	concreteProgram, err := CompileAnonymous("return 'concrete';")
	if err != nil {
		t.Fatal(err)
	}
	interfaceProgram, err := CompileAnonymous("return 'interface';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Map<Id, Worker> workers = new Map<Id, Worker>();
Worker worker = (Worker) Type.forName('ConcreteWorker').newInstance();
workers.put('001000000000001AAA', worker);
Worker fromMap = workers.get('001000000000001AAA');
System.assertEquals('concrete', fromMap.work());
System.assertEquals('concrete', workers.get('001000000000001AAA').work());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:        "Worker",
		IsInterface: true,
		Methods: map[string]Method{
			"work": {Name: "Worker.work", ClassName: "Worker", ReturnType: "String", Program: interfaceProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "ConcreteWorker",
		Interfaces: []string{"Worker"},
		Methods: map[string]Method{
			"work": {Name: "ConcreteWorker.work", ClassName: "ConcreteWorker", ReturnType: "String", Program: concreteProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInheritedStaticMembersViaSubclassName(t *testing.T) {
	staticProgram, err := CompileAnonymous("return total;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Child.total = 5;
System.assertEquals(5, Child.total);
System.assertEquals(5, Child.totalValue());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Parent",
		StaticFields: map[string]Field{
			"total": {Name: "total", Type: "Integer", Static: true, Value: Int(1)},
		},
		Methods: map[string]Method{
			"totalValue": {Name: "Parent.totalValue", ClassName: "Parent", ReturnType: "Integer", IsStatic: true, Program: staticProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Child", SuperClass: "Parent"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecEnumMethods(t *testing.T) {
	program, err := CompileAnonymous(`
	Mood mood = Mood.Happy;
	System.assertEquals('Happy', mood.name());
	System.assertEquals('Happy', mood.toString());
	System.assertEquals(0, mood.ordinal());
List<Mood> values = Mood.values();
System.assertEquals(2, values.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Mood", EnumValues: []string{"Happy", "Sad"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecObjectToStringDispatch(t *testing.T) {
	toStringProgram, err := CompileAnonymous("return 'custom';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Named named = new Named();
Plain plain = new Plain();
System.assertEquals('custom', named.toString());
System.assertEquals('Plain:{}', plain.toString());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Named",
		Methods: map[string]Method{
			"toString": {Name: "Named.toString", ClassName: "Named", ReturnType: "String", Program: toStringProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Plain"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTypeExceptionReportsRuntimeTypeForDowncastedObject(t *testing.T) {
	program, err := CompileAnonymous(`
Parent value = (Parent)new Child();
try {
	DateTime ignored = (DateTime)value;
	System.assert(false, 'expected TypeException');
} catch (System.TypeException e) {
	System.assert(e.getMessage().contains('Invalid conversion from runtime type Child to Datetime'), e.getMessage());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Parent"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Child", SuperClass: "Parent"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecObjectToStringUsedForDebugAndAssertMessages(t *testing.T) {
	toStringProgram, err := CompileAnonymous("return 'named-value';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Named named = new Named();
System.debug(named);
System.assertEquals('expected-value', named);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Named",
		Methods: map[string]Method{
			"toString": {Name: "Named.toString", ClassName: "Named", ReturnType: "String", Program: toStringProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	result, err := machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "actual <named-value>") {
		t.Fatalf("error = %v", err)
	}
	if len(result.Debug) != 1 || result.Debug[0] != "named-value" {
		t.Fatalf("debug = %#v", result.Debug)
	}
}

func TestExecSystemAssertEqualsUsesApexEqualsOverride(t *testing.T) {
	equalsProgram, err := CompileAnonymous("return true;")
	if err != nil {
		t.Fatal(err)
	}
	notEqualsProgram, err := CompileAnonymous("return false;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals(new EqualBox(), new EqualBox());
System.assertNotEquals(new DistinctBox(), new DistinctBox());
Set<EqualBox> boxes = new Set<EqualBox>{new EqualBox()};
System.assertEquals(true, boxes.remove(new EqualBox()));
System.assertEquals(0, boxes.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "EqualBox",
		Methods: map[string]Method{
			"equals": {Name: "EqualBox.equals", ClassName: "EqualBox", ReturnType: "Boolean", Params: []Param{{Name: "other", Type: "Object"}}, Program: equalsProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "DistinctBox",
		Methods: map[string]Method{
			"equals": {Name: "DistinctBox.equals", ClassName: "DistinctBox", ReturnType: "Boolean", Params: []Param{{Name: "other", Type: "Object"}}, Program: notEqualsProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestTypeHashCodeUsesTypeName(t *testing.T) {
	if valueHashCode(platformScalar("Type", "Integer")) == valueHashCode(platformScalar("Type", "String")) {
		t.Fatal("Type.hashCode ignored the type name")
	}
}

func TestStubProxyStringUsesGeneratedApexStubTypeName(t *testing.T) {
	proxy := Object("MockedList")
	proxy.Fields["__gladeStubbedType"] = String("MockedList")
	got := proxy.String()
	if !strings.HasPrefix(got, "MockedList__sfdc_ApexStub:") {
		t.Fatalf("proxy string = %q", got)
	}
}

func TestExecUserObjectEqualityUsesIdentity(t *testing.T) {
	program, err := CompileAnonymous(`
Box first = new Box();
Box second = new Box();
Box alias = first;
System.assertNotEquals(first, second);
System.assertEquals(first, alias);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Box"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDottedCallOnInstanceFieldWithSimpleReceiver(t *testing.T) {
	runProgram, err := CompileAnonymous(`
Set<String> keys = values.keySet();
return keys.size();
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Holder holder = new Holder();
System.assertEquals(1, holder.run());
`)
	if err != nil {
		t.Fatal(err)
	}
	values := typedMap("Map<String,Object>")
	values.Map[mapKey(String("name"))] = String("value")
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Holder",
		Fields: map[string]Field{
			"values": {Name: "values", Type: "Map<String,Object>", InitialValue: values},
		},
		Methods: map[string]Method{
			"run": {
				Name:       "Holder.run",
				ClassName:  "Holder",
				ReturnType: "Integer",
				Program:    runProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDottedCallOnInheritedPropertyWithSimpleReceiver(t *testing.T) {
	getterProgram, err := CompileAnonymous(`
Map<String,Object> values = new Map<String,Object>();
values.put('name', 'value');
return values;
`)
	if err != nil {
		t.Fatal(err)
	}
	runProgram, err := CompileAnonymous(`
Set<String> keys = values.keySet();
return keys.size();
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Child child = new Child();
System.assertEquals(1, child.run());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Base",
		Fields: map[string]Field{
			"values": {
				Name:   "values",
				Type:   "Map<String,Object>",
				Access: "private",
				Getter: &Method{
					Name:       "Base.values.get",
					ClassName:  "Base",
					ReturnType: "Map<String,Object>",
					Program:    getterProgram,
				},
			},
		},
		Methods: map[string]Method{
			"run": {
				Name:       "Base.run",
				ClassName:  "Base",
				ReturnType: "Integer",
				Program:    runProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Child",
		SuperClass: "Base",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAssignsFieldOnIndexedListElement(t *testing.T) {
	program, err := CompileAnonymous(`
Item first = new Item();
Item second = new Item();
List<Item> items = new List<Item>{ first, second };
items[0].next = items[1];
System.assertEquals(second, first.next);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Item",
		Fields: map[string]Field{
			"next": {Name: "next", Type: "Item"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecObjectAssignabilityUsesInheritanceAndInterfaces(t *testing.T) {
	acceptProgram, err := CompileAnonymous("return 1;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Base base = new Child();
Marker marker = new Child();
System.assertEquals(1, Util.acceptBase(new Child()));
System.assertEquals(1, Util.acceptMarker(new Child()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, class := range []Class{
		{Name: "Base"},
		{Name: "Marker", IsInterface: true},
		{Name: "Child", SuperClass: "Base", Interfaces: []string{"Marker"}},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if err := machine.RegisterMethod(Method{
		Name:       "Util.acceptBase",
		ReturnType: "Integer",
		Params:     []Param{{Name: "base", Type: "Base"}},
		Program:    acceptProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:       "Util.acceptMarker",
		ReturnType: "Integer",
		Params:     []Param{{Name: "marker", Type: "Marker"}},
		Program:    acceptProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecVirtualDispatchUsesRuntimeTypeForBaseTypedParameter(t *testing.T) {
	baseTypeProgram, err := CompileAnonymous("return Base.class;")
	if err != nil {
		t.Fatal(err)
	}
	childTypeProgram, err := CompileAnonymous("return Child.class;")
	if err != nil {
		t.Fatal(err)
	}
	helperProgram, err := CompileAnonymous("return message.getType().getName();")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Base base = new Child();
System.assertEquals('Child', base.getType().getName());
System.assertEquals('Child', Helper.eventName(new Child()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, class := range []Class{
		{Name: "Base"},
		{Name: "Child", SuperClass: "Base"},
		{Name: "Helper"},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	for _, method := range []Method{
		{Name: "Base.getType", ClassName: "Base", ReturnType: "Type", Program: baseTypeProgram},
		{Name: "Child.getType", ClassName: "Child", ReturnType: "Type", Program: childTypeProgram},
		{Name: "Helper.eventName", ClassName: "Helper", IsStatic: true, ReturnType: "String", Params: []Param{{Name: "message", Type: "Base"}}, Program: helperProgram},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStaticListMutationPersistsAcrossStaticMethods(t *testing.T) {
	addProgram, err := CompileAnonymous("values.add(value);")
	if err != nil {
		t.Fatal(err)
	}
	countProgram, err := CompileAnonymous("return values.size();")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
StaticListRegistry.addOne('x');
System.assertEquals(1, StaticListRegistry.countValues());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "StaticListRegistry",
		StaticFields: map[string]Field{
			"values": {Name: "values", Type: "List<String>", Static: true, Value: List(), InitialValue: List()},
		},
		Methods: map[string]Method{
			"addOne":      {Name: "StaticListRegistry.addOne", ClassName: "StaticListRegistry", Params: []Param{{Name: "value", Type: "String"}}, Program: addProgram, IsStatic: true},
			"countValues": {Name: "StaticListRegistry.countValues", ClassName: "StaticListRegistry", ReturnType: "Integer", Program: countProgram, IsStatic: true},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecObjectAssignabilityUsesNestedInterfaceShortName(t *testing.T) {
	acceptProgram, err := CompileAnonymous("return 1;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Outer.I item = new Outer();
System.assertEquals(1, Util.accept(new Outer()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, class := range []Class{
		{Name: "Outer.I", IsInterface: true},
		{Name: "Outer", Interfaces: []string{"I"}},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if err := machine.RegisterMethod(Method{
		Name:       "Util.accept",
		ReturnType: "Integer",
		Params:     []Param{{Name: "item", Type: "Outer.I"}},
		Program:    acceptProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRejectsUnrelatedObjectAssignment(t *testing.T) {
	program, err := CompileAnonymous("Base base = new Other();")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, class := range []Class{{Name: "Base"}, {Name: "Other"}} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err == nil || !strings.Contains(err.Error(), "cannot assign Other to Base") {
		t.Fatalf("expected object assignment error, got %v", err)
	}
}

func TestExecInvalidObjectCastThrowsTypeException(t *testing.T) {
	program, err := CompileAnonymous(`
try {
	Base base = (Base) new Other();
	System.assert(false);
} catch (System.TypeException e) {
	System.assertEquals('Invalid conversion from runtime type Other to Base', e.getMessage());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, class := range []Class{{Name: "Base"}, {Name: "Other"}} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecRejectsNonConstructableRegisteredTypes(t *testing.T) {
	cases := []struct {
		name  string
		class Class
		want  string
	}{
		{name: "abstract", class: Class{Name: "Base", IsAbstract: true}, want: "cannot instantiate abstract class Base"},
		{name: "interface", class: Class{Name: "IThing", IsInterface: true}, want: "cannot instantiate interface IThing"},
		{name: "enum", class: Class{Name: "Mood", EnumValues: []string{"Happy"}}, want: "cannot instantiate enum Mood"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous("Object value = new " + tc.class.Name + "();")
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			if err := machine.RegisterClass(tc.class); err != nil {
				t.Fatal(err)
			}
			_, err = machine.Execute(program)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestExecRejectsAbstractMethodInvocation(t *testing.T) {
	program, err := CompileAnonymous(`
Base value = new Concrete();
value.required();
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "Base",
		Methods: map[string]Method{
			"required": {Name: "Base.required", ClassName: "Base", ReturnType: "void", Modifiers: []string{"abstract"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Concrete", SuperClass: "Base"}); err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "cannot execute abstract method Base.required") {
		t.Fatalf("error = %v", err)
	}
}

func TestExecDispatchesUnqualifiedVirtualCallToConcreteOverride(t *testing.T) {
	baseCall, err := CompileAnonymous("return required();")
	if err != nil {
		t.Fatal(err)
	}
	override, err := CompileAnonymous("return 'concrete';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Base value = new Concrete();
System.assertEquals('concrete', value.callRequired());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "Base",
		IsAbstract: true,
		Methods: map[string]Method{
			"callRequired": {Name: "Base.callRequired", ClassName: "Base", ReturnType: "String", Access: "public", Program: baseCall},
			"required":     {Name: "Base.required", ClassName: "Base", ReturnType: "String", Access: "public", Modifiers: []string{"abstract"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "Concrete",
		SuperClass: "Base",
		Methods: map[string]Method{
			"required": {Name: "Concrete.required", ClassName: "Concrete", ReturnType: "String", Access: "public", Modifiers: []string{"override"}, Program: override},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDispatchesInheritedVirtualOverrideFromBaseMethod(t *testing.T) {
	baseRun, err := CompileAnonymous("this.apply(value); return value;")
	if err != nil {
		t.Fatal(err)
	}
	baseApply, err := CompileAnonymous("return null;")
	if err != nil {
		t.Fatal(err)
	}
	midApply, err := CompileAnonymous("box.put('Name', 'copied'); return null;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account value = new Account(Name = 'original');
new Concrete().run(value);
System.assertEquals('copied', value.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "BaseDomain",
		Methods: map[string]Method{
			"run":   {Name: "BaseDomain.run", ClassName: "BaseDomain", ReturnType: "Account", Params: []Param{{Name: "value", Type: "Account"}}, Access: "public", Modifiers: []string{"virtual"}, Program: baseRun},
			"apply": {Name: "BaseDomain.apply", ClassName: "BaseDomain", ReturnType: "void", Params: []Param{{Name: "box", Type: "SObject"}}, Access: "public", Modifiers: []string{"virtual"}, Program: baseApply},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "MidDomain",
		SuperClass: "BaseDomain",
		Methods: map[string]Method{
			"apply": {Name: "MidDomain.apply", ClassName: "MidDomain", ReturnType: "void", Params: []Param{{Name: "box", Type: "SObject"}}, Access: "public", Modifiers: []string{"override", "virtual"}, Program: midApply},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Concrete", SuperClass: "MidDomain"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMapKeySetPreservesDateKeys(t *testing.T) {
	program, err := CompileAnonymous(`
Date today = Date.today();
Map<Date, String> values = new Map<Date, String>();
values.put(today, 'open');
Set<Date> keys = values.keySet();
List<Date> ordered = new List<Date>(keys);
System.assertEquals(today, ordered[0]);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeIdOverloadPreferredForDeclaredId(t *testing.T) {
	stringProgram, err := CompileAnonymous("return 'string:' + value;")
	if err != nil {
		t.Fatal(err)
	}
	idProgram, err := CompileAnonymous("return 'id:' + value;")
	if err != nil {
		t.Fatal(err)
	}
	returnIDProgram, err := CompileAnonymous("return '00X000000000001AAA';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
String templateName = 'ExampleEmailVerify';
Id templateId = Util.templateId();
System.assertEquals('string:ExampleEmailVerify', Util.pick(templateName));
System.assertEquals('id:00X000000000001AAA', Util.pick(templateId));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, method := range []Method{
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "value", Type: "String"}}, Program: stringProgram},
		{Name: "Util.pick", ReturnType: "String", Params: []Param{{Name: "value", Type: "Id"}}, Program: idProgram},
		{Name: "Util.templateId", ReturnType: "Id", Program: returnIDProgram},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAssignIDToStringUses18CharacterID(t *testing.T) {
	program, err := CompileAnonymous(`
Id accountId = Id.valueOf('001000000000001');
String text = accountId;
System.assertEquals('001000000000001AAA', text);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecDateArgumentAssignableToDatetimeParameter(t *testing.T) {
	acceptProgram, err := CompileAnonymous(`
System.assertEquals('2026-05-07T00:00:00Z', value.formatGmt('yyyy-MM-dd''T''HH:mm:ss''Z'''));
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`Util.accept(Date.newInstance(2026, 5, 7));`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{
		Name:       "Util.accept",
		ReturnType: "void",
		Params:     []Param{{Name: "value", Type: "Datetime"}},
		Program:    acceptProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecWhileLoopIterationGuard(t *testing.T) {
	program, err := CompileAnonymous("while (true) { System.debug('loop'); }")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil {
		t.Fatal("expected loop guard error")
	}
}

func TestRegisterClassDropsSelfSuperclassAfterDependencyMerge(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "HostedCheckoutJavaScriptHandler",
		Namespace:  "znu",
		Dependency: true,
		Fields: map[string]Field{
			"DependencyField": {Name: "DependencyField", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "HostedCheckoutJavaScriptHandler",
		Namespace:  "znu",
		SuperClass: "znu.HostedCheckoutJavaScriptHandler",
		Fields: map[string]Field{
			"LocalField": {Name: "LocalField", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	class := machine.Classes["HostedCheckoutJavaScriptHandler"]
	if class.SuperClass != "" {
		t.Fatalf("merged superclass = %q, want empty self-reference removed", class.SuperClass)
	}
	if _, ok := class.Fields["DependencyField"]; !ok {
		t.Fatalf("dependency field was not merged: %#v", class.Fields)
	}
	if _, ok := class.Fields["LocalField"]; !ok {
		t.Fatalf("local field was not merged: %#v", class.Fields)
	}
}
