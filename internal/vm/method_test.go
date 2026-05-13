package vm

import (
	"strings"
	"testing"
)

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

func TestFFLibQualifiedMethodMapKeyUsesSignatureAndMockIdentity(t *testing.T) {
	firstMock := Object("Mocked")
	secondMock := Object("Mocked")
	first := Object("fflib_QualifiedMethod")
	first.Fields["typeName"] = String("Selector")
	first.Fields["methodName"] = String("selectById")
	first.Fields["methodArgTypes"] = List(platformScalar("Type", "Id"))
	first.Fields["mockInstance"] = firstMock
	same := Object("fflib_QualifiedMethod")
	same.Fields["typeName"] = String("Selector")
	same.Fields["methodName"] = String("selectById")
	same.Fields["methodArgTypes"] = List(platformScalar("Type", "Id"))
	same.Fields["mockInstance"] = firstMock
	differentArgs := Object("fflib_QualifiedMethod")
	differentArgs.Fields["typeName"] = String("Selector")
	differentArgs.Fields["methodName"] = String("selectById")
	differentArgs.Fields["methodArgTypes"] = List(platformScalar("Type", "Set<Id>"))
	differentArgs.Fields["mockInstance"] = firstMock
	differentMock := Object("fflib_QualifiedMethod")
	differentMock.Fields["typeName"] = String("Selector")
	differentMock.Fields["methodName"] = String("selectById")
	differentMock.Fields["methodArgTypes"] = List(platformScalar("Type", "Id"))
	differentMock.Fields["mockInstance"] = secondMock

	if mapKey(first) != mapKey(same) {
		t.Fatalf("same qualified method key mismatch: %q != %q", mapKey(first), mapKey(same))
	}
	if mapKey(first) == mapKey(differentArgs) {
		t.Fatalf("qualified method key ignored argument types: %q", mapKey(first))
	}
	if mapKey(first) == mapKey(differentMock) {
		t.Fatalf("qualified method key ignored mock identity: %q", mapKey(first))
	}
	if key, ok := fflibQualifiedMethodMapKey(first, false); !ok || key != mustFFLibQualifiedMethodMapKey(t, differentMock, false) {
		t.Fatalf("dependent qualified method key used mock identity: %q", key)
	}
}

func mustFFLibQualifiedMethodMapKey(t *testing.T, value Value, independentMocks bool) string {
	t.Helper()
	key, ok := fflibQualifiedMethodMapKey(value, independentMocks)
	if !ok {
		t.Fatalf("value is not an fflib qualified method: %#v", value)
	}
	return key
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

func TestExecTestCreateStubInterceptsPropertyGetter(t *testing.T) {
	getterProgram, err := CompileAnonymous("return missing.value;")
	if err != nil {
		t.Fatal(err)
	}
	providerProgram, err := CompileAnonymous("return null;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Base proxy = (Base)Test.createStub(Base.class, new Provider());
System.assertEquals(null, proxy.Amount);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "Base",
		Fields: map[string]Field{
			"Amount": {Name: "Amount", Type: "Decimal", Getter: &Method{Name: "Base.Amount.get", ClassName: "Base", ReturnType: "Decimal", Program: getterProgram}},
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

func TestRuntimeFFLibAnySObjectMatcherAcceptsSObjectLists(t *testing.T) {
	program, err := CompileAnonymous(`
fflib_MatcherDefinitions.AnySObject matcher = new fflib_MatcherDefinitions.AnySObject();
System.assert(matcher.matches(new Account(Name = 'Acme')));
System.assert(matcher.matches(new List<SObject>{ new Account(Name = 'Acme') }));
System.assert(!matcher.matches(new List<SObject>()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "fflib_MatcherDefinitions.AnySObject"}); err != nil {
		t.Fatal(err)
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
System.assertEquals('Plain{}', plain.toString());
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
	proxy := Object("fflib_MyList")
	proxy.Fields["__oaerStubbedType"] = String("fflib_MyList")
	got := proxy.String()
	if !strings.HasPrefix(got, "fflib_MyList__sfdc_ApexStub:") {
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
