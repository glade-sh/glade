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
	if value.Kind != ValueInt || value.Int != 0 {
		t.Fatalf("Counter.get from second clone = %#v, want zero-value static", value)
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

func TestExecWhileLoopIterationGuard(t *testing.T) {
	program, err := CompileAnonymous("while (true) { System.debug('loop'); }")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil {
		t.Fatal("expected loop guard error")
	}
}
