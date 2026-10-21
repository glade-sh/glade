package vm

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Owned named observations pin the reflection helper to source API54 and
// its caller to API62. Closed primitive list identity survives the Object
// views, and the explicit Account cast rejects before singleton assignment.
func TestPrimitiveListReflectionThroughObjectView(t *testing.T) {
	for _, api := range []string{"62.0"} {
		for _, tc := range []struct{ element, item string }{
			{"Id", "(Id)'001000000000001'"},
			{"Decimal", "2.5"},
			{"Date", "Date.newInstance(2040,1,2)"},
			{"Datetime", "Datetime.newInstanceGMT(2040,1,2,0,0,0)"},
			{"Boolean", "true"},
		} {
			for _, size := range []int{0, 1, 2} {
				t.Run(fmt.Sprintf("%s/%s/%d", api, tc.element, size), func(t *testing.T) {
					machine := New(nil)
					registerPrimitiveListProbe(t, machine, "54.0")
					source := fmt.Sprintf(`
List<Object> values = (List<Object>)Type.forName('List<%s>').newInstance();
%s
System.assertEquals(%s.class, ListElementProbe.identify(values));
System.assertEquals(%d, values.size());
`, tc.element, strings.Repeat("values.add("+tc.item+");", size), tc.element, size)
					if tc.element == "Id" && size == 1 {
						source += `
List<Object> copied = values.clone();
copied.clear();
System.assertEquals(Id.class, ListElementProbe.identify(copied));
System.assertEquals(1, values.size());
`
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

func TestPrimitiveListReductionAfterReflection(t *testing.T) {
	for _, api := range []string{"62.0"} {
		t.Run(api, func(t *testing.T) {
			machine := New(nil)
			// A legacy reflection helper receives a list created by a newer caller.
			registerPrimitiveListProbe(t, machine, "54.0")
			program, err := CompileAnonymousWithOptions(`
List<Object> ids = (List<Object>)Type.forName('List<Id>').newInstance();
for (Account row : new List<Account>{
    new Account(Id='001000000000001'), new Account(Id='001000000000002'),
    new Account(Id='001000000000001')
}) ids.add(row.Id);
System.assertEquals(Id.class, ListElementProbe.identify(ids));
Set<Id> reduced = new Set<Id>((List<Id>)(Object)ids);
System.assertEquals(new Set<Id>{'001000000000001','001000000000002'}, reduced);
System.assertEquals(3, ids.size());
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPrimitiveListReflectionKeepsControls(t *testing.T) {
	machine := New(nil)
	registerPrimitiveListProbe(t, machine, "54.0")
	program, err := CompileAnonymousWithOptions(`
System.assertEquals(String.class,ListElementProbe.identify(new List<String>{'owned'}));
System.assertEquals(Integer.class,ListElementProbe.identify(new List<Integer>{7}));
Object record = new Account(Name='owned');
System.assertEquals('owned',((Account)record).Name);
Object absent;
System.assertEquals(null,(Account)absent);
`, CompileOptions{APIVersion: "62.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

// These are local unchanged controls for the candidate's scope, not claims
// that the legacy cardinality behavior matches the platform in these shapes.
func TestPrimitiveListReflectionKeepsUncapturedCastPaths(t *testing.T) {
	for _, tc := range []struct {
		name, api, target, view, static string
		query                           bool
	}{
		{"below-helper-floor", "52.0", "Account", "List<Object>", "Object", false},
		{"uncaptured-helper56", "56.0", "Account", "List<Object>", "Object", false},
		{"uncaptured-helper61", "61.0", "Account", "List<Object>", "Object", false},
		{"uncaptured-helper62", "62.0", "Account", "List<Object>", "Object", false},
		{"uncaptured-helper67", "67.0", "Account", "List<Object>", "Object", false},
		{"uncaptured-helper68", "68.0", "Account", "List<Object>", "Object", false},
		{"above-candidate-window", "69.0", "Account", "List<Object>", "Object", false},
		{"different-record-target", "54.0", "Contact", "List<Object>", "Object", false},
		{"generic-record-target", "54.0", "SObject", "List<Object>", "Object", false},
		{"typed-list-view", "54.0", "Account", "List<Id>", "Object", false},
		{"unboxed-list-view", "54.0", "Account", "List<Object>", "List<Object>", false},
		{"direct-query-operand", "54.0", "Account", "List<Object>", "Object", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			machine.currentMethod.APIVersion = tc.api
			value := List(platformScalar("Id", "001000000000001"), platformScalar("Id", "001000000000002"))
			value.Type, value.Runtime, value.Static = tc.view, "List<Id>", tc.static
			_, err := machine.coerceCastForOperand(tc.target, value, tc.query)
			var thrown *apexThrowError
			if !errors.As(err, &thrown) || exceptionTypeName(thrown.value.Type) != "QueryException" {
				t.Fatalf("unchanged cardinality path: %v", err)
			}
		})
	}
}

func registerPrimitiveListProbe(t *testing.T, machine *VM, api string) {
	t.Helper()
	for _, spec := range []struct {
		name, paramType, source string
	}{
		{"identify", "List<Object>", "return ListElementProbe.inspect(input);"},
		{"inspect", "Object", `
try { Account ignored = (Account)input; }
catch (Exception problem) {
    System.assertEquals('System.TypeException',problem.getTypeName());
    return Type.forName(problem.getMessage().substringBetween('List<','>'));
}
return null;
`},
	} {
		program, err := CompileAnonymousWithOptions(spec.source, CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterMethod(Method{
			Name: "ListElementProbe." + spec.name, ClassName: "ListElementProbe",
			IsStatic: true, ReturnType: "Type", APIVersion: api,
			Params: []Param{{Name: "input", Type: spec.paramType}}, Program: program,
		}); err != nil {
			t.Fatal(err)
		}
	}
}
