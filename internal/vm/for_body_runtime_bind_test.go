package vm

import "testing"

// A query string built at run time can bind a loop-body local before this
// iteration declares it. Reusing last iteration's binding would let the bind
// succeed where it must fail, so such loops keep fresh per-iteration scopes.
func TestForBodyRuntimeBindDoesNotSeeStaleLocal(t *testing.T) {
	cases := map[string]string{
		"database-cursor": `String r=''; for(Integer i=0;i<2;i++){ try { Database.getCursor('SELECT Id FROM Account WHERE Name = :n' + 'm'); r+='ok'+i; } catch(Exception e){ r+='ex'+i; } String nm='x'; } System.debug('ROW '+r);`,
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			program, err := CompileAnonymous(source)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetTraceEnabled(false)
			org := testDataOrg()
			machine.SetOrg(&org)
			result, err := machine.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			got := result.Debug
			if len(got) != 1 || got[0] != "ROW ex0ex1" {
				t.Fatalf("got %v want ROW ex0ex1", got)
			}
		})
	}
}
