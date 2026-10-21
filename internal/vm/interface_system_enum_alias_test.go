package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// The interface and concrete parameter declarations denote the same platform
// enum even when only one declaration includes its System namespace.
func TestInterfaceSystemEnumAliasDispatch(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, spelling := range []struct {
			name, contract, implementation string
		}{
			{"qualifiedContract", "System.LoggingLevel", "LoggingLevel"},
			{"qualifiedImplementation", "LoggingLevel", "System.LoggingLevel"},
		} {
			for _, selected := range []string{"First", "Second"} {
				t.Run(api+"/"+spelling.name+"/"+selected, func(t *testing.T) {
					machine, compile := systemEnumAliasMachine(t, api, spelling.contract, spelling.implementation)
					program := compile(`
Object reflected = Type.forName('SeverityOwner.` + selected + `').newInstance();
SeverityContract subject = (SeverityContract)reflected;
subject.emit(LoggingLevel.INFO, 'reflected');
System.assertEquals('` + selected + `:INFO:reflected', SeverityOwner.received);
`)
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
		t.Run(api+"/directConcreteControl", func(t *testing.T) {
			machine, compile := systemEnumAliasMachine(t, api, "System.LoggingLevel", "LoggingLevel")
			program := compile(`
new SeverityOwner.First().emit(LoggingLevel.DEBUG, 'direct');
System.assertEquals('First:DEBUG:direct', SeverityOwner.received);
new SeverityOwner.Second().emit(LoggingLevel.WARN, 'direct');
System.assertEquals('Second:WARN:direct', SeverityOwner.received);
`)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func systemEnumAliasMachine(t *testing.T, api, contractType, implementationType string) (*VM, func(string) ir.Program) {
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
	for _, class := range []Class{
		{Name: "SeverityContract", APIVersion: api, Access: "public", IsInterface: true},
		{Name: "SeverityOwner", APIVersion: api, Access: "public", StaticFields: map[string]Field{
			"received": {Name: "received", Type: "String", Static: true, Access: "public", Value: String(""), InitialValue: String("")},
		}},
		{Name: "SeverityOwner.First", APIVersion: api, Access: "public", Interfaces: []string{"SeverityContract"}},
		{Name: "SeverityOwner.Second", APIVersion: api, Access: "public", Interfaces: []string{"SeverityContract"}},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if err := machine.RegisterMethod(Method{
		Name: "SeverityContract.emit", ClassName: "SeverityContract", APIVersion: api, Access: "public", ReturnType: "void", Modifiers: []string{"abstract"},
		Params: []Param{{Name: "severity", Type: contractType}, {Name: "body", Type: "String"}},
	}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"First", "Second"} {
		if err := machine.RegisterMethod(Method{
			Name: "SeverityOwner." + name + ".emit", ClassName: "SeverityOwner." + name, APIVersion: api, Access: "public", ReturnType: "void",
			Params:  []Param{{Name: "severity", Type: implementationType}, {Name: "body", Type: "String"}},
			Program: compile("SeverityOwner.received = '" + name + ":' + severity.name() + ':' + body;"),
		}); err != nil {
			t.Fatal(err)
		}
	}
	return machine, compile
}

func TestInterfaceSystemEnumAliasShadowControls(t *testing.T) {
	for _, shadow := range []string{"LoggingLevel", "SeverityOwner.LoggingLevel"} {
		t.Run(shadow, func(t *testing.T) {
			machine := New(nil)
			for _, class := range []Class{
				{Name: "SeverityContract", IsInterface: true},
				{Name: "SeverityOwner"},
				{Name: "SeverityOwner.Sink"},
				{Name: shadow, EnumValues: []string{"Custom"}},
			} {
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			contract := Method{ClassName: "SeverityContract", Params: []Param{{Type: "System.LoggingLevel"}}}
			implementation := Method{ClassName: "SeverityOwner.Sink", Params: []Param{{Type: "LoggingLevel"}}}
			if machine.sameResolvedMethodParams(contract, implementation) || machine.sameResolvedMethodParams(implementation, contract) {
				t.Fatal("a project enum must remain distinct from the platform enum")
			}
			matching := Method{ClassName: "SeverityOwner.Sink", Params: []Param{{Type: shadow}}}
			if !machine.sameResolvedMethodParams(implementation, matching) || !machine.sameResolvedMethodParams(matching, implementation) {
				t.Fatal("lexical and qualified declarations of the project enum must retain their identity")
			}
		})
	}
}
