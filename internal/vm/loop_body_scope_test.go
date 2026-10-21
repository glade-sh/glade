package vm

import "testing"

// Loop bodies are lexical blocks. A local from a completed iteration cannot
// become the receiver in the next iteration's declaration initializer.
// Native block-scope controls: name_resolution H2S1/H2S2 and for_loop_scope
// C194S011/C194S044-C194S049 at APIs 62/67/68 where captured.
func TestLoopBodyLocalsDoNotEscapeIteration(t *testing.T) {
	for _, loop := range []struct{ name, start string }{
		{name: "for", start: "for (Integer index=0; index<2; index++)"},
		{name: "foreach", start: "for (Integer index : new List<Integer>{0,1})"},
	} {
		for _, exit := range []struct{ name, code, count string }{
			{name: "normal", count: "2"},
			{name: "continue", code: "continue;", count: "2"},
			{name: "break", code: "break;", count: "1"},
		} {
			t.Run(loop.name+"/"+exit.name, func(t *testing.T) {
				machine := New(nil)
				program, err := CompileAnonymous("Integer visits=0; List<Contact> retained = new List<Contact>(); " + loop.start + ` {
  Contact contact = (Contact)Contact.SObjectType.newSObject();
  retained.add(contact);
  visits++;
  ` + exit.code + `
}
System.assertEquals(` + exit.count + `, visits);
System.assertEquals(` + exit.count + `, retained.size());
System.assertEquals('Contact',Contact.SObjectType.getDescribe().getName());`)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
				for _, local := range []string{"contact", "index"} {
					if _, exists := machine.Globals[local]; exists {
						t.Fatalf("loop local %s survived the loop", local)
					}
					if _, exists := machine.VarTypes[local]; exists {
						t.Fatalf("loop local type %s survived the loop", local)
					}
				}
			})
		}
	}
}
