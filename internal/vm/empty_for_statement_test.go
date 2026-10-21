package vm

import "testing"

func TestEmptyForStatementExecutesHeader(t *testing.T) {
	cases := []struct{ name, source string }{
		{"iterator tail", `List<Object> values=new List<Object>{1,2,3}; Object last; for(Iterator<Object> it=values.iterator();it.hasNext();last=it.next()); System.assertEquals(3,last);`},
		{"empty iterator", `List<Object> values=new List<Object>(); Object last; for(Iterator<Object> it=values.iterator();it.hasNext();last=it.next()); System.assertEquals(null,last);`},
		{"update side effect", `Integer ticks=0; for(;ticks<3;ticks++); System.assertEquals(3,ticks);`},
		{"zero iterations", `Integer ticks=0; for(;false;ticks++); System.assertEquals(0,ticks);`},
		{"braced body control", `List<Object> values=new List<Object>{1,2,3}; Object last; for(Iterator<Object> it=values.iterator();it.hasNext();last=it.next()) {} System.assertEquals(3,last);`},
		{"unbraced body control", `List<Object> values=new List<Object>{1,2,3}; Object last; for(Iterator<Object> it=values.iterator();it.hasNext();) last=it.next(); System.assertEquals(3,last);`},
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, test := range cases {
			t.Run(api+"/"+test.name, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(test.source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := Execute(program, nil); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestEmptyForStatementRejectsMalformedHeader(t *testing.T) {
	for _, source := range []string{
		`Integer ticks=0; for(;ticks<3;ticks++; System.assertEquals(0,ticks);`,
		`Integer ticks=0; for(;ticks<3;ticks++);` + ` throw;`,
	} {
		if _, err := CompileAnonymous(source); err == nil {
			t.Fatalf("malformed source accepted: %s", source)
		}
	}
}
