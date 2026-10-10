package vm

import "testing"

func TestSpacedGreaterEqualPreservesAdjacentOperatorsAndTypes(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(true,0 > = 0);
System.assertEquals(true,1 >= 0);
System.assertEquals(false,-1 > = 0);
System.assertEquals(false,0 > 0);
System.assertEquals(true,-1 < 0);
System.assertEquals(true,0 <= 0);
System.assertEquals(true,1 == 1);
System.assertEquals(true,1 != 0);
List<List<Integer>> groups = new List<List<Integer>>{new List<Integer>{7}};
System.assertEquals(7,groups[0][0]);
Integer assigned = 2;
assigned += 1;
System.assertEquals(3,assigned);
System.assertEquals('> =','> =');
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestSpacedGreaterEqualRejectsMissingAndExtraOperands(t *testing.T) {
	for _, source := range []string{
		`Boolean value = 1 > =;`,
		`Boolean value = 1 > = = 0;`,
		`Boolean value = 1 > == 0;`,
	} {
		t.Run(source, func(t *testing.T) {
			if _, err := CompileAnonymous(source); err == nil {
				t.Fatal("invalid relational expression compiled")
			}
		})
	}
}
