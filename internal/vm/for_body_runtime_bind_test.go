package vm

import (
	"fmt"
	"testing"
)

// A query string built at run time can bind a loop-body local before this
// iteration declares it. Reusing last iteration's binding would let the bind
// succeed where it must fail, so loops that reach cursor APIs keep fresh
// per-iteration scopes. Rows cover both cursor routes, classic and enhanced
// for, in anonymous code and in a class method.
func TestLoopBodyRuntimeBindDoesNotSeeStaleLocal(t *testing.T) {
	routes := map[string]string{
		"getCursor":           `Database.getCursor('SELECT Id FROM Account WHERE Name = :n' + 'm');`,
		"getPaginationCursor": `Database.getPaginationCursor('SELECT Id FROM Account WHERE Name = :n' + 'm');`,
	}
	loops := map[string]string{
		"for":     `for(Integer i=0;i<2;i++){ %s }`,
		"foreach": `for(Integer i : new List<Integer>{0,1}){ %s }`,
	}
	for route, call := range routes {
		for loop, template := range loops {
			body := fmt.Sprintf(template, `try { `+call+` r+='ok'+i; } catch(Exception e){ r+='ex'+i; } String nm='x';`)
			for _, context := range []string{"anonymous", "method"} {
				t.Run(route+"/"+loop+"/"+context, func(t *testing.T) {
					machine := New(nil)
					machine.SetTraceEnabled(false)
					org := testDataOrg()
					machine.SetOrg(&org)
					source := `String r=''; ` + body + ` System.debug('ROW '+r);`
					if context == "method" {
						method, err := CompileAnonymous(`String r=''; ` + body + ` return r;`)
						if err != nil {
							t.Fatal(err)
						}
						if err := machine.RegisterClass(Class{Name: "CursorHost"}); err != nil {
							t.Fatal(err)
						}
						if err := machine.RegisterMethod(Method{Name: "CursorHost.run", ClassName: "CursorHost", IsStatic: true, ReturnType: "String", Program: method}); err != nil {
							t.Fatal(err)
						}
						source = `System.debug('ROW ' + CursorHost.run());`
					}
					program, err := CompileAnonymous(source)
					if err != nil {
						t.Fatal(err)
					}
					result, err := machine.Execute(program)
					if err != nil {
						t.Fatal(err)
					}
					if got := result.Debug; len(got) != 1 || got[0] != "ROW ex0ex1" {
						t.Fatalf("got %v want ROW ex0ex1", got)
					}
				})
			}
		}
	}
}
