package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// Safe navigation retains the declared member type when a descriptor is null.
// The Integer length property must therefore select the Integer overload.
func TestDescribeSafeFieldNullPreservesOverloadType(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct{ name, source string }{
			{"localDescriptor", `Schema.DescribeFieldResult descriptor=null;
System.assertEquals('integer', FieldWidthDispatch.pick(descriptor?.length));`},
			{"helperDescriptor", `System.assertEquals('integer', FieldWidthDispatch.pick(FieldWidthDispatch.describe(null)?.length));`},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				machine, compile := describeSafeFieldMachine(t, api)
				if _, err := machine.Execute(compile(tc.source)); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestDescribeSafeFieldOverloadControls(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct{ name, source string }{
			{"nonnullDescriptor", `Schema.DescribeFieldResult descriptor=Account.Name.getDescribe();
System.assertEquals('integer', FieldWidthDispatch.pick(descriptor?.length));`},
			{"nonnullHelperDescriptor", `System.assertEquals('integer', FieldWidthDispatch.pick(FieldWidthDispatch.describe(Account.Name)?.length));`},
			{"customNullProperty", `WidthEnvelope descriptor=null;
System.assertEquals('integer', FieldWidthDispatch.pick(descriptor?.length));`},
			{"customNonnullProperty", `WidthEnvelope descriptor=new WidthEnvelope(); descriptor.length=23;
System.assertEquals('integer', FieldWidthDispatch.pick(descriptor?.length));`},
			{"integerNull", `System.assertEquals('integer', FieldWidthDispatch.pick((Integer)null));`},
			{"fieldNull", `System.assertEquals('field', FieldWidthDispatch.pick((SObjectField)null));`},
			{"fieldToken", `System.assertEquals('field', FieldWidthDispatch.pick(Account.Name));`},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				machine, compile := describeSafeFieldMachine(t, api)
				if _, err := machine.Execute(compile(tc.source)); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func describeSafeFieldMachine(t *testing.T, api string) (*VM, func(string) ir.Program) {
	t.Helper()
	compile := func(source string) ir.Program {
		t.Helper()
		program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		return program
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	for _, class := range []Class{
		{Name: "FieldWidthDispatch", APIVersion: api},
		{Name: "WidthEnvelope", APIVersion: api, Fields: map[string]Field{
			"length": {Name: "length", Type: "Integer"},
		}},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	for _, overload := range []struct{ param, result string }{
		{"Integer", "integer"},
		{"SObjectField", "field"},
	} {
		if err := machine.RegisterMethod(Method{
			Name: "FieldWidthDispatch.pick", ClassName: "FieldWidthDispatch", IsStatic: true,
			APIVersion: api, ReturnType: "String", Params: []Param{{Name: "value", Type: overload.param}},
			Program: compile("return '" + overload.result + "';"),
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := machine.RegisterMethod(Method{
		Name: "FieldWidthDispatch.describe", ClassName: "FieldWidthDispatch", IsStatic: true,
		APIVersion: api, ReturnType: "Schema.DescribeFieldResult", Params: []Param{{Name: "field", Type: "SObjectField"}},
		Program: compile("if(field==null) return null; return field.getDescribe();"),
	}); err != nil {
		t.Fatal(err)
	}
	return machine, compile
}
