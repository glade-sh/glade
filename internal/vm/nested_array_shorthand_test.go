package vm

import "testing"

// NA01 observes these forms in a deployed helper at API 62/67/68.
// See apextest/testdata/conformance/nested_array_shorthand.json.
func TestExecNestedArrayShorthand(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"empty construction", `Object matrix = new Integer[][]{}; System.assertNotEquals(null, matrix);`},
		{"local declaration", `Integer[][] matrix = new List<List<Integer>>(); matrix.add(new Integer[]{2,3}); System.assertEquals(2,matrix[0].size());`},
		{"three dimensions", `Object[][][] cube = new Object[][][]{new Object[][]{new Object[]{'x'}}}; System.assertEquals('x',cube[0][0][0]);`},
		{"cast", `Object value = new List<List<Integer>>(); Integer[][] matrix = (Integer[][])value; System.assertEquals(0,matrix.size());`},
		{"class literal", `Type matrixType = Integer[][].class; System.assertNotEquals(null,matrixType);`},
		{"flat array control", `Integer[] flat = new Integer[]{8}; System.assertEquals(1,flat.size()); System.assertEquals(8,flat[0]);`},
		{"generic list control", `List<List<Integer>> matrix = new List<List<Integer>>{new List<Integer>{4}}; System.assertEquals(4,matrix[0][0]);`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := New(nil).Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
