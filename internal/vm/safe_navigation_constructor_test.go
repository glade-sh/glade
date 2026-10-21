package vm

import "testing"

// Apex Developer Guide, Safe Navigation Operator: ?. retains the member's
// declared expression type, including a null result. SObject reference:
// getPopulatedFieldsAsMap() returns Map<String,Object>.
// https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/langCon_apex_SafeNavigationOperator.htm
// https://developer.salesforce.com/docs/atlas.en-us.apexref.meta/apexref/apex_methods_system_sobject.htm
func TestSafeNavigationConstructorChainsPreserveMapReturnType(t *testing.T) {
	for _, chain := range []string{"this", "super"} {
		for _, argument := range []string{"(SObject)null", "new Account(Name='Pebble')"} {
			t.Run(chain+"/"+argument, func(t *testing.T) {
				machine := New(nil)
				mapBody, err := CompileAnonymous("chosen = 'map'; empty = values == null;")
				if err != nil {
					t.Fatal(err)
				}
				recordBody, err := CompileAnonymous("chosen = 'record';")
				if err != nil {
					t.Fatal(err)
				}
				chainBody, err := CompileAnonymous(chain + "(source?.getPopulatedFieldsAsMap());")
				if err != nil {
					t.Fatal(err)
				}
				base := Class{Name: "ParcelChain", Fields: map[string]Field{
					"chosen": {Name: "chosen", Type: "String"}, "empty": {Name: "empty", Type: "Boolean"},
				}, Constructors: []Method{
					{Name: "ParcelChain.<init>", ClassName: "ParcelChain", IsConstructor: true, Params: []Param{{Name: "source", Type: "SObject"}}, Program: recordBody},
					{Name: "ParcelChain.<init>", ClassName: "ParcelChain", IsConstructor: true, Params: []Param{{Name: "values", Type: "Map<String,Object>"}}, Program: mapBody},
				}}
				target := "ParcelChain"
				if chain == "this" {
					base.Constructors[0].Program = chainBody
				}
				if err := machine.RegisterClass(base); err != nil {
					t.Fatal(err)
				}
				if chain == "super" {
					target = "ParcelChild"
					if err := machine.RegisterClass(Class{Name: target, SuperClass: "ParcelChain", Constructors: []Method{
						{Name: target + ".<init>", ClassName: target, IsConstructor: true, Params: []Param{{Name: "source", Type: "SObject"}}, Program: chainBody},
					}}); err != nil {
						t.Fatal(err)
					}
				}
				wantNull := "false"
				if argument == "(SObject)null" {
					wantNull = "true"
				}
				program, err := CompileAnonymous(target + " p = new " + target + "(" + argument + "); System.assertEquals('map',p.chosen); System.assertEquals(" + wantNull + ",p.empty);")
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

func TestThisConstructorChainUsesDeclaredSObjectReturnType(t *testing.T) {
	machine := New(nil)
	returnBody, err := CompileAnonymous("return new Account(Name='Pebble');")
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "ParcelSource"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "ParcelSource.record", ClassName: "ParcelSource", ReturnType: "SObject", Program: returnBody}); err != nil {
		t.Fatal(err)
	}
	chainBody, err := CompileAnonymous("this(source?.record());")
	if err != nil {
		t.Fatal(err)
	}
	recordBody, err := CompileAnonymous("chosen='record';")
	if err != nil {
		t.Fatal(err)
	}
	accountBody, err := CompileAnonymous("chosen='account';")
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "ParcelView", Fields: map[string]Field{"chosen": {Name: "chosen", Type: "String"}}, Constructors: []Method{
		{Name: "ParcelView.<init>", ClassName: "ParcelView", IsConstructor: true, Params: []Param{{Name: "source", Type: "ParcelSource"}}, Program: chainBody},
		{Name: "ParcelView.<init>", ClassName: "ParcelView", IsConstructor: true, Params: []Param{{Name: "record", Type: "SObject"}}, Program: recordBody},
		{Name: "ParcelView.<init>", ClassName: "ParcelView", IsConstructor: true, Params: []Param{{Name: "record", Type: "Account"}}, Program: accountBody},
	}}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("ParcelView p=new ParcelView(new ParcelSource()); System.assertEquals('record',p.chosen);")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestSafeNavigationTypedMethodNullSkipsArguments(t *testing.T) {
	machine := New(nil)
	body, err := CompileAnonymous("System.assert(false,'skipped code executed'); return null;")
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "ParcelSource"}); err != nil {
		t.Fatal(err)
	}
	for _, method := range []Method{
		{Name: "ParcelSource.fields", ClassName: "ParcelSource", ReturnType: "Map<String,Object>", Params: []Param{{Name: "n", Type: "Integer"}}, Program: body},
		{Name: "ParcelSource.tick", ClassName: "ParcelSource", IsStatic: true, ReturnType: "Integer", Program: body},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	program, err := CompileAnonymous("ParcelSource source=null; Object value=source?.fields(ParcelSource.tick()); System.assertEquals(null,value);")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	// Inspect the expression result before an Object assignment can change its view.
	expression, err := CompileAnonymous("return source?.fields(ParcelSource.tick());")
	if err != nil {
		t.Fatal(err)
	}
	value, err := machine.eval(expression.Instructions[0].Expr, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueNull || value.Type != "Map<String,Object>" || value.Static != value.Type || !isSafeNavigationNull(value) {
		t.Fatalf("safe method null lost its static type or sentinel: %#v", value)
	}
}
