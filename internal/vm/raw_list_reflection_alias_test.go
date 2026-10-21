package vm

import "testing"

func TestRawListAliasParsedReflectionSource61(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
Object source=new List<Object>{'word'};
try { Date value=(Date)source; System.assert(false); }
catch(TypeException problem) {
 String label=problem.getMessage().substringAfter('Invalid conversion from runtime type ').substringBefore(' to Date');
 System.assertEquals('List<ANY>',label);
 Type parsed=Type.forName(label);
 System.assertNotEquals(null,parsed);
 System.assert(Type.forName('List').isAssignableFrom(parsed));
}
`, CompileOptions{APIVersion: "61.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestRawListReflectionObservedPairsSource61(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
System.assert(Type.forName('List').isAssignableFrom(Type.forName('List<Object>')));
System.assert(Type.forName('List').isAssignableFrom(Type.forName('List<String>')));
`, CompileOptions{APIVersion: "61.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestRawListAliasNativeControlsSource61(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
System.assert(Type.forName('List<String>').isAssignableFrom(Type.forName('List<String>')));
System.assertEquals(false,Type.forName('List').isAssignableFrom(Type.forName('Set<Date>')));
`, CompileOptions{APIVersion: "61.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

// Unobserved aliases/versions retain their local baseline behavior. These
// controls do not claim native parity outside the captured source61 shape.
func TestRawListAliasKeepsUnobservedShapes(t *testing.T) {
	for _, api := range []string{"54.0", "60.0", "62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			machine := New(nil)
			machine.currentMethod.APIVersion = api
			if machine.rawListReflectionCandidate("List<String>", "List") {
				t.Fatalf("uncaptured String/raw reflection changed at %s", api)
			}
			if got := machine.typeForName("", "List<ANY>", false); got.Kind != ValueNull {
				t.Fatalf("uncaptured source %s changed: %v", api, got)
			}
		})
	}
	machine := New(nil)
	machine.currentMethod.APIVersion = "61.0"
	for _, name := range []string{"list<any>", "List<Any>", "Set<ANY>", "Map<String,ANY>", "List<List<ANY>>", " List<ANY>", "System.List<ANY>"} {
		if got := machine.typeForName("", name, false); got.Kind != ValueNull {
			t.Fatalf("uncaptured alias %q changed: %v", name, got)
		}
	}
	for _, ns := range []string{"", "System", "OwnedScope"} {
		if got := machine.typeForName(ns, "List<ANY>", true); got.Kind != ValueNull {
			t.Fatalf("explicit namespace %q changed: %v", ns, got)
		}
	}
}
