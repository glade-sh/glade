package vm

import (
	"fmt"
	"strings"
	"testing"
)

// The source-shadow native packet accepts this public source declaration and
// its constructor, fields, method, class literal and instanceof at API62/67/68
// in anonymous and named test execution. These VM cases cover those operations;
// descriptor JSON is deliberately separate from declaration acceptance.
func TestDescribeSourceShadowNativeConstruction(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, objectName := range []string{"User", "Account"} {
			t.Run(api+"/"+objectName, func(t *testing.T) {
				machine := describeSourceShadowVM(t, api)
				program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
Schema.DescribeSObjectResult descriptor = new Schema.DescribeSObjectResult('%s');
System.assertEquals('owned-source', descriptor.sourceMarker());
System.assertEquals('%s', descriptor.name);
System.assertEquals('DEFERRED', descriptor.sObjectDescribeOption);
Object erased = descriptor;
System.assert(erased instanceof Schema.DescribeSObjectResult);
System.assertEquals('Schema.DescribeSObjectResult', Schema.DescribeSObjectResult.class.getName());
`, objectName, objectName), CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
				// Local allocation-provenance control, not a native-visible fact.
				value := machine.Globals["descriptor"]
				if !value.classInstance || !value.projectClass {
					t.Fatal("source construction lost class allocation provenance")
				}
			})
		}
	}
}

// The second native packet observes only the two authored fields under both
// serialize overloads, with the source method still callable afterward. It does
// not project native describe URLs onto an instance of the source-defined type.
func TestDescribeSourceShadowNativeJSON(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, objectName := range []string{"User", "Account"} {
			t.Run(api+"/"+objectName, func(t *testing.T) {
				machine := describeSourceShadowVM(t, api)
				program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
Schema.DescribeSObjectResult descriptor = new Schema.DescribeSObjectResult('%s');
Object expected = JSON.deserializeUntyped('{"name":"%s","sObjectDescribeOption":"DEFERRED"}');
System.assertEquals(expected, JSON.deserializeUntyped(JSON.serialize(descriptor)));
System.assertEquals(expected, JSON.deserializeUntyped(JSON.serialize(descriptor, true)));
System.assertEquals('owned-source', descriptor.sourceMarker());
`, objectName, objectName), CompileOptions{APIVersion: api})
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

func describeSourceShadowVM(t *testing.T, api string) *VM {
	t.Helper()
	constructor, err := CompileAnonymousWithOptions(`name = objectName; sObjectDescribeOption = 'DEFERRED';`, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	marker, err := CompileAnonymousWithOptions(`return 'owned-source';`, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, class := range []Class{
		{Name: "Schema", APIVersion: api, Access: "public", PlatformType: false},
		{
			Name: "Schema.DescribeSObjectResult", APIVersion: api, Access: "public", PlatformType: false,
			Fields: map[string]Field{
				"name":                  {Name: "name", Type: "String", Access: "public"},
				"sObjectDescribeOption": {Name: "sObjectDescribeOption", Type: "String", Access: "public"},
			},
			Constructors: []Method{{
				Name: "Schema.DescribeSObjectResult.<init>", ClassName: "Schema.DescribeSObjectResult",
				APIVersion: api, Access: "public", IsConstructor: true, SourceContextBound: true,
				Params: []Param{{Name: "objectName", Type: "String"}}, Program: constructor,
			}},
			Methods: map[string]Method{"sourceMarker": {
				Name: "Schema.DescribeSObjectResult.sourceMarker", ClassName: "Schema.DescribeSObjectResult",
				APIVersion: api, Access: "public", ReturnType: "String", SourceContextBound: true, Program: marker,
			}},
		},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	return machine
}

// Existing platform constructor rejection must survive the source precedence
// fix. These local provenance controls add no native constructor observations.
func TestDescribeSourceShadowPreservesPlatformConstructorRestriction(t *testing.T) {
	for _, registered := range []bool{false, true} {
		t.Run(fmt.Sprintf("registeredPlatform=%t", registered), func(t *testing.T) {
			machine := New(nil)
			if registered {
				if err := machine.RegisterClass(Class{Name: "Schema.DescribeSObjectResult", Access: "global", PlatformType: true}); err != nil {
					t.Fatal(err)
				}
			}
			_, err := machine.constructValueWithLiteral("Schema.DescribeSObjectResult", []Value{String("User")}, nil, &Result{}, false)
			if err == nil || !strings.Contains(err.Error(), `unsupported call "Schema.DescribeSObjectResult constructor"`) {
				t.Fatalf("platform constructor restriction changed: %v", err)
			}
		})
	}
}
