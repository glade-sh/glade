package vm

import "testing"

// NM01 captures these mutations at API 62/67/68, anonymous and @IsTest.
// See apextest/testdata/conformance/nested_collection_mutation.json.
func TestExecNestedCollectionMutation(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"array third level", `Object[][][] grid = new Object[][][]{};
		 grid.add(new Object[][]{}); grid[0].add(new Object[]{});
		 grid[0][0].add(null); System.assertEquals(1,grid[0][0].size());
		 grid[0][0][0]='sample'; System.assertEquals('sample',grid[0][0][0]);`},
		{"generic third level", `List<List<List<Integer>>> grid = new List<List<List<Integer>>>{new List<List<Integer>>{new List<Integer>()}};
		 grid[0][0].add(9); System.assertEquals(1,grid[0][0].size()); System.assertEquals(9,grid[0][0][0]);`},
		{"named alias and sibling", `Object[][][] grid = new Object[][][]{new Object[][]{new Object[]{},new Object[]{}}};
		 Object[] alias=grid[0][0]; alias.add('alias');
		 System.assertEquals(1,grid[0][0].size()); System.assertEquals('alias',grid[0][0][0]); System.assertEquals(0,grid[0][1].size());`},
		{"two level control", `Object[][] grid = new Object[][]{new Object[]{}};
		 grid[0].add('flat'); System.assertEquals(1,grid[0].size()); System.assertEquals('flat',grid[0][0]);`},
		{"index error control", `Object[][][] grid = new Object[][][]{new Object[][]{new Object[]{}}};
		 Boolean caught=false; try { grid[0][1].add('bad'); } catch(ListException e) { caught=true; }
		 System.assert(caught); System.assertEquals(0,grid[0][0].size());`},
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
