package vm

import (
	"strings"
	"testing"
)

// These controls preserve the local candidate's lexical boundary. They do not
// assert native construction behavior for property or static bind sources.
func TestEmptyReferenceSumLocatorRejectsGetterSources(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, source := range []string{"currentGetter", "inheritedGetter", "staticGetter"} {
			t.Run(api+"/"+source, func(t *testing.T) {
				machine := emptyReferenceSumLocatorMachine(api)
				getter, err := CompileAnonymousWithOptions(`
calls++;
if (calls == 1) return new Set<Id>();
return new Set<Id>{'001000000000099'};
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				static := source == "staticGetter"
				property := Field{
					Name: "parents", Type: "Set<Id>", Static: static, Property: true,
					Getter: &Method{Name: "BindSource.parents.get", ClassName: "BindSource", ReturnType: "Set<Id>", IsStatic: static, APIVersion: api, Program: getter},
				}
				class := Class{
					Name: "BindSource", APIVersion: api,
					StaticFields: map[string]Field{
						"calls": {Name: "calls", Type: "Integer", Static: true, Value: Int(0)},
					},
				}
				if static {
					class.StaticFields["parents"] = property
				} else {
					class.Fields = map[string]Field{"parents": property}
				}
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
				receiver := "BindSource"
				if source == "inheritedGetter" {
					receiver = "DerivedBindSource"
					if err := machine.RegisterClass(Class{Name: receiver, SuperClass: "BindSource", APIVersion: api}); err != nil {
						t.Fatal(err)
					}
				}
				machine.currentClass = receiver
				machine.currentMethod = Method{ClassName: receiver, APIVersion: api, IsStatic: static}
				if !static {
					machine.Globals["this"] = Object(receiver)
				}
				// A stale declared view alone must not admit a getter. An active
				// Globals binding is required before ordinary lookup is invoked.
				machine.VarTypes["parents"] = "Set<Id>"
				_, err = machine.databaseQueryHandle(String(emptyReferenceSumBindingScopeQuery("parents")), Null, Null, false, &Result{})
				assertEmptyReferenceSumBindingScopeRejection(t, err)
				calls, err := machine.lookup("BindSource.calls")
				if err != nil || calls.Kind != ValueInt || calls.Int != 0 {
					t.Fatalf("construction evaluated the getter: calls=%v, err=%v", calls, err)
				}
				// Prove that ordinary lookup reaches the changing getter; a
				// second lookup would no longer see an empty set.
				for read, size := range []int{0, 1} {
					value, err := machine.lookup("parents")
					if err != nil || value.Kind != ValueSet || len(value.Set) != size {
						t.Fatalf("ordinary getter lookup %d: value=%v, err=%v", read+1, value, err)
					}
				}
			})
		}
	}
}

func TestEmptyReferenceSumLocatorRejectsStaticSources(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, source := range []string{"currentStatic", "inheritedStatic", "qualifiedStatic"} {
			t.Run(api+"/"+source, func(t *testing.T) {
				machine := emptyReferenceSumLocatorMachine(api)
				parents := Set()
				parents.Type = "Set<Id>"
				if err := machine.RegisterClass(Class{Name: "StaticBindSource", APIVersion: api, StaticFields: map[string]Field{
					"parents": {Name: "parents", Type: "Set<Id>", Static: true, Value: parents},
				}}); err != nil {
					t.Fatal(err)
				}
				machine.currentClass = "StaticBindSource"
				if source == "inheritedStatic" {
					machine.currentClass = "DerivedStaticBindSource"
					if err := machine.RegisterClass(Class{Name: machine.currentClass, SuperClass: "StaticBindSource", APIVersion: api}); err != nil {
						t.Fatal(err)
					}
				}
				machine.currentMethod = Method{ClassName: machine.currentClass, APIVersion: api, IsStatic: true}
				name := "parents"
				if source == "qualifiedStatic" {
					name = "StaticBindSource.parents"
				}
				machine.VarTypes[name] = "Set<Id>"
				_, err := machine.databaseQueryHandle(String(emptyReferenceSumBindingScopeQuery(name)), Null, Null, false, &Result{})
				assertEmptyReferenceSumBindingScopeRejection(t, err)
				value, err := machine.lookup(name)
				if err != nil || value.Kind != ValueSet || len(value.Set) != 0 {
					t.Fatalf("ordinary static lookup: value=%v, err=%v", value, err)
				}
			})
		}
	}
}

func TestEmptyReferenceSumLocatorRejectsClosedLocalSource(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			machine := emptyReferenceSumLocatorMachine(api)
			body := "{ Set<Id> parents = new Set<Id>(); } Database.getQueryLocator('" + emptyReferenceSumBindingScopeQuery("parents") + "');"
			program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			_, err = machine.Execute(program)
			assertEmptyReferenceSumBindingScopeRejection(t, err)
			if _, present := machine.lookupGlobalName("parents"); present {
				t.Fatal("closed local remains an active binding")
			}
		})
	}
}

func emptyReferenceSumBindingScopeQuery(name string) string {
	return strings.ReplaceAll("SELECT AccountId,SUM(Amount) total FROM Opportunity WHERE AccountId IN :parents AND Amount > 200 GROUP BY AccountId", ":parents", ":"+name)
}

func assertEmptyReferenceSumBindingScopeRejection(t *testing.T, err error) {
	t.Helper()
	const message = "Aggregate query does not support queryMore(), use LIMIT to restrict the results to a single batch"
	if err == nil || exceptionQualifiedTypeName(batchErrorExceptionType(err)) != "System.QueryException" || batchErrorMessage(err) != message {
		t.Fatalf("source outside the local candidate must retain aggregate rejection: %v", err)
	}
}
