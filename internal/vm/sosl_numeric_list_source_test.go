package vm

import "testing"

// This preserves the existing local bind route; it makes no Salesforce parity
// claim for numeric collection binds. The native conformance fixture captures
// literal lists separately, which must still reject numeric text-field values.
func TestSOSLNumericListBindSource(t *testing.T) {
	for _, test := range []struct{ name, expression string }{
		{"dynamic", `Search.query('FIND \'quotednumericnomatch\' RETURNING Account(Id WHERE Name IN :numbers)')`},
		{"inline", `[FIND 'quotednumericnomatch' RETURNING Account(Id WHERE Name IN :numbers)]`},
		{"nested", `Search.query('FIND \'quotednumericnomatch\' RETURNING Account(Id WHERE Name = \'other\' OR Name IN :numbers)')`},
	} {
		t.Run(test.name, func(t *testing.T) {
			program, err := CompileAnonymous(`List<Integer> numbers=new List<Integer>{1,2}; List<List<SObject>> groups=` + test.expression + `; System.assertEquals(0,groups[0].size());`)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
